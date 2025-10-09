package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	// HTTP metrics
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	HTTPResponseSize    *prometheus.HistogramVec

	// gRPC metrics
	GRPCRequestsTotal   *prometheus.CounterVec
	GRPCRequestDuration *prometheus.HistogramVec
	GRPCStreamMsgsTotal *prometheus.CounterVec

	// Database metrics
	DBQueriesTotal      *prometheus.CounterVec
	DBQueryDuration     *prometheus.HistogramVec
	DBConnectionsActive prometheus.Gauge
	DBConnectionsIdle   prometheus.Gauge

	// Cache metrics
	CacheHits    *prometheus.CounterVec
	CacheMisses  *prometheus.CounterVec
	CacheLatency *prometheus.HistogramVec

	// Business metrics
	NFTsMinted          prometheus.Counter
	CollectionsCreated  prometheus.Counter
	TransactionsTracked *prometheus.CounterVec
	UserRegistrations   prometheus.Counter
	ActiveSessions      prometheus.Gauge

	// Error metrics
	ErrorsTotal     *prometheus.CounterVec
	PanicsRecovered prometheus.Counter

	// Performance metrics
	GoroutinesActive prometheus.Gauge
	MemoryUsage      prometheus.Gauge
}

// NewMetrics creates and registers all metrics
func NewMetrics(namespace, service string) *Metrics {
	return &Metrics{
		// HTTP metrics
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request latencies in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),
		HTTPResponseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "http_response_size_bytes",
				Help:      "HTTP response sizes in bytes",
				Buckets:   []float64{100, 1000, 10000, 100000, 1000000},
			},
			[]string{"method", "endpoint"},
		),

		// gRPC metrics
		GRPCRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "grpc_requests_total",
				Help:      "Total number of gRPC requests",
			},
			[]string{"method", "status"},
		),
		GRPCRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "grpc_request_duration_seconds",
				Help:      "gRPC request latencies in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method"},
		),
		GRPCStreamMsgsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "grpc_stream_messages_total",
				Help:      "Total number of gRPC stream messages",
			},
			[]string{"method", "direction"},
		),

		// Database metrics
		DBQueriesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "db_queries_total",
				Help:      "Total number of database queries",
			},
			[]string{"query_type", "table", "status"},
		),
		DBQueryDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "db_query_duration_seconds",
				Help:      "Database query latencies in seconds",
				Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
			},
			[]string{"query_type", "table"},
		),
		DBConnectionsActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "db_connections_active",
				Help:      "Number of active database connections",
			},
		),
		DBConnectionsIdle: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "db_connections_idle",
				Help:      "Number of idle database connections",
			},
		),

		// Cache metrics
		CacheHits: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "cache_hits_total",
				Help:      "Total number of cache hits",
			},
			[]string{"cache_name"},
		),
		CacheMisses: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "cache_misses_total",
				Help:      "Total number of cache misses",
			},
			[]string{"cache_name"},
		),
		CacheLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "cache_operation_duration_seconds",
				Help:      "Cache operation latencies in seconds",
				Buckets:   []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1},
			},
			[]string{"cache_name", "operation"},
		),

		// Business metrics
		NFTsMinted: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "nfts_minted_total",
				Help:      "Total number of NFTs minted",
			},
		),
		CollectionsCreated: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "collections_created_total",
				Help:      "Total number of collections created",
			},
		),
		TransactionsTracked: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "transactions_tracked_total",
				Help:      "Total number of transactions tracked",
			},
			[]string{"chain_id", "status"},
		),
		UserRegistrations: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "user_registrations_total",
				Help:      "Total number of user registrations",
			},
		),
		ActiveSessions: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "active_sessions",
				Help:      "Number of active user sessions",
			},
		),

		// Error metrics
		ErrorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "errors_total",
				Help:      "Total number of errors",
			},
			[]string{"type", "code"},
		),
		PanicsRecovered: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "panics_recovered_total",
				Help:      "Total number of panics recovered",
			},
		),

		// Performance metrics
		GoroutinesActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "goroutines_active",
				Help:      "Number of active goroutines",
			},
		),
		MemoryUsage: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: service,
				Name:      "memory_usage_bytes",
				Help:      "Current memory usage in bytes",
			},
		),
	}
}

