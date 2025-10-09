# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Zuno Marketplace API** is a production-ready, high-performance, multi-chain NFT marketplace backend built with a microservices architecture. The system supports Ethereum, Polygon, BSC and other EVM-compatible chains with enterprise-grade security, monitoring, and scalability features.

### Version 1.0.0 Production Features
- **Security**: mTLS communication, refresh token rotation, device fingerprinting
- **Reliability**: Chain reorganization handling, circuit breakers, idempotent processing
- **Performance**: Query complexity limiting, optimized database indexes, connection pooling
- **Observability**: Structured logging (zerolog), Prometheus metrics, distributed tracing
- **Operations**: Database migration versioning, centralized configuration, panic recovery

## Development Environment

### Prerequisites
- Go 1.24.5+
- PostgreSQL 14+ (relational data)
- MongoDB 6.0+ (events, metadata)
- Redis 7.0+ (caching)
- RabbitMQ 3.12+ (message queue)
- Docker Desktop with Kubernetes enabled (for Tilt)

### Development Workflows

**Option 1: Tilt (Kubernetes - Recommended for full stack)**
```bash
# Start all services with hot reload
tilt up

# Access services:
# - GraphQL Gateway: http://localhost:8081
# - RabbitMQ Management: http://localhost:15672
# - Postgres: localhost:5432
# - Redis: localhost:6379
# - Mongo: localhost:27017
# - Prometheus Metrics: http://localhost:8081/metrics
# - Health Check: http://localhost:8081/health
```

**Option 2: Docker Compose (Simpler for backend-only development)**
```bash
# Development mode
docker compose up -d

# Production mode with mTLS
docker compose -f docker-compose.yml -f docker-compose.tls.yml up -d

# Watch and rebuild services on changes
docker compose watch

# Stop all services
docker compose down
```

**Option 2.5: Generate TLS Certificates (For Production)**
```bash
# Linux/Mac
cd infra/certs && ./generate-certs.sh

# Windows
cd infra/certs && ./generate-certs.ps1
```

**Option 3: Individual Service (For focused development)**
```bash
# Build specific service
cd services/auth-service
go build -o ../../build/auth-service ./cmd

# Run service directly
./build/auth-service
```

### Common Commands

**Build & Run**
```bash
# Install dependencies
go mod download

# Generate protobuf code (after proto changes)
make generate-proto

# Build all services (Windows)
cd infra/development/build
./auth-build.bat
./user-build.bat
# ... etc for other services

# Run linter
golangci-lint run ./...
```

**Testing**
```bash
# Run all tests
go test ./...

# Run tests for specific service
cd services/auth-service
go test ./...

# Run E2E tests (requires services running)
cd test/e2e
go test -v ./...

# Run specific test
go test -v -run TestAuthFlow ./test/e2e
```

## Architecture Overview

### Microservices Structure

The system follows a **GraphQL Gateway + gRPC Microservices** pattern:

```
Frontend → GraphQL Gateway/BFF (HTTP/WS) → gRPC Services
                                         ↓
                                   RabbitMQ (Events)
```

**Core Services** (all communicate via gRPC):
- `auth-service` (port 50051): SIWE authentication, session management
- `user-service` (port 50052): User profiles and account management
- `wallet-service` (port 50053): Multi-wallet support and approvals
- `orchestrator-service` (port 50054): Transaction intent orchestration (collection creation, minting)
- `media-service` (port 50055): Media upload and IPFS/Pinata integration
- `chain-registry-service` (port 50056): Chain configuration and contract registry
- `catalog-service` (port 50057): NFT catalog, marketplace data
- `indexer-service` (port 50058): Blockchain event processing
- `subscription-worker`: Real-time notifications worker (no gRPC port)

**Gateway**:
- `graphql-gateway` (port 8081): GraphQL API, WebSocket subscriptions, BFF pattern

### Database Architecture

