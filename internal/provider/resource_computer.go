package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
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
	client := meta.(Meta).client
	searchBase := meta.(Meta).searchBase

	name := d.Get("name").(string)
	ou := d.Get("organizational_unit").(string)

	d.SetId(name)

	err := createComputerObject(client, searchBase, name, ou)
	if err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceComputerRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(Meta).client
	searchBase := meta.(Meta).searchBase
	attributes := []string{}

	// Use the samAccountName as the resource ID
	entry, err := getComputerObject(client, searchBase, d.Id(), attributes)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("name", d.Id())
	d.Set("organizational_unit", getParentObject(entry.DN))

	return nil
}

func resourceComputerUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(Meta).client
	searchBase := meta.(Meta).searchBase
	attributes := []string{}

	newOU := d.Get("organizational_unit").(string)

	// Use the samAccountName as the resource ID
	entry, err := getComputerObject(client, searchBase, d.Id(), attributes)
	if err != nil {
		return diag.FromErr(err)
	}

	dn := entry.DN

	ou := getParentObject(dn)
	if ou != newOU {
		request := ldap.NewModifyDNRequest(dn, getChildObject(dn), true, newOU)
		err := client.ModifyDN(request)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	// use the meta value to retrieve your client from the provider configure method
	// client := meta.(*apiClient)

	return diag.Errorf("not implemented")
}

func resourceComputerDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(Meta).client
	searchBase := meta.(Meta).searchBase
	samaccountname := d.Get("name").(string)

	err := deleteComputerObject(client, searchBase, samaccountname)
	if err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func computerExists(client *ldap.Conn, searchBase string, samaccountname string) (bool, error) {
	return objectExists(client, searchBase, samaccountname, "computer")
}

func createComputerObject(client *ldap.Conn, searchBase string, samaccountname string, ou string) error {
	userAccountControl := 0x1020

	exists, err := computerExists(client, searchBase, samaccountname)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("computer object \"%s\" already exists", samaccountname)
	}

	computerDN := fmt.Sprintf("CN=%s,%s", strings.TrimSuffix(samaccountname, "$"), ou)

	request := ldap.NewAddRequest(computerDN, nil)
	request.Attribute("objectClass", []string{"computer"})
	request.Attribute("samAccountName", []string{samaccountname})
	request.Attribute("userAccountControl", []string{fmt.Sprintf("%d", userAccountControl)})

	err = client.Add(request)

	return err
}

func getComputerObject(client *ldap.Conn, searchBase string, samaccountname string, attributes []string) (*ldap.Entry, error) {
	result, err := getObject(client, searchBase, samaccountname, "computer", attributes)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func deleteComputerObject(client *ldap.Conn, searchBase string, samaccountname string) error {

	entry, err := getComputerObject(client, searchBase, samaccountname, []string{})
	if err != nil {
		return err
	}

	dn := entry.DN

	err = deleteObject(client, searchBase, dn, "computer")
	if err != nil {
		return err
	}

	return nil
}
