package main

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"time"
)

// The v2 feed is read by extension 1.2 and later, in two files:
//
//   - v2/archive-<hash>.json holds stories created before the start of last month.
//     Their points no longer move, so the file changes about once a month, and the
//     content hash in its name lets the extension keep it until it does.
//   - v2/index.json holds the newer stories plus the archive's file name, and is the
//     only file the extension downloads every day.
//
// In both, each site lists its stories by points, highest first.
type v2Archive struct {
	Version   int                  `json:"version"`
	MinPoints int                  `json:"minPoints"`
	Before    int64                `json:"before"`
	Sites     map[string][]v2Story `json:"sites"`
}

type v2Index struct {
	Version     int    `json:"version"`
	MinPoints   int    `json:"minPoints"`
	GeneratedAt int64  `json:"generatedAt"`
	Archive     string `json:"archive"`
	// Extension features to switch off for everyone, without a store review, e.g. "linkPoints"
	// if the link badges break sites. Listed in disabled-features.txt.
	Disabled []string             `json:"disabled,omitempty"`
	Sites    map[string][]v2Story `json:"sites"`
}

// v2Story is encoded as [id, points, comments, createdAt, title, pageKey] to keep the
// download small; the extension unpacks it.
type v2Story struct {
	s       story
	pageKey string
}

func (e v2Story) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{e.s.ID, e.s.Points, e.s.Comments, e.s.CreatedAt, e.s.Title, e.pageKey})
}

type v2Files struct {
	index       []byte
	archiveName string
	archive     []byte
}

// archiveCutoff is the start of the previous calendar month in UTC, so every story in
// the archive is at least a month old and the boundary only moves once a month.
func archiveCutoff(now time.Time) time.Time {
	year, month, _ := now.UTC().Date()
	return time.Date(year, month-1, 1, 0, 0, 0, 0, time.UTC)
}

func buildFeedV2(stories []story, minPoints int, now time.Time, disabled []string) (v2Files, error) {
	before := archiveCutoff(now).Unix()
	archive := v2Archive{Version: 2, MinPoints: minPoints, Before: before, Sites: map[string][]v2Story{}}
	index := v2Index{Version: 2, MinPoints: minPoints, GeneratedAt: now.Unix(), Disabled: disabled, Sites: map[string][]v2Story{}}
	for _, s := range stories {
		u, err := parseWebURL(s.URL)
		if err != nil {
			continue
		}
		site, page := v2Keys(u)
		sites := index.Sites
		if s.CreatedAt < before {
			sites = archive.Sites
		}
		sites[site] = append(sites[site], v2Story{s, page})
	}
	sortByPoints(archive.Sites)
	sortByPoints(index.Sites)

	// encoding/json writes map keys in sorted order, so equal archives are byte-identical.
	archiveJSON, err := json.Marshal(archive)
	if err != nil {
		return v2Files{}, err
	}
	sum := sha256.Sum256(archiveJSON)
	index.Archive = "archive-" + hex.EncodeToString(sum[:6]) + ".json"
	indexJSON, err := json.Marshal(index)
	if err != nil {
		return v2Files{}, err
	}
	return v2Files{index: indexJSON, archiveName: index.Archive, archive: archiveJSON}, nil
}

func sortByPoints(sites map[string][]v2Story) {
	for _, list := range sites {
		slices.SortFunc(list, func(a, b v2Story) int {
			return cmp.Or(cmp.Compare(b.s.Points, a.s.Points), cmp.Compare(a.s.ID, b.s.ID))
		})
	}
}
