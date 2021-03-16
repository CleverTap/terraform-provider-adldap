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
				Required:    true,
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
			},
		},
	}
}

func resourceUserCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*LdapClient)

	samaccountname := d.Get("samaccountname").(string)
	ou := d.Get("organizational_unit").(string)
	password := d.Get("password").(string)
	enabled := true

	attributesMap := make(map[string][]string)

	var name string
	if d.Get("name") == nil {
		name = samaccountname
	} else {
		name = d.Get("name").(string)
	}

	attributesMap["name"] = []string{name}

	d.SetId(samaccountname)

	err := createUserObject(client, samaccountname, password, ou, enabled, attributesMap)
	if err != nil {
		return diag.FromErr(err)
	}

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

	ou := account.ldapEntry.ParentObject()
	name := account.ldapEntry.entry.GetAttributeValue("name")

	d.Set("samaccountname", d.Id())
	d.Set("organizational_unit", ou)
	d.Set("name", name)
	d.Set("enabled", accountEnabled)

	return nil
}

func resourceUserUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*LdapClient)
	samaccountname := d.Id()

	account, err := client.GetAccountBySAMAccountName(samaccountname, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	if d.HasChange("organizational_unit") {
		newOU := d.Get("organizational_unit").(string)

		err := account.Move(newOU)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	// updatedAttributes := make(map[string][]string)

	if d.HasChange("name") {
		newName := d.Get("name").(string)
		err := account.Rename(newName)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("password") {
		err := account.SetPassword(d.Get("password").(string))
		if err != nil {
			return diag.FromErr(err)
		}
	}

	// if len(updatedAttributes) > 0 {
	// 	updateUserAttributes(client, samaccountname, updatedAttributes)
	// }

	return nil
}

func resourceUserDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*LdapClient)
	samaccountname := d.Get("samaccountname").(string)

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

func createUserObject(client *LdapClient, sAMAccountName string, password string, ou string, enabled bool, attributes map[string][]string) error {
	exists, err := client.AccountExists(sAMAccountName)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("user object \"%s\" already exists", sAMAccountName)
	}

	account, err := client.CreateUserAccount(sAMAccountName, ou, attributes)
	if err != nil {
		return fmt.Errorf("error creating account %s: %s", sAMAccountName, err)
	}

	err = account.SetPassword(password)
	if err != nil {
		return err
	}

	if enabled {
		err = account.Enable()
		if err != nil {
			return err
		}
	}

	return nil
}
