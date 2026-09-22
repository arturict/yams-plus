package apps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type HTTPClient struct {
	BaseURL string
	Client  *http.Client
	Headers http.Header
	// Host, when set, replaces the Host header of every request, for a server
	// that checks it against a port other than the one it is reached through.
	Host string
}

func NewHTTPClient(baseURL string) *HTTPClient {
	jar, _ := cookiejar.New(nil)
	return &HTTPClient{BaseURL: strings.TrimRight(baseURL, "/"), Client: &http.Client{Timeout: 15 * time.Second, Jar: jar}, Headers: make(http.Header)}
}

func (c *HTTPClient) SetCookie(name, value string) error {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return err
	}
	c.Client.Jar.SetCookies(base, []*http.Cookie{{Name: name, Value: value, Path: "/"}})
	return nil
}

func (c *HTTPClient) ClearCookie(name string) error {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return err
	}
	c.Client.Jar.SetCookies(base, []*http.Cookie{{Name: name, Path: "/", MaxAge: -1, Expires: time.Unix(1, 0)}})
	return nil
}

func (c *HTTPClient) Cookie(name string) string {
	values := c.CookieValues(name)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (c *HTTPClient) CookieValues(name string) []string {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil
	}
	var values []string
	for _, cookie := range c.Client.Jar.Cookies(base) {
		if cookie.Name == name {
			seen := false
			for _, value := range values {
				if value == cookie.Value {
					seen = true
					break
				}
			}
			if !seen {
				values = append(values, cookie.Value)
			}
		}
	}
	return values
}

func (c *HTTPClient) DoJSON(ctx context.Context, method, path string, body, output any, accepted ...int) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return err
	}
	for name, values := range c.Headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	if c.Host != "" {
		req.Host = c.Host
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	if len(accepted) > 0 {
		ok = false
		for _, code := range accepted {
			if resp.StatusCode == code {
				ok = true
				break
			}
		}
	}
	if !ok {
		return &HTTPError{Method: method, URL: req.URL.String(), Status: resp.StatusCode, Body: strings.TrimSpace(string(raw))}
	}
	if output != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, output); err != nil {
			return fmt.Errorf("decode %s %s: %w", method, path, err)
		}
	}
	return nil
}

func (c *HTTPClient) DoForm(ctx context.Context, method, path string, values url.Values, output any, accepted ...int) error {
	raw, err := c.doForm(ctx, method, path, values, accepted...)
	if err != nil {
		return err
	}
	if output != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, output); err != nil {
			return err
		}
	}
	return nil
}

func (c *HTTPClient) DoFormText(ctx context.Context, method, path string, values url.Values, accepted ...int) (string, error) {
	raw, err := c.doForm(ctx, method, path, values, accepted...)
	return string(raw), err
}

func (c *HTTPClient) doForm(ctx context.Context, method, path string, values url.Values, accepted ...int) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	for name, headers := range c.Headers {
		for _, value := range headers {
			req.Header.Add(name, value)
		}
	}
	if c.Host != "" {
		req.Host = c.Host
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	if len(accepted) > 0 {
		ok = false
		for _, code := range accepted {
			if resp.StatusCode == code {
				ok = true
				break
			}
		}
	}
	if !ok {
		return nil, &HTTPError{Method: method, URL: req.URL.String(), Status: resp.StatusCode, Body: strings.TrimSpace(string(raw))}
	}
	return raw, nil
}

func (c *HTTPClient) Wait(ctx context.Context, path string, timeout time.Duration) error {
	return c.WaitStatus(ctx, path, timeout, http.StatusOK, http.StatusUnauthorized, http.StatusForbidden)
}

// WaitStatus polls until the endpoint returns one of the explicitly accepted
// statuses. This is used for post-restart readiness where a public endpoint may
// answer before authenticated subsystems such as Jellyfin plugins are ready.
func (c *HTTPClient) WaitStatus(ctx context.Context, path string, timeout time.Duration, accepted ...int) error {
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		attempt, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := c.DoJSON(attempt, http.MethodGet, path, nil, nil, accepted...)
		cancel()
		if err == nil {
			return nil
		}
		last = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("wait for %s%s: %w", c.BaseURL, path, last)
}

type HTTPError struct {
	Method, URL string
	Status      int
	Body        string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s %s returned %d: %s", e.Method, e.URL, e.Status, e.Body)
}
