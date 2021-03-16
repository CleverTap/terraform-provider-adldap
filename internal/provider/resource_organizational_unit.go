package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceOrganizationalUnit() *schema.Resource {
	return &schema.Resource{
		Description: "Creates and destroys LDAP organizational units.",

		CreateContext: resourceOrganizationalUnitCreate,
		ReadContext:   resourceOrganizationalUnitRead,
		UpdateContext: resourceOrganizationalUnitUpdate,
		DeleteContext: resourceOrganizationalUnitDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"distinguished_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"create_parents": {
				Type:     schema.TypeBool,
				Default:  false,
				Optional: true,
			},
		},
	}
}

func resourceOrganizationalUnitCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(*LdapClient)

	dn := d.Get("distinguished_name").(string)
	createParents := d.Get("create_parents").(bool)

	if createParents {
		_, err := client.CreateOUAndParents(dn)
		if err != nil {
			return diag.FromErr(err)
		}
	} else {
		_, err := client.CreateOU(dn)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(dn)
	d.Set("distinguished_name", dn)

	return diags
}

func resourceOrganizationalUnitRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(*LdapClient)

	dn := d.Id()
	exists, err := client.ObjectExists(dn, "organizationalUnit")
	if err != nil {
		return diag.FromErr(err)
	}

	if exists {
		d.SetId(dn)
	} else {
		return diag.Errorf("unable to import non-existent organizational unit \"%s\"", dn)
	}

	return diags
}

func resourceOrganizationalUnitUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(*LdapClient)
	dn := d.Id()

	if d.HasChange("distinguished_name") {
		newDN := d.Get("distinguished_name").(string)

		ou, err := client.GetOU(dn)
		if err != nil {
			return diag.FromErr(err)
		}

		ou.Rename(newDN)

		d.SetId(newDN)
	}

	return diags
}

func resourceOrganizationalUnitDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(*LdapClient)
	ouDN := d.Get("distinguished_name").(string)

	ou, err := client.GetOU(ouDN)
	if err != nil {
		return diag.FromErr(err)
	}

	err = ou.Delete()
	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}
