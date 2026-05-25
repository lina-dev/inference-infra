# inference-infra

Audio transcription and summarization pipeline running on Kubernetes.

## What it does

1. A job message arrives on an SQS queue with an S3 key pointing to an audio file.
2. The **worker** downloads the file, runs it through ffmpeg (resample to 16 kHz mono `float32`), splits it into fixed-length chunks, and sends each chunk concurrently to **Triton Inference Server** over gRPC for Whisper transcription.
3. Transcribed segments are reordered and merged into a timestamped transcript, then sent to **vLLM** for summarization.
4. The transcript and summary are uploaded as JSON back to S3.

## Services

| Service | Language | Role |
|---|---|---|
| `services/worker-go` | Go | Processes jobs: download → preprocess → transcribe → summarize → upload |
| `services/scheduler-go` | Go | Scales the worker Deployment based on SQS queue depth |
| `services/api` | Go | REST API for submitting jobs and querying results |

## Models

| Path | Role |
|---|---|
| `models/asr/` | Triton backend running Whisper; accepts `float32` audio + sample rate, returns transcript bytes |

## Infrastructure

- `infra/k8s/` — raw Kubernetes manifests (namespaces, Deployments, HPA, IRSA annotations)
- `infra/helm/` — Helm chart for parameterized deploys

## Quick start

```sh
# Build all images
make build REGISTRY=your-registry TAG=dev

# Run tests
make test

# Deploy to current kubectl context
make helm-deploy \
  SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/123/jobs \
  S3_INPUT_BUCKET=my-audio-input \
  S3_OUTPUT_BUCKET=my-results \
  WORKER_ROLE_ARN=arn:aws:iam::123:role/worker
```

## Worker environment variables

| Variable | Default | Description |
|---|---|---|
| `SQS_QUEUE_URL` | required | Queue the worker polls |
| `S3_INPUT_BUCKET` | required | Bucket containing source audio |
| `S3_OUTPUT_BUCKET` | required | Bucket for transcript/summary output |
| `TRITON_ADDRESS` | required | Triton gRPC endpoint (`host:port`) |
| `TRITON_CONCURRENCY` | `4` | Parallel chunk inference requests |
| `VLLM_ADDRESS` | required | vLLM base URL |
| `WORK_DIR` | `/tmp` | Scratch directory for downloads |
| `CHUNK_DURATION_SEC` | `30` | Audio chunk length in seconds |
| `AWS_REGION` | required | AWS region |
