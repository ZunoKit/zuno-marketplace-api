package utils 

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/quangdang46/NFT-Marketplace/services/chain-registry-service/internal/domain"
)

// Helper function to parse CAIP-2 chain ID to numeric ID
func ParseChainID(chainID domain.ChainID) (int, error) {
	parts := strings.Split(string(chainID), ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid chain ID format: %s", chainID)
	}

	numericID, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid chain numeric ID: %s", parts[1])
	}

	return numericID, nil
}

// Helper function to convert database auth_type to domain enum
func DbAuthTypeToDomain(authType sql.NullString) domain.RpcAuthType {
	if !authType.Valid {
		return domain.RpcAuthNone
	}

	switch authType.String {
	case "NONE":
		return domain.RpcAuthNone
	case "KEY":
		return domain.RpcAuthKey
	case "BASIC":
		return domain.RpcAuthBasic
	case "BEARER":
		return domain.RpcAuthBearer
	default:
		return domain.RpcAuthNone
	}
}

// Helper function to convert database standard to domain enum
func DbStandardToDomain(standard sql.NullString) domain.ContractStandard {
	if !standard.Valid {
		return domain.StdCustom
	}

	switch standard.String {
	case "ERC721":
		return domain.StdERC721
	case "ERC1155":
		return domain.StdERC1155
	case "PROXY":
		return domain.StdProxy
	case "DIAMOND":
		return domain.StdDiamond
	default:
		return domain.StdCustom
	}
}