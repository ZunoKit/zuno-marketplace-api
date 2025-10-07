package websocket

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// AuthConfig contains authentication configuration for WebSocket connections
type AuthConfig struct {
	// JWT token for authentication
	Token string
	// API key for authentication (alternative to JWT)
	APIKey string
	// Custom headers to add to the WebSocket connection
	Headers map[string]string
}

// AuthenticatedClient extends Client with authentication support
type AuthenticatedClient struct {
	*Client
	authConfig    *AuthConfig
	authenticated bool
	authMu        sync.RWMutex
}

// NewAuthenticatedClient creates a new authenticated WebSocket client
func NewAuthenticatedClient(subscriptionWorkerURL string, authConfig *AuthConfig) *AuthenticatedClient {
	baseClient := NewClient(subscriptionWorkerURL)

	return &AuthenticatedClient{
		Client:     baseClient,
		authConfig: authConfig,
	}
}

// ConnectWithAuth establishes an authenticated WebSocket connection
func (c *AuthenticatedClient) ConnectWithAuth() error {
	u, err := url.Parse(c.url)
	if err != nil {
		return fmt.Errorf("invalid WebSocket URL: %w", err)
	}

	log.Printf("Connecting to subscription worker WebSocket with authentication: %s", u.String())

	// Prepare headers for authentication
	headers := http.Header{}

	// Add JWT token if provided
	if c.authConfig != nil && c.authConfig.Token != "" {
		headers.Set("Authorization", "Bearer "+c.authConfig.Token)
	}

	// Add API key if provided
	if c.authConfig != nil && c.authConfig.APIKey != "" {
		headers.Set("X-API-Key", c.authConfig.APIKey)
	}

	// Add custom headers
	if c.authConfig != nil && c.authConfig.Headers != nil {
		for key, value := range c.authConfig.Headers {
			headers.Set(key, value)
		}
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, resp, err := dialer.Dial(u.String(), headers)
	if err != nil {
		if resp != nil {
			switch resp.StatusCode {
			case http.StatusUnauthorized:
				return fmt.Errorf("authentication failed: unauthorized")
			case http.StatusForbidden:
				return fmt.Errorf("authentication failed: forbidden")
			default:
				return fmt.Errorf("failed to connect to WebSocket: %w (status: %d)", err, resp.StatusCode)
			}
		}
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.isConnected = true
	c.reconnectDelay = 1 * time.Second
	c.mu.Unlock()

	c.authMu.Lock()
	c.authenticated = true
	c.authMu.Unlock()

	log.Println("Successfully connected to subscription worker WebSocket with authentication")

	// Send authentication message if required by the server
	if err := c.sendAuthMessage(); err != nil {
		log.Printf("Failed to send authentication message: %v", err)
		// Some servers don't require explicit auth message after header-based auth
	}

	// Start message handling
	go c.handleAuthenticatedMessages()
	go c.maintainAuthenticatedConnection()

	return nil
}

// sendAuthMessage sends an authentication message to the WebSocket server
func (c *AuthenticatedClient) sendAuthMessage() error {
	if c.authConfig == nil {
		return nil
	}

	authMsg := map[string]interface{}{
		"type": "auth",
	}

	if c.authConfig.Token != "" {
		authMsg["token"] = c.authConfig.Token
	}

	if c.authConfig.APIKey != "" {
		authMsg["api_key"] = c.authConfig.APIKey
	}

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn != nil {
		return conn.WriteJSON(authMsg)
	}

	return fmt.Errorf("no connection available")
}

// handleAuthenticatedMessages processes incoming messages with auth awareness
func (c *AuthenticatedClient) handleAuthenticatedMessages() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in handleAuthenticatedMessages: %v", r)
		}
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.mu.RLock()
			conn := c.conn
			connected := c.isConnected
			c.mu.RUnlock()

			if !connected || conn == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			var msg WebSocketMessage
			err := conn.ReadJSON(&msg)
			if err != nil {
				log.Printf("Error reading WebSocket message: %v", err)
				c.handleAuthDisconnection()
				return
			}

			// Handle auth-specific message types
			switch msg.Type {
			case "auth_success":
				c.handleAuthSuccess()
			case "auth_failed":
				c.handleAuthFailure(msg.Error)
			case "auth_required":
				c.handleAuthRequired()
			default:
				// Process regular messages
				c.processMessage(&msg)
			}
		}
	}
}

