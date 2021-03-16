package provider

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	uac "github.com/audibleblink/msldapuac"
	"github.com/go-ldap/ldap/v3"
	"golang.org/x/text/encoding/unicode"
)

// Meta structure for resource access
type LdapClient struct {
	conn       *ldap.Conn
	ldapURL    string
	searchBase string
}

type LdapEntry struct {
	client              *LdapClient
	entry               *ldap.Entry
	requestedAttributes []string
}

type LdapAccount struct {
	ldapEntry *LdapEntry
}

type LdapOU struct {
	ldapEntry *LdapEntry
}

func encodePassword(password string) (string, error) {
	utf16 := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	passwordUTF, err := utf16.NewEncoder().String(fmt.Sprintf("%q", password))
	if err != nil {
		return password, err
	}

	return passwordUTF, nil
}

func joinRDNs(rdns []*ldap.RelativeDN) string {
	var segments []string
	for _, rdn := range rdns {
		segment := fmt.Sprintf("%s=%s", rdn.Attributes[0].Type, rdn.Attributes[0].Value)
		segments = append(segments, segment)
	}
	return strings.Join(segments, ",")
}

func getParentObject(distinguishedName string) (string, error) {
	dn, err := ldap.ParseDN(distinguishedName)
	if err != nil {
		return "", err
	}
	return joinRDNs(dn.RDNs[1:]), nil
}

func getChildObject(distinguishedName string) (string, error) {
	dn, err := ldap.ParseDN(distinguishedName)
	if err != nil {
		return "", err
	}

	return joinRDNs(dn.RDNs[:1]), nil
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

// LdapClient receivers

func (c *LdapClient) NewClient(url string, bindAccount string, bindPassword string, searchBase string) error {
	var err error

	if url == "" {
		return fmt.Errorf("no url provided for LDAP client")
	}

	c.ldapURL = url

	c.conn, err = ldap.DialURL(url)
	if err != nil {
		return err
	}

	err = c.bind(bindAccount, bindPassword)
	if err != nil {
		return err
	}

	if c.searchBase == "" {
		err := c.setAutoSearchBase()
		if err != nil || c.searchBase == "" {
			return fmt.Errorf("ADLDAP_SEARCH_BASE not set and LDAP search base auto-detection failed")
		}
	}

	return nil
}

func (c *LdapClient) bind(bindAccount string, bindPassword string) error {
	err := c.conn.Bind(bindAccount, bindPassword)
	return err
}

func (c *LdapClient) setAutoSearchBase() error {
	searchRequest := ldap.NewSearchRequest(
		"", // The base dn to search
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)",
		[]string{"defaultNamingContext"},
		nil,
	)

	result, err := c.conn.Search(searchRequest)
	if err != nil {
		return err
	}

	c.searchBase = result.Entries[0].GetAttributeValue("defaultNamingContext")

	return nil
}

func (c *LdapClient) LdapSearch(filter string, attributes []string) (*ldap.SearchResult, error) {
	searchRequest := ldap.NewSearchRequest(
		c.searchBase, // The base dn to search
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,     // The filter to apply
		attributes, // A list attributes to retrieve
		nil,
	)

	// TODO handle errors other than "not found", etc.

	result, err := c.conn.Search(searchRequest)
	return result, err
}

func (c *LdapClient) GetObject(objectName string, searchField string, objectClass string, attributes []string) (*LdapEntry, error) {
	ldapEntry := new(LdapEntry)
	entry, err := c.GetEntry(objectName, searchField, objectClass, attributes)
	if err != nil {
		return ldapEntry, err
	}

	ldapEntry.client = c
	ldapEntry.entry = entry
	ldapEntry.requestedAttributes = attributes

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
	return result.entry.DN, err
}

func (c *LdapClient) GetObjectByDN(distinguishedName string, attributes []string) (*LdapEntry, error) {
	return c.GetObject(distinguishedName, "distinguishedName", "*", attributes)
}

func (c *LdapClient) GetObjectBySAMAccountName(sAMAccountName string, attributes []string) (*LdapEntry, error) {
	return c.GetObject(sAMAccountName, "sAMAccountName", "*", attributes)
}

