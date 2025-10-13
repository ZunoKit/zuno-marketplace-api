# Source Tree and Module Organization

### Project Structure (Actual)

```text
zuno-marketplace-api/
├── services/                    # All microservices (11 services)
│   ├── auth-service/           # SIWE authentication, session management
│   ├── user-service/           # User profiles and account management
│   ├── wallet-service/         # Multi-wallet support and approvals
│   ├── orchestrator-service/   # Transaction intent orchestration
│   ├── media-service/          # Media upload and IPFS integration
│   ├── chain-registry-service/ # Chain configuration and contracts
│   ├── catalog-service/        # NFT catalog, marketplace data
│   ├── indexer-service/        # Blockchain event processing
│   ├── subscription-worker/    # Real-time notifications (no gRPC)
│   ├── graphql-gateway/        # GraphQL API, WebSocket subscriptions
│   └── {each service}/
│       ├── cmd/main.go         # Service entrypoint
│       ├── internal/           # Service-specific logic
│       ├── db/up.sql          # Database schema
│       └── test/              # Service tests
├── shared/                     # Shared code across services
│   ├── proto/                 # Generated protobuf code
│   ├── postgres/              # PostgreSQL utilities
│   ├── redis/                 # Redis utilities
│   ├── messaging/             # RabbitMQ utilities
│   ├── resilience/            # Circuit breakers, retries
│   ├── logging/               # Structured logging with zerolog
│   ├── monitoring/            # Prometheus metrics
│   ├── config/                # Centralized configuration
│   ├── tls/                   # TLS configuration for mTLS
│   └── [15 other utility packages]
├── proto/                      # Protobuf definitions (6 services)
├── infra/                      # Infrastructure configs
│   ├── development/k8s/       # Kubernetes manifests (Tilt)
│   ├── development/docker/    # Dockerfiles
│   └── development/build/     # Build scripts
└── test/e2e/                  # End-to-end tests
```

### Key Modules and Their Purpose

- **Authentication**: `services/auth-service/` - SIWE + JWT sessions with refresh token rotation
- **GraphQL Gateway**: `services/graphql-gateway/` - BFF pattern, WebSocket subscriptions
- **Transaction Orchestration**: `services/orchestrator-service/` - Intent-based transactions
- **Blockchain Indexing**: `services/indexer-service/` - Event processing with reorg handling
- **Media Management**: `services/media-service/` - IPFS/Pinata integration
- **Chain Registry**: `services/chain-registry-service/` - Multi-chain configuration
- **User/Wallet Management**: Separate services for user profiles and wallet operations
- **Catalog**: `services/catalog-service/` - NFT marketplace data and statistics
