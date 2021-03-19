resource "adldap_user" "example" {
  samaccountname      = "foobar"
  password            = "password123"
  name                = "Foo Bar"
  organizational_unit = "OU=Baz,DC=example,DC=com"
  enabled             = false
}
