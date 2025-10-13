package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/quangdang46/NFT-Marketplace/shared/redis"
	"github.com/quangdang46/NFT-Marketplace/shared/testutil"
	"github.com/quangdang46/NFT-Marketplace/test/qa/auth/security"
)

// TC-DATA-001-01: Schema Separation Validation
// Test: Verify test schema completely isolated from production
// Priority: P1 (Critical Data Integrity)
// TDD Phase: RED → Write this test first
func TestSchemaSeparation(t *testing.T) {
	// GIVEN: Test database with auth_test schema
	ctx := context.Background()
	testDB, err := testutil.SetupTestPostgres(ctx)
	if err != nil {
		t.Fatalf("failed to setup test postgres: %v", err)
	}
	defer testDB.Cleanup(ctx)

	// WHEN: Initializing test schema
	// Note: Skipping migration test as it requires embedded FS and service-specific migrations
	// For now, verify schema existence directly
	// migrator, err := migration.NewMigrator(&migration.Config{
	// 	DatabaseURL: testDB.ConnString,
	// 	Service:     "auth",
	// 	SchemaName:  "auth_test",
	// })
	// assert.NoError(t, err)
	// err = migrator.Up(ctx, "auth_test")
	// assert.NoError(t, err)

	// THEN: Verify test schema exists
	var schemaExists bool
	err = testDB.DB.QueryRow(`
        SELECT EXISTS(
            SELECT 1 FROM information_schema.schemata 
            WHERE schema_name = 'auth_test'
        )
    `).Scan(&schemaExists)
	assert.NoError(t, err)
	assert.True(t, schemaExists)

	// AND: Production schema should NOT exist in test DB
	err = testDB.DB.QueryRow(`
        SELECT EXISTS(
            SELECT 1 FROM information_schema.schemata 
            WHERE schema_name = 'auth'
        )
    `).Scan(&schemaExists)
	assert.NoError(t, err)
	assert.False(t, schemaExists, "Production schema found in test database")
}

// TC-DATA-001-02: Connection String Validation
// Test: System rejects production database connection strings
// Priority: P1 (Critical Data Integrity)
// TDD Phase: RED → Write this test first
func TestConnectionStringValidation(t *testing.T) {
	validator := security.NewDatabaseConnectionValidator()

	testCases := []struct {
		name        string
		connString  string
		shouldAllow bool
	}{
		{
			name:        "test_database_allowed",
			connString:  "host=localhost port=5432 dbname=testdb user=test",
			shouldAllow: true,
		},
		{
			name:        "production_database_rejected",
			connString:  "host=prod-db.example.com port=5432 dbname=marketplace user=prod",
			shouldAllow: false,
		},
		{
			name:        "production_schema_rejected",
			connString:  "host=localhost port=5432 dbname=testdb user=test search_path=auth",
			shouldAllow: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowed := validator.ValidateConnectionString(tc.connString)
			assert.Equal(t, tc.shouldAllow, allowed)
		})
	}
}

// TC-DATA-001-03: Data Cleanup Validation
// Test: Verify all test data removed after test execution
// Priority: P1 (Critical Data Integrity)
// TDD Phase: RED → Write this test first
func TestDataCleanupAfterTests(t *testing.T) {
	// GIVEN: Test database with data
	ctx := context.Background()
	testDB, err := testutil.SetupTestPostgres(ctx)
	if err != nil {
		t.Fatalf("Failed to setup test postgres: %v", err)
	}

	// Create test schema and table
	_, err = testDB.DB.Exec(`
		CREATE SCHEMA IF NOT EXISTS auth_test;
		CREATE TABLE IF NOT EXISTS auth_test.auth_nonces (
			account_id TEXT,
			chain_id TEXT,
			nonce TEXT,
			domain TEXT
		);
	`)
	assert.NoError(t, err)

	// Insert test data
	_, err = testDB.DB.Exec(`
        INSERT INTO auth_test.auth_nonces (account_id, chain_id, nonce, domain)
        VALUES ('test-account', 'eip155:11155111', 'test-nonce', 'localhost')
    `)
	assert.NoError(t, err)

	// WHEN: Running cleanup
	err = testDB.Cleanup(ctx)
	assert.NoError(t, err)

	// THEN: Container should be terminated
	assert.Nil(t, testDB.Container, "Container not properly cleaned up")

	// AND: DB should be closed (set to nil)
	assert.Nil(t, testDB.DB, "Database connection still active after cleanup")
}

