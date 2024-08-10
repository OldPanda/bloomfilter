package bloomfilter

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestNumOfBits(t *testing.T) {
	type testCase struct {
		expectedInsertions int
		errRate            float64
		expectedNumBits    int
	}
	testCases := []testCase{{500, 0.01, 4792}, {500, 0.0, 774727}, {10, 0.01, 95}}
	var numBits int
	for _, item := range testCases {
		numBits = numOfBits(item.expectedInsertions, item.errRate)
		if numBits != item.expectedNumBits {
			t.Errorf("Expected %d bits, but got %d", item.expectedNumBits, numBits)
		}
	}
}

func TestNumOfHashFunctions(t *testing.T) {
	testCases := [][]int{{500, 4792, 7}, {500, 774727, 1074}}
	var numHashFunctions int
	for _, item := range testCases {
		numHashFunctions = numOfHashFunctions(item[0], item[1])
		if numHashFunctions != item[2] {
			t.Errorf("Expected %d hash functions, but got %d", item[2], numHashFunctions)
		}
	}
}

func TestBloomFilter(t *testing.T) {
	bf, err := NewBloomFilter(500, 0.01)
	if err != nil {
		t.Errorf("Unexpected error happened on initializing Bloomfilter: %v", err)
	}

	for i := 0; i < 100; i++ {
		bf.Put(i)
	}

	for i := 0; i < 100; i++ {
		if !bf.MightContain(i) {
			t.Errorf("Expected number %d in BloomFilter", i)
		}
	}
	for i := 200; i < 300; i++ {
		if bf.MightContain(i) {
			t.Errorf("Expected number %d not in Bloomfilter", i)
		}
	}

	bf2, err := NewBloomFilterWithStrategy(300, 0.01, &Murur128Mitz32{})
	if err != nil {
		t.Errorf("Unexpected error happened on initializing Bloomfilter with strategy: %v", err)
	}
	for i := 0; i < 100; i++ {
		bf2.Put(i)
	}

	for i := 0; i < 100; i++ {
		if !bf2.MightContain(i) {
			t.Errorf("Expected number %d in BloomFilter", i)
		}
	}
	for i := 200; i < 300; i++ {
		if bf2.MightContain(i) {
			t.Errorf("Expected number %d not in Bloomfilter", i)
		}
	}

	_, err = NewBloomFilterWithStrategy(0, 0.01, &Murur128Mitz32{})
	if err != nil {
		t.Errorf("Unexpected error happened on initializing Bloomfilter with strategy: %v", err)
	}
}

func TestBloomFilterWithUnusualKeys(t *testing.T) {
	bf, _ := NewBloomFilter(500, 0.01)
	if ok := bf.Put([]int{1, 2, 3}); ok {
		t.Error("Expected false result when putting an int array")
	}
	if ok := bf.MightContain([]int{1, 2, 3}); ok {
		t.Error("Expected false result when checking existence of an int array")
	}
}

func TestBloomFilterSerialization(t *testing.T) {
	bf, _ := NewBloomFilter(500, 0.01)
	for i := 0; i < 100; i++ {
		bf.Put(i)
	}

	bf2, err := FromBytes(bf.ToBytes())
	if err != nil {
		t.Error("Deserialization failed")
	}
	if !bytes.Equal(bf2.ToBytes(), bf.ToBytes()) {
		t.Error("Deserialize then serialize failed")
	}
	for i := 0; i < 100; i++ {
		if !bf2.MightContain(i) {
			t.Errorf("Expected number %d in BloomFilter2", i)
		}
	}
	for i := 100; i < 200; i++ {
		if bf2.MightContain(i) {
			t.Errorf("Expected number %d NOT in Bloomfilter2", i)
		}
	}

	bf3, _ := NewBloomFilterWithStrategy(300, 0.01, &Murur128Mitz32{})
	for i := 100; i < 200; i++ {
		bf3.Put(i)
	}

	bf4, err := FromBytes(bf3.ToBytes())
	if err != nil {
		t.Error("Deserialization failed")
	}
	if !bytes.Equal(bf3.ToBytes(), bf4.ToBytes()) {
		t.Error("Deserialize then serialize failed")
	}
	for i := 100; i < 200; i++ {
		if !bf4.MightContain(i) {
			t.Errorf("Expected number %d in BloomFilter4", i)
		}
	}
	for i := 0; i < 100; i++ {
		if bf4.MightContain(i) {
			t.Errorf("Expected number %d NOT in BloomFilter4", i)
		}
	}
}

func TestJavaCompatibility(t *testing.T) {
	file1, _ := os.Open("guava_dump_files/100_0_001_0_to_49_test.dump")
	defer file1.Close()
	b, _ := io.ReadAll(file1)
	bf1, err := FromBytes(b)
	if err != nil {
		t.Errorf("Deserialization from Guava dump file failed: %v", err)
	}
	for i := 0; i < 50; i++ {
		if !bf1.MightContain(i) {
			t.Errorf("Expected number %d in Bloomfilter1", i)
		}
	}
	for i := 50; i < 100; i++ {
		if bf1.MightContain(i) {
			t.Errorf("Expected number %d NOT in Bloomfilter1", i)
		}
	}

	file2, _ := os.Open("guava_dump_files/500_0_01_0_to_99_test.dump")
	defer file1.Close()
	b, _ = io.ReadAll(file2)
	bf2, err := FromBytes(b)
	if err != nil {
		t.Errorf("Deserialization from Guava dump file failed: %v", err)
	}
	for i := 0; i < 100; i++ {
		if !bf2.MightContain(i) {
			t.Errorf("Expected number %d in Bloomfilter2", i)
		}
	}
	for i := 100; i < 150; i++ {
		if bf2.MightContain(i) {
			t.Errorf("Expected number %d NOT in Bloomfilter1", i)
		}
	}
}

func TestGarbageByteArray(t *testing.T) {
	garbageBytes := [][]byte{
		[]byte("this-is-a-line-of-garbage"),
		{},
		{0},
		{0, 1},
		{0, 1, 2},
		{0, 1, 2, 3, 4, 5},
	}
	for _, bytes := range garbageBytes {
		_, err := FromBytes(bytes)
		if err == nil {
			t.Errorf("Expected error on garbage byte array: %v", bytes)
		}
	}
}

func TestBloomFilterInvalidParams(t *testing.T) {
	// too small error rate
	_, err := NewBloomFilter(500, 0.0)
	if err == nil {
		t.Error("Expected error on non-positive error rate.")
	}

	// too large error rate
	_, err = NewBloomFilter(500, 1.0)
	if err == nil {
		t.Error("Expected error on error rate larger than or equal to 1.0.")
	}

	// negative insertions
	_, err = NewBloomFilter(-1, 0.01)
	if err == nil {
		t.Error("Expected error on negative insertions.")
	}
}

var bf *BloomFilter

func BenchmarkBloomfilterInsertion(b *testing.B) {
	bf, _ = NewBloomFilter(b.N, 0.01)
	for i := 0; i < b.N; i++ {
		bf.Put(i)
	}
}

func BenchmarkBloomfilterQuery(b *testing.B) {
	for i := 0; i < b.N; i++ {
		bf.MightContain(i)
	}
}

var fileContent, _ = os.ReadFile("guava_dump_files/100_0_001_0_to_49_test.dump")

func BenchmarkBloomfilterDeserialization(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FromBytes(fileContent)
	}
}
