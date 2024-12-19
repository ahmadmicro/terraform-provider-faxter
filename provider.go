package main

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"token": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				DefaultFunc: schema.EnvDefaultFunc("FAXTER_TOKEN", nil),
				Description: "The bearer token used for API authentication.",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"faxter_project":        resourceProject(),
			"faxter_server":         resourceServer(),
			"faxter_ssh_key":        resourceSSHKey(),
			"faxter_network":        resourceNetwork(),
			"faxter_router":         resourceRouter(),
			"faxter_volume":         resourceVolume(),
			"faxter_security_group": resourceSecurityGroup(),
		},
		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Hardcode the base URL here
	baseURL := "https://api.faxter.com"
	token := d.Get("token").(string)

	client := NewClient(baseURL, token)

	return client, diags
}
