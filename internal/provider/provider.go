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
				Description: "The URL of the LDAP server, prefixed with ldap:// or ldaps://. Can be specified with the `ADLDAP_URL` environment variable.",
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("ADLDAP_URL", ""),
			},
			"bind_account": {
				Description: "The full DN or UPN used to bind to the directory. Can be specified with the `ADLDAP_BIND_ACCOUNT` environment variable.",
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("ADLDAP_BIND_ACCOUNT", ""),
			},
			"bind_password": {
				Description: "The password for the bind account. Can be specified with the `ADLDAP_BIND_PASSWORD` environment variable.",
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				DefaultFunc: schema.EnvDefaultFunc("ADLDAP_BIND_PASSWORD", ""),
			},
			"search_base": {
				Description: "The base DN to use for all LDAP searches. Can be specified with the `ADLDAP_SEARCH_BASE` environment variable.  Default is to autodetect default context.",
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("ADLDAP_SEARCH_BASE", ""),
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"adldap_computer":            resourceComputer(),
			"adldap_organizational_unit": resourceOrganizationalUnit(),
			"adldap_service_principal":   resourceServicePrincipal(),
			"adldap_user":                resourceUser(),
		},

		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(c context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	ldapURL := d.Get("url").(string)
	bindAccount := d.Get("bind_account").(string)
	bindPassword := d.Get("bind_password").(string)
	searchBase := d.Get("search_base").(string)

	client := new(LdapClient)

	err := client.New(ldapURL, bindAccount, bindPassword, searchBase, false)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	return client, nil
}

func setToStingArray(set *schema.Set) []string {
	list := set.List()
	arr := make([]string, len(list))

	for i, elem := range list {
		arr[i] = elem.(string)
	}
	return arr
}
