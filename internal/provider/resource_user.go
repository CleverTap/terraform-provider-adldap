package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const DONT_EXPIRE_PASSWORD = 65536

func resourceUser() *schema.Resource {
	return &schema.Resource{
		Description: "`adldap_user` manages a user account in Active Directory.",

		CreateContext: resourceUserCreate,
		ReadContext:   resourceUserRead,
		UpdateContext: resourceUserUpdate,
		DeleteContext: resourceUserDelete,
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
				Description: "The SAMAccountName of the user.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"upn": {
				Description: "The user principal name of the user.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"spns": {
				Description: "A list of the service principal names for the user.",
				Type:        schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
			},
			"password": {
				Description: "The password for the user.",
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
			"name": {
				Description: "Full name of the user object.  Defaults to the `samaccountname` of the resource.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"description": {
				Description: "Description property of the user.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"enabled": {
				Description: "Whether the account is enabled.  Defaults to `true`.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
			},
			"dont_expire_password": {
				Description: "Whether the account's password expires according to directory settings.  Defaults to `false`.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
		},
	}
}

func resourceUserCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*LdapClient)

	sAMAccountName := d.Get("samaccountname").(string)
	upn := d.Get("upn").(string)
	ou := d.Get("organizational_unit").(string)
	password := d.Get("password").(string)
	description := d.Get("description").(string)
	enabled := true
	dontExpirePassword := d.Get("dont_expire_password").(bool)
	attributesMap := make(map[string][]string)

	spnsRaw := d.Get("spns").([]interface{})
	spns := make([]string, len(spnsRaw))
	for i, elem := range spnsRaw {
		spns[i] = elem.(string)
	}

	if d.Get("name") == "" {
		d.Set("name", sAMAccountName)
	}
	name := d.Get("name").(string)
	attributesMap["name"] = []string{name}

	if description != "" {
		attributesMap["description"] = []string{description}
	}

	if upn != "" {
		attributesMap["userPrincipalName"] = []string{upn}
	}

	if len(spns) > 0 {
		attributesMap["servicePrincipalName"] = spns
	}

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

	if dontExpirePassword {
		err = account.AddUACFlag(DONT_EXPIRE_PASSWORD)
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

	ou := account.ParentDN()

	name, _ := account.GetAttributeValue("name")
	upn, _ := account.GetAttributeValue("userPrincipalName")
	spns, _ := account.GetAttributeValues("servicePrincipalName")
	description, _ := account.GetAttributeValue("description")
	dontExpirePassword, err := account.UACFlagIsSet(DONT_EXPIRE_PASSWORD)
	if err != nil {
		return diag.FromErr(err)
	}

	accountEnabled, err := account.IsEnabled()
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("samaccountname", d.Id())
	d.Set("organizational_unit", ou)
	d.Set("name", name)
	d.Set("upn", upn)
	d.Set("spns", spns)
	d.Set("description", description)
	d.Set("dont_expire_password", dontExpirePassword)
	d.Set("enabled", accountEnabled)

	return nil
}

func resourceUserUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var err error

	client := meta.(*LdapClient)
	sAMAccountName := d.Id()

	account, err := client.GetAccountBySAMAccountName(sAMAccountName, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	if d.HasChange("organizational_unit") {
		_, newOU := d.GetChange("organizational_unit")

		err = account.Move(newOU.(string))
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("name") {
		_, newName := d.GetChange("name")
		err = account.Rename(newName.(string))
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("upn") {
		_, newUPN := d.GetChange("upn")
		err = account.UpdateAttribute("userPrincipalName", []string{newUPN.(string)})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("spns") {
		_, newSPNs := d.GetChange("spns")
		err = account.UpdateAttribute("servicePrincipalName", newSPNs.([]string))
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("description") {
		_, newDescription := d.GetChange("description")
		err = account.UpdateAttribute("description", []string{newDescription.(string)})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("password") {
		_, newPassword := d.GetChange("password")
		err = account.SetPassword(newPassword.(string))
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("enabled") {
		_, newEnabledState := d.GetChange("enabled")
		if newEnabledState.(bool) {
			err = account.Enable()
		} else {
			err = account.Disable()
		}
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("dont_expire_password") {
		_, newDontExpirePassword := d.GetChange("dont_expire_password")
		if newDontExpirePassword.(bool) {
			err = account.AddUACFlag(DONT_EXPIRE_PASSWORD)
		} else {
			err = account.RemoveUACFlag(DONT_EXPIRE_PASSWORD)
		}
		if err != nil {
			return diag.FromErr(err)
		}
	}

	// Change samaccountname last to avoid having to refresh the object
	if d.HasChange("samaccountname") {
		_, newSAMAccountName := d.GetChange("samaccountname")
		account.UpdateAttribute("sAMAccountName", []string{newSAMAccountName.(string)})

		d.SetId(newSAMAccountName.(string))
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
