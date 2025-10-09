# ✅ PRODUCTION DEPLOYMENT CHECKLIST

**Project**: Zuno Marketplace API v1.0.0
**Status**: All 25 tasks completed (100%)
**Last Updated**: 2025-01-10
**Deployment Target**: Production Environment

---

## 🎯 OVERVIEW

This checklist must be completed **in order** before deploying to production. Each section builds on the previous one.

**Estimated Total Time**: 4-6 hours
**Team Required**: Backend Lead + DevOps Engineer + QA Tester

---

## 📋 PHASE 1: CODE VERIFICATION (30-45 min)

### ✅ Critical Fixes Verification

#### Issue #1: Circuit Breaker Integration
- [ ] **Verify** `services/graphql-gateway/grpc_clients/client_with_resilience.go` exists
- [ ] **Check** All 6 gRPC clients use `NewResilientClient()`:
  - [ ] `auth_client.go`
  - [ ] `wallet_client.go`
  - [ ] `media_client.go`
  - [ ] `chain_registry_client.go`
  - [ ] `orchestrator_client.go`
  - [ ] `user_client.go` (if exists)
- [ ] **Verify** Circuit breaker config in each client:
  ```go
  MaxFailures: 5
  ResetTimeout: 60s
  HalfOpenMaxCalls: 3
  ```
- [ ] **Check** Prometheus metrics exposed: `circuit_breaker_state{service="..."}`

#### Issue #2: ERC1155 Batch Mint ABI Unpacking
- [ ] **Verify** `services/indexer-service/internal/service/mint_indexer.go` has:
  - [ ] `import "github.com/ethereum/go-ethereum/accounts/abi"`
  - [ ] `parseERC1155TransferBatch()` function exists
  - [ ] Uses `abi.UnpackIntoMap()` for data field
  - [ ] Properly converts `ids[]` and `values[]` BigInt arrays to strings
- [ ] **Check** `shared/contracts/ERC1155.json` ABI file exists
- [ ] **Verify** Domain models in `services/indexer-service/internal/domain/events.go`:
  ```go
  type TransferBatchEvent struct {
      Operator string
      From     string
      To       string
      Ids      []string
      Values   []string
  }
  ```

#### Issue #3: Panic Recovery Interceptors
- [ ] **Verify** All 6 gRPC services have panic recovery:
  - [ ] `services/auth-service/cmd/main.go`
  - [ ] `services/user-service/cmd/main.go`
  - [ ] `services/wallet-service/cmd/main.go`
  - [ ] `services/orchestrator-service/cmd/main.go`
  - [ ] `services/media-service/cmd/main.go`
  - [ ] `services/chain-registry-service/cmd/main.go`
- [ ] **Check** Each service has both interceptors:
  ```go
  grpc.ChainUnaryInterceptor(
      grpc_recovery.UnaryServerInterceptor(recoveryOpts...),
  ),
  grpc.ChainStreamInterceptor(
      grpc_recovery.StreamServerInterceptor(recoveryOpts...),
  )
  ```
- [ ] **Verify** Recovery handler logs to zerolog with stack traces
- [ ] **Verify** Returns `codes.Internal` gRPC error on panic

---

## 🔨 PHASE 2: BUILD & COMPILE (15-20 min)

### Local Build Verification
```bash
# Run these commands and verify NO errors
cd E:\zuno-marketplace-api
```

- [ ] **Install dependencies**: `go mod download`
- [ ] **Verify go.mod**: `go mod tidy` (should show no changes)
- [ ] **Build all services**:
  ```bash
  cd infra/development/build

  # Windows
  ./auth-build.bat
  ./user-build.bat
  ./wallet-build.bat
  ./orchestrator-build.bat
  ./media-build.bat
  ./chain-registry-build.bat
  ./catalog-build.bat
  ./indexer-build.bat
  ./subscription-worker-build.bat
  ./graphql-gateway-build.bat

  # Verify all builds succeeded (check build/ directory)
  dir ..\..\..\..\build
  ```
