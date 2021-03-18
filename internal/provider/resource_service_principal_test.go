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

func TestAccAdldapServicePrincipal(t *testing.T) {
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
				Config: testAccAdldapServicePrincipal(testAccount, uniqueSpn),
				Check: resource.ComposeTestCheckFunc(
					testAccAdldapCheckServicePrincipalExists("adldap_service_principal.testspn"),
					// testAccCheckServicePrincipalAttributes(rInt),
					resource.TestCheckResourceAttr("adldap_service_principal.testspn", "spn", uniqueSpn),
				),
			},
		},
		//CheckDestroy: testAccServicePrincipalDestroyed(uniqueSpn),
	})
}

// Support functions

func testAccAdldapServicePrincipal(samaccountname string, spn string) string {
	return fmt.Sprintf(`
resource "adldap_service_principal" "testspn" {
  samaccountname = "%s"
  spn = "%s"
}`, samaccountname, spn)
}

func testAccAdldapCheckServicePrincipalExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := testAccProviderMeta.Conn

		rs := s.RootModule().Resources[resourceName]
		if rs == nil {
			return fmt.Errorf("Unable to find resource %s", resourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("No instance ID is set")
		}

		searchRequest := ldap.NewSearchRequest(
			testAccProviderMeta.SearchBase, // The base dn to search
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
