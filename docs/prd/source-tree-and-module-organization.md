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

### Enhancement Scope Definition

**Enhancement Type**: ✅ **Bug Fix and Stability Improvements**

**Enhancement Description**:
Comprehensive Quality Assurance and Bug Fixing Initiative targeting systematic testing, bug identification, and stabilization across all 11 microservices of the production Zuno Marketplace API system.

**Impact Assessment**: ✅ **Significant Impact** (substantial existing code testing and potential bug fixes required)

### Goals and Background Context

**Enhanced Goals (Measurable)**:
• Achieve minimum 85% test coverage across all 11 microservices with comprehensive regression test suite
• Identify and fix all Critical/High severity bugs in user flows: SIWE auth (0 auth failures), minting (100% transaction success), indexing (0 data loss)
• Establish performance baselines and optimize services exceeding 95th percentile response time SLAs
• Document minimum 95% of discovered bugs with root cause analysis and fix tracking
• Implement automated QA pipeline with 0 false positives in critical path monitoring

**Enhanced Background Context**:
The Zuno Marketplace API serves a production NFT marketplace with real user transactions and financial implications. While the system achieved 100% production readiness score, this focused on feature completeness and infrastructure stability. **Critical Gap**: Systematic edge case testing and stress testing under production load conditions have not been performed. This initiative ensures the system can handle unexpected user behaviors, high-load scenarios, and complex transaction edge cases that could result in financial losses or user trust issues.
