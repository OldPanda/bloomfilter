package bloomfilter

// #cgo CFLAGS: -Wall
// #cgo LDFLAGS: -lm
// #include<math.h>
import "C"
import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/Workiva/go-datastructures/bitarray"
)

var strategyList []Strategy = []Strategy{&Murur128Mitz32{}, &Murur128Mitz64{}}

// DefaultMaxSerializedSize limits constructors and default deserializers to
// 64 MiB of serialized data, including the header.
const DefaultMaxSerializedSize = 64 << 20

const serializedHeaderSize = 6

// BloomFilter supports concurrent insertion, lookup, and serialization.
// A BloomFilter must never be copied, even before its first operation: copies
// share the bit array but have separate locks. Share the pointer returned by a
// constructor or deserializer instead. Its zero value is not usable.
type BloomFilter struct {
	mu               sync.RWMutex
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
	return NewBloomFilterWithStrategyAndLimit(expectedInsertions, errRate, strategy, DefaultMaxSerializedSize)
}

// NewBloomFilterWithLimit creates a filter using Murur128Mitz64 and an explicit
// maximum serialized size in bytes, including the header.
func NewBloomFilterWithLimit(expectedInsertions int, errRate float64, maxBytes int) (*BloomFilter, error) {
	return NewBloomFilterWithStrategyAndLimit(expectedInsertions, errRate, &Murur128Mitz64{}, maxBytes)
}

