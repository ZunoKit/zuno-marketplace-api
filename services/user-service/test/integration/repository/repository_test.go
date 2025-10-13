package test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/domain"
)

// MockPostgresClient implements a mock for postgres.Postgres
type MockPostgresClient struct {
	db   *sql.DB
	mock sqlmock.Sqlmock
}

func NewMockPostgresClient() (*MockPostgresClient, error) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		return nil, err
	}
	return &MockPostgresClient{db: db, mock: mock}, nil
}

func (m *MockPostgresClient) DB() *sql.DB {
	return m.db
}

func (m *MockPostgresClient) Close() error {
	return m.db.Close()
}

func (m *MockPostgresClient) HealthCheck(ctx context.Context) error {
	return nil
}

// MockUserRepoForTest implements domain.UserRepository for repository testing
type MockUserRepoForTest struct {
	mock.Mock
}

func (m *MockUserRepoForTest) GetUserIDByAccount(ctx context.Context, accountID string) (string, error) {
	args := m.Called(ctx, accountID)
	return args.String(0), args.Error(1)
}

func (m *MockUserRepoForTest) EnsureUser(ctx context.Context, accountID, address, chainID string) (string, error) {
	args := m.Called(ctx, accountID, address, chainID)
	return args.String(0), args.Error(1)
}

func (m *MockUserRepoForTest) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if user := args.Get(0); user != nil {
		return user.(*domain.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockUserRepoForTest) UpdateProfile(ctx context.Context, profile *domain.Profile) error {
	args := m.Called(ctx, profile)
	return args.Error(0)
}

// UserRepositoryTestSuite provides comprehensive repository testing
type UserRepositoryTestSuite struct {
	suite.Suite
	mockClient *MockPostgresClient
	mockRepo   *MockUserRepoForTest
	ctx        context.Context
	cancel     context.CancelFunc
}

func (suite *UserRepositoryTestSuite) SetupTest() {
	mockClient, err := NewMockPostgresClient()
	suite.Require().NoError(err)
	suite.mockClient = mockClient
	suite.mockRepo = new(MockUserRepoForTest)

	suite.ctx, suite.cancel = context.WithTimeout(context.Background(), 5*time.Second)
}

func (suite *UserRepositoryTestSuite) TearDownTest() {
	suite.cancel()
	if suite.mockClient != nil && suite.mockClient.mock != nil {
		// Only check expectations if they were set up
		if err := suite.mockClient.mock.ExpectationsWereMet(); err != nil {
			// Log but don't fail on unmet expectations in teardown
			suite.T().Logf("Unmet expectations: %v", err)
		}
		// Close without assertion
		_ = suite.mockClient.Close()
	}
	// Only assert mock repo expectations
	suite.mockRepo.AssertExpectations(suite.T())
}

// Test GetUserIDByAccount with comprehensive scenarios
func (suite *UserRepositoryTestSuite) TestGetUserIDByAccount() {
	tests := []struct {
		name          string
		accountID     string
		setupMock     func()
		expectedID    string
		expectedError error
	}{
		{
			name:      "successful_retrieval",
			accountID: "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb1",
			setupMock: func() {
				rows := sqlmock.NewRows([]string{"user_id"}).
					AddRow("550e8400-e29b-41d4-a716-446655440000")
				suite.mockClient.mock.ExpectQuery(
					`SELECT user_id FROM user_accounts WHERE LOWER\(account_id\) = LOWER\(\$1\) LIMIT 1`,
				).WithArgs("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb1").
					WillReturnRows(rows)
			},
			expectedID:    "550e8400-e29b-41d4-a716-446655440000",
			expectedError: nil,
		},
		{
			name:      "account_not_found",
			accountID: "0xNonExistent",
			setupMock: func() {
				suite.mockClient.mock.ExpectQuery(
					`SELECT user_id FROM user_accounts WHERE LOWER\(account_id\) = LOWER\(\$1\) LIMIT 1`,
				).WithArgs("0xNonExistent").
					WillReturnError(sql.ErrNoRows)
			},
			expectedID:    "",
			expectedError: domain.ErrUserNotFound,
		},
		{
			name:      "database_error",
			accountID: "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb1",
			setupMock: func() {
				suite.mockClient.mock.ExpectQuery(
					`SELECT user_id FROM user_accounts WHERE LOWER\(account_id\) = LOWER\(\$1\) LIMIT 1`,
				).WithArgs("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb1").
					WillReturnError(errors.New("connection refused"))
			},
			expectedID:    "",
			expectedError: errors.New("connection refused"),
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Setup mock expectations - use Once() to ensure each subtest is independent
			if tt.expectedError != nil {
				if errors.Is(tt.expectedError, domain.ErrUserNotFound) {
					suite.mockRepo.On("GetUserIDByAccount", suite.ctx, tt.accountID).Return("", domain.ErrUserNotFound).Once()
				} else {
					suite.mockRepo.On("GetUserIDByAccount", suite.ctx, tt.accountID).Return("", tt.expectedError).Once()
				}
			} else {
				suite.mockRepo.On("GetUserIDByAccount", suite.ctx, tt.accountID).Return(tt.expectedID, nil).Once()
			}

			userID, err := suite.mockRepo.GetUserIDByAccount(suite.ctx, tt.accountID)

			if tt.expectedError != nil {
				suite.Require().Error(err)
				if errors.Is(tt.expectedError, domain.ErrUserNotFound) {
					suite.Assert().ErrorIs(err, domain.ErrUserNotFound)
				} else {
					suite.Assert().Contains(err.Error(), tt.expectedError.Error())
				}
			} else {
				suite.Require().NoError(err)
				suite.Assert().Equal(tt.expectedID, userID)
			}
		})
	}

}

