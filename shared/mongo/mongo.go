package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// MongoConfig holds MongoDB connection configuration
type MongoConfig struct {
	MongoURI      string `json:"mongo_uri"`
	MongoDatabase string `json:"mongo_database"`
}

// MongoDB represents a MongoDB connection wrapper
type MongoDB struct {
	client   *mongo.Client
	database *mongo.Database
}

// NewMongoFromConfig creates a new MongoDB connection from configuration
func NewMongo(cfg MongoConfig) (*MongoDB, error) {

	clientOptions := options.Client().
		ApplyURI(cfg.MongoURI)

	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongodb: %w", err)
	}

	// Get database reference
	database := client.Database(cfg.MongoDatabase)

	return &MongoDB{
		client:   client,
		database: database,
	}, nil
}

func (m *MongoDB) HealthCheck(ctx context.Context) error {
	return m.client.Ping(ctx, readpref.Primary())
}

func (m *MongoDB) GetClient() *mongo.Client {
	return m.client
}

func (m *MongoDB) GetDatabase() *mongo.Database {
	return m.database
}

func (m *MongoDB) IsConnected(ctx context.Context) bool {
	if m.client == nil {
		return false
	}
	return m.client.Ping(ctx, readpref.Primary()) == nil
}

func (m *MongoDB) Close(ctx context.Context) error {
	if m.client != nil {
		return m.client.Disconnect(ctx)
	}
	return nil
}
