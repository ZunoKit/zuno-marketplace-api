package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/service"
)

// MockUserRepository is a mock implementation of UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) GetUserIDByAccount(ctx context.Context, accountID string) (string, error) {
	args := m.Called(ctx, accountID)
	return args.String(0), args.Error(1)
}

func (m *MockUserRepository) WithTx(ctx context.Context, fn func(domain.TxUserRepository) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

// MockTxUserRepository is a mock implementation of TxUserRepository
type MockTxUserRepository struct {
	mock.Mock
}

func (m *MockTxUserRepository) AcquireAccountLock(ctx context.Context, accountID string) error {
	args := m.Called(ctx, accountID)
	return args.Error(0)
}

func (m *MockTxUserRepository) GetUserIDByAccountTx(ctx context.Context, accountID string) (string, error) {
	args := m.Called(ctx, accountID)
	return args.String(0), args.Error(1)
}

func (m *MockTxUserRepository) CreateUserTx(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockTxUserRepository) CreateProfileTx(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockTxUserRepository) UpsertUserAccountTx(ctx context.Context, userID, accountID, address, chainID string) error {
	args := m.Called(ctx, userID, accountID, address, chainID)
	return args.Error(0)
}

func (m *MockTxUserRepository) TouchUserAccountTx(ctx context.Context, accountID, address string) error {
	args := m.Called(ctx, accountID, address)
	return args.Error(0)
}

// UserServiceTestSuite defines the test suite for UserService
type UserServiceTestSuite struct {
	suite.Suite
	userService *service.Service
	mockRepo    *MockUserRepository
}

func (suite *UserServiceTestSuite) SetupTest() {
	suite.mockRepo = new(MockUserRepository)
	suite.userService = service.NewUserService(suite.mockRepo).(*service.Service)
}

func (suite *UserServiceTestSuite) TestEnsureUser_ExistingUser() {
	ctx := context.Background()
	accountID := "test-account-123"
	address := "0x1234567890123456789012345678901234567890"
	chainID := "eip155:1"
	existingUserID := "user-456"

	// Mock fast path - user already exists
	suite.mockRepo.On("GetUserIDByAccount", ctx, accountID).Return(existingUserID, nil)

	result, err := suite.userService.EnsureUser(ctx, accountID, address, chainID)

	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal(existingUserID, result.UserID)
	suite.False(result.Created)
	suite.mockRepo.AssertExpectations(suite.T())
}

func (suite *UserServiceTestSuite) TestEnsureUser_NewUser() {
	ctx := context.Background()
	accountID := "test-account-123"
	address := "0x1234567890123456789012345678901234567890"
	chainID := "eip155:1"
	newUserID := "user-789"

	mockTxRepo := new(MockTxUserRepository)

	// Mock fast path - user not found
	suite.mockRepo.On("GetUserIDByAccount", ctx, accountID).Return("", domain.ErrUserNotFound)

	// Mock transaction path
	suite.mockRepo.On("WithTx", ctx, mock.AnythingOfType("func(domain.TxUserRepository) error")).
		Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(domain.TxUserRepository) error)
			
			// Mock transaction operations
			mockTxRepo.On("AcquireAccountLock", ctx, accountID).Return(nil)
			mockTxRepo.On("GetUserIDByAccountTx", ctx, accountID).Return("", domain.ErrUserNotFound)
			mockTxRepo.On("CreateUserTx", ctx).Return(newUserID, nil)
			mockTxRepo.On("CreateProfileTx", ctx, newUserID).Return(nil)
			mockTxRepo.On("UpsertUserAccountTx", ctx, newUserID, accountID, address, chainID).Return(nil)
			
			fn(mockTxRepo)
		}).Return(nil)

	result, err := suite.userService.EnsureUser(ctx, accountID, address, chainID)

	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal(newUserID, result.UserID)
	suite.True(result.Created)
	suite.mockRepo.AssertExpectations(suite.T())
	mockTxRepo.AssertExpectations(suite.T())
}

