package main

import (
	"cmp"
	"encoding/json"
	"slices"
	"time"
)

// feedV2 is read by extension 1.2 and later. Each site lists its stories by points,
// highest first, so the first entry is the site's top story.
type feedV2 struct {
	Version     int                  `json:"version"`
	GeneratedAt int64                `json:"generatedAt"`
	MinPoints   int                  `json:"minPoints"`
	Sites       map[string][]v2Story `json:"sites"`
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

func buildFeedV2(stories []story, minPoints int, now time.Time) ([]byte, error) {
	feed := feedV2{Version: 2, GeneratedAt: now.Unix(), MinPoints: minPoints, Sites: map[string][]v2Story{}}
	for _, s := range stories {
		u, err := parseWebURL(s.URL)
		if err != nil {
			continue
		}
		site, page := v2Keys(u)
		feed.Sites[site] = append(feed.Sites[site], v2Story{s, page})
	}
	for _, list := range feed.Sites {
		slices.SortFunc(list, func(a, b v2Story) int {
			return cmp.Or(cmp.Compare(b.s.Points, a.s.Points), cmp.Compare(a.s.ID, b.s.ID))
		})
	}
	return json.Marshal(feed)
}
