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

type VolumeCreateRequest struct {
  Project string `json:"project,omitempty"`
  Name    string `json:"name"`
  Storage    int    `json:"storage"`
}

type VolumeUpdateRequest struct {
  Project string `json:"project,omitempty"`
  Storage    int    `json:"storage"`
}

func resourceVolume() *schema.Resource {
  return &schema.Resource{
    CreateContext: resourceVolumeCreate,
    ReadContext:   resourceVolumeRead,
    UpdateContext: resourceVolumeUpdate,
    DeleteContext: resourceVolumeDelete,

    Schema: map[string]*schema.Schema{
      "project": {
        Type:     schema.TypeString,
        Required: true,
      },
      "name": {
        Type:     schema.TypeString,
        Required: true,
      },
      "storage": {
        Type:     schema.TypeInt,
        Required: true,
      },
    },
  }
}

func resourceVolumeCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
  c := m.(*Client)
  var diags diag.Diagnostics

  reqData := &VolumeCreateRequest{
    Project: d.Get("project").(string),
    Name:    d.Get("name").(string),
    Storage:    d.Get("storage").(int),
  }

  bodyBytes, _ := json.Marshal(reqData)
  req, err := c.newRequest("POST", "/volumes/")
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
    return diag.Errorf("Failed to create volume: %s", resp.Status)
  }

  var resourceResp ResourceResponse
  err = json.NewDecoder(resp.Body).Decode(&resourceResp)
  if err != nil {
    return diag.FromErr(err)
  }

  d.SetId(resourceResp.Name)
  return diags
}

func resourceVolumeRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
  c := m.(*Client)
  var diags diag.Diagnostics

  name := d.Id()
  project := d.Get("project").(string)
  path := fmt.Sprintf("/volumes/%s?project_name=%s", url.PathEscape(name), url.QueryEscape(project))
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
    // Volume not found
    d.SetId("")
    return diags
  }

  if resp.StatusCode != http.StatusOK {
    return diag.Errorf("Failed to read volume: %s", resp.Status)
  }

  // If needed, parse response and set any updated fields in state:
  // var volumeResp ResourceResponse
  // err = json.NewDecoder(resp.Body).Decode(&volumeResp)
  // if err == nil {
  //   // Update any fields if API returns them
  // }

  return diags
}

func resourceVolumeUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
  c := m.(*Client)
  var diags diag.Diagnostics

  name := d.Id()
  project := d.Get("project").(string)

  reqData := &VolumeUpdateRequest{
    Project: project,
    Storage:    d.Get("storage").(int),
  }

  bodyBytes, _ := json.Marshal(reqData)
  path := fmt.Sprintf("/volumes/%s?project_name=%s", url.PathEscape(name), url.QueryEscape(project))
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
    return diag.Errorf("Failed to update volume: %s", resp.Status)
  }

  // If response returns updated info, parse and update state if needed
  return diags
}

func resourceVolumeDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
  c := m.(*Client)
  var diags diag.Diagnostics

  name := d.Id()
  project := d.Get("project").(string)
  path := fmt.Sprintf("/volumes/%s?project_name=%s", url.PathEscape(name), url.QueryEscape(project))
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
    return diag.Errorf("Failed to delete volume: %s", resp.Status)
  }

  d.SetId("")
  return diags
}