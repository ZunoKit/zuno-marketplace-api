# Quick Reference - Key Files and Entry Points

### Critical Files for Understanding the System

- **GraphQL Gateway Entry**: `services/graphql-gateway/main.go`
- **Service Entries**: Each service has `services/{service-name}/cmd/main.go`
- **Configuration**: `.env.example`, `shared/config/`
- **Core Business Logic**: Individual services in `services/{service-name}/internal/service/`
- **API Definitions**: GraphQL schema in `services/graphql-gateway/graphql/`
- **Database Models**: Each service manages its own schema in `services/{service-name}/db/up.sql`
- **gRPC Definitions**: `proto/*.proto` files
- **Infrastructure**: `docker-compose.yml`, `Tiltfile`, `infra/development/`

### Production Features Already Implemented

- **Security**: mTLS (`shared/tls/`), refresh token rotation, device fingerprinting (`services/auth-service/internal/fingerprint/`)
- **Reliability**: Circuit breakers (`shared/resilience/`), panic recovery (`shared/recovery/`)
- **Performance**: Query complexity limiting (`services/graphql-gateway/directives/`), optimized DB indexes
- **Observability**: Structured logging (`shared/logging/`), Prometheus metrics (`shared/metrics/`)
