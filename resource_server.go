package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

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
	Name string `json:"name"`
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
	return diags
}

func resourceServerRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	name := d.Id()
	project := d.Get("project").(string)

	path := fmt.Sprintf("/servers/%s?project_name=%s", url.PathEscape(name), url.QueryEscape(project))
	req, err := c.newRequest("GET", path)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return diags
	}

	if resp.StatusCode != 200 {
		return diag.Errorf("Failed to read server: %s", resp.Status)
	}

	// If needed, parse the server info and update state accordingly.
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
