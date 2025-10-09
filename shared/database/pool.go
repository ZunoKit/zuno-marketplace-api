package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// PoolConfig holds connection pool configuration
type PoolConfig struct {
	// Connection settings
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string

	// Pool settings
	MaxOpenConns    int           // Maximum number of open connections
	MaxIdleConns    int           // Maximum number of idle connections
	ConnMaxLifetime time.Duration // Maximum lifetime of a connection
	ConnMaxIdleTime time.Duration // Maximum idle time of a connection

	// Performance settings
	StatementCacheMode string        // prepared statement cache mode
	ConnectTimeout     time.Duration // Connection timeout

	// Service-specific settings
	ServiceName string // For connection labeling
}

// DefaultPoolConfig returns optimized default pool configuration
func DefaultPoolConfig(serviceName string) *PoolConfig {
	return &PoolConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "",
		Database: "nft_marketplace",
		SSLMode:  "disable",

		// Optimized pool settings based on service type
		MaxOpenConns:       25,
		MaxIdleConns:       5,
		ConnMaxLifetime:    5 * time.Minute,
		ConnMaxIdleTime:    1 * time.Minute,
		StatementCacheMode: "describe",
		ConnectTimeout:     5 * time.Second,
		ServiceName:        serviceName,
	}
}

// ServicePoolConfigs returns optimized configs per service
func ServicePoolConfigs() map[string]*PoolConfig {
	return map[string]*PoolConfig{
		"auth-service": {
			MaxOpenConns:    30, // Higher for auth service
			MaxIdleConns:    10,
			ConnMaxLifetime: 10 * time.Minute,
			ConnMaxIdleTime: 2 * time.Minute,
		},
		"catalog-service": {
			MaxOpenConns:    40, // Highest for catalog queries
			MaxIdleConns:    15,
			ConnMaxLifetime: 15 * time.Minute,
			ConnMaxIdleTime: 3 * time.Minute,
		},
		"indexer-service": {
			MaxOpenConns:    20, // Moderate for batch processing
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 1 * time.Minute,
		},
		"user-service": {
			MaxOpenConns:    25,
			MaxIdleConns:    8,
			ConnMaxLifetime: 10 * time.Minute,
			ConnMaxIdleTime: 2 * time.Minute,
		},
		"wallet-service": {
			MaxOpenConns:    20,
			MaxIdleConns:    5,
			ConnMaxLifetime: 10 * time.Minute,
			ConnMaxIdleTime: 2 * time.Minute,
		},
		"orchestrator-service": {
			MaxOpenConns:    15, // Lower for orchestrator
			MaxIdleConns:    3,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 1 * time.Minute,
		},
	}
}

// NewConnectionPool creates an optimized connection pool
func NewConnectionPool(config *PoolConfig) (*sql.DB, error) {
	// Build connection string
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d application_name=%s statement_cache_mode=%s",
		config.Host,
		config.Port,
		config.User,
		config.Password,
		config.Database,
		config.SSLMode,
		int(config.ConnectTimeout.Seconds()),
		config.ServiceName,
		config.StatementCacheMode,
	)

	// Open database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), config.ConnectTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// PoolMonitor monitors connection pool health
type PoolMonitor struct {
	db       *sql.DB
	service  string
	interval time.Duration
}

// NewPoolMonitor creates a new pool monitor
func NewPoolMonitor(db *sql.DB, service string) *PoolMonitor {
	return &PoolMonitor{
		db:       db,
		service:  service,
		interval: 30 * time.Second,
	}
}

// Start begins monitoring the pool
func (pm *PoolMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(pm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pm.logStats()
		case <-ctx.Done():
			return
		}
	}
}

// logStats logs pool statistics
func (pm *PoolMonitor) logStats() {
	stats := pm.db.Stats()

	// Log if pool is under pressure
	utilizationRate := float64(stats.InUse) / float64(stats.MaxOpenConnections)
	if utilizationRate > 0.8 {
		fmt.Printf("[POOL WARNING] Service=%s High utilization: %.2f%% (InUse=%d, Max=%d)\n",
			pm.service, utilizationRate*100, stats.InUse, stats.MaxOpenConnections)
	}

	// Log if wait count is high
	if stats.WaitCount > 0 {
		fmt.Printf("[POOL WARNING] Service=%s Connection waits: Count=%d, Duration=%v\n",
			pm.service, stats.WaitCount, stats.WaitDuration)
	}

	// Log if many idle connections
	if stats.Idle > 10 { // Using a reasonable threshold instead of MaxIdleConnections
		fmt.Printf("[POOL INFO] Service=%s Many idle connections: %d\n",
			pm.service, stats.Idle)
	}
}

