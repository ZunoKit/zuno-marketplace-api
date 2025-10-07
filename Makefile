PROTO_DIR := proto
PROTO_SRC := $(wildcard $(PROTO_DIR)/*.proto)
GO_OUT := .

.PHONY: generate-proto
generate-proto:
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(GO_OUT) \
		--go-grpc_out=$(GO_OUT) \
		$(PROTO_SRC)


gql:
	go get github.com/99designs/gqlgen

.PHONY: fmt
fmt:
	gofmt -s -w .
	goimports -w .

.PHONY: lint
lint:
	golangci-lint run

.PHONY: test
test:
	go test -v -race -coverprofile=coverage.out ./...

.PHONY: test-coverage
test-coverage: test
	go tool cover -html=coverage.out -o coverage.html

.PHONY: deps
deps:
	go mod download
	go mod tidy

.PHONY: install-tools
install-tools:
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY: build-all
build-all:
	@for service in auth-service catalog-service chain-registry-service graphql-gateway media-service orchestrator-service user-service indexer-service subscription-worker; do \
		echo "Building $$service..."; \
		if [ -f "services/$$service/cmd/main.go" ]; then \
			cd services/$$service && go build -o bin/$$service cmd/main.go && cd ../..; \
		elif [ -f "services/$$service/main.go" ]; then \
			cd services/$$service && go build -o bin/$$service main.go && cd ../..; \
		else \
			echo "No main.go found for $$service"; \
		fi; \
	done

.PHONY: clean
clean:
	@for service in auth-service catalog-service chain-registry-service graphql-gateway media-service orchestrator-service user-service indexer-service subscription-worker; do \
		if [ -d "services/$$service/bin" ]; then \
			rm -rf services/$$service/bin; \
		fi; \
	done
	rm -f coverage.out coverage.html

.PHONY: help
help:
	@echo "Available commands:"
	@echo "  generate-proto  Generate protobuf files"
	@echo "  gql            Install gqlgen"
	@echo "  fmt            Format Go code"
	@echo "  lint           Run linter"
	@echo "  test           Run tests with coverage"
	@echo "  test-coverage  Run tests and generate HTML coverage report"
	@echo "  deps           Download and tidy dependencies"
	@echo "  install-tools  Install development tools"
	@echo "  build-all      Build all services"
	@echo "  clean          Clean build artifacts"
	@echo "  help           Show this help message"


