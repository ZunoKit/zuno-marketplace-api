# syntax=docker/dockerfile:1.6
# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files first (better cache)
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy only required source
COPY shared ./shared
COPY proto ./proto
COPY services/subscription-worker ./services/subscription-worker

# Build the binary
ARG CGO_ENABLED=0
ARG GOOS=linux
ARG GOARCH=amd64
RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -o build/subscription-worker ./services/subscription-worker/cmd/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Install dependencies
RUN apk --no-cache add ca-certificates

# Copy shared folder
COPY shared ./shared

# Copy binary from builder stage
COPY --from=builder /app/build ./build

# Make binary executable
RUN chmod +x /app/build/subscription-worker

ENTRYPOINT ["/app/build/subscription-worker"]
