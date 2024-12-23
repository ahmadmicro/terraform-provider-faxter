package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type ServerCreateRequest struct {
	Project         string   `json:"project,omitempty"`
	Name            string   `json:"name"`
	Flavor          string   `json:"flavor,omitempty"`
	Image           string   `json:"image,omitempty"`
	KeyName         string   `json:"key_name"`
	SecurityGroups  []string `json:"security_groups,omitempty"`
	RequestFloating bool     `json:"request_floating_ip,omitempty"`
	CloudInit       string   `json:"cloud_init,omitempty"`
	Networks        []string `json:"networks,omitempty"`
	Volumes         []string `json:"volumes,omitempty"`
}

type ServerUpdateRequest struct {
	Name            string    `json:"name"` // required by ServerUpdate schema
	Flavor          *string   `json:"flavor,omitempty"`
	Image           *string   `json:"image,omitempty"`
	SecurityGroups  *[]string `json:"security_groups,omitempty"`
	RequestFloating *bool     `json:"request_floating_ip,omitempty"`
	Networks        *[]string `json:"networks,omitempty"`
	Volumes         *[]string `json:"volumes,omitempty"`
}

type ResourceResponse struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Properties struct {
		IPAddresses []string `json:"ip_addresses"`
		// Add other fields if needed
	} `json:"properties"`
	// ... additional fields if needed
}

func resourceServer() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceServerCreate,
		ReadContext:   resourceServerRead,
		UpdateContext: resourceServerUpdate,
		DeleteContext: resourceServerDelete,
		Schema: map[string]*schema.Schema{
			"project": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "default",
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"key_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"flavor": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "copper",
			},
			"image": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "Ubuntu2204",
			},
			"security_groups": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				DefaultFunc: func() (interface{}, error) {
					return []interface{}{"default"}, nil
				},
			},
			"request_floating_ip": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"cloud_init": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"networks": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				DefaultFunc: func() (interface{}, error) {
					return []interface{}{"public1"}, nil
				},
			},
			"volumes": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				DefaultFunc: func() (interface{}, error) {
					return []interface{}{}, nil
				},
			},
			// New computed attribute to capture IP addresses
			"ip_addresses": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceServerCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	project := d.Get("project").(string)
	name := d.Get("name").(string)
	flavor := d.Get("flavor").(string)
	image := d.Get("image").(string)
	keyName := d.Get("key_name").(string)
	requestFloating := d.Get("request_floating_ip").(bool)
	cloudInit := d.Get("cloud_init").(string)

	networks := expandStringList(d.Get("networks").([]interface{}))
	volumes := expandStringList(d.Get("volumes").([]interface{}))
	securityGroups := expandStringList(d.Get("security_groups").([]interface{}))

	reqData := &ServerCreateRequest{
		Project:         project,
		Name:            name,
		Flavor:          flavor,
		Image:           image,
		KeyName:         keyName,
		SecurityGroups:  securityGroups,
		RequestFloating: requestFloating,
		CloudInit:       cloudInit,
		Networks:        networks,
		Volumes:         volumes,
	}

	bodyBytes, _ := json.Marshal(reqData)
	req, err := c.newRequest("POST", "/servers/")
	if err != nil {
		return diag.FromErr(err)
	}
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return diag.Errorf("Failed to create server: %s", resp.Status)
	}

	var resourceResps []ResourceResponse
	err = json.NewDecoder(resp.Body).Decode(&resourceResps)
	if err != nil {
		return diag.FromErr(err)
	}

	// Assume count=1 for simplicity. If multiple, handle accordingly.
	if len(resourceResps) == 0 {
		return diag.Errorf("No server returned in create response")
	}

	d.SetId(resourceResps[0].Name)

	// Implement polling to wait until the server status is "online"
	pollTimeout := 5 * time.Minute
	pollInterval := 10 * time.Second
	deadline := time.Now().Add(pollTimeout)

	for {
		// Wait for the next poll interval
		time.Sleep(pollInterval)

		// Check if context is done
		if ctx.Err() != nil {
			return diag.FromErr(ctx.Err())
		}

		// Read the current server status
		currentStatus, ipAddresses, err := getServerStatus(ctx, c, project, name)
		if err != nil {
			return diag.Errorf("Error fetching server status: %s", err)
		}

		// Update the status in the Terraform state
		if err := d.Set("status", currentStatus); err != nil {
			return diag.Errorf("Error setting status: %s", err)
		}

		// If status is "online", proceed to set ip_addresses and exit the loop
		if currentStatus == "online" {
			if err := d.Set("ip_addresses", ipAddresses); err != nil {
				return diag.Errorf("Error setting ip_addresses: %s", err)
			}
			break
		}

		// Check if the deadline has been reached
		if time.Now().After(deadline) {
			return diag.Errorf("Timed out waiting for server '%s' to become online", name)
		}
	}

	return diags
}

