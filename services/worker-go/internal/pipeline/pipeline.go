package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/inference-infra/worker/internal/chunker"
	"github.com/inference-infra/worker/internal/merger"
	"github.com/inference-infra/worker/internal/output"
	"github.com/inference-infra/worker/internal/preprocess"
	"github.com/inference-infra/worker/internal/s3"
	"github.com/inference-infra/worker/internal/triton"
	"github.com/inference-infra/worker/internal/vllm"
	"go.uber.org/zap"
)

// Config groups the external dependencies the pipeline needs.
type Config struct {
	WorkDir          string
	ChunkDurationSec int
	SampleRate       int
	S3InputBucket    string
	S3OutputBucket   string
}

// Runner executes the full transcription + summarization pipeline for one job.
type Runner struct {
	cfg    Config
	s3     *s3.Client
	triton *triton.Client
	vllm   *vllm.Client
	log    *zap.Logger
}

func NewRunner(cfg Config, s3c *s3.Client, tc *triton.Client, vc *vllm.Client, log *zap.Logger) *Runner {
	return &Runner{cfg: cfg, s3: s3c, triton: tc, vllm: vc, log: log}
}

// Run processes a single job:
// S3 download → preprocess → chunk → Triton (concurrent, ordered) → merge → vLLM → S3 upload.
func (r *Runner) Run(ctx context.Context, jobID, s3Key string) error {
	log := r.log.With(zap.String("job_id", jobID))

	jobDir := filepath.Join(r.cfg.WorkDir, jobID)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return fmt.Errorf("create job dir: %w", err)
	}
	defer os.RemoveAll(jobDir)

	// 1. Download raw audio from S3.
	rawPath := filepath.Join(jobDir, "input_raw")
	if err := r.s3.Download(ctx, r.cfg.S3InputBucket, s3Key, rawPath); err != nil {
		return fmt.Errorf("s3 download: %w", err)
	}

	// 2. Preprocess: validate, resample to target SR, downmix to mono → []float32.
	audio, err := preprocess.Run(rawPath, r.cfg.SampleRate)
	if err != nil {
		return fmt.Errorf("preprocess: %w", err)
	}
	durationSec := float64(len(audio.Samples)) / float64(audio.SampleRate)
	log.Info("preprocessed audio",
		zap.Int("samples", len(audio.Samples)),
		zap.Int("sample_rate", audio.SampleRate),
		zap.Float64("duration_sec", durationSec),
	)

	// 3. Chunk: slice the float32 array into fixed-duration segments.
	chunks, err := chunker.Split(audio, r.cfg.ChunkDurationSec)
	if err != nil {
		return fmt.Errorf("chunk: %w", err)
	}
	log.Info("chunked", zap.Int("chunks", len(chunks)))

	// 4. Transcribe: concurrent dispatch to Triton; segments arrive in ChunkIndex order.
	segCh, tritonErrCh := r.triton.Transcribe(ctx, chunks)

	// 5. Merge: drains segCh as segments arrive — does not wait for all chunks.
	transcript, err := merger.Merge(ctx, segCh)
	if err != nil {
		return fmt.Errorf("merge: %w", err)
	}
	// Surface any Triton error that occurred during transcription.
	select {
	case err := <-tritonErrCh:
		return fmt.Errorf("triton: %w", err)
	default:
	}

	// 6. Summarize via vLLM.
	summary, err := r.vllm.Summarize(ctx, transcript)
	if err != nil {
		return fmt.Errorf("vllm: %w", err)
	}

	// 7. Upload result JSON to S3.
	result := output.Result{JobID: jobID, Transcript: transcript, Summary: summary}
	outKey := fmt.Sprintf("output/%s/result.json", jobID)
	if err := r.s3.UploadJSON(ctx, r.cfg.S3OutputBucket, outKey, result); err != nil {
		return fmt.Errorf("upload: %w", err)
	}

	log.Info("job complete", zap.String("output_key", outKey))
	return nil
}
