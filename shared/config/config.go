package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// GlobalConfig holds all configuration values
type GlobalConfig struct {
	// Service Info
	ServiceName    string `json:"service_name"`
	ServiceVersion string `json:"service_version"`
	Environment    string `json:"environment"`

	// Security
	Security SecurityConfig `json:"security"`

	// Session
	Session SessionConfig `json:"session"`

	// Rate Limiting
	RateLimit RateLimitConfig `json:"rate_limit"`

	// Database
	Database DatabaseConfig `json:"database"`

	// Cache
	Cache CacheConfig `json:"cache"`

	// Messaging
	Messaging MessagingConfig `json:"messaging"`

	// Blockchain
	Blockchain BlockchainConfig `json:"blockchain"`

	// API
	API APIConfig `json:"api"`

	// Monitoring
	Monitoring MonitoringConfig `json:"monitoring"`
}

// SecurityConfig holds security settings
type SecurityConfig struct {
	JWTSecret         string        `json:"-"` // Never log secrets
	JWTExpiration     time.Duration `json:"jwt_expiration"`
	RefreshSecret     string        `json:"-"`
	RefreshExpiration time.Duration `json:"refresh_expiration"`
	BCryptCost        int           `json:"bcrypt_cost"`
	MaxLoginAttempts  int           `json:"max_login_attempts"`
	LockoutDuration   time.Duration `json:"lockout_duration"`
	RequireHTTPS      bool          `json:"require_https"`
	AllowedOrigins    []string      `json:"allowed_origins"`
	CSRFEnabled       bool          `json:"csrf_enabled"`
	SecureHeaders     bool          `json:"secure_headers"`
}

// SessionConfig holds session settings
type SessionConfig struct {
	TTL                   time.Duration `json:"ttl"`
	RefreshTTL            time.Duration `json:"refresh_ttl"`
	MaxConcurrentSessions int           `json:"max_concurrent_sessions"`
	RequireFingerprint    bool          `json:"require_fingerprint"`
	FingerprintStrictMode bool          `json:"fingerprint_strict_mode"`
	TokenRotation         bool          `json:"token_rotation"`
}

// RateLimitConfig holds rate limiting settings
type RateLimitConfig struct {
	Enabled           bool          `json:"enabled"`
	RequestsPerMinute int           `json:"requests_per_minute"`
	BurstSize         int           `json:"burst_size"`
	CleanupInterval   time.Duration `json:"cleanup_interval"`

	// Per-endpoint limits
	EndpointLimits map[string]EndpointLimit `json:"endpoint_limits"`
}

// EndpointLimit holds rate limit for specific endpoint
type EndpointLimit struct {
	RequestsPerMinute int `json:"requests_per_minute"`
	BurstSize         int `json:"burst_size"`
}

// DatabaseConfig holds database settings
type DatabaseConfig struct {
	PostgresHost     string        `json:"postgres_host"`
	PostgresPort     int           `json:"postgres_port"`
	PostgresUser     string        `json:"postgres_user"`
	PostgresPassword string        `json:"-"`
	PostgresDatabase string        `json:"postgres_database"`
	PostgresSSLMode  string        `json:"postgres_ssl_mode"`
	MaxConnections   int           `json:"max_connections"`
	MaxIdleConns     int           `json:"max_idle_conns"`
	ConnMaxLifetime  time.Duration `json:"conn_max_lifetime"`
	ConnMaxIdleTime  time.Duration `json:"conn_max_idle_time"`
}

// CacheConfig holds cache settings
type CacheConfig struct {
	RedisHost     string        `json:"redis_host"`
	RedisPort     int           `json:"redis_port"`
	RedisPassword string        `json:"-"`
	RedisDB       int           `json:"redis_db"`
	DefaultTTL    time.Duration `json:"default_ttl"`
	MaxRetries    int           `json:"max_retries"`
	PoolSize      int           `json:"pool_size"`
	MinIdleConns  int           `json:"min_idle_conns"`
	DialTimeout   time.Duration `json:"dial_timeout"`
	ReadTimeout   time.Duration `json:"read_timeout"`
	WriteTimeout  time.Duration `json:"write_timeout"`
}

// MessagingConfig holds messaging settings
type MessagingConfig struct {
	RabbitMQHost     string        `json:"rabbitmq_host"`
	RabbitMQPort     int           `json:"rabbitmq_port"`
	RabbitMQUser     string        `json:"rabbitmq_user"`
	RabbitMQPassword string        `json:"-"`
	RabbitMQVHost    string        `json:"rabbitmq_vhost"`
	RetryAttempts    int           `json:"retry_attempts"`
	RetryDelay       time.Duration `json:"retry_delay"`
	PrefetchCount    int           `json:"prefetch_count"`
	ConsumerTimeout  time.Duration `json:"consumer_timeout"`
}

