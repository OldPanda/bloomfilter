package bloomfilter

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func serializedFilter(strategy, hashes byte, words ...uint64) []byte {
	b := make([]byte, serializedHeaderSize+8*len(words))
	b[0], b[1] = strategy, hashes
	binary.BigEndian.PutUint32(b[2:serializedHeaderSize], uint32(len(words)))
	for i, word := range words {
		binary.BigEndian.PutUint64(b[serializedHeaderSize+i*8:], word)
	}
	return b
}

func TestFromBytesRejectsInvalidData(t *testing.T) {
	valid := serializedFilter(1, 1, 0)
	tests := []struct {
		name string
		data []byte
	}{
		{"unknown strategy", serializedFilter(2, 1, 0)},
		{"zero hashes strategy 32", serializedFilter(0, 0, 0)},
		{"zero hashes strategy 64", serializedFilter(1, 0, 0)},
		{"zero capacity strategy 32", serializedFilter(0, 1)},
		{"zero capacity strategy 64", serializedFilter(1, 1)},
		{"zero hashes and capacity", serializedFilter(1, 0)},
		{"trailing byte", append(serializedFilter(1, 1, 0), 0)},
		{"trailing word", append(serializedFilter(1, 1, 0), make([]byte, 8)...)},
	}
	for length := 0; length < len(valid); length++ {
		tests = append(tests, struct {
			name string
			data []byte
		}{fmt.Sprintf("truncated at byte %d", length), valid[:length]})
	}
	for _, words := range []uint32{2, 2 * 1024 * 1024, math.MaxInt32, math.MaxInt32 + 1, math.MaxUint32} {
		header := serializedFilter(1, 1)
		binary.BigEndian.PutUint32(header[2:], words)
		tests = append(tests, struct {
			name string
			data []byte
		}{fmt.Sprintf("header without %d declared words", words), header})
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bf, err := FromBytes(test.data)
			if err == nil || bf != nil {
				t.Fatalf("invalid data returned filter=%v, error=%v", bf, err)
			}
		})
	}
}

func TestFromBytesPreservesWordsAndValidState(t *testing.T) {
	// Include leading/trailing empty words and bits at word boundaries so the
	// decoder is checked independently of the serializer's iterator behavior.
	wordSets := [][]uint64{{0}, {0, 1 | uint64(1)<<63, 0}, {math.MaxUint64, 0, math.MaxUint64}}
	for _, strategy := range []byte{0, 1} {
		for _, hashes := range []byte{1, 255} {
			for i, words := range wordSets {
				t.Run(fmt.Sprintf("strategy %d hashes %d words %d", strategy, hashes, i), func(t *testing.T) {
					bf, err := FromBytes(serializedFilter(strategy, hashes, words...))
					if err != nil {
						t.Fatal(err)
					}
					if bf.numHashFunctions != int(hashes) || len(bf.array) != len(words) {
						t.Fatal("decoded header does not match the input")
					}
					for wordIdx, word := range words {
						if bf.array[wordIdx] != word {
							t.Fatalf("word %d: got %x, want %x", wordIdx, bf.array[wordIdx], word)
						}
					}
					if i == 0 && bf.MightContain("not inserted") {
						t.Fatal("empty filter must not report an uninserted key present")
					}
					bf.Put("inserted")
					if !bf.MightContain("inserted") {
						t.Fatal("decoded filter cannot find an inserted key")
					}
				})
			}
		}
	}
}

func TestFromBytesWithLimit(t *testing.T) {
	oneWord := serializedFilter(1, 1, 0)
	twoWords := serializedFilter(1, 1, 0, 1)
	tests := []struct {
		name  string
		data  []byte
		limit int
		valid bool
	}{
		{"negative limit", oneWord, -1, false},
		{"zero limit", oneWord, 0, false},
		{"limit below minimum filter", oneWord, len(oneWord) - 1, false},
		{"one word at limit", oneWord, len(oneWord), true},
		{"two words over limit", twoWords, len(twoWords) - 1, false},
		{"two words at limit", twoWords, len(twoWords), true},
		{"declared payload over limit", twoWords[:serializedHeaderSize], len(oneWord), false},
		{"truncated payload within limit", twoWords[:len(twoWords)-1], len(twoWords), false},
		{"maximum int limit with truncated payload", twoWords[:serializedHeaderSize], int(^uint(0) >> 1), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bf, err := FromBytesWithLimit(test.data, test.limit)
			if test.valid {
				if err != nil || bf == nil {
					t.Fatalf("valid data returned filter=%v, error=%v", bf, err)
				}
			} else if err == nil || bf != nil {
				t.Fatalf("invalid data returned filter=%v, error=%v", bf, err)
			}
		})
	}
}

func TestFromBytesEnforcesDefaultLimit(t *testing.T) {
	// The buffer is already supplied by the caller; rejection must not allocate
	// a second bit array or start decoding its declared words.
	b := make([]byte, DefaultMaxSerializedSize+6)
	b[0], b[1] = 1, 1
	binary.BigEndian.PutUint32(b[2:], DefaultMaxSerializedSize/8)
	if bf, err := FromBytes(b); err == nil || bf != nil {
		t.Fatalf("oversized input returned filter=%v, error=%v", bf, err)
	}
}
