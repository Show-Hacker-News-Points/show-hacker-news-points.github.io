package main

import (
	"log"

	"google.golang.org/protobuf/encoding/protowire"
)

// The v1 feed is read by extension 1.1.0 and earlier through protobufjs:
//
//	message Feed  { Bloom bloom = 1; map<string, Site> sites = 2; }
//	message Bloom { int32 m = 1; int32 k = 2; repeated int32 buckets = 3; }
//	message Site  { map<int32, Page> pages = 1; }
//	message Page  { int32 id = 1; int32 points = 2; }
//
// sites is keyed by siteKey. pages[0] is the site's top story; every other key is
// fnv1a(url.pathname) of a story's page.

type v1Page struct{ id, points int32 }

func buildFeedV1(stories []story) []byte {
	sites := map[string]map[int32]v1Page{}
	keep := func(pages map[int32]v1Page, key int32, s story) {
		if old, ok := pages[key]; !ok || s.Points > old.points {
			pages[key] = v1Page{s.ID, s.Points}
		}
	}
	skipped := 0
	for _, s := range stories {
		u, err := parseWebURL(s.URL)
		if err != nil {
			continue
		}
		site := siteKeyV1(u)
		if sites[site] == nil {
			sites[site] = map[int32]v1Page{}
		}
		keep(sites[site], 0, s)
		// v1 matches on the path alone, so a query-selected page such as /watch?v= would
		// light up every other page on the same path. Those pages only count for the site.
		if significantQuery(u.search) != "" {
			skipped++
			continue
		}
		keep(sites[site], fnv1a(u.pathname, 0), s)
	}
	log.Printf("v1 feed: %d sites, %d query-selected pages counted for their site only", len(sites), skipped)

	bloom := newBloomFilter(max(250000, int64(len(sites))*20), 13)
	for site := range sites {
		bloom.add(site)
	}

	var feed []byte
	feed = protowire.AppendTag(feed, 1, protowire.BytesType)
	feed = protowire.AppendBytes(feed, encodeBloom(bloom))
	for site, pages := range sites {
		var entry []byte
		entry = protowire.AppendTag(entry, 1, protowire.BytesType)
		entry = protowire.AppendString(entry, site)
		entry = protowire.AppendTag(entry, 2, protowire.BytesType)
		entry = protowire.AppendBytes(entry, encodeSite(pages))
		feed = protowire.AppendTag(feed, 2, protowire.BytesType)
		feed = protowire.AppendBytes(feed, entry)
	}
	return feed
}

func encodeBloom(b *bloomFilter) []byte {
	var out []byte
	out = appendInt32(out, 1, int32(b.m))
	out = appendInt32(out, 2, int32(b.k))
	var packed []byte
	for _, v := range b.buckets {
		packed = protowire.AppendVarint(packed, uint64(int64(v)))
	}
	out = protowire.AppendTag(out, 3, protowire.BytesType)
	return protowire.AppendBytes(out, packed)
}

func encodeSite(pages map[int32]v1Page) []byte {
	var out []byte
	for key, p := range pages {
		var page []byte
		page = appendInt32(page, 1, p.id)
		page = appendInt32(page, 2, p.points)
		var entry []byte
		entry = appendInt32(entry, 1, key)
		entry = protowire.AppendTag(entry, 2, protowire.BytesType)
		entry = protowire.AppendBytes(entry, page)
		out = protowire.AppendTag(out, 1, protowire.BytesType)
		out = protowire.AppendBytes(out, entry)
	}
	return out
}

// appendInt32 writes an int32 field the way protobuf does: negative values sign-extend to 10 bytes.
func appendInt32(b []byte, field protowire.Number, v int32) []byte {
	b = protowire.AppendTag(b, field, protowire.VarintType)
	return protowire.AppendVarint(b, uint64(int64(v)))
}
