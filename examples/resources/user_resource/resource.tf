resource "adldap_user" "example" {
  samaccountname      = "foobar"
  password            = "password123"
  name                = "Foo Bar"
  upn                 = "foobar@ad.example.com"
  spns                = ["SERVICE/foobar.ad.example.com", "HTTP/foobar.ad.example.com"]
  organizational_unit = "OU=Baz,DC=example,DC=com"
}