func (c *LdapClient) GetOU(distinguishedName string) (*LdapOU, error) {
	ouObj := new(LdapOU)

	obj, err := c.GetObject(distinguishedName, "distinguishedName", "organizationalUnit", nil)
	if err != nil {
		return ouObj, err
	}

	ouObj.ldapEntry = obj

	return ouObj, nil
}

func (c *LdapClient) GetAccountByDN(distinguishedName string, attributes []string) (*LdapAccount, error) {
	account := new(LdapAccount)
	ldapEntry, err := c.GetObject(distinguishedName, "distinguishedName", "*", attributes)

	account.ldapEntry = ldapEntry

	return account, err
}

func (c *LdapClient) GetAccountBySAMAccountName(sAMAccountName string, attributes []string) (*LdapAccount, error) {
	account := new(LdapAccount)
	ldapEntry, err := c.GetObject(sAMAccountName, "sAMAccountName", "*", attributes)

	account.ldapEntry = ldapEntry

	return account, err
}

func (c *LdapClient) CreateObject(distinguishedName string, attributes map[string][]string, objectClass string) (*LdapEntry, error) {
	ldapEntry := new(LdapEntry)

	exists, err := c.ObjectExists(distinguishedName, "*")
	if err != nil {
		return ldapEntry, err
	}
	if exists {
		return ldapEntry, fmt.Errorf("object \"%s\" already exists", distinguishedName)
	}

	request := ldap.NewAddRequest(distinguishedName, nil)
	request.Attribute("objectClass", []string{objectClass})

	if !(objectClass == "organizationalUnit") {
		request.Attribute("accountExpires", []string{fmt.Sprintf("%d", 0x00000000)})
	}

	for k, v := range attributes {
		request.Attribute(k, v)
	}

	err = c.conn.Add(request)
	if err != nil {
		return ldapEntry, err
	}

	var attributeNames []string
	for k := range attributes {
		attributeNames = append(attributeNames, k)
	}

	ldapEntry, err = c.GetObjectByDN(distinguishedName, attributeNames)
	if err != nil {
		return ldapEntry, err
	}

	return ldapEntry, nil
}

