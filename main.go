// Command hn-feed builds the feeds that the Reveal Top Hacker News Stories extension
// downloads: every Hacker News story above a points threshold, indexed by site and page,
// so the extension can check the page you are reading without sending its URL anywhere.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	out := flag.String("out", "public", "directory to write feed.pb and v2/ into")
	minPoints := flag.Int("min-points", 400, "smallest story score to include")
	minStories := flag.Int("min-stories", 20000, "refuse to publish when the crawl finds fewer stories than this")
	storiesFile := flag.String("stories", "", "read stories from this JSON file instead of crawling, or write them there after a crawl with -save")
	save := flag.Bool("save", false, "write the crawled stories to -stories")
	flag.Parse()

	if err := run(*out, *minPoints, *minStories, *storiesFile, *save); err != nil {
		log.Fatal(err)
	}
}

func run(out string, minPoints, minStories int, storiesFile string, save bool) error {
	stories, err := loadStories(storiesFile, save, minPoints)
	if err != nil {
		return err
	}
	// A broken source must never replace a good feed with a near-empty one.
	if len(stories) < minStories {
		return fmt.Errorf("only %d stories found, below -min-stories %d; not publishing", len(stories), minStories)
	}
	log.Printf("%d stories with at least %d points", len(stories), minPoints)

	v2, err := buildFeedV2(stories, minPoints, time.Now())
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(out, "v2"), 0o755); err != nil {
		return err
	}
	files := map[string][]byte{
		"feed.pb":                           buildFeedV1(stories),
		filepath.Join("v2", v2.archiveName): v2.archive,
		filepath.Join("v2", "index.json"):   v2.index,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(out, name), data, 0o644); err != nil {
			return err
		}
		log.Printf("wrote %s (%d bytes)", name, len(data))
	}
	return nil
}

func loadStories(file string, save bool, minPoints int) ([]story, error) {
	if file != "" && !save {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		var stories []story
		err = json.Unmarshal(data, &stories)
		return stories, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c := &crawler{client: &http.Client{Timeout: 30 * time.Second}, minPoints: minPoints}
	stories, err := c.crawl(ctx)
	if err != nil {
		return nil, err
	}
	if save {
		data, err := json.Marshal(stories)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(file, data, 0o644); err != nil {
			return nil, err
		}
	}
	return stories, nil
}
