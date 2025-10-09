package main

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/config"
	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/encode"
	grpcHandler "github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/infrastructure/grpc"
	rep "github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/infrastructure/repository"
	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/service"
	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/status"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	protoChainRegistry "github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry"
	orchestratorpb "github.com/quangdang46/NFT-Marketplace/shared/proto/orchestrator"
	"github.com/quangdang46/NFT-Marketplace/shared/recovery"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx := context.Background()

	// Load config
	cfg := config.LoadConfig()
	_ = cfg.Validate()

	pg, err := postgres.NewPostgres(cfg.Postgres)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	if err := pg.HealthCheck(ctx); err != nil {
		log.Fatalf("postgres ping: %v", err)
	}

	r, err := redis.NewRedis(cfg.Redis)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	if err := r.HealthCheck(ctx); err != nil {
		log.Fatalf("redis ping: %v", err)
	}

	repo := rep.NewOrchestratorRepo(pg, r)
	log.Printf("chain-registry-service URL: %s", cfg.ChainRegistryGRPCURL)
	conn, err := grpc.Dial(cfg.ChainRegistryGRPCURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("chain-registry connection: %v", err)
	}
	chainRegistryClient := protoChainRegistry.NewChainRegistryServiceClient(conn)

	encoder := encode.NewEncoder(chainRegistryClient)
	statusCache := status.NewStatusCache()
	statusCache.(*status.StatusCache).SetRedis(r)

	svc := service.NewOrchestratorWithTimeout(
		repo,
		encoder,
		statusCache,
		chainRegistryClient,
		cfg.Features.SessionLinkedIntents,
		time.Duration(cfg.Features.SessionValidationTimeoutMs)*time.Millisecond,
	)

	lis, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	// Create panic handler
	panicHandler := recovery.NewPanicHandler(
		recovery.WithStackLogging(true),
		recovery.WithPanicCallback(func(recovered interface{}, stack []byte) {
			log.Printf("PANIC recovered in orchestrator-service: %v\n%s", recovered, stack)
		}),
	)

	// Create gRPC server with panic recovery
	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			panicHandler.UnaryServerInterceptor(),
		),
		grpc.StreamInterceptor(panicHandler.StreamServerInterceptor()),
	)
	handler := grpcHandler.NewGRPCHandler(svc)
	orchestratorpb.RegisterOrchestratorServiceServer(s, handler)
	log.Printf("orchestrator-service gRPC on %s", cfg.GRPCPort)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
