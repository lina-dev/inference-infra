module github.com/inference-infra/worker

go 1.22

require (
	github.com/aws/aws-sdk-go-v2 v1.26.0
	github.com/aws/aws-sdk-go-v2/config v1.27.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.52.0
	github.com/aws/aws-sdk-go-v2/service/sqs v1.31.0
	github.com/google/uuid v1.6.0
	github.com/triton-inference-server/client v0.0.0-20240301000000-000000000000
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.63.0
	google.golang.org/protobuf v1.33.0
)
