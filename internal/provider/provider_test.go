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

// Acceptance tests

func TestAccProvider(t *testing.T) {
	if err := New().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
	if testAccProviderMeta.Conn == nil {
		t.Fatalf("provider not connected")
	}
}

func testAccPreCheck(t *testing.T) {
	// Not implemented
}

func testProviderConfigure(ldapURL string, searchBase string, bindAccount string, bindPassword string) (*LdapClient, error) {
	client := new(LdapClient)

	err := client.New(ldapURL, bindAccount, bindPassword, searchBase, false)
	if err != nil {
		return client, err
	}

	return client, nil
}
