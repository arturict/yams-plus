package wizard

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/arturict/yams-plus/internal/config"
	"golang.org/x/term"
)

type Result struct {
	Config        config.Config
	AdminPassword string
	Secrets       map[string]string
}

type Wizard struct {
	In    *bufio.Reader
	RawIn io.Reader
	Out   io.Writer
}

func New(in io.Reader, out io.Writer) Wizard {
	return Wizard{In: bufio.NewReader(in), RawIn: in, Out: out}
}

func (w Wizard) Run() (Result, error) {
	cfg := config.Default()
	fmt.Fprintln(w.Out, "YAMS Plus beta — fewer dashboards, more movie night.")
	fmt.Fprintln(w.Out, "Nothing is published and you will add indexers yourself in Prowlarr.")
	var err error
	if cfg.AdminUsername, err = w.text("Admin username", "admin"); err != nil {
		return Result{}, err
	}
	password, err := w.password()
	if err != nil {
		return Result{}, err
	}
	if cfg.Timezone, err = w.text("Timezone", "Etc/UTC"); err != nil {
		return Result{}, err
	}
	bind, err := w.text("Private bind address (LAN or Tailscale)", "127.0.0.1")
	if err != nil {
		return Result{}, err
	}
	cfg.BindAddresses = []string{bind}
	if cfg.Modules.Movies, err = w.yesNo("Movies", true); err != nil {
		return Result{}, err
	}
	if cfg.Modules.Series, err = w.yesNo("Series", true); err != nil {
		return Result{}, err
	}
	if !cfg.Modules.Movies && !cfg.Modules.Series {
		return Result{}, fmt.Errorf("at least movies or series must be enabled")
	}
	if cfg.Modules.Subtitles, err = w.yesNo("Subtitles", true); err != nil {
		return Result{}, err
	}
	if cfg.Modules.Books, err = w.yesNo("Books and audiobooks (beta)", false); err != nil {
		return Result{}, err
	}
	mode, err := w.choice("Download method", []string{"usenet", "torrent", "both"})
	if err != nil {
		return Result{}, err
	}
	cfg.Downloads.Mode = mode
	secrets := map[string]string{}
	if mode == "usenet" || mode == "both" {
		cfg.Downloads.Usenet.Provider, err = w.text("Usenet provider", "newshosting")
		if err != nil {
			return Result{}, err
		}
		cfg.Downloads.Usenet.Host, err = w.text("NNTP TLS host", "news.newshosting.com")
		if err != nil {
			return Result{}, err
		}
		username, err := w.text("Usenet username", "")
		if err != nil {
			return Result{}, err
		}
		secret, err := w.secret("Usenet password")
		if err != nil {
			return Result{}, err
		}
		secrets[cfg.Downloads.Usenet.UsernameSecret] = username
		secrets[cfg.Downloads.Usenet.PasswordSecret] = secret
		cfg.Downloads.Usenet.UseVPN, err = w.yesNo("Route Usenet through a VPN (TLS already remains mandatory)", false)
		if err != nil {
			return Result{}, err
		}
	}
	if mode == "torrent" || mode == "both" {
		cfg.Downloads.Torrent.UseVPN, err = w.yesNo("Protect torrent traffic with a fail-closed VPN", true)
		if err != nil {
			return Result{}, err
		}
		if cfg.Downloads.Torrent.UseVPN {
			cfg.Downloads.Torrent.PortForwarding, err = w.yesNo("Enable VPN port forwarding when supported", false)
			if err != nil {
				return Result{}, err
			}
		} else {
			accepted, err := w.yesNo("I understand torrent peers will see this host's public IP", false)
			if err != nil {
				return Result{}, err
			}
			if !accepted {
				return Result{}, fmt.Errorf("torrent without VPN was not acknowledged")
			}
		}
	}
	if cfg.Downloads.Usenet.UseVPN || cfg.Downloads.Torrent.UseVPN && (mode == "torrent" || mode == "both") {
		cfg.Downloads.VPN.Provider, err = w.text("Gluetun VPN provider", "protonvpn")
		if err != nil {
			return Result{}, err
		}
		cfg.Downloads.VPN.Type, err = w.choiceDefault("VPN protocol", []string{"wireguard", "openvpn"}, "wireguard")
		if err != nil {
			return Result{}, err
		}
		if cfg.Downloads.VPN.Type == "wireguard" {
			key, err := w.secret("WireGuard private key")
			if err != nil {
				return Result{}, err
			}
			secrets[cfg.Downloads.VPN.PrivateKeySecret] = key
		} else {
			username, err := w.text("OpenVPN username", "")
			if err != nil {
				return Result{}, err
			}
			password, err := w.secret("OpenVPN password")
			if err != nil {
				return Result{}, err
			}
			secrets[cfg.Downloads.VPN.UsernameSecret] = username
			secrets[cfg.Downloads.VPN.PasswordSecret] = password
		}
	}
	if cfg.Modules.Movies {
		cfg.Quality.Movies, err = w.mediaQuality("movie", true)
		if err != nil {
			return Result{}, err
		}
	}
	if cfg.Modules.Series {
		cfg.Quality.Series, err = w.mediaQuality("series", true)
		if err != nil {
			return Result{}, err
		}
	}
	if cfg.Modules.Subtitles {
		languages, err := w.text("Subtitle languages (comma separated)", "en,de")
		if err != nil {
			return Result{}, err
		}
		cfg.Subtitles.Languages = splitCSV(languages)
		cfg.Subtitles.Forced, err = w.yesNo("Prefer forced subtitles when available", true)
		if err != nil {
			return Result{}, err
		}
		cfg.Subtitles.HearingImpaired, err = w.yesNo("Include hearing-impaired subtitles", false)
		if err != nil {
			return Result{}, err
		}
		cfg.Subtitles.Provider, err = w.text("Bazarr subtitle provider", "opensubtitles.com")
		if err != nil {
			return Result{}, err
		}
		username, err := w.text("Subtitle provider username", "")
		if err != nil {
			return Result{}, err
		}
		secret, err := w.secret("Subtitle provider password")
		if err != nil {
			return Result{}, err
		}
		secrets[cfg.Subtitles.UsernameSecret], secrets[cfg.Subtitles.PasswordSecret] = username, secret
	}
	if err := cfg.Validate(); err != nil {
		return Result{}, err
	}
	return Result{Config: cfg, AdminPassword: password, Secrets: secrets}, nil
}

