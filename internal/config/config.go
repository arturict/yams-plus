package config

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const SchemaVersion = 1

type Config struct {
	SchemaVersion int             `yaml:"schemaVersion" json:"schemaVersion"`
	ProjectName   string          `yaml:"projectName" json:"projectName"`
	Timezone      string          `yaml:"timezone" json:"timezone"`
	AdminUsername string          `yaml:"adminUsername" json:"adminUsername"`
	DataRoot      string          `yaml:"dataRoot" json:"dataRoot"`
	BindAddresses []string        `yaml:"bindAddresses" json:"bindAddresses"`
	Ports         Ports           `yaml:"ports" json:"ports"`
	Runtime       Runtime         `yaml:"runtime" json:"runtime"`
	Modules       Modules         `yaml:"modules" json:"modules"`
	Downloads     Downloads       `yaml:"downloads" json:"downloads"`
	Quality       Quality         `yaml:"quality" json:"quality"`
	Subtitles     Subtitles       `yaml:"subtitles" json:"subtitles"`
	Plugins       PluginPolicy    `yaml:"plugins" json:"plugins"`
	Affiliate     AffiliatePolicy `yaml:"affiliate" json:"affiliate"`
}

type Runtime struct {
	PUID                 int  `yaml:"puid" json:"puid"`
	PGID                 int  `yaml:"pgid" json:"pgid"`
	HardwareAcceleration bool `yaml:"hardwareAcceleration" json:"hardwareAcceleration"`
}

type Ports struct {
	Jellyfin          int `yaml:"jellyfin" json:"jellyfin"`
	JellyfinDiscovery int `yaml:"jellyfinDiscovery" json:"jellyfinDiscovery"`
	Seerr             int `yaml:"seerr" json:"seerr"`
	Prowlarr          int `yaml:"prowlarr" json:"prowlarr"`
	Radarr            int `yaml:"radarr" json:"radarr"`
	Sonarr            int `yaml:"sonarr" json:"sonarr"`
	Bazarr            int `yaml:"bazarr" json:"bazarr"`
	SABnzbd           int `yaml:"sabnzbd" json:"sabnzbd"`
	QBittorrent       int `yaml:"qbittorrent" json:"qbittorrent"`
	Shelfmark         int `yaml:"shelfmark" json:"shelfmark"`
	Audiobookshelf    int `yaml:"audiobookshelf" json:"audiobookshelf"`
}

type Modules struct {
	Movies    bool `yaml:"movies" json:"movies"`
	Series    bool `yaml:"series" json:"series"`
	Subtitles bool `yaml:"subtitles" json:"subtitles"`
	Books     bool `yaml:"books" json:"books"`
}

type Downloads struct {
	Mode    string  `yaml:"mode" json:"mode"`
	Usenet  Usenet  `yaml:"usenet" json:"usenet"`
	Torrent Torrent `yaml:"torrent" json:"torrent"`
	VPN     VPN     `yaml:"vpn" json:"vpn"`
}

type Usenet struct {
	Provider       string `yaml:"provider" json:"provider"`
	Host           string `yaml:"host" json:"host"`
	Port           int    `yaml:"port" json:"port"`
	Connections    int    `yaml:"connections" json:"connections"`
	UsernameSecret string `yaml:"usernameSecret" json:"usernameSecret"`
	PasswordSecret string `yaml:"passwordSecret" json:"passwordSecret"`
	UseVPN         bool   `yaml:"useVpn" json:"useVpn"`
}

type Torrent struct {
	UseVPN         bool `yaml:"useVpn" json:"useVpn"`
	PortForwarding bool `yaml:"portForwarding" json:"portForwarding"`
}

type VPN struct {
	Provider         string `yaml:"provider" json:"provider"`
	Type             string `yaml:"type" json:"type"`
	PrivateKeySecret string `yaml:"privateKeySecret" json:"privateKeySecret"`
	UsernameSecret   string `yaml:"usernameSecret" json:"usernameSecret"`
	PasswordSecret   string `yaml:"passwordSecret" json:"passwordSecret"`
}

