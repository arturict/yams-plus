package apps

import "testing"

func TestClearCookieRemovesSeededSession(t *testing.T) {
	client := NewHTTPClient("http://127.0.0.1:8080")
	if err := client.SetCookie("shelfmark_session", "stale"); err != nil {
		t.Fatal(err)
	}
	if client.Cookie("shelfmark_session") != "stale" {
		t.Fatal("expected seeded session")
	}
	if values := client.CookieValues("shelfmark_session"); len(values) != 1 || values[0] != "stale" {
		t.Fatalf("cookie values = %#v", values)
	}
	if err := client.ClearCookie("shelfmark_session"); err != nil {
		t.Fatal(err)
	}
	if client.Cookie("shelfmark_session") != "" {
		t.Fatal("expected session to be removed")
	}
}
