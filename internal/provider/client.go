package provider

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"sort"

	"github.com/go-ldap/ldap/v3"
	"golang.org/x/text/encoding/unicode"
)

// Meta structure for resource access
type Meta struct {
	client     *ldap.Conn
	searchBase string
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

func newClient(url string, bindAccount string, bindPassword string, searchBase string) (*ldap.Conn, string, error) {
	if url == "" {
		return nil, "", fmt.Errorf("no url provided for LDAP client")
	}

	conn, err := dialLdap(url)
	if err != nil {
		return nil, "", err
	}

	err = bindLdap(conn, bindAccount, bindPassword)
	if err != nil {
		return nil, "", err
	}

	if searchBase == "UNSET" {
		newSearchBase, err := detectSearchBase(conn)
		if err != nil {
			return nil, "", fmt.Errorf("ADLDAP_SEARCH_BASE not set and LDAP search base auto-detection failed")
		}
		searchBase = newSearchBase
	}

	return conn, searchBase, nil
}

func dialLdap(url string) (*ldap.Conn, error) {
	conn, err := ldap.DialURL(url)
	return conn, err
}

func bindLdap(client *ldap.Conn, bindAccount string, bindPassword string) error {
	err := client.Bind(bindAccount, bindPassword)
	return err
}

func ldapSearch(client *ldap.Conn, searchBase string, filter string, attributes []string) (*ldap.SearchResult, error) {
	searchRequest := ldap.NewSearchRequest(
		searchBase, // The base dn to search
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,     // The filter to apply
		attributes, // A list attributes to retrieve
		nil,
	)

	result, err := client.Search(searchRequest)

	// TODO handle errors other than "not found", etc.

	return result, err
}

func getDN(client *ldap.Conn, searchBase string, samAccountName string) (string, error) {
	result, err := getObject(client, searchBase, samAccountName, "*", []string{})
	if err != nil {
		return "", err
	}

	dn := result.DN
	return dn, nil
}

func detectSearchBase(client *ldap.Conn) (string, error) {
	searchRequest := ldap.NewSearchRequest(
		"", // The base dn to search
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)",
		[]string{"defaultNamingContext"},
		nil,
	)

	result, err := client.Search(searchRequest)
	if err != nil {
		return "", err
	}

	searchBase := result.Entries[0].GetAttributeValue("defaultNamingContext")

	return searchBase, nil
}

func objectExists(client *ldap.Conn, searchBase string, objectName string, objectClass string) (bool, error) {
	searchAttribute := "samAccountName"
	if match, _ := regexp.MatchString(`.+=.+`, objectName); match {
		searchAttribute = "distinguishedName"
	}

	filter := fmt.Sprintf("(&(objectClass=%s)(%s=%s))", objectClass, searchAttribute, objectName)

	result, err := ldapSearch(client, searchBase, filter, []string{})
	if err != nil {
		return false, err
	}
	if len(result.Entries) == 1 {
		return true, nil
	}
	if len(result.Entries) > 1 {
		return false, fmt.Errorf("too many results (%d) returned for %s object \"%s\", expected 1", len(result.Entries), objectClass, objectName)
	}
	return false, nil
}

func getObject(client *ldap.Conn, searchBase string, objectName string, objectClass string, attributes []string) (*ldap.Entry, error) {
	searchAttribute := "samAccountName"
	if match, _ := regexp.MatchString(`.+=.+`, objectName); match {
		searchAttribute = "distinguishedName"
	}

	filter := fmt.Sprintf("(&(objectClass=%s)(%s=%s))", objectClass, searchAttribute, objectName)

	result, err := ldapSearch(client, searchBase, filter, attributes)
	if err != nil {
		return nil, err
	}
	if len(result.Entries) > 1 {
		return nil, fmt.Errorf("too many results (%d) returned for %s object \"%s\", expected 1", len(result.Entries), objectClass, objectName)
	}
	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("no entry returned for %s object \"%s\"", objectClass, objectName)
	}
	return result.Entries[0], nil
}

func deleteObject(client *ldap.Conn, searchBase string, distinguishedName string, objectClass string) error {
	exists, err := objectExists(client, searchBase, distinguishedName, objectClass)
	if err != nil {
		return err
	}

	if exists {
		request := ldap.NewDelRequest(distinguishedName, nil)
		err := client.Del(request)
		if err != nil {
			return err
		}
	}
	return nil
}

