package encode

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/domain"
	chainpb "github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:generate mockgen -destination=mock_chainregistry_test.go -package=encode github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry ChainRegistryServiceClient

func TestEncodeMint_ERC721(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock chain registry client
	mockClient := NewMockChainRegistryServiceClient(ctrl)
	encoder := NewEncoder(mockClient)

	// Test data
	chainID := domain.ChainID("eip155:1")
	contract := domain.Address("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb2")
	standard := domain.StdERC721
	minter := domain.Address("0xUser123456789012345678901234567890123456")

	// Mock ABI response for ERC721
	erc721ABI := `{
		"abi": [
			{
				"inputs": [{"name": "to", "type": "address"}],
				"name": "mint",
				"outputs": [],
				"stateMutability": "payable",
				"type": "function"
			}
		]
	}`

	// Setup expectations
	mockClient.EXPECT().
		GetAbiByAddress(ctx, &chainpb.GetAbiByAddressRequest{
			ChainId: string(chainID),
			Address: string(contract),
		}).
		Return(&chainpb.GetAbiBlobResponse{
			AbiJson: erc721ABI,
		}, nil)

	// No need to mock GetGasPolicy anymore since we're using default value

	// Execute
	input := domain.PrepareMintInput{
		ChainID:  chainID,
		Contract: contract,
		Minter:   minter,
		Standard: standard,
		Quantity: uint64(1),
	}

	to, data, value, err := encoder.EncodeMint(ctx, chainID, contract, standard, input)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, contract, to)
	assert.NotEmpty(t, data)
	assert.Equal(t, "0", value)

	// Verify the encoded data is not empty (contains the method signature and encoded address)
	// The first 4 bytes are the method signature
	assert.True(t, len(data) >= 4, "encoded data should contain at least method signature")
}

func TestEncodeMint_ERC721_WithQuantity(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockChainRegistryServiceClient(ctrl)
	encoder := NewEncoder(mockClient)

	chainID := domain.ChainID("eip155:1")
	contract := domain.Address("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb2")
	standard := domain.StdERC721
	minter := domain.Address("0xUser123456789012345678901234567890123456")

	// Mock ABI with mint(address, uint256) signature
	erc721ABI := `{
		"abi": [
			{
				"inputs": [
					{"name": "to", "type": "address"},
					{"name": "quantity", "type": "uint256"}
				],
				"name": "mint",
				"outputs": [],
				"stateMutability": "payable",
				"type": "function"
			}
		]
	}`

	mockClient.EXPECT().
		GetAbiByAddress(ctx, gomock.Any()).
		Return(&chainpb.GetAbiBlobResponse{
			AbiJson: erc721ABI,
		}, nil)

	// No need to mock GetGasPolicy anymore

	input := domain.PrepareMintInput{
		ChainID:  chainID,
		Contract: contract,
		Minter:   minter,
		Standard: standard,
		Quantity: uint64(5),
	}

	to, data, value, err := encoder.EncodeMint(ctx, chainID, contract, standard, input)

	require.NoError(t, err)
	assert.Equal(t, contract, to)
	assert.NotEmpty(t, data)
	assert.Equal(t, "0", value) // No mint price in policy
}

func TestEncodeMint_ERC1155(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockChainRegistryServiceClient(ctrl)
	encoder := NewEncoder(mockClient)

	chainID := domain.ChainID("eip155:137") // Polygon
	contract := domain.Address("0x1155Contract1234567890123456789012345678")
	standard := domain.StdERC1155
	minter := domain.Address("0xUser123456789012345678901234567890123456")

	// Mock ABI for ERC1155
	erc1155ABI := `{
		"abi": [
			{
				"inputs": [
					{"name": "to", "type": "address"},
					{"name": "id", "type": "uint256"},
					{"name": "amount", "type": "uint256"},
					{"name": "data", "type": "bytes"}
				],
				"name": "mint",
				"outputs": [],
				"stateMutability": "payable",
				"type": "function"
			}
		]
	}`

	mockClient.EXPECT().
		GetAbiByAddress(ctx, &chainpb.GetAbiByAddressRequest{
			ChainId: string(chainID),
			Address: string(contract),
		}).
		Return(&chainpb.GetAbiBlobResponse{
			AbiJson: erc1155ABI,
		}, nil)

	// No need to mock GetGasPolicy anymore

	input := domain.PrepareMintInput{
		ChainID:  chainID,
		Contract: contract,
		Minter:   minter,
		Standard: standard,
		Quantity: uint64(10),
	}

	to, data, value, err := encoder.EncodeMint(ctx, chainID, contract, standard, input)

	require.NoError(t, err)
	assert.Equal(t, contract, to)
	assert.NotEmpty(t, data)
	assert.Equal(t, "0", value)
}

