
FROM golang:1.24-alpine AS builder
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=auto
WORKDIR /src


COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download


COPY services/graphql-gateway ./services/graphql-gateway
COPY shared ./shared


RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -tags netgo -ldflags="-s -w" \
      -o /out/graphql-gateway ./services/graphql-gateway/main.go


FROM alpine:3.20
RUN apk add --no-cache ca-certificates netcat-openbsd && update-ca-certificates


RUN addgroup -S app && adduser -S app -G app
USER app

WORKDIR /app
COPY --from=builder /out/graphql-gateway /app/graphql-gateway



EXPOSE 8081
ENTRYPOINT ["/app/graphql-gateway"]
