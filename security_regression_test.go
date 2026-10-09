package bloomfilter

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"os"
	"sync"
	"testing"
)

func TestConstructorSecurityBounds(t *testing.T) {
	var nil32 *Murur128Mitz32
	var nil64 *Murur128Mitz64
	tests := []struct {
		name     string
		n        int
		p        float64
		strategy Strategy
		limit    int
	}{
		{"NaN", 1, math.NaN(), &Murur128Mitz64{}, DefaultMaxSerializedSize},
		{"positive infinity", 1, math.Inf(1), &Murur128Mitz64{}, DefaultMaxSerializedSize},
		{"negative infinity", 1, math.Inf(-1), &Murur128Mitz64{}, DefaultMaxSerializedSize},
		{"zero bits", 1, .99, &Murur128Mitz64{}, DefaultMaxSerializedSize},
		{"nil strategy", 10, .01, nil, DefaultMaxSerializedSize},
		{"typed nil 32", 10, .01, nil32, DefaultMaxSerializedSize},
		{"typed nil 64", 10, .01, nil64, DefaultMaxSerializedSize},
		{"256 hashes", 10, math.Exp2(-256), &Murur128Mitz64{}, DefaultMaxSerializedSize},
		{"smallest positive rate", 10, math.SmallestNonzeroFloat64, &Murur128Mitz64{}, DefaultMaxSerializedSize},
		{"maximum insertions", int(^uint(0) >> 1), .01, &Murur128Mitz64{}, DefaultMaxSerializedSize},
		{"invalid limit", 10, .01, &Murur128Mitz64{}, 13},
		{"one byte short", 500, .01, &Murur128Mitz64{}, 605},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bf, err := NewBloomFilterWithStrategyAndLimit(test.n, test.p, test.strategy, test.limit)
			if err == nil || bf != nil {
				t.Fatalf("unsafe parameters returned filter=%v error=%v", bf, err)
			}
		})
	}
	for _, strategy := range []Strategy{&Murur128Mitz32{}, &Murur128Mitz64{}} {
		bf, err := NewBloomFilterWithStrategyAndLimit(500, .01, strategy, 606)
		if err != nil || len(bf.ToBytes()) != 606 {
			t.Fatalf("exact size budget failed: %v", err)
		}
		bf, err = NewBloomFilterWithStrategy(10, math.Exp2(-255), strategy)
		if err != nil || bf.numHashFunctions != 255 {
			t.Fatalf("255-hash boundary failed: %v", err)
		}
		bf.Put("present")
		loaded, err := FromBytes(bf.ToBytes())
		if err != nil || !loaded.MightContain("present") || loaded.numHashFunctions != 255 {
			t.Fatalf("255-hash roundtrip failed: %v", err)
		}
	}
	if _, err := NewBloomFilterWithLimit(500, .01, 606); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteSerialization(t *testing.T) {
	for _, strategy := range []byte{0, 1} {
		for i, words := range [][]uint64{{0}, {0, 0, 0}, {0, 1 << 63, 0}, {0, 1, 0, 1 << 63, 0}} {
			t.Run(fmt.Sprintf("strategy %d sparse %d", strategy, i), func(t *testing.T) {
				input := serializedFilter(strategy, 7, words...)
				bf, err := FromBytes(input)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(input, bf.ToBytes()) {
					t.Fatal("serialization changed empty words or their positions")
				}
			})
		}
	}
	bf, _ := NewBloomFilter(500, .01)
	if _, err := FromBytes(bf.ToBytes()); err != nil {
		t.Fatalf("new empty filter cannot be deserialized: %v", err)
	}
	bf.numHashFunctions = 256
	if bf.ToBytes() != nil {
		t.Fatal("invalid hash count must not be truncated and serialized")
	}
	if (&BloomFilter{}).ToBytes() != nil {
		t.Fatal("zero-value filter must not be serialized")
	}
}

