package provider

import (
	"context"
	"strings"

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
			"organizational_unit": {
				Description: "The OU that the user should be in.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"display_name": {
				Description: "Full name of the user object.  Defaults to the `samaccountname` of the resource.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"email_address": {
				Description: "User's Email Address",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"enabled": {
				Description: "Whether the account is enabled.  Defaults to `true`.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
			"dont_expire_password": {
				Description: "Whether the account's password expires according to directory settings.  Defaults to `false`.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
			"sam_account_name": {
				Description: "The SAMAccountName of the user.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"user_principal_name": {
				Description: "The user principal name of the user.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"service_principal_names": {
				Description: "A list of the service principal names for the user.",
				Type:        schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
			},
			"password": {
				Description: "The password for the user.",
				Type:        schema.TypeString,
				Sensitive:   true,
				Optional:    true,
			},
			"description": {
				Description: "Description property of the user.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"given_name": {
				Description: "User's given name.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"surname": {
				Description: "User's last name or surname.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"initials": {
				Description:  "Initials that represent part of a user's name. Maximum 6 char.",
				Type:         schema.TypeString,
				Optional:     true,
			},
		},
	}
}

func resourceUserCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*LdapClient)

	attributesMap := make(map[string][]string)

	sAMAccountName := d.Get("sam_account_name").(string)
	
	userPrincipalName := d.Get("user_principal_name").(string)
	if userPrincipalName != "" {
		attributesMap["userPrincipalName"] = []string{userPrincipalName}
	}

	servicePrincipalName := setToStingArray(d.Get("service_principal_names").(*schema.Set))
	if len(servicePrincipalName) > 0 {
		attributesMap["servicePrincipalName"] = servicePrincipalName
	}

	distinguishedName := d.Get("organizational_unit").(string)
	password := d.Get("password").(string)
	description := d.Get("description").(string)
	if description != "" {
		attributesMap["description"] = []string{description}
	}

	enabled := d.Get("enabled").(bool)
	dontExpirePassword := d.Get("dont_expire_password").(bool)

	if d.Get("display_name") == "" {
		d.Set("display_name", sAMAccountName)
	}
	displayName := d.Get("display_name").(string)
	attributesMap["displayName"] = []string{displayName}
	
	mail := d.Get("email_address").(string)
	if mail != "" {
		attributesMap["mail"] = []string{mail}
	}
	
	givenName := d.Get("given_name").(string)
	if givenName != "" {
		attributesMap["givenName"] = []string{givenName}
	}
	
	sn := d.Get("surname").(string)
	if sn != "" {
		attributesMap["sn"] = []string{sn}
	}
	
	initials := d.Get("initials").(string)
	if initials != "" {
		attributesMap["initials"] = []string{initials}
	}

	account, err := client.CreateUserAccount(sAMAccountName, password, distinguishedName, attributesMap)
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
	requestedAttributes := []string{"displayName", "givenName", "sn", "mail", "initials"}

	// Use the samAccountName as the resource ID
	account, err := client.GetAccountBySAMAccountName(d.Id(), requestedAttributes)
	if err != nil {
		if strings.Contains(err.Error(), "no entry returned") {
			d.SetId("")	
			return nil		
		}
		return diag.FromErr(err)
	}

	distinguishedName := account.ParentDN()
	givenName, _ := account.GetAttributeValue("givenName")
	sn, _ := account.GetAttributeValue("sn")
	initials, _ := account.GetAttributeValue("initials")
	mail, _ := account.GetAttributeValue("mail")
	displayName, _ := account.GetAttributeValue("displayName")
	userPrincipalName, _ := account.GetAttributeValue("userPrincipalName")
	servicePrincipalName, _ := account.GetAttributeValues("servicePrincipalName")
	description, _ := account.GetAttributeValue("description")
	dontExpirePassword, err := account.UACFlagIsSet(DONT_EXPIRE_PASSWORD)
	if err != nil {
		return diag.FromErr(err)
	}

	accountEnabled, err := account.IsEnabled()
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("sam_account_name", d.Id())
	d.Set("organizational_unit", distinguishedName)
	d.Set("display_name", displayName)
	d.Set("user_principal_name", userPrincipalName)
	d.Set("service_principal_names", servicePrincipalName)
	d.Set("description", description)
	d.Set("dont_expire_password", dontExpirePassword)
	d.Set("enabled", accountEnabled)
	d.Set("given_name", givenName)
	d.Set("surname", sn)
	d.Set("initials", initials)
	d.Set("email_address", mail)

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

	if d.HasChange("display_name") {
		_, newName := d.GetChange("display_name")
		err = account.UpdateAttribute("displayName", []string{newName.(string)})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("given_name") {
		_, newName := d.GetChange("given_name")
		err = account.UpdateAttribute("givenName", []string{newName.(string)})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("surname") {
		_, newName := d.GetChange("surname")
		err = account.UpdateAttribute("sn", []string{newName.(string)})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("initials") {
		_, newInitial := d.GetChange("initials")
		err = account.UpdateAttribute("initials", []string{newInitial.(string)})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("email_address") {
		_, newMail := d.GetChange("email_address")
		err = account.UpdateAttribute("mail", []string{newMail.(string)})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("user_principal_name") {
		_, newUPN := d.GetChange("user_principal_name")
		err = account.UpdateAttribute("userPrincipalName", []string{newUPN.(string)})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("service_principal_names") {
		_, newSPNs := d.GetChange("service_principal_names")
		err = account.UpdateAttribute("servicePrincipalName", setToStingArray(newSPNs.(*schema.Set)))
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

	if d.HasChange("password") && d.Get("password").(string)!="" {
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
	sAMAccountName := d.Id()

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
