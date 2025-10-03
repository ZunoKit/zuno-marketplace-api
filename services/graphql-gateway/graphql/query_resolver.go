package graphql_resolver

import (
	"context"

	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql/schemas"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/middleware"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/utils"
	authpb "github.com/quangdang46/NFT-Marketplace/shared/proto/auth"
	chainregpb "github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry"
)

type QueryResolver struct {
	server *Resolver
}

func (r *QueryResolver) Health(ctx context.Context) (string, error) {
	return "ok", nil
}

func (r *QueryResolver) Me(ctx context.Context) (*schemas.User, error) {
	// First, check if user is already authenticated via Bearer token
	if user := middleware.GetCurrentUser(ctx); user != nil {
		return &schemas.User{
			ID: user.UserID,
		}, nil
	}

	// If no Bearer token, try silent refresh using cookie
	if r.server.authClient == nil {
		return nil, nil // Return null if auth service not available
	}

	// Get HTTP request from context
	req := middleware.GetRequest(ctx)
	if req == nil {
		return nil, nil // Return null if request not available
	}

	// Try to get refresh token from cookie
	refreshToken := middleware.GetRefreshTokenFromCookie(req)
	if refreshToken == "" {
		return nil, nil // Return null if no refresh token (not logged in)
	}

	// Get client info for audit
	ip, userAgent := middleware.GetClientInfo(req)

	// Try to refresh session silently
	resp, err := (*r.server.authClient.Client).RefreshSession(ctx, &authpb.RefreshSessionRequest{
		RefreshToken: refreshToken,
		UserAgent:    userAgent,
		IpAddress:    ip,
	})
	if err != nil {
		// If refresh fails, clear the cookie and return null
		if rw := middleware.GetResponseWriter(ctx); rw != nil {
			middleware.ClearRefreshTokenCookie(rw)
		}
		return nil, nil
	}

	// Set new refresh token cookie
	if rw := middleware.GetResponseWriter(ctx); rw != nil {
		middleware.SetRefreshTokenCookie(rw, resp.GetRefreshToken())
	}

	// Return user info with new access token available for future requests
	return &schemas.User{
		ID: resp.GetUserId(),
	}, nil
}

func (r *QueryResolver) ChainContracts(ctx context.Context, chainID string) (*schemas.ChainContracts, error) {
	if r.server.chainRegistryClient == nil || r.server.chainRegistryClient.Client == nil {
		return nil, nil
	}
	resp, err := (*r.server.chainRegistryClient.Client).GetContracts(ctx, &chainregpb.GetContractsRequest{ChainId: chainID})
	if err != nil {
		return nil, err
	}
	contracts := make([]*schemas.Contract, 0, len(resp.GetContracts()))
	for _, c := range resp.GetContracts() {
		contracts = append(contracts, utils.MapContract(c))
	}
	chainParams := utils.MapChainParams(resp.GetParams())
	return &schemas.ChainContracts{
		ChainID:         resp.GetChainId(),
		ChainNumeric:    int(resp.GetChainNumeric()),
		NativeSymbol:    utils.StrPtrOrNil(resp.GetNativeSymbol()),
		Contracts:       contracts,
		Params:          chainParams,
		RegistryVersion: resp.GetRegistryVersion(),
	}, nil
}

func (r *QueryResolver) ChainGasPolicy(ctx context.Context, chainID string) (*schemas.ChainGasPolicy, error) {
	if r.server.chainRegistryClient == nil || r.server.chainRegistryClient.Client == nil {
		return nil, nil
	}
	resp, err := (*r.server.chainRegistryClient.Client).GetGasPolicy(ctx, &chainregpb.GetGasPolicyRequest{ChainId: chainID})
	if err != nil {
		return nil, err
	}
	policy := utils.MapGasPolicy(resp.GetPolicy())
	return &schemas.ChainGasPolicy{
		ChainID:         resp.GetChainId(),
		Policy:          policy,
		RegistryVersion: resp.GetRegistryVersion(),
	}, nil
}

func (r *QueryResolver) ChainRPCEndpoints(ctx context.Context, chainID string) (*schemas.ChainRPCEndpoints, error) {
	if r.server.chainRegistryClient == nil || r.server.chainRegistryClient.Client == nil {
		return nil, nil
	}
	resp, err := (*r.server.chainRegistryClient.Client).GetRpcEndpoints(ctx, &chainregpb.GetRpcEndpointsRequest{ChainId: chainID})
	if err != nil {
		return nil, err
	}
	eps := make([]*schemas.RPCEndpoint, 0, len(resp.GetEndpoints()))
	for _, e := range resp.GetEndpoints() {
		eps = append(eps, utils.MapRPCEndpoint(e))
	}
	return &schemas.ChainRPCEndpoints{
		ChainID:         resp.GetChainId(),
		Endpoints:       eps,
		RegistryVersion: resp.GetRegistryVersion(),
	}, nil
}

func (r *QueryResolver) ContractMeta(ctx context.Context, chainID string, address string) (*schemas.ContractMeta, error) {
	if r.server.chainRegistryClient == nil || r.server.chainRegistryClient.Client == nil {
		return nil, nil
	}
	resp, err := (*r.server.chainRegistryClient.Client).GetContractMeta(ctx, &chainregpb.GetContractMetaRequest{ChainId: chainID, Address: address})
	if err != nil {
		return nil, err
	}
	return &schemas.ContractMeta{
		ChainID:         resp.GetChainId(),
		Contract:        utils.MapContract(resp.GetContract()),
		RegistryVersion: resp.GetRegistryVersion(),
	}, nil
}
