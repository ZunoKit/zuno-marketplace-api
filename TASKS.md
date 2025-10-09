# TASKS - Zuno Marketplace API

> Danh sách các task cần thực hiện để đưa hệ thống lên production-ready
>
> **Tổng quan:** 5 Critical | 8 High Priority | 10 Medium Priority | 5 Low Priority

---

## 🔴 CRITICAL TASKS (Làm ngay - Tuần 1-2)

### Task 1: Implement Refresh Token Rotation
**Priority:** CRITICAL
**Effort:** 3-5 ngày
**Assignee:** Backend Security Team

**Mô tả:**
Hiện tại refresh token được reuse mà không rotation, tạo lỗ hổng bảo mật nghiêm trọng.

**Files cần sửa:**
- `services/auth-service/internal/service/service.go`
- `services/auth-service/internal/domain/domain.go`
- `services/auth-service/db/up.sql`

**Checklist:**
- [ ] Thêm field `previous_refresh_hash` vào table `sessions`
- [ ] Thêm field `token_family_id` để track token chains
- [ ] Update `Refresh()` method để generate new refresh token
- [ ] Invalidate old refresh token sau khi refresh
- [ ] Implement token family tracking để detect replay attacks
- [ ] Thêm test cases cho token rotation
- [ ] Update API documentation

**Acceptance Criteria:**
- Mỗi lần refresh phải tạo refresh token mới
- Old refresh token phải bị invalidate ngay lập tức
- Detect và block token reuse attacks
- All existing tests pass + new tests có coverage >= 80%

---

### Task 2: Implement mTLS for gRPC Communication
**Priority:** CRITICAL
**Effort:** 5-7 ngày
**Assignee:** DevOps + Backend Team

**Mô tả:**
Tất cả gRPC communication đang dùng insecure credentials, data truyền plaintext.

**Files cần sửa:**
- `services/*/cmd/main.go` (tất cả services)
- `services/graphql-gateway/grpc_clients/*.go`
- `infra/development/k8s/*.yaml`
- Thêm `infra/certs/` cho certificate management

**Checklist:**
- [ ] Generate CA certificate cho internal communication
- [ ] Generate server certificates cho mỗi gRPC service
- [ ] Generate client certificates cho gateway
- [ ] Update all gRPC servers để require TLS
- [ ] Update all gRPC clients để use TLS credentials
- [ ] Setup certificate rotation mechanism
- [ ] Add certificate validation tests
- [ ] Document certificate management process
- [ ] Setup monitoring cho TLS handshake failures

**Acceptance Criteria:**
- Tất cả gRPC traffic được encrypt
- Certificate validation works correctly
- Auto-rotation trước khi cert expire
- Zero downtime during cert rotation

**References:**
```go
// Server side
creds, _ := credentials.NewServerTLSFromFile("server.crt", "server.key")
grpc.NewServer(grpc.Creds(creds))

// Client side
creds, _ := credentials.NewClientTLSFromFile("ca.crt", "")
grpc.Dial(url, grpc.WithTransportCredentials(creds))
```

---

### Task 3: Fix ERC1155 Log Parsing Implementation
**Priority:** CRITICAL
**Effort:** 2-3 ngày
**Assignee:** Blockchain Integration Team

**Mô tả:**
ERC1155 parsing đang return hardcoded values (tokenId=0, amount=1), gây corrupt dữ liệu NFT.

**Files cần sửa:**
- `services/indexer-service/internal/service/mint_indexer.go` (lines 404-435)
- `shared/contracts/` (ensure ERC1155 ABI available)

**Checklist:**
- [ ] Load ERC1155 ABI definition
- [ ] Implement proper `abi.UnpackValues()` cho TransferSingle event
- [ ] Implement proper `abi.UnpackValues()` cho TransferBatch event
- [ ] Handle big.Int tokenId và amount correctly
- [ ] Add validation cho decoded values
- [ ] Test với real blockchain data từ testnet
- [ ] Add regression tests với known ERC1155 contracts
- [ ] Verify existing ERC1155 data và fix nếu cần

