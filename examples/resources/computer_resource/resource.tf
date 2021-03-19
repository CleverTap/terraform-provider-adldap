resource "adldap_computer" "example" {
  name                = "foo"
  organizational_unit = "OU=Baz,DC=example,DC=com"
}
