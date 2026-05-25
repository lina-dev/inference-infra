package preprocess

import (
	"encoding/binary"
	"math"
	"testing"
)

// encodeF32le writes floats as raw little-endian f32 bytes, mirroring ffmpeg -f f32le output.
func encodeF32le(vals []float32) []byte {
	buf := make([]byte, len(vals)*4)
	for i, v := range vals {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// Test 4: decodePCMf32le correctly recovers float32 values from f32le bytes.
func TestDecodePCMf32le_roundtrip(t *testing.T) {
	want := []float32{0.0, 0.5, -0.5, 1.0, -1.0, 0.123456}
	raw := encodeF32le(want)

	got, err := decodePCMf32le(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sample[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

// Test 5: decodePCMf32le rejects a byte slice whose length is not a multiple of 4.
func TestDecodePCMf32le_nonMultipleOf4(t *testing.T) {
	for _, badLen := range []int{1, 2, 3, 5, 7} {
		_, err := decodePCMf32le(make([]byte, badLen))
		if err == nil {
			t.Errorf("len=%d: expected error, got nil", badLen)
		}
	}
}
