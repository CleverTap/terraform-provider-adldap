package provider

import (
	"context"
	"fmt"
	"log"
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
		DeleteContext: resourceOrganizationalUnitDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"distinguished_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"create_parents": {
				Type:     schema.TypeBool,
				Default:  false,
				Optional: true,
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
	createParents := d.Get("create_parents").(bool)

	err := createOU(client, searchBase, dn, createParents)
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
		return diag.Errorf("unable to import non-existent organizational unit \"%s\"", dn)
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
	return objectExists(client, searchBase, ou, "organizationalUnit")
}

func createOU(client *ldap.Conn, searchBase string, ou string, createParents bool) error {
	searchBaseDN, _ := ldap.ParseDN(searchBase)
	ouDN, _ := ldap.ParseDN(ou)

	if !searchBaseDN.AncestorOf(ouDN) {
		return fmt.Errorf("organizational unit \"%s\" is not an ancestor of search base \"%s\"", ou, searchBase)
	}

	exists, err := ouExists(client, searchBase, ou)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("organizational unit \"%s\" already exists", ou)
	}

	parentOU := getParentObject(ou)

	if parentOU != searchBase {
		match, err := regexp.MatchString(`(?i)^ou=`, ou)
		if err != nil {
			log.Fatal(err)
		}
		if match {
			parentExists, err := ouExists(client, searchBase, parentOU)
			if err != nil {
				log.Fatal(err)
			}
			if !parentExists {
				if createParents {
					err := createOU(client, searchBase, parentOU, true)
					if err != nil {
						return err
					}
				} else {
					return fmt.Errorf("parent for organizational unit \"%s\" does not exist", ou)
				}
			}
		}
	}

	ouCN := ouRegexp.FindStringSubmatch(ou)[1]

	request := ldap.NewAddRequest(ou, nil)
	request.Attribute("objectClass", []string{"organizationalUnit"})
	request.Attribute("ou", []string{ouCN})
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
