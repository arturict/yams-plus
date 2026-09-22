package config

import (
	"strings"
	"testing"
)

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

// The guard exists to keep the stack off the public internet. Matching the
// "100." prefix accepted publicly routed addresses such as Amazon's
// 100.24.0.0/13; only Tailscale's 100.64.0.0/10 may pass.
func TestRejectsPublic100Addresses(t *testing.T) {
	base := func() Config {
		cfg := Default()
		cfg.AdminUsername = "admin"
		cfg.Downloads.Usenet.Host = "news.example.test"
		return cfg
	}
	for _, addr := range []string{"100.24.5.6", "100.200.1.1", "100.63.255.255", "100.128.0.1"} {
		cfg := base()
		cfg.BindAddresses = []string{addr}
		if err := cfg.Validate(); err == nil {
			t.Fatalf("public address %q was accepted as a Tailscale address", addr)
		}
	}
	for _, addr := range []string{"100.64.0.1", "100.101.102.103", "100.127.255.255"} {
		cfg := base()
		cfg.BindAddresses = []string{addr}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Tailscale address %q was rejected: %v", addr, err)
		}
	}
}

// 720p is offered by the wizard and selected by three shipped examples, but the
// Recyclarr templates create no 720p profile. Selecting it alongside a real
// profile is fine; selecting it alone silently produced an empty profile list.
func TestSevenTwentyIsAFallbackTierNotAProfile(t *testing.T) {
	base := func() Config {
		cfg := Default()
		cfg.AdminUsername = "admin"
		cfg.Downloads.Usenet.Host = "news.example.test"
		return cfg
	}
	cfg := base()
	cfg.Quality.Movies = MediaQuality{Profiles: []string{"720p"}, DefaultProfile: "720p"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("a 720p-only movie selection creates no Recyclarr profile and must be rejected")
	}
	cfg = base()
	cfg.Modules.Series = true
	cfg.Quality.Series = MediaQuality{Profiles: []string{"720p"}, DefaultProfile: "720p"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("a 720p-only series selection must be rejected")
	}
	cfg = base()
	cfg.Quality.Movies = MediaQuality{Profiles: []string{"720p", "1080p"}, DefaultProfile: "1080p"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("720p alongside 1080p must stay valid: %v", err)
	}
	if got := RenderedProfiles([]string{"720p", "1080p", "2160p"}); len(got) != 2 || got[0] != "1080p" || got[1] != "2160p" {
		t.Fatalf("RenderedProfiles = %v, want the two profiles the templates create", got)
	}
	if got := RenderedProfiles([]string{"720p"}); len(got) != 0 {
		t.Fatalf("RenderedProfiles = %v, want none", got)
	}
}

// Seerr requests with the default profile, so a default Recyclarr never
// creates must fail validation instead of the install, and 720p without 1080p
// is a silent no-op.
func TestQualityDefaultMustBeACreatedProfile(t *testing.T) {
	for _, q := range []MediaQuality{
		{Profiles: []string{"720p", "1080p"}, DefaultProfile: "720p"},
		{Profiles: []string{"720p", "2160p"}, DefaultProfile: "2160p"},
		{Profiles: []string{"1080p"}, DefaultProfile: "2160p"},
	} {
		cfg := Default()
		cfg.AdminUsername = "admin"
		cfg.Downloads.Usenet.Host = "news.example.test"
		cfg.Quality.Movies = q
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "quality.movies") {
			t.Errorf("%v with default %s must be rejected on its quality, got %v", q.Profiles, q.DefaultProfile, err)
		}
	}
	cfg := Default()
	cfg.AdminUsername = "admin"
	cfg.Downloads.Usenet.Host = "news.example.test"
	cfg.Quality.Movies = MediaQuality{Profiles: []string{"720p", "1080p", "2160p"}, DefaultProfile: "2160p"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("a 2160p default with 720p fallback must stay valid: %v", err)
	}
}

// projectName is written into compose.yaml unquoted and names every container.
func TestProjectNameFollowsComposeRules(t *testing.T) {
	for _, name := range []string{"", "Media", "media stack", "-media", "media\nimage: evil", "media:1"} {
		cfg := Default()
		cfg.AdminUsername = "admin"
		cfg.Downloads.Usenet.Host = "news.example.test"
		cfg.ProjectName = name
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "projectName") {
			t.Errorf("projectName %q must be rejected, got %v", name, err)
		}
	}
	for _, name := range []string{"yamsplus", "media2", "media_stack-2"} {
		cfg := Default()
		cfg.AdminUsername = "admin"
		cfg.Downloads.Usenet.Host = "news.example.test"
		cfg.ProjectName = name
		if err := cfg.Validate(); err != nil {
			t.Errorf("projectName %q must be accepted: %v", name, err)
		}
	}
}
