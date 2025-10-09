# Zuno Marketplace API

A production-ready, high-performance, multi-chain NFT marketplace backend built with microservices architecture, featuring enterprise-grade security, monitoring, and scalability.

## 🚀 Features

### Core Functionality
- **Multi-Chain Support**: Ethereum, Polygon, BSC, and other EVM chains
- **SIWE Authentication**: Secure Sign-In with Ethereum with token rotation
- **Real-time Updates**: WebSocket subscriptions for live data
- **Scalable Architecture**: Microservices with gRPC and mTLS communication
- **Advanced NFT Features**: ERC721/ERC1155 collections, minting, marketplace operations
- **Media Processing**: IPFS integration with CDN optimization
- **Comprehensive Indexing**: Real-time blockchain event processing with reorg handling

### Production-Ready Features (v1.0.0)
- **🔒 Security**: 
  - Refresh token rotation with replay attack detection
  - mTLS for all internal service communication
  - Device fingerprinting for session tracking
  - Transaction validation before processing
- **⚡ Performance**:
  - GraphQL query complexity limiting
  - Circuit breakers for external services
  - Optimized database indexes and connection pooling
  - Request timeouts at all layers
- **🛡️ Reliability**:
  - Chain reorganization handling
  - Idempotent event processing
  - Panic recovery with Sentry integration
  - Goroutine leak prevention
- **📊 Observability**:
  - Structured logging with zerolog
  - Prometheus metrics collection
  - Distributed tracing support
  - Health checks and readiness probes
- **🔧 Operations**:
  - Centralized configuration management
  - Database migration versioning
  - Comprehensive error handling
  - API documentation and SDKs

## 🏗️ Architecture

### Microservices Overview

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Frontend      │    │   GraphQL        │    │   Services      │
│   (Next.js)     │───▶│   Gateway/BFF    │───▶│   (gRPC)        │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌──────────────────┐
                       │   Message Queue  │
                       │   (RabbitMQ)     │
                       └──────────────────┘
```

### Core Services

- **Auth Service**: SIWE authentication and session management
- **User Service**: User profiles and account management
- **Wallet Service**: Multi-wallet support and approvals
- **Collection Service**: NFT collection creation and management
- **Mint Service**: NFT minting operations
- **Catalog Service**: NFT indexing and marketplace data
- **Indexer Service**: Blockchain event processing
- **Media Service**: File upload and IPFS integration

## 🛠️ Tech Stack

### Backend
- **Language**: Go 1.21+
- **Communication**: gRPC with mTLS, GraphQL
- **Message Queue**: RabbitMQ with DLX
- **Cache**: Redis with clustering support

### Databases
- **PostgreSQL**: Relational data with optimized indexes
- **MongoDB**: Document storage for events and metadata
- **Migration Tool**: golang-migrate for version control

### Infrastructure
- **Storage**: S3, IPFS, Pinata
- **Blockchain**: Multi-chain JSON-RPC with failover
- **Monitoring**: Prometheus, Sentry, structured logs
- **Security**: mTLS, JWT with rotation, rate limiting

## 📚 Documentation

### Architecture & Design
- [System Overview](./docs/architecture/system-overview.md)
- [Database Schema](./docs/architecture/database-schema.md)
- [Chain Registry](./docs/architecture/chain-registry.md)

### Implementation Guides
- [Authentication Flow](./docs/knowledge/authentication-flow.md)
- [Collection Creation](./docs/knowledge/collection-creation-flow.md)
- [Minting Process](./docs/knowledge/minting-process.md)
- [Media Handling](./docs/knowledge/media-handling.md)
- [Creation Guide](./docs/knowledge/creation-guide.md)

## 🚦 Getting Started

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- MongoDB 6.0+
- Redis 7.0+
- RabbitMQ 3.12+
- Docker & Docker Compose (optional)
- Make (for build automation)

### Quick Start with Docker

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd zuno-marketplace-api
   ```

2. **Set up environment variables**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

3. **Generate TLS certificates (for production)**
   ```bash
   cd infra/certs
   ./generate-certs.sh  # Linux/Mac
   # or
   ./generate-certs.ps1  # Windows
   ```

4. **Start all services**
   ```bash
   # Development mode
   docker-compose up -d
   
   # Production mode with TLS
   docker-compose -f docker-compose.yml -f docker-compose.tls.yml up -d
   ```

5. **Run database migrations**
   ```bash
   docker-compose exec auth-service go run cmd/migrate/main.go up
   ```

6. **Verify installation**
   ```bash
   curl http://localhost:8081/health
   curl http://localhost:8081/metrics
   ```

### Manual Installation

1. **Install dependencies**
   ```bash
   go mod download
   ```