type Quality struct {
	Movies MediaQuality `yaml:"movies" json:"movies"`
	Series MediaQuality `yaml:"series" json:"series"`
	Audio  string       `yaml:"audio" json:"audio"`
	HDR    string       `yaml:"hdr" json:"hdr"`
}

type MediaQuality struct {
	Profiles        []string `yaml:"profiles" json:"profiles"`
	DefaultProfile  string   `yaml:"default" json:"default"`
	FallbackUpgrade bool     `yaml:"fallbackUpgrade" json:"fallbackUpgrade"`
}

type Subtitles struct {
	Languages       []string `yaml:"languages" json:"languages"`
	Forced          bool     `yaml:"forced" json:"forced"`
	HearingImpaired bool     `yaml:"hearingImpaired" json:"hearingImpaired"`
	Provider        string   `yaml:"provider" json:"provider"`
	UsernameSecret  string   `yaml:"usernameSecret" json:"usernameSecret"`
	PasswordSecret  string   `yaml:"passwordSecret" json:"passwordSecret"`
}

type PluginPolicy struct {
	RequiredCompatible    []string `yaml:"requiredCompatible" json:"requiredCompatible"`
	Excluded              []string `yaml:"excluded" json:"excluded"`
	PlaybackRetentionDays int      `yaml:"playbackRetentionDays" json:"playbackRetentionDays"`
}

type AffiliatePolicy struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

func Default() Config {
	return Config{
		SchemaVersion: SchemaVersion,
		ProjectName:   "yamsplus",
		Timezone:      "Etc/UTC",
		DataRoot:      "/srv/yamsplus",
		BindAddresses: []string{"127.0.0.1"},
		Ports:         Ports{Jellyfin: 8096, JellyfinDiscovery: 7359, Seerr: 5055, Prowlarr: 9696, Radarr: 7878, Sonarr: 8989, Bazarr: 6767, SABnzbd: 8080, QBittorrent: 8081, Shelfmark: 8084, Audiobookshelf: 13378},
		Runtime:       Runtime{PUID: 1000, PGID: 1000},
		Modules:       Modules{Movies: true, Series: true, Subtitles: true},
		Downloads: Downloads{
			Mode:    "usenet",
			Usenet:  Usenet{Provider: "custom", Port: 563, Connections: 20, UsernameSecret: "usenet_username", PasswordSecret: "usenet_password"},
			Torrent: Torrent{UseVPN: true},
			VPN:     VPN{Provider: "protonvpn", Type: "wireguard", PrivateKeySecret: "wireguard_private_key", UsernameSecret: "openvpn_username", PasswordSecret: "openvpn_password"},
		},
		Quality: Quality{
			Movies: MediaQuality{Profiles: []string{"1080p", "2160p"}, DefaultProfile: "1080p", FallbackUpgrade: true},
			Series: MediaQuality{Profiles: []string{"1080p"}, DefaultProfile: "1080p", FallbackUpgrade: true},
			Audio:  "original",
			HDR:    "compatible",
		},
		Subtitles: Subtitles{Languages: []string{"en", "de"}, Provider: "opensubtitles.com", UsernameSecret: "subtitles_username", PasswordSecret: "subtitles_password"},
		Plugins: PluginPolicy{
			RequiredCompatible: []string{"AudioDB", "Custom Tabs", "EditorsChoice", "File Transformation", "Intro Skipper", "Jellyfin Enhanced", "MusicBrainz", "OMDb", "Playback Reporting", "Plugin Pages", "Reports", "Studio Images", "TMDb"},
			Excluded:           []string{"SSO-Auth"}, PlaybackRetentionDays: 90,
		},
	}
}

