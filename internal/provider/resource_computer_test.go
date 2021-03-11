package provider

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

var (
	testComputer    = fmt.Sprintf("tfacctst-%d$", rand.New(rand.NewSource(time.Now().UnixNano())).Intn(999999))
	testComputerOU  = os.Getenv("ADLDAP_TEST_COMPUTER_OU")
	testComputerOU2 = os.Getenv("ADLDAP_TEST_COMPUTER_OU2")
)

func init() {
	if testComputerOU == "" {
		testComputerOU = testAccProviderMeta.searchBase
	}
}

func TestAccResourceComputer(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceComputer(testComputer, testComputerOU),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"adldap_computer.foo", "organizational_unit", testComputerOU),
				),
			},
			{
				Config: testAccResourceComputer(testComputer, testComputerOU2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"adldap_computer.foo", "organizational_unit", testComputerOU2),
				),
			},
		},
	})
}

func testAccResourceComputer(computerName string, computerOU string) string {
	return fmt.Sprintf(`
resource "adldap_computer" "foo" {
  name                = "%s"
  organizational_unit = "%s"
}
`, computerName, computerOU)
}