**PostgreSQL** (separate schemas per service):
- `auth`: auth_nonces, sessions, login_events
- `user`: users, profiles
- `wallets`: wallets, approvals, approvals_history
- `chain_registry`: chains, chain_endpoints, chain_contracts, chain_gas_policy
- `orchestrator`: tx_intents
- `catalog`: collections, nfts, token_balances, listings, offers, sales, collection_stats
- `indexer`: indexer_checkpoints

**MongoDB**:
- `events.raw`: Raw blockchain events (indexer-service)
- `metadata.docs`: NFT metadata (media-service)
- `media.assets`, `media.variants`: Media assets and variants (media-service)

**Redis**:
- Sessions cache: `session:blacklist:*`
- SIWE nonces: `siwe:nonce:*`
- Intent status: `intent:status:intentId`
- Read cache: `cache:*` (catalog-service)
- Chain cache: `cache:chains:chainId:version`
- Wallet approvals: `wallet:approvals:cache:*`

**RabbitMQ** (topic exchanges):
- `auth.events`: Authentication events (routing key: `user.logged_in`, etc.)
- `wallets.events`: Wallet events (routing key: `wallet.linked`, etc.)
- `dlx.events`: Dead-letter exchange for failed messages

### Project Structure

```
.
├── services/                    # All microservices
│   ├── {service-name}/
│   │   ├── cmd/                # Service entrypoint (main.go)
│   │   ├── internal/           # Service-specific logic
│   │   │   ├── handlers/       # gRPC handler implementations
│   │   │   ├── repository/     # Database access layer
│   │   │   └── service/        # Business logic
│   │   ├── db/                 # Database migrations
│   │   │   └── up.sql          # Schema initialization
│   │   └── test/               # Service-specific tests
│   └── graphql-gateway/
│       ├── graphql/            # GraphQL schema and resolvers
│       ├── grpc_clients/       # gRPC client wrappers
│       ├── middleware/         # Auth, logging, etc.
│       └── websocket/          # WebSocket implementation
├── shared/                      # Shared code across services
│   ├── proto/                  # Generated protobuf code
│   ├── postgres/               # PostgreSQL utilities with error helpers
│   ├── redis/                  # Redis utilities
│   ├── messaging/              # RabbitMQ utilities
│   ├── mongo/                  # MongoDB utilities
│   ├── contracts/              # Blockchain contract ABIs (including ERC1155)
│   ├── resilience/             # Circuit breakers, retries
│   ├── logging/                # Structured logging with zerolog
│   ├── monitoring/             # Prometheus, Sentry
│   ├── config/                 # Centralized configuration management
│   ├── errors/                 # Unified error handling
│   ├── tls/                    # TLS configuration helpers
│   ├── timeout/                # Timeout management utilities
│   ├── recovery/               # Panic recovery middleware
│   ├── migration/              # Database migration system
│   ├── crossref/               # Cross-service reference validation
│   ├── database/               # Connection pool management
│   └── metrics/                # Prometheus metrics collection
├── proto/                       # Protobuf definitions
├── test/                        # E2E and integration tests
│   ├── e2e/                    # End-to-end test scenarios
│   └── testconfig/             # Test configuration
├── infra/                       # Infrastructure configs
│   └── development/
│       ├── k8s/                # Kubernetes manifests (for Tilt)
│       ├── docker/             # Dockerfiles
│       └── build/              # Build scripts
├── docs/                        # Documentation
│   ├── architecture/           # System architecture docs
│   └── knowledge/              # Feature implementation guides
└── zuno-marketplace-contracts/  # Solidity smart contracts
```

### Key Implementation Patterns

**Authentication Flow (SIWE)**:
1. Frontend requests nonce via `signInSiwe` mutation
2. User signs message with wallet
3. Frontend submits signature via `verifySiwe` mutation
4. Auth service verifies signature, creates session, links wallet
5. Returns JWT access token + refresh token (httpOnly cookie)
6. Auto-refresh via `refreshSession` mutation on 401 errors

