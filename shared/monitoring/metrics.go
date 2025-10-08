package monitoring

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// MetricType defines the type of metric
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
	MetricTypeSummary   MetricType = "summary"
)

// Metric represents a single metric
type Metric struct {
	Name        string
	Type        MetricType
	Value       float64
	Labels      map[string]string
	Timestamp   time.Time
	Description string
}

// MetricsCollector collects and stores metrics
type MetricsCollector struct {
	metrics    sync.Map
	counters   sync.Map
	gauges     sync.Map
	histograms sync.Map

	// System metrics
	startTime time.Time

	// Request metrics
	requestCount  uint64
	errorCount    uint64
	totalDuration uint64 // in milliseconds

	// Service-specific metrics
	serviceMetrics map[string]*ServiceMetrics
	mu             sync.RWMutex
}

// ServiceMetrics holds metrics for a specific service
type ServiceMetrics struct {
	RequestCount    uint64
	ErrorCount      uint64
	TotalDuration   uint64 // in milliseconds
	AverageDuration float64
	LastRequestTime time.Time
	HealthStatus    string
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		startTime:      time.Now(),
		serviceMetrics: make(map[string]*ServiceMetrics),
	}
}

// IncrementCounter increments a counter metric
func (mc *MetricsCollector) IncrementCounter(name string, labels map[string]string) {
	key := mc.getMetricKey(name, labels)

	if val, ok := mc.counters.Load(key); ok {
		counter := val.(*uint64)
		atomic.AddUint64(counter, 1)
	} else {
		counter := uint64(1)
		mc.counters.Store(key, &counter)
	}

	// Also increment global request count if it's a request metric
	if name == "http_requests_total" || name == "grpc_requests_total" {
		atomic.AddUint64(&mc.requestCount, 1)
	}

	if name == "errors_total" {
		atomic.AddUint64(&mc.errorCount, 1)
	}
}

// SetGauge sets a gauge metric value
func (mc *MetricsCollector) SetGauge(name string, value float64, labels map[string]string) {
	key := mc.getMetricKey(name, labels)
	mc.gauges.Store(key, value)
}

// RecordHistogram records a value in a histogram
func (mc *MetricsCollector) RecordHistogram(name string, value float64, labels map[string]string) {
	key := mc.getMetricKey(name, labels)

	if val, ok := mc.histograms.Load(key); ok {
		histogram := val.(*Histogram)
		histogram.Observe(value)
	} else {
		histogram := NewHistogram()
		histogram.Observe(value)
		mc.histograms.Store(key, histogram)
	}

	// Record duration if it's a duration metric
	if name == "http_request_duration_ms" || name == "grpc_request_duration_ms" {
		atomic.AddUint64(&mc.totalDuration, uint64(value))
	}
}

// RecordServiceRequest records a request for a specific service
func (mc *MetricsCollector) RecordServiceRequest(service string, duration time.Duration, err error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.serviceMetrics[service] == nil {
		mc.serviceMetrics[service] = &ServiceMetrics{
			HealthStatus: "healthy",
		}
	}

	metrics := mc.serviceMetrics[service]
	atomic.AddUint64(&metrics.RequestCount, 1)

	if err != nil {
		atomic.AddUint64(&metrics.ErrorCount, 1)
	}

	durationMs := uint64(duration.Milliseconds())
	atomic.AddUint64(&metrics.TotalDuration, durationMs)

	// Update average duration
	totalRequests := atomic.LoadUint64(&metrics.RequestCount)
	totalDuration := atomic.LoadUint64(&metrics.TotalDuration)
	metrics.AverageDuration = float64(totalDuration) / float64(totalRequests)

	metrics.LastRequestTime = time.Now()

	// Update health status based on error rate
	errorRate := float64(metrics.ErrorCount) / float64(metrics.RequestCount)
	if errorRate > 0.5 {
		metrics.HealthStatus = "unhealthy"
	} else if errorRate > 0.1 {
		metrics.HealthStatus = "degraded"
	} else {
		metrics.HealthStatus = "healthy"
	}
}

// GetSystemMetrics returns current system metrics
func (mc *MetricsCollector) GetSystemMetrics() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	uptime := time.Since(mc.startTime)
	requestCount := atomic.LoadUint64(&mc.requestCount)
	errorCount := atomic.LoadUint64(&mc.errorCount)
	totalDuration := atomic.LoadUint64(&mc.totalDuration)

	var avgDuration float64
	if requestCount > 0 {
		avgDuration = float64(totalDuration) / float64(requestCount)
	}

	return map[string]interface{}{
		"uptime_seconds":     uptime.Seconds(),
		"goroutines":         runtime.NumGoroutine(),
		"memory_alloc_bytes": memStats.Alloc,
		"memory_sys_bytes":   memStats.Sys,
		"memory_heap_bytes":  memStats.HeapAlloc,
		"gc_runs":            memStats.NumGC,
		"cpu_cores":          runtime.NumCPU(),
		"request_count":      requestCount,
		"error_count":        errorCount,
		"error_rate":         float64(errorCount) / float64(requestCount+1),
		"avg_duration_ms":    avgDuration,
	}
}

