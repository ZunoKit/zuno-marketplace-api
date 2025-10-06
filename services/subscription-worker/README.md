# Subscription Worker Service

This service handles real-time intent status updates and WebSocket notifications in the NFT Marketplace system.

## Architecture

The service follows Clean Architecture principles with the following structure:

```
services/subscription-worker/
├── cmd/                    # Application entry points
│   └── main.go            # Main application setup
├── internal/              # Private application code
│   ├── config/           # Configuration management
│   ├── domain/           # Business domain models and interfaces
│   ├── service/          # Business logic implementation
│   │   └── subscription_worker_service.go # Core service logic
│   └── infrastructure/   # External dependencies implementations
│       ├── events/       # Event handling (RabbitMQ)
│       ├── repository/   # Data persistence (Redis)
│       └── websocket/    # WebSocket connection management
├── Dockerfile            # Container build configuration
└── README.md            # This file
```

### Layer Responsibilities

1. **Domain Layer** (`internal/domain/`)
   - Contains business domain interfaces
   - Defines contracts for repositories and services
   - Pure business logic, no implementation details

2. **Service Layer** (`internal/service/`)
   - Implements intent matching and resolution logic
   - Coordinates WebSocket notifications
   - Processes collection domain events

3. **Infrastructure Layer** (`internal/infrastructure/`)
   - `repository/`: Implements Redis-based intent storage
   - `events/`: Handles RabbitMQ event consumption
   - `websocket/`: Manages WebSocket connections and subscriptions

## Core Features

### Intent Management
- **Intent Status Tracking**: Stores and tracks intent statuses in Redis
- **Contract-based Matching**: Matches collection events to pending intents by contract address
- **Transaction Hash Matching**: Matches intents using transaction hashes
- **Expiration Handling**: Automatically cleans up expired intents

### Real-time Notifications
- **WebSocket Server**: Provides real-time updates to connected clients
- **Subscription Management**: Handles intent-specific subscriptions
- **Connection Management**: Manages multiple simultaneous WebSocket connections
- **Health Monitoring**: Built-in health checks and statistics

### Event Processing
- **Domain Event Consumption**: Processes collection events from the catalog service
- **Intent Resolution**: Resolves pending intents when matching events are received
- **Status Broadcasting**: Broadcasts status updates to subscribed clients

## Configuration

The service uses environment variables for configuration:

```bash
# Redis Configuration
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# RabbitMQ Configuration
RABBITMQ_URL=amqp://guest:guest@localhost:5672/
RABBITMQ_EXCHANGE=nft-marketplace

# WebSocket Configuration
WEBSOCKET_HOST=0.0.0.0
WEBSOCKET_PORT=8080
WEBSOCKET_MAX_CONNECTIONS=1000

# Event Consumer Configuration
SUBSCRIPTION_QUEUE_NAME=subscription.collections.domain
```

## WebSocket API

### Connection
Connect to: `ws://localhost:8080/ws`

### Message Format
All messages use JSON format:

```json
{
  "type": "message_type",
  "intent_id": "intent_identifier",
  "data": {},
  "timestamp": "2024-01-01T00:00:00Z"
}
```

### Client Messages

#### Subscribe to Intent Updates
```json
{
  "type": "subscribe",
  "intent_id": "intent_123"
}
```

#### Unsubscribe from Intent Updates
```json
{
  "type": "unsubscribe",
  "intent_id": "intent_123"
}
```

#### Ping (Health Check)
```json
{
  "type": "ping"
}
```

### Server Messages

#### Status Update
```json
{
  "type": "status_update",
  "intent_id": "intent_123",
  "data": {
    "intent_id": "intent_123",
    "status": "success",
    "chain_id": "1",
    "tx_hash": "0x...",
    "contract_address": "0x...",
    "collection_name": "My Collection"
  },
  "timestamp": "2024-01-01T00:00:00Z"
}
```

#### Subscription Confirmation
```json
{
  "type": "subscribed",
  "intent_id": "intent_123",
  "data": {
    "status": "subscribed to intent"
  },
  "timestamp": "2024-01-01T00:00:00Z"
}
```

#### Error Message
```json
{
  "type": "error",
  "intent_id": "intent_123",
  "error": "Error description",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

## Integration Points

### Catalog Service
- **Input**: Consumes collection domain events via RabbitMQ
- **Routing Keys**: `collections.domain.upserted`, `collections.domain.created`
- **Queue**: `subscription.collections.domain`

### GraphQL Gateway
- **Output**: Provides real-time WebSocket updates to replace polling
- **Connection**: GraphQL Gateway connects as WebSocket client
- **Protocol**: Custom WebSocket message protocol

### Orchestrator Service
- **Coordination**: Intent data originates from orchestrator service
- **Storage**: Intent statuses cached in Redis for fast access

## Deployment

### Docker
```bash
# Build container
docker build -t nft-marketplace/subscription-worker .

# Run container
docker run -p 8080:8080 \
  -e REDIS_ADDR=redis:6379 \
  -e RABBITMQ_URL=amqp://guest:guest@rabbitmq:5672/ \
  nft-marketplace/subscription-worker
```

### Health Checks
- **WebSocket Health**: `GET /health` - Returns connection statistics
- **Service Health**: Internal health checks for Redis and RabbitMQ connections

## Monitoring

### Metrics Available
- Active WebSocket connections count
- Intent subscription statistics
- Event processing metrics
- Redis connection health
- RabbitMQ consumer status

### Endpoints
- `GET /health` - Health status and basic metrics
- `GET /stats` - Detailed subscription statistics
- `WebSocket /ws` - Main WebSocket endpoint

## Key Benefits

1. **Real-time Updates**: Eliminates polling overhead with push-based notifications
2. **Scalable Architecture**: Supports multiple concurrent WebSocket connections
3. **Fault Tolerance**: Automatic reconnection and error handling
4. **Intent Resolution**: Efficient matching of blockchain events to user intents
5. **Clean Separation**: Decoupled from GraphQL layer for independent scaling

## Development

### Running Locally
```bash
# Start dependencies
docker-compose up redis rabbitmq

# Run service
go run ./services/subscription-worker/cmd/main.go
```

### Testing WebSocket Connection
```bash
# Using wscat
wscat -c ws://localhost:8080/ws

# Send subscription message
{"type":"subscribe","intent_id":"test_intent_123"}
```