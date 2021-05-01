package bloomfilter

import (
	"encoding/binary"
	"math"
)

// GetBytes ...
func GetBytes(arg interface{}) []byte {
	switch v := arg.(type) {
	case int32:
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], uint32(v))
		return buf[:]
	case uint32:
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], v)
		return buf[:]
	case int64:
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], uint64(v))
		return buf[:]
	case uint64:
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], v)
		return buf[:]
	case int:
		if v >= math.MinInt32 && v <= math.MaxInt32 {
			var buf [4]byte
			binary.LittleEndian.PutUint32(buf[:], uint32(v))
			return buf[:]
		} else if v >= math.MinInt64 && v <= math.MaxInt64 {
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], uint64(v))
			return buf[:]
		} else {
			return []byte{}
		}
	case string:
		return []byte(v)
	case []byte:
		return v
	default:
		return []byte{}
	}
}
