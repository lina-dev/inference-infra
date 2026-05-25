package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type submitRequest struct {
	S3Key string `json:"s3_key"`
}

type submitResponse struct {
	JobID string `json:"job_id"`
}

var (
	sqsClient *sqs.Client
	queueURL  string
	log       *zap.Logger
)

func main() {
	log, _ = zap.NewProduction()
	defer log.Sync()

	queueURL = mustEnv("SQS_QUEUE_URL")
	region := getEnv("AWS_REGION", "us-east-1")

	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		log.Fatal("aws config", zap.Error(err))
	}
	sqsClient = sqs.NewFromConfig(awsCfg)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/jobs", handleSubmit)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Info("api listening", zap.String("addr", srv.Addr))
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal("server error", zap.Error(err))
	}
}

func handleSubmit(w http.ResponseWriter, r *http.Request) {
	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.S3Key == "" {
		http.Error(w, "s3_key is required", http.StatusBadRequest)
		return
	}

	jobID := uuid.New().String()
	body, _ := json.Marshal(map[string]string{"job_id": jobID, "s3_key": req.S3Key})

	_, err := sqsClient.SendMessage(r.Context(), &sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		log.Error("send message failed", zap.Error(err))
		http.Error(w, "failed to enqueue job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(submitResponse{JobID: jobID})
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		panic(fmt.Sprintf("required env var %s not set", k))
	}
	return v
}

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
