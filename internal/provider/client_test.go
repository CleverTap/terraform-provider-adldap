package provider

import (
	"testing"
)

func TestAdldapLdapDNParentDN(t *testing.T) {
	cases := []struct {
		ou     string
		parent string
	}{
		{
			ou:     "DC=example,DC=com",
			parent: "DC=com",
		},
		{
			ou:     "CN=Some computer,DC=example,DC=com",
			parent: "DC=example,DC=com",
		},
		{
			ou:     "DC=com",
			parent: "",
		},
		{
			ou:     "OU=First Unit, DC=example, DC=com",
			parent: "DC=example,DC=com",
		},
		{
			ou:     "OU=First Unit,DC=example,DC=com",
			parent: "DC=example,DC=com",
		},
		{
			ou:     "OU=Second Unit,OU=First Unit,DC=example,DC=com",
			parent: "OU=First Unit,DC=example,DC=com",
		},
	}

	for _, c := range cases {
		dn, err := NewLdapDN(c.ou)
		if err != nil {
			t.Fatalf("error in getParentObject: %s", err)
		}

		got := dn.ParentDN()
		if got != c.parent {
			t.Fatalf("error matching output and expected for \"%s\": got %s, expected %s", c.ou, got, c.parent)
		}
	}
}

func TestAdldapLdapDNRDN(t *testing.T) {
	cases := []struct {
		ou    string
		child string
	}{
		{
			ou:    "CN=Some Computer,DC=example,DC=com",
			child: "CN=Some Computer",
		},
		{
			ou:    "DC=com",
			child: "DC=com",
		},
		{
			ou:    "OU=First Unit, DC=example, DC=com",
			child: "OU=First Unit",
		},
		{
			ou:    "OU=First Unit,DC=example,DC=com",
			child: "OU=First Unit",
		},
		{
			ou:    "OU=Second Unit,OU=First Unit,DC=example,DC=com",
			child: "OU=Second Unit",
		},
	}

	for _, c := range cases {
		dn, err := NewLdapDN(c.ou)
		if err != nil {
			t.Fatalf("error in getParentObject: %s", err)
		}
		got := dn.RDN()
		if got != c.child {
			t.Fatalf("Error matching output and expected for \"%s\": got %s, expected %s", c.ou, got, c.child)
		}
	}
}

func TestAdldapClientSliceIsSubset(t *testing.T) {
	cases := []struct {
		parent   []string
		subset   []string
		expected bool
	}{
		{
			parent:   []string{"a", "b", "c"},
			subset:   []string{"a", "c"},
			expected: true,
		},
		{
			parent:   []string{"a"},
			subset:   []string{"a"},
			expected: true,
		},
		{
			parent:   []string{"a"},
			subset:   []string{"b"},
			expected: false,
		},
		{
			parent:   []string{"a", "b", "c"},
			subset:   []string{"d", "e"},
			expected: false,
		},
		{
			parent:   []string{"a", "b", "c"},
			subset:   []string{"a", "b", "c", "d", "e"},
			expected: false,
		},
		{
			parent:   []string{"a", "b", "c"},
			subset:   []string{"a", "e"},
			expected: false,
		},
	}

	for _, c := range cases {
		got := sliceIsSubset(c.parent, c.subset)
		if got != c.expected {
			t.Fatalf("Error matching output and expected for \"%s\"x\"%s\": got %t, expected %t", c.parent, c.subset, got, c.expected)
		}
	}

}