// HTTPMiddleware is a middleware that records HTTP metrics
func (m *Metrics) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Process request
		next.ServeHTTP(wrapped, r)

		// Record metrics
		duration := time.Since(start).Seconds()
		endpoint := r.URL.Path
		method := r.Method
		status := wrapped.statusCode

		m.HTTPRequestsTotal.WithLabelValues(method, endpoint, fmt.Sprintf("%d", status)).Inc()
		m.HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration)
		m.HTTPResponseSize.WithLabelValues(method, endpoint).Observe(float64(wrapped.bytesWritten))
	})
}

// GRPCUnaryInterceptor records metrics for unary gRPC calls
func (m *Metrics) GRPCUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		// Handle request
		resp, err := handler(ctx, req)

		// Record metrics
		duration := time.Since(start).Seconds()
		code := status.Code(err).String()

		m.GRPCRequestsTotal.WithLabelValues(info.FullMethod, code).Inc()
		m.GRPCRequestDuration.WithLabelValues(info.FullMethod).Observe(duration)

		if err != nil {
			m.ErrorsTotal.WithLabelValues("grpc", code).Inc()
		}

		return resp, err
	}
}

// GRPCStreamInterceptor records metrics for streaming gRPC calls
func (m *Metrics) GRPCStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()

		// Wrap stream to count messages
		wrapped := &metricsServerStream{
			ServerStream: ss,
			metrics:      m,
			method:       info.FullMethod,
		}

		// Handle stream
		err := handler(srv, wrapped)

		// Record metrics
		duration := time.Since(start).Seconds()
		code := status.Code(err).String()

		m.GRPCRequestsTotal.WithLabelValues(info.FullMethod, code).Inc()
		m.GRPCRequestDuration.WithLabelValues(info.FullMethod).Observe(duration)

		if err != nil {
			m.ErrorsTotal.WithLabelValues("grpc_stream", code).Inc()
		}

		return err
	}
}

// RecordDBQuery records database query metrics
func (m *Metrics) RecordDBQuery(queryType, table string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}

	m.DBQueriesTotal.WithLabelValues(queryType, table, status).Inc()
	m.DBQueryDuration.WithLabelValues(queryType, table).Observe(duration.Seconds())

	if err != nil {
		m.ErrorsTotal.WithLabelValues("database", queryType).Inc()
	}
}

// RecordCacheOperation records cache operation metrics
func (m *Metrics) RecordCacheOperation(cacheName, operation string, hit bool, duration time.Duration) {
	if hit {
		m.CacheHits.WithLabelValues(cacheName).Inc()
	} else {
		m.CacheMisses.WithLabelValues(cacheName).Inc()
	}

	m.CacheLatency.WithLabelValues(cacheName, operation).Observe(duration.Seconds())
}

// Handler returns the Prometheus metrics handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// responseWriter wraps http.ResponseWriter to capture metrics
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(bytes []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(bytes)
	rw.bytesWritten += n
	return n, err
}

// metricsServerStream wraps grpc.ServerStream to count messages
type metricsServerStream struct {
	grpc.ServerStream
	metrics *Metrics
	method  string
}

func (s *metricsServerStream) SendMsg(m interface{}) error {
	err := s.ServerStream.SendMsg(m)
	s.metrics.GRPCStreamMsgsTotal.WithLabelValues(s.method, "sent").Inc()
	return err
}

func (s *metricsServerStream) RecvMsg(m interface{}) error {
	err := s.ServerStream.RecvMsg(m)
	s.metrics.GRPCStreamMsgsTotal.WithLabelValues(s.method, "received").Inc()
	return err
}
