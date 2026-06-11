REGISTRY ?= ghcr.io/inference-infra
TAG      ?= latest

.PHONY: build push lint test deploy

build:
	docker build -t $(REGISTRY)/worker:$(TAG) services/worker-go
	docker build -t $(REGISTRY)/scheduler:$(TAG) services/scheduler-go
	docker build -t $(REGISTRY)/api:$(TAG) services/api
	docker build -t $(REGISTRY)/triton-whisper:$(TAG) models/asr

push:
	docker push $(REGISTRY)/worker:$(TAG)
	docker push $(REGISTRY)/scheduler:$(TAG)
	docker push $(REGISTRY)/api:$(TAG)
	docker push $(REGISTRY)/triton-whisper:$(TAG)

lint:
	cd services/worker-go    && go vet ./... && golangci-lint run
	cd services/scheduler-go && go vet ./... && golangci-lint run
	cd services/api          && go vet ./... && golangci-lint run

test:
	cd services/worker-go    && go test ./...
	cd services/scheduler-go && go test ./...
	cd services/api          && go test ./...

deploy:
	kubectl apply -f infra/k8s/namespaces/
	kubectl apply -f infra/k8s/sqs/
	kubectl apply -f infra/k8s/triton/
	kubectl apply -f infra/k8s/vllm/
	kubectl apply -f infra/k8s/worker/
	kubectl apply -f infra/k8s/scheduler/
	kubectl apply -f infra/k8s/api/

helm-deploy:
	helm upgrade --install inference-infra infra/helm/inference-infra \
	  --set aws.sqsQueueUrl=$(SQS_QUEUE_URL) \
	  --set aws.s3InputBucket=$(S3_INPUT_BUCKET) \
	  --set aws.s3OutputBucket=$(S3_OUTPUT_BUCKET) \
	  --set aws.workerRoleArn=$(WORKER_ROLE_ARN) \
	  --set worker.tag=$(TAG) \
	  --set scheduler.tag=$(TAG)
