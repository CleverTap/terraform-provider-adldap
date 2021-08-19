package provider

import (
	"fmt"
	"sort"
	"strings"

	uac "github.com/audibleblink/msldapuac"
	"github.com/go-ldap/ldap/v3"
	"golang.org/x/text/encoding/unicode"
)

type LdapClient struct {
	*ldap.Conn
	LdapURL         string
	SearchBase      string
	ActIdempotently bool
}

func encodePassword(password string) (string, error) {
	utf16 := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	passwordUTF, err := utf16.NewEncoder().String(fmt.Sprintf("%q", password))
	if err != nil {
		return password, err
	}
	return passwordUTF, nil
}

func stringSlicesEqual(a []string, b []string) bool {
	sort.Strings(a)
	sort.Strings(b)

	// If one is nil, the other must also be nil.
	if (a == nil) != (b == nil) {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func sliceIsSubset(parent []string, subset []string) bool {
	if len(subset) == len(parent) {
		return stringSlicesEqual(parent, subset)
	}
	if len(subset) > len(parent) {
		return false
	}

	for _, s := range subset {
		valuePresent := false
		for _, p := range parent {
			if p == s {
				valuePresent = true
			}
		}
		if !valuePresent {
			return false
		}
	}
	return true
}

// LdapClient receivers

func (c *LdapClient) New(url string, bindAccount string, bindPassword string, searchBase string, actIdempotently bool) error {
	var err error

	if url == "" {
		return fmt.Errorf("no url provided for LDAP client")
	}

	c.LdapURL = url
	c.ActIdempotently = actIdempotently

	c.Conn, err = ldap.DialURL(url)
	if err != nil {
		return err
	}

	err = c.Bind(bindAccount, bindPassword)
	if err != nil {
		return err
	}

	if c.SearchBase == "" {
		defaultNamingContext, err := c.DefaultNamingContext()
		c.SearchBase = defaultNamingContext
		if err != nil || c.SearchBase == "" {
			return fmt.Errorf("searchBase is empty and Active Directory auto-detection failed")
		}
	}

	return nil
}

func (c *LdapClient) Bind(bindAccount string, bindPassword string) error {
	err := c.Conn.Bind(bindAccount, bindPassword)
	return err
}

func (c *LdapClient) DefaultNamingContext() (string, error) {
	searchRequest := ldap.NewSearchRequest(
		"", // The base dn to search
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)",
		[]string{"defaultNamingContext"},
		nil,
	)

	result, err := c.Conn.Search(searchRequest)
	if err != nil {
		return "", err
	}

	defaultNamingContext := result.Entries[0].GetAttributeValue("defaultNamingContext")

	return defaultNamingContext, nil
}

func (c *LdapClient) LdapSearch(filter string, attributes []string) (*ldap.SearchResult, error) {
	searchRequest := ldap.NewSearchRequest(
		c.SearchBase, // The base dn to search
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,     // The filter to apply
		attributes, // A list attributes to retrieve
		nil,
	)

	// TODO handle errors other than "not found", etc.

	result, err := c.Conn.Search(searchRequest)
	return result, err
}

func (c *LdapClient) GetObject(objectName string, searchField string, objectClass string, attributes []string) (*LdapEntry, error) {
	entry, err := c.GetEntry(objectName, searchField, objectClass, attributes)
	if err != nil {
		return &LdapEntry{}, err
	}
	ldapEntry := &LdapEntry{
		LdapClient:          c,
		Entry:               entry,
		requestedAttributes: attributes,
	}

	return ldapEntry, nil
}

func (c *LdapClient) GetEntry(objectName string, searchField string, objectClass string, attributes []string) (*ldap.Entry, error) {
	filter := fmt.Sprintf("(&(objectClass=%s)(%s=%s))", objectClass, searchField, objectName)

	results, err := c.LdapSearch(filter, attributes)
	if err != nil {
		return nil, err
	}
	if len(results.Entries) > 1 {
		return nil, fmt.Errorf("too many results (%d) returned for %s object \"%s\", expected 1", len(results.Entries), objectClass, objectName)
	}
	if len(results.Entries) == 0 {
		return nil, fmt.Errorf("no entry returned for %s object \"%s\"", objectClass, objectName)
	}
	return results.Entries[0], nil
}

func (c *LdapClient) ObjectExists(objectDN string, objectClass string) (bool, error) {
	filter := fmt.Sprintf("(&(objectClass=%s)(distinguishedName=%s))", objectClass, objectDN)

	results, err := c.LdapSearch(filter, nil)
	if err != nil {
		return false, err
	}
	if len(results.Entries) > 1 {
		return false, fmt.Errorf("too many results (%d) returned for %s object \"%s\", expected 1", len(results.Entries), "distinguishedName", objectDN)
	}
	if len(results.Entries) == 0 {
		return false, nil
	}
	return true, nil
}

func (c *LdapClient) ContainerExists(objectDN string) (bool, error) {
	filter := fmt.Sprintf("(&(|(objectClass=organizationalUnit)(objectClass=container)(objectClass=domain))(distinguishedName=%s))", objectDN)

	results, err := c.LdapSearch(filter, nil)
	if err != nil {
		return false, err
	}
	if len(results.Entries) > 1 {
		return false, fmt.Errorf("too many results (%d) returned for %s object \"%s\", expected 1", len(results.Entries), "distinguishedName", objectDN)
	}
	if len(results.Entries) == 0 {
		return false, nil
	}
	return true, nil
}

func (c *LdapClient) AccountExists(sAMAccountName string) (bool, error) {
	filter := fmt.Sprintf("(&(objectClass=%s)(samAccountName=%s))", "*", sAMAccountName)

	results, err := c.LdapSearch(filter, nil)
	if err != nil {
		return false, err
	}
	if len(results.Entries) > 1 {
		return false, fmt.Errorf("too many results (%d) returned for %s object \"%s\", expected 1", len(results.Entries), "sAMAccountName", sAMAccountName)
	}
	if len(results.Entries) == 0 {
		return false, nil
	}
	return true, nil
}

func (c *LdapClient) GetDN(sAMAccountName string) (string, error) {
	result, err := c.GetObjectBySAMAccountName(sAMAccountName, nil)
	return result.DN, err
}

func (c *LdapClient) GetObjectByDN(distinguishedName string, attributes []string) (*LdapEntry, error) {
	return c.GetObject(distinguishedName, "distinguishedName", "*", attributes)
}

func (c *LdapClient) GetObjectBySAMAccountName(sAMAccountName string, attributes []string) (*LdapEntry, error) {
	return c.GetObject(sAMAccountName, "sAMAccountName", "*", attributes)
}

func (c *LdapClient) GetOU(distinguishedName string) (*LdapOU, error) {
	ldapEntry, err := c.GetObject(distinguishedName, "distinguishedName", "organizationalUnit", nil)
	if err != nil {
		return &LdapOU{}, err
	}

	ldapOU := &LdapOU{
		LdapEntry: ldapEntry,
	}

	return ldapOU, nil
}

func (c *LdapClient) GetAccountByDN(distinguishedName string, attributes []string) (*LdapAccount, error) {
	ldapEntry, err := c.GetObject(distinguishedName, "distinguishedName", "*", attributes)
	if err != nil {
		return &LdapAccount{}, err
	}

	account := &LdapAccount{
		LdapEntry: ldapEntry,
	}

	return account, err
}

func (c *LdapClient) GetAccountBySAMAccountName(sAMAccountName string, attributes []string) (*LdapAccount, error) {
	ldapEntry, err := c.GetObject(sAMAccountName, "sAMAccountName", "*", attributes)
	if err != nil {
		return &LdapAccount{}, err
	}

	account := &LdapAccount{
		LdapEntry: ldapEntry,
	}

	return account, err
}

func (c *LdapClient) CreateObject(distinguishedName string, attributes map[string][]string, objectClass string) (*LdapEntry, error) {

	exists, err := c.ObjectExists(distinguishedName, "*")
	if err != nil {
		return new(LdapEntry), err
	}
	if exists {
		return new(LdapEntry), fmt.Errorf("object \"%s\" already exists", distinguishedName)
	}

	request := ldap.NewAddRequest(distinguishedName, nil)
	request.Attribute("objectClass", []string{objectClass})

	// if !(objectClass == "organizationalUnit") {
	// 	request.Attribute("accountExpires", []string{fmt.Sprintf("%d", 0x00000000)})
	// }

	for k, v := range attributes {
		request.Attribute(k, v)
	}

	err = c.Conn.Add(request)
	if err != nil {
		return new(LdapEntry), err
	}

	var attributeNames []string
	for k := range attributes {
		attributeNames = append(attributeNames, k)
	}

	ldapEntry, err := c.GetObjectByDN(distinguishedName, attributeNames)
	if err != nil {
		return ldapEntry, err
	}

	return ldapEntry, nil
}

func (c *LdapClient) CreateOU(distinguishedName string) (*LdapOU, error) {
	var ou *LdapOU

	parsedSearchBase, _ := ldap.ParseDN(c.SearchBase)
	parsedOU, _ := ldap.ParseDN(distinguishedName)
	if !parsedSearchBase.AncestorOf(parsedOU) {
		return ou, fmt.Errorf("organizational unit \"%s\" is not an ancestor of search base \"%s\"", distinguishedName, c.SearchBase)
	}
	if parsedOU.RDNs[0].Attributes[0].Type != "OU" {
		return ou, fmt.Errorf("\"%s\" is not an OU distinguished name", distinguishedName)
	}

	_, err := c.CreateObject(distinguishedName, nil, "organizationalUnit")
	if err != nil {
		return ou, err
	}

	ou, err = c.GetOU(distinguishedName)

	return ou, err
}

func (c *LdapClient) CreateOUAndParents(distinguishedName string) (*LdapOU, error) {
	var ou *LdapOU

	dn, err := NewLdapDN(distinguishedName)
	if err != nil {
		return ou, err
	}
	parentOU := dn.ParentDN()

	parentExists, err := c.ObjectExists(parentOU, "*")
	if err != nil {
		return ou, err
	}

	if !parentExists {
		c.CreateOUAndParents(parentOU)
	}

	return c.CreateOU(distinguishedName)
}

func (c *LdapClient) CreateAccount(sAMAccountName string, ou string, attributes map[string][]string, objectClass string, userAccountControl int) (*LdapAccount, error) {
	var name string
	if attributes == nil {
		attributes = make(map[string][]string)
	}

	if val, ok := attributes["displayName"]; ok {
		name = val[0]
	} else {
		name = strings.TrimRight(sAMAccountName, "$")
	}

	dn := fmt.Sprintf("CN=%s,%s", name, ou)
	attributes["sAMAccountName"] = []string{sAMAccountName}
	attributes["userAccountControl"] = []string{fmt.Sprintf("%d", userAccountControl)}

	ldapEntry, err := c.CreateObject(dn, attributes, objectClass)
	if err != nil {
		return &LdapAccount{}, err
	}
	account := &LdapAccount{
		LdapEntry: ldapEntry,
	}

	return account, nil
}

func (c *LdapClient) CreateUserAccount(sAMAccountName string, password string, ou string, attributes map[string][]string) (*LdapAccount, error) {
	userAccountControl := uac.NormalAccount | uac.Accountdisable

	account, err := c.CreateAccount(sAMAccountName, ou, attributes, "user", userAccountControl)
	if err != nil {
		return new(LdapAccount), fmt.Errorf("error creating user account: %s", err)
	}

	if password != "" {
		err := account.SetPassword(password)
		if err != nil {
			return nil, fmt.Errorf("error setting password: %s", err)
		}
	}

	return account, nil
}

func (c *LdapClient) CreateComputerAccount(sAMAccountName string, ou string, attributes map[string][]string) (*LdapAccount, error) {
	userAccountControl := uac.WorkstationTrustAccount
	return c.CreateAccount(sAMAccountName, ou, attributes, "computer", userAccountControl)
}
