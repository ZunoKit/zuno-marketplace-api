package grpc_handler

import (
	"context"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/domain"
	authProto "github.com/quangdang46/NFT-Marketplace/shared/proto/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type gRPCHandler struct {
	authProto.UnimplementedAuthServiceServer
	authService domain.AuthService
}

func NewgRPCHandler(server *grpc.Server, authService domain.AuthService) *gRPCHandler {
	handler := &gRPCHandler{
		authService: authService,
	}
	return handler
}

func (g *gRPCHandler) GetNonce(ctx context.Context, req *authProto.GetNonceRequest) (*authProto.GetNonceResponse, error) {
	accountID := req.GetAccountId()
	chainID := req.GetChainId()
	domain := req.GetDomain()

	if accountID == "" || chainID == "" || domain == "" {
		return nil, status.Errorf(codes.InvalidArgument, "account_id, chain_id, and domain are required")
	}

	nonce, err := g.authService.GetNonce(ctx, accountID, chainID, domain)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get nonce: %v", err)
	}

	return &authProto.GetNonceResponse{
		Nonce: nonce,
	}, nil
}

func (g *gRPCHandler) VerifySiwe(ctx context.Context, req *authProto.VerifySiweRequest) (*authProto.VerifySiweResponse, error) {
	accountID := req.GetAccountId()
	message := req.GetMessage()
	signature := req.GetSignature()

	if accountID == "" || message == "" || signature == "" {
		return nil, status.Errorf(codes.InvalidArgument, "account_id, message, and signature are required")
	}

	result, err := g.authService.VerifySiwe(ctx, accountID, message, signature)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to verify SIWE: %v", err)
	}

	response := &authProto.VerifySiweResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt.Format(time.RFC3339),
		UserId:       result.UserID,
		Address:      result.Address,
		ChainId:      result.ChainID,
	}

	return response, nil
}

func (g *gRPCHandler) RefreshSession(ctx context.Context, req *authProto.RefreshSessionRequest) (*authProto.RefreshSessionResponse, error) {
	refreshToken := req.GetRefreshToken()
	if refreshToken == "" {
		return nil, status.Errorf(codes.InvalidArgument, "refresh_token is required")
	}

	result, err := g.authService.Refresh(ctx, refreshToken)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "failed to refresh session: %v", err)
	}

	response := &authProto.RefreshSessionResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt.Format(time.RFC3339),
		UserId:       result.UserID,
	}

	return response, nil
}

func (g *gRPCHandler) RevokeSession(ctx context.Context, req *authProto.RevokeSessionRequest) (*authProto.RevokeSessionResponse, error) {
	sessionID := req.GetSessionId()
	if sessionID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "session_id is required")
	}

	err := g.authService.Logout(ctx, sessionID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to revoke session: %v", err)
	}

	return &authProto.RevokeSessionResponse{
		Success: true,
	}, nil
}

func (g *gRPCHandler) RevokeSessionByRefreshToken(ctx context.Context, req *authProto.RevokeSessionByRefreshTokenRequest) (*authProto.RevokeSessionByRefreshTokenResponse, error) {
	refreshToken := req.GetRefreshToken()
	if refreshToken == "" {
		return nil, status.Errorf(codes.InvalidArgument, "refresh_token is required")
	}

	err := g.authService.LogoutByRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "failed to revoke session: %v", err)
	}

	return &authProto.RevokeSessionByRefreshTokenResponse{
		Success: true,
	}, nil
}
