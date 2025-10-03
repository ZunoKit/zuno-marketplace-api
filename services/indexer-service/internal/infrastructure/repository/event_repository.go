package repository

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
    "go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/domain"
	mongoClient "github.com/quangdang46/NFT-Marketplace/shared/mongo"
)

const (
	eventsCollection = "events.raw"
	auditCollection  = "audit_trail"
)

type EventRepository struct {
	client     *mongoClient.MongoDB
	database   *mongo.Database
	collection *mongo.Collection
	auditCol   *mongo.Collection
}

// NewEventRepository creates a new MongoDB event repository
func NewEventRepository(client *mongoClient.MongoDB) *EventRepository {
	database := client.GetDatabase()
	collection := database.Collection(eventsCollection)
	auditCol := database.Collection(auditCollection)

	repo := &EventRepository{
		client:     client,
		database:   database,
		collection: collection,
		auditCol:   auditCol,
	}

	// Create indexes for efficient querying
	repo.createIndexes()
	repo.createAuditIndexes()

	return repo
}

// createIndexes creates necessary indexes for the events collection
func (r *EventRepository) createIndexes() {
	ctx := context.Background()

	// Unique compound index for deduplication (chainId, txHash, logIndex)
    uniqueIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "chain_id", Value: 1},
			{Key: "tx_hash", Value: 1},
			{Key: "log_index", Value: 1},
		},
        Options: options.Index().SetUnique(true),
	}

	// Index for querying by chain and block number
	blockIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "chain_id", Value: 1},
			{Key: "block_number", Value: 1},
		},
	}

	// Index for querying by contract address
	contractIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "chain_id", Value: 1},
			{Key: "contract_address", Value: 1},
		},
	}

	// Index for querying by event name
	eventIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "chain_id", Value: 1},
			{Key: "event_name", Value: 1},
		},
	}

	// Index for sorting by creation time
	timeIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "created_at", Value: -1},
		},
	}

	indexes := []mongo.IndexModel{uniqueIndex, blockIndex, contractIndex, eventIndex, timeIndex}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		// Log error but don't fail - indexes might already exist
		fmt.Printf("Warning: failed to create indexes: %v\n", err)
	}
}

// createAuditIndexes creates indexes for the audit trail collection
func (r *EventRepository) createAuditIndexes() {
	ctx := context.Background()
	idx := []mongo.IndexModel{
		{Keys: bson.D{{Key: "session_id", Value: 1}, {Key: "timestamp", Value: -1}}},
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "timestamp", Value: -1}}},
		{Keys: bson.D{{Key: "tx_hash", Value: 1}}},
		{Keys: bson.D{{Key: "contract_address", Value: 1}, {Key: "timestamp", Value: -1}}},
	}
	_, err := r.auditCol.Indexes().CreateMany(ctx, idx)
	if err != nil {
		fmt.Printf("Warning: failed to create audit indexes: %v\n", err)
	}
}

// InsertAuditRecord writes an immutable audit record
func (r *EventRepository) InsertAuditRecord(ctx context.Context, record bson.M) error {
	if record == nil {
		return fmt.Errorf("audit record cannot be nil")
	}
	if _, ok := record["created_at"]; !ok {
		record["created_at"] = time.Now()
	}
	_, err := r.auditCol.InsertOne(ctx, record)
	if err != nil {
		return fmt.Errorf("failed to insert audit record: %w", err)
	}
	return nil
}

// StoreRawEvent stores a raw blockchain event with deduplication
func (r *EventRepository) StoreRawEvent(ctx context.Context, event *domain.RawEvent) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	// Generate unique ID if not provided
	if event.ID == "" {
		event.ID = generateEventID(event.ChainID, event.TxHash, event.LogIndex)
	}

	// Set timestamps
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	if event.ObservedAt.IsZero() {
		event.ObservedAt = time.Now()
	}

	// Convert big.Int to string for MongoDB storage
	eventDoc := r.eventToDocument(event)

	// Use upsert to handle duplicates gracefully
	filter := bson.M{
		"chain_id":  event.ChainID,
		"tx_hash":   event.TxHash,
		"log_index": event.LogIndex,
	}

	update := bson.M{
		"$setOnInsert": eventDoc,
	}

    updOpts := options.UpdateOne().SetUpsert(true)
    result, err := r.collection.UpdateOne(ctx, filter, update, updOpts)
	if err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}

	// Check if this was a duplicate (no upsert occurred)
	if result.UpsertedCount == 0 && result.ModifiedCount == 0 {
		// Event already exists, this is not an error for idempotency
		return nil
	}

	return nil
}

