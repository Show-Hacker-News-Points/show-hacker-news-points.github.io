# hn-feed

Builds the story list for the Reveal Top Hacker News Stories extension: every Hacker News story with 400+ points, fetched from the Algolia HN API.

The extension downloads the whole list and checks pages on the device, so browsing never leaves the browser.

## Outputs

| File | Read by | Shape |
|---|---|---|
| `feed.pb` | extension 1.1 and earlier | protobuf + bloom filter keyed by `fnv_1a(pathname)` |
| `v2/index.json` | extension 1.2 and later, daily | stories from the last one to two months, plus the archive's name |
| `v2/archive-<hash>.json` | extension 1.2 and later, about monthly | older stories, whose points no longer change |

`urlkey.go` and the extension's `feed.js` must compute the same site and page keys. `testdata/keys.json` is shared by both test suites.

## Run

```sh
go test ./...
go run . -out public                                     # crawl Algolia (~1 min) and write all feeds
go run . -out public -stories stories.json -save         # also keep the crawl for offline rebuilds
go run . -out public -stories stories.json               # rebuild from a saved crawl
```

The command refuses to write anything when the crawl finds fewer than `-min-stories` (default 20000). The previous robot kept publishing after its source broke, and shipped a 7.8 KB feed.

## Publishing

`.github/workflows/feed.yml` rebuilds every six hours and deploys `public/` to GitHub Pages with the Actions deploy. Nothing is committed, so the repository stays small.