// getServerStatus fetches the current status and IP addresses of the server.
func getServerStatus(ctx context.Context, c *Client, project, name string) (string, []string, error) {
	// Construct the API path with query parameters
	path := fmt.Sprintf("/servers/%s?project_name=%s", url.PathEscape(name), url.QueryEscape(project))
	req, err := c.newRequest("GET", path)
	if err != nil {
		return "", nil, err
	}

	// Send the HTTP request
	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	// Handle 404 Not Found
	if resp.StatusCode == 404 {
		return "", nil, fmt.Errorf("server '%s' not found", name)
	}

	// Check for successful response
	if resp.StatusCode != 200 {
		// Read response body for error details
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("failed to get server status: %s - %s", resp.Status, string(body))
	}

	// Decode the response
	var resourceResps ResourceResponse
	err = json.NewDecoder(resp.Body).Decode(&resourceResps)
	if err != nil {
		return "", nil, fmt.Errorf("error decoding read response: %s", err)
	}

	currentStatus := resourceResps.Status
	ipAddresses := resourceResps.Properties.IPAddresses

	return currentStatus, ipAddresses, nil
}

// resourceServerRead handles reading the server resource from the API.
func resourceServerRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	name := d.Id()
	project := d.Get("project").(string)

	// Read the current server status and IP addresses
	currentStatus, ipAddresses, err := getServerStatus(ctx, c, project, name)
	if err != nil {
		if err.Error() == fmt.Sprintf("server '%s' not found", name) {
			d.SetId("")
			return diags
		}
		return diag.Errorf("Error reading server: %s", err)
	}

	// Update the state with status and ip_addresses
	if err := d.Set("status", currentStatus); err != nil {
		return diag.Errorf("Error setting status: %s", err)
	}

	if err := d.Set("ip_addresses", ipAddresses); err != nil {
		return diag.Errorf("Error setting ip_addresses: %s", err)
	}

	return diags
}

func resourceServerUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	name := d.Id()
	project := d.Get("project").(string)

	// According to ServerUpdate schema, name is required
	updateReq := &ServerUpdateRequest{
		Name: d.Get("name").(string),
	}

	if d.HasChange("flavor") {
		flavor := d.Get("flavor").(string)
		updateReq.Flavor = &flavor
	}
	if d.HasChange("image") {
		image := d.Get("image").(string)
		updateReq.Image = &image
	}
	if d.HasChange("request_floating_ip") {
		rf := d.Get("request_floating_ip").(bool)
		updateReq.RequestFloating = &rf
	}
	if d.HasChange("networks") {
		networks := expandStringList(d.Get("networks").([]interface{}))
		updateReq.Networks = &networks
	}
	if d.HasChange("volumes") {
		volumes := expandStringList(d.Get("volumes").([]interface{}))
		updateReq.Volumes = &volumes
	}
	if d.HasChange("security_groups") {
		securityGroups := expandStringList(d.Get("security_groups").([]interface{}))
		updateReq.SecurityGroups = &securityGroups
	}

	bodyBytes, _ := json.Marshal(updateReq)
	path := fmt.Sprintf("/servers/%s?project_name=%s", url.PathEscape(name), url.QueryEscape(project))
	req, err := c.newRequest("PUT", path)
	if err != nil {
		return diag.FromErr(err)
	}

	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return diag.Errorf("Failed to update server: %s", resp.Status)
	}

	return diags
}

func resourceServerDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	name := d.Id()
	project := d.Get("project").(string)
	path := fmt.Sprintf("/servers/%s?project_name=%s", url.PathEscape(name), url.QueryEscape(project))
	req, err := c.newRequest("DELETE", path)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return diag.Errorf("Failed to delete server: %s", resp.Status)
	}

	d.SetId("")
	return diags
}

// Helper function to convert a []interface{} to []string
func expandStringList(list []interface{}) []string {
	var result []string
	for _, v := range list {
		result = append(result, v.(string))
	}
	return result
}
