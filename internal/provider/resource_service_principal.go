package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const validSpnRegex = "^[\\w\\d]+/(([a-zA-Z]|[a-zA-Z][a-zA-Z0-9\\-]*[a-zA-Z0-9])\\.)*([A-Za-z]|[A-Za-z][A-Za-z0-9\\-]*[A-Za-z0-9])(:\\d{1,5})?$"

func resourceServicePrincipal() *schema.Resource {
	return &schema.Resource{
		// This description is used by the documentation generator and the language server.
		Description: "Manages service principal names associated to samaccountnames.",

		CreateContext: resourceServicePrincipalCreate,
		ReadContext:   resourceServicePrincipalRead,
		DeleteContext: resourceServicePrincipalDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
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

	client := meta.(Meta).client
	spn := d.Get("spn").(string)
	samaccountname := d.Get("samaccountname").(string)
	searchBase := meta.(Meta).searchBase

	err := createSPN(client, searchBase, spn, samaccountname)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%s---%s", spn, samaccountname))
	d.Set("spn", spn)
	d.Set("samaccountname", samaccountname)

	return diags
}

func resourceServicePrincipalRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(Meta).client
	searchBase := meta.(Meta).searchBase

	spnStrings := strings.Split(d.Id(), "---")
	if len(spnStrings) != 2 {
		return diag.Errorf("Resource ID \"%s\" is in the wrong format.  Please import using \"service/host---samaccountname\" format.", d.Id())
	}

	spn := spnStrings[0]
	samaccountname := spnStrings[1]

	if !spnFormatValid(spn) {
		return diag.Errorf("SPN must be of format \"service/host(:port)\".")
	}

	id := fmt.Sprintf("%s---%s", spn, samaccountname)
	exists, err := spnExists(client, searchBase, samaccountname, spn)
	if err != nil {
		return diag.FromErr(err)
	}

	if exists {
		d.SetId(id)
		d.Set("spn", spn)
		d.Set("samaccountname", samaccountname)
	} else {
		return diag.Errorf("SPN \"%s\" does not exist.  Unable to import.", spn)
	}

	return diags
}

func resourceServicePrincipalDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(Meta).client
	spn := d.Get("spn").(string)
	samaccountname := d.Get("samaccountname").(string)
	searchBase := meta.(Meta).searchBase

	err := deleteSPN(client, searchBase, spn, samaccountname)
	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

func spnExists(client *ldap.Conn, searchBase string, samaccountname string, spn string) (bool, error) {
	filter := fmt.Sprintf("(&(objectClass=organizationalPerson)(samAccountName=%s)(servicePrincipalName=%s))", samaccountname, spn)
	requestedAttributes := []string{"servicePrincipalName"}

	result, err := ldapSearch(client, searchBase, filter, requestedAttributes)
	if err != nil {
		return false, err
	}
	if len(result.Entries) == 1 {
		return true, nil
	}
	return false, nil
}

func createSPN(client *ldap.Conn, searchBase string, spn string, samaccountname string) error {
	exists, err := spnExists(client, searchBase, samaccountname, spn)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("SPN already exists")
	}

	dn := getDN(client, searchBase, samaccountname)
	request := ldap.NewModifyRequest(dn, nil)
	request.Add("ServicePrincipalName", []string{spn})

	err = client.Modify(request)

	return err
}

func deleteSPN(client *ldap.Conn, searchBase string, spn string, samaccountname string) error {
	exists, err := spnExists(client, searchBase, samaccountname, spn)
	if err != nil {
		return err
	}
	if exists {
		dn := getDN(client, searchBase, samaccountname)
		request := ldap.NewModifyRequest(dn, nil)
		request.Delete("ServicePrincipalName", []string{spn})

		err := client.Modify(request)

		return err
	}
	return nil
}

func spnFormatValid(spn string) bool {
	valid, err := regexp.MatchString(validSpnRegex, spn)
	if err != nil {
		return false
	}
	return valid
}
