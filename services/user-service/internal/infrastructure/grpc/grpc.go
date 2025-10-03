package grpc_handler

import (
	"context"

	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/domain"
	userProto "github.com/quangdang46/NFT-Marketplace/shared/proto/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type gRPCHandler struct {
	userProto.UnimplementedUserServiceServer
	userService domain.UserService
}

func NewgRPCHandler(userService domain.UserService) *gRPCHandler {
	handler := &gRPCHandler{
		userService: userService,
	}
	return handler
}

func (s *gRPCHandler) EnsureUser(ctx context.Context, req *userProto.EnsureUserRequest) (*userProto.EnsureUserResponse, error) {
	// Validate request
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	if req.AccountId == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id is required")
	}

	if req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "address is required")
	}

	// Call service layer
	result, err := s.userService.EnsureUser(ctx, req.AccountId, req.Address, req.ChainId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Return response
	return &userProto.EnsureUserResponse{
		UserId:  result.UserID,
		Created: result.Created,
	}, nil
}