func (c *LdapClient) CreateOU(distinguishedName string) (*LdapOU, error) {
	var ou *LdapOU

	parsedSearchBase, _ := ldap.ParseDN(c.searchBase)
	parsedOU, _ := ldap.ParseDN(distinguishedName)
	if !parsedSearchBase.AncestorOf(parsedOU) {
		return ou, fmt.Errorf("organizational unit \"%s\" is not an ancestor of search base \"%s\"", distinguishedName, c.searchBase)
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

	parentOU, err := getParentObject(distinguishedName)
	if err != nil {
		return ou, err
	}

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
	account := new(LdapAccount)
	if attributes == nil {
		attributes = make(map[string][]string)
	}

	if val, ok := attributes["name"]; ok {
		name = val[0]
	} else {
		name = strings.TrimRight(sAMAccountName, "$")
	}

	dn := fmt.Sprintf("CN=%s,%s", name, ou)
	attributes["sAMAccountName"] = []string{sAMAccountName}
	attributes["userAccountControl"] = []string{fmt.Sprintf("%d", userAccountControl)}

	obj, err := c.CreateObject(dn, attributes, objectClass)
	if err != nil {
		return account, err
	}
	account.ldapEntry = obj

	return account, nil
}

func (c *LdapClient) CreateUserAccount(sAMAccountName string, ou string, attributes map[string][]string) (*LdapAccount, error) {
	userAccountControl := uac.NormalAccount | uac.Accountdisable
	return c.CreateAccount(sAMAccountName, ou, attributes, "user", userAccountControl)
}

func (c *LdapClient) CreateComputerAccount(sAMAccountName string, ou string, attributes map[string][]string) (*LdapAccount, error) {
	userAccountControl := uac.WorkstationTrustAccount
	return c.CreateAccount(sAMAccountName, ou, attributes, "computer", userAccountControl)
}

// LdapEntry receivers

func (e *LdapEntry) ParentObject() string {
	parent, err := getParentObject(e.entry.DN)
	if err != nil {
		log.Fatal(err)
	}
	return parent
}

func (e *LdapEntry) ChildObject() string {
	parent, err := getChildObject(e.entry.DN)
	if err != nil {
		log.Fatal(err)
	}
	return parent
}

func (e *LdapEntry) Refresh() error {
	ldapObject, err := e.client.GetObjectByDN(e.entry.DN, e.requestedAttributes)
	if err != nil {
		return err
	}

	e.entry = ldapObject.entry
	e.requestedAttributes = ldapObject.requestedAttributes

	return nil
}

func (e *LdapEntry) Move(destinationOU string) error {
	dn := e.entry.DN

	newOUExists, err := e.client.ObjectExists(destinationOU, "organizationalUnit")
	if err != nil {
		return err
	}
	if !newOUExists {
		return fmt.Errorf("cannot move object %s to non-existent organization unit \"%s\"", dn, destinationOU)
	}

	currentOU := e.ParentObject()
	currentOUDN, err := ldap.ParseDN(currentOU)
	if err != nil {
		return err
	}

	currentRDN := e.ChildObject()

	newOUDN, err := ldap.ParseDN(destinationOU)
	if err != nil {
		return err
	}

	if !currentOUDN.Equal(newOUDN) {
		request := ldap.NewModifyDNRequest(dn, currentRDN, true, destinationOU)
		err := e.client.conn.ModifyDN(request)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *LdapEntry) Rename(newCN string) error {
	dn := e.entry.DN
	fullNewCN := fmt.Sprintf("CN=%s", newCN)
	parsedOldDN, err := ldap.ParseDN(dn)
	if err != nil {
		return err
	}

	parsedOU, err := ldap.ParseDN(e.ParentObject())
	if err != nil {
		return err
	}

	newRDN, err := ldap.ParseDN(fullNewCN)
	if err != nil {
		return err
	}

	parsedNewDN, err := ldap.ParseDN(joinRDNs(append(newRDN.RDNs, parsedOU.RDNs...)))
	if err != nil {
		return err
	}

	if !parsedOldDN.Equal(parsedNewDN) {
		request := ldap.NewModifyDNRequest(dn, fullNewCN, true, "")
		err := e.client.conn.ModifyDN(request)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *LdapEntry) Delete() error {
	request := ldap.NewDelRequest(e.entry.DN, nil)
	err := e.client.conn.Del(request)
	if err != nil {
		return err
	}
	return nil
}

func (e *LdapEntry) UpdateAttribute(name string, value []string) error {
	attributeMap := map[string][]string{}
	attributeMap[name] = value

	err := e.UpdateAttributes(attributeMap)
	return err
}

func (e *LdapEntry) UpdateAttributes(attributeMap map[string][]string) error {
	request := ldap.NewModifyRequest(e.entry.DN, nil)

	for attr, newValue := range attributeMap {
		oldValue := e.entry.GetAttributeValues(attr)
		if !stringSlicesEqual(oldValue, newValue) {
			request.Replace(attr, newValue)
		}
	}
	if len(request.Changes) > 0 {
		err := e.client.conn.Modify(request)
		if err != nil {
			return err
		}
	}
	return nil
}

// LdapAccount receivers

func (a *LdapAccount) Move(destinationOU string) error {
	err := a.ldapEntry.Move(destinationOU)
	return err
}

func (a *LdapAccount) Rename(newCN string) error {
	err := a.ldapEntry.Rename(newCN)
	return err
}

func (a *LdapAccount) Delete() error {
	err := a.ldapEntry.Delete()
	return err
}

func (a *LdapAccount) Enable() error {
	err := a.RemoveUserAccountControl(uac.Accountdisable)
	return err
}

func (a *LdapAccount) Disable() error {
	err := a.AddUserAccountControl(uac.Accountdisable)
	return err
}

func (a *LdapAccount) IsEnabled() (bool, error) {
	currentUAC, err := a.GetUserAccountControl()
	if err != nil {
		return true, err
	}

	isDisabled, err := uac.IsSet(currentUAC, uac.Accountdisable)
	if err != nil {
		return true, err
	}

	return !isDisabled, nil
}

func (a *LdapAccount) GetUserAccountControl() (int64, error) {
	uacAttrPresent := false
	for _, attr := range a.ldapEntry.entry.Attributes {
		if attr.Name == "userAccountControl" {
			uacAttrPresent = true
		}
	}
	if !uacAttrPresent {
		a.ldapEntry.requestedAttributes = append(a.ldapEntry.requestedAttributes, "userAccountControl")
		err := a.ldapEntry.Refresh()
		if err != nil {
			return -1, err
		}
	}
	uacStr := a.ldapEntry.entry.GetAttributeValue("userAccountControl")
	if uacStr == "" {
		return -1, fmt.Errorf("userAccountControl attribute was not requested during initial retrieval")
	}
	result, err := strconv.ParseInt(uacStr, 10, 64)
	return result, err
}

func (a *LdapAccount) SetUserAccountControl(uacFlags int64) error {
	uacStr := fmt.Sprintf("%d", uacFlags)
	err := a.ldapEntry.UpdateAttribute("userAccountControl", []string{uacStr})

	return err
}

func (a *LdapAccount) AddUserAccountControl(flags int64) error {
	currentUAC, err := a.GetUserAccountControl()
	if err != nil {
		return err
	}

	newUAC := currentUAC | flags

	err = a.SetUserAccountControl(newUAC)

	return err
}

func (a *LdapAccount) RemoveUserAccountControl(flags int64) error {
	currentUAC, err := a.GetUserAccountControl()
	if err != nil {
		return err
	}

	newUAC := currentUAC &^ flags

	err = a.SetUserAccountControl(newUAC)

	return err
}

func (a *LdapAccount) SetPassword(password string) error {
	passwordEncoded, err := encodePassword(password)
	if err != nil {
		return err
	}

	err = a.ldapEntry.UpdateAttribute("unicodePwd", []string{passwordEncoded})
	if err != nil {
		return err
	}

	return nil
}

// LdapOU receivers

func (a *LdapOU) IsEmpty() (bool, error) {
	searchRequest := ldap.NewSearchRequest(
		a.ldapEntry.entry.DN, // The base dn to search
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)", // The filter to apply
		nil,               // A list attributes to retrieve
		nil,
	)

	result, err := a.ldapEntry.client.conn.Search(searchRequest)
	if err != nil {
		return false, err
	}

	// Subordinate subtree scope is not available in ldap module so search will always return 1 entry for the searchBase.
	if len(result.Entries) > 1 {
		return false, nil
	}

	return true, nil
}

func (a *LdapOU) Delete() error {
	isEmpty, err := a.IsEmpty()
	if err != nil {
		return err
	}
	if !isEmpty {
		return fmt.Errorf("unable to delete \"%s\": organizational unit is not empty", a.ldapEntry.entry.DN)
	}

	err = a.ldapEntry.Delete()
	return err
}

func (a *LdapOU) Rename(distinguishedName string) error {
	oldDN := a.ldapEntry.entry.DN
	parsedOldDN, err := ldap.ParseDN(oldDN)
	if err != nil {
		return err
	}

	newParentOU, err := getParentObject(distinguishedName)
	if err != nil {
		return err
	}

	newParentExists, err := a.ldapEntry.client.ObjectExists(newParentOU, "organizationalUnit")
	if err != nil {
		return err
	}
	if !newParentExists {
		return fmt.Errorf("ancestors for renamed organization unit \"%s\" must already exist", distinguishedName)
	}

	parsedNewDN, err := ldap.ParseDN(distinguishedName)
	if err != nil {
		return err
	}

	newRDN := joinRDNs(parsedNewDN.RDNs[:1])
	newSup := joinRDNs(parsedNewDN.RDNs[1:])

	if !parsedNewDN.Equal(parsedOldDN) {
		request := ldap.NewModifyDNRequest(oldDN, newRDN, true, newSup)
		err := a.ldapEntry.client.conn.ModifyDN(request)
		if err != nil {
			return err
		}
	}
	return nil
}