- [ ] **Run linter**: `golangci-lint run ./...`
  - [ ] Zero critical errors
  - [ ] Zero security warnings from `gosec`

### Docker Build Verification
```bash
cd E:\zuno-marketplace-api
```

- [ ] **Build all Docker images**:
  ```bash
  docker compose build --no-cache
  ```
- [ ] **Verify images created** (10 services):
  ```bash
  docker images | grep zuno-marketplace
  ```
- [ ] **Check image sizes** (should be < 100MB each for Alpine-based)

---

## 🧪 PHASE 3: TESTING (1-2 hours)

### Unit Tests
```bash
cd E:\zuno-marketplace-api
```

- [ ] **Run all unit tests**:
  ```bash
  go test ./... -v -race -coverprofile=coverage.out
  ```
- [ ] **Verify coverage** (aim for > 70%):
  ```bash
  go tool cover -func=coverage.out | grep total
  ```
- [ ] **Check critical packages**:
  - [ ] `shared/resilience` - Circuit breaker tests pass
  - [ ] `services/auth-service` - Token rotation tests pass
  - [ ] `services/indexer-service` - ERC1155 parsing tests pass
  - [ ] `shared/recovery` - Panic recovery tests pass

### Integration Tests
```bash
# Start dependencies first
docker compose up -d postgres redis mongodb rabbitmq
```

- [ ] **Database connection tests**:
  ```bash
  go test ./services/auth-service/internal/infrastructure/repository/... -v
  ```
- [ ] **Redis connection tests**:
  ```bash
  go test ./shared/redis/... -v
  ```
- [ ] **RabbitMQ connection tests**:
  ```bash
  go test ./shared/messaging/... -v
  ```

### End-to-End Tests
```bash
# Start ALL services
docker compose up -d
```

- [ ] **Wait for services to be healthy** (check logs):
  ```bash
  docker compose ps
  docker compose logs --tail=50
  ```
- [ ] **Run E2E test suite**:
  ```bash
  cd test/e2e
  go test -v ./... -timeout 10m
  ```
- [ ] **Critical E2E flows**:
  - [ ] Authentication flow (SIWE sign-in + refresh)
  - [ ] Collection creation flow
  - [ ] NFT minting flow
  - [ ] WebSocket subscription flow

---

## 🔐 PHASE 4: SECURITY VERIFICATION (30 min)

### TLS/mTLS Setup
```bash
cd infra/certs
```

- [ ] **Generate production certificates**:
  ```bash
  # Windows
  ./generate-certs.ps1

  # Linux/Mac
  ./generate-certs.sh
  ```
- [ ] **Verify certificate files created**:
  - [ ] `ca.crt`, `ca.key` (Certificate Authority)
  - [ ] `server.crt`, `server.key` (Server certificates)
  - [ ] `client.crt`, `client.key` (Client certificates)
- [ ] **Set proper permissions** (Linux/Mac only):
  ```bash
  chmod 400 *.key
  chmod 644 *.crt
  ```
- [ ] **Test mTLS connection**:
  ```bash
  docker compose -f docker-compose.yml -f docker-compose.tls.yml up -d auth-service

  openssl s_client -connect localhost:50051 \
    -cert client.crt -key client.key -CAfile ca.crt

  # Should show "Verify return code: 0 (ok)"
  ```

### Environment Variables Security
- [ ] **Review `.env.example`** - No secrets committed
- [ ] **Create production `.env`**:
  ```env
  # JWT Secrets (MUST be >= 256 bits / 32 bytes)
  JWT_SECRET=<generate-with-openssl-rand-hex-32>
  REFRESH_SECRET=<generate-with-openssl-rand-hex-32>

  # Database credentials
  POSTGRES_PASSWORD=<strong-password>
  MONGO_ROOT_PASSWORD=<strong-password>
  REDIS_PASSWORD=<strong-password>
  RABBITMQ_DEFAULT_PASS=<strong-password>

  # Security settings
  TLS_ENABLED=true
  RATE_LIMIT_ENABLED=true
  MAX_QUERY_COMPLEXITY=1000
  MAX_QUERY_DEPTH=10

  # Monitoring
  SENTRY_DSN=<your-production-sentry-dsn>
  SENTRY_ENVIRONMENT=production
  PROMETHEUS_ENABLED=true

  # Blockchain RPCs (use paid providers for production)
  ETHEREUM_RPC_URL=<infura-or-alchemy-url>
  POLYGON_RPC_URL=<polygon-rpc-url>
  BSC_RPC_URL=<bsc-rpc-url>
  ```
