package provider

import (
	"fmt"

	"github.com/go-ldap/ldap/v3"
)

type LdapOU struct {
	*LdapEntry
}

// LdapOU receivers

func (o *LdapOU) IsEmpty() (bool, error) {
	searchRequest := ldap.NewSearchRequest(
		o.DN, // The base dn to search
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)", // The filter to apply
		nil,               // A list attributes to retrieve
		nil,
	)

	result, err := o.Conn.Search(searchRequest)
	if err != nil {
		return false, err
	}

	// Subordinate subtree scope is not available in ldap module so search will always return 1 entry for the searchBase.
	if len(result.Entries) > 1 {
		return false, nil
	}

	return true, nil
}

func (o *LdapOU) Delete() error {
	isEmpty, err := o.IsEmpty()
	if err != nil {
		return err
	}
	if !isEmpty {
		return fmt.Errorf("unable to delete \"%s\": organizational unit is not empty", o.DN)
	}

	err = o.LdapEntry.Delete()
	return err
}

func (o *LdapOU) Rename(distinguishedName string) error {
	return o.ChangeDN(distinguishedName)
}
