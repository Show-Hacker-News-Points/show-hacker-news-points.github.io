package main

import (
	"slices"
	"strings"
	"testing"
)

// Expected values come from running the bloomfilter module inside the v1 extension's bundle.js.
func TestFnv1aMatchesJavaScript(t *testing.T) {
	tests := []struct {
		in         string
		want       int32
		wantSeeded int32
	}{
		{"", 1493338014, -1554989545},
		{"/", -614126953, 1441528965},
		{"/watch", 1479303001, 1053450471},
		{"github.com/torvalds", -827858584, 1662365132},
		{"/ünïcödé/路径", 53750762, -242853927},
		{"😀", 187876127, -1846870398},
		{strings.Repeat("a", 300), 1522933536, -206636452},
	}
	for _, tt := range tests {
		if got := fnv1a(tt.in, 0); got != tt.want {
			t.Errorf("fnv1a(%q) = %d, want %d", tt.in, got, tt.want)
		}
		if got := fnv1a(tt.in, bloomSeed); got != tt.wantSeeded {
			t.Errorf("fnv1a(%q, seed) = %d, want %d", tt.in, got, tt.wantSeeded)
		}
	}
}

func TestBloomFilterMatchesJavaScript(t *testing.T) {
	b := newBloomFilter(250000, 13)
	if b.m != 250016 {
		t.Fatalf("m = %d, want 250016", b.m)
	}
	wantLocs := []int64{60397, 104534, 148671, 192808, 236945, 31066, 75203, 119340, 163477, 207614, 1735, 45872, 90009}
	if got := b.locations("ncase.me"); !slices.Equal(got, wantLocs) {
		t.Errorf("locations = %v, want %v", got, wantLocs)
	}

	for _, s := range []string{"ncase.me", "theverge.com", "github.com/duckduckgo"} {
		b.add(s)
	}
	wantSet := map[int]int32{
		54: 128, 120: 1048576, 641: 2097152, 970: 67108864, 985: 536870912, 1162: 4194304,
		1246: 65536, 1433: 65536, 1507: 8, 1683: 8388608, 1767: 4194304, 1887: 8192,
		2204: 16777216, 2350: 8, 2812: 33554432, 3266: 4194304, 3503: 4096, 3729: 4096,
		3763: -2147483648, 3765: 4096, 4024: 262144, 4285: 32, 4286: 8192, 4645: -2147483648,
		4807: 16384, 5108: 2097152, 5328: 32768, 5849: 65536, 6020: 134217728, 6025: 256,
		6281: 16384, 6370: 131072, 6487: 1073741824, 6542: 2, 6802: 1048576, 6891: 262144,
		7063: 128, 7404: 131072, 7412: 524288,
	}
	for i, v := range b.buckets {
		if v != wantSet[i] {
			t.Errorf("bucket %d = %d, want %d", i, v, wantSet[i])
		}
	}
	if !b.test("theverge.com") || b.test("example.com") {
		t.Error("test() disagrees with add()")
	}
}