// NewBloomFilterWithStrategyAndLimit creates a filter with an explicit strategy
// and maximum serialized size. maxBytes must be at least 14. Configurations
// requiring zero bits or more than 255 hashes are rejected, as in Guava.
func NewBloomFilterWithStrategyAndLimit(expectedInsertions int, errRate float64, strategy Strategy, maxBytes int) (*BloomFilter, error) {
	if math.IsNaN(errRate) || math.IsInf(errRate, 0) {
		return nil, errors.New("error rate must be finite")
	}
	if errRate <= 0.0 {
		return nil, errors.New("error rate must be > 0.0")
	}
	if errRate >= 1.0 {
		return nil, errors.New("error rate must be < 1.0")
	}
	if expectedInsertions < 0 {
		return nil, errors.New("expected insertions must be >= 0")
	}
	if maxBytes < serializedHeaderSize+8 {
		return nil, errors.New("maximum serialized size must be at least 14 bytes")
	}
	switch s := strategy.(type) {
	case *Murur128Mitz32:
		if s == nil {
			return nil, errors.New("strategy must not be nil")
		}
	case *Murur128Mitz64:
		if s == nil {
			return nil, errors.New("strategy must not be nil")
		}
	default:
		return nil, errors.New("strategy must be Murur128Mitz32 or Murur128Mitz64")
	}
	if expectedInsertions == 0 {
		expectedInsertions = 1
	}
	// Bound the floating-point result before converting to int or allocating.
	// The serialized word count must fit Guava's positive signed 32-bit length.
	maxWords := uint64(maxBytes-serializedHeaderSize) / 8
	if maxWords > math.MaxInt32 {
		maxWords = math.MaxInt32
	}
	bits := numOfBitsFloat(expectedInsertions, errRate)
	if math.IsNaN(bits) || math.IsInf(bits, 0) || bits < 1 {
		return nil, errors.New("configuration must produce a positive finite bit count")
	}
	if bits >= float64(maxWords*64+1) || bits >= float64(int(^uint(0)>>1)) {
		return nil, errors.New("configuration exceeds the filter size limit")
	}
	numBits := int(bits)
	numHashFunctions := numOfHashFunctions(expectedInsertions, numBits)
	if numHashFunctions > 255 {
		return nil, errors.New("configuration requires more than 255 hash functions")
	}
	bloomFilter := &BloomFilter{
		numHashFunctions: numHashFunctions,
		array:            bitarray.NewBitArray((uint64(numBits) + 63) / 64 * 64),
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
	return fromBytes(b, maxBytes, false)
}

// FromLegacyBytes reads snapshots created by this Go library's former 32-bit
// strategy (hash loop starting at zero). It uses DefaultMaxSerializedSize.
// Use FromBytes for Java Guava data. Legacy snapshots must be rebuilt from
// their original keys before exchanging them with Guava.
func FromLegacyBytes(b []byte) (*BloomFilter, error) {
	return FromLegacyBytesWithLimit(b, DefaultMaxSerializedSize)
}

// FromLegacyBytesWithLimit reads legacy Go snapshots with an explicit maximum
// serialized size. Only strategy zero differs from the standard decoder.
func FromLegacyBytesWithLimit(b []byte, maxBytes int) (*BloomFilter, error) {
	return fromBytes(b, maxBytes, true)
}

func fromBytes(b []byte, maxBytes int, legacy bool) (*BloomFilter, error) {
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
	if legacy && strategyIndex == 0 {
		strategy = &legacyMurur128Mitz32{}
	}

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

// Put inserts a supported key, including empty strings and byte slices, and
// reports whether any bits changed. Unsupported key types return false.
// Use PutChecked to distinguish unsupported keys from an unchanged insertion.
func (bf *BloomFilter) Put(key interface{}) bool {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	return bf.strategy.put(key, bf.numHashFunctions, bf.array)
}

// PutChecked inserts a supported key and reports whether any bits changed.
// Unsupported key types return false and ErrUnsupportedKey without modifying
// the filter. Key encoding is documented by GetBytesChecked.
func (bf *BloomFilter) PutChecked(key interface{}) (bool, error) {
	encoded, err := GetBytesChecked(key)
	if err != nil {
		return false, err
	}
	return bf.Put(encoded), nil
}

// MightContain reports possible membership for supported keys. False means
// absence or an unsupported key type; true can be a false positive.
// Use MightContainChecked to distinguish unsupported keys from absence.
func (bf *BloomFilter) MightContain(key interface{}) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	return bf.strategy.mightContain(key, bf.numHashFunctions, bf.array)
}

// MightContainChecked reports possible membership for supported keys and
// returns ErrUnsupportedKey for other types. A true result can be a false
// positive. Key encoding is documented by GetBytesChecked.
func (bf *BloomFilter) MightContainChecked(key interface{}) (bool, error) {
	encoded, err := GetBytesChecked(key)
	if err != nil {
		return false, err
	}
	return bf.MightContain(encoded), nil
}

// ToBytes serializes BloomFilter to byte array, which is compatible with
// Java's Guava library. It takes a consistent snapshot of all capacity words,
// including leading and trailing zeros. It returns nil for invalid filter state.
// Filters loaded by FromLegacyBytes retain the legacy Go hashing semantics;
// their serialized strategy-zero data must not be used as Guava data.
func (bf *BloomFilter) ToBytes() []byte {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	if bf.numHashFunctions < 1 || bf.numHashFunctions > 255 || bf.array == nil || bf.strategy == nil {
		return nil
	}
	numWords := bf.array.Capacity() / 64
	if numWords == 0 || numWords > math.MaxInt32 || numWords > uint64(int(^uint(0)>>1)-serializedHeaderSize)/8 {
		return nil
	}
	b := make([]byte, serializedHeaderSize+int(numWords)*8)
	b[0], b[1] = byte(bf.strategy.getOrdinal()), byte(bf.numHashFunctions)
	binary.BigEndian.PutUint32(b[2:serializedHeaderSize], uint32(numWords))
	for iter := bf.array.Blocks(); iter.Next(); {
		index, block := iter.Value()
		offset := serializedHeaderSize + int(index)*8
		binary.BigEndian.PutUint64(b[offset:offset+8], uint64(block))
	}
	return b
}

func numOfBits(expectedInsertions int, errRate float64) int {
	return int(numOfBitsFloat(expectedInsertions, errRate))
}

func numOfBitsFloat(expectedInsertions int, errRate float64) float64 {
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
	return float64(C.double(-expectedInsertions) * C.log(errorRate) / (C.log(C.double(2.0)) * C.log(C.double(2.0))))
}

func numOfHashFunctions(expectedInsertions int, numBits int) int {
	// See numOfBits for the compatibility constraints on replacing C math calls.
	return int(math.Max(1.0, float64(C.round(C.double(numBits)/C.double(expectedInsertions)*C.log(C.double(2.0))))))
}
