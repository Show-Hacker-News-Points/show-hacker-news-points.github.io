package main

import (
	"strings"
	"testing"
	"time"
)

func TestCheckRecent(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	hoursAgo := func(h int) story { return story{CreatedAt: now.Add(-time.Duration(h) * time.Hour).Unix()} }
	tests := []struct {
		name    string
		stories []story
		maxAge  time.Duration
		wantErr bool
	}{
		{name: "newest story 10 hours old", stories: []story{hoursAgo(500), hoursAgo(10)}, maxAge: 48 * time.Hour},
		{name: "newest story 3 days old", stories: []story{hoursAgo(500), hoursAgo(72)}, maxAge: 48 * time.Hour, wantErr: true},
		{name: "check turned off", stories: []story{hoursAgo(5000)}, maxAge: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkRecent(tt.stories, tt.maxAge, now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkRecent() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "72h0m0s old") {
				t.Errorf("error %q doesn't give the age", err)
			}
		})
	}
}
