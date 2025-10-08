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

# ============================================================================
# Testing Commands
# ============================================================================

.PHONY: test
test: ## Run all tests
	@echo "Running all tests..."
	@$(MAKE) test-unit
	@$(MAKE) test-integration

.PHONY: test-unit
test-unit: ## Run unit tests for all services
	@echo "Running unit tests..."
	@go test -v -race -cover ./services/*/test/unit/...

.PHONY: test-integration
test-integration: ## Run integration tests for all services
	@echo "Running integration tests..."
	@go test -v -race -cover ./services/*/test/integration/...

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests
	@echo "Running e2e tests..."
	@docker-compose up -d
	@sleep 10  # Wait for services to start
	@go test -v -race -cover ./test/e2e/...
	@docker-compose down

.PHONY: test-service
test-service: ## Run tests for a specific service (usage: make test-service SERVICE=auth-service)
	@echo "Running tests for $(SERVICE)..."
	@go test -v -race -cover ./services/$(SERVICE)/test/...

.PHONY: test-coverage
test-coverage: ## Generate test coverage report
	@echo "Generating test coverage report..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: test-coverage-service
test-coverage-service: ## Generate test coverage for a specific service
	@echo "Generating test coverage for $(SERVICE)..."
	@go test -v -race -coverprofile=coverage-$(SERVICE).out ./services/$(SERVICE)/...
	@go tool cover -html=coverage-$(SERVICE).out -o coverage-$(SERVICE).html
	@echo "Coverage report generated: coverage-$(SERVICE).html"

.PHONY: test-benchmark
test-benchmark: ## Run benchmark tests
	@echo "Running benchmark tests..."
	@go test -bench=. -benchmem ./...

.PHONY: test-watch
test-watch: ## Run tests in watch mode
	@echo "Running tests in watch mode..."
	@gotestsum --watch -- -v -race ./...

.PHONY: test-clean
test-clean: ## Clean test artifacts
	@echo "Cleaning test artifacts..."
	@rm -f coverage*.out coverage*.html
	@rm -rf test-results/
	@echo "Test artifacts cleaned"

.PHONY: deps
deps:
	go mod download
	go mod tidy
