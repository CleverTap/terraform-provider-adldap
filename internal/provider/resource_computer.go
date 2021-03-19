package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceComputer() *schema.Resource {
	return &schema.Resource{
		// This description is used by the documentation generator and the language server.
		Description: "`adldap_computer` manages a computer account in Active Directory.",

		CreateContext: resourceComputerCreate,
		ReadContext:   resourceComputerRead,
		UpdateContext: resourceComputerUpdate,
		DeleteContext: resourceComputerDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"id": {
				Description: "The ID (SAMAccountName) of the user.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"samaccountname": {
				Description: "The SAMAccountName of the computer object, with trailing \"$\".",
				Type:        schema.TypeString,
				Required:    true,
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

	sAMAccountName := d.Get("samaccountname").(string)
	ou := d.Get("organizational_unit").(string)

	_, err := client.CreateComputerAccount(sAMAccountName, ou, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(sAMAccountName)

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

	ldapDN, err := NewLdapDN(account.DN)
	parent := ldapDN.ParentDN()
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("samaccountname", d.Id())
	d.Set("organizational_unit", parent)

	return nil
}

func resourceComputerUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var account *LdapAccount
	var err error

	client := meta.(*LdapClient)
	sAMAccountName := d.Id()

	if d.HasChanges("organizational_unit", "samaccountname") {
		account, err = client.GetAccountBySAMAccountName(sAMAccountName, nil)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("organizational_unit") {
		_, newOU := d.GetChange("organizational_unit")

		err = account.Move(newOU.(string))
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("samaccountname") {
		_, newSAMAccountName := d.GetChange("samaccountname")
		account.UpdateAttribute("sAMAccountName", []string{newSAMAccountName.(string)})

		d.SetId(newSAMAccountName.(string))
	}

	return nil
}

func resourceComputerDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*LdapClient)
	sAMAccountName := d.Get("samaccountname").(string)

	account, err := client.GetAccountBySAMAccountName(sAMAccountName, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	err = account.Delete()
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}