func (w Wizard) mediaQuality(label string, advanced4K bool) (config.MediaQuality, error) {
	profiles := []string{}
	sevenTwenty, err := w.yesNo("Accept 720p "+label+" releases as a fallback below 1080p", false)
	if err != nil {
		return config.MediaQuality{}, err
	}
	fullHD, err := w.yesNo("Support 1080p "+label+" releases (recommended)", true)
	if err != nil {
		return config.MediaQuality{}, err
	}
	fourKLabel := "Support 4K " + label + " releases"
	if advanced4K && label == "series" {
		fourKLabel += " (advanced)"
	}
	fourK, err := w.yesNo(fourKLabel, false)
	if err != nil {
		return config.MediaQuality{}, err
	}
	if sevenTwenty {
		profiles = append(profiles, "720p")
	}
	if fullHD {
		profiles = append(profiles, "1080p")
	}
	if fourK {
		profiles = append(profiles, "2160p")
	}
	if len(profiles) == 0 {
		return config.MediaQuality{}, fmt.Errorf("select at least one %s quality", label)
	}
	if len(config.RenderedProfiles(profiles)) == 0 {
		return config.MediaQuality{}, fmt.Errorf("select 1080p or 4K %s: 720p is only a fallback tier of the 1080p profile", label)
	}
	defaultProfile := profiles[0]
	for _, profile := range profiles {
		if profile == "1080p" {
			defaultProfile = profile
		}
	}
	defaultProfile, err = w.choiceDefault("Default "+label+" quality", profiles, defaultProfile)
	if err != nil {
		return config.MediaQuality{}, err
	}
	return config.MediaQuality{Profiles: profiles, DefaultProfile: defaultProfile, FallbackUpgrade: true}, nil
}

func (w Wizard) text(label, defaultValue string) (string, error) {
	if defaultValue == "" {
		fmt.Fprintf(w.Out, "%s: ", label)
	} else {
		fmt.Fprintf(w.Out, "%s [%s]: ", label, defaultValue)
	}
	line, err := w.In.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		line = defaultValue
	}
	if line == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return line, nil
}

func (w Wizard) password() (string, error) {
	password, err := w.secret("Admin password (minimum 14 characters)")
	if err != nil {
		return "", err
	}
	if len(password) < 14 {
		return "", fmt.Errorf("admin password must contain at least 14 characters")
	}
	return password, nil
}

func (w Wizard) secret(label string) (string, error) {
	fmt.Fprintf(w.Out, "%s: ", label)
	if file, ok := w.RawIn.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		raw, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(w.Out)
		if err != nil {
			return "", err
		}
		value := strings.TrimSpace(string(raw))
		if value == "" {
			return "", fmt.Errorf("%s is required", label)
		}
		return value, nil
	}
	line, err := w.In.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return value, nil
}

func (w Wizard) yesNo(label string, defaultValue bool) (bool, error) {
	def := "y/N"
	if defaultValue {
		def = "Y/n"
	}
	for {
		fmt.Fprintf(w.Out, "%s [%s]: ", label, def)
		line, err := w.In.ReadString('\n')
		if err != nil && err != io.EOF {
			return false, err
		}
		line = strings.ToLower(strings.TrimSpace(line))
		if line == "" {
			return defaultValue, nil
		}
		if line == "y" || line == "yes" {
			return true, nil
		}
		if line == "n" || line == "no" {
			return false, nil
		}
		fmt.Fprintln(w.Out, "Please answer y or n.")
	}
}

func (w Wizard) choice(label string, values []string) (string, error) {
	fmt.Fprintf(w.Out, "%s (%s): ", label, strings.Join(values, "/"))
	line, err := w.In.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	for _, value := range values {
		if line == value {
			return value, nil
		}
	}
	if index, err := strconv.Atoi(line); err == nil && index > 0 && index <= len(values) {
		return values[index-1], nil
	}
	return "", fmt.Errorf("choose one of %s", strings.Join(values, ", "))
}

func (w Wizard) choiceDefault(label string, values []string, defaultValue string) (string, error) {
	fmt.Fprintf(w.Out, "%s (%s) [%s]: ", label, strings.Join(values, "/"), defaultValue)
	line, err := w.In.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return defaultValue, nil
	}
	for _, value := range values {
		if line == value {
			return value, nil
		}
	}
	return "", fmt.Errorf("%s must be one of %s", label, strings.Join(values, ", "))
}

func splitCSV(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
