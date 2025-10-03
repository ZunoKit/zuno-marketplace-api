package utils

import (
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/domain"
	mediaProto "github.com/quangdang46/NFT-Marketplace/shared/proto/media"
)

// Media conversion functions
func ProtoToDomainMediaKind(kind mediaProto.MediaKind) string {
	switch kind {
	case mediaProto.MediaKind_IMAGE:
		return "IMAGE"
	case mediaProto.MediaKind_VIDEO:
		return "VIDEO"
	case mediaProto.MediaKind_AUDIO:
		return "AUDIO"
	default:
		return "OTHER"
	}
}

func DomainToProtoMediaKind(kind string) mediaProto.MediaKind {
	switch kind {
	case "IMAGE":
		return mediaProto.MediaKind_IMAGE
	case "VIDEO":
		return mediaProto.MediaKind_VIDEO
	case "AUDIO":
		return mediaProto.MediaKind_AUDIO
	default:
		return mediaProto.MediaKind_OTHER
	}
}

func DomainToProtoPinStatus(status domain.PinStatus) mediaProto.PinStatus {
	switch status {
	case domain.PinPending:
		return mediaProto.PinStatus_PENDING
	case domain.PinPinning:
		return mediaProto.PinStatus_PINNING
	case domain.PinPinned:
		return mediaProto.PinStatus_PINNED
	case domain.PinFailed:
		return mediaProto.PinStatus_FAILED
	default:
		return mediaProto.PinStatus_PIN_STATUS_UNSPECIFIED
	}
}

// Asset mapping functions
func DomainToProtoAsset(asset *domain.AssetDoc) *mediaProto.Asset {
	protoAsset := &mediaProto.Asset{
		Id:        asset.ID,
		Kind:      DomainToProtoMediaKind(asset.Kind),
		Mime:      asset.Mime,
		Bytes:     uint64(asset.Bytes),
		S3Key:     asset.S3Key,
		Sha256:    asset.SHA256,
		PinStatus: DomainToProtoPinStatus(domain.PinStatus(asset.PinStatus)),
		RefCount:  asset.RefCount,
		CreatedAt: timestamppb.New(asset.CreatedAt),
	}

	if asset.Width != nil {
		protoAsset.Width = wrapperspb.UInt32(*asset.Width)
	}

	if asset.Height != nil {
		protoAsset.Height = wrapperspb.UInt32(*asset.Height)
	}

	if asset.IPFSCID != nil {
		protoAsset.IpfsCid = wrapperspb.String(*asset.IPFSCID)
	}

	if asset.GatewayURL != nil {
		protoAsset.GatewayUrl = wrapperspb.String(*asset.GatewayURL)
	}

	// Convert variants
	protoAsset.Variants = make([]*mediaProto.MediaVariant, len(asset.Variants))
	for i, v := range asset.Variants {
		protoAsset.Variants[i] = &mediaProto.MediaVariant{
			Id:     v.ID,
			CdnUrl: v.CDNURL,
			Width:  v.Width,
			Height: v.Height,
			Format: mediaProto.VariantFormat(mediaProto.VariantFormat_value[v.Format]),
		}
	}

	return protoAsset
}
