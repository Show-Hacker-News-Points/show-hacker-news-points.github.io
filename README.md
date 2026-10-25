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
go run . -out public -stories stories.json -max-age 0    # rebuild from a saved crawl
```

The command refuses to write anything when the crawl finds fewer than `-min-stories` (default 20000), or when its newest story is older than `-max-age` (default 48 hours), which means the source stopped indexing new stories. The previous robot kept publishing after its source broke, and shipped a 7.8 KB feed.

## Publishing

`.github/workflows/feed.yml` rebuilds every six hours and deploys `public/` to GitHub Pages with the Actions deploy. Nothing is committed, so the repository stays small.

## Operating the feed

The feed went stale for three years once without anyone noticing, so these are the signals to watch:

- **GitHub emails a failed "Build and publish feed" run.** Pages keeps serving the last good build. Rerun it from the Actions tab once the cause is fixed.
- **GitHub emails a failed "Check feed freshness" run.** The live `v2/index.json` is over a day old, which means builds have been failing or have stopped. Both scheduled workflows re-enable themselves on every build, including failed ones, so GitHub's 60-day inactivity rule doesn't silence them.
- **Rolling back a bad feed:** every build's artifact stays downloadable for 7 days under the run's Artifacts. Re-running an older successful run redeploys it.
- **Switching an extension feature off for everyone:** add its name to `disabled-features.txt` and push. The next deploy, minutes later, lists it in `index.json`, and the extension honours it on its next daily refresh. Remove the line to switch it back on.
- **Dependabot PRs** for Actions and Go modules arrive weekly and need merging, or the pinned action versions eventually stop running.
