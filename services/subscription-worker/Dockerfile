# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the subscription worker service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o subscription-worker ./services/subscription-worker/cmd/main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/subscription-worker .

# Expose WebSocket port
EXPOSE 8080

# Run the subscription worker service
CMD ["./subscription-worker"]