// GetServiceMetrics returns metrics for all services
func (mc *MetricsCollector) GetServiceMetrics() map[string]*ServiceMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[string]*ServiceMetrics)
	for name, metrics := range mc.serviceMetrics {
		result[name] = &ServiceMetrics{
			RequestCount:    atomic.LoadUint64(&metrics.RequestCount),
			ErrorCount:      atomic.LoadUint64(&metrics.ErrorCount),
			TotalDuration:   atomic.LoadUint64(&metrics.TotalDuration),
			AverageDuration: metrics.AverageDuration,
			LastRequestTime: metrics.LastRequestTime,
			HealthStatus:    metrics.HealthStatus,
		}
	}

	return result
}

// GetHealthStatus returns overall health status
func (mc *MetricsCollector) GetHealthStatus() map[string]interface{} {
	systemMetrics := mc.GetSystemMetrics()
	serviceMetrics := mc.GetServiceMetrics()

	// Determine overall health
	overallHealth := "healthy"
	unhealthyServices := []string{}

	for name, metrics := range serviceMetrics {
		if metrics.HealthStatus == "unhealthy" {
			unhealthyServices = append(unhealthyServices, name)
			overallHealth = "unhealthy"
		} else if metrics.HealthStatus == "degraded" && overallHealth == "healthy" {
			overallHealth = "degraded"
		}
	}

	// Check system metrics for issues
	if errorRate, ok := systemMetrics["error_rate"].(float64); ok && errorRate > 0.1 {
		if overallHealth == "healthy" {
			overallHealth = "degraded"
		}
	}

	return map[string]interface{}{
		"status":             overallHealth,
		"timestamp":          time.Now(),
		"uptime":             systemMetrics["uptime_seconds"],
		"unhealthy_services": unhealthyServices,
		"system_metrics":     systemMetrics,
		"service_metrics":    serviceMetrics,
	}
}

// getMetricKey generates a unique key for a metric
func (mc *MetricsCollector) getMetricKey(name string, labels map[string]string) string {
	key := name
	for k, v := range labels {
		key += fmt.Sprintf("_%s_%s", k, v)
	}
	return key
}

// Histogram implements a simple histogram for tracking distributions
type Histogram struct {
	mu     sync.RWMutex
	values []float64
	sum    float64
	count  uint64
}

// NewHistogram creates a new histogram
func NewHistogram() *Histogram {
	return &Histogram{
		values: make([]float64, 0),
	}
}

// Observe adds a value to the histogram
func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.values = append(h.values, value)
	h.sum += value
	h.count++

	// Keep only last 1000 values to prevent memory issues
	if len(h.values) > 1000 {
		h.values = h.values[len(h.values)-1000:]
	}
}

// GetStats returns histogram statistics
func (h *Histogram) GetStats() map[string]float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.count == 0 {
		return map[string]float64{
			"count": 0,
			"sum":   0,
			"avg":   0,
			"min":   0,
			"max":   0,
		}
	}

	min := h.values[0]
	max := h.values[0]

	for _, v := range h.values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	return map[string]float64{
		"count": float64(h.count),
		"sum":   h.sum,
		"avg":   h.sum / float64(h.count),
		"min":   min,
		"max":   max,
	}
}

// MetricsMiddleware is HTTP middleware for collecting metrics
func MetricsMiddleware(collector *MetricsCollector) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Record metrics
			duration := time.Since(start)

			labels := map[string]string{
				"method": r.Method,
				"path":   r.URL.Path,
				"status": fmt.Sprintf("%d", wrapped.statusCode),
			}

			collector.IncrementCounter("http_requests_total", labels)
			collector.RecordHistogram("http_request_duration_ms", float64(duration.Milliseconds()), labels)

			if wrapped.statusCode >= 400 {
				collector.IncrementCounter("errors_total", labels)
			}
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Global metrics collector instance
var globalCollector = NewMetricsCollector()

// GetGlobalCollector returns the global metrics collector
func GetGlobalCollector() *MetricsCollector {
	return globalCollector
}

// IncrementGlobalCounter increments a global counter
func IncrementGlobalCounter(name string, labels map[string]string) {
	globalCollector.IncrementCounter(name, labels)
}

// RecordGlobalDuration records a duration in the global collector
func RecordGlobalDuration(name string, duration time.Duration, labels map[string]string) {
	globalCollector.RecordHistogram(name, float64(duration.Milliseconds()), labels)
}

// RecordServiceRequestGlobal records a service request in the global collector
func RecordServiceRequestGlobal(service string, duration time.Duration, err error) {
	globalCollector.RecordServiceRequest(service, duration, err)
}
