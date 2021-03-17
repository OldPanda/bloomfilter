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
		{3147483647, []byte{255, 201, 154, 187, 0, 0, 0, 0}},
		{uint64(123456), []byte{64, 226, 1, 0, 0, 0, 0, 0}},
		{uint32(123456), []byte{64, 226, 1, 0}},
	}

	for _, item := range cases {
		b := GetBytes(item.value)
		if !bytes.Equal(b, item.expected) {
			t.Errorf("Expected %v, got %v", item.expected, b)
		}
	}
}
