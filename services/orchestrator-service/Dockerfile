FROM golang:1.24-alpine AS builder
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=auto
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY services/orchestrator-service ./services/orchestrator-service
COPY shared ./shared

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -tags netgo -ldflags="-s -w" -o /out/orchestrator-service ./services/orchestrator-service/cmd/main.go


FROM alpine:3.20
RUN apk add --no-cache ca-certificates netcat-openbsd && update-ca-certificates

RUN addgroup -S app && adduser -S app -G app
USER app

WORKDIR /app
COPY --from=builder /out/orchestrator-service /app/orchestrator-service


EXPOSE 50054
ENTRYPOINT ["/app/orchestrator-service"]




