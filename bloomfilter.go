package bloomfilter

// #cgo CFLAGS: -Wall
// #include<math.h>
import "C"
import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"

	"github.com/Workiva/go-datastructures/bitarray"
)

var strategyList []Strategy = []Strategy{&Murur128Mitz32{}, &Murur128Mitz64{}}

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
		return nil, errors.New("Error rate must be > 0.0")
	}
	if errRate >= 1.0 {
		return nil, errors.New("Error rate must be < 1.0")
	}
	if expectedInsertions < 0 {
		return nil, errors.New("Expected insertions must be >= 0")
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

// FromBytes creates a new BloomFilter instance by deserializing from byte array.
func FromBytes(b []byte) (*BloomFilter, error) {
	reader := bytes.NewReader(b)
	// read strategy
	strategyByte, err := reader.ReadByte()
	if err != nil {
		return nil, errors.New("Failed to read strategy")
	}
	strategy := strategyList[int(strategyByte)]

	// read number of hash functions
	numHashFuncByte, err := reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("Failed to read number of hash functions: %v", err)
	}
	numHashFunctions := int(numHashFuncByte)

	// read bitarray capacity
	numUint64Bytes, err := ioutil.ReadAll(io.LimitReader(reader, 4))
	if err != nil {
		return nil, fmt.Errorf("Failed to read number of bits: %v", err)
	}
	numUint64 := binary.BigEndian.Uint32(numUint64Bytes)
	array := bitarray.NewBitArray(uint64(numUint64) * 64)

	// put blocks back to bitarray
	for blockIdx := 0; blockIdx < int(numUint64); blockIdx++ {
		block, err := ioutil.ReadAll(io.LimitReader(reader, 8))
		if err != nil {
			return nil, fmt.Errorf("Failed to build bitarray: %v", err)
		}
		num := binary.BigEndian.Uint64(block)
		var pos uint64 = 1 << 63
		var index uint64
		for i := 0; i < 64; i++ {
			if num&pos > 0 {
				index = uint64(blockIdx*64 + (64 - i - 1))
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
	// Use C functions to calculate logarithm here since Go's built-in math lib doesn't give accurate result.
	// See https://github.com/golang/go/issues/9546 for details.
	return int(C.double(-expectedInsertions) * C.log(errorRate) / (C.log(C.double(2.0)) * C.log(C.double(2.0))))
}

func numOfHashFunctions(expectedInsertions int, numBits int) int {
	return int(math.Max(1.0, float64(C.round(C.double(numBits)/C.double(expectedInsertions)*C.log(C.double(2.0))))))
}
