# Development and Deployment

### Local Development Setup

**Option 1: Tilt (Kubernetes - Recommended)**
```bash
tilt up  # Starts all services with hot reload
```

**Option 2: Docker Compose**
```bash
docker compose up -d  # Development mode
docker compose -f docker-compose.yml -f docker-compose.tls.yml up -d  # With mTLS
```

**Option 3: Individual Services**
```bash
cd services/auth-service && go run cmd/main.go
```

### Build and Deployment Process

- **Build**: Individual Dockerfiles in `infra/development/docker/`
- **Orchestration**: Kubernetes via Tilt or Docker Compose
- **Configuration**: Environment variables, see `.env.example`
- **TLS**: Certificate generation scripts in `infra/certs/`

### Service Ports (Standard Configuration)

- GraphQL Gateway: 8081 (HTTP/WebSocket)
- Auth Service: 50051 (gRPC)
- User Service: 50052 (gRPC)
- Wallet Service: 50053 (gRPC)
- Orchestrator Service: 50054 (gRPC)
- Media Service: 50055 (gRPC)
- Chain Registry Service: 50056 (gRPC)
- Catalog Service: 50057 (gRPC)
- Indexer Service: 50058 (gRPC)
