provider "adldap" {
  url           = "ldaps://example.com"
  bind_account  = "admin"
  bind_password = "password123"
  search_base   = "DC=example,DC=com"
}