- [ ] **Generate secrets**:
  ```bash
  # JWT Secret
  openssl rand -hex 32

  # Refresh Secret
  openssl rand -hex 32

  # Database passwords
  openssl rand -base64 24
  ```
- [ ] **Verify `.env` is in `.gitignore`**

### Security Scan
- [ ] **Run Trivy security scan**:
  ```bash
  trivy image zuno-marketplace-auth-service:latest
  trivy image zuno-marketplace-graphql-gateway:latest
  ```
- [ ] **Check for HIGH/CRITICAL vulnerabilities**
- [ ] **Run gosec** (already done in build phase, but double-check):
  ```bash
  gosec -fmt json -out gosec-report.json ./...
  ```
- [ ] **Review gosec report** - Zero critical issues

---

## 🚀 PHASE 5: DEPLOYMENT DRY RUN (45-60 min)

### Staging Environment Deployment
```bash
cd E:\zuno-marketplace-api
```

- [ ] **Deploy to staging**:
  ```bash
  # Option 1: Docker Compose (staging)
  docker compose -f docker-compose.yml -f docker-compose.tls.yml up -d

  # Option 2: Kubernetes/Tilt (staging)
  tilt up
  ```
- [ ] **Verify all services running**:
  ```bash
  # Docker
  docker compose ps

  # Kubernetes
  kubectl get pods -n dev
  ```
- [ ] **Check logs for errors**:
  ```bash
  # Docker
  docker compose logs --tail=100 | grep -i error

  # Kubernetes
  kubectl logs -n dev -l app=auth-service --tail=100
  ```

### Health Checks
- [ ] **GraphQL Gateway**: `curl http://localhost:8081/health`
  - [ ] Returns `200 OK`
- [ ] **Prometheus Metrics**: `curl http://localhost:8081/metrics`
  - [ ] Returns metrics in Prometheus format
  - [ ] Verify `circuit_breaker_state` metric exists
- [ ] **Database connectivity**:
  ```bash
  docker compose exec postgres psql -U postgres -c "\l"
  docker compose exec mongodb mongosh --eval "db.adminCommand('ping')"
  docker compose exec redis redis-cli ping
  ```
- [ ] **RabbitMQ Management**: http://localhost:15672
  - [ ] Login with credentials
  - [ ] Verify exchanges exist: `auth.events`, `wallets.events`

### Smoke Tests
- [ ] **Test 1: Authentication Flow**
  ```graphql
  mutation SignIn {
    signInSiwe(address: "0x1234...", chainId: "eip155:1") {
      nonce
      message
    }
  }

  mutation Verify {
    verifySiwe(address: "0x1234...", signature: "0xabc...", chainId: "eip155:1") {
      accessToken
      expiresAt
    }
  }

  mutation Refresh {
    refreshSession {
      accessToken
      expiresAt
    }
  }
  ```
  - [ ] Sign-in returns nonce
  - [ ] Verify returns access token
  - [ ] Refresh rotates token successfully

- [ ] **Test 2: Circuit Breaker Behavior**
  ```bash
  # Stop auth-service
  docker compose stop auth-service

  # Make 10 GraphQL requests (should fail fast after 5)
  for i in {1..10}; do
    curl -X POST http://localhost:8081/graphql \
      -H "Content-Type: application/json" \
      -d '{"query": "query { me { id } }"}' &
  done

  # Check circuit breaker state
  curl http://localhost:8081/metrics | grep circuit_breaker_state
  # Should show state="open"

  # Restart service
  docker compose start auth-service

  # Wait 60 seconds, circuit should close
  sleep 60
  curl http://localhost:8081/metrics | grep circuit_breaker_state
  # Should show state="closed"
  ```