**Acceptance Criteria:**
- Decode đúng tokenId từ log.Data
- Decode đúng amount từ log.Data
- Support cả single và batch transfers
- Pass tests với Polygon/BSC ERC1155 contracts

**Code Example:**
```go
erc1155ABI, _ := abi.JSON(strings.NewReader(ERC1155ABI))
var transferSingle struct {
    ID    *big.Int
    Value *big.Int
}
err := erc1155ABI.UnpackIntoInterface(&transferSingle, "TransferSingle", log.Data)
```

---

### Task 4: Fix Intent Tracking Race Condition
**Priority:** CRITICAL
**Effort:** 2-3 ngày
**Assignee:** Backend Core Team

**Mô tả:**
Race condition trong TrackTx - check-then-update pattern không thread-safe.

**Files cần sửa:**
- `services/orchestrator-service/internal/service/service.go` (lines 287-294)
- `services/orchestrator-service/db/up.sql`

**Checklist:**
- [ ] Add unique constraint `(chain_id, tx_hash)` trong DB
- [ ] Update `TrackTx()` để handle unique violation error
- [ ] Thay check-then-update bằng upsert hoặc handle DB constraint
- [ ] Add distributed lock nếu cần (Redis-based)
- [ ] Add concurrent test cases
- [ ] Load testing để verify race condition fixed

**Acceptance Criteria:**
- Không thể có 2 intents với cùng tx_hash
- Concurrent requests handle correctly
- Proper error messages cho duplicate tx attempts
- No data inconsistency under load

**Migration SQL:**
```sql
ALTER TABLE tx_intents
ADD CONSTRAINT unique_chain_tx
UNIQUE (chain_id, tx_hash)
WHERE tx_hash IS NOT NULL;
```

---

### Task 5: Add GraphQL Query Complexity Limiting
**Priority:** CRITICAL
**Effort:** 2-3 ngày
**Assignee:** API Gateway Team

**Mô tả:**
GraphQL không có query complexity limits, dễ bị DoS với nested queries.

**Files cần sửa:**
- `services/graphql-gateway/main.go`
- `services/graphql-gateway/graphql/resolver.go`

**Checklist:**
- [ ] Define complexity costs cho mỗi field
- [ ] Implement complexity calculator middleware
- [ ] Set max complexity limit (e.g., 1000 points)
- [ ] Add depth limiting (e.g., max 10 levels)
- [ ] Log complex queries để monitor abuse
- [ ] Add rate limiting per user/IP
- [ ] Test với malicious nested queries
- [ ] Document complexity costs trong schema

**Acceptance Criteria:**
- Reject queries exceeding complexity limit
- Return clear error messages với complexity score
- Normal queries không bị affect
- DoS attacks bị block effectively

**Implementation:**
```go
import "github.com/99designs/gqlgen/graphql/handler/extension"

srv.Use(extension.FixedComplexityLimit(1000))
```

---

## 🔥 HIGH PRIORITY TASKS (Tuần 3-6)

### Task 6: Implement Chain Reorganization Handling
**Priority:** HIGH
**Effort:** 5-7 ngày
**Assignee:** Blockchain Team

**Files cần sửa:**
- `services/indexer-service/internal/service/indexer_service.go`
- `services/indexer-service/db/up.sql`
- `services/catalog-service/internal/service/catalog_service.go`

**Checklist:**
- [ ] Add `previous_block_hash` field vào `indexer_checkpoints`
- [ ] Implement block hash continuity check
- [ ] Add reorg detection logic
- [ ] Implement rollback mechanism (64 blocks safe depth)
- [ ] Mark affected data as `reorged` trong catalog
- [ ] Re-index từ safe checkpoint khi detect reorg
- [ ] Add reorg event notifications
- [ ] Test với testnet reorg scenarios

