package crossref

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ReferenceType defines the type of cross-service reference
type ReferenceType string

const (
	RefTypeUser       ReferenceType = "user"
	RefTypeWallet     ReferenceType = "wallet"
	RefTypeCollection ReferenceType = "collection"
	RefTypeNFT        ReferenceType = "nft"
	RefTypeIntent     ReferenceType = "intent"
)

// CrossServiceValidator validates references across service boundaries
type CrossServiceValidator struct {
	validators map[ReferenceType]Validator
	cache      *ReferenceCache
	mu         sync.RWMutex
}

// Validator interface for specific reference validators
type Validator interface {
	Validate(ctx context.Context, id string) (bool, error)
	ValidateBatch(ctx context.Context, ids []string) (map[string]bool, error)
}

// ReferenceCache caches validation results
type ReferenceCache struct {
	entries map[string]*CacheEntry
	ttl     time.Duration
	mu      sync.RWMutex
}

// CacheEntry represents a cached validation result
type CacheEntry struct {
	Valid      bool
	ValidUntil time.Time
}

// NewCrossServiceValidator creates a new cross-service validator
func NewCrossServiceValidator() *CrossServiceValidator {
	return &CrossServiceValidator{
		validators: make(map[ReferenceType]Validator),
		cache: &ReferenceCache{
			entries: make(map[string]*CacheEntry),
			ttl:     5 * time.Minute,
		},
	}
}

// RegisterValidator registers a validator for a reference type
func (v *CrossServiceValidator) RegisterValidator(refType ReferenceType, validator Validator) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.validators[refType] = validator
}

// ValidateReference validates a cross-service reference
func (v *CrossServiceValidator) ValidateReference(ctx context.Context, refType ReferenceType, id string) error {
	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s", refType, id)
	if valid, found := v.cache.Get(cacheKey); found {
		if !valid {
			return fmt.Errorf("%s with id %s not found", refType, id)
		}
		return nil
	}

	// Get validator
	v.mu.RLock()
	validator, exists := v.validators[refType]
	v.mu.RUnlock()

	if !exists {
		// No validator registered, assume valid (fail open)
		// In production, you might want to fail closed instead
		return nil
	}

	// Validate
	valid, err := validator.Validate(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to validate %s reference: %w", refType, err)
	}

	// Cache result
	v.cache.Set(cacheKey, valid)

	if !valid {
		return fmt.Errorf("%s with id %s not found", refType, id)
	}

	return nil
}

// ValidateReferences validates multiple references
func (v *CrossServiceValidator) ValidateReferences(ctx context.Context, refs map[ReferenceType][]string) error {
	var errors []string

	for refType, ids := range refs {
		for _, id := range ids {
			if err := v.ValidateReference(ctx, refType, id); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("reference validation failed: %v", errors)
	}

	return nil
}

// Get retrieves a cached entry
func (c *ReferenceCache) Get(key string) (bool, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists || time.Now().After(entry.ValidUntil) {
		return false, false
	}

	return entry.Valid, true
}

// Set stores an entry in cache
func (c *ReferenceCache) Set(key string, valid bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &CacheEntry{
		Valid:      valid,
		ValidUntil: time.Now().Add(c.ttl),
	}

	// Simple cleanup
	if len(c.entries) > 10000 {
		c.cleanup()
	}
}

// cleanup removes expired entries
func (c *ReferenceCache) cleanup() {
	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.ValidUntil) {
			delete(c.entries, key)
		}
	}
}

// CompensatingTransaction handles cross-service consistency
type CompensatingTransaction struct {
	steps     []Step
	completed []int
	mu        sync.Mutex
}

// Step represents a step in a compensating transaction
type Step struct {
	Name     string
	Execute  func(ctx context.Context) error
	Rollback func(ctx context.Context) error
}

// NewCompensatingTransaction creates a new compensating transaction
func NewCompensatingTransaction() *CompensatingTransaction {
	return &CompensatingTransaction{
		steps:     []Step{},
		completed: []int{},
	}
}

// AddStep adds a step to the transaction
func (ct *CompensatingTransaction) AddStep(step Step) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.steps = append(ct.steps, step)
}

// Execute runs all steps and rolls back on failure
func (ct *CompensatingTransaction) Execute(ctx context.Context) error {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	// Execute each step
	for i, step := range ct.steps {
		if err := step.Execute(ctx); err != nil {
			// Step failed, rollback completed steps
			rollbackErr := ct.rollback(ctx, i-1)
			if rollbackErr != nil {
				return fmt.Errorf("step '%s' failed: %w, rollback also failed: %v",
					step.Name, err, rollbackErr)
			}
			return fmt.Errorf("step '%s' failed and was rolled back: %w", step.Name, err)
		}
		ct.completed = append(ct.completed, i)
	}

	return nil
}

// rollback rolls back completed steps in reverse order
func (ct *CompensatingTransaction) rollback(ctx context.Context, lastCompleted int) error {
	var errors []string

	// Rollback in reverse order
	for i := lastCompleted; i >= 0; i-- {
		step := ct.steps[i]
		if step.Rollback != nil {
			if err := step.Rollback(ctx); err != nil {
				errors = append(errors, fmt.Sprintf("failed to rollback '%s': %v", step.Name, err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("rollback errors: %v", errors)
	}

	return nil
}
