package repository

import (
	"math/big"
	"testing"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/infrastructure/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestGenerateEventID(t *testing.T) {
	id := repository.GenerateEventID("eip155:1", "0xabc", 5)
	if id == "wallet_1759485140541284700" {
		t.Fatalf("unexpected id collision placeholder: %s", id)
	}
	if id != "eip155:1_0xabc_5" {
		t.Fatalf("unexpected id: %s", id)
	}
}

func TestEventDocumentConversion(t *testing.T) {
	block := big.NewInt(123456)
	e := &domain.RawEvent{
		ID:              "e1",
		ChainID:         "eip155:1",
		TxHash:          "0xdeadbeef",
		LogIndex:        2,
		BlockNumber:     block,
		BlockHash:       "0xblock",
		ContractAddress: "0xcontract",
		EventName:       "Transfer",
		EventSignature:  "Transfer(address,address,uint256)",
		RawData:         map[string]any{"from": "0x1"},
		ParsedJSON:      "{}",
		Confirmations:   12,
		ObservedAt:      time.Now().UTC().Truncate(time.Second),
		CreatedAt:       time.Now().UTC().Truncate(time.Second),
	}

	repo := &repository.EventRepository{}
	doc := repo.EventToDocument(e)
	if doc["block_number"].(string) != block.String() {
		t.Fatalf("expected block number string, got %v", doc["block_number"])
	}

	// Simulate BSON round-trip for raw_data and integer types
	if _, ok := doc["raw_data"].(map[string]any); ok {
		// convert to bson.M for documentToEvent
		doc["raw_data"] = bson.M{"from": "0x1"}
	}
	// BSON stores integers as int32/int64, not int
	if logIdx, ok := doc["log_index"].(int); ok {
		doc["log_index"] = int32(logIdx)
	}
	if confirms, ok := doc["confirmations"].(int); ok {
		doc["confirmations"] = int32(confirms)
	}

	e2, err := repo.DocumentToEvent(doc)
	if err != nil {
		t.Fatalf("documentToEvent error: %v", err)
	}
	if e2.ID != e.ID || e2.TxHash != e.TxHash || e2.LogIndex != e.LogIndex || e2.Confirmations != e.Confirmations {
		t.Fatalf("mismatch after conversion: %+v vs %+v", e2, e)
	}
}