- [ ] **Test 3: Panic Recovery**
  ```bash
  # Trigger a panic in any service (if you have a test endpoint)
  # OR check logs for panic recovery behavior

  docker compose logs auth-service | grep -i panic
  # Should show recovery messages, NOT crash

  # Verify service is still running
  docker compose ps auth-service
  # Should show "Up" status
  ```

- [ ] **Test 4: ERC1155 Batch Mint**
  ```bash
  # Deploy a test batch mint transaction to testnet
  # OR use a known batch mint tx hash

  # Trigger indexer to process the transaction
  # Check catalog database for batch minted NFTs

  docker compose exec postgres psql -U postgres -d marketplace \
    -c "SELECT * FROM catalog.nfts WHERE token_id IN ('1', '2', '3');"

  # Should show all 3 NFTs from batch mint
  ```

---

## 📊 PHASE 6: MONITORING SETUP (30 min)

### Prometheus & Grafana
- [ ] **Verify Prometheus scraping**:
  ```bash
  curl http://localhost:9090/api/v1/targets
  # All targets should be "up"
  ```
- [ ] **Import Grafana dashboards**:
  - [ ] Service health dashboard
  - [ ] Database metrics dashboard
  - [ ] Circuit breaker dashboard
  - [ ] Error rate dashboard

### Sentry Integration
- [ ] **Verify Sentry DSN configured** in `.env`
- [ ] **Test error reporting**:
  ```bash
  # Trigger a test error
  curl -X POST http://localhost:8081/graphql \
    -d '{"query": "mutation { triggerTestError }"}'

  # Check Sentry dashboard
  # Should show error event
  ```

### Log Aggregation
- [ ] **Verify structured logs**:
  ```bash
  docker compose logs auth-service | tail -20

  # Should see JSON logs with:
  # - level (info, error, warn)
  # - timestamp
  # - service
  # - trace_id
  # - message
  ```
- [ ] **Set up log forwarding** (if using ELK/Loki):
  - [ ] Configure log shipper
  - [ ] Verify logs appear in aggregation system

### Alerts Configuration
- [ ] **Create Prometheus alert rules**:
  ```yaml
  # High error rate alert
  - alert: HighErrorRate
    expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
    for: 5m
    severity: critical

  # Circuit breaker open alert
  - alert: CircuitBreakerOpen
    expr: circuit_breaker_state{state="open"} == 1
    for: 2m
    severity: warning

  # Service down alert
  - alert: ServiceDown
    expr: up{job="auth-service"} == 0
    for: 1m
    severity: critical
  ```
- [ ] **Test alerts**:
  ```bash
  # Stop a service
  docker compose stop auth-service

  # Wait for alert to fire (check Prometheus Alerts page)
  # Should see "ServiceDown" alert

  # Restart service
  docker compose start auth-service
  ```

---

## 🗄️ PHASE 7: DATABASE READINESS (20 min)

### Schema Verification
- [ ] **Check all schemas exist**:
  ```sql
  -- PostgreSQL
  SELECT schema_name FROM information_schema.schemata
  WHERE schema_name IN ('auth', 'user', 'wallets', 'chain_registry',
                         'orchestrator', 'catalog', 'indexer');
  ```
  - [ ] All 7 schemas present

- [ ] **Verify critical tables**:
  ```sql
  -- Auth schema
  SELECT tablename FROM pg_tables WHERE schemaname = 'auth';
  -- Should show: auth_nonces, sessions, login_events

  -- Orchestrator schema
  SELECT tablename FROM pg_tables WHERE schemaname = 'orchestrator';
  -- Should show: tx_intents, session_intent_audit

  -- Catalog schema
  SELECT tablename FROM pg_tables WHERE schemaname = 'catalog';
  -- Should show: collections, nfts, token_balances, listings, offers, sales
  ```

