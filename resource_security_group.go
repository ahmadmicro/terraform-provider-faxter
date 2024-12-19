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

type SecurityGroupRuleRequest struct {
  Protocol         string `json:"protocol,omitempty"`
  PortRangeMin     int    `json:"port_range_min,omitempty"`
  PortRangeMax     int    `json:"port_range_max,omitempty"`
  Direction        string `json:"direction,omitempty"`
  RemoteIpPrefix   string `json:"remote_ip_prefix,omitempty"`
  RemoteGroupId    string `json:"remote_group_id,omitempty"`
  EtherType        string `json:"ether_type,omitempty"`
}

type SecurityGroupCreateRequest struct {
  Project string                    `json:"project,omitempty"`
  Name    string                    `json:"name"`
  Rules   []SecurityGroupRuleRequest `json:"rules"`
}

func resourceSecurityGroup() *schema.Resource {
  return &schema.Resource{
    CreateContext: resourceSecurityGroupCreate,
    ReadContext:   resourceSecurityGroupRead,
    UpdateContext: resourceSecurityGroupUpdate,
    DeleteContext: resourceSecurityGroupDelete,

    Schema: map[string]*schema.Schema{
      "project": {
        Type:     schema.TypeString,
        Required: true,
      },
      "name": {
        Type:     schema.TypeString,
        Required: true,
      },
      "rules": {
        Type:     schema.TypeList,
        Optional: true,
        Elem: &schema.Resource{
          Schema: map[string]*schema.Schema{
            "protocol": {
              Type:     schema.TypeString,
              Optional: true,
              Default:  "tcp",
            },
            "port_range_min": {
              Type:     schema.TypeInt,
              Optional: true,
            },
            "port_range_max": {
              Type:     schema.TypeInt,
              Optional: true,
            },
            "direction": {
              Type:     schema.TypeString,
              Optional: true,
              Default:  "ingress",
            },
            "remote_ip_prefix": {
              Type:     schema.TypeString,
              Optional: true,
              Default:  "0.0.0.0/0",
            },
            "remote_group_id": {
              Type:     schema.TypeString,
              Optional: true,
            },
            "ether_type": {
              Type:     schema.TypeString,
              Optional: true,
              Default:  "IPv4",
            },
          },
        },
      },
    },
  }
}

func resourceSecurityGroupCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
  c := m.(*Client)
  var diags diag.Diagnostics

  project := d.Get("project").(string)
  name := d.Get("name").(string)

  rules := d.Get("rules").([]interface{})
  var sgRules []SecurityGroupRuleRequest
  for _, r := range rules {
    ruleMap := r.(map[string]interface{})
    sgRules = append(sgRules, SecurityGroupRuleRequest{
      Protocol:       ruleMap["protocol"].(string),
      PortRangeMin:   ruleMap["port_range_min"].(int),
      PortRangeMax:   ruleMap["port_range_max"].(int),
      Direction:      ruleMap["direction"].(string),
      RemoteIpPrefix: ruleMap["remote_ip_prefix"].(string),
      RemoteGroupId:  ruleMap["remote_group_id"].(string),
      EtherType:      ruleMap["ether_type"].(string),
    })
  }

  reqData := &SecurityGroupCreateRequest{
    Project: project,
    Name:    name,
    Rules:   sgRules,
  }

  bodyBytes, _ := json.Marshal(reqData)
  req, err := c.newRequest("POST", "/security_groups/")
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
    return diag.Errorf("Failed to create security group: %s", resp.Status)
  }

  var resourceResp ResourceResponse
  err = json.NewDecoder(resp.Body).Decode(&resourceResp)
  if err != nil {
    return diag.FromErr(err)
  }

  d.SetId(resourceResp.Name)
  return diags
}

func resourceSecurityGroupRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
  c := m.(*Client)
  var diags diag.Diagnostics

  name := d.Id()
  project := d.Get("project").(string)
  path := fmt.Sprintf("/security_groups/%s?project_name=%s", name, url.QueryEscape(project))
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
    return diag.Errorf("Failed to read security group: %s", resp.Status)
  }

  // If needed, parse response to update fields
  return diags
}

func resourceSecurityGroupUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
  c := m.(*Client)
  var diags diag.Diagnostics

  oldName := d.Id()
  project := d.Get("project").(string)
  newName := d.Get("name").(string)

  rules := d.Get("rules").([]interface{})
  var sgRules []SecurityGroupRuleRequest
  for _, r := range rules {
    ruleMap := r.(map[string]interface{})
    sgRules = append(sgRules, SecurityGroupRuleRequest{
      Protocol:       ruleMap["protocol"].(string),
      PortRangeMin:   ruleMap["port_range_min"].(int),
      PortRangeMax:   ruleMap["port_range_max"].(int),
      Direction:      ruleMap["direction"].(string),
      RemoteIpPrefix: ruleMap["remote_ip_prefix"].(string),
      RemoteGroupId:  ruleMap["remote_group_id"].(string),
      EtherType:      ruleMap["ether_type"].(string),
    })
  }

  updateBody := &SecurityGroupCreateRequest{
    Project: project,
    Name:    newName,
    Rules:   sgRules,
  }

  bodyBytes, _ := json.Marshal(updateBody)
  path := fmt.Sprintf("/security_groups/%s?project_name=%s", oldName, url.QueryEscape(project))
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
    return diag.Errorf("Failed to update security group: %s", resp.Status)
  }

  d.SetId(newName)
  return diags
}

func resourceSecurityGroupDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
  c := m.(*Client)
  var diags diag.Diagnostics

  name := d.Id()
  project := d.Get("project").(string)
  path := fmt.Sprintf("/security_groups/%s?project_name=%s", name, url.QueryEscape(project))
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
    return diag.Errorf("Failed to delete security group: %s", resp.Status)
  }

  d.SetId("")
  return diags
}