// TC-DATA-001-04: Redis Namespace Isolation
// Test: Test data uses separate Redis namespace (test:*)
// Priority: P1 (Critical Data Integrity)
// TDD Phase: RED → Write this test first
func TestRedisNamespaceIsolation(t *testing.T) {
	// GIVEN: Test Redis with namespace
	ctx := context.Background()
	redisContainer, redisURL, err := testutil.SetupTestRedis(ctx)
	if err != nil {
		t.Fatalf("failed to setup test redis: %v", err)
	}
	defer redisContainer.Terminate(ctx)

	// Extract host and port from redisURL (format: redis://host:port)
	host, err := redisContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}
	
	port, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("failed to get mapped port: %v", err)
	}

	// Parse port string to int
	redisPort := 0
	_, err = fmt.Sscanf(port.Port(), "%d", &redisPort)
	if err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}

	redisClient, err := redis.NewRedis(redis.RedisConfig{
		RedisHost:     host,
		RedisPort:     redisPort,
		RedisPassword: "",
		RedisDB:       0,
	})
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}
	defer redisClient.Close()

	// Validate Redis connection
	if err := redisClient.Ping(ctx); err != nil {
		t.Fatalf("redis connection failed: %v", err)
	}

	// WHEN: Storing test data with namespace prefix
	// Note: Redis namespace isolation should be handled at the application level
	// by prefixing all keys with "test:"
	namespacedKey := "test:nonce:account123"
	err = redisClient.Set(ctx, namespacedKey, "test-nonce-value", 5*time.Minute)
	assert.NoError(t, err)

	_ = redisURL // Keep for reference

	// THEN: Key should be retrievable with namespace
	value, err := redisClient.Get(ctx, namespacedKey)
	assert.NoError(t, err)
	assert.Equal(t, "test-nonce-value", value)

	// AND: Production namespace keys should not exist
	prodKeys, err := redisClient.Keys(ctx, "siwe:*")
	assert.NoError(t, err)
	assert.Empty(t, prodKeys, "production Redis namespace contaminated")
}

// TC-DATA-001-05: Testcontainer Isolation
// Test: Each test gets fresh isolated containers
// Priority: P1 (Critical Data Integrity)
// TDD Phase: RED → Write this test first
func TestContainerIsolation(t *testing.T) {
	ctx := context.Background()

	// GIVEN: First test container
	db1, err := testutil.SetupTestPostgres(ctx)
	if err != nil {
		t.Fatalf("Failed to setup first test postgres: %v", err)
	}

	// Insert data in first container
	_, err = db1.DB.Exec(`
        CREATE SCHEMA IF NOT EXISTS auth_test;
        CREATE TABLE IF NOT EXISTS auth_test.test_table (id SERIAL PRIMARY KEY, value TEXT);
        INSERT INTO auth_test.test_table (value) VALUES ('container1');
    `)
	assert.NoError(t, err)

	// WHEN: Creating second test container
	db2, err := testutil.SetupTestPostgres(ctx)
	if err != nil {
		db1.Cleanup(ctx)
		t.Fatalf("Failed to setup second test postgres: %v", err)
	}

	// THEN: Second container should be isolated (no data from first)
	var count int
	err = db2.DB.QueryRow(`
        SELECT COUNT(*) FROM information_schema.tables 
        WHERE table_schema = 'auth_test' AND table_name = 'test_table'
    `).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count, "Second container not isolated from first")

	// Cleanup
	db1.Cleanup(ctx)
	db2.Cleanup(ctx)
}

