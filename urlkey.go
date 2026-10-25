package main

import (
	"fmt"
	"net"
	"slices"
	"strings"

	"golang.org/x/net/idna"
)

// webURL holds the parts of an http(s) URL exactly as a browser's `new URL()` reports them,
// because the extension hashes url.hostname and url.pathname on the browser side.
type webURL struct {
	hostname string
	pathname string
	search   string // without the leading "?"
}

// parseWebURL follows the WHATWG URL parser for http and https closely enough for the
// links people submit to Hacker News. The verify step checks every feed URL against Node.
func parseWebURL(raw string) (webURL, error) {
	raw = strings.Trim(raw, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f ")
	raw = strings.NewReplacer("\t", "", "\n", "", "\r", "").Replace(raw)

	scheme, rest, ok := strings.Cut(raw, ":")
	scheme = strings.ToLower(scheme)
	if !ok || (scheme != "http" && scheme != "https") {
		return webURL{}, fmt.Errorf("not an http(s) URL: %q", raw)
	}
	rest = strings.TrimLeft(rest, `/\`)

	authorityEnd := strings.IndexAny(rest, `/\?#`)
	if authorityEnd < 0 {
		authorityEnd = len(rest)
	}
	hostname, err := parseHost(rest[:authorityEnd])
	if err != nil {
		return webURL{}, err
	}
	rest = rest[authorityEnd:]

	rest, _, _ = strings.Cut(rest, "#")
	path, query, hasQuery := strings.Cut(rest, "?")
	u := webURL{hostname: hostname, pathname: normalizePath(path)}
	if hasQuery {
		u.search = percentEncode(query, isQueryEncoded)
	}
	return u, nil
}

func parseHost(authority string) (string, error) {
	if i := strings.LastIndex(authority, "@"); i >= 0 {
		authority = authority[i+1:]
	}
	host := authority
	if strings.HasPrefix(host, "[") {
		end := strings.Index(host, "]")
		if end < 0 {
			return "", fmt.Errorf("bad IPv6 host %q", authority)
		}
		return strings.ToLower(host[:end+1]), nil
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" {
		return "", fmt.Errorf("empty host in %q", authority)
	}
	if isASCII(host) && !strings.Contains(host, "xn--") {
		return host, nil
	}
	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return "", fmt.Errorf("bad host %q: %w", host, err)
	}
	return ascii, nil
}

func normalizePath(path string) string {
	path = strings.ReplaceAll(path, `\`, "/")
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	var out []string
	for i, seg := range segments {
		last := i == len(segments)-1
		switch strings.ToLower(seg) {
		case ".", "%2e":
			if last {
				out = append(out, "")
			}
		case "..", ".%2e", "%2e.", "%2e%2e":
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
			if last {
				out = append(out, "")
			}
		default:
			out = append(out, percentEncode(seg, isPathEncoded))
		}
	}
	return "/" + strings.Join(out, "/")
}

func isPathEncoded(c byte) bool {
	return c < 0x20 || c > 0x7e || strings.IndexByte(" \"#<>?`{}", c) >= 0
}

func isQueryEncoded(c byte) bool {
	return c < 0x20 || c > 0x7e || strings.IndexByte(" \"#<>'", c) >= 0
}

func percentEncode(s string, shouldEncode func(byte) bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if shouldEncode(c) {
			fmt.Fprintf(&b, "%%%02X", c)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 0x7e {
			return false
		}
	}
	return true
}

// siteKeyV1 ports filterDomain from extension 1.1 and earlier: the hostname without "www.",
// plus the owner for github.com so each GitHub user or org counts as its own site.
func siteKeyV1(u webURL) string {
	key := strings.TrimPrefix(u.hostname, "www.")
	if u.hostname == "github.com" {
		key += "/" + firstSegment(u.pathname)
	}
	return key
}

func firstSegment(pathname string) string {
	seg, _, _ := strings.Cut(strings.TrimPrefix(pathname, "/"), "/")
	return seg
}

// hostAliases maps mobile and renamed hosts onto the host most HN submissions use.
var hostAliases = map[string]string{
	"x.com":              "twitter.com",
	"mobile.x.com":       "twitter.com",
	"mobile.twitter.com": "twitter.com",
	"m.youtube.com":      "youtube.com",
	"old.reddit.com":     "reddit.com",
	"new.reddit.com":     "reddit.com",
	"m.facebook.com":     "facebook.com",
}

// accountHosts host many unrelated authors, so the first path segment (the account) is
// part of the site. Otherwise every tweet would show the site's top story.
var accountHosts = []string{"github.com", "gitlab.com", "codeberg.org", "twitter.com", "medium.com"}

// v2Keys returns the site and the page within it that the v2 feed and extension 1.2 index by.
func v2Keys(u webURL) (site, page string) {
	host := strings.TrimPrefix(u.hostname, "www.")
	if alias, ok := hostAliases[host]; ok {
		host = alias
	}
	if wiki, ok := strings.CutSuffix(host, ".m.wikipedia.org"); ok {
		host = wiki + ".wikipedia.org"
	}
	pathname, search := u.pathname, u.search
	if host == "youtu.be" {
		host, pathname, search = "youtube.com", "/watch", "v="+firstSegment(u.pathname)
	}
	if host == "youtube.com" && pathname == "/watch" {
		search = onlyParam(search, "v")
	}
	if host == "twitter.com" {
		search = "" // tweets are identified by path; share links add ?s= and ?t=
	}

	site = host
	if slices.Contains(accountHosts, host) {
		// These hosts treat account and repository names case-insensitively.
		pathname = strings.ToLower(pathname)
		site += "/" + firstSegment(pathname)
	}
	page = strings.TrimSuffix(pathname, "/")
	if q := significantQuery(search); q != "" {
		page += "?" + q
	}
	return site, page
}

func onlyParam(search, name string) string {
	for _, pair := range strings.Split(search, "&") {
		if k, _, _ := strings.Cut(pair, "="); k == name {
			return pair
		}
	}
	return ""
}

// trackingParams never change which page a URL points to, so they are dropped from page keys.
var trackingParams = []string{
	"fbclid", "gclid", "dclid", "msclkid", "mc_cid", "mc_eid", "igshid",
	"ref", "ref_src", "ref_url", "si", "feature", "share", "smid", "cmpid",
}

func isTrackingParam(name string) bool {
	return strings.HasPrefix(name, "utm_") || slices.Contains(trackingParams, name)
}

// significantQuery keeps the query parameters that select a page, such as YouTube's v=,
// sorted so that parameter order does not matter.
func significantQuery(search string) string {
	var kept []string
	for _, pair := range strings.Split(search, "&") {
		name, _, _ := strings.Cut(pair, "=")
		if pair == "" || isTrackingParam(strings.ToLower(name)) {
			continue
		}
		kept = append(kept, pair)
	}
	slices.Sort(kept)
	return strings.Join(kept, "&")
}
