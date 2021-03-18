package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceUser() *schema.Resource {
	return &schema.Resource{
		Description: "Manage a user object in Active Directory.",

		CreateContext: resourceUserCreate,
		ReadContext:   resourceUserRead,
		UpdateContext: resourceUserUpdate,
		DeleteContext: resourceUserDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"samaccountname": {
				Description: "SamAccountName of the user object.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"password": {
				Description: "Password for the user object.",
				Type:        schema.TypeString,
				Sensitive:   true,
				Required:    true,
				ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
					password := val.(string)
					if password == "" {
						errs = append(errs, fmt.Errorf("password must not be empty"))
					}
					return
				},
			},
			"organizational_unit": {
				Description: "The OU that the user should be in.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"enabled": {
				Description: "Whether the account is enabled.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
			},
			"name": {
				Description: "Full name of the user object.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

func resourceUserCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*LdapClient)

	sAMAccountName := d.Get("samaccountname").(string)
	ou := d.Get("organizational_unit").(string)
	password := d.Get("password").(string)
	enabled := true
	attributesMap := make(map[string][]string)

	if d.Get("name") == "" {
		d.Set("name", sAMAccountName)
	}
	name := d.Get("name").(string)

	attributesMap["name"] = []string{name}

	account, err := client.CreateUserAccount(sAMAccountName, password, ou, attributesMap)
	if err != nil {
		return diag.Errorf("error creating account %s: %s", sAMAccountName, err)
	}

	if enabled {
		err = account.Enable()
		if err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(sAMAccountName)

	return nil
}

func resourceUserRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*LdapClient)
	requestedAttributes := []string{"name"}

	// Use the samAccountName as the resource ID
	account, err := client.GetAccountBySAMAccountName(d.Id(), requestedAttributes)
	if err != nil {
		return diag.FromErr(err)
	}

	accountEnabled, err := account.IsEnabled()
	if err != nil {
		return diag.FromErr(err)
	}

	ou := account.ParentDN()
	name, _ := account.GetAttributeValue("name")

	d.Set("samaccountname", d.Id())
	d.Set("organizational_unit", ou)
	d.Set("name", name)
	d.Set("enabled", accountEnabled)

	return nil
}

func resourceUserUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*LdapClient)
	sAMAccountName := d.Id()

	account, err := client.GetAccountBySAMAccountName(sAMAccountName, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	if d.HasChange("organizational_unit") {
		_, newOU := d.GetChange("organizational_unit")

		err := account.Move(newOU.(string))
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("name") {
		_, newName := d.GetChange("name")
		err := account.Rename(newName.(string))
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("password") {
		_, newPassword := d.GetChange("password")
		err := account.SetPassword(newPassword.(string))
		if err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceUserDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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
