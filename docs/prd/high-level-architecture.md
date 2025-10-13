# High Level Architecture

### Technical Summary

**Architecture Pattern**: GraphQL Gateway + gRPC Microservices
**Current Status**: Production Ready (v1.0.0) with 100/100 production readiness score

### Actual Tech Stack (from go.mod)

| Category | Technology | Version | Notes |
| -------- | ---------- | ------- | ----- |
| Runtime | Go | 1.24.5 | Latest stable |
| Gateway | GraphQL (gqlgen) | 0.17.78 | Custom complexity limiting |
| Communication | gRPC | 1.75.0 | All inter-service communication |
| Database | PostgreSQL | 14+ | Multi-schema design |
| Cache | Redis | 7.0+ | Sessions, nonces, caching |
| Document DB | MongoDB | 6.0+ | Events, metadata |
| Message Queue | RabbitMQ | 3.12+ | Event-driven architecture |
| Blockchain | go-ethereum | 1.16.2 | Multi-chain support |
| Auth | SIWE | 0.2.1 | Sign-In with Ethereum |
| Monitoring | Prometheus | 1.20.5 | Metrics collection |
| Logging | zerolog | 1.34.0 | Structured logging |

### Repository Structure Reality Check

- **Type**: Monorepo with microservices
- **Module**: Single Go module with shared packages
- **Build**: Makefile + Docker Compose + Tilt (Kubernetes)
- **Notable**: Shared packages prevent circular dependencies
