# Testing Reality

### Current Test Coverage

- **Unit Tests**: Service-specific tests in each `services/{service}/test/`
- **Integration Tests**: Database integration with testcontainers
- **E2E Tests**: Complete workflow tests in `test/e2e/`
- **Mocking**: Protocol buffer mocks generated via `golang/mock`

### Running Tests

```bash
# All tests
go test ./...

# Service-specific tests
cd services/auth-service && go test ./...

# E2E tests (requires running services)
cd test/e2e && go test -v ./...

# Generate mocks
make generate-proto
```
