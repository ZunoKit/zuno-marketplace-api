package graphql_resolver

import (
	"context"
	"fmt"
	"io"

	"github.com/99designs/gqlgen/graphql"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql/schemas"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/middleware"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/utils"
	authpb "github.com/quangdang46/NFT-Marketplace/shared/proto/auth"
	chainregpb "github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry"
	"github.com/quangdang46/NFT-Marketplace/shared/proto/media"
)

type MutationResolver struct {
	server *Resolver
}

func (r *MutationResolver) SignInSiwe(ctx context.Context, input schemas.SignInSiweInput) (*schemas.NoncePayload, error) {
	// Validate input early
	if input.AccountID == "" || input.ChainID == "" || input.Domain == "" {
		return nil, fmt.Errorf("invalid sign in siwe input")
	}
	if r.server.authClient == nil {
		return nil, fmt.Errorf("auth service unavailable")
	}
	nonceResponse, err := (*r.server.authClient.Client).GetNonce(ctx, &authpb.GetNonceRequest{
		AccountId: input.AccountID,
		ChainId:   input.ChainID,
		Domain:    input.Domain,
	})
	if err != nil {
		return nil, err
	}

	return &schemas.NoncePayload{Nonce: nonceResponse.Nonce}, nil
}

func (r *MutationResolver) VerifySiwe(ctx context.Context, input schemas.VerifySiweInput) (*schemas.AuthPayload, error) {
	if input.AccountID == "" || input.Message == "" || input.Signature == "" {
		return nil, fmt.Errorf("invalid verify siwe input")
	}
	if r.server.authClient == nil {
		return nil, fmt.Errorf("auth service unavailable")
	}

	resp, err := (*r.server.authClient.Client).VerifySiwe(ctx, &authpb.VerifySiweRequest{
		AccountId: input.AccountID,
		Message:   input.Message,
		Signature: input.Signature,
	})
	if err != nil {
		return nil, err
	}

	// Set refresh token as httpOnly cookie for security
	if rw := middleware.GetResponseWriter(ctx); rw != nil {
		middleware.SetRefreshTokenCookie(rw, resp.GetRefreshToken())
	}

	return &schemas.AuthPayload{
		AccessToken:  resp.GetAccessToken(),
		RefreshToken: resp.GetRefreshToken(), // Also return in response body
		ExpiresAt:    resp.GetExpiresAt(),
		UserID:       resp.GetUserId(),
	}, nil
}

func (r *MutationResolver) RefreshSession(ctx context.Context) (*schemas.AuthPayload, error) {
	if r.server.authClient == nil {
		return nil, fmt.Errorf("auth service unavailable")
	}

	// Get HTTP request from context
	req := middleware.GetRequest(ctx)
	if req == nil {
		return nil, fmt.Errorf("request not available")
	}

	// Get refresh token from httpOnly cookie
	refreshToken := middleware.GetRefreshTokenFromCookie(req)
	if refreshToken == "" {
		return nil, fmt.Errorf("refresh token not found")
	}

	// Get client info for audit
	ip, userAgent := middleware.GetClientInfo(req)

	// Call auth service to refresh session
	resp, err := (*r.server.authClient.Client).RefreshSession(ctx, &authpb.RefreshSessionRequest{
		RefreshToken: refreshToken,
		UserAgent:    userAgent,
		IpAddress:    ip,
	})
	if err != nil {
		return nil, err
	}

	// Set new refresh token cookie
	if rw := middleware.GetResponseWriter(ctx); rw != nil {
		middleware.SetRefreshTokenCookie(rw, resp.GetRefreshToken())
	}

	return &schemas.AuthPayload{
		AccessToken:  resp.GetAccessToken(),
		RefreshToken: resp.GetRefreshToken(),
		ExpiresAt:    resp.GetExpiresAt(),
		UserID:       resp.GetUserId(),
	}, nil
}

func (r *MutationResolver) Logout(ctx context.Context) (bool, error) {
	if r.server.authClient == nil {
		return false, fmt.Errorf("auth service unavailable")
	}

	// Check if user is authenticated via Bearer token
	if user := middleware.GetCurrentUser(ctx); user != nil {
		// Use session ID from JWT token for more precise logout
		_, err := (*r.server.authClient.Client).RevokeSession(ctx, &authpb.RevokeSessionRequest{
			SessionId: user.SessionID,
		})
		if err != nil {
			// Log error but continue with cookie cleanup
		}
	}

	// Also try to revoke using refresh token from cookie as fallback
	req := middleware.GetRequest(ctx)
	if req != nil {
		refreshToken := middleware.GetRefreshTokenFromCookie(req)
		if refreshToken != "" {
			_, err := (*r.server.authClient.Client).RevokeSessionByRefreshToken(ctx, &authpb.RevokeSessionByRefreshTokenRequest{
				RefreshToken: refreshToken,
			})
			if err != nil {
				// Log error but still clear cookie - don't fail logout
			}
		}
	}

	// Clear refresh token cookie
	if rw := middleware.GetResponseWriter(ctx); rw != nil {
		middleware.ClearRefreshTokenCookie(rw)
	}

	return true, nil
}

// UpdateProfile is an example of a protected mutation that requires authentication
func (r *MutationResolver) UpdateProfile(ctx context.Context, displayName *string) (bool, error) {
	// This demonstrates how to use authentication in resolvers
	return middleware.WithAuth(ctx, func(ctx context.Context, user *middleware.CurrentUser) (bool, error) {
		// User is guaranteed to be authenticated here
		// You can access user.UserID and user.SessionID

		// TODO: Call user service to update profile
		// For now, just return success to demonstrate the pattern
		_ = displayName // Use the parameter

		fmt.Printf("User %s updating profile with display name: %v\n", user.UserID, displayName)
		return true, nil
	})
}

func (r *MutationResolver) BumpChainVersion(ctx context.Context, input schemas.BumpChainVersionInput) (*schemas.BumpChainVersionPayload, error) {
	if r.server.chainRegistryClient == nil || r.server.chainRegistryClient.Client == nil {
		return nil, fmt.Errorf("chain registry service unavailable")
	}
	reason := ""
	if input.Reason != nil {
		reason = *input.Reason
	}
	resp, err := (*r.server.chainRegistryClient.Client).BumpVersion(ctx, &chainregpb.BumpVersionRequest{ChainId: input.ChainID, Reason: reason})
	if err != nil {
		return nil, err
	}
	return &schemas.BumpChainVersionPayload{Ok: resp.GetOk(), NewVersion: resp.GetNewVersion()}, nil
}

// UploadMedia uploads multiple media files
func (r *MutationResolver) UploadMedia(ctx context.Context, files []*graphql.Upload) ([]*schemas.MediaAsset, error) {
	if r.server.mediaClient == nil || r.server.mediaClient.Client == nil {
		return nil, fmt.Errorf("media service unavailable")
	}

	var assets []*schemas.MediaAsset
	for _, file := range files {
		fileData, err := io.ReadAll(file.File)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", file.Filename, err)
		}

		req := &media.SingleUploadRequest{
			FileData: fileData,
			Filename: file.Filename,
			Mime:     file.ContentType,
			Kind:     media.MediaKind_IMAGE, // Default to image
		}

		client := *r.server.mediaClient.Client
		resp, err := client.UploadSingleFile(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to upload file %s: %w", file.Filename, err)
		}

		asset := utils.MapAssetToGraphQL(resp.Asset)
		assets = append(assets, asset)
	}

	return assets, nil
}
