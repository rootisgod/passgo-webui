package api

import "testing"

func TestFirstSafeIPv4ChoosesPrivateAddress(t *testing.T) {
	host, ok := firstSafeIPv4([]string{"", "fe80::1", "10.42.0.15"})
	if !ok {
		t.Fatal("expected private IPv4 to be accepted")
	}
	if host != "10.42.0.15" {
		t.Fatalf("host = %q, want 10.42.0.15", host)
	}
}

func TestFirstSafeIPv4HandlesCIDRAddresses(t *testing.T) {
	host, ok := firstSafeIPv4([]string{"192.168.64.23/24"})
	if !ok {
		t.Fatal("expected CIDR IPv4 to be accepted")
	}
	if host != "192.168.64.23" {
		t.Fatalf("host = %q, want 192.168.64.23", host)
	}
}

func TestFirstSafeIPv4RejectsPublicAndInvalidAddresses(t *testing.T) {
	if host, ok := firstSafeIPv4([]string{"8.8.8.8", "not-an-ip", "2001:db8::1"}); ok {
		t.Fatalf("expected no safe host, got %q", host)
	}
}
