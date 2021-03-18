package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceServicePrincipal() *schema.Resource {
	return &schema.Resource{
		// This description is used by the documentation generator and the language server.
		Description: "Manages service principal names associated to samaccountnames.",

		CreateContext: resourceServicePrincipalCreate,
		ReadContext:   resourceServicePrincipalRead,
		DeleteContext: resourceServicePrincipalDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"samaccountname": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"spn": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceServicePrincipalCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(*LdapClient)
	spn := d.Get("spn").(string)
	sAMAccountName := d.Get("samaccountname").(string)

	account, err := client.GetAccountBySAMAccountName(sAMAccountName, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	err = account.AddServicePrincipal(spn)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%s---%s", spn, sAMAccountName))
	d.Set("spn", spn)
	d.Set("samaccountname", sAMAccountName)

	return diags
}

func resourceServicePrincipalRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(*LdapClient)

	spnStrings := strings.Split(d.Id(), "---")
	if len(spnStrings) != 2 {
		return diag.Errorf("Resource ID \"%s\" is in the wrong format.  Please import using \"service/host---samaccountname\" format.", d.Id())
	}

	spn := spnStrings[0]
	sAMAccountName := spnStrings[1]

	id := fmt.Sprintf("%s---%s", spn, sAMAccountName)

	account, err := client.GetAccountBySAMAccountName(sAMAccountName, []string{"servicePrincipalName"})
	if err != nil {
		return diag.FromErr(err)
	}

	exists, err := account.HasServicePrincipal(spn)
	if err != nil {
		return diag.FromErr(err)
	}

	if exists {
		d.SetId(id)
		d.Set("spn", spn)
		d.Set("samaccountname", sAMAccountName)
	} else {
		return diag.Errorf("SPN \"%s\" does not exist.  Unable to import.", spn)
	}

	return diags
}

func resourceServicePrincipalDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(*LdapClient)
	spn := d.Get("spn").(string)
	sAMAccountName := d.Get("samaccountname").(string)

	account, err := client.GetAccountBySAMAccountName(sAMAccountName, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	err = account.RemoveServicePrincipal(spn)
	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}
