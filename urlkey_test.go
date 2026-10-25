package main

import (
	"encoding/json"
	"os"
	"testing"
)

// testdata/keys.json is shared with the extension's JavaScript tests so both sides agree.
func TestV2KeysSharedVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/keys.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct{ URL, Site, Page string }
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		u, err := parseWebURL(v.URL)
		if err != nil {
			t.Errorf("parseWebURL(%q): %v", v.URL, err)
			continue
		}
		site, page := v2Keys(u)
		if site != v.Site || page != v.Page {
			t.Errorf("v2Keys(%q) = (%q, %q), want (%q, %q)", v.URL, site, page, v.Site, v.Page)
		}
	}
}

func TestSiteKeyV1(t *testing.T) {
	tests := []struct{ url, want string }{
		{"https://www.theverge.com/a", "theverge.com"},
		{"https://github.com/Genymobile/scrcpy", "github.com/Genymobile"},
		{"https://github.com/", "github.com/"},
		{"https://x.com/jack", "x.com"},
	}
	for _, tt := range tests {
		u, err := parseWebURL(tt.url)
		if err != nil {
			t.Fatal(err)
		}
		if got := siteKeyV1(u); got != tt.want {
			t.Errorf("siteKeyV1(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestParseWebURLRejects(t *testing.T) {
	for _, raw := range []string{"", "ftp://example.com/", "mailto:a@b.c", "https:///"} {
		if _, err := parseWebURL(raw); err == nil {
			t.Errorf("parseWebURL(%q) succeeded, want error", raw)
		}
	}
}