func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg := Default()
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, raw, 0o640); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (c Config) Validate() error {
	var problems []string
	if c.SchemaVersion != SchemaVersion {
		problems = append(problems, fmt.Sprintf("schemaVersion must be %d", SchemaVersion))
	}
	if strings.TrimSpace(c.AdminUsername) == "" {
		problems = append(problems, "adminUsername is required")
	}
	if !c.Modules.Movies && !c.Modules.Series {
		problems = append(problems, "at least movies or series must be enabled")
	}
	if c.Modules.Subtitles && !c.Modules.Movies && !c.Modules.Series {
		problems = append(problems, "subtitles require movies or series")
	}
	if c.Downloads.Mode != "usenet" && c.Downloads.Mode != "torrent" && c.Downloads.Mode != "both" {
		problems = append(problems, "downloads.mode must be usenet, torrent, or both")
	}
	if c.Runtime.PUID <= 0 || c.Runtime.PGID <= 0 {
		problems = append(problems, "runtime.puid and runtime.pgid must be positive")
	}
	if c.Downloads.Mode == "usenet" || c.Downloads.Mode == "both" {
		if strings.TrimSpace(c.Downloads.Usenet.Host) == "" {
			problems = append(problems, "downloads.usenet.host is required")
		}
		if c.Downloads.Usenet.Port != 443 && c.Downloads.Usenet.Port != 563 {
			problems = append(problems, "Usenet must use TLS port 443 or 563")
		}
	}
	vpnEnabled := c.Downloads.Usenet.UseVPN || ((c.Downloads.Mode == "torrent" || c.Downloads.Mode == "both") && c.Downloads.Torrent.UseVPN)
	if vpnEnabled {
		if strings.TrimSpace(c.Downloads.VPN.Provider) == "" {
			problems = append(problems, "downloads.vpn.provider is required when VPN is enabled")
		}
		if c.Downloads.VPN.Type != "wireguard" && c.Downloads.VPN.Type != "openvpn" {
			problems = append(problems, "downloads.vpn.type must be wireguard or openvpn")
		}
		if c.Downloads.VPN.Type == "wireguard" && c.Downloads.VPN.PrivateKeySecret == "" {
			problems = append(problems, "downloads.vpn.privateKeySecret is required for WireGuard")
		}
		if c.Downloads.VPN.Type == "openvpn" && (c.Downloads.VPN.UsernameSecret == "" || c.Downloads.VPN.PasswordSecret == "") {
			problems = append(problems, "downloads.vpn username and password secrets are required for OpenVPN")
		}
	}
	if len(c.BindAddresses) == 0 {
		problems = append(problems, "bindAddresses must list at least one loopback, private, or Tailscale IPv4 address")
	}
	for _, addr := range c.BindAddresses {
		ip := net.ParseIP(addr)
		if ip == nil {
			problems = append(problems, fmt.Sprintf("invalid bind address %q", addr))
			continue
		}
		if ip.To4() == nil {
			// Published ports are rendered as "ADDRESS:host:container", which is
			// ambiguous for IPv6 literals, and every app URL is built the same way.
			problems = append(problems, fmt.Sprintf("bind address %q must be IPv4", addr))
			continue
		}
		if !ip.IsLoopback() && !ip.IsPrivate() && !IsTailscale(ip) {
			problems = append(problems, fmt.Sprintf("bind address %q is not loopback, private, or Tailscale IPv4", addr))
		}
	}
	portValues := map[string]int{"jellyfin": c.Ports.Jellyfin, "seerr": c.Ports.Seerr, "prowlarr": c.Ports.Prowlarr, "radarr": c.Ports.Radarr, "sonarr": c.Ports.Sonarr, "bazarr": c.Ports.Bazarr, "sabnzbd": c.Ports.SABnzbd, "qbittorrent": c.Ports.QBittorrent, "shelfmark": c.Ports.Shelfmark, "audiobookshelf": c.Ports.Audiobookshelf}
	usedPorts := map[int]string{}
	for name, port := range portValues {
		if port < 1 || port > 65535 {
			problems = append(problems, fmt.Sprintf("ports.%s must be between 1 and 65535", name))
			continue
		}
		if previous, exists := usedPorts[port]; exists {
			problems = append(problems, fmt.Sprintf("ports.%s conflicts with ports.%s on %d", name, previous, port))
		}
		usedPorts[port] = name
	}
	if c.Ports.JellyfinDiscovery < 1 || c.Ports.JellyfinDiscovery > 65535 {
		problems = append(problems, "ports.jellyfinDiscovery must be between 1 and 65535")
	}
	if err := validateQuality("movies", c.Quality.Movies, c.Modules.Movies); err != nil {
		problems = append(problems, err.Error())
	}
	if err := validateQuality("series", c.Quality.Series, c.Modules.Series); err != nil {
		problems = append(problems, err.Error())
	}
	if c.Quality.Audio != "original" {
		problems = append(problems, "quality.audio must be original in schema v1")
	}
	if c.Affiliate.Enabled {
		problems = append(problems, "affiliate links cannot be enabled in the beta candidate")
	}
	if c.Plugins.PlaybackRetentionDays <= 0 || c.Plugins.PlaybackRetentionDays%30 != 0 {
		problems = append(problems, "plugins.playbackRetentionDays must be a positive multiple of 30 because Playback Reporting stores retention in months")
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// tailscaleCGNAT is the carrier-grade NAT range Tailscale assigns from.
// Matching on the "100." prefix alone would accept publicly routed addresses
// such as 100.24.5.6 (Amazon) and defeat the no-public-bind guard.
var tailscaleCGNAT = netip.MustParsePrefix("100.64.0.0/10")

// IsTailscale reports whether ip is inside Tailscale's 100.64.0.0/10 range.
func IsTailscale(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip.To4())
	return ok && tailscaleCGNAT.Contains(addr)
}

// LocalHost returns the address the host itself uses to reach the published
// application ports. A wildcard bind is not a routable destination, so it maps
// to loopback; LAN and Tailscale addresses are returned unchanged.
func (c Config) LocalHost() (string, error) {
	if len(c.BindAddresses) == 0 {
		return "", errors.New("config has no bindAddresses; set at least one loopback, private, or Tailscale IPv4 address")
	}
	host := c.BindAddresses[0]
	if host == "0.0.0.0" {
		return "127.0.0.1", nil
	}
	return host, nil
}

// recyclarrProfiles are the quality profiles the Recyclarr templates can
// create. TRaSH publishes no 720p profile id, so 720p is a fallback tier the
// 1080p profile accepts on the way to its cutoff, never a profile of its own.
var recyclarrProfiles = []string{"1080p", "2160p"}

// RenderedProfiles returns the selected profiles that are actually created.
func RenderedProfiles(selected []string) []string {
	rendered := make([]string, 0, len(selected))
	for _, profile := range recyclarrProfiles {
		if slices.Contains(selected, profile) {
			rendered = append(rendered, profile)
		}
	}
	return rendered
}

func validateQuality(name string, q MediaQuality, enabled bool) error {
	if !enabled {
		return nil
	}
	allowed := map[string]bool{"720p": true, "1080p": true, "2160p": true}
	seen := map[string]bool{}
	for _, profile := range q.Profiles {
		if !allowed[profile] {
			return fmt.Errorf("quality.%s contains unsupported profile %q", name, profile)
		}
		seen[profile] = true
	}
	if seen["720p"] && !seen["1080p"] {
		return fmt.Errorf("quality.%s selects 720p without 1080p; 720p creates no profile because it is a fallback tier of the 1080p profile, not a profile of its own", name)
	}
	rendered := RenderedProfiles(q.Profiles)
	if len(rendered) == 0 {
		return fmt.Errorf("quality.%s must select 1080p or 2160p", name)
	}
	// Seerr requests with the default profile, so it has to be one Recyclarr
	// actually creates; a 720p default passed here and then failed the install
	// at the Seerr step, after every image had been pulled.
	if !seen[q.DefaultProfile] || !slices.Contains(rendered, q.DefaultProfile) {
		return fmt.Errorf("quality.%s default must be one of its selected profiles and either 1080p or 2160p, not %q", name, q.DefaultProfile)
	}
	return nil
}
