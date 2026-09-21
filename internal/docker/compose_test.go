package docker

import "testing"

func TestParseContainersJSONArray(t *testing.T) {
	// Compose v2.21 and newer print a single JSON array.
	out := `[{"Name":"yamsplus-jellyfin-1","Service":"jellyfin","State":"running","Health":"healthy"},
{"Name":"yamsplus-sonarr-1","Service":"sonarr","State":"exited","Health":""}]` + "\n"
	list, err := parseContainers(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0].Service != "jellyfin" || list[0].State != "running" || list[0].Health != "healthy" {
		t.Fatalf("first = %+v", list[0])
	}
	if list[1].Service != "sonarr" || list[1].State != "exited" {
		t.Fatalf("second = %+v", list[1])
	}
}

func TestParseContainersNewlineDelimited(t *testing.T) {
	// Older Compose releases print one JSON object per line.
	out := "{\"Name\":\"yamsplus-jellyfin-1\",\"Service\":\"jellyfin\",\"State\":\"running\",\"Health\":\"healthy\"}\r\n" +
		"\n" +
		"{\"Name\":\"yamsplus-prowlarr-1\",\"Service\":\"prowlarr\",\"State\":\"running\",\"Health\":\"\"}\n"
	list, err := parseContainers(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0].Name != "yamsplus-jellyfin-1" || list[1].Service != "prowlarr" {
		t.Fatalf("list = %+v", list)
	}
}

func TestParseContainersEmptyOutput(t *testing.T) {
	list, err := parseContainers("  \n")
	if err != nil {
		t.Fatal(err)
	}
	if list != nil {
		t.Fatalf("list = %+v, want nil", list)
	}
}

func TestParseContainersRejectsGarbage(t *testing.T) {
	if _, err := parseContainers("not json\n"); err == nil {
		t.Fatal("expected an error for undecodable compose output")
	}
	if _, err := parseContainers(`[{"Service":]`); err == nil {
		t.Fatal("expected an error for a malformed JSON array")
	}
}
