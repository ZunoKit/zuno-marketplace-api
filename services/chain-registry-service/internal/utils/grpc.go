package utils

import (
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/chain-registry-service/internal/domain"
	chainpb "github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry"
)

// DomainToProtoContractStandard converts domain contract standard to protobuf
func DomainToProtoContractStandard(std domain.ContractStandard) chainpb.ContractStandard {
	switch std {
	case domain.StdCustom:
		return chainpb.ContractStandard_STD_CUSTOM
	case domain.StdERC721:
		return chainpb.ContractStandard_STD_ERC721
	case domain.StdERC1155:
		return chainpb.ContractStandard_STD_ERC1155
	case domain.StdProxy:
		return chainpb.ContractStandard_STD_PROXY
	case domain.StdDiamond:
		return chainpb.ContractStandard_STD_DIAMOND
	default:
		return chainpb.ContractStandard_STD_CUSTOM
	}
}

// DomainToProtoRpcAuthType converts domain RPC auth type to protobuf
func DomainToProtoRpcAuthType(authType domain.RpcAuthType) chainpb.RpcAuthType {
	switch authType {
	case domain.RpcAuthNone:
		return chainpb.RpcAuthType_RPC_AUTH_NONE
	case domain.RpcAuthKey:
		return chainpb.RpcAuthType_RPC_AUTH_KEY
	case domain.RpcAuthBasic:
		return chainpb.RpcAuthType_RPC_AUTH_BASIC
	case domain.RpcAuthBearer:
		return chainpb.RpcAuthType_RPC_AUTH_BEARER
	default:
		return chainpb.RpcAuthType_RPC_AUTH_NONE
	}
}

// DomainToProtoContract converts domain contract to protobuf
func DomainToProtoContract(contract domain.Contract) *chainpb.Contract {
	protoContract := &chainpb.Contract{
		Name:       contract.Name,
		Address:    contract.Address,
		StartBlock: int32(contract.StartBlock),
		Standard:   DomainToProtoContractStandard(contract.Standard),
	}

	if contract.VerifiedAt != nil {
		protoContract.VerifiedAt = contract.VerifiedAt.Format(time.RFC3339)
	}
	if contract.ImplAddress != nil {
		protoContract.ImplAddress = *contract.ImplAddress
	}
	if contract.AbiSHA256 != nil {
		protoContract.AbiSha256 = *contract.AbiSHA256
	}

	return protoContract
}

// DomainToProtoGasPolicy converts domain gas policy to protobuf
func DomainToProtoGasPolicy(policy domain.GasPolicy) *chainpb.GasPolicy {
	return &chainpb.GasPolicy{
		MaxFeeGwei:              policy.MaxFeeGwei,
		PriorityFeeGwei:         policy.PriorityFeeGwei,
		Multiplier:              policy.Multiplier,
		LastObservedBaseFeeGwei: policy.LastObservedBaseFeeGwei,
		UpdatedAt:               policy.UpdatedAt.Format(time.RFC3339),
	}
}

// DomainToProtoRpcEndpoint converts domain RPC endpoint to protobuf
func DomainToProtoRpcEndpoint(endpoint domain.RpcEndpoint) *chainpb.RpcEndpoint {
	protoEndpoint := &chainpb.RpcEndpoint{
		Url:      endpoint.URL,
		Priority: endpoint.Priority,
		Weight:   endpoint.Weight,
		AuthType: DomainToProtoRpcAuthType(endpoint.AuthType),
		Active:   endpoint.Active,
	}

	if endpoint.RateLimit != nil {
		protoEndpoint.RateLimit = *endpoint.RateLimit
	}

	return protoEndpoint
}

// DomainToProtoChainParams converts domain chain params to protobuf
func DomainToProtoChainParams(params domain.ChainParams) *chainpb.ChainParams {
	return &chainpb.ChainParams{
		RequiredConfirmations: params.RequiredConfirmations,
		ReorgDepth:            params.ReorgDepth,
		BlockTimeMs:           params.BlockTimeMs,
	}
}
