package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Reflecting the schema from the API:
// NetworkCreate requires a "subnets" array of objects {name: string, cidr: string}.

type SubnetCreateRequest struct {
	Name string `json:"name"`
	CIDR string `json:"cidr"`
}

type NetworkCreateRequest struct {
	Project string                `json:"project,omitempty"`
	Name    string                `json:"name"`
	Subnets []SubnetCreateRequest `json:"subnets"`
}

func resourceNetwork() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceNetworkCreate,
		ReadContext:   resourceNetworkRead,
		UpdateContext: resourceNetworkUpdate,
		DeleteContext: resourceNetworkDelete,

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
			"subnets": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"cidr": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
				Description: "A list of subnet configurations for this network.",
			},
		},
	}
}

func resourceNetworkCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	project := d.Get("project").(string)
	name := d.Get("name").(string)
	subnetsIface := d.Get("subnets").([]interface{})

	var subnets []SubnetCreateRequest
	for _, subnetRaw := range subnetsIface {
		subnetMap := subnetRaw.(map[string]interface{})
		subnets = append(subnets, SubnetCreateRequest{
			Name: subnetMap["name"].(string),
			CIDR: subnetMap["cidr"].(string),
		})
	}

	reqData := &NetworkCreateRequest{
		Project: project,
		Name:    name,
		Subnets: subnets,
	}

	bodyBytes, _ := json.Marshal(reqData)
	req, err := c.newRequest("POST", "/networks/")
	if err != nil {
		return diag.FromErr(err)
	}
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return diag.Errorf("Failed to create network: %s", resp.Status)
	}

	var resourceResp ResourceResponse
	err = json.NewDecoder(resp.Body).Decode(&resourceResp)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(resourceResp.Name)
	return diags
}

func resourceNetworkRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	name := d.Id()
	project := d.Get("project").(string)
	path := fmt.Sprintf("/networks/%s?project_name=%s", url.PathEscape(name), url.QueryEscape(project))
	req, err := c.newRequest("GET", path)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		d.SetId("")
		return diags
	}

	if resp.StatusCode != http.StatusOK {
		return diag.Errorf("Failed to read network: %s", resp.Status)
	}

	// If the GET response returns info about subnets, parse them here and update state.
	// The schema suggests it might return ResourceResponse, which may not have detailed subnets info.
	// Without subnet details in the response, we cannot reliably update subnets.
	// Assuming we only confirm existence for now.

	return diags
}

func resourceNetworkUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	oldName := d.Id()
	project := d.Get("project").(string)
	newName := d.Get("name").(string)
	subnetsIface := d.Get("subnets").([]interface{})

	var subnets []SubnetCreateRequest
	for _, subnetRaw := range subnetsIface {
		subnetMap := subnetRaw.(map[string]interface{})
		subnets = append(subnets, SubnetCreateRequest{
			Name: subnetMap["name"].(string),
			CIDR: subnetMap["cidr"].(string),
		})
	}

	updateBody := &NetworkCreateRequest{
		Project: project,
		Name:    newName,
		Subnets: subnets,
	}

	bodyBytes, _ := json.Marshal(updateBody)
	path := fmt.Sprintf("/networks/%s?project_name=%s", url.PathEscape(oldName), url.QueryEscape(project))
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

	if resp.StatusCode != http.StatusOK {
		return diag.Errorf("Failed to update network: %s", resp.Status)
	}

	// If the network name changes are allowed and accepted, update ID.
	d.SetId(newName)
	return diags
}

func resourceNetworkDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	name := d.Id()
	project := d.Get("project").(string)
	path := fmt.Sprintf("/networks/%s?project_name=%s", url.PathEscape(name), url.QueryEscape(project))
	req, err := c.newRequest("DELETE", path)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return diag.Errorf("Failed to delete network: %s", resp.Status)
	}

	d.SetId("")
	return diags
}