func TestEncodeMint_NoABI(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockChainRegistryServiceClient(ctrl)
	encoder := NewEncoder(mockClient)

	chainID := domain.ChainID("eip155:1")
	contract := domain.Address("0xInvalidContract")
	standard := domain.StdERC721
	minter := domain.Address("0xUser123456789012345678901234567890123456")

	// Mock ABI not found error
	mockClient.EXPECT().
		GetAbiByAddress(ctx, gomock.Any()).
		Return(nil, errors.New("ABI not found"))

	input := domain.PrepareMintInput{
		ChainID:  chainID,
		Contract: contract,
		Minter:   minter,
		Standard: standard,
		Quantity: uint64(1),
	}

	_, _, _, err := encoder.EncodeMint(ctx, chainID, contract, standard, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get ABI for contract")
}

func TestEncodeMint_UnsupportedStandard(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockChainRegistryServiceClient(ctrl)
	encoder := NewEncoder(mockClient)

	chainID := domain.ChainID("eip155:1")
	contract := domain.Address("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb2")
	standard := domain.Standard("ERC20") // Unsupported
	minter := domain.Address("0xUser123456789012345678901234567890123456")

	// Mock valid ABI response
	validABI := `{
		"abi": [
			{
				"inputs": [{"name": "to", "type": "address"}],
				"name": "mint",
				"outputs": [],
				"type": "function"
			}
		]
	}`

	mockClient.EXPECT().
		GetAbiByAddress(ctx, gomock.Any()).
		Return(&chainpb.GetAbiBlobResponse{
			AbiJson: validABI,
		}, nil)

	// No need to mock GetGasPolicy anymore

	input := domain.PrepareMintInput{
		ChainID:  chainID,
		Contract: contract,
		Minter:   minter,
		Standard: standard,
		Quantity: uint64(1),
	}

	_, _, _, err := encoder.EncodeMint(ctx, chainID, contract, standard, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported token standard")
}

func TestEncodeMint_NoMintMethod(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockChainRegistryServiceClient(ctrl)
	encoder := NewEncoder(mockClient)

	chainID := domain.ChainID("eip155:1")
	contract := domain.Address("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb2")
	standard := domain.StdERC721
	minter := domain.Address("0xUser123456789012345678901234567890123456")

	// Mock ABI without mint method
	noMintABI := `{
		"abi": [
			{
				"inputs": [],
				"name": "totalSupply",
				"outputs": [{"name": "", "type": "uint256"}],
				"type": "function"
			}
		]
	}`

	mockClient.EXPECT().
		GetAbiByAddress(ctx, gomock.Any()).
		Return(&chainpb.GetAbiBlobResponse{
			AbiJson: noMintABI,
		}, nil)

	// No need to mock GetGasPolicy anymore

	input := domain.PrepareMintInput{
		ChainID:  chainID,
		Contract: contract,
		Minter:   minter,
		Standard: standard,
		Quantity: uint64(1),
	}

	_, _, _, err := encoder.EncodeMint(ctx, chainID, contract, standard, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no suitable mint method found")
}

func TestEncodeMint_EmptyMinter(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := NewMockChainRegistryServiceClient(ctrl)
	encoder := NewEncoder(mockClient)

	chainID := domain.ChainID("eip155:1")
	contract := domain.Address("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb2")
	standard := domain.StdERC721

	// Mock valid ABI
	validABI := `{
		"abi": [
			{
				"inputs": [{"name": "to", "type": "address"}],
				"name": "mint",
				"outputs": [],
				"type": "function"
			}
		]
	}`

	mockClient.EXPECT().
		GetAbiByAddress(ctx, gomock.Any()).
		Return(&chainpb.GetAbiBlobResponse{
			AbiJson: validABI,
		}, nil)

	input := domain.PrepareMintInput{
		ChainID:  chainID,
		Contract: contract,
		Minter:   "", // Empty minter
		Standard: standard,
		Quantity: uint64(1),
	}

	_, _, _, err := encoder.EncodeMint(ctx, chainID, contract, standard, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "minter address cannot be empty")
}

// Benchmark tests
func BenchmarkEncodeMint_ERC721(b *testing.B) {
	ctx := context.Background()
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()

	mockClient := NewMockChainRegistryServiceClient(ctrl)
	encoder := NewEncoder(mockClient)

	chainID := domain.ChainID("eip155:1")
	contract := domain.Address("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb2")
	standard := domain.StdERC721
	minter := domain.Address("0xUser123456789012345678901234567890123456")

	erc721ABI := `{
		"abi": [
			{
				"inputs": [{"name": "to", "type": "address"}],
				"name": "mint",
				"outputs": [],
				"type": "function"
			}
		]
	}`

	mockClient.EXPECT().
		GetAbiByAddress(ctx, gomock.Any()).
		Return(&chainpb.GetAbiBlobResponse{
			AbiJson: erc721ABI,
		}, nil).AnyTimes()

	// No need to mock GetGasPolicy anymore

	input := domain.PrepareMintInput{
		ChainID:  chainID,
		Contract: contract,
		Minter:   minter,
		Standard: standard,
		Quantity: uint64(1),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = encoder.EncodeMint(ctx, chainID, contract, standard, input)
	}
}

func BenchmarkEncodeMint_ERC1155(b *testing.B) {
	ctx := context.Background()
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()

	mockClient := NewMockChainRegistryServiceClient(ctrl)
	encoder := NewEncoder(mockClient)

	chainID := domain.ChainID("eip155:137")
	contract := domain.Address("0x1155Contract1234567890123456789012345678")
	standard := domain.StdERC1155
	minter := domain.Address("0xUser123456789012345678901234567890123456")

	erc1155ABI := `{
		"abi": [
			{
				"inputs": [
					{"name": "to", "type": "address"},
					{"name": "id", "type": "uint256"},
					{"name": "amount", "type": "uint256"},
					{"name": "data", "type": "bytes"}
				],
				"name": "mint",
				"outputs": [],
				"type": "function"
			}
		]
	}`

	mockClient.EXPECT().
		GetAbiByAddress(ctx, gomock.Any()).
		Return(&chainpb.GetAbiBlobResponse{
			AbiJson: erc1155ABI,
		}, nil).AnyTimes()

	// No need to mock GetGasPolicy anymore

	input := domain.PrepareMintInput{
		ChainID:  chainID,
		Contract: contract,
		Minter:   minter,
		Standard: standard,
		Quantity: uint64(10),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = encoder.EncodeMint(ctx, chainID, contract, standard, input)
	}
}
