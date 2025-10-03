package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	graphql_resolver "github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql/schemas"
	grpcclients "github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/grpc_clients"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/middleware"
	orchestratorpb "github.com/quangdang46/NFT-Marketplace/shared/proto/orchestrator"
	"google.golang.org/grpc"
)

// MockOrchestratorServiceClient is a mock implementation of OrchestratorServiceClient
type MockOrchestratorServiceClient struct {
	mock.Mock
}

func (m *MockOrchestratorServiceClient) PrepareCreateCollection(ctx context.Context, req *orchestratorpb.PrepareCreateCollectionRequest, opts ...grpc.CallOption) (*orchestratorpb.PrepareCreateCollectionResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*orchestratorpb.PrepareCreateCollectionResponse), args.Error(1)
}

func (m *MockOrchestratorServiceClient) PrepareMint(ctx context.Context, req *orchestratorpb.PrepareMintRequest, opts ...grpc.CallOption) (*orchestratorpb.PrepareMintResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*orchestratorpb.PrepareMintResponse), args.Error(1)
}

func (m *MockOrchestratorServiceClient) TrackTx(ctx context.Context, req *orchestratorpb.TrackTxRequest, opts ...grpc.CallOption) (*orchestratorpb.TrackTxResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*orchestratorpb.TrackTxResponse), args.Error(1)
}

func (m *MockOrchestratorServiceClient) GetIntentStatus(ctx context.Context, req *orchestratorpb.GetIntentStatusRequest, opts ...grpc.CallOption) (*orchestratorpb.GetIntentStatusResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*orchestratorpb.GetIntentStatusResponse), args.Error(1)
}

// OrchestratorResolverTestSuite defines the test suite for orchestrator resolvers
type OrchestratorResolverTestSuite struct {
	suite.Suite
	resolver               *graphql_resolver.Resolver
	mockOrchestratorClient *MockOrchestratorServiceClient
	mutationResolver       schemas.MutationResolver
	subscriptionResolver   schemas.SubscriptionResolver
}

func (suite *OrchestratorResolverTestSuite) SetupTest() {
	suite.mockOrchestratorClient = new(MockOrchestratorServiceClient)

	// Create mock gRPC client
	var oc orchestratorpb.OrchestratorServiceClient = suite.mockOrchestratorClient
	orchestratorClient := &grpcclients.OrchestratorClient{Client: &oc}

	suite.resolver = graphql_resolver.NewResolver(nil, nil, nil).WithOrchestratorClient(orchestratorClient)
	suite.mutationResolver = suite.resolver.Mutation()
	suite.subscriptionResolver = suite.resolver.Subscription()
}

func (suite *OrchestratorResolverTestSuite) createAuthenticatedContext() context.Context {
	ctx := context.Background()
	user := &middleware.CurrentUser{
		UserID:    "test-user-id",
		SessionID: "test-session-id",
	}
	ctx = context.WithValue(ctx, middleware.CurrentUserKey, user)
	ctx = context.WithValue(ctx, middleware.SessionIDKey, user.SessionID)
	return ctx
}

func (suite *OrchestratorResolverTestSuite) TestPrepareCreateCollection_Success() {
	// Arrange
	ctx := suite.createAuthenticatedContext()
	input := schemas.PrepareCreateCollectionInput{
		ChainID: "1",
		Name:    "Test Collection",
		Symbol:  "TEST",
	}

	expectedResponse := &orchestratorpb.PrepareCreateCollectionResponse{
		IntentId: "test-intent-id",
		Tx: &orchestratorpb.TxRequest{
			To:             "0x1234567890123456789012345678901234567890",
			Data:           []byte("0x123456"),
			Value:          "0",
			PreviewAddress: "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		},
	}

	suite.mockOrchestratorClient.On("PrepareCreateCollection", ctx, mock.AnythingOfType("*orchestrator.PrepareCreateCollectionRequest")).
		Return(expectedResponse, nil)

	// Act
	result, err := suite.mutationResolver.PrepareCreateCollection(ctx, input)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "test-intent-id", result.IntentID)
	assert.Equal(suite.T(), "0x1234567890123456789012345678901234567890", result.TxRequest.To)
	assert.Equal(suite.T(), "0x123456", result.TxRequest.Data)
	assert.Equal(suite.T(), "0", result.TxRequest.Value)
	assert.Equal(suite.T(), "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd", *result.TxRequest.PreviewAddress)

	suite.mockOrchestratorClient.AssertExpectations(suite.T())
}

