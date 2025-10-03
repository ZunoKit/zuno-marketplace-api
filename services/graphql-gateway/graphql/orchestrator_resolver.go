package graphql_resolver

import (
	"context"
	"fmt"

	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql/schemas"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/middleware"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/utils"
	orchestratorpb "github.com/quangdang46/NFT-Marketplace/shared/proto/orchestrator"
)

func (r *MutationResolver) PrepareCreateCollection(ctx context.Context, input schemas.PrepareCreateCollectionInput) (*schemas.PrepareCreateCollectionPayload, error) {
	// Validate input early
	if input.ChainID == "" || input.Name == "" || input.Symbol == "" || input.Type == "" {
		return nil, fmt.Errorf("invalid prepare create collection input: missing required fields")
	}

	// Get current user for creator field
	user := middleware.GetCurrentUser(ctx)
	if user == nil {
		return nil, fmt.Errorf("authentication required")
	}

	if r.server.orchestratorClient == nil || r.server.orchestratorClient.Client == nil {
		return nil, fmt.Errorf("orchestrator service unavailable")
	}

	// Optional fields
	var tokenURI, description string
	var mintPrice, royaltyFee, maxSupply, mintLimitPerWallet, mintStartTime, allowlistMintPrice, publicMintPrice, allowlistStageDuration uint64
	if input.TokenURI != nil {
		tokenURI = *input.TokenURI
	}
	if input.Description != nil {
		description = *input.Description
	}

	mintPrice = utils.ParseUint64(input.MintPrice)
	royaltyFee = utils.ParseUint64(input.RoyaltyFee)
	maxSupply = utils.ParseUint64(input.MaxSupply)
	mintLimitPerWallet = utils.ParseUint64(input.MintLimitPerWallet)
	mintStartTime = utils.ParseUint64(input.MintStartTime)
	allowlistMintPrice = utils.ParseUint64(input.AllowlistMintPrice)
	publicMintPrice = utils.ParseUint64(input.PublicMintPrice)
	allowlistStageDuration = utils.ParseUint64(input.AllowlistStageDuration)

	// Call orchestrator service
	resp, err := (*r.server.orchestratorClient.Client).PrepareCreateCollection(ctx, &orchestratorpb.PrepareCreateCollectionRequest{
		ChainId:                input.ChainID,
		Name:                   input.Name,
		Symbol:                 input.Symbol,
		Creator:                user.UserID,
		TokenUri:               tokenURI,
		Type:                   input.Type, // Add the type field
		Description:            description,
		MintPrice:              mintPrice,
		RoyaltyFee:             royaltyFee,
		MaxSupply:              maxSupply,
		MintLimitPerWallet:     mintLimitPerWallet,
		MintStartTime:          mintStartTime,
		AllowlistMintPrice:     allowlistMintPrice,
		PublicMintPrice:        publicMintPrice,
		AllowlistStageDuration: allowlistStageDuration,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to prepare create collection: %w", err)
	}

	// Convert response to GraphQL schema
	txRequest := &schemas.TxRequest{
		To:             resp.Tx.To,
		Data:           string(resp.Tx.Data),
		Value:          resp.Tx.Value,
		PreviewAddress: &resp.Tx.PreviewAddress,
	}

	return &schemas.PrepareCreateCollectionPayload{
		IntentID:  resp.IntentId,
		TxRequest: txRequest,
	}, nil
}

func (r *MutationResolver) PrepareMint(ctx context.Context, input schemas.PrepareMintInput) (*schemas.PrepareMintPayload, error) {
	// Validate input early
	if input.ChainID == "" || input.Contract == "" || input.Standard == "" {
		return nil, fmt.Errorf("invalid prepare mint input: missing required fields")
	}

	// Get current user for minter field
	user := middleware.GetCurrentUser(ctx)
	if user == nil {
		return nil, fmt.Errorf("authentication required")
	}

	if r.server.orchestratorClient == nil || r.server.orchestratorClient.Client == nil {
		return nil, fmt.Errorf("orchestrator service unavailable")
	}

	// Prepare quantity field
	quantity := uint64(1) // default value
	if input.Quantity != nil {
		quantity = uint64(*input.Quantity)
	}

	// Call orchestrator service
	resp, err := (*r.server.orchestratorClient.Client).PrepareMint(ctx, &orchestratorpb.PrepareMintRequest{
		ChainId:  input.ChainID,
		Contract: input.Contract,
		Minter:   user.UserID,
		Standard: input.Standard,
		Quantity: quantity,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to prepare mint: %w", err)
	}

	// Convert response to GraphQL schema
	txRequest := &schemas.TxRequest{
		To:             resp.Tx.To,
		Data:           string(resp.Tx.Data),
		Value:          resp.Tx.Value,
		PreviewAddress: &resp.Tx.PreviewAddress,
	}

	return &schemas.PrepareMintPayload{
		IntentID:  resp.IntentId,
		TxRequest: txRequest,
	}, nil
}

func (r *MutationResolver) TrackTx(ctx context.Context, input schemas.TrackTxInput) (bool, error) {
	// Validate input early
	if input.IntentID == "" || input.ChainID == "" || input.TxHash == "" {
		return false, fmt.Errorf("invalid track tx input: missing required fields")
	}

	if r.server.orchestratorClient == nil || r.server.orchestratorClient.Client == nil {
		return false, fmt.Errorf("orchestrator service unavailable")
	}

	// Prepare contract field
	var contract string
	if input.Contract != nil {
		contract = *input.Contract
	}

	// Call orchestrator service
	resp, err := (*r.server.orchestratorClient.Client).TrackTx(ctx, &orchestratorpb.TrackTxRequest{
		IntentId: input.IntentID,
		ChainId:  input.ChainID,
		TxHash:   input.TxHash,
		Contract: contract,
	})
	if err != nil {
		return false, fmt.Errorf("failed to track transaction: %w", err)
	}

	return resp.Ok, nil
}
