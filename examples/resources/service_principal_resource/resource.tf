resource "adldap_service_principal" "example" {
  samaccountname = "foo"
  spn            = "bar/baz"
}
