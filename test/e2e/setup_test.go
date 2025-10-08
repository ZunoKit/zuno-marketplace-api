package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"
)

type E2ETestSuite struct {
	suite.Suite
	compose   *compose.LocalDockerCompose
	ctx       context.Context
	baseURL   string
	grpcPorts map[string]string
}

func (suite *E2ETestSuite) SetupSuite() {
	suite.ctx = context.Background()

	// Start docker-compose stack
	suite.compose = compose.NewLocalDockerCompose(
		[]string{"../../docker-compose.yml"},
		"nft-marketplace-e2e",
	)

	// Start all services
	execErr := suite.compose.
		WithCommand([]string{"up", "-d"}).
		Invoke()
	if execErr.Error != nil {
		suite.Require().NoError(execErr.Error)
	}

	// Wait for services to be ready
	suite.waitForServices()

	// Set up service URLs
	suite.baseURL = "http://localhost:8081"
	suite.grpcPorts = map[string]string{
		"auth":           "50051",
		"user":           "50052",
		"wallet":         "50053",
		"orchestrator":   "50054",
		"media":          "50055",
		"chain-registry": "50056",
		"catalog":        "50057",
		"indexer":        "50058",
	}
}

func (suite *E2ETestSuite) TearDownSuite() {
	// Stop and remove containers
	execErr := suite.compose.Down()
	if execErr.Error != nil {
		suite.Require().NoError(execErr.Error)
	}
}

func (suite *E2ETestSuite) waitForServices() {
	// Wait for PostgreSQL
	suite.waitForPostgres()

	// Wait for Redis
	suite.waitForRedis()

	// Wait for RabbitMQ
	suite.waitForRabbitMQ()

	// Wait for MongoDB
	suite.waitForMongoDB()

	// Wait for GraphQL Gateway
	suite.waitForGraphQL()

	// Additional wait for all services to initialize
	time.Sleep(10 * time.Second)
}

func (suite *E2ETestSuite) waitForPostgres() {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
	}

	suite.waitForContainer(req, "PostgreSQL")
}

func (suite *E2ETestSuite) waitForRedis() {
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second),
	}

	suite.waitForContainer(req, "Redis")
}

func (suite *E2ETestSuite) waitForRabbitMQ() {
	req := testcontainers.ContainerRequest{
		Image:        "rabbitmq:3-management-alpine",
		ExposedPorts: []string{"5672/tcp", "15672/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5672/tcp"),
			wait.ForHTTP("/").WithPort("15672/tcp"),
		).WithStartupTimeout(60 * time.Second),
	}

	suite.waitForContainer(req, "RabbitMQ")
}

func (suite *E2ETestSuite) waitForMongoDB() {
	req := testcontainers.ContainerRequest{
		Image:        "mongo:6",
		ExposedPorts: []string{"27017/tcp"},
		WaitingFor:   wait.ForListeningPort("27017/tcp").WithStartupTimeout(60 * time.Second),
	}

	suite.waitForContainer(req, "MongoDB")
}

func (suite *E2ETestSuite) waitForGraphQL() {
	// Wait for GraphQL Gateway to be ready
	req := testcontainers.ContainerRequest{
		ExposedPorts: []string{"8081/tcp"},
		WaitingFor: wait.ForHTTP("/health").
			WithPort("8081/tcp").
			WithStartupTimeout(120 * time.Second),
	}

	suite.waitForContainer(req, "GraphQL Gateway")
}

func (suite *E2ETestSuite) waitForContainer(req testcontainers.ContainerRequest, name string) {
	fmt.Printf("Waiting for %s to be ready...\n", name)
	// Implementation would check if container is ready
	// This is simplified for example
}

func TestE2ESuite(t *testing.T) {
	suite.Run(t, new(E2ETestSuite))
}
