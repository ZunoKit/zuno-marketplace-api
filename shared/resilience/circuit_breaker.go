package resilience

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// State represents the circuit breaker state
type State int32

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name             string
	maxFailures      uint32
	resetTimeout     time.Duration
	halfOpenMaxCalls uint32

	state           int32  // atomic
	failures        uint32 // atomic
	lastFailureTime int64  // atomic (Unix timestamp)
	halfOpenCalls   uint32 // atomic

	mu              sync.RWMutex
	successCount    uint64
	failureCount    uint64
	lastStateChange time.Time
	onStateChange   func(name string, from, to State)
}

// CircuitBreakerConfig holds configuration for a circuit breaker
type CircuitBreakerConfig struct {
	Name             string
	MaxFailures      uint32
	ResetTimeout     time.Duration
	HalfOpenMaxCalls uint32
	OnStateChange    func(name string, from, to State)
}

// DefaultCircuitBreakerConfig returns default circuit breaker configuration
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		Name:             "default",
		MaxFailures:      5,
		ResetTimeout:     60 * time.Second,
		HalfOpenMaxCalls: 3,
		OnStateChange:    nil,
	}
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	cb := &CircuitBreaker{
		name:             config.Name,
		maxFailures:      config.MaxFailures,
		resetTimeout:     config.ResetTimeout,
		halfOpenMaxCalls: config.HalfOpenMaxCalls,
		state:            int32(StateClosed),
		lastStateChange:  time.Now(),
		onStateChange:    config.OnStateChange,
	}

	return cb
}

// Execute runs the given function if the circuit breaker allows it
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	// Check if we can execute
	if !cb.canExecute() {
		return fmt.Errorf("circuit breaker '%s' is OPEN", cb.name)
	}

	// Execute the function
	err := fn(ctx)

	// Record result
	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}

	return err
}

// canExecute checks if execution is allowed based on current state
func (cb *CircuitBreaker) canExecute() bool {
	state := cb.GetState()

	switch state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if enough time has passed to try half-open
		lastFailure := time.Unix(atomic.LoadInt64(&cb.lastFailureTime), 0)
		if time.Since(lastFailure) > cb.resetTimeout {
			cb.transitionTo(StateHalfOpen)
			return true
		}
		return false

	case StateHalfOpen:
		// Allow limited calls in half-open state
		calls := atomic.LoadUint32(&cb.halfOpenCalls)
		if calls < cb.halfOpenMaxCalls {
			atomic.AddUint32(&cb.halfOpenCalls, 1)
			return true
		}
		return false

	default:
		return false
	}
}

// recordSuccess records a successful execution
func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	cb.successCount++
	cb.mu.Unlock()

	state := cb.GetState()

	switch state {
	case StateHalfOpen:
		// Successful call in half-open state, check if we should close
		calls := atomic.LoadUint32(&cb.halfOpenCalls)
		if calls >= cb.halfOpenMaxCalls {
			// All half-open calls succeeded, close the circuit
			cb.transitionTo(StateClosed)
		}

	case StateClosed:
		// Reset failure count on success in closed state
		atomic.StoreUint32(&cb.failures, 0)
	}
}

// recordFailure records a failed execution
func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	cb.failureCount++
	cb.mu.Unlock()

	atomic.StoreInt64(&cb.lastFailureTime, time.Now().Unix())
	failures := atomic.AddUint32(&cb.failures, 1)

	state := cb.GetState()

	switch state {
	case StateClosed:
		if failures >= cb.maxFailures {
			cb.transitionTo(StateOpen)
		}

	case StateHalfOpen:
		// Any failure in half-open state opens the circuit
		cb.transitionTo(StateOpen)
	}
}

// transitionTo changes the circuit breaker state
func (cb *CircuitBreaker) transitionTo(newState State) {
	oldState := State(atomic.SwapInt32(&cb.state, int32(newState)))

	if oldState != newState {
		cb.mu.Lock()
		cb.lastStateChange = time.Now()
		cb.mu.Unlock()

		// Reset counters based on new state
		switch newState {
		case StateClosed:
			atomic.StoreUint32(&cb.failures, 0)
			atomic.StoreUint32(&cb.halfOpenCalls, 0)

		case StateHalfOpen:
			atomic.StoreUint32(&cb.halfOpenCalls, 0)

		case StateOpen:
			atomic.StoreUint32(&cb.halfOpenCalls, 0)
		}

		// Notify state change
		if cb.onStateChange != nil {
			cb.onStateChange(cb.name, oldState, newState)
		}
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() State {
	return State(atomic.LoadInt32(&cb.state))
}

// GetStats returns statistics about the circuit breaker
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		Name:            cb.name,
		State:           cb.GetState(),
		Failures:        atomic.LoadUint32(&cb.failures),
		SuccessCount:    cb.successCount,
		FailureCount:    cb.failureCount,
		LastStateChange: cb.lastStateChange,
		LastFailureTime: time.Unix(atomic.LoadInt64(&cb.lastFailureTime), 0),
	}
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.transitionTo(StateClosed)
}

// CircuitBreakerStats holds statistics for a circuit breaker
type CircuitBreakerStats struct {
	Name            string
	State           State
	Failures        uint32
	SuccessCount    uint64
	FailureCount    uint64
	LastStateChange time.Time
	LastFailureTime time.Time
}

// CircuitBreakerGroup manages multiple circuit breakers
type CircuitBreakerGroup struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

// NewCircuitBreakerGroup creates a new circuit breaker group
func NewCircuitBreakerGroup() *CircuitBreakerGroup {
	return &CircuitBreakerGroup{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// Get returns a circuit breaker by name, creating it if it doesn't exist
func (g *CircuitBreakerGroup) Get(name string) *CircuitBreaker {
	g.mu.RLock()
	cb, exists := g.breakers[name]
	g.mu.RUnlock()

	if exists {
		return cb
	}

	// Create new circuit breaker
	g.mu.Lock()
	defer g.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists := g.breakers[name]; exists {
		return cb
	}

	config := DefaultCircuitBreakerConfig()
	config.Name = name
	cb = NewCircuitBreaker(config)
	g.breakers[name] = cb

	return cb
}

// GetWithConfig returns a circuit breaker by name with custom config
func (g *CircuitBreakerGroup) GetWithConfig(name string, config *CircuitBreakerConfig) *CircuitBreaker {
	g.mu.Lock()
	defer g.mu.Unlock()

	if cb, exists := g.breakers[name]; exists {
		return cb
	}

	config.Name = name
	cb := NewCircuitBreaker(config)
	g.breakers[name] = cb

	return cb
}

// GetAllStats returns statistics for all circuit breakers
func (g *CircuitBreakerGroup) GetAllStats() []CircuitBreakerStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := make([]CircuitBreakerStats, 0, len(g.breakers))
	for _, cb := range g.breakers {
		stats = append(stats, cb.GetStats())
	}

	return stats
}

// ResetAll resets all circuit breakers to closed state
func (g *CircuitBreakerGroup) ResetAll() {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, cb := range g.breakers {
		cb.Reset()
	}
}