**Acceptance Criteria:**
- Detect reorgs within 1 block
- Rollback và re-index correctly
- No data corruption sau reorg
- Users notified về affected transactions

---

### Task 7: Add Circuit Breakers to All gRPC Calls
**Priority:** HIGH
**Effort:** 3-5 ngày
**Assignee:** Backend Reliability Team

**Files cần sửa:**
- `services/graphql-gateway/grpc_clients/*.go`
- `shared/resilience/circuit_breaker.go` (already exists)

**Checklist:**
- [ ] Wrap all gRPC client calls với circuit breaker
- [ ] Configure failure thresholds per service
- [ ] Implement fallback strategies
- [ ] Add metrics cho circuit breaker states
- [ ] Test failure scenarios
- [ ] Document circuit breaker behavior

**Configuration:**
```go
cb := circuitbreaker.New(
    circuitbreaker.WithFailureThreshold(5),
    circuitbreaker.WithTimeout(10*time.Second),
    circuitbreaker.WithCooldown(30*time.Second),
)
```

---

### Task 8: Fix Idempotency in Catalog Service
**Priority:** HIGH
**Effort:** 3-4 ngày
**Assignee:** Backend Core Team

**Files cần sửa:**
- `services/catalog-service/internal/service/catalog_service.go` (lines 40-80)
- `services/catalog-service/internal/repository/*.go`

**Checklist:**
- [ ] Wrap event processing trong database transaction
- [ ] Make `MarkProcessed` và data updates atomic
- [ ] Add transaction rollback trên errors
- [ ] Implement retry logic cho failed events
- [ ] Add dead-letter queue cho permanently failed events
- [ ] Test idempotency với duplicate events

**Acceptance Criteria:**
- Event processing là atomic
- Duplicate events không affect data
- Failed events có thể retry safely
- No data loss hoặc corruption

---

### Task 9: Add Transaction Validation Before Tracking
**Priority:** HIGH
**Effort:** 4-5 ngày
**Assignee:** Blockchain Integration Team

**Files cần sửa:**
- `services/orchestrator-service/internal/service/service.go`
- Add new `internal/blockchain/validator.go`

**Checklist:**
- [ ] Validate tx exists on-chain before accepting
- [ ] Verify tx was sent to correct contract address
- [ ] Check tx matches intent type (collection/mint)
- [ ] Validate tx hasn't already failed on-chain
- [ ] Add timeout cho blockchain validation calls
- [ ] Implement caching cho validated txs
- [ ] Handle pending/not-mined txs appropriately

**Acceptance Criteria:**
- Reject fake/invalid transaction hashes
- Validate contract address matches
- Proper error messages cho invalid txs
- Performance không bị ảnh hưởng đáng kể

---

### Task 10: Implement Unified Error Handling Pattern
**Priority:** HIGH
**Effort:** 5-7 ngày
**Assignee:** All Backend Teams

**Files cần sửa:**
- `shared/errors/` (new package)
- All services `internal/domain/error.go`

**Checklist:**
- [ ] Define error types hierarchy
- [ ] Implement error wrapping với context
- [ ] Add error code system
- [ ] Create error translation layer (domain → gRPC → GraphQL)
- [ ] Add structured error logging
- [ ] Update all services để use new pattern
- [ ] Document error handling guidelines

**Error Types:**
```go
// Domain errors
ErrNotFound
ErrUnauthorized
ErrInvalidInput
ErrConflict
ErrInternal

// With context
errors.Wrap(ErrNotFound, "collection", collectionId)
```

---

## 🟠 MEDIUM PRIORITY TASKS (Tháng 2-3)

### Task 11: Add Foreign Key Constraints
**Priority:** MEDIUM
**Effort:** 3-4 ngày

**Checklist:**
- [ ] Analyze cross-service references
- [ ] Decide: FK constraints vs compensating transactions
- [ ] Implement chosen strategy
- [ ] Add cascade delete logic nếu cần
- [ ] Test data integrity

---

