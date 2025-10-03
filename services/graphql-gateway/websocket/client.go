package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketMessage represents a message from the subscription worker
type WebSocketMessage struct {
	Type      string      `json:"type"`
	IntentID  string      `json:"intent_id,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Error     string      `json:"error,omitempty"`
}

// IntentStatusData represents intent status data in WebSocket messages
type IntentStatusData struct {
	IntentID        string      `json:"intent_id"`
	Status          string      `json:"status"`
	ChainID         string      `json:"chain_id,omitempty"`
	TxHash          string      `json:"tx_hash,omitempty"`
	ContractAddress string      `json:"contract_address,omitempty"`
	Data            interface{} `json:"data,omitempty"`
}

// SubscriptionCallback is called when a message is received for a subscribed intent
type SubscriptionCallback func(intentID string, data *IntentStatusData) error

// Client manages WebSocket connections to the subscription worker service
type Client struct {
	url               string
	conn              *websocket.Conn
	subscriptions     map[string][]SubscriptionCallback
	mu                sync.RWMutex
	reconnectInterval time.Duration
	maxReconnectDelay time.Duration
	isConnected       bool
	ctx               context.Context
	cancel            context.CancelFunc
	reconnectDelay    time.Duration
}

// NewClient creates a new WebSocket client
func NewClient(subscriptionWorkerURL string) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Client{
		url:               subscriptionWorkerURL,
		subscriptions:     make(map[string][]SubscriptionCallback),
		reconnectInterval: 5 * time.Second,
		maxReconnectDelay: 60 * time.Second,
		reconnectDelay:    1 * time.Second,
		ctx:               ctx,
		cancel:            cancel,
	}
}

// Connect establishes a WebSocket connection to the subscription worker
func (c *Client) Connect() error {
	u, err := url.Parse(c.url)
	if err != nil {
		return fmt.Errorf("invalid WebSocket URL: %w", err)
	}

	log.Printf("Connecting to subscription worker WebSocket: %s", u.String())

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.isConnected = true
	c.reconnectDelay = 1 * time.Second // Reset reconnect delay on successful connection
	c.mu.Unlock()

	log.Println("Successfully connected to subscription worker WebSocket")

	// Start message handling
	go c.handleMessages()
	go c.maintainConnection()

	return nil
}

// Subscribe subscribes to intent status updates for a specific intent ID
func (c *Client) Subscribe(intentID string, callback SubscriptionCallback) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.subscriptions[intentID] == nil {
		c.subscriptions[intentID] = make([]SubscriptionCallback, 0)
	}
	c.subscriptions[intentID] = append(c.subscriptions[intentID], callback)

	// Send subscription message if connected
	if c.isConnected && c.conn != nil {
		subscribeMsg := map[string]string{
			"type":      "subscribe",
			"intent_id": intentID,
		}
		
		if err := c.conn.WriteJSON(subscribeMsg); err != nil {
			log.Printf("Failed to send subscribe message for intent %s: %v", intentID, err)
			return err
		}
		
		log.Printf("Subscribed to intent updates: %s", intentID)
	}

	return nil
}

// Unsubscribe removes all subscriptions for an intent ID
func (c *Client) Unsubscribe(intentID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.subscriptions, intentID)

	// Send unsubscribe message if connected
	if c.isConnected && c.conn != nil {
		unsubscribeMsg := map[string]string{
			"type":      "unsubscribe",
			"intent_id": intentID,
		}
		
		if err := c.conn.WriteJSON(unsubscribeMsg); err != nil {
			log.Printf("Failed to send unsubscribe message for intent %s: %v", intentID, err)
			return err
		}
		
		log.Printf("Unsubscribed from intent updates: %s", intentID)
	}

	return nil
}

// handleMessages processes incoming WebSocket messages
func (c *Client) handleMessages() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in handleMessages: %v", r)
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
				c.handleDisconnection()
				return
			}

			c.processMessage(&msg)
		}
	}
}

// processMessage processes a received WebSocket message
func (c *Client) processMessage(msg *WebSocketMessage) {
	switch msg.Type {
	case "status_update":
		c.handleStatusUpdate(msg)
	case "subscribed":
		log.Printf("Subscription confirmed for intent: %s", msg.IntentID)
	case "unsubscribed":
		log.Printf("Unsubscription confirmed for intent: %s", msg.IntentID)
	case "pong":
		// Heartbeat response, no action needed
	case "error":
		log.Printf("WebSocket error for intent %s: %s", msg.IntentID, msg.Error)
	default:
		log.Printf("Unknown message type received: %s", msg.Type)
	}
}

// handleStatusUpdate processes status update messages
func (c *Client) handleStatusUpdate(msg *WebSocketMessage) {
	if msg.IntentID == "" {
		log.Printf("Received status update without intent ID")
		return
	}

	// Convert msg.Data to IntentStatusData
	var statusData IntentStatusData
	if dataBytes, err := json.Marshal(msg.Data); err == nil {
		if err := json.Unmarshal(dataBytes, &statusData); err != nil {
			log.Printf("Failed to parse status data for intent %s: %v", msg.IntentID, err)
			return
		}
	} else {
		log.Printf("Failed to marshal status data for intent %s: %v", msg.IntentID, err)
		return
	}

	// Call all registered callbacks for this intent
	c.mu.RLock()
	callbacks := c.subscriptions[msg.IntentID]
	c.mu.RUnlock()

	for _, callback := range callbacks {
		if err := callback(msg.IntentID, &statusData); err != nil {
			log.Printf("Callback error for intent %s: %v", msg.IntentID, err)
		}
	}
}

// maintainConnection handles reconnection logic
func (c *Client) maintainConnection() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mu.RLock()
			connected := c.isConnected
			c.mu.RUnlock()

			if connected {
				// Send ping to check connection health
				c.ping()
			} else {
				// Attempt to reconnect
				c.reconnect()
			}
		}
	}
}

// ping sends a ping message to check connection health
func (c *Client) ping() {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn != nil {
		pingMsg := map[string]string{
			"type": "ping",
		}
		
		if err := conn.WriteJSON(pingMsg); err != nil {
			log.Printf("Failed to send ping: %v", err)
			c.handleDisconnection()
		}
	}
}

// handleDisconnection handles connection loss
func (c *Client) handleDisconnection() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isConnected {
		c.isConnected = false
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		log.Println("WebSocket connection lost")
	}
}

// reconnect attempts to reconnect to the WebSocket server
func (c *Client) reconnect() {
	c.mu.Lock()
	reconnectDelay := c.reconnectDelay
	if reconnectDelay < c.maxReconnectDelay {
		c.reconnectDelay = reconnectDelay * 2
		if c.reconnectDelay > c.maxReconnectDelay {
			c.reconnectDelay = c.maxReconnectDelay
		}
	}
	c.mu.Unlock()

	log.Printf("Attempting to reconnect in %v", reconnectDelay)
	time.Sleep(reconnectDelay)

	if err := c.Connect(); err != nil {
		log.Printf("Reconnection failed: %v", err)
		return
	}

	// Resubscribe to all intents
	c.mu.RLock()
	subscriptions := make(map[string]bool)
	for intentID := range c.subscriptions {
		subscriptions[intentID] = true
	}
	c.mu.RUnlock()

	for intentID := range subscriptions {
		subscribeMsg := map[string]string{
			"type":      "subscribe",
			"intent_id": intentID,
		}
		
		if err := c.conn.WriteJSON(subscribeMsg); err != nil {
			log.Printf("Failed to resubscribe to intent %s: %v", intentID, err)
		} else {
			log.Printf("Resubscribed to intent: %s", intentID)
		}
	}
}

// Close closes the WebSocket connection and stops all goroutines
func (c *Client) Close() error {
	c.cancel() // Cancel context to stop goroutines

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.isConnected = false

	log.Println("WebSocket client closed")
	return nil
}

// IsConnected returns whether the client is currently connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isConnected
}