package bloomfilter

// #cgo CFLAGS: -Wall
// #cgo LDFLAGS: -lm
// #include<math.h>
import "C"
import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/Workiva/go-datastructures/bitarray"
)

var strategyList []Strategy = []Strategy{&Murur128Mitz32{}, &Murur128Mitz64{}}

// DefaultMaxSerializedSize limits the serialized input accepted by FromBytes
// to 64 MiB, including its header.
const DefaultMaxSerializedSize = 64 << 20

const serializedHeaderSize = 6

// BloomFilter definition includes the number of hash functions, bit array, and strategy for hashing.
type BloomFilter struct {
	numHashFunctions int
	array            bitarray.BitArray
	strategy         Strategy
}

// NewBloomFilter creates a new BloomFilter instance with `Murur128Mitz64` as default strategy.
func NewBloomFilter(expectedInsertions int, errRate float64) (*BloomFilter, error) {
	return NewBloomFilterWithStrategy(expectedInsertions, errRate, &Murur128Mitz64{})
}

// NewBloomFilterWithStrategy creates a new BloomFilter instance with given expected insertions/capacity,
// error rate, and strategy. For now the available strategies are
// * &Murur128Mitz32{}
// * &Murur128Mitz64{}
func NewBloomFilterWithStrategy(expectedInsertions int, errRate float64, strategy Strategy) (*BloomFilter, error) {
	if errRate <= 0.0 {
		return nil, errors.New("error rate must be > 0.0")
	}
	if errRate >= 1.0 {
		return nil, errors.New("error rate must be < 1.0")
	}
	if expectedInsertions < 0 {
		return nil, errors.New("expected insertions must be >= 0")
	}
	if expectedInsertions == 0 {
		expectedInsertions = 1
	}
	numBits := numOfBits(expectedInsertions, errRate)
	numHashFunctions := numOfHashFunctions(expectedInsertions, numBits)
	bloomFilter := &BloomFilter{
		numHashFunctions: numHashFunctions,
		array:            bitarray.NewBitArray(uint64(math.Ceil(float64(numBits)/64.0) * 64.0)),
		strategy:         strategy,
	}

	return bloomFilter, nil
}

// FromBytes creates a new BloomFilter instance from a complete serialized byte
// array of at most DefaultMaxSerializedSize bytes. It rejects invalid headers,
// truncated payloads, and trailing bytes before allocating the bit array.
func FromBytes(b []byte) (*BloomFilter, error) {
	return FromBytesWithLimit(b, DefaultMaxSerializedSize)
}

// FromBytesWithLimit deserializes a complete BloomFilter with an explicit maximum
// serialized size in bytes, including its header. maxBytes must be at least 14
// (the header and one word). Choose a limit that fits the application's memory
// budget; the bit array requires additional memory approximately equal to the
// payload size. The word count must fit Guava's positive signed 32-bit length.
func FromBytesWithLimit(b []byte, maxBytes int) (*BloomFilter, error) {
	if maxBytes < serializedHeaderSize+8 {
		return nil, errors.New("maximum serialized size must be at least 14 bytes")
	}
	if len(b) > maxBytes {
		return nil, fmt.Errorf("serialized size %d exceeds limit %d", len(b), maxBytes)
	}
	if len(b) < serializedHeaderSize {
		return nil, errors.New("serialized BloomFilter header must contain 6 bytes")
	}

	strategyIndex := int(b[0])
	if strategyIndex >= len(strategyList) {
		return nil, fmt.Errorf("unknown strategy byte: %v", b[0])
	}
	strategy := strategyList[strategyIndex]

	numHashFunctions := int(b[1])
	if numHashFunctions == 0 {
		return nil, errors.New("number of hash functions must be between 1 and 255")
	}

	numUint64 := binary.BigEndian.Uint32(b[2:serializedHeaderSize])
	if numUint64 == 0 || numUint64 > math.MaxInt32 {
		return nil, errors.New("number of bit array words must be between 1 and 2147483647")
	}
	// Calculate in uint64 so hostile word counts cannot overflow an int. Exact
	// length validation also makes the subsequent offsets safe on 32-bit hosts.
	requiredSize := uint64(serializedHeaderSize) + uint64(numUint64)*8
	if requiredSize > uint64(maxBytes) {
		return nil, fmt.Errorf("declared serialized size %d exceeds limit %d", requiredSize, maxBytes)
	}
	if requiredSize != uint64(len(b)) {
		return nil, fmt.Errorf("invalid serialized size: expected %d bytes, got %d", requiredSize, len(b))
	}

	array := bitarray.NewBitArray(uint64(numUint64) * 64)

	// put blocks back to bitarray
	for blockIdx := 0; blockIdx < int(numUint64); blockIdx++ {
		offset := serializedHeaderSize + blockIdx*8
		num := binary.BigEndian.Uint64(b[offset : offset+8])
		var pos uint64 = 1 << 63
		var index uint64
		for i := 0; i < 64; i++ {
			if num&pos > 0 {
				index = uint64(blockIdx)*64 + uint64(64-i-1)
				array.SetBit(index)
			}
			pos >>= 1
		}
	}

	return &BloomFilter{
		numHashFunctions: numHashFunctions,
		array:            array,
		strategy:         strategy,
	}, nil
}

// Put inserts element of any type into BloomFilter.
func (bf *BloomFilter) Put(key interface{}) bool {
	return bf.strategy.put(key, bf.numHashFunctions, bf.array)
}

// MightContain returns a boolean value to indicate if given element is in BloomFilter.
func (bf *BloomFilter) MightContain(key interface{}) bool {
	return bf.strategy.mightContain(key, bf.numHashFunctions, bf.array)
}

// ToBytes serializes BloomFilter to byte array, which is compatible with
// Java's Guava library.
func (bf *BloomFilter) ToBytes() []byte {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(bf.strategy.getOrdinal()))
	buf.WriteByte(byte(bf.numHashFunctions))
	binary.Write(buf, binary.BigEndian, uint32(math.Ceil(float64(bf.array.Capacity())/64.0)))
	for iter := bf.array.Blocks(); iter.Next(); {
		_, block := iter.Value()
		binary.Write(buf, binary.BigEndian, block)
	}
	return buf.Bytes()
}

func numOfBits(expectedInsertions int, errRate float64) int {
	if errRate == 0.0 {
		errRate = math.Pow(2.0, -1074.0) // the same number of Double.MIN_VALUE in Java
	}
	errorRate := C.double(errRate)
	// Retain C.log to preserve the existing sizing behavior. Go's math.Log is
	// not guaranteed to match Java's Math.log bit for bit: a last-bit difference
	// can change this truncated bit count and the allocation's 64-bit block
	// count, changing hash positions and serialized bytes.
	// C.log also depends on the platform's libm and does not guarantee exact
	// Guava compatibility across platforms. Removing cgo requires an explicit
	// Guava/JVM target and exact comparisons of sizing and hash counts, including
	// truncation and rounding boundaries.
	// Go issue #9546 was closed as invalid, not fixed; it does not establish
	// bit-identical results. See https://github.com/OldPanda/bloomfilter/issues/24.
	return int(C.double(-expectedInsertions) * C.log(errorRate) / (C.log(C.double(2.0)) * C.log(C.double(2.0))))
}

func numOfHashFunctions(expectedInsertions int, numBits int) int {
	// See numOfBits for the compatibility constraints on replacing C math calls.
	return int(math.Max(1.0, float64(C.round(C.double(numBits)/C.double(expectedInsertions)*C.log(C.double(2.0))))))
}