// GetStats returns current pool statistics
func (pm *PoolMonitor) GetStats() PoolStats {
	stats := pm.db.Stats()
	return PoolStats{
		OpenConnections:   stats.OpenConnections,
		InUse:             stats.InUse,
		Idle:              stats.Idle,
		WaitCount:         stats.WaitCount,
		WaitDuration:      stats.WaitDuration,
		MaxIdleClosed:     stats.MaxIdleClosed,
		MaxLifetimeClosed: stats.MaxLifetimeClosed,
	}
}

// PoolStats represents pool statistics
type PoolStats struct {
	OpenConnections   int
	InUse             int
	Idle              int
	WaitCount         int64
	WaitDuration      time.Duration
	MaxIdleClosed     int64
	MaxLifetimeClosed int64
}

// AutoTuner automatically adjusts pool settings based on load
type AutoTuner struct {
	db            *sql.DB
	config        *PoolConfig
	checkInterval time.Duration
	adjustments   int
}

// NewAutoTuner creates a new auto-tuner
func NewAutoTuner(db *sql.DB, config *PoolConfig) *AutoTuner {
	return &AutoTuner{
		db:            db,
		config:        config,
		checkInterval: 1 * time.Minute,
		adjustments:   0,
	}
}

// Start begins auto-tuning
func (at *AutoTuner) Start(ctx context.Context) {
	ticker := time.NewTicker(at.checkInterval)
	defer ticker.Stop()

	var history []PoolStats

	for {
		select {
		case <-ticker.C:
			stats := at.db.Stats()
			poolStats := PoolStats{
				OpenConnections: stats.OpenConnections,
				InUse:           stats.InUse,
				Idle:            stats.Idle,
				WaitCount:       stats.WaitCount,
				WaitDuration:    stats.WaitDuration,
			}

			history = append(history, poolStats)
			if len(history) > 10 {
				history = history[1:]
			}

			// Auto-tune based on history
			if len(history) >= 5 {
				at.tune(history)
			}

		case <-ctx.Done():
			return
		}
	}
}

// tune adjusts pool settings based on statistics
func (at *AutoTuner) tune(history []PoolStats) {
	// Calculate average utilization
	var totalInUse, totalWaitCount int
	for _, stat := range history {
		totalInUse += stat.InUse
		totalWaitCount += int(stat.WaitCount)
	}

	avgInUse := totalInUse / len(history)
	avgWaitCount := totalWaitCount / len(history)

	currentMax := at.config.MaxOpenConns

	// Increase pool size if consistently high utilization
	if avgInUse > int(float64(currentMax)*0.8) || avgWaitCount > 0 {
		newMax := min(currentMax+5, 100) // Cap at 100
		if newMax != currentMax {
			at.db.SetMaxOpenConns(newMax)
			at.config.MaxOpenConns = newMax
			at.adjustments++
			fmt.Printf("[POOL TUNING] Increased max connections: %d -> %d\n", currentMax, newMax)
		}
	}

	// Decrease pool size if consistently low utilization
	if avgInUse < int(float64(currentMax)*0.3) && currentMax > 10 {
		newMax := max(currentMax-5, 10) // Min at 10
		if newMax != currentMax {
			at.db.SetMaxOpenConns(newMax)
			at.config.MaxOpenConns = newMax
			at.adjustments++
			fmt.Printf("[POOL TUNING] Decreased max connections: %d -> %d\n", currentMax, newMax)
		}
	}

	// Adjust idle connections
	currentIdle := at.config.MaxIdleConns
	targetIdle := max(avgInUse/3, 2) // Keep 1/3 of average as idle
	if targetIdle != currentIdle {
		at.db.SetMaxIdleConns(targetIdle)
		at.config.MaxIdleConns = targetIdle
		fmt.Printf("[POOL TUNING] Adjusted idle connections: %d -> %d\n", currentIdle, targetIdle)
	}
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// PgBouncer configuration generator
func GeneratePgBouncerConfig(services map[string]*PoolConfig) string {
	config := "[databases]\n"

	for service, pool := range services {
		config += fmt.Sprintf("%s = host=%s port=%d dbname=%s pool_size=%d reserve_pool_size=%d\n",
			service,
			pool.Host,
			pool.Port,
			pool.Database,
			pool.MaxOpenConns,
			pool.MaxIdleConns,
		)
	}

	config += "\n[pgbouncer]\n"
	config += "pool_mode = transaction\n"
	config += "max_client_conn = 1000\n"
	config += "default_pool_size = 25\n"
	config += "reserve_pool_size = 5\n"
	config += "reserve_pool_timeout = 3\n"
	config += "server_lifetime = 3600\n"
	config += "server_idle_timeout = 600\n"
	config += "stats_period = 60\n"

	return config
}