### Indexes Verification
- [ ] **Check indexes created**:
  ```sql
  SELECT schemaname, tablename, indexname, indexdef
  FROM pg_indexes
  WHERE schemaname IN ('auth', 'orchestrator', 'catalog', 'indexer')
  ORDER BY schemaname, tablename;
  ```
  - [ ] `unique_chain_tx` constraint exists on `tx_intents`
  - [ ] `idx_sessions_refresh_hash` exists
  - [ ] `idx_nfts_collection_chain` exists
  - [ ] BRIN indexes exist on time-series columns

### Migrations Status
- [ ] **Verify migration system**:
  ```bash
  # Check migration history
  docker compose exec postgres psql -U postgres -d marketplace \
    -c "SELECT * FROM schema_migrations ORDER BY version;"

  # Should show all migrations applied
  ```

### MongoDB Collections
- [ ] **Verify MongoDB collections**:
  ```bash
  docker compose exec mongodb mongosh

  use events
  show collections
  # Should show: raw

  use metadata
  show collections
  # Should show: docs

  use media
  show collections
  # Should show: assets, variants
  ```

### Backup & Recovery
- [ ] **Test backup script**:
  ```bash
  # PostgreSQL backup
  docker compose exec postgres pg_dump -U postgres marketplace > backup.sql

  # MongoDB backup
  docker compose exec mongodb mongodump --archive=mongo_backup.gz --gzip
  ```
- [ ] **Test restore**:
  ```bash
  # Create test database
  docker compose exec postgres createdb -U postgres test_restore

  # Restore backup
  docker compose exec postgres psql -U postgres test_restore < backup.sql

  # Verify data
  docker compose exec postgres psql -U postgres test_restore \
    -c "SELECT count(*) FROM auth.sessions;"
  ```

---

## 🌐 PHASE 8: PRODUCTION DEPLOYMENT (30 min)

### Pre-Deployment
- [ ] **Tag release**:
  ```bash
  git tag -a v1.0.0 -m "Production release v1.0.0"
  git push origin v1.0.0
  ```
- [ ] **Build production Docker images**:
  ```bash
  docker compose -f docker-compose.yml -f docker-compose.tls.yml build --no-cache
  ```
- [ ] **Push to container registry**:
  ```bash
  docker tag zuno-marketplace-auth-service:latest registry.example.com/zuno/auth:v1.0.0
  docker push registry.example.com/zuno/auth:v1.0.0
  # Repeat for all services
  ```

### Deployment
- [ ] **Deploy to production** (choose your method):

  **Option A: Docker Compose**
  ```bash
  # On production server
  docker compose -f docker-compose.yml -f docker-compose.tls.yml up -d
  ```

  **Option B: Kubernetes**
  ```bash
  kubectl apply -f infra/production/k8s/
  kubectl rollout status deployment/auth-service -n production
  ```

- [ ] **Verify deployment**:
  ```bash
  # Docker
  docker compose ps
  # All services should be "Up"

  # Kubernetes
  kubectl get pods -n production
  # All pods should be "Running"
  ```

### Post-Deployment Verification
- [ ] **Health checks pass**:
  ```bash
  curl https://api.zunomarketplace.com/health
  # Returns 200 OK
  ```
- [ ] **Metrics endpoint accessible**:
  ```bash
  curl https://api.zunomarketplace.com/metrics
  # Returns Prometheus metrics
  ```
- [ ] **GraphQL Playground accessible** (disable in production after testing):
  ```bash
  curl https://api.zunomarketplace.com/playground
  ```

---

## 📈 PHASE 9: POST-DEPLOYMENT MONITORING (1-2 hours)

### Immediate Monitoring (First 30 min)
- [ ] **Watch error logs**:
  ```bash
  # Docker
  docker compose logs -f | grep -i error

  # Kubernetes
  kubectl logs -f -l app.kubernetes.io/name=auth-service -n production | grep -i error
  ```
- [ ] **Monitor error rate** in Prometheus:
  ```promql
  rate(http_requests_total{status=~"5.."}[5m])
  # Should be < 0.01 (1%)
  ```