func (suite *OrchestratorResolverTestSuite) TestPrepareCreateCollection_WithOptionalFields() {
	// Arrange
	ctx := suite.createAuthenticatedContext()
	tokenURI := "ipfs://logo"
	desc := "banner desc"
	input := schemas.PrepareCreateCollectionInput{
		ChainID:     "1",
		Name:        "Test Collection",
		Symbol:      "TEST",
		TokenURI:    &tokenURI,
		Description: &desc,
	}

	expectedResponse := &orchestratorpb.PrepareCreateCollectionResponse{
		IntentId: "test-intent-id",
		Tx: &orchestratorpb.TxRequest{
			To:             "0x1234567890123456789012345678901234567890",
			Data:           []byte("0x123456"),
			Value:          "0",
			PreviewAddress: "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		},
	}

	suite.mockOrchestratorClient.On("PrepareCreateCollection", ctx, mock.AnythingOfType("*orchestrator.PrepareCreateCollectionRequest")).
		Return(expectedResponse, nil)

	// Act
	result, err := suite.mutationResolver.PrepareCreateCollection(ctx, input)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)

	suite.mockOrchestratorClient.AssertExpectations(suite.T())
}

func (suite *OrchestratorResolverTestSuite) TestPrepareCreateCollection_MissingRequiredFields() {
	// Arrange
	ctx := suite.createAuthenticatedContext()
	input := schemas.PrepareCreateCollectionInput{
		ChainID: "",
		Name:    "Test Collection",
		Symbol:  "TEST",
	}

	// Act
	result, err := suite.mutationResolver.PrepareCreateCollection(ctx, input)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing required fields")
}

func (suite *OrchestratorResolverTestSuite) TestPrepareCreateCollection_NotAuthenticated() {
	// Arrange
	ctx := context.Background() // No authentication
	input := schemas.PrepareCreateCollectionInput{
		ChainID: "1",
		Name:    "Test Collection",
		Symbol:  "TEST",
	}

	// Act
	result, err := suite.mutationResolver.PrepareCreateCollection(ctx, input)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "authentication required")
}

func (suite *OrchestratorResolverTestSuite) TestPrepareMint_Success() {
	// Arrange
	ctx := suite.createAuthenticatedContext()
	input := schemas.PrepareMintInput{
		ChainID:  "1",
		Contract: "0x1234567890123456789012345678901234567890",
		Standard: "ERC721",
		Quantity: nil, // Use default value
	}

	expectedResponse := &orchestratorpb.PrepareMintResponse{
		IntentId: "test-mint-intent-id",
		Tx: &orchestratorpb.TxRequest{
			To:             "0x1234567890123456789012345678901234567890",
			Data:           []byte("0x654321"),
			Value:          "0",
			PreviewAddress: "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		},
	}

	suite.mockOrchestratorClient.On("PrepareMint", ctx, mock.AnythingOfType("*orchestrator.PrepareMintRequest")).
		Return(expectedResponse, nil)

	// Act
	result, err := suite.mutationResolver.PrepareMint(ctx, input)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "test-mint-intent-id", result.IntentID)
	assert.Equal(suite.T(), "0x1234567890123456789012345678901234567890", result.TxRequest.To)
	assert.Equal(suite.T(), "0x654321", result.TxRequest.Data)
	assert.Equal(suite.T(), "0", result.TxRequest.Value)

	suite.mockOrchestratorClient.AssertExpectations(suite.T())
}

func (suite *OrchestratorResolverTestSuite) TestPrepareMint_WithQuantity() {
	// Arrange
	ctx := suite.createAuthenticatedContext()
	quantity := 5
	input := schemas.PrepareMintInput{
		ChainID:  "1",
		Contract: "0x1234567890123456789012345678901234567890",
		Standard: "ERC1155",
		Quantity: &quantity,
	}

	expectedResponse := &orchestratorpb.PrepareMintResponse{
		IntentId: "test-mint-intent-id",
		Tx: &orchestratorpb.TxRequest{
			To:             "0x1234567890123456789012345678901234567890",
			Data:           []byte("0x654321"),
			Value:          "0",
			PreviewAddress: "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		},
	}

	suite.mockOrchestratorClient.On("PrepareMint", ctx, mock.AnythingOfType("*orchestrator.PrepareMintRequest")).
		Return(expectedResponse, nil)

	// Act
	result, err := suite.mutationResolver.PrepareMint(ctx, input)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)

	suite.mockOrchestratorClient.AssertExpectations(suite.T())
}