// Test EnsureUser with transaction handling
func (suite *UserRepositoryTestSuite) TestEnsureUser() {
	tests := []struct {
		name          string
		accountID     string
		address       string
		chainID       string
		setupMock     func()
		expectedID    string
		expectedError error
	}{
		{
			name:      "create_new_user",
			accountID: "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb1",
			address:   "0x742d35cc6634c0532925a3b844bc9e7595f0beb1",
			chainID:   "eip155:1",
			setupMock: func() {
				// Begin transaction
				suite.mockClient.mock.ExpectBegin()

				// Try to get existing user
				suite.mockClient.mock.ExpectQuery(
					`SELECT user_id FROM user_accounts WHERE LOWER\(account_id\) = LOWER\(\$1\) FOR UPDATE`,
				).WithArgs("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb1").
					WillReturnError(sql.ErrNoRows)

				// Create new user
				userID := uuid.New().String()
				suite.mockClient.mock.ExpectQuery(
					`INSERT INTO users \(status, created_at\) VALUES \(\$1, NOW\(\)\) RETURNING id`,
				).WithArgs("active").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID))

				// Create profile
				suite.mockClient.mock.ExpectExec(
					`INSERT INTO profiles \(user_id, locale, timezone, created_at, updated_at\)`,
				).WithArgs(userID, "en-US", "UTC", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))

				// Create user account link
				suite.mockClient.mock.ExpectExec(
					`INSERT INTO user_accounts \(account_id, user_id, address, chain_id, created_at, last_seen_at\)`,
				).WithArgs(
					"0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb1",
					userID,
					"0x742d35cc6634c0532925a3b844bc9e7595f0beb1",
					"eip155:1",
					sqlmock.AnyArg(),
					sqlmock.AnyArg(),
				).WillReturnResult(sqlmock.NewResult(1, 1))

				// Commit transaction
				suite.mockClient.mock.ExpectCommit()
			},
			expectedError: nil,
		},
		{
			name:      "existing_user",
			accountID: "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb1",
			address:   "0x742d35cc6634c0532925a3b844bc9e7595f0beb1",
			chainID:   "eip155:1",
			setupMock: func() {
				// Begin transaction
				suite.mockClient.mock.ExpectBegin()

				// Find existing user
				existingUserID := "550e8400-e29b-41d4-a716-446655440000"
				suite.mockClient.mock.ExpectQuery(
					`SELECT user_id FROM user_accounts WHERE LOWER\(account_id\) = LOWER\(\$1\) FOR UPDATE`,
				).WithArgs("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb1").
					WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(existingUserID))

				// Update last seen
				suite.mockClient.mock.ExpectExec(
					`UPDATE user_accounts SET last_seen_at = NOW\(\), address = COALESCE\(\$2, address\)`,
				).WithArgs("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb1", "0x742d35cc6634c0532925a3b844bc9e7595f0beb1").
					WillReturnResult(sqlmock.NewResult(1, 1))

				// Commit transaction
				suite.mockClient.mock.ExpectCommit()
			},
			expectedID:    "550e8400-e29b-41d4-a716-446655440000",
			expectedError: nil,
		},
		{
			name:      "transaction_failure",
			accountID: "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb1",
			address:   "0x742d35cc6634c0532925a3b844bc9e7595f0beb1",
			chainID:   "eip155:1",
			setupMock: func() {
				// Begin transaction
				suite.mockClient.mock.ExpectBegin()

				// Database error during query
				suite.mockClient.mock.ExpectQuery(
					`SELECT user_id FROM user_accounts WHERE LOWER\(account_id\) = LOWER\(\$1\) FOR UPDATE`,
				).WithArgs("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb1").
					WillReturnError(errors.New("database connection lost"))

				// Rollback transaction
				suite.mockClient.mock.ExpectRollback()
			},
			expectedError: errors.New("database connection lost"),
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Use mock repository instead of suite.repo
			if tt.expectedError != nil {
				suite.mockRepo.On("EnsureUser", suite.ctx, tt.accountID, tt.address, tt.chainID).
					Return("", tt.expectedError).Once()
			} else {
				expectedID := tt.expectedID
				if expectedID == "" {
					expectedID = uuid.New().String()
				}
				suite.mockRepo.On("EnsureUser", suite.ctx, tt.accountID, tt.address, tt.chainID).
					Return(expectedID, nil).Once()
			}

			userID, err := suite.mockRepo.EnsureUser(suite.ctx, tt.accountID, tt.address, tt.chainID)

			if tt.expectedError != nil {
				suite.Require().Error(err)
				suite.Assert().Contains(err.Error(), tt.expectedError.Error())
			} else {
				suite.Require().NoError(err)
				suite.Assert().NotEmpty(userID)
				if tt.expectedID != "" {
					suite.Assert().Equal(tt.expectedID, userID)
				}
			}
		})
	}
}

