package provider

import (
	"context"
	"fmt"
	"log"

	"github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Meta structure for resource access
type Meta struct {
	client     *ldap.Conn
	searchBase string
}

// New returns a terraform.ResourceProvider.
func New() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"url": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("ADLDAP_URL", nil),
				Description: descriptions["url"],
			},
			"bind_account": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("ADLDAP_BIND_ACCOUNT", nil),
				Description: descriptions["bind_account"],
			},
			"bind_password": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				DefaultFunc: schema.EnvDefaultFunc("ADLDAP_BIND_PASSWORD", nil),
				Description: descriptions["bind_password"],
			},
			"search_base": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("ADLDAP_SEARCH_BASE", "UNSET"),
				Description: descriptions["search_base"],
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"adldap_service_principal":   resourceServicePrincipal(),
			"adldap_organizational_unit": resourceOrganizationalUnit(),
		},

		ConfigureContextFunc: providerConfigure,
	}
}

var descriptions map[string]string

func init() {
	descriptions = map[string]string{
		"url":           "The URL of the LDAP server, prefixed with ldap:// or ldaps://",
		"bind_account":  "The full DN or samAccountName used to bind to the directory",
		"bind_password": "The password for the bind account",
		"search_base":   "The base DN to use for all LDAP searches",
	}
}

func providerConfigure(c context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	ldapURL := d.Get("url").(string)
	bindAccount := d.Get("bind_account").(string)
	bindPassword := d.Get("bind_password").(string)
	searchBase := d.Get("search_base").(string)

	if ldapURL == "" {
		return nil, diag.Errorf("No url provided")
	}

	conn, err := dialLdap(ldapURL)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	err = bindLdap(conn, bindAccount, bindPassword)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	if searchBase == "UNSET" {
		newSearchBase, err := detectSearchBase(conn)
		if err != nil {
			log.Fatalf("ADLDAP_SEARCH_BASE not set and LDAP search base auto-detection failed.")
		}
		searchBase = newSearchBase
	}

	meta := Meta{
		client:     conn,
		searchBase: searchBase,
	}

	return meta, nil
}

func dialLdap(url string) (*ldap.Conn, error) {
	conn, err := ldap.DialURL(url)
	return conn, err
}

func bindLdap(client *ldap.Conn, bindAccount string, bindPassword string) error {
	err := client.Bind(bindAccount, bindPassword)
	return err
}

func ldapSearch(client *ldap.Conn, searchBase string, filter string, attributes []string) (*ldap.SearchResult, error) {
	searchRequest := ldap.NewSearchRequest(
		searchBase, // The base dn to search
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,     // The filter to apply
		attributes, // A list attributes to retrieve
		nil,
	)

	result, err := client.Search(searchRequest)

	// TODO handle errors other than "not found", etc.

	return result, err
}

func getDN(client *ldap.Conn, searchBase string, samAccountName string) string {
	filter := fmt.Sprintf("(&(objectClass=organizationalPerson)(samAccountName=%s))", samAccountName)
	requestedAttributes := []string{"dn"}

	result, err := ldapSearch(client, searchBase, filter, requestedAttributes)

	if err != nil {
		log.Fatal(err)
	}
	if len(result.Entries) > 1 {
		log.Fatalf("More than one DN returned for samAccountName %s.", samAccountName)
	}
	if len(result.Entries) == 0 {
		log.Fatalf("No entry found for samAccountName %s.", samAccountName)
	}

	dn := result.Entries[0].DN
	return dn
}

func detectSearchBase(client *ldap.Conn) (string, error) {
	searchRequest := ldap.NewSearchRequest(
		"", // The base dn to search
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)",
		[]string{"defaultNamingContext"},
		nil,
	)

	result, err := client.Search(searchRequest)
	if err != nil {
		return "", err
	}

	searchBase := result.Entries[0].GetAttributeValue("defaultNamingContext")

	return searchBase, nil
}
