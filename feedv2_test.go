package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestArchiveCutoff(t *testing.T) {
	tests := []struct{ now, want string }{
		{"2026-10-24T21:00:00Z", "2026-09-01T00:00:00Z"},
		{"2026-01-05T00:00:00Z", "2025-12-01T00:00:00Z"},
		{"2026-03-01T00:00:00Z", "2026-02-01T00:00:00Z"},
	}
	for _, tt := range tests {
		now, _ := time.Parse(time.RFC3339, tt.now)
		if got := archiveCutoff(now).Format(time.RFC3339); got != tt.want {
			t.Errorf("archiveCutoff(%s) = %s, want %s", tt.now, got, tt.want)
		}
	}
}

func TestFeedV2SplitsByAge(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-10-24T21:00:00Z")
	stories := []story{
		{ID: 1, URL: "https://example.com/old", Points: 900, CreatedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()},
		{ID: 2, URL: "https://example.com/new", Points: 450, CreatedAt: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC).Unix()},
		{ID: 3, URL: "https://example.com/edge", Points: 500, CreatedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).Unix()},
		{ID: 4, URL: "", Points: 2000, CreatedAt: 0},
	}
	files, err := buildFeedV2(stories, 400, now, nil)
	if err != nil {
		t.Fatal(err)
	}
	var archive, index struct {
		Archive string
		Sites   map[string][][]any
	}
	if err := json.Unmarshal(files.archive, &archive); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(files.index, &index); err != nil {
		t.Fatal(err)
	}
	if got := len(archive.Sites["example.com"]); got != 1 {
		t.Errorf("archive has %d example.com stories, want 1 (the 2020 one)", got)
	}
	if got := len(index.Sites["example.com"]); got != 2 {
		t.Errorf("index has %d example.com stories, want 2 (Sep 1 and Oct 20)", got)
	}
	if index.Archive != files.archiveName {
		t.Errorf("index names archive %q, want %q", index.Archive, files.archiveName)
	}

	// Later in the same month the archive must be byte-identical, so clients keep their copy.
	later, err := buildFeedV2(stories, 400, now.Add(5*24*time.Hour), nil)
	if err != nil {
		t.Fatal(err)
	}
	if later.archiveName != files.archiveName || !bytes.Equal(later.archive, files.archive) {
		t.Error("archive changed within the same month")
	}
	if bytes.Equal(later.index, files.index) {
		t.Error("index should carry the new generatedAt")
	}
}

func TestFeedV2DisabledFeatures(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-10-24T21:00:00Z")
	stories := []story{{ID: 1, URL: "https://example.com/a", Points: 500, CreatedAt: now.Unix()}}
	off, err := buildFeedV2(stories, 400, now, []string{"linkPoints"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(off.index), `"disabled":["linkPoints"]`) {
		t.Errorf("index lacks the disabled list: %s", off.index)
	}
	on, err := buildFeedV2(stories, 400, now, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(on.index), `"disabled"`) {
		t.Errorf("an empty disabled list should be left out: %s", on.index)
	}
}

func TestReadDisabledFeatures(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disabled-features.txt")
	if got, err := readDisabledFeatures(path); err != nil || got != nil {
		t.Errorf("missing file: got %v, %v", got, err)
	}
	if err := os.WriteFile(path, []byte("# comment\n\nlinkPoints\n  other  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readDisabledFeatures(path)
	if err != nil || strings.Join(got, ",") != "linkPoints,other" {
		t.Errorf("got %v, %v", got, err)
	}
}
