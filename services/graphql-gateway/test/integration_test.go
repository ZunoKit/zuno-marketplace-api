package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	graphql_resolver "github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql/schemas"
	grpcclients "github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/grpc_clients"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/middleware"
	authpb "github.com/quangdang46/NFT-Marketplace/shared/proto/auth"
	walletpb "github.com/quangdang46/NFT-Marketplace/shared/proto/wallet"
)

// GraphQLRequest represents a GraphQL request
type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// GraphQLResponse represents a GraphQL response
type GraphQLResponse struct {
	Data   interface{}    `json:"data,omitempty"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

type GraphQLError struct {
	Message string        `json:"message"`
	Path    []interface{} `json:"path,omitempty"`
}

// IntegrationTestSuite defines the test suite for integration tests
type IntegrationTestSuite struct {
	suite.Suite
	server               *httptest.Server
	resolver             *graphql_resolver.Resolver
	mockAuthClient       *MockAuthServiceClient
	mockWalletClient     *MockWalletServiceClient
}

func (suite *IntegrationTestSuite) SetupTest() {
	suite.mockAuthClient = new(MockAuthServiceClient)
	suite.mockWalletClient = new(MockWalletServiceClient)
	// Create mock gRPC clients using interfaces
	var ac authpb.AuthServiceClient = suite.mockAuthClient
	var wc walletpb.WalletServiceClient = suite.mockWalletClient
	authClient := &grpcclients.AuthClient{Client: &ac}
	walletClient := &grpcclients.WalletClient{Client: &wc}
	suite.resolver = graphql_resolver.NewResolver(authClient, walletClient, nil)
	es := schemas.NewExecutableSchema(schemas.Config{Resolvers: suite.resolver})

	// Create GraphQL handler with middleware chain
	graphqlHandler := handler.NewDefaultServer(es)

	// Apply middleware chain: Auth -> Cookie -> GraphQL
	middlewareChain := middleware.CreateAuthMiddleware()(
		middleware.CookieMiddleware(graphqlHandler),
	)

	// Create test server
	mux := http.NewServeMux()
	mux.Handle("/graphql", middlewareChain)
	mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	suite.server = httptest.NewServer(mux)
}

func (suite *IntegrationTestSuite) TearDownTest() {
	suite.server.Close()
}

func (suite *IntegrationTestSuite) TestHealthEndpoint() {
	resp, err := http.Get(suite.server.URL + "/health")
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var body bytes.Buffer
	_, err = body.ReadFrom(resp.Body)
	suite.Require().NoError(err)
	suite.Equal("OK", body.String())
}

func (suite *IntegrationTestSuite) TestHealthQuery() {
	query := `query { health }`

	response := suite.executeGraphQLQuery(query, nil, "")

	suite.NotNil(response.Data)
	data := response.Data.(map[string]interface{})
	suite.Equal("ok", data["health"])
	suite.Empty(response.Errors)
}

func (suite *IntegrationTestSuite) TestSignInSiweMutation() {
	query := `
		mutation SignInSiwe($input: SignInSiweInput!) {
			signInSiwe(input: $input) {
				nonce
			}
		}
	`

	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"accountId": "test-account-123",
			"chainId":   "eip155:1",
			"domain":    "localhost",
		},
	}

	expectedResponse := &authpb.GetNonceResponse{
		Nonce: "test-nonce-456",
	}

	suite.mockAuthClient.On("GetNonce", mock.Anything, mock.MatchedBy(func(req *authpb.GetNonceRequest) bool {
		return req.AccountId == "test-account-123" &&
			req.ChainId == "eip155:1" &&
			req.Domain == "localhost"
	})).Return(expectedResponse, nil)

	response := suite.executeGraphQLQuery(query, variables, "")

	suite.NotNil(response.Data)
	suite.Empty(response.Errors)

	data := response.Data.(map[string]interface{})
	signInData := data["signInSiwe"].(map[string]interface{})
	suite.Equal(expectedResponse.Nonce, signInData["nonce"])

	suite.mockAuthClient.AssertExpectations(suite.T())
}

func (suite *IntegrationTestSuite) TestVerifySiweMutation() {
	query := `
		mutation VerifySiwe($input: VerifySiweInput!) {
			verifySiwe(input: $input) {
				accessToken
				refreshToken
				userId
				expiresAt
			}
		}
	`

	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"accountId": "test-account-123",
			"message":   "test message",
			"signature": "0x1234567890abcdef",
		},
	}

	expectedResponse := &authpb.VerifySiweResponse{
		AccessToken:  "access-token-123",
		RefreshToken: "refresh-token-456",
		ExpiresAt:    "2023-12-31T23:59:59Z",
		UserId:       "user-789",
		Address:      "0x1234567890123456789012345678901234567890",
		ChainId:      "eip155:1",
	}

	suite.mockAuthClient.On("VerifySiwe", mock.Anything, mock.MatchedBy(func(req *authpb.VerifySiweRequest) bool {
		return req.AccountId == "test-account-123" &&
			req.Message == "test message" &&
			req.Signature == "0x1234567890abcdef"
	})).Return(expectedResponse, nil)

	response := suite.executeGraphQLQuery(query, variables, "")

	suite.NotNil(response.Data)
	suite.Empty(response.Errors)

	data := response.Data.(map[string]interface{})
	verifyData := data["verifySiwe"].(map[string]interface{})
	suite.Equal(expectedResponse.AccessToken, verifyData["accessToken"])
	suite.Equal(expectedResponse.RefreshToken, verifyData["refreshToken"])
	suite.Equal(expectedResponse.UserId, verifyData["userId"])
	suite.Equal(expectedResponse.ExpiresAt, verifyData["expiresAt"])

	suite.mockAuthClient.AssertExpectations(suite.T())
}

func (suite *IntegrationTestSuite) TestMeQuery_Unauthenticated() {
	query := `query { me { id } }`

	response := suite.executeGraphQLQuery(query, nil, "")

	suite.NotNil(response.Data)
	suite.Empty(response.Errors)

	data := response.Data.(map[string]interface{})
	suite.Nil(data["me"])
}

func (suite *IntegrationTestSuite) TestMeQuery_Authenticated() {
	query := `query { me { id } }`

	// Create a valid JWT token
	accessToken := suite.createValidJWT("user-123", "session-456")

	response := suite.executeGraphQLQuery(query, nil, accessToken)

	suite.NotNil(response.Data)
	suite.Empty(response.Errors)

	data := response.Data.(map[string]interface{})
	me := data["me"].(map[string]interface{})
	suite.Equal("user-123", me["id"])
}

func (suite *IntegrationTestSuite) TestProtectedMutation_Authenticated() {
	query := `
		mutation UpdateProfile($displayName: String) {
			updateProfile(displayName: $displayName)
		}
	`

	variables := map[string]interface{}{
		"displayName": "New Display Name",
	}

	// Create a valid JWT token
	accessToken := suite.createValidJWT("user-123", "session-456")

	response := suite.executeGraphQLQuery(query, variables, accessToken)

	suite.NotNil(response.Data)
	suite.Empty(response.Errors)

	data := response.Data.(map[string]interface{})
	suite.True(data["updateProfile"].(bool))
}

func (suite *IntegrationTestSuite) TestProtectedMutation_Unauthenticated() {
	query := `
		mutation UpdateProfile($displayName: String) {
			updateProfile(displayName: $displayName)
		}
	`

	variables := map[string]interface{}{
		"displayName": "New Display Name",
	}

	response := suite.executeGraphQLQuery(query, variables, "")

	suite.Empty(response.Data)
	suite.NotEmpty(response.Errors)
	suite.Contains(response.Errors[0].Message, "authentication required")
}

func (suite *IntegrationTestSuite) TestInvalidQuery() {
	query := `query { invalidField }`

	response := suite.executeGraphQLQuery(query, nil, "")

	suite.Empty(response.Data)
	suite.NotEmpty(response.Errors)
	suite.Contains(response.Errors[0].Message, "Cannot query field")
}

func (suite *IntegrationTestSuite) TestMalformedQuery() {
	query := `query { health ` // Missing closing brace

	response := suite.executeGraphQLQuery(query, nil, "")

	suite.Empty(response.Data)
	suite.NotEmpty(response.Errors)
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

// Helper methods
func (suite *IntegrationTestSuite) executeGraphQLQuery(query string, variables map[string]interface{}, accessToken string) *GraphQLResponse {
	reqBody := GraphQLRequest{
		Query:     query,
		Variables: variables,
	}

	jsonBody, err := json.Marshal(reqBody)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", suite.server.URL+"/graphql", bytes.NewBuffer(jsonBody))
	suite.Require().NoError(err)

	req.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	var response GraphQLResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.Require().NoError(err)

	return &response
}

func (suite *IntegrationTestSuite) createValidJWT(userID, sessionID string) string {
	// Create a JWT compatible with middleware.CreateAuthMiddleware()
	// which uses env JWT_SECRET with default "default-jwt-secret-for-development"
	secret := []byte("default-jwt-secret-for-development")
	claims := jwt.MapClaims{
		"sub":        userID,
		"session_id": sessionID,
		"iss":        "nft-marketplace-auth",
		"exp":        time.Now().Add(time.Hour).Unix(),
		"iat":        time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString(secret)
	return signed
}

// Performance tests
func TestGraphQLPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	t.Run("HighThroughputQueries", func(t *testing.T) {
		// Test high throughput query handling
		t.Skip("Requires performance test setup")
	})

	t.Run("LargeQueryComplexity", func(t *testing.T) {
		// Test handling of complex queries
		t.Skip("Requires complex query test setup")
	})
}

// Load tests
func TestGraphQLLoadHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load tests in short mode")
	}

	t.Run("ConcurrentConnections", func(t *testing.T) {
		// Test concurrent connection handling
		t.Skip("Requires load test setup")
	})

	t.Run("MemoryUsage", func(t *testing.T) {
		// Test memory usage under load
		t.Skip("Requires memory profiling setup")
	})
}

// Security tests
func TestGraphQLSecurity(t *testing.T) {
	t.Run("QueryComplexityLimits", func(t *testing.T) {
		// Test query complexity limits
		t.Skip("Requires complexity limiting setup")
	})

	t.Run("RateLimiting", func(t *testing.T) {
		// Test rate limiting
		t.Skip("Requires rate limiting setup")
	})

	t.Run("InputValidation", func(t *testing.T) {
		// Test input validation and sanitization
		t.Skip("Requires input validation tests")
	})
}

// Error handling tests
func TestGraphQLErrorHandling(t *testing.T) {
	t.Run("ServiceDowntime", func(t *testing.T) {
		// Test behavior when backend services are down
		t.Skip("Requires service downtime simulation")
	})

	t.Run("PartialFailures", func(t *testing.T) {
		// Test handling of partial failures in federated queries
		t.Skip("Requires partial failure simulation")
	})

	t.Run("TimeoutHandling", func(t *testing.T) {
		// Test timeout handling for slow backend services
		t.Skip("Requires timeout simulation")
	})
}

// WebSocket tests (if implemented)
func TestGraphQLWebSocket(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping WebSocket tests in short mode")
	}

	t.Run("SubscriptionLifecycle", func(t *testing.T) {
		// Test WebSocket subscription lifecycle
		t.Skip("Requires WebSocket subscription implementation")
	})

	t.Run("AuthenticationOverWebSocket", func(t *testing.T) {
		// Test authentication over WebSocket connections
		t.Skip("Requires WebSocket auth implementation")
	})

	t.Run("RealtimeUpdates", func(t *testing.T) {
		// Test real-time updates via subscriptions
		t.Skip("Requires real-time update implementation")
	})
}
