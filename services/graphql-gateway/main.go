package main

import (
	"log"
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/config"
	graphql_resolver "github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql/schemas"
	grpcclients "github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/grpc_clients"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/middleware"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/websocket"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()
	cfg.Validate()

	log.Printf("Starting GraphQL Gateway with auth: %s, user: %s, wallet: %s, media: %s, chain-registry: %s, orchestrator: %s",
		cfg.AuthServiceURL, cfg.UserServiceURL, cfg.WalletServiceURL, cfg.MediaServiceURL, cfg.ChainRegistryServiceURL, cfg.OrchestratorServiceURL)

	var (
		authClient          *grpcclients.AuthClient
		walletClient        *grpcclients.WalletClient
		mediaClient         *grpcclients.MediaClient
		chainRegistryClient *grpcclients.ChainRegistryClient
		orchestratorClient  *grpcclients.OrchestratorClient
	)

	if cfg.AuthServiceURL != "" {
		authClient = grpcclients.NewAuthClient(cfg.AuthServiceURL)
	}

	if cfg.WalletServiceURL != "" {
		walletClient = grpcclients.NewWalletClient(cfg.WalletServiceURL)
	}

	if cfg.MediaServiceURL != "" {
		mediaClient = grpcclients.NewMediaClient(cfg.MediaServiceURL)
	}

	if cfg.ChainRegistryServiceURL != "" {
		chainRegistryClient = grpcclients.NewChainRegistryClient(cfg.ChainRegistryServiceURL)
	}

	if cfg.OrchestratorServiceURL != "" {
		orchestratorClient = grpcclients.NewOrchestratorClient(cfg.OrchestratorServiceURL)
	}

	// Initialize WebSocket client for subscription worker
	var wsClient *websocket.Client
	if cfg.SubscriptionWorkerWSURL != "" {
		wsClient = websocket.NewClient(cfg.SubscriptionWorkerWSURL)
		if err := wsClient.Connect(); err != nil {
			log.Printf("Warning: Failed to connect to subscription worker WebSocket: %v", err)
			log.Println("GraphQL subscriptions will fall back to polling mode")
		} else {
			log.Printf("Successfully connected to subscription worker WebSocket: %s", cfg.SubscriptionWorkerWSURL)
		}
	}

	resolver := graphql_resolver.NewResolver(authClient, walletClient, mediaClient).WithChainRegistryClient(chainRegistryClient).WithOrchestratorClient(orchestratorClient)
	
	// Connect WebSocket client if available
	if wsClient != nil {
		resolver = resolver.WithWebSocketClient(wsClient)
	}
	// TODO: pass collectionClient into resolver once gql schema/resolvers are added
	es := schemas.NewExecutableSchema(schemas.Config{Resolvers: resolver})

	// Create GraphQL handler with middleware chain
	graphqlHandler := handler.NewDefaultServer(es)

	// Apply middleware chain: Auth -> Cookie -> GraphQL
	middlewareChain := middleware.CreateAuthMiddleware()(
		middleware.CookieMiddleware(graphqlHandler),
	)

	http.Handle("/graphql", middlewareChain)
	http.Handle("/playground", playground.Handler("GraphQL playground", "/graphql"))
	http.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	log.Printf("GraphQL server running at %s/playground", cfg.HTTPAddr)

	log.Fatal(http.ListenAndServe(cfg.HTTPAddr, nil))
}
