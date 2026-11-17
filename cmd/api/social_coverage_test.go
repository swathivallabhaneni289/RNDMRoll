package main

import (
	"slices"
	"testing"
)

// ADM-06: the report email goes to each ADMIN_EMAILS address that has an
// account: in lower case, once each, in the order given, leaving out the
// addresses no account had when the server started (missing, in any case)
// and empty entries.
func TestSocialCoverage_AdminAddressesAreTheAccountsOnceInLowerCase(t *testing.T) {
	for _, c := range []struct {
		name         string
		all, missing []string
		want         []string
	}{
		{"case, spaces, a repeat, a missing one and an empty entry",
			[]string{" Boss@Example.com", "boss@example.com", "late@example.com", ""}, []string{"LATE@example.com"},
			[]string{"boss@example.com"}},
		{"the order is kept",
			[]string{"b@example.com", "A@Example.com", "c@example.com"}, nil,
			[]string{"b@example.com", "a@example.com", "c@example.com"}},
		{"a missing address with spaces", []string{"a@example.com", "b@example.com"}, []string{" B@example.com "},
			[]string{"a@example.com"}},
		{"every address missing", []string{"a@example.com"}, []string{"a@example.com"}, nil},
		{"no addresses", nil, nil, nil},
		{"only blanks", []string{"", "  "}, nil, nil},
	} {
		if got := adminAddresses(c.all, c.missing); !slices.Equal(got, c.want) || (c.want == nil && got != nil) {
			t.Fatalf("%s: adminAddresses(%q, %q) = %q, want %q", c.name, c.all, c.missing, got, c.want)
		}
	}
}
