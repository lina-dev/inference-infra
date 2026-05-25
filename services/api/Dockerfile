FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /api ./main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /api /usr/local/bin/api
EXPOSE 8080
ENTRYPOINT ["api"]
