package wizard

import (
	"bytes"
	"strings"
	"testing"
)

func TestWizardUsenetHappyPath(t *testing.T) {
	input := strings.Join([]string{
		"captain", "this-is-a-long-password", "Europe/Zurich", "127.0.0.1",
		"y", "y", "y", "n", "usenet", "newshosting", "news.example.test", "user", "secret", "n",
		"n", "y", "y", "", // movie qualities and 1080p default
		"n", "y", "n", "", // series qualities and 1080p default
		"en,de", "y", "n", "opensubtitles.com", "subs-user", "subs-secret",
	}, "\n") + "\n"
	var out bytes.Buffer
	result, err := New(strings.NewReader(input), &out).Run()
	if err != nil {
		t.Fatal(err)
	}
	if result.Config.Downloads.Mode != "usenet" || result.Config.AdminUsername != "captain" {
		t.Fatalf("unexpected result: %#v", result.Config)
	}
	if result.Secrets["usenet_password"] != "secret" {
		t.Fatal("Usenet password was not captured")
	}
	if strings.Contains(out.String(), "secret\n") {
		t.Fatal("wizard echoed secret itself")
	}
}
