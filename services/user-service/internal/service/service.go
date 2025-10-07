package service

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/domain"
)

type Service struct {
	userRepo  domain.UserRepository
	publisher domain.UserEventPublisher
}

func NewUserService(userRepo domain.UserRepository, publisher domain.UserEventPublisher) domain.UserService {
	return &Service{
		userRepo:  userRepo,
		publisher: publisher,
	}
}

func (s *Service) EnsureUser(ctx context.Context, accountID, address, chainID string) (domain.UserID, bool, error) {
	// Validate inputs
	if err := s.validateEnsureUserInputs(accountID, address, chainID); err != nil {
		return "", false, err
	}

	// Normalize address to lowercase
	address = strings.ToLower(address)

	// Check if user already exists
	existingUser, err := s.userRepo.GetUserByAddress(ctx, address)
	if err == nil && existingUser != nil {
		// User already exists, return existing user ID
		log.Printf("User already exists for address %s: %s", address, existingUser.ID)
		return existingUser.ID, false, nil
	}

	// Create new user
	userID := uuid.New().String()
	now := time.Now()

	user := &domain.User{
		ID:        domain.UserID(userID),
		Status:    domain.UserStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		return "", false, fmt.Errorf("failed to create user: %w", err)
	}

	// Create default profile
	profile := &domain.Profile{
		UserID:      domain.UserID(userID),
		Username:    s.generateUsername(address),
		DisplayName: s.formatDisplayName(address),
		Bio:         "",
		Locale:      "en",
		Timezone:    "UTC",
		UpdatedAt:   now,
	}

	if err := s.userRepo.CreateProfile(ctx, profile); err != nil {
		// Log error but don't fail user creation
		log.Printf("Warning: Failed to create default profile for user %s: %v", userID, err)
	}

	// Publish user created event (non-blocking)
	if s.publisher != nil {
		go func() {
			event := &domain.UserCreatedEvent{
				UserID:    domain.UserID(userID),
				AccountID: accountID,
				Address:   address,
				ChainID:   chainID,
				CreatedAt: now,
			}
			if err := s.publisher.PublishUserCreated(context.Background(), event); err != nil {
				log.Printf("Failed to publish user created event: %v", err)
			}
		}()
	}

	log.Printf("Created new user %s for address %s", userID, address)
	return domain.UserID(userID), true, nil
}

func (s *Service) GetUser(ctx context.Context, userID domain.UserID) (*domain.User, *domain.Profile, error) {
	// Validate user ID
	if userID == "" {
		return nil, nil, domain.ErrInvalidUserID
	}

	// Get user
	user, err := s.userRepo.GetUser(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check if user is active
	if user.Status != domain.UserStatusActive {
		if user.Status == domain.UserStatusBanned {
			return nil, nil, domain.ErrUserBanned
		}
		if user.Status == domain.UserStatusDeleted {
			return nil, nil, domain.ErrUserDeleted
		}
	}

	// Get profile (optional, don't fail if not found)
	profile, err := s.userRepo.GetProfile(ctx, userID)
	if err != nil {
		// Log warning but return user without profile
		log.Printf("Warning: Failed to get profile for user %s: %v", userID, err)
		profile = nil
	}

	return user, profile, nil
}

func (s *Service) UpdateProfile(ctx context.Context, profile *domain.Profile) (*domain.Profile, error) {
	// Validate profile
	if profile == nil || profile.UserID == "" {
		return nil, fmt.Errorf("invalid profile: user ID is required")
	}

	// Validate user exists and is active
	user, err := s.userRepo.GetUser(ctx, profile.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if user.Status != domain.UserStatusActive {
		return nil, fmt.Errorf("cannot update profile for inactive user")
	}

	// Sanitize and validate profile fields
	if err := s.validateProfileUpdate(profile); err != nil {
		return nil, fmt.Errorf("invalid profile data: %w", err)
	}

	// Update timestamp
	profile.UpdatedAt = time.Now()

	// Update profile in database
	if err := s.userRepo.UpdateProfile(ctx, profile); err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	return profile, nil
}

// validateEnsureUserInputs validates the input parameters for EnsureUser
func (s *Service) validateEnsureUserInputs(accountID, address, chainID string) error {
	if accountID == "" {
		return fmt.Errorf("account ID is required")
	}

	if address == "" {
		return domain.ErrInvalidAddress
	}

	// Validate Ethereum address format
	address = strings.ToLower(address)
	if !isValidEthereumAddress(address) {
		return domain.ErrInvalidAddress
	}

	// ChainID is optional but if provided, should be valid CAIP-2 format
	if chainID != "" && !isValidCAIP2ChainID(chainID) {
		return fmt.Errorf("invalid chain ID format")
	}

	return nil
}

// validateProfileUpdate validates profile update data
func (s *Service) validateProfileUpdate(profile *domain.Profile) error {
	// Validate username (alphanumeric, underscore, 3-30 chars)
	if profile.Username != "" {
		if len(profile.Username) < 3 || len(profile.Username) > 30 {
			return fmt.Errorf("username must be between 3 and 30 characters")
		}
		if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(profile.Username) {
			return fmt.Errorf("username can only contain letters, numbers, and underscores")
		}
	}

	// Validate display name (max 50 chars)
	if len(profile.DisplayName) > 50 {
		return fmt.Errorf("display name cannot exceed 50 characters")
	}

	// Validate bio (max 500 chars)
	if len(profile.Bio) > 500 {
		return fmt.Errorf("bio cannot exceed 500 characters")
	}

	// Validate URLs
	if profile.AvatarURL != "" && !isValidURL(profile.AvatarURL) {
		return fmt.Errorf("invalid avatar URL")
	}

	if profile.BannerURL != "" && !isValidURL(profile.BannerURL) {
		return fmt.Errorf("invalid banner URL")
	}

	return nil
}

// generateUsername generates a default username from address
func (s *Service) generateUsername(address string) string {
	// Use last 8 characters of address
	if len(address) >= 8 {
		return "user_" + address[len(address)-8:]
	}
	return "user_" + address
}

// formatDisplayName formats a display name from address
func (s *Service) formatDisplayName(address string) string {
	if len(address) > 10 {
		return address[:6] + "..." + address[len(address)-4:]
	}
	return address
}

// isValidEthereumAddress validates Ethereum address format
func isValidEthereumAddress(address string) bool {
	matched, _ := regexp.MatchString(`^0x[0-9a-f]{40}$`, address)
	return matched
}

// isValidCAIP2ChainID validates CAIP-2 chain ID format
func isValidCAIP2ChainID(chainID string) bool {
	matched, _ := regexp.MatchString(`^[a-z0-9]+:[a-zA-Z0-9]+$`, chainID)
	return matched
}

// isValidURL validates URL format
func isValidURL(url string) bool {
	// Simple URL validation - can be enhanced
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}
