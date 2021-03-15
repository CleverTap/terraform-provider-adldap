package provider

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/sethvargo/go-password/password"
)

var (
	rInt             = rand.New(rand.NewSource(time.Now().UnixNano())).Intn(999999)
	testUser         = fmt.Sprintf("tfacctst-%d", rInt)
	testUserFullName = fmt.Sprintf("Terraform acceptance testing-%d", rInt)
	testUserOU       = os.Getenv("ADLDAP_TEST_USER_OU")
	testUserOU2      = os.Getenv("ADLDAP_TEST_USER_OU2")
)

func init() {
	if testUserOU == "" {
		testUserOU = testAccProviderMeta.searchBase
	}
}

func TestAccResourceUser(t *testing.T) {
	testUserPassword, err := password.Generate(9, 1, 1, false, false)
	testUserPassword = strings.ReplaceAll(testUserPassword, "\\", "\\\\")
	testUserPassword = strings.ReplaceAll(testUserPassword, "\"", "\\\"")
	testUserPassword2, err := password.Generate(9, 1, 1, false, false)
	testUserPassword2 = strings.ReplaceAll(testUserPassword2, "\\", "\\\\")
	testUserPassword2 = strings.ReplaceAll(testUserPassword2, "\"", "\\\"")

	if err != nil {
		t.Error(err)
	}

	resource.UnitTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceUser(testUser, testUserPassword, testUserFullName, testUserOU),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"adldap_user.foo", "samaccountname", testUser),
					testAccUserBind(testUser, testUserPassword),
				),
			},
			{
				Config: testAccResourceUser(testUser, testUserPassword, testUserFullName, testUserOU2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"adldap_user.foo", "organizational_unit", testUserOU2),
				),
			},
			{
				Config: testAccResourceUser(testUser, testUserPassword, testUserFullName+"-2", testUserOU2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"adldap_user.foo", "name", testUserFullName+"-2"),
				),
			},
			{
				Config: testAccResourceUser(testUser, testUserPassword2, testUserFullName+"-2", testUserOU2),
				Check: resource.ComposeTestCheckFunc(
					testAccUserBind(testUser, testUserPassword2),
				),
			},
		},
	})
}

func testAccResourceUser(userName string, password string, fullName string, userOU string) string {
	return fmt.Sprintf(`
resource "adldap_user" "foo" {
  samaccountname      = "%s"
  password            = "%s"
  name                = "%s"
  organizational_unit = "%s"
}
`, userName, password, fullName, userOU)
}

func testAccUserBind(samaccountname string, password string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		dn, err := getDN(testAccProviderMeta.client, testAccProviderMeta.searchBase, samaccountname)
		if err != nil {
			return err
		}
		_, err = testProviderConfigure(testConfig.url, testConfig.searchBase, dn, password)
		if err != nil {
			return fmt.Errorf("error binding to test account %s: %s", samaccountname, err)
		}
		return nil
	}
}
