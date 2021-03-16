package provider

import (
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
var testAccProviderMeta *LdapClient

func init() {
	testAccProvider = New()
	testAccProviders = map[string]*schema.Provider{
		"adldap": testAccProvider,
	}
	testAccProviderMeta, _ = testProviderConfigure(testConfig.url, testConfig.searchBase, testConfig.bindAccount, testConfig.bindPassword)
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
		got, err := getParentObject(c.ou)
		if err != nil {
			t.Fatalf("error in getParentObject: %s", err)
		}
		if got != c.parent {
			t.Fatalf("error matching output and expected for \"%s\": got %s, expected %s", c.ou, got, c.parent)
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
		got, err := getChildObject(c.ou)
		if err != nil {
			t.Fatalf("error in getParentObject: %s", err)
		}
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
	if testAccProviderMeta.conn == nil {
		t.Fatalf("provider not connected")
	}
}

func testAccPreCheck(t *testing.T) {
	// Not implemented
}

func testProviderConfigure(ldapURL string, searchBase string, bindAccount string, bindPassword string) (*LdapClient, error) {
	client := new(LdapClient)

	err := client.NewClient(ldapURL, bindAccount, bindPassword, searchBase)
	if err != nil {
		return client, err
	}

	return client, nil
}
