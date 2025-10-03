package utils

import (
	"fmt"

	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql/schemas"
	chainregpb "github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry"
	mediaProto "github.com/quangdang46/NFT-Marketplace/shared/proto/media"
)

// Media mapping functions
func MapAssetToGraphQL(asset *mediaProto.Asset) *schemas.MediaAsset {
	if asset == nil {
		return nil
	}

	var width, height *int
	if asset.Width != nil {
		w := int(asset.Width.Value)
		width = &w
	}
	if asset.Height != nil {
		h := int(asset.Height.Value)
		height = &h
	}

	var ipfsCid *string
	if asset.IpfsCid != nil {
		ipfsCid = &asset.IpfsCid.Value
	}

	variants := make([]*schemas.MediaVariant, len(asset.Variants))
	for i, v := range asset.Variants {
		variants[i] = &schemas.MediaVariant{
			ID:     v.Id,
			CdnURL: v.CdnUrl,
			Width:  int(v.Width),
			Height: int(v.Height),
			Format: ConvertVariantFormatFromProto(v.Format),
		}
	}

	var url *schemas.MediaUrls
	if asset.GatewayUrl != nil {
		url = &schemas.MediaUrls{
			Gateway: &asset.GatewayUrl.Value,
		}
	}

	createdAt := asset.CreatedAt.AsTime()

	var bytes *string
	if asset.Bytes > 0 {
		b := fmt.Sprintf("%d", asset.Bytes)
		bytes = &b
	}

	return &schemas.MediaAsset{
		ID:        asset.Id,
		Kind:      ConvertMediaKindFromProto(asset.Kind),
		Mime:      asset.Mime,
		Bytes:     bytes,
		Width:     width,
		Height:    height,
		Sha256:    asset.Sha256,
		PinStatus: ConvertPinStatusFromProto(asset.PinStatus),
		IpfsCid:   ipfsCid,
		CreatedAt: createdAt.Format("2006-01-02T15:04:05Z07:00"),
		RefCount:  int(asset.RefCount),
		Variants:  variants,
		URL:       url,
	}
}

// Chain Registry mapping functions
func MapContract(c *chainregpb.Contract) *schemas.Contract {
	if c == nil {
		return nil
	}
	return &schemas.Contract{
		Name:        c.GetName(),
		Address:     c.GetAddress(),
		StartBlock:  IntPtrIfNonZero(c.GetStartBlock()),
		VerifiedAt:  StrPtrOrNil(c.GetVerifiedAt()),
		Standard:    ConvertContractStandardToPtr(c.GetStandard()),
		ImplAddress: StrPtrOrNil(c.GetImplAddress()),
		AbiSha256:   StrPtrOrNil(c.GetAbiSha256()),
	}
}

func MapRPCEndpoint(e *chainregpb.RpcEndpoint) *schemas.RPCEndpoint {
	if e == nil {
		return nil
	}
	return &schemas.RPCEndpoint{
		URL:       e.GetUrl(),
		Priority:  int(e.GetPriority()),
		Weight:    int(e.GetWeight()),
		AuthType:  ConvertRPCAuthTypeToStrPtr(e.GetAuthType()),
		RateLimit: IntPtrIfNonZero(e.GetRateLimit()),
		Active:    e.GetActive(),
	}
}

func MapGasPolicy(p *chainregpb.GasPolicy) *schemas.GasPolicy {
	if p == nil {
		return nil
	}
	return &schemas.GasPolicy{
		MaxFeeGwei:              p.GetMaxFeeGwei(),
		PriorityFeeGwei:         p.GetPriorityFeeGwei(),
		Multiplier:              p.GetMultiplier(),
		LastObservedBaseFeeGwei: FloatPtrOrNil(p.GetLastObservedBaseFeeGwei()),
		UpdatedAt:               StrPtrOrNil(p.GetUpdatedAt()),
	}
}

func MapChainParams(p *chainregpb.ChainParams) *schemas.ChainParams {
	if p == nil {
		return nil
	}
	return &schemas.ChainParams{
		RequiredConfirmations: int(p.GetRequiredConfirmations()),
		ReorgDepth:            int(p.GetReorgDepth()),
		BlockTimeMs:           IntPtrIfNonZero(p.GetBlockTimeMs()),
	}
}