func TestConcurrentFilterOperations(t *testing.T) {
	for _, strategy := range []Strategy{&Murur128Mitz32{}, &Murur128Mitz64{}} {
		bf, err := NewBloomFilterWithStrategy(2048, .001, strategy)
		if err != nil {
			t.Fatal(err)
		}
		bf.Put("before workers")
		var wg sync.WaitGroup
		start := make(chan struct{})
		for worker := 0; worker < 8; worker++ {
			wg.Add(1)
			go func(worker int) {
				defer wg.Done()
				<-start
				for i := 0; i < 128; i++ {
					key := worker*128 + i
					bf.Put(key)
					if !bf.MightContain(key) {
						t.Errorf("inserted key %d was lost", key)
					}
					if i%16 == 0 {
						snapshot, err := FromBytes(bf.ToBytes())
						if err != nil || !snapshot.MightContain("before workers") || !snapshot.MightContain(key) {
							t.Errorf("inconsistent snapshot: %v", err)
						}
					}
				}
			}(worker)
		}
		close(start)
		wg.Wait()
		for key := 0; key < 1024; key++ {
			if !bf.MightContain(key) {
				t.Errorf("completed writers lost key %d", key)
			}
		}
	}
}

type unloggableKey struct{}

func (unloggableKey) String() string { panic("key must not be formatted") }

func TestRejectedKeysAreNotLogged(t *testing.T) {
	var output bytes.Buffer
	oldOutput := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(oldOutput)
	for _, strategy := range []Strategy{&Murur128Mitz32{}, &Murur128Mitz64{}} {
		bf, _ := NewBloomFilterWithStrategy(10, .01, strategy)
		for _, key := range []interface{}{unloggableKey{}, map[string]string{"credential": "test-marker"}, nil} {
			if bf.Put(key) || bf.MightContain(key) {
				t.Fatal("unsupported key must return false")
			}
		}
	}
	if output.Len() != 0 {
		t.Fatal("library wrote rejected key data to the process logger")
	}
}

func TestGuavaBothStrategies(t *testing.T) {
	for _, test := range []struct {
		name     string
		strategy Strategy
	}{
		{"mitz32", &Murur128Mitz32{}},
		{"mitz64", &Murur128Mitz64{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, err := os.ReadFile("guava_dump_files/500_0_01_0_to_99_" + test.name + ".dump")
			if err != nil {
				t.Fatal(err)
			}
			bf, err := FromBytes(data)
			if err != nil {
				t.Fatal(err)
			}
			created, _ := NewBloomFilterWithStrategy(500, .01, test.strategy)
			for i := 0; i < 100; i++ {
				if !bf.MightContain(i) {
					t.Fatalf("Guava inserted key %d is absent", i)
				}
				created.Put(i)
			}
			if !bytes.Equal(data, bf.ToBytes()) || !bytes.Equal(data, created.ToBytes()) {
				t.Fatal("Go serialization differs from the Java fixture")
			}
		})
	}
}

func TestLegacyGoStrategyMigration(t *testing.T) {
	// The historical Go loop places this key at bit 51; Guava places it at bit 0.
	data := serializedFilter(0, 1, uint64(1)<<51)
	legacy, err := FromLegacyBytes(data)
	if err != nil || !legacy.MightContain("security-audit-key") {
		t.Fatalf("legacy snapshot lost its inserted key: %v", err)
	}
	legacy.Put("new legacy key")
	loaded, err := FromLegacyBytes(legacy.ToBytes())
	if err != nil || !loaded.MightContain("security-audit-key") || !loaded.MightContain("new legacy key") {
		t.Fatalf("legacy snapshot roundtrip failed: %v", err)
	}
	guava, _ := FromBytes(data)
	if guava.MightContain("security-audit-key") {
		t.Fatal("standard decoder must use the Guava loop, not legacy hashing")
	}
	for _, invalid := range [][]byte{data[:6], serializedFilter(0, 0, 0), serializedFilter(0, 1), append(data, 0)} {
		if bf, err := FromLegacyBytes(invalid); err == nil || bf != nil {
			t.Fatal("legacy decoder bypassed structural validation")
		}
	}
	if bf, err := FromLegacyBytesWithLimit(data, 13); err == nil || bf != nil {
		t.Fatal("legacy decoder bypassed its size limit")
	}
}
