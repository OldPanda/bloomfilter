package bloomfilter

import (
	"bytes"
	"errors"
	"math"
	"os"
	"testing"
)

func TestCheckedKeyEncoding(t *testing.T) {
	for _, test := range []struct {
		key  interface{}
		want []byte
	}{
		{"", []byte{}},
		{[]byte{}, []byte{}},
		{[]byte(nil), []byte{}},
		{"雪", []byte{0xe9, 0x9b, 0xaa}},
		{"\xff", []byte{0xff}},
		{int32(-1), []byte{255, 255, 255, 255}},
		{uint32(math.MaxUint32), []byte{255, 255, 255, 255}},
		{int64(-1), []byte{255, 255, 255, 255, 255, 255, 255, 255}},
		{uint64(math.MaxUint64), []byte{255, 255, 255, 255, 255, 255, 255, 255}},
		{int(math.MaxInt32), []byte{255, 255, 255, 127}},
		{int(math.MinInt32), []byte{0, 0, 0, 128}},
	} {
		got, err := GetBytesChecked(test.key)
		if err != nil || !bytes.Equal(got, test.want) {
			t.Fatalf("encoding %T: got %x, error %v; want %x", test.key, got, err, test.want)
		}
		if !bytes.Equal(GetBytes(test.key), test.want) {
			t.Fatal("unchecked encoding differs from checked encoding")
		}
	}
}

func TestUnsupportedKeysAreDistinctFromEmptyKeys(t *testing.T) {
	type namedInt int64
	for _, strategy := range []Strategy{&Murur128Mitz32{}, &Murur128Mitz64{}} {
		bf, err := NewBloomFilterWithStrategy(500, .01, strategy)
		if err != nil {
			t.Fatal(err)
		}
		before := bf.ToBytes()
		for _, key := range []interface{}{nil, true, uint(1), int16(1), float64(1), namedInt(1), unloggableKey{}, map[string]string{"secret": "test-marker"}} {
			if encoded, err := GetBytesChecked(key); encoded != nil || !errors.Is(err, ErrUnsupportedKey) {
				t.Fatalf("unsupported %T did not return ErrUnsupportedKey", key)
			}
			if changed, err := bf.PutChecked(key); changed || !errors.Is(err, ErrUnsupportedKey) {
				t.Fatalf("checked insertion accepted unsupported %T", key)
			}
			if present, err := bf.MightContainChecked(key); present || !errors.Is(err, ErrUnsupportedKey) {
				t.Fatalf("checked lookup accepted unsupported %T", key)
			}
			if bf.Put(key) || bf.MightContain(key) || len(GetBytes(key)) != 0 {
				t.Fatalf("unchecked API changed unsupported-key behavior for %T", key)
			}
		}
		if !bytes.Equal(before, bf.ToBytes()) {
			t.Fatal("rejected keys changed the filter")
		}
		if present, err := bf.MightContainChecked(""); present || err != nil {
			t.Fatal("a valid empty key must be absent from an empty filter")
		}
	}
}

func TestEmptyKeysMatchGuava(t *testing.T) {
	for _, test := range []struct {
		name     string
		strategy Strategy
	}{
		{"mitz32", &Murur128Mitz32{}},
		{"mitz64", &Murur128Mitz64{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture, err := os.ReadFile("guava_dump_files/500_0_01_empty_" + test.name + ".dump")
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []interface{}{"", []byte{}, []byte(nil)} {
				bf, err := NewBloomFilterWithStrategy(500, .01, test.strategy)
				if err != nil {
					t.Fatal(err)
				}
				if !bf.Put(key) {
					t.Fatal("empty key did not change an empty filter")
				}
				if changed, err := bf.PutChecked(key); changed || err != nil {
					t.Fatal("reinserting a valid empty key must be unchanged, without an error")
				}
				if !bytes.Equal(fixture, bf.ToBytes()) {
					t.Fatal("empty-key serialization differs from Java Guava")
				}
				for _, decode := range []func([]byte) (*BloomFilter, error){FromBytes, FromLegacyBytes} {
					loaded, err := decode(fixture)
					if err != nil {
						t.Fatal(err)
					}
					for _, equivalent := range []interface{}{"", []byte{}, []byte(nil)} {
						if present, err := loaded.MightContainChecked(equivalent); !present || err != nil || !loaded.MightContain(equivalent) {
							t.Fatal("serialized empty key was lost")
						}
					}
				}
			}
		})
	}
}

func TestCheckedKeyAliases(t *testing.T) {
	bf, err := NewBloomFilter(500, .01)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := bf.PutChecked(int32(65)); !changed || err != nil {
		t.Fatal("checked insertion failed")
	}
	for _, key := range []interface{}{int(65), uint32(65), "A\x00\x00\x00", []byte{65, 0, 0, 0}} {
		if present, err := bf.MightContainChecked(key); !present || err != nil {
			t.Fatal("equivalent byte encodings no longer match")
		}
	}
}
