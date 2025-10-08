package grpc_handler

import (
	"context"

	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/domain"
	protoUser "github.com/quangdang46/NFT-Marketplace/shared/proto/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type gRPCHandler struct {
	protoUser.UnimplementedUserServiceServer
	userService domain.UserService
}

func NewgRPCHandler(server *grpc.Server, userService domain.UserService) *gRPCHandler {
	handler := &gRPCHandler{
		userService: userService,
	}
	return handler
}

func (g *gRPCHandler) EnsureUser(ctx context.Context, req *protoUser.EnsureUserRequest) (*protoUser.EnsureUserResponse, error) {
	accountID := req.GetAccountId()
	address := req.GetAddress()
	chainID := req.GetChainId()

	if accountID == "" || address == "" {
		return nil, status.Errorf(codes.InvalidArgument, "account_id and address are required")
	}

	userID, created, err := g.userService.EnsureUser(ctx, accountID, address, chainID)
	if err != nil {
		// Map domain errors to gRPC status codes
		if err == domain.ErrInvalidAddress {
			return nil, status.Errorf(codes.InvalidArgument, "invalid address: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to ensure user: %v", err)
	}

	return &protoUser.EnsureUserResponse{
		UserId:  string(userID),
		Created: created,
	}, nil
}