// BlockchainConfig holds blockchain settings
type BlockchainConfig struct {
	IndexerBatchSize   int           `json:"indexer_batch_size"`
	IndexerInterval    time.Duration `json:"indexer_interval"`
	ConfirmationBlocks int           `json:"confirmation_blocks"`
	SafeBlockDepth     int           `json:"safe_block_depth"`
	MaxReorgDepth      int           `json:"max_reorg_depth"`
	GasPriceMultiplier float64       `json:"gas_price_multiplier"`
	MaxGasPrice        string        `json:"max_gas_price"`
	TransactionTimeout time.Duration `json:"transaction_timeout"`
	RPCTimeout         time.Duration `json:"rpc_timeout"`

	// Chain-specific configs
	Chains map[string]ChainConfig `json:"chains"`
}

// ChainConfig holds chain-specific settings
type ChainConfig struct {
	ChainID        string `json:"chain_id"`
	RPCURL         string `json:"rpc_url"`
	WSURL          string `json:"ws_url"`
	ExplorerURL    string `json:"explorer_url"`
	NativeCurrency string `json:"native_currency"`
	BlockTime      int    `json:"block_time"` // seconds
}

// APIConfig holds API settings
type APIConfig struct {
	MaxRequestSize      int64         `json:"max_request_size"`
	RequestTimeout      time.Duration `json:"request_timeout"`
	IdleTimeout         time.Duration `json:"idle_timeout"`
	ShutdownTimeout     time.Duration `json:"shutdown_timeout"`
	MaxQueryComplexity  int           `json:"max_query_complexity"`
	MaxQueryDepth       int           `json:"max_query_depth"`
	EnablePlayground    bool          `json:"enable_playground"`
	EnableIntrospection bool          `json:"enable_introspection"`
	EnableMetrics       bool          `json:"enable_metrics"`
	EnableProfiling     bool          `json:"enable_profiling"`
}

// MonitoringConfig holds monitoring settings
type MonitoringConfig struct {
	SentryDSN       string        `json:"-"`
	SentryEnv       string        `json:"sentry_env"`
	TracingSampling float64       `json:"tracing_sampling"`
	MetricsInterval time.Duration `json:"metrics_interval"`
	HealthCheckPath string        `json:"health_check_path"`
	MetricsPath     string        `json:"metrics_path"`
	LogLevel        string        `json:"log_level"`
	LogFormat       string        `json:"log_format"`
}

