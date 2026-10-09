package bloomfilter

import (
	"encoding/binary"
	"math/bits"
)

// murmur3Sum128 returns the seed-zero MurmurHash3 x64_128 hash used by Guava.
// It implements Austin Appleby's public-domain reference algorithm:
// https://github.com/aappleby/smhasher/blob/master/src/MurmurHash3.cpp
// Explicit little-endian reads make the result independent of host byte order
// and slice alignment. The function does not modify or retain the input.
func murmur3Sum128(data []byte) (uint64, uint64) {
	const c1 uint64 = 0x87c37b91114253d5
	const c2 uint64 = 0x4cf5ad432745937f
	length := uint64(len(data))
	var h1, h2 uint64

	for len(data) >= 16 {
		k1 := binary.LittleEndian.Uint64(data[:8])
		k2 := binary.LittleEndian.Uint64(data[8:16])
		h1 ^= bits.RotateLeft64(k1*c1, 31) * c2
		h1 = bits.RotateLeft64(h1, 27)
		h1 += h2
		h1 = h1*5 + 0x52dce729

		h2 ^= bits.RotateLeft64(k2*c2, 33) * c1
		h2 = bits.RotateLeft64(h2, 31)
		h2 += h1
		h2 = h2*5 + 0x38495ab5
		data = data[16:]
	}

	// Pack the remaining 0..15 bytes without reading beyond the slice. Zero
	// bytes in either half have no effect when mixed, including an empty tail.
	var k1, k2 uint64
	for index, value := range data {
		if index < 8 {
			k1 |= uint64(value) << (uint(index) * 8)
		} else {
			k2 |= uint64(value) << (uint(index-8) * 8)
		}
	}
	h1 ^= bits.RotateLeft64(k1*c1, 31) * c2
	h2 ^= bits.RotateLeft64(k2*c2, 33) * c1

	h1 ^= length
	h2 ^= length
	h1 += h2
	h2 += h1
	h1 = murmur3Fmix64(h1)
	h2 = murmur3Fmix64(h2)
	h1 += h2
	h2 += h1
	return h1, h2
}

func murmur3Fmix64(value uint64) uint64 {
	value ^= value >> 33
	value *= 0xff51afd7ed558ccd
	value ^= value >> 33
	value *= 0xc4ceb9fe1a85ec53
	return value ^ (value >> 33)
}
