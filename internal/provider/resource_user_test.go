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
	testUserPassword = fmt.Sprintf("tfacctst123!%d", rInt)
	testUserOU       = os.Getenv("ADLDAP_TEST_USER_OU")
	testUserOU2      = os.Getenv("ADLDAP_TEST_USER_OU2")
)

func init() {
	if testUserOU == "" {
		testUserOU = testAccProviderMeta.SearchBase
	}
}

func TestAccAdldapResourceUser(t *testing.T) {
	testUserPassword2, err := password.Generate(9, 1, 1, false, false)
	if err != nil {
		t.Error(err)
	}
	testUserPassword2 = strings.ReplaceAll(testUserPassword2, "\\", "\\\\")
	testUserPassword2 = strings.ReplaceAll(testUserPassword2, "\"", "\\\"")

	resource.UnitTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccAdldapResourceUser(testUser, testUserPassword, testUserFullName, testUserOU),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"adldap_user.foo", "samaccountname", testUser),
					resource.TestCheckTypeSetElemAttr(
						"adldap_user.foo", "spns.*", fmt.Sprintf("TFTEST/%s", testUser)),
					resource.TestCheckTypeSetElemAttr(
						"adldap_user.foo", "spns.*", fmt.Sprintf("TFTEST-2/%s", testUser)),
					testAccAdldapUserBind(testUser, testUserPassword),
				),
			},
			{
				Config: testAccAdldapResourceUser(testUser, testUserPassword, testUserFullName, testUserOU2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"adldap_user.foo", "organizational_unit", testUserOU2),
				),
			},
			{
				Config: testAccAdldapResourceUser(testUser, testUserPassword, testUserFullName+"-2", testUserOU2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"adldap_user.foo", "name", testUserFullName+"-2"),
				),
			},
			{
				Config: testAccAdldapResourceUser(testUser, testUserPassword2, testUserFullName+"-2", testUserOU2),
				Check: resource.ComposeTestCheckFunc(
					testAccAdldapUserBind(testUser, testUserPassword2),
				),
			},
			{
				Config: testAccAdldapResourceUser(testUser+"b", testUserPassword2, testUserFullName+"-2", testUserOU2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"adldap_user.foo", "samaccountname", testUser+"b"),
					resource.TestCheckTypeSetElemAttr(
						"adldap_user.foo", "spns.*", fmt.Sprintf("TFTEST/%s", testUser+"b")),
					resource.TestCheckTypeSetElemAttr(
						"adldap_user.foo", "spns.*", fmt.Sprintf("TFTEST-2/%s", testUser+"b")),
				),
			},
		},
	})
}

func testAccAdldapResourceUser(userName string, password string, fullName string, userOU string) string {
	return fmt.Sprintf(`
resource "adldap_user" "foo" {
  samaccountname      = "%s"
  password            = "%s"
  organizational_unit = "%s"
  name                = "%s"
  upn                 = "%s@example.com"
  spns                = ["TFTEST/%s","TFTEST-2/%s"]
}
`, userName, password, userOU, fullName, userName, userName, userName)
}

func testAccAdldapUserBind(samaccountname string, password string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		dn, err := testAccProviderMeta.GetDN(samaccountname)
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