// LoadConfig loads configuration from environment and files
func LoadConfig() (*GlobalConfig, error) {
	// Load .env file if exists
	_ = godotenv.Load()

	config := &GlobalConfig{
		ServiceName:    getEnvString("SERVICE_NAME", "unknown"),
		ServiceVersion: getEnvString("SERVICE_VERSION", "unknown"),
		Environment:    getEnvString("ENVIRONMENT", "development"),

		Security: SecurityConfig{
			JWTSecret:         getEnvString("JWT_SECRET", ""),
			JWTExpiration:     getEnvDuration("JWT_EXPIRATION", 1*time.Hour),
			RefreshSecret:     getEnvString("REFRESH_SECRET", ""),
			RefreshExpiration: getEnvDuration("REFRESH_EXPIRATION", 24*time.Hour),
			BCryptCost:        getEnvInt("BCRYPT_COST", 10),
			MaxLoginAttempts:  getEnvInt("MAX_LOGIN_ATTEMPTS", 5),
			LockoutDuration:   getEnvDuration("LOCKOUT_DURATION", 15*time.Minute),
			RequireHTTPS:      getEnvBool("REQUIRE_HTTPS", false),
			AllowedOrigins:    getEnvStringSlice("ALLOWED_ORIGINS", []string{"*"}),
			CSRFEnabled:       getEnvBool("CSRF_ENABLED", true),
			SecureHeaders:     getEnvBool("SECURE_HEADERS", true),
		},

		Session: SessionConfig{
			TTL:                   getEnvDuration("SESSION_TTL", 24*time.Hour),
			RefreshTTL:            getEnvDuration("SESSION_REFRESH_TTL", 7*24*time.Hour),
			MaxConcurrentSessions: getEnvInt("MAX_CONCURRENT_SESSIONS", 5),
			RequireFingerprint:    getEnvBool("REQUIRE_FINGERPRINT", true),
			FingerprintStrictMode: getEnvBool("FINGERPRINT_STRICT_MODE", false),
			TokenRotation:         getEnvBool("TOKEN_ROTATION", true),
		},

		RateLimit: RateLimitConfig{
			Enabled:           getEnvBool("RATE_LIMIT_ENABLED", true),
			RequestsPerMinute: getEnvInt("RATE_LIMIT_RPM", 60),
			BurstSize:         getEnvInt("RATE_LIMIT_BURST", 10),
			CleanupInterval:   getEnvDuration("RATE_LIMIT_CLEANUP", 1*time.Minute),
			EndpointLimits:    loadEndpointLimits(),
		},

		Database: DatabaseConfig{
			PostgresHost:     getEnvString("POSTGRES_HOST", "localhost"),
			PostgresPort:     getEnvInt("POSTGRES_PORT", 5432),
			PostgresUser:     getEnvString("POSTGRES_USER", "postgres"),
			PostgresPassword: getEnvString("POSTGRES_PASSWORD", ""),
			PostgresDatabase: getEnvString("POSTGRES_DATABASE", "nft_marketplace"),
			PostgresSSLMode:  getEnvString("POSTGRES_SSL_MODE", "disable"),
			MaxConnections:   getEnvInt("DB_MAX_CONNECTIONS", 25),
			MaxIdleConns:     getEnvInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime:  getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
			ConnMaxIdleTime:  getEnvDuration("DB_CONN_MAX_IDLE_TIME", 1*time.Minute),
		},

		Cache: CacheConfig{
			RedisHost:     getEnvString("REDIS_HOST", "localhost"),
			RedisPort:     getEnvInt("REDIS_PORT", 6379),
			RedisPassword: getEnvString("REDIS_PASSWORD", ""),
			RedisDB:       getEnvInt("REDIS_DB", 0),
			DefaultTTL:    getEnvDuration("CACHE_DEFAULT_TTL", 5*time.Minute),
			MaxRetries:    getEnvInt("REDIS_MAX_RETRIES", 3),
			PoolSize:      getEnvInt("REDIS_POOL_SIZE", 10),
			MinIdleConns:  getEnvInt("REDIS_MIN_IDLE_CONNS", 2),
			DialTimeout:   getEnvDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:   getEnvDuration("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout:  getEnvDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
		},

		Messaging: MessagingConfig{
			RabbitMQHost:     getEnvString("RABBITMQ_HOST", "localhost"),
			RabbitMQPort:     getEnvInt("RABBITMQ_PORT", 5672),
			RabbitMQUser:     getEnvString("RABBITMQ_USER", "guest"),
			RabbitMQPassword: getEnvString("RABBITMQ_PASSWORD", "guest"),
			RabbitMQVHost:    getEnvString("RABBITMQ_VHOST", "/"),
			RetryAttempts:    getEnvInt("MQ_RETRY_ATTEMPTS", 3),
			RetryDelay:       getEnvDuration("MQ_RETRY_DELAY", 1*time.Second),
			PrefetchCount:    getEnvInt("MQ_PREFETCH_COUNT", 10),
			ConsumerTimeout:  getEnvDuration("MQ_CONSUMER_TIMEOUT", 30*time.Second),
		},

		Blockchain: BlockchainConfig{
			IndexerBatchSize:   getEnvInt("INDEXER_BATCH_SIZE", 100),
			IndexerInterval:    getEnvDuration("INDEXER_INTERVAL", 10*time.Second),
			ConfirmationBlocks: getEnvInt("CONFIRMATION_BLOCKS", 12),
			SafeBlockDepth:     getEnvInt("SAFE_BLOCK_DEPTH", 64),
			MaxReorgDepth:      getEnvInt("MAX_REORG_DEPTH", 128),
			GasPriceMultiplier: getEnvFloat("GAS_PRICE_MULTIPLIER", 1.2),
			MaxGasPrice:        getEnvString("MAX_GAS_PRICE", "500000000000"), // 500 Gwei
			TransactionTimeout: getEnvDuration("TRANSACTION_TIMEOUT", 5*time.Minute),
			RPCTimeout:         getEnvDuration("RPC_TIMEOUT", 30*time.Second),
			Chains:             loadChainConfigs(),
		},

		API: APIConfig{
			MaxRequestSize:      getEnvInt64("MAX_REQUEST_SIZE", 10*1024*1024), // 10MB
			RequestTimeout:      getEnvDuration("REQUEST_TIMEOUT", 30*time.Second),
			IdleTimeout:         getEnvDuration("IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout:     getEnvDuration("SHUTDOWN_TIMEOUT", 30*time.Second),
			MaxQueryComplexity:  getEnvInt("MAX_QUERY_COMPLEXITY", 1000),
			MaxQueryDepth:       getEnvInt("MAX_QUERY_DEPTH", 10),
			EnablePlayground:    getEnvBool("ENABLE_PLAYGROUND", false),
			EnableIntrospection: getEnvBool("ENABLE_INTROSPECTION", false),
			EnableMetrics:       getEnvBool("ENABLE_METRICS", true),
			EnableProfiling:     getEnvBool("ENABLE_PROFILING", false),
		},

		Monitoring: MonitoringConfig{
			SentryDSN:       getEnvString("SENTRY_DSN", ""),
			SentryEnv:       getEnvString("SENTRY_ENVIRONMENT", "development"),
			TracingSampling: getEnvFloat("TRACING_SAMPLING", 0.1),
			MetricsInterval: getEnvDuration("METRICS_INTERVAL", 10*time.Second),
			HealthCheckPath: getEnvString("HEALTH_CHECK_PATH", "/health"),
			MetricsPath:     getEnvString("METRICS_PATH", "/metrics"),
			LogLevel:        getEnvString("LOG_LEVEL", "info"),
			LogFormat:       getEnvString("LOG_FORMAT", "json"),
		},
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// Validate validates the configuration
func (c *GlobalConfig) Validate() error {
	if c.Security.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if c.Security.RefreshSecret == "" {
		return fmt.Errorf("REFRESH_SECRET is required")
	}
	if c.Database.PostgresPassword == "" && c.Environment == "production" {
		return fmt.Errorf("POSTGRES_PASSWORD is required in production")
	}
	return nil
}

// Helper functions

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

func loadEndpointLimits() map[string]EndpointLimit {
	// Load from environment or config file
	// Format: ENDPOINT_LIMITS=signin:10:2,signup:5:1
	limits := make(map[string]EndpointLimit)

	if value := os.Getenv("ENDPOINT_LIMITS"); value != "" {
		pairs := strings.Split(value, ",")
		for _, pair := range pairs {
			parts := strings.Split(pair, ":")
			if len(parts) == 3 {
				endpoint := parts[0]
				rpm, _ := strconv.Atoi(parts[1])
				burst, _ := strconv.Atoi(parts[2])
				limits[endpoint] = EndpointLimit{
					RequestsPerMinute: rpm,
					BurstSize:         burst,
				}
			}
		}
	}

	// Set defaults if not configured
	if _, ok := limits["signin"]; !ok {
		limits["signin"] = EndpointLimit{RequestsPerMinute: 10, BurstSize: 2}
	}
	if _, ok := limits["signup"]; !ok {
		limits["signup"] = EndpointLimit{RequestsPerMinute: 5, BurstSize: 1}
	}

	return limits
}

func loadChainConfigs() map[string]ChainConfig {
	chains := make(map[string]ChainConfig)

	// Load from environment or config file
	// This is a simplified version, in production you'd load from a config file

	// Ethereum Mainnet
	if rpc := os.Getenv("ETH_MAINNET_RPC"); rpc != "" {
		chains["eip155:1"] = ChainConfig{
			ChainID:        "eip155:1",
			RPCURL:         rpc,
			WSURL:          os.Getenv("ETH_MAINNET_WS"),
			ExplorerURL:    "https://etherscan.io",
			NativeCurrency: "ETH",
			BlockTime:      12,
		}
	}

	// Polygon
	if rpc := os.Getenv("POLYGON_RPC"); rpc != "" {
		chains["eip155:137"] = ChainConfig{
			ChainID:        "eip155:137",
			RPCURL:         rpc,
			WSURL:          os.Getenv("POLYGON_WS"),
			ExplorerURL:    "https://polygonscan.com",
			NativeCurrency: "MATIC",
			BlockTime:      2,
		}
	}

	// BSC
	if rpc := os.Getenv("BSC_RPC"); rpc != "" {
		chains["eip155:56"] = ChainConfig{
			ChainID:        "eip155:56",
			RPCURL:         rpc,
			WSURL:          os.Getenv("BSC_WS"),
			ExplorerURL:    "https://bscscan.com",
			NativeCurrency: "BNB",
			BlockTime:      3,
		}
	}

	return chains
}

// ToJSON converts config to JSON
func (c *GlobalConfig) ToJSON() (string, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
