package provider

import (
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

type LdapDN struct {
	*ldap.DN
}

func JoinRDNs(rdns []*ldap.RelativeDN) string {
	var segments []string
	for _, rdn := range rdns {
		segment := fmt.Sprintf("%s=%s", rdn.Attributes[0].Type, rdn.Attributes[0].Value)
		segments = append(segments, segment)
	}
	return strings.Join(segments, ",")
}

func NewLdapDN(distinguishedName string) (LdapDN, error) {
	parsedDN, err := ldap.ParseDN(distinguishedName)
	if err != nil {
		return LdapDN{}, fmt.Errorf("error parsing DN: %s", err)
	}

	ldapDN := LdapDN{
		DN: parsedDN,
	}

	return ldapDN, nil
}

func (dn *LdapDN) Equal(other LdapDN) bool {
	return dn.DN.Equal(other.DN)
}

// func (dn *LdapDN) distinguishedName() string {
// 	return joinRDNs(dn.RDNs)
// }

func (dn *LdapDN) RDN() string {
	return JoinRDNs([]*ldap.RelativeDN{dn.RDNs[0]})
}

func (dn *LdapDN) ParentDN() string {
	if len(dn.RDNs) < 2 {
		return ""
	}
	return JoinRDNs(dn.RDNs[1:])
}

func (dn *LdapDN) Name() string {
	return dn.RDNs[0].Attributes[0].Value
}