func (suite *UserServiceTestSuite) TestEnsureUser_ExistingUserInTransaction() {
	ctx := context.Background()
	accountID := "test-account-123"
	address := "0x1234567890123456789012345678901234567890"
	chainID := "eip155:1"
	existingUserID := "user-456"

	mockTxRepo := new(MockTxUserRepository)

	// Mock fast path - user not found (race condition)
	suite.mockRepo.On("GetUserIDByAccount", ctx, accountID).Return("", domain.ErrUserNotFound)

	// Mock transaction path - but user exists in transaction (created by concurrent request)
	suite.mockRepo.On("WithTx", ctx, mock.AnythingOfType("func(domain.TxUserRepository) error")).
		Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(domain.TxUserRepository) error)
			
			// Mock transaction operations
			mockTxRepo.On("AcquireAccountLock", ctx, accountID).Return(nil)
			mockTxRepo.On("GetUserIDByAccountTx", ctx, accountID).Return(existingUserID, nil)
			mockTxRepo.On("TouchUserAccountTx", ctx, accountID, address).Return(nil)
			
			fn(mockTxRepo)
		}).Return(nil)

	result, err := suite.userService.EnsureUser(ctx, accountID, address, chainID)

	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal(existingUserID, result.UserID)
	suite.False(result.Created)
	suite.mockRepo.AssertExpectations(suite.T())
	mockTxRepo.AssertExpectations(suite.T())
}

func (suite *UserServiceTestSuite) TestEnsureUser_InvalidAccountID() {
	ctx := context.Background()
	
	testCases := []struct {
		name      string
		accountID string
		address   string
		chainID   string
	}{
		{
			name:      "empty account_id",
			accountID: "",
			address:   "0x1234567890123456789012345678901234567890",
			chainID:   "eip155:1",
		},
		{
			name:      "too long account_id",
			accountID: string(make([]byte, 256)), // 256 characters
			address:   "0x1234567890123456789012345678901234567890",
			chainID:   "eip155:1",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result, err := suite.userService.EnsureUser(ctx, tc.accountID, tc.address, tc.chainID)
			
			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "invalid_input")
		})
	}
}

func (suite *UserServiceTestSuite) TestEnsureUser_InvalidAddress() {
	ctx := context.Background()
	
	testCases := []struct {
		name    string
		address string
	}{
		{
			name:    "empty address",
			address: "",
		},
		{
			name:    "invalid length",
			address: "0x123",
		},
		{
			name:    "missing 0x prefix",
			address: "1234567890123456789012345678901234567890",
		},
		{
			name:    "invalid hex characters",
			address: "0x123456789012345678901234567890123456789g",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result, err := suite.userService.EnsureUser(ctx, "test-account", tc.address, "eip155:1")
			
			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "invalid_input")
		})
	}
}

func (suite *UserServiceTestSuite) TestEnsureUser_InvalidChainID() {
	ctx := context.Background()
	accountID := "test-account-123"
	address := "0x1234567890123456789012345678901234567890"

	result, err := suite.userService.EnsureUser(ctx, accountID, address, "")

	suite.Error(err)
	suite.Nil(result)
	suite.Contains(err.Error(), "invalid_input")
}

func (suite *UserServiceTestSuite) TestEnsureUser_DatabaseError() {
	ctx := context.Background()
	accountID := "test-account-123"
	address := "0x1234567890123456789012345678901234567890"
	chainID := "eip155:1"

	// Mock database error
	suite.mockRepo.On("GetUserIDByAccount", ctx, accountID).
		Return("", domain.NewDatabaseError("connection_failed", assert.AnError))

	result, err := suite.userService.EnsureUser(ctx, accountID, address, chainID)

	suite.Error(err)
	suite.Nil(result)
	suite.Contains(err.Error(), "database_operation_failed")
	suite.mockRepo.AssertExpectations(suite.T())
}

