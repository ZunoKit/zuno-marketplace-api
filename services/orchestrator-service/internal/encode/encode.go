package encode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	chainpb "github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry"

	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/utils"
)

type Encoder struct {
	chainRegistry chainpb.ChainRegistryServiceClient
}

func NewEncoder(chainRegistry chainpb.ChainRegistryServiceClient) domain.Encoder {
	return &Encoder{
		chainRegistry: chainRegistry,
	}
}

func (e *Encoder) EncodeCreateCollection(ctx context.Context, chainID domain.ChainID, factory domain.Address, p domain.PrepareCreateCollectionInput) (to domain.Address, data []byte, value string, preview *domain.Address, err error) {
	abiResp, err := e.chainRegistry.GetAbiByAddress(ctx, &chainpb.GetAbiByAddressRequest{
		ChainId: string(chainID),
		Address: string(factory),
	})
	if err != nil {
		return "", nil, "", nil, fmt.Errorf("get ABI by address: %w", err)
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(abiResp.AbiJson), &raw); err != nil {
		return "", nil, "", nil, fmt.Errorf("parse ABI json: %w", err)
	}
	abiArr, ok := raw["abi"]
	if !ok {
		return "", nil, "", nil, fmt.Errorf("abi field not found")
	}
	abiArrayBytes, err := json.Marshal(abiArr)
	if err != nil {
		return "", nil, "", nil, fmt.Errorf("marshal abi array: %w", err)
	}
	parsedABI, err := abi.JSON(bytes.NewReader(abiArrayBytes))
	if err != nil {
		return "", nil, "", nil, fmt.Errorf("parse abi: %w", err)
	}

	var methodName string
	switch p.Type {
	case domain.StdERC721:
		methodName = "createERC721Collection"
	case domain.StdERC1155:
		methodName = "createERC1155Collection"
	default:
		return "", nil, "", nil, fmt.Errorf("unsupported collection type: %s", p.Type)
	}

	if _, exists := parsedABI.Methods[methodName]; !exists {
		return "", nil, "", nil, fmt.Errorf("method %s not found in factory ABI", methodName)
	}

	if p.Name == "" {
		return "", nil, "", nil, fmt.Errorf("collection name cannot be empty")
	}
	if p.Symbol == "" {
		return "", nil, "", nil, fmt.Errorf("collection symbol cannot be empty")
	}
	if p.Creator == "" {
		return "", nil, "", nil, fmt.Errorf("creator address cannot be empty")
	}

	ownerAddr := common.HexToAddress(string(p.Creator))

	tuple := domain.CollectionParams{
		Name:                   p.Name,
		Symbol:                 p.Symbol,
		Owner:                  ownerAddr, // Use common.Address directly
		Description:            utils.GetStringValue(p.Description, ""),
		MintPrice:              utils.ToBigInt(utils.GetUint64Value(p.MintPrice, 0)),
		RoyaltyFee:             utils.ToBigInt(utils.GetUint64Value(p.RoyaltyFee, 0)),
		MaxSupply:              utils.ToBigInt(utils.GetUint64Value(p.MaxSupply, 0)),
		MintLimitPerWallet:     utils.ToBigInt(utils.GetUint64Value(p.MintLimitPerWallet, 0)),
		MintStartTime:          utils.ToBigInt(utils.GetUint64Value(p.MintStartTime, 0)),
		AllowlistMintPrice:     utils.ToBigInt(utils.GetUint64Value(p.AllowlistMintPrice, 0)),
		PublicMintPrice:        utils.ToBigInt(utils.GetUint64Value(p.PublicMintPrice, 0)),
		AllowlistStageDuration: utils.ToBigInt(utils.GetUint64Value(p.AllowlistStageDuration, 0)),
		TokenURI:               p.TokenURI,
	}

	packed, err := parsedABI.Pack(methodName, tuple)
	if err != nil {
		return "", nil, "", nil, fmt.Errorf("pack calldata: %w", err)
	}

	return factory, packed, "0", nil, nil
}

func (e *Encoder) EncodeMint(ctx context.Context, chainID domain.ChainID, contract domain.Address, standard domain.Standard, p domain.PrepareMintInput) (to domain.Address, data []byte, value string, err error) {
	return "", nil, "", nil
}
