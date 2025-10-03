package graphql_resolver

import (
	"context"
	"fmt"
	"io"

	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql/schemas"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/utils"
	"github.com/quangdang46/NFT-Marketplace/shared/proto/media"
)

// Query resolvers
func (r *QueryResolver) MediaAsset(ctx context.Context, id string) (*schemas.MediaAsset, error) {
	resp, err := (*r.server.mediaClient.Client).GetAsset(ctx, &media.GetAssetRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}

	return utils.MapAssetToGraphQL(resp.Asset), nil
}

func (r *QueryResolver) MediaAssetByCid(ctx context.Context, cid string) (*schemas.MediaAsset, error) {
	resp, err := (*r.server.mediaClient.Client).GetAssetByCid(ctx, &media.GetAssetByCidRequest{
		Cid: cid,
	})
	if err != nil {
		return nil, err
	}

	return utils.MapAssetToGraphQL(resp.Asset), nil
}

// Mutation resolvers
func (r *MutationResolver) UploadSingleFile(ctx context.Context, input schemas.UploadSingleFileInput) (*schemas.UploadSingleFilePayload, error) {
	fileData, err := io.ReadAll(input.File.File)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	req := &media.SingleUploadRequest{
		FileData: fileData,
		Filename: input.File.Filename,
		Mime:     input.File.ContentType,
		Kind:     utils.ConvertMediaKindToProto(input.Kind),
	}

	client := *r.server.mediaClient.Client
	resp, err := client.UploadSingleFile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	asset := utils.MapAssetToGraphQL(resp.Asset)

	var url *schemas.MediaUrls
	var cid *string

	if resp.Asset.GatewayUrl != nil {
		url = &schemas.MediaUrls{
			Gateway: &resp.Asset.GatewayUrl.Value,
		}
	}

	if resp.Asset.IpfsCid != nil {
		cid = &resp.Asset.IpfsCid.Value
	}

	return &schemas.UploadSingleFilePayload{
		Asset:        asset,
		Deduplicated: resp.Deduplicated,
		URL:          url,
		Cid:          cid,
	}, nil
}
