resource "adldap_user" "example" {
  sam_account_name        = "foobar"
  password                = "password123"
  display_name            = "Foo Bar"
  user_principal_name     = "foobar@ad.example.com"
  service_principal_names = ["SERVICE/foobar.ad.example.com", "HTTP/foobar.ad.example.com"]
  organizational_unit     = "OU=Baz,DC=example,DC=com"
  email_address           = "foobar@example.com"
  given_name              = "foo"
  surname                 = "bar"
  initials                = "d"
  enabled                 = true
}