package provider

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

var (
	testSpn     string = "test/terraformtest%d"
	testAccount string = os.Getenv("ADLDAP_TEST_ACCOUNT")
)

// Unit tests

func TestSpnFormatValid(t *testing.T) {
	cases := []struct {
		spn      string
		expected bool
	}{
		{
			spn:      "http/some.web.server.com",
			expected: true,
		},
		{
			spn:      "http/shortname",
			expected: true,
		},
		{
			spn:      "http/some.web.server.com:80",
			expected: true,
		},
		{
			spn:      "http/shortname:80",
			expected: true,
		},
		{
			spn:      "http\\some.web.server.com",
			expected: false,
		},
		{
			spn:      "some.web.server.com",
			expected: false,
		},
		{
			spn:      "http/WEB01:http",
			expected: false,
		},
	}

	for _, c := range cases {
		got := spnFormatValid(c.spn)
		if got != c.expected {
			t.Fatalf("Error matching output and expected for \"%s\": got %t, expected %t", c.spn, got, c.expected)
		}
	}
}

// Acceptance tests

//func TestAccGetDN(t *testing.T) {
// Requires local test data to pass
// t.Skip()

// got, err := getDN(testAccProviderMeta.client, testAccProviderMeta.searchBase, testAccount)
// if err != nil {
// 	t.Error(err)
// }
// if got != testAccountDN {
// 	t.Errorf("getDN failed: got \"%s\" expected \"%s\".", got, testAccountDN)
// }
//}

func TestAccSpnExists(t *testing.T) {
	// Needs local data for positive test cases

	rInt := rand.New(rand.NewSource(time.Now().UnixNano())).Int()
	cases := []struct {
		samaccountname string
		spn            string
		expected       bool
	}{
		{
			samaccountname: fmt.Sprintf("account%d", rInt),
			spn:            "fake/doesnt.exist",
			expected:       false,
		},
	}
	for _, c := range cases {
		got, err := spnExists(testAccProviderMeta.client, testAccProviderMeta.searchBase, c.samaccountname, c.spn)
		if err != nil {
			t.Error(err)
		}
		if got != c.expected {
			t.Fatalf("Error matching output and expected for spn %s and account %s: got %t, expected %t", c.spn, c.samaccountname, got, c.expected)
		}
	}
}

func TestAccServicePrincipal(t *testing.T) {
	rInt := rand.New(rand.NewSource(time.Now().UnixNano())).Int()
	uniqueSpn := fmt.Sprintf(testSpn, rInt)
	if testAccount == "" {
		t.Fatalf("ADLDAP_TEST_ACCOUNT environment variable must be set for acceptance tests to function.")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccServicePrincipal(testAccount, uniqueSpn),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServicePrincipalExists("adldap_service_principal.testspn"),
					// testAccCheckServicePrincipalAttributes(rInt),
					resource.TestCheckResourceAttr("adldap_service_principal.testspn", "spn", uniqueSpn),
				),
			},
		},
		CheckDestroy: testAccServicePrincipalDestroyed(uniqueSpn),
	})
}

// Support functions

func testAccServicePrincipal(samaccountname string, spn string) string {
	return fmt.Sprintf(`
resource "adldap_service_principal" "testspn" {
  samaccountname = "%s"
  spn = "%s"
}`, samaccountname, spn)
}

func testAccCheckServicePrincipalExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := testAccProviderMeta.client

		rs := s.RootModule().Resources[resourceName]
		if rs == nil {
			return fmt.Errorf("Unable to find resource %s", resourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("No instance ID is set")
		}

		searchRequest := ldap.NewSearchRequest(
			testAccProviderMeta.searchBase, // The base dn to search
			ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
			fmt.Sprintf("(&(objectClass=organizationalPerson)(samAccountName=%s))", rs.Primary.Attributes["samaccountname"]), // The filter to apply
			[]string{"servicePrincipalNames"}, // A list attributes to retrieve
			nil,
		)
		t, err := client.Search(searchRequest)

		if err != nil {
			return err
		}

		if t == nil {
			return fmt.Errorf("SPN %s not found", rs.Primary.Attributes["spn"])
		}

		return nil
	}
}

func testAccServicePrincipalDestroyed(spn string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return nil
	}
}
