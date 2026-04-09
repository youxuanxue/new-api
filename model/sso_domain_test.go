//go:build tt
// +build tt

package model

import "testing"

func TestUS156_AllowedEmailDomainAccepted(t *testing.T) {
	c := &SSOConfig{AllowedDomains: "corp.example.com"}
	if !c.IsEmailAllowedForSSO("alice@corp.example.com") {
		t.Fatal("expected allowed domain to pass")
	}
}

func TestUS156_DisallowedEmailDomainRejected(t *testing.T) {
	c := &SSOConfig{AllowedDomains: "corp.example.com"}
	if c.IsEmailAllowedForSSO("bob@evil.test") {
		t.Fatal("expected disallowed domain to fail")
	}
}

func TestUS156_MultipleAllowedDomainsWithWhitespace(t *testing.T) {
	c := &SSOConfig{AllowedDomains: "corp.example.com, other.example.com"}
	if !c.IsEmailAllowedForSSO("u@other.example.com") {
		t.Fatal("expected second domain (after comma+space) to match after trim")
	}
}

func TestUS156_EmptyAllowedDomainsAllowsAny(t *testing.T) {
	c := &SSOConfig{AllowedDomains: ""}
	if !c.IsEmailAllowedForSSO("anyone@wherever.example") {
		t.Fatal("empty allowed_domains means no restriction (product default)")
	}
}
