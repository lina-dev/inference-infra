package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	SQSQueueURL         string
	S3InputBucket       string
	S3OutputBucket      string
	TritonAddress       string
	VLLMAddress         string
	WorkDir             string
	ChunkDurationSec    int
	TritonConcurrency   int
	AWSRegion           string
}

func Load() (*Config, error) {
	cfg := &Config{
		SQSQueueURL:      requireEnv("SQS_QUEUE_URL"),
		S3InputBucket:    requireEnv("S3_INPUT_BUCKET"),
		S3OutputBucket:   requireEnv("S3_OUTPUT_BUCKET"),
		TritonAddress:    getEnv("TRITON_ADDRESS", "triton-service:8001"),
		VLLMAddress:      getEnv("VLLM_ADDRESS", "vllm-service:8000"),
		WorkDir:          getEnv("WORK_DIR", "/tmp/worker"),
		AWSRegion:        getEnv("AWS_REGION", "us-east-1"),
		ChunkDurationSec: 1,
	}

	if v := os.Getenv("CHUNK_DURATION_SEC"); v != "" {
		d, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid CHUNK_DURATION_SEC: %w", err)
		}
		cfg.ChunkDurationSec = d
	}

	cfg.TritonConcurrency = 4
	if v := os.Getenv("TRITON_CONCURRENCY"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid TRITON_CONCURRENCY: %w", err)
		}
		cfg.TritonConcurrency = n
	}

	if cfg.SQSQueueURL == "" {
		return nil, fmt.Errorf("SQS_QUEUE_URL is required")
	}
	return cfg, nil
}

func requireEnv(key string) string {
	return os.Getenv(key)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