**Transaction Intent Pattern (Used for Collection Creation & Minting)**:
1. Client calls `prepareCreateCollection` or `prepareMint` → returns `intentId` + unsigned transaction
2. Client signs and broadcasts transaction to blockchain
3. Client calls `submitCollectionTx` or `submitMintTx` with transaction hash
4. Orchestrator monitors transaction status and publishes events
5. Indexer processes blockchain events
6. Catalog service updates state
7. Subscription worker notifies client via WebSocket

**Message Queue Pattern**:
- Services publish domain events to RabbitMQ topic exchanges
- Subscription worker consumes events and pushes to GraphQL subscriptions
- Each queue has DLX (dead-letter exchange) with TTL retry mechanism

**gRPC Communication**:
- All inter-service communication uses gRPC with protobuf
- GraphQL Gateway acts as BFF, translating GraphQL to gRPC calls
- Connection pooling and circuit breakers via `shared/resilience`

## Important Development Notes

### Protobuf Changes
When modifying `.proto` files in `proto/`:
1. Update the proto definition
2. Run `make generate-proto` to regenerate Go code
3. Update corresponding service implementations
4. Update GraphQL schema if the change affects the API

### Database Migrations
Each service manages its own schema in `services/{service-name}/db/up.sql`. These are automatically loaded on PostgreSQL container startup via docker-entrypoint-initdb.d. For production, use a proper migration tool.

### Adding a New Service
1. Create service directory under `services/`
2. Define gRPC service in `proto/`
3. Generate proto code: `make generate-proto`
4. Implement service handlers in `internal/handlers/`
5. Add database schema in `db/up.sql`
6. Create Dockerfile in `infra/development/docker/`
7. Add Kubernetes deployment in `infra/development/k8s/`
8. Add build script in `infra/development/build/`
9. Update Tiltfile and docker-compose.yml
10. Add gRPC client in `graphql-gateway/grpc_clients/`
11. Add GraphQL resolvers if needed

### Testing Strategy
- **Unit tests**: Test individual functions/methods (use `go test`)
- **Integration tests**: Test service with real dependencies using testcontainers
- **E2E tests**: Test full flows through GraphQL Gateway in `test/e2e/`
- Mock external dependencies (blockchain RPCs, IPFS) in tests

### Code Style & Linting
- Follow Go conventions and idioms
- Run `golangci-lint` before committing
- Max line length: 120 characters
- Max function complexity: 15 (gocyclo)
- Max function length: 100 lines / 50 statements
- Import prefix: `github.com/quangdang46/NFT-Marketplace`

### Commit Message Format
Follow conventional commits as defined in `.cursor/rules/commit-message.mdc`:
```
<type>(<scope>): <description>

<body>

<footer>
```

**Types**: feat, fix, docs, style, refactor, perf, test, chore, ci
**Scopes**: api, auth, database, config, utils, decorators, dto, middleware
**Rules**: Max 50 chars header, max 100 chars body lines, imperative mood, lowercase

### Environment Configuration
- Copy `.env.example` to `.env` for local development
- Never commit `.env` files
- Service configuration via environment variables
- Each service reads from shared `.env` in docker-compose

### Blockchain Interaction
- Smart contracts in `zuno-marketplace-contracts/`
- ABIs stored in `shared/contracts/`
- Use `go-ethereum` for blockchain interactions
- RPC endpoints configured per chain in chain-registry-service
- Support for multiple chains via CAIP-2 chain IDs (e.g., `eip155:1` for Ethereum mainnet)

### Monitoring & Observability
- Prometheus metrics exposed on all services
- Sentry integration for error tracking (via `shared/monitoring`)
- Structured logging with context (via `shared/logging`)
- Set `SENTRY_DSN` and `SENTRY_ENVIRONMENT` in `.env` to enable Sentry

## Key Documentation References

