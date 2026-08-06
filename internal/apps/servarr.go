package apps

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Servarr struct {
	Name       string
	API        *HTTPClient
	APIVersion string
}

type apiConfigXML struct {
	APIKey string `xml:"ApiKey"`
}

func DiscoverAPIKey(configDir string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	path := filepath.Join(configDir, "config.xml")
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil {
			var cfg apiConfigXML
			if xml.Unmarshal(raw, &cfg) == nil && strings.TrimSpace(cfg.APIKey) != "" {
				return strings.TrimSpace(cfg.APIKey), nil
			}
		}
		time.Sleep(time.Second)
	}
	return "", fmt.Errorf("API key did not appear in %s", path)
}

func (s Servarr) prefix() string {
	if s.APIVersion == "" {
		return "/api/v3"
	}
	return "/api/" + s.APIVersion
}

func (s Servarr) Authenticate(apiKey string) { s.API.Headers.Set("X-Api-Key", apiKey) }

func (s Servarr) Wait(ctx context.Context) error { return s.API.Wait(ctx, "/ping", 5*time.Minute) }

func (s Servarr) EnsureHostAuth(ctx context.Context, username, password string) error {
	if password == "" {
		return nil
	}
	path := s.prefix() + "/config/host"
	var current map[string]any
	if err := s.API.DoJSON(ctx, http.MethodGet, path, nil, &current); err != nil {
		return err
	}
	current["authenticationMethod"] = "forms"
	current["authenticationRequired"] = "enabled"
	current["username"] = username
	current["password"] = password
	current["passwordConfirmation"] = password
	return s.API.DoJSON(ctx, http.MethodPut, path, current, &current)
}

func (s Servarr) EnsureRootFolder(ctx context.Context, path string) error {
	endpoint := s.prefix() + "/rootfolder"
	var current []map[string]any
	if err := s.API.DoJSON(ctx, http.MethodGet, endpoint, nil, &current); err != nil {
		return err
	}
	for _, root := range current {
		if strings.EqualFold(fmt.Sprint(root["path"]), path) {
			return nil
		}
	}
	return s.API.DoJSON(ctx, http.MethodPost, endpoint, map[string]any{"path": path}, nil)
}

type DownloadClientSpec struct {
	Name           string
	Implementation string
	Protocol       string
	Priority       int
	Fields         map[string]any
}

func (s Servarr) EnsureDownloadClient(ctx context.Context, wanted DownloadClientSpec) error {
	base := s.prefix() + "/downloadclient"
	var schemas []map[string]any
	if err := s.API.DoJSON(ctx, http.MethodGet, base+"/schema", nil, &schemas); err != nil {
		return err
	}
	var body map[string]any
	for _, schema := range schemas {
		if strings.EqualFold(fmt.Sprint(schema["implementation"]), wanted.Implementation) {
			body = cloneMap(schema)
			break
		}
	}
	if body == nil {
		return fmt.Errorf("%s does not expose %s download-client schema", s.Name, wanted.Implementation)
	}
	body["name"], body["enable"], body["priority"] = wanted.Name, true, wanted.Priority
	if wanted.Protocol != "" {
		body["protocol"] = wanted.Protocol
	}
	if err := setSchemaFields(body, wanted.Fields); err != nil {
		return fmt.Errorf("%s %s: %w", s.Name, wanted.Name, err)
	}
	var existing []map[string]any
	if err := s.API.DoJSON(ctx, http.MethodGet, base, nil, &existing); err != nil {
		return err
	}
	for _, item := range existing {
		if strings.EqualFold(fmt.Sprint(item["name"]), wanted.Name) {
			body["id"] = item["id"]
			return s.API.DoJSON(ctx, http.MethodPut, base+"/"+fmt.Sprint(item["id"]), body, nil)
		}
	}
	return s.API.DoJSON(ctx, http.MethodPost, base, body, nil)
}

