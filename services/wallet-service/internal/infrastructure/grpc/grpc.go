package grpc

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/proto/wallet"
)

type WalletGRPCServer struct {
	wallet.UnimplementedWalletServiceServer
	service   domain.WalletService
	publisher domain.EventPublisher
}

func NewWalletGRPCServer(service domain.WalletService, publisher domain.EventPublisher) *WalletGRPCServer {
	return &WalletGRPCServer{
		service:   service,
		publisher: publisher,
	}
}

func (s *WalletGRPCServer) UpsertLink(ctx context.Context, req *wallet.UpsertLinkRequest) (*wallet.UpsertLinkResponse, error) {
	// Validate request
	if err := s.validateUpsertLinkRequest(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	// Convert gRPC request to domain model
	domainLink := s.requestToDomain(req)

	// Call service layer
	result, err := s.service.UpsertLink(ctx, domainLink)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "internal server error: %v", err)
	}

	// Publish event if wallet was linked (created or primary changed)
	if result.Created || result.PrimaryChanged {
		event := &domain.WalletLinkedEvent{
			UserID:    result.Link.UserID,
			AccountID: result.Link.AccountID,
			WalletID:  result.Link.ID,
			Address:   result.Link.Address,
			ChainID:   result.Link.ChainID,
			IsPrimary: result.Link.IsPrimary,
			LinkedAt:  time.Now(),
		}

		// Publish asynchronously - don't fail the response if publishing fails
		go func() {
			if publishErr := s.publisher.PublishWalletLinked(context.Background(), event); publishErr != nil {
				// Log error in production
				fmt.Printf("Failed to publish wallet linked event: %v\n", publishErr)
			}
		}()
	}

	// Convert domain result to gRPC response
	response := s.domainToResponse(result)

	return response, nil
}

func (s *WalletGRPCServer) validateUpsertLinkRequest(req *wallet.UpsertLinkRequest) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}
	if req.UserId == "" {
		return fmt.Errorf("user_id is required")
	}

	if req.AccountId == "" {
		return fmt.Errorf("account_id is required")
	}

	if req.Address == "" {
		return fmt.Errorf("address is required")
	}

	if req.ChainId == "" {
		return fmt.Errorf("chain_id is required")
	}

	return nil
}

func (s *WalletGRPCServer) requestToDomain(req *wallet.UpsertLinkRequest) domain.WalletLink {
	return domain.WalletLink{
		UserID:    req.UserId,
		AccountID: req.AccountId,
		Address:   req.Address,
		ChainID:   req.ChainId,
		IsPrimary: req.IsPrimary,
		// ID, timestamps will be set by the service/repository layers
	}
}

func (s *WalletGRPCServer) domainLinkToProto(link *domain.WalletLink) *wallet.WalletLink {
	protoLink := &wallet.WalletLink{
		Id:        link.ID,
		UserId:    link.UserID,
		AccountId: link.AccountID,
		Address:   link.Address,
		ChainId:   link.ChainID,
		IsPrimary: link.IsPrimary,
		CreatedAt: timestamppb.New(link.CreatedAt),
		UpdatedAt: timestamppb.New(link.UpdatedAt),
	}

	if link.VerifiedAt != nil {
		protoLink.VerifiedAt = timestamppb.New(*link.VerifiedAt)
	}

	return protoLink
}

func (s *WalletGRPCServer) domainToResponse(result *domain.WalletUpsertResult) *wallet.UpsertLinkResponse {
	return &wallet.UpsertLinkResponse{
		Link:           s.domainLinkToProto(result.Link),
		Created:        result.Created,
		PrimaryChanged: result.PrimaryChanged,
	}
}

// Exposed helper wrappers for testing
func (s *WalletGRPCServer) DomainLinkToProto(link *domain.WalletLink) *wallet.WalletLink {
	return s.domainLinkToProto(link)
}

func (s *WalletGRPCServer) RequestToDomain(req *wallet.UpsertLinkRequest) domain.WalletLink {
	return s.requestToDomain(req)
}
