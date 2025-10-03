package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	graphql_resolver "github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql/schemas"
	grpcclients "github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/grpc_clients"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/middleware"
)

// OrchestratorIntegrationTestSuite defines integration tests for orchestrator functionality
type OrchestratorIntegrationTestSuite struct {
	suite.Suite
	resolver *graphql_resolver.Resolver
}

func (suite *OrchestratorIntegrationTestSuite) SetupTest() {
	// Create resolver without any gRPC clients for integration testing
	// In a real integration test, you would connect to actual services
	suite.resolver = graphql_resolver.NewResolver(nil, nil, nil)
}

func (suite *OrchestratorIntegrationTestSuite) createAuthenticatedContext() context.Context {
	ctx := context.Background()
	user := &middleware.CurrentUser{
		UserID:    "test-user-id",
		SessionID: "test-session-id",
	}
	ctx = context.WithValue(ctx, middleware.CurrentUserKey, user)
	ctx = context.WithValue(ctx, middleware.SessionIDKey, user.SessionID)
	return ctx
}

func (suite *OrchestratorIntegrationTestSuite) TestResolverStructure() {
	// Test that the resolver structure is properly set up
	assert.NotNil(suite.T(), suite.resolver)

	mutationResolver := suite.resolver.Mutation()
	assert.NotNil(suite.T(), mutationResolver)

	subscriptionResolver := suite.resolver.Subscription()
	assert.NotNil(suite.T(), subscriptionResolver)
}

func (suite *OrchestratorIntegrationTestSuite) TestOrchestratorClientIntegration() {
	// Test that orchestrator client can be added to resolver
	orchestratorClient := &grpcclients.OrchestratorClient{}
	resolver := suite.resolver.WithOrchestratorClient(orchestratorClient)

	assert.NotNil(suite.T(), resolver)
	// Note: orchestratorClient field is unexported, so we can't directly test it
	// The fact that WithOrchestratorClient doesn't panic is sufficient
}

func (suite *OrchestratorIntegrationTestSuite) TestInputValidation() {
	// Test input validation without service calls
	mutationResolver := suite.resolver.Mutation()

	// Test with missing required fields
	ctx := suite.createAuthenticatedContext()
	input := schemas.PrepareCreateCollectionInput{
		ChainID: "", // Missing required field
		Name:    "Test Collection",
		Symbol:  "TEST",
	}

	result, err := mutationResolver.PrepareCreateCollection(ctx, input)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing required fields")
}

func (suite *OrchestratorIntegrationTestSuite) TestAuthenticationRequirement() {
	// Test that authentication is required
	mutationResolver := suite.resolver.Mutation()

	// Test without authentication
	ctx := context.Background() // No authentication
	input := schemas.PrepareCreateCollectionInput{
		ChainID: "1",
		Name:    "Test Collection",
		Symbol:  "TEST",
	}

	result, err := mutationResolver.PrepareCreateCollection(ctx, input)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "authentication required")
}

func (suite *OrchestratorIntegrationTestSuite) TestServiceUnavailable() {
	// Test behavior when orchestrator service is unavailable
	mutationResolver := suite.resolver.Mutation()

	ctx := suite.createAuthenticatedContext()
	input := schemas.PrepareCreateCollectionInput{
		ChainID: "1",
		Name:    "Test Collection",
		Symbol:  "TEST",
	}

	result, err := mutationResolver.PrepareCreateCollection(ctx, input)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "orchestrator service unavailable")
}

func (suite *OrchestratorIntegrationTestSuite) TestSubscriptionStructure() {
	// Test subscription resolver structure
	subscriptionResolver := suite.resolver.Subscription()

	ctx := context.Background()
	intentID := "test-intent-id"

	result, err := subscriptionResolver.OnIntentStatus(ctx, intentID)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "orchestrator service unavailable")
}

// Run the integration test suite
func TestOrchestratorIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(OrchestratorIntegrationTestSuite))
}