2. **Set up databases**
   ```bash
   # PostgreSQL
   createdb nft_marketplace
   psql nft_marketplace < services/auth-service/migrations/000001_init_schema.up.sql
   psql nft_marketplace < services/auth-service/migrations/000002_add_indexes.up.sql
   
   # MongoDB
   mongosh --eval "use nft_marketplace"
   
   # Redis
   redis-cli ping
   ```

3. **Start services individually**
   ```bash
   # Terminal 1: Auth Service
   cd services/auth-service && go run cmd/main.go
   
   # Terminal 2: User Service
   cd services/user-service && go run cmd/main.go
   
   # Terminal 3: GraphQL Gateway
   cd services/graphql-gateway && go run main.go
   ```

## 🔧 Development

### Service Structure

```
services/
├── auth-service/           # Authentication & sessions
├── catalog-service/        # NFT catalog & marketplace
├── indexer-service/        # Blockchain event indexing
├── orchestrator-service/   # Transaction orchestration
└── subscription-worker/    # Real-time notifications
```

### Running Services

Each service can be run independently:

```bash
cd services/auth-service
go run main.go
```

### Testing

```bash
# Run all tests
go test ./...

# Run specific service tests
cd services/auth-service
go test ./...
```

## 📡 API Usage

### GraphQL Endpoint

```
POST /graphql
Authorization: Bearer <jwt-token>
X-Chain-Id: eip155:1
```

### Authentication with Token Rotation

Use SIWE (Sign-In with Ethereum) for authentication:

```graphql
# Step 1: Get nonce
mutation {
  signInSiwe(input: {
    accountId: "0x..."
    chainId: "eip155:1"
    domain: "app.zuno.com"
  }) {
    nonce
    message
  }
}

# Step 2: Verify signature
mutation {
  verifySiwe(input: {
    signature: "0x..."
    message: "..."
    nonce: "..."
  }) {
    accessToken
    refreshToken
    user {
      id
      wallets
    }
  }
}
```

### Collection Creation with Transaction Validation

```graphql
mutation {
  prepareCreateCollection(input: {
    name: "My Collection"
    symbol: "MC"
    chainId: "eip155:1"
    type: "ERC721"
  }) {
    intentId
    txRequest {
      to
      data
      value
      gasLimit
    }
  }
}
```

## 🚀 Production Deployment

### Security Checklist

- [ ] Generate production TLS certificates
- [ ] Enable mTLS for internal services
- [ ] Configure JWT secrets (minimum 256-bit)
- [ ] Set up refresh token rotation
- [ ] Enable device fingerprinting
- [ ] Configure rate limiting
- [ ] Set up CORS policies
- [ ] Enable security headers

### Performance Optimization

- [ ] Configure connection pool sizes per service
- [ ] Enable database query caching
- [ ] Set up Redis clustering
- [ ] Configure GraphQL query complexity limits
- [ ] Enable circuit breakers
- [ ] Set appropriate request timeouts

### Monitoring Setup

```bash
# Prometheus metrics
curl http://localhost:8081/metrics

# Health checks
curl http://localhost:8081/health

# Readiness probe
curl http://localhost:8081/ready
```

### Environment Variables

Key environment variables for production:

```env
# Security
JWT_SECRET=<256-bit-secret>
REFRESH_SECRET=<256-bit-secret>
TLS_CERT_PATH=/certs/server.crt
TLS_KEY_PATH=/certs/server.key

# Database
POSTGRES_MAX_CONNECTIONS=100
POSTGRES_POOL_SIZE=25
DB_ENABLE_SSL=true

# Redis
REDIS_CLUSTER_ENABLED=true
REDIS_POOL_SIZE=50

# Performance
MAX_QUERY_COMPLEXITY=1000
MAX_QUERY_DEPTH=10
REQUEST_TIMEOUT=30s

# Monitoring
SENTRY_DSN=https://...
PROMETHEUS_ENABLED=true
LOG_LEVEL=info
```

## 🔄 Deployment

### Environment Configuration

- **Development**: Local setup with docker-compose
- **Staging**: Kubernetes cluster with staging configs
- **Production**: Kubernetes cluster with production configs

### CI/CD Pipeline

The project uses GitHub Actions for automated testing and deployment:

- **Testing**: Run on every PR
- **Staging**: Deploy to staging on main branch
- **Production**: Deploy on release tags

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Follow Go best practices and idioms
- Write comprehensive tests for new features
- Update documentation for API changes
- Use conventional commit messages

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

- **Documentation**: Check the [docs](./docs/) directory
- **Issues**: Create an issue for bugs or feature requests
- **Discussions**: Use GitHub Discussions for questions

## 🏷️ Version

Current version: `v1.0.0`

---

**Built with ❤️ by the Zuno team**