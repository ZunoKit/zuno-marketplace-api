package grpc_handler

import (
	"context"

	"github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/domain"
	protoWallet "github.com/quangdang46/NFT-Marketplace/shared/proto/wallet"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type gRPCHandler struct {
	protoWallet.UnimplementedWalletServiceServer
	walletService domain.WalletService
}

func NewgRPCHandler(server *grpc.Server, walletService domain.WalletService) *gRPCHandler {
	handler := &gRPCHandler{
		walletService: walletService,
	}
	return handler
}

func (g *gRPCHandler) UpsertLink(ctx context.Context, req *protoWallet.UpsertLinkRequest) (*protoWallet.UpsertLinkResponse, error) {
	userID := req.GetUserId()
	accountID := req.GetAccountId()
	address := req.GetAddress()
	chainID := req.GetChainId()
	isPrimary := req.GetIsPrimary()
	walletType := req.GetType()
	connector := req.GetConnector()
	label := req.GetLabel()

	if userID == "" || accountID == "" || address == "" || chainID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "user_id, account_id, address, and chain_id are required")
	}

	link, created, primaryChanged, err := g.walletService.UpsertLink(ctx,
		domain.UserID(userID),
		accountID,
		domain.Address(address),
		domain.ChainID(chainID),
		isPrimary,
		walletType,
		connector,
		label,
	)

	if err != nil {
		// Map domain errors to gRPC status codes
		if err == domain.ErrInvalidAddress {
			return nil, status.Errorf(codes.InvalidArgument, "invalid address: %v", err)
		}
		if err == domain.ErrInvalidChainID {
			return nil, status.Errorf(codes.InvalidArgument, "invalid chain ID: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to upsert wallet link: %v", err)
	}

	// Convert to proto response
	protoLink := &protoWallet.WalletLink{
		Id:         string(link.ID),
		UserId:     string(link.UserID),
		AccountId:  link.AccountID,
		Address:    string(link.Address),
		ChainId:    string(link.ChainID),
		IsPrimary:  link.IsPrimary,
		VerifiedAt: timestamppb.New(link.VerifiedAt),
		CreatedAt:  timestamppb.New(link.CreatedAt),
		UpdatedAt:  timestamppb.New(link.UpdatedAt),
	}

	return &protoWallet.UpsertLinkResponse{
		Link:           protoLink,
		Created:        created,
		PrimaryChanged: primaryChanged,
	}, nil
}
