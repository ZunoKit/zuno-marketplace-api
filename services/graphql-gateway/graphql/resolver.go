package graphql_resolver

import (
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql/schemas"
	grpcclients "github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/grpc_clients"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/websocket"
)

type Resolver struct {
	authClient          *grpcclients.AuthClient
	walletClient        *grpcclients.WalletClient
	mediaClient         *grpcclients.MediaClient
	chainRegistryClient *grpcclients.ChainRegistryClient
	orchestratorClient  *grpcclients.OrchestratorClient
	websocketClient     *websocket.Client
}

func NewResolver(authClient *grpcclients.AuthClient, walletClient *grpcclients.WalletClient, mediaClient *grpcclients.MediaClient) *Resolver {
	return &Resolver{
		authClient:   authClient,
		walletClient: walletClient,
		mediaClient:  mediaClient,
	}
}

func (r *Resolver) WithOrchestratorClient(c *grpcclients.OrchestratorClient) *Resolver {
	r.orchestratorClient = c
	return r
}

func (r *Resolver) WithChainRegistryClient(c *grpcclients.ChainRegistryClient) *Resolver {
	r.chainRegistryClient = c
	return r
}

func (r *Resolver) WithWebSocketClient(c *websocket.Client) *Resolver {
	r.websocketClient = c
	return r
}

// gqlgen root bindings
func (r *Resolver) Mutation() schemas.MutationResolver { return &MutationResolver{server: r} }
func (r *Resolver) Query() schemas.QueryResolver       { return &QueryResolver{server: r} }
func (r *Resolver) Subscription() schemas.SubscriptionResolver {
	return &SubscriptionResolver{server: r}
}