- [ ] **Check circuit breaker state**:
  ```promql
  circuit_breaker_state
  # All should be state="closed"
  ```
- [ ] **Monitor response times**:
  ```promql
  histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
  # P95 should be < 500ms
  ```

### Performance Baselines
- [ ] **Establish baseline metrics**:
  ```promql
  # Requests per second
  rate(http_requests_total[5m])

  # Database query time
  histogram_quantile(0.95, rate(db_query_duration_seconds_bucket[5m]))

  # Cache hit rate
  redis_cache_hits / (redis_cache_hits + redis_cache_misses)
  ```
- [ ] **Document baselines** in monitoring dashboard

### User Acceptance Testing
- [ ] **Test critical user flows**:
  - [ ] Connect wallet and sign in
  - [ ] Create collection
  - [ ] Mint NFT
  - [ ] List NFT for sale
  - [ ] Make offer on NFT
  - [ ] Accept offer / complete sale
  - [ ] Transfer NFT
- [ ] **Verify real-time notifications**:
  - [ ] WebSocket connection establishes
  - [ ] Subscription updates received
  - [ ] Events appear in correct order

---

## ✅ FINAL SIGN-OFF

### Team Sign-Off
- [ ] **Backend Lead**: Code review completed ✅
- [ ] **DevOps Engineer**: Infrastructure verified ✅
- [ ] **QA Tester**: All tests passed ✅
- [ ] **Security Officer**: Security audit approved ✅
- [ ] **Product Owner**: UAT approved ✅

### Documentation Sign-Off
- [ ] **README.md**: Deployment instructions verified
- [ ] **CLAUDE.md**: Production readiness status updated to 100%
- [ ] **TASKS.md**: All 25 tasks marked complete
- [ ] **CRITICAL-FIXES.md**: All issues resolved
- [ ] **API Documentation**: GraphQL schema docs up to date
- [ ] **Runbooks**: Created for common issues
- [ ] **On-call Guide**: Created for incident response

### Compliance Checklist
- [ ] **GDPR Compliance**: User data handling reviewed
- [ ] **Data Retention**: Backup policy documented
- [ ] **Incident Response**: Plan documented and team trained
- [ ] **Disaster Recovery**: RTO/RPO defined and tested
- [ ] **SLA**: Service level agreements defined

---

## 🎉 PRODUCTION GO-LIVE

### Go/No-Go Decision

**GO CRITERIA** (ALL must be checked):
- [ ] All 9 phases completed ✅
- [ ] Zero critical bugs in staging ✅
- [ ] All team members signed off ✅
- [ ] Monitoring and alerts configured ✅
- [ ] Rollback plan documented ✅
- [ ] Support team on standby ✅

**Decision**: ☐ GO  ☐ NO-GO

**Signed**: ________________________
**Date**: ________________________
**Time**: ________________________

---

## 📞 EMERGENCY CONTACTS

**On-Call Rotation**:
- Primary: [Name] - [Phone] - [Email]
- Secondary: [Name] - [Phone] - [Email]
- Escalation: [Name] - [Phone] - [Email]

**Vendor Support**:
- Database: [Vendor] - [Support URL/Phone]
- Cloud Provider: [Vendor] - [Support URL/Phone]
- Monitoring: [Sentry/Datadog] - [Support URL]

---

## 🔄 ROLLBACK PLAN

**IF CRITICAL ISSUE OCCURS**:

1. **Stop deployment**:
   ```bash
   # Docker
   docker compose down

   # Kubernetes
   kubectl rollout undo deployment/auth-service -n production
   ```

2. **Restore previous version**:
   ```bash
   git checkout v0.9.0  # Previous stable version
   docker compose up -d
   ```

3. **Restore database** (if schema changed):
   ```bash
   psql -U postgres marketplace < backup_pre_v1.0.0.sql
   ```

4. **Notify stakeholders**:
   - Send incident report
   - Update status page
   - Schedule post-mortem

---

**Checklist Version**: 1.0
**Last Updated**: 2025-01-10
**Owner**: DevOps Team
**Review Frequency**: Before each major release
