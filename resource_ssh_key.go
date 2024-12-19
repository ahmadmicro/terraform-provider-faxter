package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type SSHKeyCreateRequest struct {
	Project   string `json:"project,omitempty"`
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

type SSHKeyUpdateRequest struct {
	Project   string `json:"project,omitempty"`
	Name      string `json:"name,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
}

func resourceSSHKey() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSSHKeyCreate,
		ReadContext:   resourceSSHKeyRead,
		UpdateContext: resourceSSHKeyUpdate,
		DeleteContext: resourceSSHKeyDelete,

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
			"public_key": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
			},
		},
	}
}

func resourceSSHKeyCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	reqData := &SSHKeyCreateRequest{
		Project:   d.Get("project").(string),
		Name:      d.Get("name").(string),
		PublicKey: d.Get("public_key").(string),
	}

	bodyBytes, _ := json.Marshal(reqData)
	req, err := c.newRequest("POST", "/ssh_keys/")
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
		return diag.Errorf("Failed to create SSH key: %s", resp.Status)
	}

	var resourceResp ResourceResponse
	err = json.NewDecoder(resp.Body).Decode(&resourceResp)
	if err != nil {
		return diag.FromErr(err)
	}

	// Use the 'id' from the resource response as the Terraform ID
	// The endpoint expects an integer key_name as a path param on read/delete.
	d.SetId(resourceResp.Name)

	return diags
}

func resourceSSHKeyRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	id := d.Id()
	path := fmt.Sprintf("/ssh_keys/%s", id)
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
		// Key no longer exists
		d.SetId("")
		return diags
	}

	if resp.StatusCode != http.StatusOK {
		return diag.Errorf("Failed to read SSH key: %s", resp.Status)
	}

	// If needed, parse resource again (not strictly necessary if name doesn't change)
	// Just confirm it still exists.
	return diags
}

func resourceSSHKeyUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	// Since the ssh_key ID is stored as an integer string, retrieve that
	id := d.Id()
	project := d.Get("project").(string)
	name := d.Get("name").(string)
	publicKey := d.Get("public_key").(string)

	reqData := &SSHKeyUpdateRequest{
		Project:   project,
		Name:      name, // If name is editable
		PublicKey: publicKey,
	}

	bodyBytes, _ := json.Marshal(reqData)
	path := fmt.Sprintf("/ssh_keys/%s", id) // Using the ID as key_name as per previous logic
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
		return diag.Errorf("Failed to update ssh key: %s", resp.Status)
	}

	return diags
}

func resourceSSHKeyDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics

	id := d.Id()
	path := fmt.Sprintf("/ssh_keys/%s", id)
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
		return diag.Errorf("Failed to delete SSH key: %s", resp.Status)
	}

	d.SetId("")
	return diags
}
