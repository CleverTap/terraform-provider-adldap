package provider

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

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
			"adldap_computer":            resourceComputer(),
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

func getDN(client *ldap.Conn, searchBase string, samAccountName string) (string, error) {
	result, err := getObject(client, searchBase, samAccountName, "*", []string{})
	if err != nil {
		return "", err
	}

	dn := result.DN
	return dn, nil
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

func objectExists(client *ldap.Conn, searchBase string, objectName string, objectClass string) (bool, error) {
	searchAttribute := "samAccountName"
	if match, _ := regexp.MatchString(`.+=.+`, objectName); match {
		searchAttribute = "distinguishedName"
	}

	filter := fmt.Sprintf("(&(objectClass=%s)(%s=%s))", objectClass, searchAttribute, objectName)

	result, err := ldapSearch(client, searchBase, filter, []string{})
	if err != nil {
		return false, err
	}
	if len(result.Entries) == 1 {
		return true, nil
	}
	if len(result.Entries) > 1 {
		return false, fmt.Errorf("too many results (%d) returned for %s object \"%s\", expected 1", len(result.Entries), objectClass, objectName)
	}
	return false, nil
}

func getObject(client *ldap.Conn, searchBase string, objectName string, objectClass string, attributes []string) (*ldap.Entry, error) {
	searchAttribute := "samAccountName"
	if match, _ := regexp.MatchString(`.+=.+`, objectName); match {
		searchAttribute = "distinguishedName"
	}

	filter := fmt.Sprintf("(&(objectClass=%s)(%s=%s))", objectClass, searchAttribute, objectName)

	result, err := ldapSearch(client, searchBase, filter, attributes)
	if err != nil {
		return nil, err
	}
	if len(result.Entries) > 1 {
		return nil, fmt.Errorf("too many results (%d) returned for %s object \"%s\", expected 1", len(result.Entries), objectClass, objectName)
	}
	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("no entry returned for %s object \"%s\"", objectClass, objectName)
	}
	return result.Entries[0], nil
}

func deleteObject(client *ldap.Conn, searchBase string, distinguishedName string, objectClass string) error {
	exists, err := objectExists(client, searchBase, distinguishedName, objectClass)
	if err != nil {
		return err
	}

	if exists {
		request := ldap.NewDelRequest(distinguishedName, nil)
		err := client.Del(request)
		if err != nil {
			return err
		}
	}
	return nil
}

func getParentObject(ou string) string {
	if ou == "" {
		log.Fatalf("unable to get parent object of empty string")
	}

	dn, err := ldap.ParseDN(ou)
	if err != nil {
		log.Fatal(err)
	}

	return joinRDNs(dn.RDNs[1:])
}

func getChildObject(ou string) string {
	if ou == "" {
		log.Fatalf("unable to get child object of empty string")
	}

	dn, err := ldap.ParseDN(ou)
	if err != nil {
		log.Fatal(err)
	}

	return joinRDNs(dn.RDNs[:0])
}

func joinRDNs(rdns []*ldap.RelativeDN) string {
	var segments []string
	for _, rdn := range rdns {
		segment := fmt.Sprintf("%s=%s", rdn.Attributes[0].Type, rdn.Attributes[0].Value)
		segments = append(segments, segment)
	}
	return strings.Join(segments, ",")
}
