package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestDatabase provides a test database instance
type TestDatabase struct {
	Container testcontainers.Container
	DB        *sql.DB
	DSN       string
}

// SetupTestPostgres creates a PostgreSQL container for testing
func SetupTestPostgres(ctx context.Context) (*TestDatabase, error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections"),
			wait.ForListeningPort("5432/tcp"),
		).WithDeadline(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return nil, fmt.Errorf("failed to get container port: %w", err)
	}

	dsn := fmt.Sprintf("host=%s port=%s user=test password=test dbname=testdb sslmode=disable",
		host, port.Port())

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Wait for database to be ready
	for i := 0; i < 30; i++ {
		if err := db.Ping(); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Create auth_test schema for QA tests
	_, err = db.Exec("CREATE SCHEMA IF NOT EXISTS auth_test")
	if err != nil {
		return nil, fmt.Errorf("failed to create auth_test schema: %w", err)
	}

	return &TestDatabase{
		Container: container,
		DB:        db,
		DSN:       dsn,
	}, nil
}

// Cleanup terminates the container and closes connections
func (td *TestDatabase) Cleanup(ctx context.Context) error {
	if td.DB != nil {
		td.DB.Close()
		td.DB = nil
	}
	if td.Container != nil {
		err := td.Container.Terminate(ctx)
		td.Container = nil
		return err
	}
	return nil
}

// SetupTestRedis creates a Redis container for testing
func SetupTestRedis(ctx context.Context) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to start redis container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "6379")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get container port: %w", err)
	}

	redisURL := fmt.Sprintf("redis://%s:%s", host, port.Port())

	return container, redisURL, nil
}

// SetupTestMongoDB creates a MongoDB container for testing
func SetupTestMongoDB(ctx context.Context) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "mongo:6",
		ExposedPorts: []string{"27017/tcp"},
		WaitingFor:   wait.ForListeningPort("27017/tcp").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to start mongodb container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "27017")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get container port: %w", err)
	}

	mongoURL := fmt.Sprintf("mongodb://%s:%s", host, port.Port())

	return container, mongoURL, nil
}
