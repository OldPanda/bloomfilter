package bloomfilter

import (
	"log"
	"math"

	"github.com/Workiva/go-datastructures/bitarray"
	"github.com/spaolacci/murmur3"
)

// Strategy defines necessary functions for a strategy.
type Strategy interface {
	put(key interface{}, numHashFunctions int, array bitarray.BitArray) bool
	mightContain(key interface{}, numHashFunctions int, array bitarray.BitArray) bool
	getOrdinal() int
}

// Murur128Mitz32 is the implementation of Guava's MURMUR128_MITZ_32 class in Go.
// See https://github.com/google/guava/blob/master/guava/src/com/google/common/hash/BloomFilterStrategies.java#L45 for details.
type Murur128Mitz32 struct{}

func (m *Murur128Mitz32) put(key interface{}, numHashFunctions int, array bitarray.BitArray) bool {
	bitSize := array.Capacity()
	bytes := GetBytes(key)
	if len(bytes) == 0 {
		log.Printf("Failed to convert %v to byte array\n", key)
		return false
	}
	hash64, _ := murmur3.Sum128(bytes)
	hash1 := int32(hash64)
	hash2 := int32(hash64 >> 32)

	bitsChanged := false
	var i int32 = 0
	for ; i < int32(numHashFunctions); i++ {
		combinedHash := hash1 + (i * hash2)
		if combinedHash < 0 {
			combinedHash = int32(uint32(combinedHash) ^ uint32(0xFFFFFFFF))
		}
		index := uint64(combinedHash) % bitSize
		result, _ := array.GetBit(index)
		if !result {
			bitsChanged = true
			array.SetBit(index)
		}
	}

	return bitsChanged
}

func (m *Murur128Mitz32) mightContain(key interface{}, numHashFunctions int, array bitarray.BitArray) bool {
	bitSize := array.Capacity()
	bytes := GetBytes(key)
	if len(bytes) == 0 {
		log.Printf("Failed to convert %v to byte array\n", key)
		return false
	}
	hash64, _ := murmur3.Sum128(bytes)
	hash1 := int32(hash64)
	hash2 := int32(hash64 >> 32)

	var i int32 = 0
	for ; i < int32(numHashFunctions); i++ {
		combinedHash := hash1 + (i * hash2)
		if combinedHash < 0 {
			combinedHash = int32(uint32(combinedHash) ^ uint32(0xFFFFFFFF))
		}
		index := uint64(combinedHash) % bitSize
		result, _ := array.GetBit(index)
		if !result {
			return false
		}
	}

	return true
}

func (m *Murur128Mitz32) getOrdinal() int {
	return 0
}

// Murur128Mitz64 is the implementation of Guava's MURMUR128_MITZ_64 class in Go.
// See https://github.com/google/guava/blob/master/guava/src/com/google/common/hash/BloomFilterStrategies.java#L93 for details.
type Murur128Mitz64 struct{}

func (m *Murur128Mitz64) put(key interface{}, numHashFunctions int, array bitarray.BitArray) bool {
	bitSize := array.Capacity()
	bytes := GetBytes(key)
	if len(bytes) == 0 {
		log.Printf("Failed to convert %v to byte array\n", key)
		return false
	}
	hash1, hash2 := murmur3.Sum128(bytes)

	bitsChanged := false
	combinedHash := hash1
	var index uint64
	for i := 0; i < numHashFunctions; i++ {
		index = (combinedHash & math.MaxInt64) % bitSize
		result, _ := array.GetBit(index)
		if !result {
			bitsChanged = true
			array.SetBit(index)
		}
		combinedHash += hash2
	}

	return bitsChanged
}

func (m *Murur128Mitz64) mightContain(key interface{}, numHashFunctions int, array bitarray.BitArray) bool {
	bitSize := array.Capacity()
	bytes := GetBytes(key)
	if len(bytes) == 0 {
		log.Printf("Failed to convert %v to byte array\n", key)
		return false
	}
	hash1, hash2 := murmur3.Sum128(bytes)

	combinedHash := hash1
	var index uint64
	for i := 0; i < numHashFunctions; i++ {
		index = (combinedHash & math.MaxInt64) % bitSize
		result, _ := array.GetBit(index)
		if !result {
			return false
		}
		combinedHash += hash2
	}

	return true
}

func (m *Murur128Mitz64) getOrdinal() int {
	return 1
}
