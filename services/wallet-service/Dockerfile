FROM golang:1.24-alpine AS builder
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=auto
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY services/wallet-service ./services/wallet-service
COPY shared ./shared

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -tags netgo -ldflags="-s -w" -o /out/wallet-service ./services/wallet-service/cmd/main.go


FROM alpine:3.20
RUN apk add --no-cache ca-certificates netcat-openbsd && update-ca-certificates

RUN addgroup -S app && adduser -S app -G app
USER app

WORKDIR /app
COPY --from=builder /out/wallet-service /app/wallet-service


EXPOSE 50053
ENTRYPOINT ["/app/wallet-service"]




