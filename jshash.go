package main

import "unicode/utf16"

// fnv1a ports fnv_1a from the "bloomfilter" npm package that the v1 extension bundles.
// It is not standard FNV-1a: it hashes UTF-16 code units, folds in a seed and finishes
// with an extra mix, so the result must match the JavaScript bit for bit.
func fnv1a(s string, seed uint32) int32 {
	a := uint32(2166136261) ^ seed
	for _, c := range utf16.Encode([]rune(s)) {
		if hi := uint32(c) & 0xff00; hi != 0 {
			a = (a ^ hi>>8) * 16777619
		}
		a = (a ^ uint32(c)&0xff) * 16777619
	}
	a += a << 13
	a ^= a >> 7
	a += a << 3
	a ^= a >> 17
	a += a << 5
	return int32(a)
}

// bloomSeed is the second-hash seed hardcoded in the "bloomfilter" npm package.
const bloomSeed = 1576284489

type bloomFilter struct {
	m       int64
	k       int
	buckets []int32
}

func newBloomFilter(bits int64, k int) *bloomFilter {
	n := (bits + 31) / 32
	return &bloomFilter{m: n * 32, k: k, buckets: make([]int32, n)}
}

// locations mirrors BloomFilter.prototype.locations, including JavaScript's
// sign-preserving % on the running index.
func (b *bloomFilter) locations(v string) []int64 {
	a := int64(fnv1a(v, 0))
	step := int64(fnv1a(v, bloomSeed))
	x := a % b.m
	locs := make([]int64, b.k)
	for i := range locs {
		if x < 0 {
			locs[i] = x + b.m
		} else {
			locs[i] = x
		}
		x = (x + step) % b.m
	}
	return locs
}

func (b *bloomFilter) add(v string) {
	for _, l := range b.locations(v) {
		b.buckets[l/32] |= int32(uint32(1) << (l % 32))
	}
}

func (b *bloomFilter) test(v string) bool {
	for _, l := range b.locations(v) {
		if uint32(b.buckets[l/32])&(uint32(1)<<(l%32)) == 0 {
			return false
		}
	}
	return true
}