func (s Servarr) QualityProfiles(ctx context.Context) ([]map[string]any, error) {
	var profiles []map[string]any
	err := s.API.DoJSON(ctx, http.MethodGet, s.prefix()+"/qualityprofile", nil, &profiles)
	return profiles, err
}

func (s Servarr) EnsureApplication(ctx context.Context, implementation, name, targetURL, targetAPIKey string) error {
	base := s.prefix() + "/applications"
	var schemas []map[string]any
	if err := s.API.DoJSON(ctx, http.MethodGet, base+"/schema", nil, &schemas); err != nil {
		return err
	}
	var body map[string]any
	for _, schema := range schemas {
		if strings.EqualFold(fmt.Sprint(schema["implementation"]), implementation) {
			body = cloneMap(schema)
			break
		}
	}
	if body == nil {
		return fmt.Errorf("prowlarr does not expose %s application schema", implementation)
	}
	body["name"], body["syncLevel"] = name, "fullSync"
	fields := map[string]any{"prowlarrUrl": "http://prowlarr:9696", "baseUrl": targetURL, "apiKey": targetAPIKey}
	if err := setSchemaFields(body, fields); err != nil {
		return err
	}
	var existing []map[string]any
	if err := s.API.DoJSON(ctx, http.MethodGet, base, nil, &existing); err != nil {
		return err
	}
	for _, item := range existing {
		if strings.EqualFold(fmt.Sprint(item["name"]), name) {
			body["id"] = item["id"]
			return s.API.DoJSON(ctx, http.MethodPut, base+"/"+fmt.Sprint(item["id"]), body, nil)
		}
	}
	return s.API.DoJSON(ctx, http.MethodPost, base, body, nil)
}

func (s Servarr) IndexerCount(ctx context.Context) (int, error) {
	var indexers []map[string]any
	if err := s.API.DoJSON(ctx, http.MethodGet, s.prefix()+"/indexer", nil, &indexers); err != nil {
		return 0, err
	}
	return len(indexers), nil
}

// EnsureUsenetFallback makes Usenet immediate and torrent a 60 minute
// fallback. This is intentionally set only for a Both installation.
func (s Servarr) EnsureUsenetFallback(ctx context.Context) error {
	base := s.prefix() + "/delayprofile"
	var profiles []map[string]any
	if err := s.API.DoJSON(ctx, http.MethodGet, base, nil, &profiles); err != nil {
		return err
	}
	wanted := map[string]any{
		"enableUsenet": true, "enableTorrent": true,
		"preferredProtocol": "usenet", "usenetDelay": 0, "torrentDelay": 60,
		"order": 1, "tags": []int{},
	}
	for _, profile := range profiles {
		// Arr always keeps a global delay profile without tags. Reuse it so a
		// repeated apply never accumulates competing global policies.
		if tags, ok := profile["tags"].([]any); !ok || len(tags) == 0 {
			for key, value := range wanted {
				profile[key] = value
			}
			return s.API.DoJSON(ctx, http.MethodPut, base+"/"+fmt.Sprint(profile["id"]), profile, nil)
		}
	}
	return s.API.DoJSON(ctx, http.MethodPost, base, wanted, nil)
}

func setSchemaFields(body map[string]any, values map[string]any) error {
	raw, ok := body["fields"].([]any)
	if !ok {
		return fmt.Errorf("schema fields are missing")
	}
	seen := map[string]bool{}
	for _, item := range raw {
		field, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := fmt.Sprint(field["name"])
		for wanted, value := range values {
			if strings.EqualFold(name, wanted) {
				field["value"] = value
				seen[wanted] = true
			}
		}
	}
	for name := range values {
		if !seen[name] {
			return fmt.Errorf("schema field %q is unavailable", name)
		}
	}
	return nil
}

func cloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	if fields, ok := input["fields"].([]any); ok {
		copied := make([]any, len(fields))
		for i, item := range fields {
			if field, ok := item.(map[string]any); ok {
				copied[i] = cloneMap(field)
			} else {
				copied[i] = item
			}
		}
		out["fields"] = copied
	}
	return out
}
