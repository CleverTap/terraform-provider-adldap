package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// New returns a terraform.ResourceProvider.
func New() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"url": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("ADLDAP_URL", nil),
				Description: descriptions["url"],
			},
			"bind_account": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("ADLDAP_BIND_ACCOUNT", nil),
				Description: descriptions["bind_account"],
			},
			"bind_password": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				DefaultFunc: schema.EnvDefaultFunc("ADLDAP_BIND_PASSWORD", nil),
				Description: descriptions["bind_password"],
			},
			"search_base": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("ADLDAP_SEARCH_BASE", "UNSET"),
				Description: descriptions["search_base"],
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"adldap_service_principal":   resourceServicePrincipal(),
			"adldap_organizational_unit": resourceOrganizationalUnit(),
			"adldap_computer":            resourceComputer(),
			"adldap_user":                resourceUser(),
		},

		ConfigureContextFunc: providerConfigure,
	}
}

var descriptions map[string]string

func init() {
	descriptions = map[string]string{
		"url":           "The URL of the LDAP server, prefixed with ldap:// or ldaps://",
		"bind_account":  "The full DN or samAccountName used to bind to the directory",
		"bind_password": "The password for the bind account",
		"search_base":   "The base DN to use for all LDAP searches",
	}
}

func providerConfigure(c context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	ldapURL := d.Get("url").(string)
	bindAccount := d.Get("bind_account").(string)
	bindPassword := d.Get("bind_password").(string)
	searchBase := d.Get("search_base").(string)

	conn, searchBase, err := newClient(ldapURL, bindAccount, bindPassword, searchBase)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	meta := Meta{
		client:     conn,
		searchBase: searchBase,
	}

	return meta, nil
}
