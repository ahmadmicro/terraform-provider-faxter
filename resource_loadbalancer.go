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

// ServerItem represents a single backend server object for the load balancer.
type ServerItem struct {
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Endpoint string `json:"endpoint"`
}

type LoadBalancerCreateRequest struct {
	Project           string       `json:"project,omitempty"`
	Name              string       `json:"name"`
	Port              int          `json:"port,omitempty"`
	Networks          []string     `json:"networks,omitempty"`
	SubNetworks       []string     `json:"sub_networks,omitempty"`
	KeyName           string       `json:"key_name,omitempty"`
	RequestFloatingIP bool         `json:"request_floating_ip,omitempty"`
	SSLEnabled        bool         `json:"ssl_enabled,omitempty"`
	Servers           []ServerItem `json:"servers,omitempty"`
	SecurityGroups    []string     `json:"security_groups,omitempty"`
}

// If your API has a separate "Update" schema, define it similarly.
// For simplicity, we'll reuse a structure, but typically you'd have a separate struct.
type LoadBalancerUpdateRequest struct {
	Name              string        `json:"name"` // required
	Port              *int          `json:"port,omitempty"`
	Networks          *[]string     `json:"networks,omitempty"`
	SubNetworks       *[]string     `json:"sub_networks,omitempty"`
	KeyName           *string       `json:"key_name,omitempty"`
	RequestFloatingIP *bool         `json:"request_floating_ip,omitempty"`
	SSLEnabled        *bool         `json:"ssl_enabled,omitempty"`
	Servers           *[]ServerItem `json:"servers,omitempty"`
	SecurityGroups    *[]string     `json:"security_groups,omitempty"`
}

// The API response might look like a ResourceResponse, or a custom LB struct
type LoadBalancerResponse struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Properties struct {
		// Possibly more detail here if your API returns it
	} `json:"properties"`
}

func resourceLoadBalancer() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceLoadBalancerCreate,
		ReadContext:   resourceLoadBalancerRead,
		UpdateContext: resourceLoadBalancerUpdate,
		DeleteContext: resourceLoadBalancerDelete,

		Schema: map[string]*schema.Schema{
			"project": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "default",
				Description: "Name of the Faxter project in which to create the load balancer.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Unique name of the load balancer.",
			},
			"port": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     80,
				Description: "The port on which the load balancer listens.",
			},
			"networks": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				DefaultFunc: func() (interface{}, error) {
					return []interface{}{"public1"}, nil
				},
				Description: "List of networks to which the load balancer is attached.",
			},
			"sub_networks": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				DefaultFunc: func() (interface{}, error) {
					return []interface{}{}, nil
				},
			},
			"key_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Optional SSH key name used if the LB runs in a VM-based context.",
			},
			"request_floating_ip": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether to request a floating IP for external connectivity.",
			},
			"ssl_enabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "If true, the load balancer will terminate SSL.",
			},
			"servers": {
				Type:        schema.TypeList,
				Required:    true,
				Description: "List of backend server objects for this load balancer.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"ip": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "IP address of the backend server.",
						},
						"port": {
							Type:        schema.TypeInt,
							Required:    true,
							Description: "Port of the backend server.",
						},
						"endpoint": {
							Type:        schema.TypeString,
							Optional:    true,
							Default:     "/",
							Description: "Endpoint path for this backend server.",
						},
					},
				},
			},
			"security_groups": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				DefaultFunc: func() (interface{}, error) {
					return []interface{}{"default"}, nil
				},
				Description: "One or more security groups attached to this load balancer.",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Current status of the load balancer (if returned by the API).",
			},
		},
	}
}

func resourceLoadBalancerCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	project := d.Get("project").(string)
	name := d.Get("name").(string)
	port := d.Get("port").(int)
	networks := expandStringList(d.Get("networks").([]interface{}))
	sub_networks := expandStringList(d.Get("sub_networks").([]interface{}))
	keyName := d.Get("key_name").(string)
	requestFloatingIP := d.Get("request_floating_ip").(bool)
	sslEnabled := d.Get("ssl_enabled").(bool)
	servers := expandServerItems(d.Get("servers").([]interface{}))
	securityGroups := expandStringList(d.Get("security_groups").([]interface{}))

	reqData := &LoadBalancerCreateRequest{
		Project:           project,
		Name:              name,
		Port:              port,
		Networks:          networks,
		SubNetworks:       sub_networks,
		KeyName:           keyName,
		RequestFloatingIP: requestFloatingIP,
		SSLEnabled:        sslEnabled,
		Servers:           servers,
		SecurityGroups:    securityGroups,
	}

	bodyBytes, _ := json.Marshal(reqData)
	req, err := c.newRequest("POST", "/loadbalancers/")
	if err != nil {
		return diag.FromErr(err)
	}
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return diag.Errorf("Failed to create load balancer: %s - %s", resp.Status, string(body))
	}

	var lbResp LoadBalancerResponse
	err = json.NewDecoder(resp.Body).Decode(&lbResp)
	if err != nil {
		return diag.FromErr(err)
	}

	// Use the name from the response as the Terraform ID
	d.SetId(lbResp.Name)

	// If the API returns a status, record it
	_ = d.Set("status", lbResp.Status)

	return diags
}

func resourceLoadBalancerRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	project := d.Get("project").(string)
	name := d.Id()

	path := fmt.Sprintf("/loadbalancers/%s?project_name=%s", url.PathEscape(name), url.PathEscape(project))
	req, err := c.newRequest("GET", path)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return diags
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return diag.Errorf("Failed to read load balancer: %s - %s", resp.Status, string(body))
	}

	var lbResp LoadBalancerResponse
	err = json.NewDecoder(resp.Body).Decode(&lbResp)
	if err != nil {
		return diag.FromErr(err)
	}

	// Update any known fields. The API might not return all fields; if so, we skip updating them.
	_ = d.Set("status", lbResp.Status)

	return diags
}

func resourceLoadBalancerUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	oldName := d.Id()
	project := d.Get("project").(string)
	newName := d.Get("name").(string)

	updateReq := &LoadBalancerUpdateRequest{
		Name: newName, // The API requires name
	}

	if d.HasChange("port") {
		newPort := d.Get("port").(int)
		updateReq.Port = &newPort
	}
	if d.HasChange("networks") {
		nets := expandStringList(d.Get("networks").([]interface{}))
		updateReq.Networks = &nets
	}
	if d.HasChange("sub_networks") {
		subs := expandStringList(d.Get("sub_networks").([]interface{}))
		updateReq.SubNetworks = &subs
	}
	if d.HasChange("key_name") {
		newKeyName := d.Get("key_name").(string)
		updateReq.KeyName = &newKeyName
	}
	if d.HasChange("request_floating_ip") {
		rfi := d.Get("request_floating_ip").(bool)
		updateReq.RequestFloatingIP = &rfi
	}
	if d.HasChange("ssl_enabled") {
		newSSL := d.Get("ssl_enabled").(bool)
		updateReq.SSLEnabled = &newSSL
	}
	if d.HasChange("servers") {
		newServers := expandServerItems(d.Get("servers").([]interface{}))
		updateReq.Servers = &newServers
	}
	if d.HasChange("security_groups") {
		newSGs := expandStringList(d.Get("security_groups").([]interface{}))
		updateReq.SecurityGroups = &newSGs
	}

	bodyBytes, _ := json.Marshal(updateReq)
	path := fmt.Sprintf("/loadbalancers/%s?project_name=%s", url.PathEscape(oldName), url.PathEscape(project))
	req, err := c.newRequest("PUT", path)
	if err != nil {
		return diag.FromErr(err)
	}
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return diag.Errorf("Failed to update load balancer: %s - %s", resp.Status, string(body))
	}

	// If the name changed, update the ID
	d.SetId(newName)

	return diags
}

func resourceLoadBalancerDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	name := d.Id()
	project := d.Get("project").(string)
	path := fmt.Sprintf("/loadbalancers/%s?project_name=%s", url.PathEscape(name), url.PathEscape(project))

	req, err := c.newRequest("DELETE", path)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return diag.Errorf("Failed to delete load balancer: %s - %s", resp.Status, string(body))
	}

	d.SetId("")
	return diags
}

// expandServerItems converts a []interface{} -> []ServerItem
func expandServerItems(list []interface{}) []ServerItem {
	servers := make([]ServerItem, 0, len(list))
	for _, v := range list {
		serverMap := v.(map[string]interface{})
		servers = append(servers, ServerItem{
			IP:       serverMap["ip"].(string),
			Port:     serverMap["port"].(int),
			Endpoint: serverMap["endpoint"].(string),
		})
	}
	return servers
}
