package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceOrganizationalUnit() *schema.Resource {
	return &schema.Resource{
		Description: "`adldap_organizational_unit` manages an OU in Active Directory.",

		CreateContext: resourceOrganizationalUnitCreate,
		ReadContext:   resourceOrganizationalUnitRead,
		UpdateContext: resourceOrganizationalUnitUpdate,
		DeleteContext: resourceOrganizationalUnitDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceOrganizationalUnitImport,
		},

		Schema: map[string]*schema.Schema{
			"id": {
				Description: "The ID (DN) of the organizational unit.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"distinguished_name": {
				Description: "The full distinguished name of the organizational unit.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"create_parents": {
				Description: "Whether to create all required parent OUs. These parent OUs will not be managed or removed automatically unless specified in another resource. Defaults to `false`.",
				Type:        schema.TypeBool,
				Default:     false,
				Optional:    true,
			},
		},
	}
}

func resourceOrganizationalUnitCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(*LdapClient)

	dn := d.Get("distinguished_name").(string)
	createParents :=  d.Get("create_parents").(bool)

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
		d.Set("distinguished_name", dn)
		d.Set("create_parents", false)
	} else {
		d.SetId("")
		return nil
	}

	return diags
}

func resourceOrganizationalUnitUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(*LdapClient)
	dn := d.Id()

	if d.HasChange("distinguished_name") {
		_, newDN := d.GetChange("distinguished_name")

		ou, err := client.GetOU(dn)
		if err != nil {
			return diag.FromErr(err)
		}

		err = ou.Rename(newDN.(string))
		if err != nil {
			return diag.FromErr(err)
		}

		d.SetId(newDN.(string))
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

func resourceOrganizationalUnitImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {

	client := meta.(*LdapClient)

	dn := d.Id()
	exists, err := client.ObjectExists(dn, "organizationalUnit")
	if err != nil {
		return nil, err
	}

	if exists {
		d.SetId(dn)
		d.Set("distinguished_name", dn)
		d.Set("create_parents", false)
	} else {
		return nil, err
	}

	return []*schema.ResourceData{d}, nil
}
