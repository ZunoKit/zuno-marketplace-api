package testconfig

import (
	"os"
	"time"
)

// TestConfig holds test configuration
type TestConfig struct {
	DatabaseURL      string
	RedisURL         string
	MongoURL         string
	RabbitMQURL      string
	TestTimeout      time.Duration
	ParallelTests    bool
	VerboseLogging   bool
	CleanupAfterTest bool
}

// GetTestConfig returns test configuration from environment
func GetTestConfig() *TestConfig {
	return &TestConfig{
		DatabaseURL:      getEnv("TEST_DATABASE_URL", "postgres://test:test@localhost:5432/testdb?sslmode=disable"),
		RedisURL:         getEnv("TEST_REDIS_URL", "redis://localhost:6379"),
		MongoURL:         getEnv("TEST_MONGO_URL", "mongodb://localhost:27017"),
		RabbitMQURL:      getEnv("TEST_RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		TestTimeout:      getDuration("TEST_TIMEOUT", 30*time.Second),
		ParallelTests:    getBool("TEST_PARALLEL", true),
		VerboseLogging:   getBool("TEST_VERBOSE", false),
		CleanupAfterTest: getBool("TEST_CLEANUP", true),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

func getBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}
