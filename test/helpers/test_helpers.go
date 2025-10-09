package helpers

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestDatabase provides a test database
type TestDatabase struct {
	Container testcontainers.Container
	DB        *sql.DB
	DSN       string
}

// SetupTestPostgres creates a test PostgreSQL instance
func SetupTestPostgres(t *testing.T) *TestDatabase {
	ctx := context.Background()

	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:14-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	dsn, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
		postgresContainer.Terminate(ctx)
	})

	return &TestDatabase{
		Container: postgresContainer,
		DB:        db,
		DSN:       dsn,
	}
}

// TestRedis provides a test Redis instance
type TestRedis struct {
	Container testcontainers.Container
	URL       string
}

// SetupTestRedis creates a test Redis instance
func SetupTestRedis(t *testing.T) *TestRedis {
	ctx := context.Background()

	redisContainer, err := redis.RunContainer(ctx,
		testcontainers.WithImage("redis:7-alpine"),
		redis.WithSnapshotting(10, 1),
		redis.WithLogLevel(redis.LogLevelVerbose),
	)
	require.NoError(t, err)

	url, err := redisContainer.ConnectionString(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		redisContainer.Terminate(ctx)
	})

	return &TestRedis{
		Container: redisContainer,
		URL:       url,
	}
}

// TestGRPCServer provides a test gRPC server
type TestGRPCServer struct {
	Server   *grpc.Server
	Listener net.Listener
	Port     int
}

// SetupTestGRPCServer creates a test gRPC server
func SetupTestGRPCServer(t *testing.T) *TestGRPCServer {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := grpc.NewServer()
	port := listener.Addr().(*net.TCPAddr).Port

	go server.Serve(listener)

	t.Cleanup(func() {
		server.GracefulStop()
		listener.Close()
	})

	return &TestGRPCServer{
		Server:   server,
		Listener: listener,
		Port:     port,
	}
}

// CreateTestGRPCClient creates a test gRPC client
func CreateTestGRPCClient(t *testing.T, port int) *grpc.ClientConn {
	conn, err := grpc.Dial(
		fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		conn.Close()
	})

	return conn
}

// TestFixtures provides test data fixtures
type TestFixtures struct {
	UserID       string
	WalletAddr   string
	ChainID      string
	CollectionID string
	NFTID        string
	TxHash       string
}

// NewTestFixtures creates standard test fixtures
func NewTestFixtures() *TestFixtures {
	return &TestFixtures{
		UserID:       "550e8400-e29b-41d4-a716-446655440000",
		WalletAddr:   "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb0",
		ChainID:      "eip155:1",
		CollectionID: "660e8400-e29b-41d4-a716-446655440001",
		NFTID:        "770e8400-e29b-41d4-a716-446655440002",
		TxHash:       "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
	}
}

// AssertEventually asserts that condition becomes true within timeout
func AssertEventually(t *testing.T, condition func() bool, timeout time.Duration, msg string) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("Condition not met within %v: %s", timeout, msg)
}

// MockData generators
type MockDataGenerator struct {
	t *testing.T
}

// NewMockDataGenerator creates a new mock data generator
func NewMockDataGenerator(t *testing.T) *MockDataGenerator {
	return &MockDataGenerator{t: t}
}

// GenerateWalletAddress generates a valid Ethereum address
func (g *MockDataGenerator) GenerateWalletAddress() string {
	return fmt.Sprintf("0x%040x", rand.Int63())
}

// GenerateTxHash generates a valid transaction hash
func (g *MockDataGenerator) GenerateTxHash() string {
	return fmt.Sprintf("0x%064x", rand.Int63())
}

// GenerateSignature generates a mock signature
func (g *MockDataGenerator) GenerateSignature() string {
	return fmt.Sprintf("0x%0130x", rand.Int63())
}

// TableTest structure for table-driven tests
type TableTest struct {
	Name      string
	Input     interface{}
	Expected  interface{}
	ShouldErr bool
	ErrMsg    string
}

// RunTableTests runs table-driven tests
func RunTableTests(t *testing.T, tests []TableTest, testFunc func(interface{}) (interface{}, error)) {
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			result, err := testFunc(tt.Input)

			if tt.ShouldErr {
				assert.Error(t, err)
				if tt.ErrMsg != "" {
					assert.Contains(t, err.Error(), tt.ErrMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.Expected, result)
			}
		})
	}
}
