package test

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/service"
)

func TestParseERC1155TransferSingle(t *testing.T) {
	// Create indexer service (mock dependencies)
	indexerService := &service.IndexerService{}
	mintIndexer := service.NewMintIndexer(indexerService)

	tests := []struct {
		name     string
		log      *domain.Log
		expected *domain.TransferSingleEvent
		wantErr  bool
	}{
		{
			name: "Valid TransferSingle event",
			log: &domain.Log{
				Topics: []string{
					"0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62", // TransferSingle event signature
					"0x000000000000000000000000123456789abcdef0123456789abcdef012345678", // operator
					"0x0000000000000000000000000000000000000000000000000000000000000000", // from (zero = mint)
					"0x000000000000000000000000abcdef0123456789abcdef0123456789abcdef01", // to
				},
				Data: "0x" +
					"0000000000000000000000000000000000000000000000000000000000000001" + // id = 1
					"0000000000000000000000000000000000000000000000000000000000000064", // value = 100
			},
			expected: &domain.TransferSingleEvent{
				Operator: "0x123456789abcdef0123456789abcdef012345678",
				From:     "0x0000000000000000000000000000000000000000",
				To:       "0xabcdef0123456789abcdef0123456789abcdef01",
				ID:       "1",
				Value:    "100",
			},
		},
		{
			name: "Large token ID and amount",
			log: &domain.Log{
				Topics: []string{
					"0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62",
					"0x000000000000000000000000123456789abcdef0123456789abcdef012345678",
					"0x0000000000000000000000001111111111111111111111111111111111111111",
					"0x0000000000000000000000002222222222222222222222222222222222222222",
				},
				Data: "0x" +
					"00000000000000000000000000000000000000000000000000000000deadbeef" + // id = 3735928559
					"0000000000000000000000000000000000000000000000000de0b6b3a7640000", // value = 1e18 (1 token with 18 decimals)
			},
			expected: &domain.TransferSingleEvent{
				Operator: "0x123456789abcdef0123456789abcdef012345678",
				From:     "0x1111111111111111111111111111111111111111",
				To:       "0x2222222222222222222222222222222222222222",
				ID:       "3735928559",
				Value:    "1000000000000000000",
			},
		},
		{
			name: "Invalid topics length",
			log: &domain.Log{
				Topics: []string{
					"0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62",
					"0x000000000000000000000000123456789abcdef0123456789abcdef012345678",
				},
				Data: "0x00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000064",
			},
			wantErr: true,
		},
		{
			name: "Invalid data length",
			log: &domain.Log{
				Topics: []string{
					"0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62",
					"0x000000000000000000000000123456789abcdef0123456789abcdef012345678",
					"0x0000000000000000000000000000000000000000000000000000000000000000",
					"0x000000000000000000000000abcdef0123456789abcdef0123456789abcdef01",
				},
				Data: "0x0001", // Too short
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mintIndexer.ParseERC1155TransferSingle(tt.log)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.Operator, result.Operator)
			assert.Equal(t, tt.expected.From, result.From)
			assert.Equal(t, tt.expected.To, result.To)
			assert.Equal(t, tt.expected.ID, result.ID)
			assert.Equal(t, tt.expected.Value, result.Value)
		})
	}
}

func TestParseERC1155TransferBatch(t *testing.T) {
	// Create indexer service
	indexerService := &service.IndexerService{}
	mintIndexer := service.NewMintIndexer(indexerService)

	tests := []struct {
		name     string
		log      *domain.Log
		expected *domain.TransferBatchEvent
		wantErr  bool
	}{
		{
			name: "Valid TransferBatch with single item",
			log: &domain.Log{
				Topics: []string{
					"0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb", // TransferBatch event signature
					"0x000000000000000000000000123456789abcdef0123456789abcdef012345678", // operator
					"0x0000000000000000000000000000000000000000000000000000000000000000", // from
					"0x000000000000000000000000abcdef0123456789abcdef0123456789abcdef01", // to
				},
				Data: createBatchData([]uint64{1}, []uint64{100}),
			},
			expected: &domain.TransferBatchEvent{
				Operator: "0x123456789abcdef0123456789abcdef012345678",
				From:     "0x0000000000000000000000000000000000000000",
				To:       "0xabcdef0123456789abcdef0123456789abcdef01",
				IDs:      []string{"1"},
				Values:   []string{"100"},
			},
		},
		{
			name: "Valid TransferBatch with multiple items",
			log: &domain.Log{
				Topics: []string{
					"0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb",
					"0x000000000000000000000000123456789abcdef0123456789abcdef012345678",
					"0x0000000000000000000000001111111111111111111111111111111111111111",
					"0x0000000000000000000000002222222222222222222222222222222222222222",
				},
				Data: createBatchData([]uint64{10, 20, 30}, []uint64{1, 2, 3}),
			},
			expected: &domain.TransferBatchEvent{
				Operator: "0x123456789abcdef0123456789abcdef012345678",
				From:     "0x1111111111111111111111111111111111111111",
				To:       "0x2222222222222222222222222222222222222222",
				IDs:      []string{"10", "20", "30"},
				Values:   []string{"1", "2", "3"},
			},
		},
		{
			name: "Invalid topics length",
			log: &domain.Log{
				Topics: []string{
					"0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb",
				},
				Data: createBatchData([]uint64{1}, []uint64{100}),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mintIndexer.ParseERC1155TransferBatch(tt.log)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.Operator, result.Operator)
			assert.Equal(t, tt.expected.From, result.From)
			assert.Equal(t, tt.expected.To, result.To)
			assert.Equal(t, tt.expected.IDs, result.IDs)
			assert.Equal(t, tt.expected.Values, result.Values)
		})
	}
}

// Helper function to create batch data for testing
func createBatchData(ids []uint64, values []uint64) string {
	// Calculate offsets
	idsOffset := uint64(64)                                  // After the two offset values
	valuesOffset := idsOffset + 32 + (uint64(len(ids)) * 32) // After ids array

	data := make([]byte, 0)

	// Add offset to ids array
	offsetBytes := make([]byte, 32)
	new(big.Int).SetUint64(idsOffset).FillBytes(offsetBytes)
	data = append(data, offsetBytes...)

	// Add offset to values array
	offsetBytes = make([]byte, 32)
	new(big.Int).SetUint64(valuesOffset).FillBytes(offsetBytes)
	data = append(data, offsetBytes...)

	// Add ids array length and elements
	lengthBytes := make([]byte, 32)
	new(big.Int).SetUint64(uint64(len(ids))).FillBytes(lengthBytes)
	data = append(data, lengthBytes...)

	for _, id := range ids {
		idBytes := make([]byte, 32)
		new(big.Int).SetUint64(id).FillBytes(idBytes)
		data = append(data, idBytes...)
	}

	// Add values array length and elements
	lengthBytes = make([]byte, 32)
	new(big.Int).SetUint64(uint64(len(values))).FillBytes(lengthBytes)
	data = append(data, lengthBytes...)

	for _, value := range values {
		valueBytes := make([]byte, 32)
		new(big.Int).SetUint64(value).FillBytes(valueBytes)
		data = append(data, valueBytes...)
	}

	return "0x" + hex.EncodeToString(data)
}