### Task 12: Implement Session Fingerprinting
**Priority:** MEDIUM
**Effort:** 2-3 ngày

**Checklist:**
- [ ] Track device fingerprint (IP, User-Agent, etc)
- [ ] Validate fingerprint on session refresh
- [ ] Add configurable strictness levels
- [ ] Handle legitimate device/IP changes
- [ ] Add suspicious activity detection

---

### Task 13: Migrate to Structured Logging
**Priority:** MEDIUM
**Effort:** 4-5 ngày

**Checklist:**
- [ ] Choose logger (zerolog vs zap)
- [ ] Create logging wrapper package
- [ ] Replace all `fmt.Printf`, `log.Printf`
- [ ] Add context fields (user_id, session_id, request_id)
- [ ] Setup log aggregation (ELK/Loki)
- [ ] Document logging standards

---

### Task 14: Configuration Management
**Priority:** MEDIUM
**Effort:** 3-4 ngày

**Checklist:**
- [ ] Move hardcoded values to config
- [ ] Implement config validation
- [ ] Add environment-specific configs
- [ ] Setup config reload without restart
- [ ] Document all config options

**Hardcoded Values to Extract:**
- Session TTL (24 hours)
- Nonce TTL (5 minutes)
- Indexer batch size (100 blocks)
- JWT expiration (1 hour)
- Rate limits

---

### Task 15: Database Migration Versioning
**Priority:** MEDIUM
**Effort:** 2-3 ngày

**Checklist:**
- [ ] Setup golang-migrate or Goose
- [ ] Convert existing up.sql to versioned migrations
- [ ] Add down migrations
- [ ] Implement migration CI/CD
- [ ] Document migration process

---

### Task 16: Add Request Timeouts
**Priority:** MEDIUM
**Effort:** 2 ngày

**Checklist:**
- [ ] Add context timeout to all gRPC calls
- [ ] Configure per-service timeout values
- [ ] Add database query timeouts
- [ ] Handle timeout errors gracefully
- [ ] Monitor timeout occurrences

---

### Task 17: Fix Goroutine Leak in Rate Limiter
**Priority:** MEDIUM
**Effort:** 1-2 ngày

**Files:** `services/auth-service/internal/middleware/ratelimit.go`

**Checklist:**
- [ ] Add done channel to rate limiter
- [ ] Implement graceful shutdown
- [ ] Add goroutine leak tests
- [ ] Verify cleanup on server stop

---

### Task 18: Add Panic Recovery Middleware
**Priority:** MEDIUM
**Effort:** 1 ngày

**Checklist:**
- [ ] Add `grpc_recovery.UnaryServerInterceptor()`
- [ ] Add `grpc_recovery.StreamServerInterceptor()`
- [ ] Log panic stack traces
- [ ] Alert on panic occurrences
- [ ] Test panic scenarios

---

### Task 19: Implement Auth Schema Directives
**Priority:** MEDIUM
**Effort:** 2-3 ngày

**Checklist:**
- [ ] Define `@auth` directive trong schema
- [ ] Implement directive validation logic
- [ ] Mark all protected resolvers
- [ ] Remove manual auth checks
- [ ] Test unauthorized access

---

### Task 20: Add GraphQL Rate Limiting
**Priority:** MEDIUM
**Effort:** 2-3 ngày

**Checklist:**
- [ ] Implement rate limiter middleware
- [ ] Support per-user và per-IP limits
- [ ] Add operation-specific limits
- [ ] Return proper rate limit headers
- [ ] Monitor rate limit violations

---

## 🟡 LOW PRIORITY TASKS (Technical Debt)

### Task 21: Database Index Optimization
**Priority:** LOW
**Effort:** 2-3 ngày

**Checklist:**
- [ ] Add missing FK indexes
- [ ] Optimize partial indexes
- [ ] Add covering indexes cho hot queries
- [ ] Run EXPLAIN ANALYZE trên slow queries
- [ ] Document indexing strategy

