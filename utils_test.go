package bloomfilter

import (
	"bytes"
	"testing"
)

func TestGetBytes(t *testing.T) {
	type testCase struct {
		value    interface{}
		expected []byte
	}
	cases := []testCase{
		{"hello", []byte{104, 101, 108, 108, 111}},
		{"bloomfilter", []byte{98, 108, 111, 111, 109, 102, 105, 108, 116, 101, 114}},
		{12345, []byte{57, 48, 0, 0}},
		{int32(12345), []byte{57, 48, 0, 0}},
		{int32(-12345), []byte{199, 207, 255, 255}},
		{3147483647, []byte{255, 201, 154, 187, 0, 0, 0, 0}},
		{int64(3147483647), []byte{255, 201, 154, 187, 0, 0, 0, 0}},
		{int64(-3147483647), []byte{1, 54, 101, 68, 255, 255, 255, 255}},
		{uint64(123456), []byte{64, 226, 1, 0, 0, 0, 0, 0}},
		{uint32(123456), []byte{64, 226, 1, 0}},
		{4611686018427387905, []byte{1, 0, 0, 0, 0, 0, 0, 64}},
		{[]byte{1, 2, 3}, []byte{1, 2, 3}},
		{[]int{1, 2, 3}, []byte{}},
	}

	for _, item := range cases {
		b := GetBytes(item.value)
		if !bytes.Equal(b, item.expected) {
			t.Errorf("Expected %v, got %v", item.expected, b)
		}
	}
}
