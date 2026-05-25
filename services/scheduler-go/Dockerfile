FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /scheduler ./main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /scheduler /usr/local/bin/scheduler
ENTRYPOINT ["scheduler"]
