package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const algoliaURL = "https://hn.algolia.com/api/v1/search_by_date"

// Algolia never returns more than 1000 hits for one query, so the crawl splits time
// windows until each one fits in a single page.
const algoliaMaxHits = 1000

type story struct {
	ID        int32  `json:"id"`
	ObjectID  string `json:"objectID"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Points    int32  `json:"points"`
	Comments  int32  `json:"num_comments"`
	CreatedAt int64  `json:"created_at_i"`
}

type algoliaPage struct {
	NbHits int     `json:"nbHits"`
	Hits   []story `json:"hits"`
}

type crawler struct {
	client    *http.Client
	minPoints int
}

// crawl returns every story with at least minPoints points, oldest first.
func (c *crawler) crawl(ctx context.Context) ([]story, error) {
	total, err := c.query(ctx, 0, time.Now().Add(24*time.Hour).Unix(), 0)
	if err != nil {
		return nil, err
	}
	stories, err := c.crawlWindow(ctx, 0, time.Now().Add(24*time.Hour).Unix())
	if err != nil {
		return nil, err
	}
	// Points change while the crawl runs, so a story can cross the threshold between queries.
	if missing := total.NbHits - len(stories); missing > total.NbHits/100 {
		return nil, fmt.Errorf("crawl found %d stories but Algolia reports %d", len(stories), total.NbHits)
	}
	return stories, nil
}

func (c *crawler) crawlWindow(ctx context.Context, from, to int64) ([]story, error) {
	page, err := c.query(ctx, from, to, algoliaMaxHits)
	if err != nil {
		return nil, err
	}
	if page.NbHits <= algoliaMaxHits {
		return page.Hits, nil
	}
	if to-from <= 1 {
		return nil, fmt.Errorf("more than %d stories in one second at %d", algoliaMaxHits, from)
	}
	mid := from + (to-from)/2
	older, err := c.crawlWindow(ctx, from, mid)
	if err != nil {
		return nil, err
	}
	newer, err := c.crawlWindow(ctx, mid, to)
	if err != nil {
		return nil, err
	}
	return append(older, newer...), nil
}

// query fetches stories created in [from, to).
func (c *crawler) query(ctx context.Context, from, to int64, hitsPerPage int) (algoliaPage, error) {
	params := url.Values{
		"tags":                  {"story"},
		"numericFilters":        {fmt.Sprintf("points>=%d,created_at_i>=%d,created_at_i<%d", c.minPoints, from, to)},
		"hitsPerPage":           {strconv.Itoa(hitsPerPage)},
		"attributesToRetrieve":  {"objectID,title,url,points,num_comments,created_at_i"},
		"attributesToHighlight": {""},
	}
	var page algoliaPage
	var lastErr error
	for attempt := range 4 {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*attempt) * time.Second)
		}
		lastErr = c.get(ctx, algoliaURL+"?"+params.Encode(), &page)
		if lastErr == nil {
			break
		}
		log.Printf("algolia query %d-%d failed (attempt %d): %v", from, to, attempt+1, lastErr)
	}
	if lastErr != nil {
		return algoliaPage{}, lastErr
	}
	for i := range page.Hits {
		id, err := strconv.ParseInt(page.Hits[i].ObjectID, 10, 32)
		if err != nil {
			return algoliaPage{}, fmt.Errorf("bad objectID %q: %w", page.Hits[i].ObjectID, err)
		}
		page.Hits[i].ID = int32(id)
	}
	return page, nil
}

func (c *crawler) get(ctx context.Context, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "hn-feed (+https://github.com/Show-Hacker-News-Points)")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
