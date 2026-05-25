package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/inference-infra/worker/internal/config"
	"github.com/inference-infra/worker/internal/pipeline"
	"github.com/inference-infra/worker/internal/preprocess"
	"github.com/inference-infra/worker/internal/s3"
	"github.com/inference-infra/worker/internal/sqs"
	"github.com/inference-infra/worker/internal/triton"
	"github.com/inference-infra/worker/internal/vllm"
	"go.uber.org/zap"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("load config", zap.Error(err))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		log.Fatal("aws config", zap.Error(err))
	}

	tritonClient, err := triton.NewClient(cfg.TritonAddress, cfg.TritonConcurrency)
	if err != nil {
		log.Fatal("triton client", zap.Error(err))
	}
	defer tritonClient.Close()

	runner := pipeline.NewRunner(
		pipeline.Config{
			WorkDir:          cfg.WorkDir,
			ChunkDurationSec: cfg.ChunkDurationSec,
			SampleRate:       preprocess.DefaultSampleRate,
			S3InputBucket:    cfg.S3InputBucket,
			S3OutputBucket:   cfg.S3OutputBucket,
		},
		s3.New(awsCfg),
		tritonClient,
		vllm.NewClient(cfg.VLLMAddress),
		log,
	)

	consumer := sqs.NewConsumer(
		awssqs.NewFromConfig(awsCfg),
		cfg.SQSQueueURL,
		func(ctx context.Context, job sqs.Job) error {
			return runner.Run(ctx, job.JobID, job.S3Key)
		},
		log,
	)

	log.Info("worker started", zap.String("queue", cfg.SQSQueueURL))
	if err := consumer.Run(ctx); err != nil {
		log.Error("consumer exited", zap.Error(err))
	}
}