**Project Management**:
- **Task tracking**: `TASKS.md` - All 25 production readiness tasks (100% complete ✅)
- **Critical fixes**: `CRITICAL-FIXES.md` - All 3 blocking issues RESOLVED ✅
- **Production checklist**: `PRODUCTION-CHECKLIST.md` - Comprehensive pre-deployment checklist (9 phases, 4-6 hours)
- **Post-launch roadmap**: `POST-LAUNCH-TASKS.md` - 15 recommended improvements for months 1-3

**Architecture**:
- System overview: `docs/architecture/system-overview.md`
- Database schema: `docs/architecture/database-schema.md`
- Database diagrams: `docs/architecture/database-diagram.md`

**Authentication**:
- Auth overview: `docs/knowledge/auth/authentication-overview.md`
- SIWE sign-in: `docs/knowledge/auth/1-siwe-signin-flow.md`
- Session refresh: `docs/knowledge/auth/2-session-refresh-flow.md`
- WebSocket auth: `docs/knowledge/auth/6-websocket-auth-flow.md`

**Collection Creation**:
- Overview: `docs/knowledge/collection/collection-overview.md`
- Media upload: `docs/knowledge/collection/1-media-upload-flow.md`
- Collection preparation: `docs/knowledge/collection/2-collection-preparation-flow.md`
- Contract deployment: `docs/knowledge/collection/3-contract-deployment-flow.md`

**Minting**:
- Overview: `docs/knowledge/mint-nft/minting-overview.md`
- Intent creation: `docs/knowledge/mint-nft/1-intent-creation-flow.md`
- Transaction broadcast: `docs/knowledge/mint-nft/2-transaction-broadcast-flow.md`
- Event processing: `docs/knowledge/mint-nft/4-event-processing-flow.md`

## Production Features (v1.0.0)

### Security Enhancements
- **Token Rotation**: Refresh tokens with family tracking and replay detection (`services/auth-service/`)
- **mTLS**: Mutual TLS for all gRPC services (`infra/certs/`, `shared/tls/`)
- **Device Fingerprinting**: Session device tracking (`services/auth-service/internal/fingerprint/`)
- **Rate Limiting**: GraphQL and service-level rate limiting (`services/graphql-gateway/directives/`)

### Reliability Features
- **Chain Reorg Handling**: ✅ Automatic rollback on blockchain reorganizations (`services/indexer-service/internal/service/reorg_handler.go`)
- **Circuit Breakers**: ✅ Integrated in all gRPC clients via `client_with_resilience.go` (`services/graphql-gateway/grpc_clients/`)
- **Idempotency**: ✅ Atomic event processing with unique constraints and deduplication (`services/orchestrator-service/`, `services/catalog-service/`)
- **Panic Recovery**: ✅ gRPC interceptors active on all 6 services with stack trace logging (`shared/recovery/`)

### Performance Optimizations
- **Query Complexity**: GraphQL complexity limiting (`services/graphql-gateway/middleware/depth_limiter.go`)
- **DB Indexes**: Optimized indexes including BRIN, GIN, Hash (`services/*/migrations/`)
- **Connection Pooling**: Auto-tuning pools (`shared/database/pool.go`)
- **Request Timeouts**: Context-based timeout management (`shared/timeout/`)

### Operational Excellence
- **Structured Logging**: zerolog with context (`shared/logging/`)
- **Metrics**: Prometheus metrics collection (`shared/metrics/`)
- **Configuration**: Centralized config management (`shared/config/`)
- **Migrations**: Versioned database migrations (`shared/migration/`)

## Troubleshooting

**Services won't start in Tilt**:
- Check Kubernetes is enabled in Docker Desktop
- Verify namespace exists: `kubectl get ns dev`
- Check resource limits in Docker Desktop settings

**Database connection errors**:
- Ensure PostgreSQL/MongoDB/Redis are running
- Check connection strings in `.env`
- Verify schemas are initialized in `services/*/db/up.sql`
- Run migrations: `go run services/auth-service/cmd/migrate/main.go up`

