package provider

import (
	"fmt"
	"log"

	"github.com/go-ldap/ldap/v3"
)

// Type LdapEntry includes an ldap.Entry as well as the client
type LdapEntry struct {
	*LdapClient
	*ldap.Entry
	requestedAttributes []string // Used to refresh the entry with initial and additional attributes
}

// LdapEntry receivers

func (e *LdapEntry) ParentDN() string {
	dn, err := NewLdapDN(e.DN)
	if err != nil {
		log.Fatal(err)
	}

	return dn.ParentDN()
}

func (e *LdapEntry) RDN() string {
	dn, err := NewLdapDN(e.DN)
	if err != nil {
		log.Fatal(err)
	}

	return dn.RDN()
}

func (e *LdapEntry) Refresh() error {
	ldapObject, err := e.GetObjectByDN(e.DN, e.requestedAttributes)
	if err != nil {
		return err
	}

	e.Entry = ldapObject.Entry
	e.requestedAttributes = ldapObject.requestedAttributes

	return nil
}

func (e *LdapEntry) Move(destinationContainer string) error {
	dn, err := NewLdapDN(e.DN)
	if err != nil {
		return err
	}
	destinationDN, err := NewLdapDN(destinationContainer)
	if err != nil {
		return err
	}

	newDN := JoinRDNs(append(dn.RDNs[:1], destinationDN.RDNs...))

	e.ChangeDN(newDN)

	return nil
}

func (e *LdapEntry) Rename(newRDN string) error {
	dn, err := NewLdapDN(e.DN)
	if err != nil {
		return err
	}
	rDN, err := NewLdapDN(newRDN)
	if err != nil {
		return err
	}

	newDN := JoinRDNs(append(rDN.RDNs, dn.RDNs[1:]...))

	e.ChangeDN(newDN)

	return nil
}

func (e *LdapEntry) ChangeDN(newDistinguishedName string) error {
	alreadyExists, err := e.ObjectExists(newDistinguishedName, "*")
	if err != nil {
		return err
	}
	if alreadyExists {
		return fmt.Errorf("rename failed: an object with distinguishedName \"%s\" already exists", newDistinguishedName)
	}

	oldDistinguishedName := e.DN

	oldDN, err := NewLdapDN(oldDistinguishedName)
	if err != nil {
		return err
	}
	newDN, err := NewLdapDN(newDistinguishedName)
	if err != nil {
		return err
	}

	newRDN := newDN.RDN()
	newParentDN := newDN.ParentDN()

	if !oldDN.Equal(newDN) {
		if oldDN.ParentDN() == newParentDN {
			newParentDN = ""
		} else {
			newContainerExists, err := e.ContainerExists(newParentDN)
			if err != nil {
				return err
			}
			if !newContainerExists {
				return fmt.Errorf("cannot move object %s to non-existent or non-container object \"%s\"", oldDistinguishedName, newParentDN)
			}
		}

		request := ldap.NewModifyDNRequest(oldDistinguishedName, newRDN, true, newParentDN)
		err = e.Conn.ModifyDN(request)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *LdapEntry) Delete() error {
	request := ldap.NewDelRequest(e.DN, nil)
	err := e.Conn.Del(request)
	if err != nil {
		return err
	}
	return nil
}

func (e *LdapEntry) AddAttributeWithValues(name string, value []string) error {
	exists := e.HasAttributeWithValues(name, value)
	if exists {
		return fmt.Errorf("attribute %s with value %s already exists", name, value)
	}

	request := ldap.NewModifyRequest(e.DN, nil)
	request.Add(name, value)

	err := e.Conn.Modify(request)
	if err != nil {
		return err
	}

	return nil
}

func (e *LdapEntry) GetAttributeValue(name string) (string, error) {
	value, err := e.GetAttributeValues(name)
	if err != nil {
		return "", err
	}
	if len(value) > 0 {
		return value[0], nil
	}
	return "", nil
}

func (e *LdapEntry) GetAttributeValues(name string) ([]string, error) {
	attrPresent := false
	if len(e.Attributes) > 0 {
		for _, attr := range e.Attributes {
			if attr.Name == name {
				attrPresent = true
			}
		}
	}
	if !attrPresent {
		e.requestedAttributes = append(e.requestedAttributes, name)
		err := e.Refresh()
		if err != nil {
			return []string{}, fmt.Errorf("error refreshing LdapEntry: %s", err)
		}
	}

	attributes := e.Entry.GetAttributeValues(name)

	return attributes, nil

}

func (e *LdapEntry) HasAttributeWithValues(name string, values []string) bool {
	attributes := e.Entry.GetAttributeValues(name)

	return sliceIsSubset(attributes, values)
}

func (e *LdapEntry) UpdateAttribute(name string, values []string) error {
	attributeMap := map[string][]string{}
	attributeMap[name] = values

	err := e.UpdateAttributes(attributeMap)
	return err
}

func (e *LdapEntry) UpdateAttributes(attributeMap map[string][]string) error {
	request := ldap.NewModifyRequest(e.DN, nil)

	for attr, newValue := range attributeMap {
		oldValue := e.Entry.GetAttributeValues(attr)
		if !stringSlicesEqual(oldValue, newValue) {
			request.Replace(attr, newValue)
		}
	}
	if len(request.Changes) > 0 {
		err := e.Conn.Modify(request)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *LdapEntry) RemoveAttributeValue(name string, value []string) error {
	dn := e.DN
	request := ldap.NewModifyRequest(dn, nil)
	request.Delete(name, value)

	err := e.Conn.Modify(request)
	if err != nil {
		return err
	}

	return nil
}
