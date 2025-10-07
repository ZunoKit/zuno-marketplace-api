package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Authentication metrics
	AuthAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_attempts_total",
			Help: "Total number of authentication attempts",
		},
		[]string{"method", "status"},
	)

	AuthDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "auth_duration_seconds",
			Help:    "Duration of authentication operations in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	// Nonce metrics
	NonceGenerated = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "nonce_generated_total",
			Help: "Total number of nonces generated",
		},
	)

	NonceVerified = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nonce_verified_total",
			Help: "Total number of nonces verified",
		},
		[]string{"status"},
	)

	NonceExpired = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "nonce_expired_total",
			Help: "Total number of expired nonces",
		},
	)

	// Session metrics
	SessionsCreated = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "sessions_created_total",
			Help: "Total number of sessions created",
		},
	)

	SessionsRefreshed = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "sessions_refreshed_total",
			Help: "Total number of sessions refreshed",
		},
	)

	SessionsRevoked = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "sessions_revoked_total",
			Help: "Total number of sessions revoked",
		},
	)

	ActiveSessions = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "sessions_active",
			Help: "Current number of active sessions",
		},
	)

	// Token metrics
	TokensIssued = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tokens_issued_total",
			Help: "Total number of tokens issued",
		},
		[]string{"type"}, // access or refresh
	)

	TokenRefreshReuse = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "token_refresh_reuse_detected_total",
			Help: "Total number of refresh token reuse attempts detected",
		},
	)

	// Rate limit metrics
	RateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limit_hits_total",
			Help: "Total number of rate limit hits",
		},
		[]string{"method"},
	)

	// Error metrics
	AuthErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_errors_total",
			Help: "Total number of authentication errors",
		},
		[]string{"method", "error_type"},
	)

	// Wallet metrics
	WalletsLinked = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "wallets_linked_total",
			Help: "Total number of wallets linked",
		},
	)

	// User metrics
	UsersCreated = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "users_created_total",
			Help: "Total number of users created",
		},
	)

	// Database metrics
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Duration of database queries in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	DBConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_connections",
			Help: "Number of database connections",
		},
		[]string{"state"}, // active, idle, total
	)

	// Cache metrics
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_type"},
	)

	CacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_type"},
	)

	// gRPC metrics
	GRPCRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "Duration of gRPC requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method"},
	)

	GRPCRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"service", "method", "status"},
	)
)

// RecordAuthAttempt records an authentication attempt
func RecordAuthAttempt(method, status string) {
	AuthAttempts.WithLabelValues(method, status).Inc()
}

// RecordAuthError records an authentication error
func RecordAuthError(method, errorType string) {
	AuthErrors.WithLabelValues(method, errorType).Inc()
}

// RecordNonceVerification records nonce verification result
func RecordNonceVerification(success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	NonceVerified.WithLabelValues(status).Inc()
}

// RecordTokenIssued records token issuance
func RecordTokenIssued(tokenType string) {
	TokensIssued.WithLabelValues(tokenType).Inc()
}

// RecordRateLimitHit records a rate limit hit
func RecordRateLimitHit(method string) {
	RateLimitHits.WithLabelValues(method).Inc()
}

// RecordDBQuery records database query duration
func RecordDBQuery(operation string, duration float64) {
	DBQueryDuration.WithLabelValues(operation).Observe(duration)
}

// RecordCacheAccess records cache hit or miss
func RecordCacheAccess(cacheType string, hit bool) {
	if hit {
		CacheHits.WithLabelValues(cacheType).Inc()
	} else {
		CacheMisses.WithLabelValues(cacheType).Inc()
	}
}

// RecordGRPCRequest records gRPC request metrics
func RecordGRPCRequest(service, method, status string, duration float64) {
	GRPCRequestsTotal.WithLabelValues(service, method, status).Inc()
	GRPCRequestDuration.WithLabelValues(service, method).Observe(duration)
}
