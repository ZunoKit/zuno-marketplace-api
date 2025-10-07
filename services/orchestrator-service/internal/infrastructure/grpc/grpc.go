package grpc_handler

import (
	"context"
	"fmt"

	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/utils"
	orchestratorpb "github.com/quangdang46/NFT-Marketplace/shared/proto/orchestrator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCHandler struct {
	orchestratorpb.UnimplementedOrchestratorServiceServer
	svc domain.OrchestratorService
}

func NewGRPCHandler(svc domain.OrchestratorService) *GRPCHandler {
	return &GRPCHandler{svc: svc}
}

func (h *GRPCHandler) PrepareCreateCollection(ctx context.Context, req *orchestratorpb.PrepareCreateCollectionRequest) (*orchestratorpb.PrepareCreateCollectionResponse, error) {
	input := utils.ConvertCreateCollectionRequest(req)

	result, err := h.svc.PrepareCreateCollection(ctx, input)
	if err != nil {
		return nil, h.handleError(err)
	}

	return utils.ConvertCreateCollectionResponse(result), nil
}

func (h *GRPCHandler) PrepareMint(ctx context.Context, req *orchestratorpb.PrepareMintRequest) (*orchestratorpb.PrepareMintResponse, error) {
	input := utils.ConvertMintRequest(req)

	result, err := h.svc.PrepareMint(ctx, input)
	if err != nil {
		return nil, h.handleError(err)
	}

	return utils.ConvertMintResponse(result), nil
}

func (h *GRPCHandler) TrackTx(ctx context.Context, req *orchestratorpb.TrackTxRequest) (*orchestratorpb.TrackTxResponse, error) {
	input := utils.ConvertTrackTxRequest(req)

	ok, err := h.svc.TrackTx(ctx, input)
	if err != nil {
		return nil, h.handleError(err)
	}

	return utils.ConvertTrackTxResponse(ok), nil
}

func (h *GRPCHandler) GetIntentStatus(ctx context.Context, req *orchestratorpb.GetIntentStatusRequest) (*orchestratorpb.GetIntentStatusResponse, error) {
	result, err := h.svc.GetIntentStatus(ctx, req.IntentId)
	if err != nil {
		return nil, h.handleError(err)
	}

	return utils.ConvertIntentStatusResponse(result), nil
}

func (h *GRPCHandler) handleError(err error) error {
	switch err {
	case domain.ErrNotFound:
		return status.Error(codes.NotFound, "intent not found")
	case domain.ErrInvalidInput:
		return status.Error(codes.InvalidArgument, "invalid input")
	case domain.ErrDuplicateTx:
		return status.Error(codes.AlreadyExists, "duplicate transaction")
	case domain.ErrUnsupportedStd:
		return status.Error(codes.InvalidArgument, "unsupported standard")
	case domain.ErrUnauthenticated:
		return status.Error(codes.Unauthenticated, "unauthenticated")
	case domain.ErrSessionTimeout:
		return status.Error(codes.DeadlineExceeded, "session validation timeout")
	default:
		return status.Error(codes.Internal, fmt.Sprintf("internal error: %v", err))
	}
}