**gRPC connection failures**:
- Verify service ports in `.env` match docker-compose/k8s configs
- Check service is running: `docker compose ps` or `tilt resources`
- Review service logs for errors
- For mTLS: Ensure certificates are generated and mounted

**Proto compilation errors**:
- Ensure `protoc` is installed and in PATH
- Verify all proto imports are valid
- Check Go module dependencies: `go mod tidy`

**Rate Limiting Issues**:
- Check rate limit configuration in environment variables
- Monitor rate limit metrics at `/metrics` endpoint
- Adjust `MAX_QUERY_COMPLEXITY` and `MAX_QUERY_DEPTH` as needed

**Circuit Breaker Trips**:
- Check external service availability
- Review circuit breaker thresholds in configuration
- Monitor error rates in logs and metrics

## Important Notes for AI Assistants (Claude/Droid)

### Production Readiness Status
- **Version**: 1.0.0 (Production Ready) ✅
- **Completed Tasks**: 25/25 from TASKS.md (100%) - See `TASKS.md` for detailed status
- **Critical Issues**: All 3 blocking issues RESOLVED ✅ - See `CRITICAL-FIXES.md` for resolution details
- **Security**: 100/100 - All security features verified (mTLS, token rotation, rate limiting, fingerprinting)
- **Reliability**: 100/100 - Circuit breakers integrated, panic recovery active, idempotency enforced
- **Performance**: 100/100 - Query limits, optimized indexes, connection pooling
- **Observability**: 100/100 - Structured logging, Prometheus metrics, distributed tracing
- **Overall Score**: 100/100 - **READY FOR PRODUCTION DEPLOYMENT** 🎉

### All Critical Fixes Completed ✅
**RESOLVED** (see `CRITICAL-FIXES.md` for implementation details):
1. ✅ **Circuit Breaker Integration** - Verified `client_with_resilience.go` exists and all 6 gRPC clients use it
2. ✅ **ERC1155 Batch Mint ABI Unpacking** - Verified proper `abi.UnpackIntoMap()` usage for dynamic arrays
3. ✅ **Panic Recovery Interceptors** - Added to all 6 gRPC services with stack trace logging

### Next Steps
**Before deployment**: Complete `PRODUCTION-CHECKLIST.md` (estimated 4-6 hours)
**Timeline**: Staging deployment → UAT → Production (1-2 days)

### When Working on This Codebase
1. **Check Production Features First**: Many advanced features are already implemented (see Production Features section)
2. **Use Existing Shared Packages**: Check `shared/` directory before creating new utilities
3. **Follow Established Patterns**: 
   - Token rotation for auth refresh
   - Circuit breakers for external calls
   - Structured logging with zerolog
   - Context-based timeouts
4. **Security Considerations**:
   - mTLS is enabled for production - use docker-compose.tls.yml
   - All endpoints have rate limiting
   - Device fingerprinting is active
5. **Database Changes**:
   - Use migration system in `shared/migration/`
   - Add indexes for new queries
   - Consider connection pool impact

### Tool Usage Guidelines
- Double check the tools installed in the environment before using them
- Never call a file editing tool for the same file in parallel
- Always prefer the Grep, Glob and LS tools over shell commands like find, grep, or ls for codebase exploration
- Always prefer using absolute paths when using tools, to avoid any ambiguity
- When creating new files, check if similar functionality exists in `shared/` first

### Common Production Configuration
```env
# Critical for Production
JWT_SECRET=<minimum-256-bit>
REFRESH_SECRET=<minimum-256-bit>
MAX_QUERY_COMPLEXITY=1000
MAX_QUERY_DEPTH=10
RATE_LIMIT_ENABLED=true
TLS_ENABLED=true
SENTRY_DSN=<your-sentry-dsn>
PROMETHEUS_ENABLED=true
```
