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
