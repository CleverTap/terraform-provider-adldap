provider "adldap" {
  url           = "ldaps://example.com"
  bind_account  = "admin@example.com" # Or "CN=admin,CN=Users,DC=example,DC=com"
  bind_password = "password123"
  search_base   = "DC=example,DC=com"
}
