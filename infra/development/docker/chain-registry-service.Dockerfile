# syntax=docker/dockerfile:1.6
# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files first (better cache)
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy only required source
COPY shared ./shared
COPY services/chain-registry-service ./services/chain-registry-service
COPY services/chain-registry-service/internal/seed/abi ./services/chain-registry-service/internal/seed/abi

# Build the binary
ARG CGO_ENABLED=0
ARG GOOS=linux
ARG GOARCH=amd64
RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -o build/chain-registry-service ./services/chain-registry-service/cmd/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy shared folder
COPY shared ./shared

# Copy binary from builder stage
COPY --from=builder /app/build ./build
COPY --from=builder /app/services/chain-registry-service/internal/seed/abi \
                    /app/services/chain-registry-service/internal/seed/abi

# Make binary executable
RUN chmod +x /app/build/chain-registry-service

ENTRYPOINT ["/app/build/chain-registry-service"]