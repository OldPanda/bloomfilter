package bloomfilter

import (
	"math"
)

// Strategy defines necessary functions for a strategy.
type Strategy interface {
	put(key interface{}, numHashFunctions int, array []uint64) bool
	mightContain(key interface{}, numHashFunctions int, array []uint64) bool
	getOrdinal() int
}

// Murur128Mitz32 is the implementation of Guava's MURMUR128_MITZ_32 class in Go.
// See https://github.com/google/guava/blob/master/guava/src/com/google/common/hash/BloomFilterStrategies.java#L45 for details.
type Murur128Mitz32 struct{}

func (m *Murur128Mitz32) put(key interface{}, numHashFunctions int, array []uint64) bool {
	return putMurmur32(key, numHashFunctions, array, 1)
}

func putMurmur32(key interface{}, numHashFunctions int, array []uint64, start int32) bool {
	bitSize := uint64(len(array)) * 64
	bytes, err := GetBytesChecked(key)
	if err != nil {
		return false
	}
	hash64, _ := murmur3Sum128(bytes)
	hash1 := int32(hash64)
	hash2 := int32(hash64 >> 32)

	bitsChanged := false
	for i := start; i < start+int32(numHashFunctions); i++ {
		combinedHash := hash1 + (i * hash2)
		if combinedHash < 0 {
			combinedHash = int32(uint32(combinedHash) ^ uint32(0xFFFFFFFF))
		}
		index := uint64(combinedHash) % bitSize
		word, mask := index/64, uint64(1)<<(index%64)
		if array[word]&mask == 0 {
			bitsChanged = true
			array[word] |= mask
		}
	}

	return bitsChanged
}

func (m *Murur128Mitz32) mightContain(key interface{}, numHashFunctions int, array []uint64) bool {
	return mightContainMurmur32(key, numHashFunctions, array, 1)
}

func mightContainMurmur32(key interface{}, numHashFunctions int, array []uint64, start int32) bool {
	bitSize := uint64(len(array)) * 64
	bytes, err := GetBytesChecked(key)
	if err != nil {
		return false
	}
	hash64, _ := murmur3Sum128(bytes)
	hash1 := int32(hash64)
	hash2 := int32(hash64 >> 32)

	for i := start; i < start+int32(numHashFunctions); i++ {
		combinedHash := hash1 + (i * hash2)
		if combinedHash < 0 {
			combinedHash = int32(uint32(combinedHash) ^ uint32(0xFFFFFFFF))
		}
		index := uint64(combinedHash) % bitSize
		if array[index/64]&(uint64(1)<<(index%64)) == 0 {
			return false
		}
	}

	return true
}

func (m *Murur128Mitz32) getOrdinal() int {
	return 0
}

// The old Go strategy used the same ordinal as Guava despite different loop
// bounds. There is no reliable way to identify it from serialized bytes alone.
type legacyMurur128Mitz32 struct{}

func (m *legacyMurur128Mitz32) put(key interface{}, numHashFunctions int, array []uint64) bool {
	return putMurmur32(key, numHashFunctions, array, 0)
}

func (m *legacyMurur128Mitz32) mightContain(key interface{}, numHashFunctions int, array []uint64) bool {
	return mightContainMurmur32(key, numHashFunctions, array, 0)
}

func (m *legacyMurur128Mitz32) getOrdinal() int { return 0 }

// Murur128Mitz64 is the implementation of Guava's MURMUR128_MITZ_64 class in Go.
// See https://github.com/google/guava/blob/master/guava/src/com/google/common/hash/BloomFilterStrategies.java#L93 for details.
type Murur128Mitz64 struct{}

func (m *Murur128Mitz64) put(key interface{}, numHashFunctions int, array []uint64) bool {
	bitSize := uint64(len(array)) * 64
	bytes, err := GetBytesChecked(key)
	if err != nil {
		return false
	}
	hash1, hash2 := murmur3Sum128(bytes)

	bitsChanged := false
	combinedHash := hash1
	var index uint64
	for i := 0; i < numHashFunctions; i++ {
		index = (combinedHash & math.MaxInt64) % bitSize
		word, mask := index/64, uint64(1)<<(index%64)
		if array[word]&mask == 0 {
			bitsChanged = true
			array[word] |= mask
		}
		combinedHash += hash2
	}

	return bitsChanged
}

func (m *Murur128Mitz64) mightContain(key interface{}, numHashFunctions int, array []uint64) bool {
	bitSize := uint64(len(array)) * 64
	bytes, err := GetBytesChecked(key)
	if err != nil {
		return false
	}
	hash1, hash2 := murmur3Sum128(bytes)

	combinedHash := hash1
	var index uint64
	for i := 0; i < numHashFunctions; i++ {
		index = (combinedHash & math.MaxInt64) % bitSize
		if array[index/64]&(uint64(1)<<(index%64)) == 0 {
			return false
		}
		combinedHash += hash2
	}

	return true
}

func (m *Murur128Mitz64) getOrdinal() int {
	return 1
}
