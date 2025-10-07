package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/service"
)

// MockUserRepository mocks the user repository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) CreateUser(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetUser(ctx context.Context, userID domain.UserID) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if user := args.Get(0); user != nil {
		return user.(*domain.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockUserRepository) GetUserByAddress(ctx context.Context, address string) (*domain.User, error) {
	args := m.Called(ctx, address)
	if user := args.Get(0); user != nil {
		return user.(*domain.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockUserRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) CreateProfile(ctx context.Context, profile *domain.Profile) error {
	args := m.Called(ctx, profile)
	return args.Error(0)
}

func (m *MockUserRepository) GetProfile(ctx context.Context, userID domain.UserID) (*domain.Profile, error) {
	args := m.Called(ctx, userID)
	if profile := args.Get(0); profile != nil {
		return profile.(*domain.Profile), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockUserRepository) UpdateProfile(ctx context.Context, profile *domain.Profile) error {
	args := m.Called(ctx, profile)
	return args.Error(0)
}

// MockUserEventPublisher mocks the event publisher
type MockUserEventPublisher struct {
	mock.Mock
}

func (m *MockUserEventPublisher) PublishUserCreated(ctx context.Context, event *domain.UserCreatedEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

// UserServiceTestSuite defines the test suite
type UserServiceTestSuite struct {
	suite.Suite
	userService   domain.UserService
	mockRepo      *MockUserRepository
	mockPublisher *MockUserEventPublisher
}

func (suite *UserServiceTestSuite) SetupTest() {
	suite.mockRepo = new(MockUserRepository)
	suite.mockPublisher = new(MockUserEventPublisher)
	suite.userService = service.NewUserService(suite.mockRepo, suite.mockPublisher)
}

func (suite *UserServiceTestSuite) TestEnsureUser_CreateNew() {
	ctx := context.Background()
	accountID := "0x1234567890123456789012345678901234567890"
	address := "0x1234567890123456789012345678901234567890"
	chainID := "eip155:1"

	// Mock no existing user
	suite.mockRepo.On("GetUserByAddress", ctx, address).Return(nil, domain.ErrUserNotFound)

	// Mock create user
	suite.mockRepo.On("CreateUser", ctx, mock.AnythingOfType("*domain.User")).Return(nil)

	// Mock create profile
	suite.mockRepo.On("CreateProfile", ctx, mock.AnythingOfType("*domain.Profile")).Return(nil)

	// Act
	userID, created, err := suite.userService.EnsureUser(ctx, accountID, address, chainID)

	// Assert
	suite.NoError(err)
	suite.NotEmpty(userID)
	suite.True(created)
	suite.mockRepo.AssertExpectations(suite.T())
}

func (suite *UserServiceTestSuite) TestEnsureUser_ExistingUser() {
	ctx := context.Background()
	accountID := "0x1234567890123456789012345678901234567890"
	address := "0x1234567890123456789012345678901234567890"
	chainID := "eip155:1"

	existingUser := &domain.User{
		ID:        "existing-user-123",
		Status:    domain.UserStatusActive,
		CreatedAt: time.Now(),
	}

	// Mock existing user
	suite.mockRepo.On("GetUserByAddress", ctx, address).Return(existingUser, nil)

	// Act
	userID, created, err := suite.userService.EnsureUser(ctx, accountID, address, chainID)

	// Assert
	suite.NoError(err)
	suite.Equal(existingUser.ID, userID)
	suite.False(created)
	suite.mockRepo.AssertExpectations(suite.T())
}

func (suite *UserServiceTestSuite) TestGetUser_Active() {
	ctx := context.Background()
	userID := domain.UserID("user123")

	user := &domain.User{
		ID:        userID,
		Status:    domain.UserStatusActive,
		CreatedAt: time.Now(),
	}

	profile := &domain.Profile{
		UserID:      userID,
		Username:    "testuser",
		DisplayName: "Test User",
	}

	suite.mockRepo.On("GetUser", ctx, userID).Return(user, nil)
	suite.mockRepo.On("GetProfile", ctx, userID).Return(profile, nil)

	// Act
	resultUser, resultProfile, err := suite.userService.GetUser(ctx, userID)

	// Assert
	suite.NoError(err)
	suite.NotNil(resultUser)
	suite.NotNil(resultProfile)
	suite.Equal(user.ID, resultUser.ID)
	suite.Equal(profile.Username, resultProfile.Username)
	suite.mockRepo.AssertExpectations(suite.T())
}

func (suite *UserServiceTestSuite) TestGetUser_Banned() {
	ctx := context.Background()
	userID := domain.UserID("user123")

	user := &domain.User{
		ID:        userID,
		Status:    domain.UserStatusBanned,
		CreatedAt: time.Now(),
	}

	suite.mockRepo.On("GetUser", ctx, userID).Return(user, nil)

	// Act
	_, _, err := suite.userService.GetUser(ctx, userID)

	// Assert
	suite.Error(err)
	suite.Equal(domain.ErrUserBanned, err)
	suite.mockRepo.AssertExpectations(suite.T())
}

func (suite *UserServiceTestSuite) TestUpdateProfile() {
	ctx := context.Background()
	userID := domain.UserID("user123")

	user := &domain.User{
		ID:     userID,
		Status: domain.UserStatusActive,
	}

	profile := &domain.Profile{
		UserID:      userID,
		Username:    "newusername",
		DisplayName: "New Display Name",
		Bio:         "Updated bio",
	}

	suite.mockRepo.On("GetUser", ctx, userID).Return(user, nil)
	suite.mockRepo.On("UpdateProfile", ctx, mock.AnythingOfType("*domain.Profile")).Return(nil)

	// Act
	result, err := suite.userService.UpdateProfile(ctx, profile)

	// Assert
	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal(profile.Username, result.Username)
	suite.mockRepo.AssertExpectations(suite.T())
}

func TestUserServiceTestSuite(t *testing.T) {
	suite.Run(t, new(UserServiceTestSuite))
}

// Additional validation tests
func TestUserService_Validation(t *testing.T) {
	mockRepo := new(MockUserRepository)
	mockPublisher := new(MockUserEventPublisher)
	userService := service.NewUserService(mockRepo, mockPublisher)

	ctx := context.Background()

	t.Run("InvalidAccountID", func(t *testing.T) {
		_, _, err := userService.EnsureUser(ctx, "", "0x123", "eip155:1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "account ID is required")
	})

	t.Run("InvalidAddress", func(t *testing.T) {
		_, _, err := userService.EnsureUser(ctx, "account", "invalid", "eip155:1")
		assert.Error(t, err)
		assert.Equal(t, domain.ErrInvalidAddress, err)
	})

	t.Run("InvalidChainID", func(t *testing.T) {
		_, _, err := userService.EnsureUser(
			ctx, "account", "0x1234567890123456789012345678901234567890", "invalid",
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid chain ID")
	})
}

func TestProfileValidation(t *testing.T) {
	mockRepo := new(MockUserRepository)
	mockPublisher := new(MockUserEventPublisher)
	userService := service.NewUserService(mockRepo, mockPublisher)

	ctx := context.Background()
	userID := domain.UserID("user123")

	user := &domain.User{
		ID:     userID,
		Status: domain.UserStatusActive,
	}

	mockRepo.On("GetUser", ctx, userID).Return(user, nil)

	t.Run("InvalidUsername", func(t *testing.T) {
		profile := &domain.Profile{
			UserID:   userID,
			Username: "ab", // Too short
		}

		_, err := userService.UpdateProfile(ctx, profile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "username must be between 3 and 30 characters")
	})

	t.Run("InvalidUsernameCharacters", func(t *testing.T) {
		profile := &domain.Profile{
			UserID:   userID,
			Username: "user@123", // Invalid characters
		}

		mockRepo.On("GetUser", ctx, userID).Return(user, nil).Once()

		_, err := userService.UpdateProfile(ctx, profile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "username can only contain")
	})

	t.Run("BioTooLong", func(t *testing.T) {
		longBio := make([]byte, 501)
		for i := range longBio {
			longBio[i] = 'a'
		}

		profile := &domain.Profile{
			UserID:   userID,
			Username: "validuser",
			Bio:      string(longBio),
		}

		mockRepo.On("GetUser", ctx, userID).Return(user, nil).Once()

		_, err := userService.UpdateProfile(ctx, profile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bio cannot exceed 500 characters")
	})
}
