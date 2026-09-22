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
		"y", "n", "y", "", // movies: 1080p, no 720p fallback, 4K, default 1080p
		"y", "n", "n", // series: 1080p only, so no default question
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
	if result.Config.Quality.Movies.DefaultProfile != "1080p" || result.Config.Quality.Series.DefaultProfile != "1080p" {
		t.Fatalf("unexpected quality defaults: %#v", result.Config.Quality)
	}
}

// Deselecting 1080p used to leave 720p as the pre-filled default, which
// Recyclarr never creates, so the install failed at the Seerr step. Without
// 1080p the 720p question is not asked and the default is a created profile.
func TestWizardQualityWithout1080pDefaultsToACreatedProfile(t *testing.T) {
	input := strings.Join([]string{"n", "y"}, "\n") + "\n"
	var out bytes.Buffer
	quality, err := New(strings.NewReader(input), &out).mediaQuality("movies", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "720p") {
		t.Fatalf("720p must not be offered without 1080p:\n%s", out.String())
	}
	if len(quality.Profiles) != 1 || quality.Profiles[0] != "2160p" || quality.DefaultProfile != "2160p" {
		t.Fatalf("unexpected quality: %#v", quality)
	}
}