// handleAuthSuccess handles successful authentication
func (c *AuthenticatedClient) handleAuthSuccess() {
	c.authMu.Lock()
	c.authenticated = true
	c.authMu.Unlock()

	log.Println("WebSocket authentication successful")

	// Re-subscribe to all active subscriptions
	c.resubscribeAll()
}

// handleAuthFailure handles authentication failure
func (c *AuthenticatedClient) handleAuthFailure(errorMsg string) {
	c.authMu.Lock()
	c.authenticated = false
	c.authMu.Unlock()

	log.Printf("WebSocket authentication failed: %s", errorMsg)

	// Close connection and trigger reconnection
	c.handleAuthDisconnection()
}

// handleAuthRequired handles authentication requirement from server
func (c *AuthenticatedClient) handleAuthRequired() {
	log.Println("Server requires authentication, sending auth message")

	if err := c.sendAuthMessage(); err != nil {
		log.Printf("Failed to send authentication message: %v", err)
		c.handleAuthDisconnection()
	}
}

// handleAuthDisconnection handles disconnection with authentication awareness
func (c *AuthenticatedClient) handleAuthDisconnection() {
	c.authMu.Lock()
	c.authenticated = false
	c.authMu.Unlock()

	c.handleDisconnection()
}

// maintainAuthenticatedConnection maintains the connection with authentication
func (c *AuthenticatedClient) maintainAuthenticatedConnection() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mu.RLock()
			connected := c.isConnected
			conn := c.conn
			c.mu.RUnlock()

			if connected && conn != nil {
				// Send authenticated ping
				pingMsg := map[string]interface{}{
					"type": "ping",
				}

				// Add auth token to ping if needed
				if c.authConfig != nil && c.authConfig.Token != "" {
					pingMsg["token"] = c.authConfig.Token
				}

				if err := conn.WriteJSON(pingMsg); err != nil {
					log.Printf("Failed to send authenticated ping: %v", err)
					c.handleAuthDisconnection()
				}
			} else {
				// Try to reconnect with authentication
				log.Println("Connection lost, attempting to reconnect with authentication...")
				if err := c.reconnectWithAuth(); err != nil {
					log.Printf("Failed to reconnect with authentication: %v", err)
				}
			}
		}
	}
}

// reconnectWithAuth attempts to reconnect with authentication
func (c *AuthenticatedClient) reconnectWithAuth() error {
	c.mu.Lock()
	if c.isConnected {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	// Exponential backoff for reconnection
	time.Sleep(c.reconnectDelay)

	if err := c.ConnectWithAuth(); err != nil {
		// Increase reconnect delay with exponential backoff
		c.reconnectDelay = c.reconnectDelay * 2
		if c.reconnectDelay > c.maxReconnectDelay {
			c.reconnectDelay = c.maxReconnectDelay
		}
		return err
	}

	return nil
}

// resubscribeAll re-subscribes to all active subscriptions after reconnection
func (c *AuthenticatedClient) resubscribeAll() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for intentID := range c.subscriptions {
		subscribeMsg := map[string]interface{}{
			"type":      "subscribe",
			"intent_id": intentID,
		}

		// Add auth token if needed
		if c.authConfig != nil && c.authConfig.Token != "" {
			subscribeMsg["token"] = c.authConfig.Token
		}

		if c.conn != nil {
			if err := c.conn.WriteJSON(subscribeMsg); err != nil {
				log.Printf("Failed to re-subscribe to intent %s: %v", intentID, err)
			} else {
				log.Printf("Re-subscribed to intent updates: %s", intentID)
			}
		}
	}
}

// IsAuthenticated returns whether the client is authenticated
func (c *AuthenticatedClient) IsAuthenticated() bool {
	c.authMu.RLock()
	defer c.authMu.RUnlock()
	return c.authenticated
}

// UpdateAuth updates the authentication configuration
func (c *AuthenticatedClient) UpdateAuth(authConfig *AuthConfig) error {
	c.authConfig = authConfig

	// If connected, send new auth message
	c.mu.RLock()
	connected := c.isConnected
	c.mu.RUnlock()

	if connected {
		return c.sendAuthMessage()
	}

	return nil
}

// SubscribeWithContext subscribes with a context that includes authentication
func (c *AuthenticatedClient) SubscribeWithContext(ctx context.Context, intentID string, callback SubscriptionCallback) error {
	// Extract auth token from context if available
	if token, ok := ctx.Value("auth_token").(string); ok {
		if c.authConfig == nil {
			c.authConfig = &AuthConfig{}
		}
		c.authConfig.Token = token
	}

	return c.Subscribe(intentID, callback)
}
