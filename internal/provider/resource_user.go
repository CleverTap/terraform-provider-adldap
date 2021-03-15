package provider

import (
	"context"
	"fmt"
	"strconv"

	uac "github.com/audibleblink/msldapuac"
	"github.com/go-ldap/ldap/v3"
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
	client := meta.(Meta).client
	searchBase := meta.(Meta).searchBase

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

	err := createUserObject(client, searchBase, samaccountname, password, ou, enabled, attributesMap)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceUserRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(Meta).client
	searchBase := meta.(Meta).searchBase
	attributes := []string{"userAccountControl", "name"}

	// Use the samAccountName as the resource ID
	entry, err := getUserObject(client, searchBase, d.Id(), attributes)
	if err != nil {
		return diag.FromErr(err)
	}

	attributeMap := map[string][]string{}
	for _, v := range entry.Attributes {
		attributeMap[v.Name] = v.Values
	}

	uacInt, err := strconv.ParseInt(attributeMap["userAccountControl"][0], 10, 0)
	if err != nil {
		return diag.FromErr(err)
	}

	accountDisabled, err := uac.IsSet(uacInt, uac.Accountdisable)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("samaccountname", d.Id())
	d.Set("organizational_unit", getParentObject(entry.DN))
	d.Set("name", attributeMap["name"][0])
	d.Set("enabled", !accountDisabled)

	return nil
}

func resourceUserUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(Meta).client
	searchBase := meta.(Meta).searchBase
	samaccountname := d.Id()

	if d.HasChange("organizational_unit") {
		newOU := d.Get("organizational_unit").(string)

		err := moveUserObject(client, searchBase, samaccountname, newOU)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	updatedAttributes := make(map[string][]string)

	if d.HasChange("name") {
		newName := d.Get("name").(string)
		err := renameUserObject(client, searchBase, samaccountname, newName)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("password") {
		err := setObjectPassword(client, searchBase, samaccountname, "user", d.Get("password").(string))
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if len(updatedAttributes) > 0 {
		updateUserAttributes(client, searchBase, samaccountname, updatedAttributes)
	}

	return nil
}

func resourceUserDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(Meta).client
	searchBase := meta.(Meta).searchBase
	samaccountname := d.Get("samaccountname").(string)

	err := deleteUserObject(client, searchBase, samaccountname)
	if err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func userExists(client *ldap.Conn, searchBase string, samaccountname string) (bool, error) {
	return objectExists(client, searchBase, samaccountname, "user")
}

func createUserObject(client *ldap.Conn, searchBase string, samaccountname string, password string, ou string, enabled bool, attributes map[string][]string) error {
	exists, err := userExists(client, searchBase, samaccountname)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("user object \"%s\" already exists", samaccountname)
	}

	userDN := fmt.Sprintf("CN=%s,%s", attributes["name"][0], ou)
	initialUAC := uac.NormalAccount | uac.Accountdisable

	request := ldap.NewAddRequest(userDN, nil)
	request.Attribute("objectClass", []string{"user"})
	request.Attribute("samAccountName", []string{samaccountname})
	request.Attribute("userAccountControl", []string{fmt.Sprintf("%d", initialUAC)}) // Create the account in a disabled state
	request.Attribute("accountExpires", []string{fmt.Sprintf("%d", 0x00000000)})

	for k, v := range attributes {
		request.Attribute(k, v)
	}

	err = client.Add(request)
	if err != nil {
		return err
	}

	err = setObjectPassword(client, searchBase, samaccountname, "user", password)
	if err != nil {
		return err
	}

	if enabled {
		err = setUserEnableUAC(client, searchBase, samaccountname, true)
		if err != nil {
			return err
		}
	}

	return nil
}

func getUserObject(client *ldap.Conn, searchBase string, samaccountname string, attributes []string) (*ldap.Entry, error) {
	result, err := getObject(client, searchBase, samaccountname, "user", attributes)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func moveUserObject(client *ldap.Conn, searchBase string, samaccountname string, newOU string) error {
	err := moveObject(client, searchBase, samaccountname, "user", newOU)
	if err != nil {
		return err
	}

	return nil
}

func renameUserObject(client *ldap.Conn, searchBase string, samaccountname string, newName string) error {
	err := renameObject(client, searchBase, samaccountname, "user", newName)
	if err != nil {
		return err
	}

	return nil
}

func deleteUserObject(client *ldap.Conn, searchBase string, samaccountname string) error {
	entry, err := getUserObject(client, searchBase, samaccountname, []string{})
	if err != nil {
		return err
	}

	dn := entry.DN

	err = deleteObject(client, searchBase, dn, "user")
	if err != nil {
		return err
	}

	return nil
}

func updateUserAttributes(client *ldap.Conn, searchBase string, samaccountname string, attributeMap map[string][]string) error {
	err := updateObjectAttributes(client, searchBase, samaccountname, "user", attributeMap)
	if err != nil {
		return err
	}
	return nil
}

func setUserEnableUAC(client *ldap.Conn, searchBase string, samaccountname string, desiredEnabled bool) error {
	entry, err := getUserObject(client, searchBase, samaccountname, []string{"userAccountControl"})
	if err != nil {
		return err
	}

	uacInt, err := strconv.ParseInt(entry.Attributes[0].Values[0], 10, 0)
	if err != nil {
		return err
	}

	currentlyDisabled, err := uac.IsSet(uacInt, uac.Accountdisable)
	if err != nil {
		return err
	}

	if !currentlyDisabled != desiredEnabled {
		var newUAC int64
		attributeMap := make(map[string][]string)

		if currentlyDisabled {
			newUAC = uacInt &^ uac.Accountdisable
		} else {
			newUAC = uacInt | uac.Accountdisable
		}

		attributeMap["userAccountControl"] = []string{fmt.Sprintf("%d", newUAC)}
		err := updateObjectAttributes(client, searchBase, samaccountname, "user", attributeMap)
		if err != nil {
			return err
		}
	}

	return nil
}
