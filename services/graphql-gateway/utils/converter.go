package utils

import (
	"strconv"

	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql/schemas"
	chainregpb "github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry"
	mediaProto "github.com/quangdang46/NFT-Marketplace/shared/proto/media"
)

// Media conversion functions
func ConvertMediaKindToProto(kind schemas.MediaKind) mediaProto.MediaKind {
	switch kind {
	case schemas.MediaKindImage:
		return mediaProto.MediaKind_IMAGE
	case schemas.MediaKindVideo:
		return mediaProto.MediaKind_VIDEO
	case schemas.MediaKindAudio:
		return mediaProto.MediaKind_AUDIO
	case schemas.MediaKindOther:
		return mediaProto.MediaKind_OTHER
	default:
		return mediaProto.MediaKind_MEDIA_KIND_UNSPECIFIED
	}
}

func ConvertMediaKindFromProto(kind mediaProto.MediaKind) schemas.MediaKind {
	switch kind {
	case mediaProto.MediaKind_IMAGE:
		return schemas.MediaKindImage
	case mediaProto.MediaKind_VIDEO:
		return schemas.MediaKindVideo
	case mediaProto.MediaKind_AUDIO:
		return schemas.MediaKindAudio
	case mediaProto.MediaKind_OTHER:
		return schemas.MediaKindOther
	default:
		return schemas.MediaKindOther
	}
}

func ConvertVariantFormatFromProto(format mediaProto.VariantFormat) schemas.VariantFormat {
	switch format {
	case mediaProto.VariantFormat_JPG:
		return schemas.VariantFormatJpg
	case mediaProto.VariantFormat_WEBP:
		return schemas.VariantFormatWebp
	case mediaProto.VariantFormat_PNG:
		return schemas.VariantFormatPng
	case mediaProto.VariantFormat_MP4:
		return schemas.VariantFormatMp4
	case mediaProto.VariantFormat_GIF:
		return schemas.VariantFormatGif
	default:
		return schemas.VariantFormatJpg
	}
}

func ConvertPinStatusFromProto(status mediaProto.PinStatus) schemas.PinStatus {
	switch status {
	case mediaProto.PinStatus_PENDING:
		return schemas.PinStatusPending
	case mediaProto.PinStatus_PINNING:
		return schemas.PinStatusPinning
	case mediaProto.PinStatus_PINNED:
		return schemas.PinStatusPinned
	case mediaProto.PinStatus_FAILED:
		return schemas.PinStatusFailed
	default:
		return schemas.PinStatusPending
	}
}

// Chain Registry conversion functions
func ConvertContractStandardToPtr(std chainregpb.ContractStandard) *schemas.ContractStandard {
	var v schemas.ContractStandard
	switch std {
	case chainregpb.ContractStandard_STD_ERC721:
		v = schemas.ContractStandardErc721
	case chainregpb.ContractStandard_STD_ERC1155:
		v = schemas.ContractStandardErc1155
	case chainregpb.ContractStandard_STD_PROXY:
		v = schemas.ContractStandardProxy
	case chainregpb.ContractStandard_STD_DIAMOND:
		v = schemas.ContractStandardDiamond
	case chainregpb.ContractStandard_STD_CUSTOM:
		v = schemas.ContractStandardCustom
	default:
		return nil
	}
	return &v
}

func ConvertRPCAuthTypeToStrPtr(a chainregpb.RpcAuthType) *string {
	var s string
	switch a {
	case chainregpb.RpcAuthType_RPC_AUTH_KEY:
		s = "KEY"
	case chainregpb.RpcAuthType_RPC_AUTH_BASIC:
		s = "BASIC"
	case chainregpb.RpcAuthType_RPC_AUTH_BEARER:
		s = "BEARER"
	case chainregpb.RpcAuthType_RPC_AUTH_NONE:
		fallthrough
	default:
		return nil
	}
	return &s
}

// Utility functions
func IntPtrIfNonZero[T ~int32 | ~uint32 | ~int64 | ~uint64](v T) *int {
	if v == 0 {
		return nil
	}
	x := int(v)
	return &x
}

func FloatPtrOrNil(v float64) *float64 {
	if v == 0 {
		return nil
	}
	x := v
	return &x
}

func StrPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	x := s
	return &x
}

func PtrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func ParseUint64(s *string) uint64 {
	if s == nil || *s == "" {
		return 0
	}
	v, err := strconv.ParseUint(*s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
