package chunker

import (
	"fmt"

	"github.com/inference-infra/worker/internal/preprocess"
)

// Chunk is a contiguous slice of float32 PCM samples with timing metadata.
type Chunk struct {
	Index      int
	Data       []float32
	SampleRate int
	StartSec   float64
	EndSec     float64
}

// Split partitions a float32 PCM array into fixed-duration chunks.
// The last chunk may be shorter than durationSec if the audio doesn't divide evenly.
func Split(audio preprocess.Audio, durationSec int) ([]Chunk, error) {
	if durationSec <= 0 {
		return nil, fmt.Errorf("durationSec must be > 0, got %d", durationSec)
	}
	if len(audio.Samples) == 0 {
		return nil, fmt.Errorf("audio has no samples")
	}

	samplesPerChunk := audio.SampleRate * durationSec
	totalSamples := len(audio.Samples)
	nChunks := (totalSamples + samplesPerChunk - 1) / samplesPerChunk // ceiling division

	chunks := make([]Chunk, 0, nChunks)
	for i := range nChunks {
		start := i * samplesPerChunk
		end := start + samplesPerChunk
		if end > totalSamples {
			end = totalSamples
		}

		startSec := float64(start) / float64(audio.SampleRate)
		endSec := float64(end) / float64(audio.SampleRate)

		// Slice shares the backing array — no copy needed.
		chunks = append(chunks, Chunk{
			Index:      i,
			Data:       audio.Samples[start:end],
			SampleRate: audio.SampleRate,
			StartSec:   startSec,
			EndSec:     endSec,
		})
	}

	return chunks, nil
}
