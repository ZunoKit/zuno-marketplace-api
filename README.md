# Zuno Marketplace API

A high-performance, multi-chain NFT marketplace backend built with microservices architecture.

## 🚀 Features

- **Multi-Chain Support**: Ethereum, Polygon, BSC, and more
- **SIWE Authentication**: Secure Sign-In with Ethereum
- **Real-time Updates**: WebSocket subscriptions for live data
- **Scalable Architecture**: Microservices with gRPC communication
- **Advanced NFT Features**: Collections, minting, marketplace operations
- **Media Processing**: IPFS integration with CDN optimization
- **Comprehensive Indexing**: Real-time blockchain event processing

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
- **Language**: Go
- **Communication**: gRPC, GraphQL
- **Message Queue**: RabbitMQ
- **Cache**: Redis

### Databases
- **PostgreSQL**: Relational data (auth, users, collections)
- **MongoDB**: Document storage (events, metadata)

### Infrastructure
- **Storage**: S3, IPFS
- **Blockchain**: JSON-RPC endpoints
- **Monitoring**: (Configure as needed)

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

### Installation

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd zuno-marketplace-api
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Set up environment variables**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

4. **Run database migrations**
   ```bash
   # Add migration commands here
   ```

5. **Start services**
   ```bash
   # Start individual services or use docker-compose
   docker-compose up -d
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
```

### Authentication

Use SIWE (Sign-In with Ethereum) for authentication:

```graphql
mutation {
  signInSiwe(input: {
    accountId: "0x..."
    chainId: "eip155:1"
    domain: "app.zuno.com"
  }) {
    nonce
  }
}
```

### Collection Creation

```graphql
mutation {
  prepareCreateCollection(input: {
    name: "My Collection"
    symbol: "MC"
    chainId: "eip155:1"
  }) {
    intentId
    txRequest {
      to
      data
      value
    }
  }
}
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