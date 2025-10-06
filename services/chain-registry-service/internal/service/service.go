package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/chain-registry-service/internal/domain"
	"google.golang.org/grpc/metadata"
)

type Service struct {
	repo domain.ChainRegistryRepository
}

func New(repo domain.ChainRegistryRepository) domain.ChainRegistryService {
	return &Service{repo: repo}
}

func (s *Service) GetContracts(ctx context.Context, chainID domain.ChainID) (*domain.ChainContracts, error) {
	if err := ValidateGetContractsRequest(chainID); err != nil {
		return nil, err
	}

	chainContracts, err := s.repo.GetContracts(ctx, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get contracts from repository: %w", err)
	}

	s.audit(ctx, "GetContracts", map[string]any{
		"chain_id":  chainID,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})

	return chainContracts, nil
}

func (s *Service) GetGasPolicy(ctx context.Context, chainID domain.ChainID) (*domain.ChainGasPolicy, error) {
	if err := ValidateGetGasPolicyRequest(chainID); err != nil {
		return nil, err
	}

	chainGasPolicy, err := s.repo.GetGasPolicy(ctx, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas policy from repository: %w", err)
	}

	s.audit(ctx, "GetGasPolicy", map[string]any{
		"chain_id":  chainID,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})

	return chainGasPolicy, nil
}

func (s *Service) GetRpcEndpoints(ctx context.Context, chainID domain.ChainID) (*domain.ChainRpcEndpoints, error) {
	if err := ValidateGetRpcEndpointsRequest(chainID); err != nil {
		return nil, err
	}

	chainRpcEndpoints, err := s.repo.GetRpcEndpoints(ctx, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get RPC endpoints from repository: %w", err)
	}

	s.audit(ctx, "GetRpcEndpoints", map[string]any{
		"chain_id":  chainID,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})

	return chainRpcEndpoints, nil
}

func (s *Service) BumpVersion(ctx context.Context, chainID domain.ChainID, reason string) (ok bool, newVersion string, err error) {
	if err := ValidateBumpVersionRequest(chainID, reason); err != nil {
		return false, "", err
	}

	newVersion, err = s.repo.BumpVersion(ctx, chainID, reason)
	if err != nil {
		return false, "", fmt.Errorf("failed to bump version in repository: %w", err)
	}

	s.audit(ctx, "BumpVersion", map[string]any{
		"chain_id":  chainID,
		"reason":    reason,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})

	return true, newVersion, nil
}

func (s *Service) GetContractMeta(ctx context.Context, chainID domain.ChainID, address domain.Address) (*domain.ContractMeta, error) {
	if err := ValidateGetContractMetaRequest(chainID, address); err != nil {
		return nil, err
	}

	contractMeta, err := s.repo.GetContractMeta(ctx, chainID, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract meta from repository: %w", err)
	}

	s.audit(ctx, "GetContractMeta", map[string]any{
		"chain_id":  chainID,
		"address":   address,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})

	return contractMeta, nil
}

func (s *Service) GetAbiBlob(ctx context.Context, sha domain.Sha256) (abiJSON []byte, etag string, err error) {
	if err := ValidateGetAbiBlobRequest(sha); err != nil {
		return nil, "", err
	}

	abiJSON, etag, err = s.repo.GetAbiBlob(ctx, sha)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get ABI blob from repository: %w", err)
	}

	s.audit(ctx, "GetAbiBlob", map[string]any{
		"sha256":    sha,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})

	return abiJSON, etag, nil
}

func (s *Service) ResolveProxy(ctx context.Context, chainID domain.ChainID, address domain.Address) (implAddress domain.Address, abiSha256 domain.Sha256, err error) {
	if err := ValidateResolveProxyRequest(chainID, address); err != nil {
		return "", "", err
	}

	implAddress, abiSha256, err = s.repo.ResolveProxy(ctx, chainID, address)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve proxy from repository: %w", err)
	}

	s.audit(ctx, "ResolveProxy", map[string]any{
		"chain_id":  chainID,
		"address":   address,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	})

	return implAddress, abiSha256, nil
}

func (s *Service) GetAbiByAddress(ctx context.Context, chainID domain.ChainID, address domain.Address) (abiJSON []byte, etag string, err error) {
	if err := ValidateGetAbiByAddressRequest(chainID, address); err != nil {
		return nil, "", err
	}

	meta, err := s.repo.GetContractMeta(ctx, chainID, address)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get contract meta: %w", err)
	}
	if meta.Contract.AbiSHA256 == nil || *meta.Contract.AbiSHA256 == "" {
		return nil, "", fmt.Errorf("abi_sha256 not available for %s on %s", address, chainID)
	}
	abiJSON, etag, err = s.repo.GetAbiBlob(ctx, domain.Sha256(*meta.Contract.AbiSHA256))
	if err == nil {
		s.audit(ctx, "GetAbiByAddress", map[string]any{
			"chain_id":  chainID,
			"address":   address,
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	return abiJSON, etag, err
}

// audit emits structured audit logs with optional session context from gRPC metadata
func (s *Service) audit(ctx context.Context, method string, fields map[string]any) {
	var sessionID string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		vals := md.Get("x-auth-session-id")
		if len(vals) > 0 {
			sessionID = vals[0]
		}
	}
	line := fmt.Sprintf("audit|event=chain_registry_call|method=%s", method)
	if sessionID != "" {
		line += fmt.Sprintf("|session_id=%s", sessionID)
	}
	for k, v := range fields {
		line += fmt.Sprintf("|%s=%v", k, v)
	}
	log.Println(line)
}