func getParentObject(ou string) string {
	if ou == "" {
		log.Fatalf("unable to get parent object of empty string")
	}

	dn, err := ldap.ParseDN(ou)
	if err != nil {
		log.Fatal(err)
	}

	return joinRDNs(dn.RDNs[1:])
}

func getChildObject(ou string) string {
	if ou == "" {
		log.Fatalf("unable to get child object of empty string")
	}

	dn, err := ldap.ParseDN(ou)
	if err != nil {
		log.Fatal(err)
	}

	return joinRDNs(dn.RDNs[:1])
}

func joinRDNs(rdns []*ldap.RelativeDN) string {
	var segments []string
	for _, rdn := range rdns {
		segment := fmt.Sprintf("%s=%s", rdn.Attributes[0].Type, rdn.Attributes[0].Value)
		segments = append(segments, segment)
	}
	return strings.Join(segments, ",")
}

func moveObject(client *ldap.Conn, searchBase string, samaccountname string, objectClass string, newOU string) error {
	newOUExists, err := ouExists(client, searchBase, newOU)
	if err != nil {
		return err
	}
	if !newOUExists {
		return fmt.Errorf("cannot move %s object %s to non-existent organization unit \"%s\"", objectClass, samaccountname, newOU)
	}

	entry, err := getObject(client, searchBase, samaccountname, objectClass, []string{})
	if err != nil {
		return err
	}

	dn := entry.DN

	ou := getParentObject(dn)
	ouDN, err := ldap.ParseDN(ou)
	if err != nil {
		return err
	}

	newOUDN, err := ldap.ParseDN(newOU)
	if err != nil {
		return err
	}

	if !ouDN.Equal(newOUDN) {
		request := ldap.NewModifyDNRequest(dn, getChildObject(dn), true, newOU)
		err := client.ModifyDN(request)
		if err != nil {
			return err
		}
	}

	return nil
}

func renameObject(client *ldap.Conn, searchBase string, samaccountname string, objectClass string, newName string) error {
	entry, err := getObject(client, searchBase, samaccountname, objectClass, []string{"name"})
	if err != nil {
		return err
	}

	dn := entry.DN
	oldDN, err := ldap.ParseDN(entry.DN)
	if err != nil {
		return err
	}

	oldOU, err := ldap.ParseDN(getParentObject(dn))
	if err != nil {
		return err
	}

	newCN, err := ldap.ParseDN(fmt.Sprintf("CN=%s", newName))
	if err != nil {
		return err
	}
	newDN, err := ldap.ParseDN(joinRDNs(append(newCN.RDNs, oldOU.RDNs...)))
	if err != nil {
		return err
	}

	if !oldDN.Equal(newDN) {
		request := ldap.NewModifyDNRequest(dn, fmt.Sprintf("CN=%s", newName), true, "")
		err := client.ModifyDN(request)
		if err != nil {
			return err
		}
	}

	return nil
}

func updateObjectAttributes(client *ldap.Conn, searchBase string, samaccountname string, objectClass string, attributeMap map[string][]string) error {
	for k, v := range attributeMap {
		err := updateObjectAttribute(client, searchBase, samaccountname, objectClass, k, v)
		if err != nil {
			return err
		}
	}

	return nil
}

func updateObjectAttribute(client *ldap.Conn, searchBase string, samaccountname string, objectClass string, attribute string, newValue []string) error {
	entry, err := getObject(client, searchBase, samaccountname, objectClass, []string{})
	if err != nil {
		return err
	}
	dn := entry.DN

	attributeMap := map[string][]string{}
	for _, v := range entry.Attributes {
		attributeMap[v.Name] = v.Values
	}

	oldValue := attributeMap[attribute]

	if !stringSlicesEqual(oldValue, newValue) {
		request := ldap.NewModifyRequest(dn, nil)
		request.Replace(attribute, newValue)
		err = client.Modify(request)
		if err != nil {
			return err
		}
	}

	return nil
}

func encodePassword(password string) (string, error) {
	utf16 := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	passwordUTF, err := utf16.NewEncoder().String(fmt.Sprintf("%q", password))
	if err != nil {
		return password, err
	}

	return passwordUTF, nil
}

func setObjectPassword(client *ldap.Conn, searchBase string, samaccountname string, objectClass string, password string) error {
	passwordEncoded, err := encodePassword(password)
	if err != nil {
		return err
	}

	err = updateObjectAttribute(client, searchBase, samaccountname, objectClass, "unicodePwd", []string{passwordEncoded})
	if err != nil {
		return err
	}

	return nil
}
