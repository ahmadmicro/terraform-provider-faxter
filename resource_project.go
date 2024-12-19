package main

import (
  "context"
  "encoding/json"
  "fmt"
  "io"
  "bytes"
  "github.com/hashicorp/terraform-plugin-sdk/v2/diag"
  "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type ProjectCreateRequest struct {
  Name string `json:"name"`
}

func resourceProject() *schema.Resource {
  return &schema.Resource{
    CreateContext: resourceProjectCreate,
    ReadContext:   resourceProjectRead,
	UpdateContext: resourceProjectUpdate,
    DeleteContext: resourceProjectDelete,

    Schema: map[string]*schema.Schema{
      "name": {
        Type:     schema.TypeString,
        Required: true,
      },
    },
  }
}

func resourceProjectCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
  c := m.(*Client)
  var diags diag.Diagnostics

  name := d.Get("name").(string)

  // Create project
  bodyData := &ProjectCreateRequest{Name: name}
  bodyBytes, _ := json.Marshal(bodyData)
  req, err := c.newRequest("POST", "/projects")
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
    return diag.Errorf("Failed to create project: %s", resp.Status)
  }

  // On success, set the ID to project name (as unique ID)
  d.SetId(name)

  return diags
}

func resourceProjectRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
  c := m.(*Client)
  var diags diag.Diagnostics

  name := d.Id()

  req, err := c.newRequest("GET", fmt.Sprintf("/projects/%s", name))
  if err != nil {
    return diag.FromErr(err)
  }

  resp, err := c.httpClient.Do(req)
  if err != nil {
    return diag.FromErr(err)
  }
  defer resp.Body.Close()

  if resp.StatusCode == 404 {
    // If project not found, remove it from state
    d.SetId("")
    return diags
  }

  if resp.StatusCode != 200 {
    return diag.Errorf("Failed to read project: %s", resp.Status)
  }

  // If needed, parse project response to update state
  // Currently we only store `name`
  // If project exists, ensure `name` matches
  d.Set("name", name)

  return diags
}

func resourceProjectUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*Client)
	var diags diag.Diagnostics
  
	oldName, newName := d.GetChange("name")
	// The API uses the project_name path param. If the name changed, we must handle how the API treats renames.
	// If renaming is allowed by the API, we would:
	// 1) Use the old name in the URL.
	// 2) Send the new name in the PUT request body.
	// If renaming is not allowed, we might have to recreate the resource.
	// Assuming renaming is allowed for demonstration:
  
	projectName := oldName.(string)
	updateBody := &ProjectCreateRequest{
	  Name: newName.(string),
	}
  
	bodyBytes, _ := json.Marshal(updateBody)
	req, err := c.newRequest("PUT", fmt.Sprintf("/projects/%s", projectName))
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
	  return diag.Errorf("Failed to update project: %s", resp.Status)
	}
  
	// If successful, set ID to the new name.
	d.SetId(newName.(string))
  
	return diags
}

func resourceProjectDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
  c := m.(*Client)
  var diags diag.Diagnostics

  name := d.Id()

  req, err := c.newRequest("DELETE", fmt.Sprintf("/projects/%s", name))
  if err != nil {
    return diag.FromErr(err)
  }

  resp, err := c.httpClient.Do(req)
  if err != nil {
    return diag.FromErr(err)
  }
  defer resp.Body.Close()

  if resp.StatusCode != 200 {
    return diag.Errorf("Failed to delete project: %s", resp.Status)
  }

  // Remove from state
  d.SetId("")

  return diags
}