// GetRawEvent retrieves a raw event by unique key
func (r *EventRepository) GetRawEvent(ctx context.Context, chainID, txHash string, logIndex int) (*domain.RawEvent, error) {
	filter := bson.M{
		"chain_id":  chainID,
		"tx_hash":   txHash,
		"log_index": logIndex,
	}

	var eventDoc bson.M
	err := r.collection.FindOne(ctx, filter).Decode(&eventDoc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("event not found: chain=%s, tx=%s, logIndex=%d", chainID, txHash, logIndex)
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	return r.documentToEvent(eventDoc)
}

// GetEventsByBlock retrieves all events for a specific block
func (r *EventRepository) GetEventsByBlock(ctx context.Context, chainID string, blockNumber *big.Int) ([]*domain.RawEvent, error) {
	filter := bson.M{
		"chain_id":     chainID,
		"block_number": blockNumber.String(),
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find events for block: %w", err)
	}
	defer cursor.Close(ctx)

	var events []*domain.RawEvent
	for cursor.Next(ctx) {
		var eventDoc bson.M
		if err := cursor.Decode(&eventDoc); err != nil {
			return nil, fmt.Errorf("failed to decode event: %w", err)
		}

		event, err := r.documentToEvent(eventDoc)
		if err != nil {
			return nil, fmt.Errorf("failed to convert document to event: %w", err)
		}

		events = append(events, event)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return events, nil
}

// GetEventsByContract retrieves events for a specific contract
func (r *EventRepository) GetEventsByContract(ctx context.Context, chainID, contractAddress string, limit int64) ([]*domain.RawEvent, error) {
	filter := bson.M{
		"chain_id":         chainID,
		"contract_address": contractAddress,
	}

    // Build find options using v2 API (builder pattern)
    findOpts := options.Find().SetSort(bson.M{"created_at": -1})
    if limit > 0 {
        findOpts = findOpts.SetLimit(limit)
    }

    cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to find events for contract: %w", err)
	}
	defer cursor.Close(ctx)

	var events []*domain.RawEvent
	for cursor.Next(ctx) {
		var eventDoc bson.M
		if err := cursor.Decode(&eventDoc); err != nil {
			return nil, fmt.Errorf("failed to decode event: %w", err)
		}

		event, err := r.documentToEvent(eventDoc)
		if err != nil {
			return nil, fmt.Errorf("failed to convert document to event: %w", err)
		}

		events = append(events, event)
	}

	return events, nil
}

// EventToDocument converts a domain event to a MongoDB document
// Exported for testing purposes
func (r *EventRepository) EventToDocument(event *domain.RawEvent) bson.M {
	return r.eventToDocument(event)
}

// eventToDocument converts a domain event to a MongoDB document
func (r *EventRepository) eventToDocument(event *domain.RawEvent) bson.M {
	doc := bson.M{
		"_id":              event.ID,
		"chain_id":         event.ChainID,
		"tx_hash":          event.TxHash,
		"log_index":        event.LogIndex,
		"block_number":     event.BlockNumber.String(),
		"block_hash":       event.BlockHash,
		"contract_address": event.ContractAddress,
		"event_name":       event.EventName,
		"event_signature":  event.EventSignature,
		"raw_data":         event.RawData,
		"parsed_json":      event.ParsedJSON,
		"confirmations":    event.Confirmations,
		"observed_at":      event.ObservedAt,
		"created_at":       event.CreatedAt,
	}

	return doc
}

// DocumentToEvent converts a MongoDB document to a domain event
// Exported for testing purposes
func (r *EventRepository) DocumentToEvent(doc bson.M) (*domain.RawEvent, error) {
	return r.documentToEvent(doc)
}

// documentToEvent converts a MongoDB document to a domain event
func (r *EventRepository) documentToEvent(doc bson.M) (*domain.RawEvent, error) {
	event := &domain.RawEvent{}

	// Extract fields with type conversion
	if id, ok := doc["_id"].(string); ok {
		event.ID = id
	}

	if chainID, ok := doc["chain_id"].(string); ok {
		event.ChainID = chainID
	}

	if txHash, ok := doc["tx_hash"].(string); ok {
		event.TxHash = txHash
	}

	if logIndex, ok := doc["log_index"].(int32); ok {
		event.LogIndex = int(logIndex)
	} else if logIndex, ok := doc["log_index"].(int64); ok {
		event.LogIndex = int(logIndex)
	}

	if blockNumberStr, ok := doc["block_number"].(string); ok {
		blockNumber := new(big.Int)
		blockNumber.SetString(blockNumberStr, 10)
		event.BlockNumber = blockNumber
	}

	if blockHash, ok := doc["block_hash"].(string); ok {
		event.BlockHash = blockHash
	}

	if contractAddress, ok := doc["contract_address"].(string); ok {
		event.ContractAddress = contractAddress
	}

	if eventName, ok := doc["event_name"].(string); ok {
		event.EventName = eventName
	}

	if eventSignature, ok := doc["event_signature"].(string); ok {
		event.EventSignature = eventSignature
	}

	if rawData, ok := doc["raw_data"].(bson.M); ok {
		// Convert bson.M to map[string]interface{}
		event.RawData = make(map[string]interface{})
		for k, v := range rawData {
			event.RawData[k] = v
		}
	}

	if parsedJSON, ok := doc["parsed_json"].(string); ok {
		event.ParsedJSON = parsedJSON
	}

	if confirmations, ok := doc["confirmations"].(int32); ok {
		event.Confirmations = int(confirmations)
	} else if confirmations, ok := doc["confirmations"].(int64); ok {
		event.Confirmations = int(confirmations)
	}

	if observedAt, ok := doc["observed_at"].(time.Time); ok {
		event.ObservedAt = observedAt
	}

	if createdAt, ok := doc["created_at"].(time.Time); ok {
		event.CreatedAt = createdAt
	}

	return event, nil
}

// GenerateEventID creates a unique ID for an event
// Exported for testing purposes
func GenerateEventID(chainID, txHash string, logIndex int) string {
	return generateEventID(chainID, txHash, logIndex)
}

// generateEventID creates a unique ID for an event
func generateEventID(chainID, txHash string, logIndex int) string {
	return fmt.Sprintf("%s_%s_%d", chainID, txHash, logIndex)
}

// Health check for the repository
func (r *EventRepository) HealthCheck(ctx context.Context) error {
	return r.client.HealthCheck(ctx)
}
