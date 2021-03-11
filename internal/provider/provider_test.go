package provider

import (
	"log"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type TestValues struct {
	url          string
	bindAccount  string
	bindPassword string
	searchBase   string
}

var testConfig = TestValues{
	url:          os.Getenv("ADLDAP_URL"),
	searchBase:   os.Getenv("ADLDAP_SEARCH_BASE"),
	bindAccount:  os.Getenv("ADLDAP_BIND_ACCOUNT"),
	bindPassword: os.Getenv("ADLDAP_BIND_PASSWORD"),
}
var testAccProviders map[string]*schema.Provider
var testAccProvider *schema.Provider
var testAccProviderMeta Meta

func init() {
	testAccProvider = New()
	testAccProviders = map[string]*schema.Provider{
		"adldap": testAccProvider,
	}
	testAccProviderMeta, _ = testProviderConfigure(testConfig.url, testConfig.searchBase, testConfig.bindAccount, testConfig.bindPassword)
	if _, set := os.LookupEnv("ADLDAP_SEARCH_BASE"); !set && testAccProviderMeta.searchBase == "" {
		newSearchBase, err := detectSearchBase(testAccProviderMeta.client)
		if err != nil {
			log.Fatalf("ADLDAP_SEARCH_BASE not set and LDAP search base auto-detection failed.")
		}
		testAccProviderMeta.searchBase = newSearchBase
	}
}

// Unit tests

func TestGetParentObject(t *testing.T) {
	cases := []struct {
		ou     string
		parent string
	}{
		{
			ou:     "DC=example,DC=com",
			parent: "DC=com",
		},
		{
			ou:     "CN=Some computer,DC=example,DC=com",
			parent: "DC=example,DC=com",
		},
		{
			ou:     "DC=com",
			parent: "",
		},
		{
			ou:     "OU=First Unit, DC=example, DC=com",
			parent: "DC=example,DC=com",
		},
		{
			ou:     "OU=First Unit,DC=example,DC=com",
			parent: "DC=example,DC=com",
		},
		{
			ou:     "OU=Second Unit,OU=First Unit,DC=example,DC=com",
			parent: "OU=First Unit,DC=example,DC=com",
		},
	}

	for _, c := range cases {
		got := getParentObject(c.ou)
		if got != c.parent {
			t.Fatalf("Error matching output and expected for \"%s\": got %s, expected %s", c.ou, got, c.parent)
		}
	}
}

func TestGetChildObject(t *testing.T) {
	cases := []struct {
		ou    string
		child string
	}{
		{
			ou:    "CN=Some Computer,DC=example,DC=com",
			child: "CN=Some Computer",
		},
		{
			ou:    "DC=com",
			child: "DC=com",
		},
		{
			ou:    "OU=First Unit, DC=example, DC=com",
			child: "OU=First Unit",
		},
		{
			ou:    "OU=First Unit,DC=example,DC=com",
			child: "OU=First Unit",
		},
		{
			ou:    "OU=Second Unit,OU=First Unit,DC=example,DC=com",
			child: "OU=Second Unit",
		},
	}

	for _, c := range cases {
		got := getChildObject(c.ou)
		if got != c.child {
			t.Fatalf("Error matching output and expected for \"%s\": got %s, expected %s", c.ou, got, c.child)
		}
	}
}

// Acceptance tests

func TestAccProvider(t *testing.T) {
	if err := New().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestAccDetectSearchBase(t *testing.T) {
	var expected string
	if testConfig.searchBase == "" {
		expected = testAccProviderMeta.searchBase
	} else {
		expected = testConfig.searchBase
	}

	result, err := detectSearchBase(testAccProviderMeta.client)
	if err != nil {
		t.Fatal(err)
	}
	if result != expected {
		t.Fatalf("Error autodetecting searchbase: expected %s got %s", expected, result)
	}
}

func testAccPreCheck(t *testing.T) {
	// Not implemented
}

func testProviderConfigure(ldapURL string, searchBase string, bindAccount string, bindPassword string) (Meta, error) {
	var err error
	// ignoreSsl := d.Get("ignore_ssl").(bool)

	if ldapURL == "" {
		log.Fatalf("No LDAP URL provided to test provider.")
	}

	conn, err := dialLdap(ldapURL)
	if err != nil {
		log.Fatalf("Error on LDAP dial: %s", err)
	}

	err = bindLdap(conn, bindAccount, bindPassword)
	if err != nil {
		log.Fatalf("Error on bind: %s", err)
	}

	meta := Meta{
		client:     conn,
		searchBase: searchBase,
	}
	return meta, err
}
