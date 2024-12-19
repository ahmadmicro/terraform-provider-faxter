package main

import (
  "context"
  "encoding/json"
  "fmt"
  "io"
  "bytes"
  "net/http"
  "net/url"

  "github.com/hashicorp/terraform-plugin-sdk/v2/diag"
  "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type RouterCreateRequest struct {
  Project         string   `json:"project,omitempty"`
  Name            string   `json:"name"`
  ConnectExternal bool     `json:"connect_external,omitempty"`
  Subnets         []string `json:"subnets"`
}

func resourceRouter() *schema.Resource {
  return &schema.Resource{
    CreateContext: resourceRouterCreate,
    ReadContext:   resourceRouterRead,
    UpdateContext: resourceRouterUpdate,
    DeleteContext: resourceRouterDelete,

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
      "connect_external": {
        Type:     schema.TypeBool,
        Optional: true,
        Default:  true,
      },
      "subnets": {
        Type:     schema.TypeList,
        Required: true,
        Elem:     &schema.Schema{Type: schema.TypeString},
      },
    },
  }
}

func resourceRouterCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
  c := m.(*Client)
  var diags diag.Diagnostics

  reqData := &RouterCreateRequest{
    Project:         d.Get("project").(string),
    Name:            d.Get("name").(string),
    ConnectExternal: d.Get("connect_external").(bool),
  }

  subnets := d.Get("subnets").([]interface{})
  for _, s := range subnets {
    reqData.Subnets = append(reqData.Subnets, s.(string))
  }

  bodyBytes, _ := json.Marshal(reqData)
  req, err := c.newRequest("POST", "/routers/")
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
    return diag.Errorf("Failed to create router: %s", resp.Status)
  }

  var resourceResp ResourceResponse
  err = json.NewDecoder(resp.Body).Decode(&resourceResp)
  if err != nil {
    return diag.FromErr(err)
  }

  d.SetId(resourceResp.Name)
  return diags
}

func resourceRouterRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
  c := m.(*Client)
  var diags diag.Diagnostics

  name := d.Id()
  project := d.Get("project").(string)
  path := fmt.Sprintf("/routers/%s?project_name=%s", name, url.QueryEscape(project))
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
    return diag.Errorf("Failed to read router: %s", resp.Status)
  }

  // If needed, parse and update fields
  return diags
}

func resourceRouterUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
  c := m.(*Client)
  var diags diag.Diagnostics

  oldName := d.Id()
  project := d.Get("project").(string)
  newName := d.Get("name").(string)
  connectExternal := d.Get("connect_external").(bool)
  subnetsIface := d.Get("subnets").([]interface{})
  var subnets []string
  for _, s := range subnetsIface {
    subnets = append(subnets, s.(string))
  }

  updateBody := &RouterCreateRequest{
    Project:         project,
    Name:            newName,
    ConnectExternal: connectExternal,
    Subnets:         subnets,
  }

  bodyBytes, _ := json.Marshal(updateBody)
  path := fmt.Sprintf("/routers/%s?project_name=%s", oldName, url.QueryEscape(project))
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
    return diag.Errorf("Failed to update router: %s", resp.Status)
  }

  d.SetId(newName)
  return diags
}

func resourceRouterDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
  c := m.(*Client)
  var diags diag.Diagnostics

  name := d.Id()
  project := d.Get("project").(string)
  path := fmt.Sprintf("/routers/%s?project_name=%s", name, url.QueryEscape(project))
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
    return diag.Errorf("Failed to delete router: %s", resp.Status)
  }

  d.SetId("")
  return diags
}