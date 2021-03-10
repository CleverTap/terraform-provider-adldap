package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var ouRegexp = regexp.MustCompile(`(?i)^OU=([^,]+).*$`)

func resourceOrganizationalUnit() *schema.Resource {
	return &schema.Resource{
		Description: "Creates and destroys LDAP organizational units.",

		CreateContext: resourceOrganizationalUnitCreate,
		ReadContext:   resourceOrganizationalUnitRead,
		// UpdateContext: resourceOrganizationalUnitUpdate,
		DeleteContext: resourceOrganizationalUnitDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"distinguished_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceOrganizationalUnitCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(Meta).client
	searchBase := meta.(Meta).searchBase
	dn := d.Get("distinguished_name").(string)

	err := createOU(client, searchBase, dn)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(dn)
	d.Set("distinguished_name", dn)

	return diags
}

func resourceOrganizationalUnitRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(Meta).client
	searchBase := meta.(Meta).searchBase

	dn := d.Id()
	exists, err := ouExists(client, searchBase, dn)
	if err != nil {
		return diag.FromErr(err)
	}

	if exists {
		d.SetId(dn)
	} else {
		return diag.Errorf("OU \"%s\" does not exist.  Unable to import.", dn)
	}

	return diags
}

func resourceOrganizationalUnitDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(Meta).client
	searchBase := meta.(Meta).searchBase
	ou := d.Get("distinguished_name").(string)

	err := deleteOU(client, searchBase, ou)
	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

func ouExists(client *ldap.Conn, searchBase string, ou string) (bool, error) {
	filter := fmt.Sprintf("(&(objectClass=organizationalUnit)(distinguishedName=%s))", ou)
	requestedAttributes := []string{"distinguishedName"}

	result, err := ldapSearch(client, searchBase, filter, requestedAttributes)
	if err != nil {
		return false, err
	}
	if len(result.Entries) == 1 {
		return true, nil
	}
	return false, nil
}

func createOU(client *ldap.Conn, searchBase string, ou string) error {
	exists, err := ouExists(client, searchBase, ou)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("OU already exists")
	}

	match := ouRegexp.FindStringSubmatch(ou)[1]

	request := ldap.NewAddRequest(ou, nil)
	request.Attribute("objectClass", []string{"organizationalUnit"})
	request.Attribute("ou", []string{match})
	err = client.Add(request)

	return err
}

func deleteOU(client *ldap.Conn, searchBase string, ou string) error {
	exists, err := ouExists(client, searchBase, ou)
	if err != nil {
		return err
	}
	if exists {
		request := ldap.NewDelRequest(ou, nil)
		err := client.Del(request)

		return err
	}
	return nil
}