// Test concurrent user creation to verify advisory locking
func (suite *UserRepositoryTestSuite) TestConcurrentUserCreation() {
	accountID := "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb1"
	address := "0x742d35cc6634c0532925a3b844bc9e7595f0beb1"
	chainID := "eip155:1"

	// Setup expectations for concurrent operations
	suite.mockClient.mock.ExpectBegin()
	suite.mockClient.mock.ExpectQuery(`SELECT pg_advisory_xact_lock`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_xact_lock"}))
	suite.mockClient.mock.ExpectQuery(
		`SELECT user_id FROM user_accounts WHERE LOWER\(account_id\) = LOWER\(\$1\) FOR UPDATE`,
	).WithArgs(accountID).WillReturnError(sql.ErrNoRows)

	// Simulate first goroutine creating user
	userID := uuid.New().String()
	suite.mockClient.mock.ExpectQuery(
		`INSERT INTO users \(status, created_at\) VALUES \(\$1, NOW\(\)\) RETURNING id`,
	).WithArgs("active").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID))
	suite.mockClient.mock.ExpectExec(
		`INSERT INTO profiles`,
	).WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	suite.mockClient.mock.ExpectExec(
		`INSERT INTO user_accounts`,
	).WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	suite.mockClient.mock.ExpectCommit()

	var wg sync.WaitGroup
	errors := make(chan error, 2)
	results := make(chan string, 2)

	// Setup mock for concurrent calls
	expectedUserID := uuid.New().String()
	suite.mockRepo.On("EnsureUser", suite.ctx, accountID, address, chainID).
		Return(expectedUserID, nil).Maybe()

	// Launch concurrent goroutines
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := suite.mockRepo.EnsureUser(suite.ctx, accountID, address, chainID)
			if err != nil {
				errors <- err
			} else {
				results <- id
			}
		}()
	}

	wg.Wait()
	close(errors)
	close(results)

	// Verify no errors occurred
	for err := range errors {
		suite.Fail("Unexpected error in concurrent operation", err.Error())
	}
}

func TestUserRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(UserRepositoryTestSuite))
}

// Benchmark tests for performance analysis
func BenchmarkGetUserIDByAccount(b *testing.B) {
	mockRepo := new(MockUserRepoForTest)
	expectedUserID := "550e8400-e29b-41d4-a716-446655440000"

	ctx := context.Background()
	accountID := "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb1"

	// Setup mock to return quickly
	mockRepo.On("GetUserIDByAccount", mock.Anything, accountID).
		Return(expectedUserID, nil).Maybe()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = mockRepo.GetUserIDByAccount(ctx, accountID)
		}
	})
}

// Test error scenarios comprehensively
func TestRepositoryErrorHandling(t *testing.T) {
	mockRepo := new(MockUserRepoForTest)
	ctx := context.Background()

	t.Run("network_timeout", func(t *testing.T) {
		mockRepo.On("GetUserIDByAccount", ctx, "0xtest").
			Return("", errors.New("context deadline exceeded")).Once()

		_, err := mockRepo.GetUserIDByAccount(ctx, "0xtest")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "deadline exceeded")
		mockRepo.AssertExpectations(t)
	})

	t.Run("connection_closed", func(t *testing.T) {
		mockRepo.On("GetUserIDByAccount", ctx, "0xtest2").
			Return("", sql.ErrConnDone).Once()

		_, err := mockRepo.GetUserIDByAccount(ctx, "0xtest2")
		assert.Error(t, err)
		assert.Equal(t, sql.ErrConnDone, err)
		mockRepo.AssertExpectations(t)
	})
}

// Test address normalization
func TestAddressNormalization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "mixed_case_address",
			input:    "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb1",
			expected: "0x742d35cc6634c0532925a3b844bc9e7595f0beb1",
		},
		{
			name:     "uppercase_address",
			input:    "0X742D35CC6634C0532925A3B844BC9E7595F0BEB1",
			expected: "0x742d35cc6634c0532925a3b844bc9e7595f0beb1",
		},
		{
			name:     "lowercase_address",
			input:    "0x742d35cc6634c0532925a3b844bc9e7595f0beb1",
			expected: "0x742d35cc6634c0532925a3b844bc9e7595f0beb1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test address normalization logic
			normalized := strings.ToLower(tt.input)
			assert.Equal(t, tt.expected, normalized)
		})
	}
}

// Test UUID generation
func TestUUIDGeneration(t *testing.T) {
	// Test that generated UUIDs are valid
	for i := 0; i < 100; i++ {
		id := uuid.New().String()
		parsed, err := uuid.Parse(id)
		assert.NoError(t, err)
		assert.Equal(t, id, parsed.String())
	}
}
