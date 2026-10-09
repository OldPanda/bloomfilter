package bloomfilter

import (
	"encoding/binary"
	"errors"
	"math"
)

// ErrUnsupportedKey indicates a key type without a defined byte encoding.
// The error never includes the key's value.
var ErrUnsupportedKey = errors.New("unsupported BloomFilter key type")

// GetBytes encodes supported keys as documented by GetBytesChecked. It returns
// an empty byte slice for unsupported types; use GetBytesChecked to distinguish
// them from valid empty strings and byte slices.
func GetBytes(arg interface{}) []byte {
	encoded, err := GetBytesChecked(arg)
	if err != nil {
		return []byte{}
	}
	return encoded
}

// GetBytesChecked encodes int32/uint32 as four little-endian bytes and
// int64/uint64 as eight little-endian bytes. int uses four bytes when its value
// fits int32, otherwise eight. Signed integers use two's-complement encoding.
// Strings use their raw Go bytes; []byte is returned without copying, including
// nil and empty slices. Empty strings are valid. All other types (including a
// nil interface) return ErrUnsupportedKey without formatting or logging the key.
// Encodings have no type prefix, so different key types can encode identically.
func GetBytesChecked(arg interface{}) ([]byte, error) {
	switch v := arg.(type) {
	case int32:
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], uint32(v))
		return buf[:], nil
	case uint32:
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], v)
		return buf[:], nil
	case int64:
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], uint64(v))
		return buf[:], nil
	case uint64:
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], v)
		return buf[:], nil
	case int:
		if v >= math.MinInt32 && v <= math.MaxInt32 {
			var buf [4]byte
			binary.LittleEndian.PutUint32(buf[:], uint32(v))
			return buf[:], nil
		}
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], uint64(v))
		return buf[:], nil
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	default:
		return nil, ErrUnsupportedKey
	}
}
