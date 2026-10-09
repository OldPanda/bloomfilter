//go:build go1.18
// +build go1.18

package bloomfilter

import (
	"bytes"
	"testing"
)

func FuzzFromBytesWithLimit(f *testing.F) {
	const budget = 4096
	for _, seed := range [][]byte{
		{}, {1}, {1, 1, 255, 255, 255, 255},
		serializedFilter(0, 1, 0), serializedFilter(1, 255, 0, 1<<63, 0),
		serializedFilter(1, 0, 0), serializedFilter(2, 1, 0),
		append(serializedFilter(1, 7, 1), 0),
	} {
		f.Add(seed, uint16(budget))
	}
	f.Add(serializedFilter(1, 1, 0), uint16(13))
	f.Add(serializedFilter(1, 1, 0), uint16(14))
	f.Fuzz(func(t *testing.T, input []byte, requestedLimit uint16) {
		// Keep allocations bounded independently of hostile headers and limits.
		limit := int(requestedLimit)
		if limit > budget {
			limit = budget
		}
		for _, decode := range []func([]byte, int) (*BloomFilter, error){FromBytesWithLimit, FromLegacyBytesWithLimit} {
			bf, err := decode(input, limit)
			if err != nil {
				if bf != nil {
					t.Fatal("rejected input returned a filter")
				}
				continue
			}
			if bf == nil || len(input) > limit || !bytes.Equal(input, bf.ToBytes()) {
				t.Fatal("accepted input exceeded its budget or changed during serialization")
			}
			key := []byte("fuzz roundtrip key")
			if _, err := bf.PutChecked(key); err != nil {
				t.Fatal(err)
			}
			bf.Put("")
			loaded, err := decode(bf.ToBytes(), limit)
			if err != nil {
				t.Fatal("modified filter cannot be reloaded:", err)
			}
			if present, err := loaded.MightContainChecked(key); !present || err != nil || !loaded.MightContain("") {
				t.Fatal("roundtrip lost an inserted key")
			}
		}
	})
}