func (suite *UserServiceTestSuite) TestEnsureUser_TransactionError() {
	ctx := context.Background()
	accountID := "test-account-123"
	address := "0x1234567890123456789012345678901234567890"
	chainID := "eip155:1"

	// Mock fast path - user not found
	suite.mockRepo.On("GetUserIDByAccount", ctx, accountID).Return("", domain.ErrUserNotFound)

	// Mock transaction error
	suite.mockRepo.On("WithTx", ctx, mock.AnythingOfType("func(domain.TxUserRepository) error")).
		Return(assert.AnError)

	result, err := suite.userService.EnsureUser(ctx, accountID, address, chainID)

	suite.Error(err)
	suite.Nil(result)
	suite.mockRepo.AssertExpectations(suite.T())
}

func TestUserServiceTestSuite(t *testing.T) {
	suite.Run(t, new(UserServiceTestSuite))
}

// Additional unit tests for validation functions
func TestValidateAccountID(t *testing.T) {
	validAccountIDs := []string{
		"test-account",
		"user123",
		"account_with_underscore",
		"account-with-dash",
		"a", // minimum length
		string(make([]byte, 255)), // maximum length
	}

	invalidAccountIDs := []string{
		"",                         // empty
		string(make([]byte, 256)),  // too long
	}

	for _, accountID := range validAccountIDs {
		assert.NoError(t, domain.ValidateAccountID(accountID), "Expected %s to be valid", accountID)
	}

	for _, accountID := range invalidAccountIDs {
		assert.Error(t, domain.ValidateAccountID(accountID), "Expected %s to be invalid", accountID)
	}
}

func TestValidateAddress(t *testing.T) {
	validAddresses := []string{
		"0x1234567890123456789012345678901234567890",
		"0xAbCdEf1234567890123456789012345678901234",
		"0x0000000000000000000000000000000000000000",
		"0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF",
	}

	invalidAddresses := []string{
		"",                                           // empty
		"1234567890123456789012345678901234567890",   // missing 0x
		"0x123",                                      // too short
		"0x12345678901234567890123456789012345678901", // too long
		"0x123456789012345678901234567890123456789g", // invalid hex
		"0X1234567890123456789012345678901234567890", // uppercase X
	}

	for _, address := range validAddresses {
		assert.NoError(t, domain.ValidateAddress(address), "Expected %s to be valid", address)
	}

	for _, address := range invalidAddresses {
		assert.Error(t, domain.ValidateAddress(address), "Expected %s to be invalid", address)
	}
}

func TestValidateChainID(t *testing.T) {
    validChainIDs := []string{
        "eip155:1",
        "eip155:137",
        "cosmos:cosmoshub-4",
        "bip122:000000000019d6689c085ae165831e93",
        "a:b", // minimum format
    }

    invalidChainIDs := []string{
        "", // empty only (current validation)
    }

	for _, chainID := range validChainIDs {
		assert.NoError(t, domain.ValidateChainID(chainID), "Expected %s to be valid", chainID)
	}

	for _, chainID := range invalidChainIDs {
		assert.Error(t, domain.ValidateChainID(chainID), "Expected %s to be invalid", chainID)
	}
}

// Benchmark tests
func BenchmarkEnsureUser_ExistingUser(b *testing.B) {
	mockRepo := new(MockUserRepository)
	userService := service.NewUserService(mockRepo)
	ctx := context.Background()
	
	mockRepo.On("GetUserIDByAccount", mock.Anything, mock.Anything).
		Return("user-123", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userService.EnsureUser(ctx, "test-account", "0x1234567890123456789012345678901234567890", "eip155:1")
	}
}

func BenchmarkValidateAddress(b *testing.B) {
	address := "0x1234567890123456789012345678901234567890"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		domain.ValidateAddress(address)
	}
}

func BenchmarkValidateAccountID(b *testing.B) {
	accountID := "test-account-123"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		domain.ValidateAccountID(accountID)
	}
}
