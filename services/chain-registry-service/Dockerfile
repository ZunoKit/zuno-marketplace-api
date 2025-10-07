FROM golang:1.24-alpine AS builder
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=auto
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY services/chain-registry-service ./services/chain-registry-service
COPY services/chain-registry-service/internal/seed/abi ./services/chain-registry-service/internal/seed/abi

COPY shared ./shared

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -tags netgo -ldflags="-s -w" -o /out/chain-registry-service ./services/chain-registry-service/cmd/main.go


FROM alpine:3.20
RUN apk add --no-cache ca-certificates netcat-openbsd && update-ca-certificates

RUN addgroup -S app && adduser -S app -G app
USER app

WORKDIR /app
COPY --from=builder /out/chain-registry-service /app/chain-registry-service

EXPOSE 50056
ENTRYPOINT ["/app/chain-registry-service"]