func (suite *OrchestratorResolverTestSuite) TestTrackTx_Success() {
	// Arrange
	ctx := context.Background()
	contract := "0x1234567890123456789012345678901234567890"
	input := schemas.TrackTxInput{
		IntentID: "test-intent-id",
		ChainID:  "1",
		TxHash:   "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Contract: &contract,
	}

	expectedResponse := &orchestratorpb.TrackTxResponse{
		Ok: true,
	}

	suite.mockOrchestratorClient.On("TrackTx", ctx, mock.AnythingOfType("*orchestrator.TrackTxRequest")).
		Return(expectedResponse, nil)

	// Act
	result, err := suite.mutationResolver.TrackTx(ctx, input)

	// Assert
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), result)

	suite.mockOrchestratorClient.AssertExpectations(suite.T())
}

func (suite *OrchestratorResolverTestSuite) TestTrackTx_WithoutContract() {
	// Arrange
	ctx := context.Background()
	input := schemas.TrackTxInput{
		IntentID: "test-intent-id",
		ChainID:  "1",
		TxHash:   "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Contract: nil,
	}

	expectedResponse := &orchestratorpb.TrackTxResponse{
		Ok: true,
	}

	suite.mockOrchestratorClient.On("TrackTx", ctx, mock.AnythingOfType("*orchestrator.TrackTxRequest")).
		Return(expectedResponse, nil)

	// Act
	result, err := suite.mutationResolver.TrackTx(ctx, input)

	// Assert
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), result)

	suite.mockOrchestratorClient.AssertExpectations(suite.T())
}

func (suite *OrchestratorResolverTestSuite) TestTrackTx_MissingRequiredFields() {
	// Arrange
	ctx := context.Background()
	input := schemas.TrackTxInput{
		IntentID: "",
		ChainID:  "1",
		TxHash:   "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
	}

	// Act
	result, err := suite.mutationResolver.TrackTx(ctx, input)

	// Assert
	assert.Error(suite.T(), err)
	assert.False(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing required fields")
}

func (suite *OrchestratorResolverTestSuite) TestOnIntentStatus_Success() {
	// Arrange
	ctx := context.Background()
	intentID := "test-intent-id"

	expectedResponse := &orchestratorpb.GetIntentStatusResponse{
		IntentId:        "test-intent-id",
		Kind:            "create_collection",
		Status:          "pending",
		ChainId:         "1",
		TxHash:          "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		ContractAddress: "0x1234567890123456789012345678901234567890",
	}

	suite.mockOrchestratorClient.On("GetIntentStatus", ctx, mock.AnythingOfType("*orchestrator.GetIntentStatusRequest")).
		Return(expectedResponse, nil)

	// Act
	result, err := suite.subscriptionResolver.OnIntentStatus(ctx, intentID)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)

	// Wait for the goroutine to send the first message
	select {
	case status := <-result:
		assert.Equal(suite.T(), "test-intent-id", status.IntentID)
		assert.Equal(suite.T(), "create_collection", status.Kind)
		assert.Equal(suite.T(), schemas.IntentStatusPending, status.Status)
		assert.Equal(suite.T(), "1", *status.ChainID)
		assert.Equal(suite.T(), "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd", *status.TxHash)
		assert.Equal(suite.T(), "0x1234567890123456789012345678901234567890", *status.ContractAddress)
	default:
		suite.T().Error("Expected to receive status update")
	}

	suite.mockOrchestratorClient.AssertExpectations(suite.T())
}

func (suite *OrchestratorResolverTestSuite) TestOnIntentStatus_InvalidIntentID() {
	// Arrange
	ctx := context.Background()
	intentID := ""

	// Act
	result, err := suite.subscriptionResolver.OnIntentStatus(ctx, intentID)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "invalid intent ID")
}

func (suite *OrchestratorResolverTestSuite) TestOrchestratorServiceUnavailable() {
	// Arrange
	resolver := graphql_resolver.NewResolver(nil, nil, nil) // No orchestrator client
	mutationResolver := resolver.Mutation()

	ctx := suite.createAuthenticatedContext()
	input := schemas.PrepareCreateCollectionInput{
		ChainID: "1",
		Name:    "Test Collection",
		Symbol:  "TEST",
	}

	// Act
	result, err := mutationResolver.PrepareCreateCollection(ctx, input)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "orchestrator service unavailable")
}

// Run the test suite
func TestOrchestratorResolverTestSuite(t *testing.T) {
	suite.Run(t, new(OrchestratorResolverTestSuite))
}
