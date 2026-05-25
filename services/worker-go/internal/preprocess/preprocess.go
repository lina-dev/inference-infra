package preprocess

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

const DefaultSampleRate = 16000

// Audio holds a normalized float32 PCM buffer and its sample rate.
type Audio struct {
	Samples    []float32
	SampleRate int
}

// Run decodes any audio file to a mono float32 PCM array.
// It downmixes to 1 channel and resamples to targetSR Hz via ffmpeg.
// Output samples are in the range [-1.0, 1.0].
func Run(inputPath string, targetSR int) (Audio, error) {
	if err := validateStream(inputPath); err != nil {
		return Audio{}, fmt.Errorf("validate: %w", err)
	}

	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-ar", strconv.Itoa(targetSR), // resample to target rate
		"-ac", "1",                    // downmix to mono
		"-f", "f32le",                 // raw float32 little-endian, no container
		"-",                           // pipe to stdout
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return Audio{}, fmt.Errorf("ffmpeg decode: %w\n%s", err, stderr.String())
	}

	samples, err := decodePCMf32le(stdout.Bytes())
	if err != nil {
		return Audio{}, fmt.Errorf("decode pcm: %w", err)
	}

	return Audio{Samples: samples, SampleRate: targetSR}, nil
}

// decodePCMf32le converts a raw f32le byte slice to []float32.
func decodePCMf32le(raw []byte) ([]float32, error) {
	if len(raw)%4 != 0 {
		return nil, fmt.Errorf("pcm length %d is not a multiple of 4", len(raw))
	}
	samples := make([]float32, len(raw)/4)
	buf := bytes.NewReader(raw)
	for i := range samples {
		var bits uint32
		if err := binary.Read(buf, binary.LittleEndian, &bits); err != nil {
			return nil, err
		}
		samples[i] = math.Float32frombits(bits)
	}
	return samples, nil
}

// validateStream confirms that the file has at least one audio stream.
func validateStream(path string) error {
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=codec_type",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		return fmt.Errorf("ffprobe: %w", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("no audio stream in %s", path)
	}
	return nil
}