---

### Task 22: Connection Pool Tuning
**Priority:** LOW
**Effort:** 1-2 ngày

**Checklist:**
- [ ] Configure DB connection pool settings
- [ ] Monitor connection usage
- [ ] Tune pool sizes per service load
- [ ] Add connection pool metrics

---

### Task 23: Add Metrics Collection
**Priority:** LOW
**Effort:** 3-4 ngày

**Checklist:**
- [ ] Implement Prometheus metrics
- [ ] Add custom business metrics
- [ ] Create Grafana dashboards
- [ ] Setup alerting rules

---

### Task 24: Improve Test Coverage
**Priority:** LOW
**Effort:** Ongoing

**Checklist:**
- [ ] Add integration tests
- [ ] Add E2E test scenarios
- [ ] Mock external dependencies
- [ ] Target 80% coverage minimum

---

### Task 25: API Documentation
**Priority:** LOW
**Effort:** 3-5 ngày

**Checklist:**
- [ ] Generate OpenAPI specs
- [ ] Document GraphQL schema
- [ ] Create API usage guides
- [ ] Add code examples
- [ ] Setup docs website

---

## 📊 TASK TRACKING

### Sprint Planning

**Sprint 1 (Week 1-2): Critical Security**
- Task 1: Refresh Token Rotation
- Task 2: mTLS Implementation
- Task 3: ERC1155 Fix
- Task 4: Race Condition Fix
- Task 5: Query Complexity Limiting

**Sprint 2 (Week 3-4): Core Reliability**
- Task 6: Chain Reorg Handling
- Task 7: Circuit Breakers
- Task 8: Idempotency Fix

**Sprint 3 (Week 5-6): Production Hardening**
- Task 9: Transaction Validation
- Task 10: Error Handling
- Task 18: Panic Recovery

**Sprint 4-6 (Month 2-3): Medium Priority**
- Tasks 11-20

**Continuous (Month 3+): Technical Debt**
- Tasks 21-25

---

## 🎯 SUCCESS METRICS

### Security Metrics
- [ ] 0 critical vulnerabilities
- [ ] 0 high-severity vulnerabilities
- [ ] All data encrypted in transit

### Reliability Metrics
- [ ] 99.9% uptime SLA
- [ ] < 1% error rate
- [ ] < 500ms p95 latency

### Quality Metrics
- [ ] 80%+ test coverage
- [ ] 0 critical bugs in production
- [ ] < 5 medium bugs per month

### Performance Metrics
- [ ] < 100ms p50 API response time
- [ ] < 500ms p95 API response time
- [ ] Support 1000+ concurrent users

---

## 📝 NOTES

### Dependencies
- Task 2 (mTLS) blocks production deployment
- Task 3 (ERC1155) blocks ERC1155 support
- Task 10 (Error Handling) should be done before other tasks

### Resources Needed
- 2-3 Backend Engineers (full-time)
- 1 DevOps Engineer (part-time)
- 1 Security Engineer (consultant)
- 1 QA Engineer (full-time)

### Estimated Timeline
- **Critical Tasks:** 2 weeks
- **High Priority:** 4 weeks
- **Medium Priority:** 8 weeks
- **Total to Production-Ready:** 10-12 weeks

### Risk Factors
- Breaking changes trong auth flow (Task 1)
- Certificate management complexity (Task 2)
- Data migration cho ERC1155 fix (Task 3)
- Team availability và skill gaps

---

## 📚 REFERENCES

- [SIWE Specification](https://eips.ethereum.org/EIPS/eip-4361)
- [gRPC Authentication Guide](https://grpc.io/docs/guides/auth/)
- [GraphQL Security Best Practices](https://graphql.org/learn/best-practices/)
- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [Microservices Security](https://www.oreilly.com/library/view/microservices-security-in/9781617295959/)

---

**Last Updated:** 2025-10-09
**Next Review:** Weekly during Sprints 1-3, Monthly thereafter
