package chunker

import (
	"testing"

	"github.com/inference-infra/worker/internal/preprocess"
)

const sr = 16000 // samples per second

func audio(samples []float32) preprocess.Audio {
	return preprocess.Audio{Samples: samples, SampleRate: sr}
}

func makeSamples(n int) []float32 {
	s := make([]float32, n)
	for i := range s {
		s[i] = float32(i)
	}
	return s
}

// Test 1: audio length is an exact multiple of durationSec — all chunks same size.
func TestSplit_evenDivision(t *testing.T) {
	samples := makeSamples(3 * sr) // 3 seconds
	chunks, err := Split(audio(samples), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if c.Index != i {
			t.Errorf("chunk %d: Index = %d", i, c.Index)
		}
		if len(c.Data) != sr {
			t.Errorf("chunk %d: len(Data) = %d, want %d", i, len(c.Data), sr)
		}
		wantStart := float64(i)
		wantEnd := float64(i + 1)
		if c.StartSec != wantStart || c.EndSec != wantEnd {
			t.Errorf("chunk %d: time = [%.2f, %.2f], want [%.2f, %.2f]",
				i, c.StartSec, c.EndSec, wantStart, wantEnd)
		}
		// Data must be a slice of the original backing array, not a copy.
		if &c.Data[0] != &samples[i*sr] {
			t.Errorf("chunk %d: Data is not a slice of the original array", i)
		}
	}
}

// Test 2: audio length is not a multiple of durationSec — last chunk is shorter.
func TestSplit_unevenDivision(t *testing.T) {
	half := sr / 2
	samples := makeSamples(2*sr + half) // 2.5 seconds
	chunks, err := Split(audio(samples), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if len(chunks[0].Data) != sr {
		t.Errorf("chunk 0: len = %d, want %d", len(chunks[0].Data), sr)
	}
	if len(chunks[1].Data) != sr {
		t.Errorf("chunk 1: len = %d, want %d", len(chunks[1].Data), sr)
	}
	if len(chunks[2].Data) != half {
		t.Errorf("chunk 2 (tail): len = %d, want %d", len(chunks[2].Data), half)
	}
	if chunks[2].EndSec != 2.5 {
		t.Errorf("chunk 2: EndSec = %.4f, want 2.5", chunks[2].EndSec)
	}
}

// Test 3: invalid inputs return errors without panicking.
func TestSplit_inputErrors(t *testing.T) {
	cases := []struct {
		name     string
		audio    preprocess.Audio
		duration int
	}{
		{"zero duration", audio(makeSamples(sr)), 0},
		{"negative duration", audio(makeSamples(sr)), -1},
		{"empty samples", audio(nil), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Split(tc.audio, tc.duration)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
