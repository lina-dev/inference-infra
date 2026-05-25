FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git protoc protobuf-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /worker ./main.go

FROM alpine:3.19
RUN apk add --no-cache ffmpeg ca-certificates
COPY --from=builder /worker /usr/local/bin/worker
ENV WORK_DIR=/tmp/worker
RUN mkdir -p /tmp/worker
ENTRYPOINT ["worker"]
