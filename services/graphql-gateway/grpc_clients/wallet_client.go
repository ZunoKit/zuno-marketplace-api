package grpcclients

import (
	"log"

	"github.com/quangdang46/NFT-Marketplace/shared/proto/wallet"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type WalletClient struct {
	Client *wallet.WalletServiceClient
	conn   *grpc.ClientConn
}

func NewWalletClient(url string) *WalletClient {
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	conn, err := grpc.Dial(url, dialOptions...)
	if err != nil {
		log.Fatalf("failed to dial auth service: %v", err)
	}

	client := wallet.NewWalletServiceClient(conn)

	return &WalletClient{
		Client: &client,
		conn:   conn,
	}
}
