package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceComputer() *schema.Resource {
	return &schema.Resource{
		// This description is used by the documentation generator and the language server.
		Description: "Computer object in Active Directory.",

		CreateContext: resourceComputerCreate,
		ReadContext:   resourceComputerRead,
		UpdateContext: resourceComputerUpdate,
		DeleteContext: resourceComputerDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "SamAccountName of the computer object, with trailing \"$\".",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"organizational_unit": {
				Description: "The OU that the computer should be in.",
				Type:        schema.TypeString,
				Required:    true,
			},
		},
	}
}

func resourceComputerCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*LdapClient)

	name := d.Get("name").(string)
	ou := d.Get("organizational_unit").(string)

	d.SetId(name)

	_, err := client.CreateComputerAccount(name, ou, nil)
	if err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceComputerRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*LdapClient)
	attributes := []string{}

	// Use the samAccountName as the resource ID
	account, err := client.GetAccountBySAMAccountName(d.Id(), attributes)
	if err != nil {
		return diag.FromErr(err)
	}

	parent, err := getParentObject(account.ldapEntry.entry.DN)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("name", d.Id())
	d.Set("organizational_unit", parent)

	return nil
}

func resourceComputerUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*LdapClient)
	name := d.Id()
	if d.HasChange("organizational_unit") {
		newOU := d.Get("organizational_unit").(string)

		account, err := client.GetAccountBySAMAccountName(name, nil)
		if err != nil {
			return diag.FromErr(err)
		}

		err = account.Move(newOU)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceComputerDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*LdapClient)
	samaccountname := d.Get("name").(string)

	account, err := client.GetAccountBySAMAccountName(samaccountname, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	err = account.Delete()
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}
