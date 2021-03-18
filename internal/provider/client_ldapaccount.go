package provider

import (
	"fmt"
	"strconv"

	uac "github.com/audibleblink/msldapuac"
)

// Type LdapAccount extends LdapEntry
type LdapAccount struct {
	*LdapEntry
}

// LdapAccount receivers

func (a *LdapAccount) Enable() error {
	return a.RemoveUACFlag(uac.Accountdisable)
}

func (a *LdapAccount) Disable() error {
	return a.AddUACFlag(uac.Accountdisable)
}

func (a *LdapAccount) Rename(newName string) error {
	newRDN := fmt.Sprintf("CN=%s", newName)
	err := a.LdapEntry.Rename(newRDN)
	if err != nil {
		return err
	}

	// err = a.UpdateAttribute("name", []string{newName})

	return nil
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
	uacStr, err := a.GetAttributeValue("userAccountControl")
	if err != nil {
		return -1, err
	}
	result, err := strconv.ParseInt(uacStr, 10, 64)
	return result, err
}

func (a *LdapAccount) SetUACFlag(uacFlags int64) error {
	uacStr := fmt.Sprintf("%d", uacFlags)
	err := a.UpdateAttribute("userAccountControl", []string{uacStr})

	return err
}

func (a *LdapAccount) AddUACFlag(flags int64) error {
	currentUAC, err := a.GetUserAccountControl()
	if err != nil {
		return err
	}

	newUAC := currentUAC | flags

	err = a.SetUACFlag(newUAC)

	return err
}

func (a *LdapAccount) RemoveUACFlag(flags int64) error {
	currentUAC, err := a.GetUserAccountControl()
	if err != nil {
		return err
	}

	newUAC := currentUAC &^ flags

	err = a.SetUACFlag(newUAC)

	return err
}

func (a *LdapAccount) SetPassword(password string) error {
	passwordEncoded, err := encodePassword(password)
	if err != nil {
		return err
	}

	err = a.UpdateAttribute("unicodePwd", []string{passwordEncoded})
	if err != nil {
		return err
	}

	return nil
}

func (a *LdapAccount) AddServicePrincipal(spn string) error {
	err := a.AddAttributeWithValues("servicePrincipalName", []string{spn})
	if err != nil {
		return err
	}

	return nil
}

func (a *LdapAccount) RemoveServicePrincipal(spn string) error {
	exists, err := a.HasServicePrincipal(spn)
	if err != nil {
		return err
	}

	if exists {
		err := a.RemoveAttributeValue("servicePrincipalName", []string{spn})
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *LdapAccount) GetServicePrincipals() ([]string, error) {
	return a.GetAttributeValues("servicePrincipalName")
}

func (a *LdapAccount) HasServicePrincipal(spn string) (bool, error) {

	spns, err := a.GetServicePrincipals()
	if err != nil {
		return false, err
	}

	hasSPN := false
	for _, attr := range spns {
		if attr == spn {
			hasSPN = true
		}
	}

	return hasSPN, nil
}
