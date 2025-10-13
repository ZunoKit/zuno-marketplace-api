# Integration Points and External Dependencies

### External Services

| Service | Purpose | Integration Type | Key Files |
| ------- | ------- | ---------------- | --------- |
| IPFS/Pinata | Media storage | HTTP API | `services/media-service/` |
| Ethereum RPCs | Blockchain data | JSON-RPC | `services/chain-registry-service/` |
| Various Chains | Multi-chain support | JSON-RPC | Chain configs in registry |

### Internal Integration Points

- **gRPC Communication**: All services communicate via gRPC with circuit breakers
- **Event Bus**: RabbitMQ topic exchanges for domain events
- **WebSocket**: Real-time notifications through GraphQL subscriptions
- **Database**: No cross-service database queries (proper microservice isolation)
