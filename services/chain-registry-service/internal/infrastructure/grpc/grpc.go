package grpc_handler

import (
	"context"

	"github.com/quangdang46/NFT-Marketplace/services/chain-registry-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/chain-registry-service/internal/utils"
	chainpb "github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCHandler struct {
	chainpb.UnimplementedChainRegistryServiceServer
	svc domain.ChainRegistryService
}

func NewGRPCHandler(svc domain.ChainRegistryService) *GRPCHandler {
	return &GRPCHandler{svc: svc}
}

func (h *GRPCHandler) GetContracts(ctx context.Context, req *chainpb.GetContractsRequest) (*chainpb.GetContractsResponse, error) {
	if req.ChainId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "chain_id is required")
	}

	chainContracts, err := h.svc.GetContracts(ctx, domain.ChainID(req.ChainId))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get contracts: %v", err)
	}

	contracts := make([]*chainpb.Contract, len(chainContracts.Contracts))
	for i, contract := range chainContracts.Contracts {
		contracts[i] = utils.DomainToProtoContract(contract)
	}

	return &chainpb.GetContractsResponse{
		ChainId:         string(chainContracts.ChainID),
		ChainNumeric:    chainContracts.ChainNumeric,
		Contracts:       contracts,
		Params:          utils.DomainToProtoChainParams(chainContracts.Params),
		RegistryVersion: chainContracts.RegistryVersion,
		NativeSymbol:    chainContracts.NativeSymbol,
	}, nil
}

func (h *GRPCHandler) GetGasPolicy(ctx context.Context, req *chainpb.GetGasPolicyRequest) (*chainpb.GetGasPolicyResponse, error) {
	if req.ChainId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "chain_id is required")
	}

	chainGasPolicy, err := h.svc.GetGasPolicy(ctx, domain.ChainID(req.ChainId))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get gas policy: %v", err)
	}

	return &chainpb.GetGasPolicyResponse{
		ChainId:         string(chainGasPolicy.ChainID),
		Policy:          utils.DomainToProtoGasPolicy(chainGasPolicy.Policy),
		RegistryVersion: chainGasPolicy.RegistryVersion,
	}, nil
}

func (h *GRPCHandler) GetRpcEndpoints(ctx context.Context, req *chainpb.GetRpcEndpointsRequest) (*chainpb.GetRpcEndpointsResponse, error) {
	if req.ChainId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "chain_id is required")
	}

	chainRpcEndpoints, err := h.svc.GetRpcEndpoints(ctx, domain.ChainID(req.ChainId))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get RPC endpoints: %v", err)
	}

	endpoints := make([]*chainpb.RpcEndpoint, len(chainRpcEndpoints.Endpoints))
	for i, endpoint := range chainRpcEndpoints.Endpoints {
		endpoints[i] = utils.DomainToProtoRpcEndpoint(endpoint)
	}

	return &chainpb.GetRpcEndpointsResponse{
		ChainId:         string(chainRpcEndpoints.ChainID),
		Endpoints:       endpoints,
		RegistryVersion: chainRpcEndpoints.RegistryVersion,
	}, nil
}

func (h *GRPCHandler) GetContractMeta(ctx context.Context, req *chainpb.GetContractMetaRequest) (*chainpb.GetContractMetaResponse, error) {
	if req.ChainId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "chain_id is required")
	}
	if req.Address == "" {
		return nil, status.Errorf(codes.InvalidArgument, "address is required")
	}

	contractMeta, err := h.svc.GetContractMeta(ctx, domain.ChainID(req.ChainId), domain.Address(req.Address))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get contract meta: %v", err)
	}

	return &chainpb.GetContractMetaResponse{
		ChainId:         string(contractMeta.ChainID),
		Contract:        utils.DomainToProtoContract(contractMeta.Contract),
		RegistryVersion: contractMeta.RegistryVersion,
	}, nil
}

func (h *GRPCHandler) GetAbiBlob(ctx context.Context, req *chainpb.GetAbiBlobRequest) (*chainpb.GetAbiBlobResponse, error) {
	if req.AbiSha256 == "" {
		return nil, status.Errorf(codes.InvalidArgument, "abi_sha256 is required")
	}

	abiJSON, etag, err := h.svc.GetAbiBlob(ctx, domain.Sha256(req.AbiSha256))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get ABI blob: %v", err)
	}

	return &chainpb.GetAbiBlobResponse{
		AbiJson: string(abiJSON),
		Etag:    etag,
	}, nil
}

func (h *GRPCHandler) GetAbiByAddress(ctx context.Context, req *chainpb.GetAbiByAddressRequest) (*chainpb.GetAbiBlobResponse, error) {
	if req.ChainId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "chain_id is required")
	}
	if req.Address == "" {
		return nil, status.Errorf(codes.InvalidArgument, "address is required")
	}

	abiJSON, etag, err := h.svc.GetAbiByAddress(ctx, domain.ChainID(req.ChainId), domain.Address(req.Address))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get ABI by address: %v", err)
	}

	return &chainpb.GetAbiBlobResponse{AbiJson: string(abiJSON), Etag: etag}, nil
}

func (h *GRPCHandler) ResolveProxy(ctx context.Context, req *chainpb.ResolveProxyRequest) (*chainpb.ResolveProxyResponse, error) {
	if req.ChainId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "chain_id is required")
	}
	if req.Address == "" {
		return nil, status.Errorf(codes.InvalidArgument, "address is required")
	}

	implAddress, abiSha256, err := h.svc.ResolveProxy(ctx, domain.ChainID(req.ChainId), domain.Address(req.Address))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to resolve proxy: %v", err)
	}

	return &chainpb.ResolveProxyResponse{
		ChainId:         req.ChainId,
		ProxyAddress:    req.Address,
		ImplAddress:     string(implAddress),
		AbiSha256:       string(abiSha256),
		RegistryVersion: "1.0.0",
	}, nil
}

func (h *GRPCHandler) BumpVersion(ctx context.Context, req *chainpb.BumpVersionRequest) (*chainpb.BumpVersionResponse, error) {
	if req.ChainId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "chain_id is required")
	}
	if req.Reason == "" {
		return nil, status.Errorf(codes.InvalidArgument, "reason is required")
	}

	ok, newVersion, err := h.svc.BumpVersion(ctx, domain.ChainID(req.ChainId), req.Reason)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to bump version: %v", err)
	}

	return &chainpb.BumpVersionResponse{
		Ok:         ok,
		NewVersion: newVersion,
	}, nil
}
