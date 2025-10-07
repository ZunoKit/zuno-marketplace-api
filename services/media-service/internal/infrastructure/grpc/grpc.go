package grpc_handler

import (
	"bytes"
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/utils"
	mediaProto "github.com/quangdang46/NFT-Marketplace/shared/proto/media"
)

type gRPCHandler struct {
	mediaProto.UnimplementedMediaServiceServer
	mediaService domain.MediaService
}

func NewgRPCHandler(mediaService domain.MediaService) *gRPCHandler {
	handler := &gRPCHandler{
		mediaService: mediaService,
	}
	return handler
}

func (g *gRPCHandler) UploadSingleFile(ctx context.Context, req *mediaProto.SingleUploadRequest) (*mediaProto.UploadAndPinResponse, error) {
	if req.FileData == nil || len(req.FileData) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "file data cannot be empty")
	}
	if req.Filename == "" {
		return nil, status.Errorf(codes.InvalidArgument, "filename is required")
	}
	if req.Mime == "" {
		return nil, status.Errorf(codes.InvalidArgument, "mime type is required")
	}

	// Convert protobuf meta to domain
	domainMeta := domain.UploadMeta{
		Filename: req.Filename,
		Mime:     req.Mime,
		Kind:     utils.ProtoToDomainMediaKind(req.Kind),
		OwnerID:  req.OwnerId,
	}

	if req.Width != nil {
		w := req.Width.Value
		domainMeta.Width = &w
	}

	if req.Height != nil {
		h := req.Height.Value
		domainMeta.Height = &h
	}

	// Call service with file data
	asset, dedup, err := g.mediaService.UploadAndPin(ctx, domainMeta, bytes.NewReader(req.FileData), int64(len(req.FileData)))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to upload and pin: %v", err)
	}

	// Convert response
	response := &mediaProto.UploadAndPinResponse{
		Asset:        utils.DomainToProtoAsset(asset),
		Deduplicated: dedup,
	}

	return response, nil
}

func (g *gRPCHandler) GetAsset(ctx context.Context, req *mediaProto.GetAssetRequest) (*mediaProto.GetAssetResponse, error) {
	asset, err := g.mediaService.GetAsset(ctx, req.Id)
	if err != nil {
		if err == domain.ErrAssetNotFound {
			return nil, status.Errorf(codes.NotFound, "asset not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get asset: %v", err)
	}

	return &mediaProto.GetAssetResponse{
		Asset: utils.DomainToProtoAsset(asset),
	}, nil
}

func (g *gRPCHandler) GetAssetByCid(ctx context.Context, req *mediaProto.GetAssetByCidRequest) (*mediaProto.GetAssetResponse, error) {
	asset, err := g.mediaService.GetAssetByCID(ctx, req.Cid)
	if err != nil {
		if err == domain.ErrAssetNotFound {
			return nil, status.Errorf(codes.NotFound, "asset not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get asset by CID: %v", err)
	}

	return &mediaProto.GetAssetResponse{
		Asset: utils.DomainToProtoAsset(asset),
	}, nil
}
