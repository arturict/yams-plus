package config

import "testing"

func TestDefaultNeedsProviderHost(t *testing.T) {
	cfg := Default()
	cfg.AdminUsername = "admin"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing Usenet host to fail validation")
	}
	cfg.Downloads.Usenet.Host = "news.example.test"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid default rejected: %v", err)
	}
}

func TestRejectsPublicBind(t *testing.T) {
	cfg := Default()
	cfg.AdminUsername = "admin"
	cfg.Downloads.Usenet.Host = "news.example.test"
	cfg.BindAddresses = []string{"8.8.8.8"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected public bind address to be rejected")
	}
}

func TestRejectsEmptyBindAddresses(t *testing.T) {
	cfg := Default()
	cfg.AdminUsername = "admin"
	cfg.Downloads.Usenet.Host = "news.example.test"
	cfg.BindAddresses = nil
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected an empty bind address list to be rejected")
	}
}

// Published ports and every generated app URL use "ADDRESS:PORT", which an IPv6
// literal silently corrupts.
func TestRejectsIPv6Bind(t *testing.T) {
	cfg := Default()
	cfg.AdminUsername = "admin"
	cfg.Downloads.Usenet.Host = "news.example.test"
	cfg.BindAddresses = []string{"::1"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected an IPv6 bind address to be rejected")
	}
}

func TestLocalHost(t *testing.T) {
	cfg := Default()
	if _, err := (Config{}).LocalHost(); err == nil {
		t.Fatal("expected an error when no bind address is configured")
	}
	cfg.BindAddresses = []string{"0.0.0.0"}
	if host, err := cfg.LocalHost(); err != nil || host != "127.0.0.1" {
		t.Fatalf("host=%q err=%v, want loopback for a wildcard bind", host, err)
	}
	cfg.BindAddresses = []string{"100.101.102.103", "127.0.0.1"}
	if host, err := cfg.LocalHost(); err != nil || host != "100.101.102.103" {
		t.Fatalf("host=%q err=%v, want the Tailscale address unchanged", host, err)
	}
}
