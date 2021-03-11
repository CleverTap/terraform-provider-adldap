package provider

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccOrganizationalUnit(t *testing.T) {
	testOU := fmt.Sprintf("OU=Terraform Acceptance Test %d,%s", rand.New(rand.NewSource(time.Now().UnixNano())).Int(), testAccProviderMeta.searchBase)

	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationalUnit(testOU),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckOrganizationalUnitExists("adldap_organizational_unit.testou"),
					resource.TestCheckResourceAttr("adldap_organizational_unit.testou", "distinguished_name", testOU),
				),
			},
		},
		CheckDestroy: testAccOrganizationalUnitDestroyed(testOU),
	})
}

func TestAccOuExists(t *testing.T) {
	// Needs local data for positive test cases

	rInt := rand.New(rand.NewSource(time.Now().UnixNano())).Int()

	cases := []struct {
		ou       string
		expected bool
	}{
		{
			ou:       fmt.Sprintf("OU=Test OU %d,DC=example,DC=com", rInt),
			expected: false,
		},
	}
	for _, c := range cases {
		got, err := ouExists(testAccProviderMeta.client, testAccProviderMeta.searchBase, c.ou)
		if err != nil {
			t.Error(err)
		}
		if got != c.expected {
			t.Fatalf("Error matching output and expected for ou \"%s\": got %t, expected %t", c.ou, got, c.expected)
		}
	}
}

// Support functions

func testAccOrganizationalUnit(ou string) string {
	return fmt.Sprintf(`
resource "adldap_organizational_unit" "testou" {
  distinguished_name = "%s"
  create_parents = true
}`, ou)
}

func testAccCheckOrganizationalUnitExists(resourceName string) resource.TestCheckFunc {
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
			fmt.Sprintf("(&(objectClass=organizationalUnit)(distinguishedName=%s))", rs.Primary.Attributes["ou"]), // The filter to apply
			[]string{"organizationalUnit"}, // A list attributes to retrieve
			nil,
		)
		t, err := client.Search(searchRequest)

		if err != nil {
			return err
		}

		if t == nil {
			return fmt.Errorf("OU \"%s\" not found", rs.Primary.Attributes["OU"])
		}

		return nil
	}
}

func testAccOrganizationalUnitDestroyed(ou string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		exists, err := ouExists(testAccProviderMeta.client, testAccProviderMeta.searchBase, ou)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("OU \"%s\" still exists after tests", ou)
		}
		return nil
	}
}
