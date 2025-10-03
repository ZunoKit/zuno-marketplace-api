package utils

import (
	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/domain"
	orchestratorpb "github.com/quangdang46/NFT-Marketplace/shared/proto/orchestrator"
)

// ConvertCreateCollectionRequest converts protobuf request to domain input
func ConvertCreateCollectionRequest(req *orchestratorpb.PrepareCreateCollectionRequest) domain.PrepareCreateCollectionInput {
	input := domain.PrepareCreateCollectionInput{
		ChainID:  req.ChainId,
		Name:     req.Name,
		Symbol:   req.Symbol,
		Creator:  req.Creator,
		TokenURI: req.TokenUri,
		Type:     domain.Standard(req.Type),
	}

	// Handle optional fields properly
	if req.Description != "" {
		description := req.Description
		input.Description = &description
	}
	if req.MintPrice != 0 {
		mintPrice := req.MintPrice
		input.MintPrice = &mintPrice
	}
	if req.RoyaltyFee != 0 {
		royaltyFee := req.RoyaltyFee
		input.RoyaltyFee = &royaltyFee
	}
	if req.MaxSupply != 0 {
		maxSupply := req.MaxSupply
		input.MaxSupply = &maxSupply
	}
	if req.MintLimitPerWallet != 0 {
		mintLimitPerWallet := req.MintLimitPerWallet
		input.MintLimitPerWallet = &mintLimitPerWallet
	}
	if req.MintStartTime != 0 {
		mintStartTime := req.MintStartTime
		input.MintStartTime = &mintStartTime
	}
	if req.AllowlistMintPrice != 0 {
		allowlistMintPrice := req.AllowlistMintPrice
		input.AllowlistMintPrice = &allowlistMintPrice
	}
	if req.PublicMintPrice != 0 {
		publicMintPrice := req.PublicMintPrice
		input.PublicMintPrice = &publicMintPrice
	}
	if req.AllowlistStageDuration != 0 {
		allowlistStageDuration := req.AllowlistStageDuration
		input.AllowlistStageDuration = &allowlistStageDuration
	}

	return input
}

// ConvertMintRequest converts protobuf mint request to domain input
func ConvertMintRequest(req *orchestratorpb.PrepareMintRequest) domain.PrepareMintInput {
	return domain.PrepareMintInput{
		ChainID:  req.ChainId,
		Contract: req.Contract,
		Standard: domain.Standard(req.Standard),
		Minter:   req.Minter,
		Quantity: req.Quantity,
	}
}

// ConvertTrackTxRequest converts protobuf track tx request to domain input
func ConvertTrackTxRequest(req *orchestratorpb.TrackTxRequest) domain.TrackTxInput {
	var contractAddr *domain.Address
	if req.Contract != "" {
		contractAddr = &req.Contract
	}

	return domain.TrackTxInput{
		IntentID: req.IntentId,
		ChainID:  req.ChainId,
		TxHash:   req.TxHash,
		Contract: contractAddr,
	}
}

// ConvertCreateCollectionResponse converts domain result to protobuf response
func ConvertCreateCollectionResponse(result *domain.PrepareCreateCollectionResult) *orchestratorpb.PrepareCreateCollectionResponse {
	previewAddr := ""
	if result.Tx.PreviewAddress != nil {
		previewAddr = *result.Tx.PreviewAddress
	}

	return &orchestratorpb.PrepareCreateCollectionResponse{
		IntentId: result.IntentID,
		Tx: &orchestratorpb.TxRequest{
			To:             result.Tx.To,
			Data:           result.Tx.Data,
			Value:          result.Tx.Value,
			PreviewAddress: previewAddr,
		},
	}
}

// ConvertMintResponse converts domain mint result to protobuf response
func ConvertMintResponse(result *domain.PrepareMintResult) *orchestratorpb.PrepareMintResponse {
	previewAddr := ""
	if result.Tx.PreviewAddress != nil {
		previewAddr = *result.Tx.PreviewAddress
	}

	return &orchestratorpb.PrepareMintResponse{
		IntentId: result.IntentID,
		Tx: &orchestratorpb.TxRequest{
			To:             result.Tx.To,
			Data:           result.Tx.Data,
			Value:          result.Tx.Value,
			PreviewAddress: previewAddr,
		},
	}
}

// ConvertTrackTxResponse converts domain track tx result to protobuf response
func ConvertTrackTxResponse(ok bool) *orchestratorpb.TrackTxResponse {
	return &orchestratorpb.TrackTxResponse{
		Ok: ok,
	}
}

// ConvertIntentStatusResponse converts domain intent status to protobuf response
func ConvertIntentStatusResponse(result *domain.IntentStatusPayload) *orchestratorpb.GetIntentStatusResponse {
	chainID := ""
	if result.ChainID != nil {
		chainID = *result.ChainID
	}

	txHash := ""
	if result.TxHash != nil {
		txHash = *result.TxHash
	}

	contractAddr := ""
	if result.ContractAddress != nil {
		contractAddr = *result.ContractAddress
	}

	return &orchestratorpb.GetIntentStatusResponse{
		IntentId:        result.IntentID,
		Kind:            string(result.Kind),
		Status:          string(result.Status),
		ChainId:         chainID,
		TxHash:          txHash,
		ContractAddress: contractAddr,
	}
}
