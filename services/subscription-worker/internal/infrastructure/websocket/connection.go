package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/quangdang46/NFT-Marketplace/services/subscription-worker/internal/domain"
)

// Connection implements the WebSocketConnection interface
type Connection struct {
	id        string
	conn      *websocket.Conn
	send      chan []byte
	manager   *Manager
	intentIDs map[string]bool
	mu        sync.RWMutex
	isActive  bool
	closeOnce sync.Once
}

// NewConnection creates a new WebSocket connection
func NewConnection(id string, conn *websocket.Conn, manager *Manager) *Connection {
	return &Connection{
		id:        id,
		conn:      conn,
		send:      make(chan []byte, 256),
		manager:   manager,
		intentIDs: make(map[string]bool),
		isActive:  true,
	}
}

// Send sends a message to the client
func (c *Connection) Send(message *domain.WebSocketMessage) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.isActive {
		return fmt.Errorf("connection is closed")
	}

	// Marshal message to JSON
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Send to channel (non-blocking)
	select {
	case c.send <- data:
		return nil
	default:
		// Channel full, close connection
		go c.Close()
		return fmt.Errorf("send channel full, closing connection")
	}
}

// Close closes the connection
func (c *Connection) Close() error {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.isActive = false
		c.mu.Unlock()

		close(c.send)
		c.conn.Close()
		
		// Remove from manager
		c.manager.RemoveConnection(c.id)
	})
	return nil
}

// GetID returns the connection ID
func (c *Connection) GetID() string {
	return c.id
}

// GetIntentIDs returns the intent IDs this connection is subscribed to
func (c *Connection) GetIntentIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	intentIDs := make([]string, 0, len(c.intentIDs))
	for intentID := range c.intentIDs {
		intentIDs = append(intentIDs, intentID)
	}
	return intentIDs
}

// AddIntentID adds an intent ID to the subscription list
func (c *Connection) AddIntentID(intentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.intentIDs[intentID] = true
}

// RemoveIntentID removes an intent ID from the subscription list
func (c *Connection) RemoveIntentID(intentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.intentIDs, intentID)
}

// IsActive returns whether the connection is active
func (c *Connection) IsActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isActive
}

// HasIntentID checks if the connection is subscribed to an intent
func (c *Connection) HasIntentID(intentID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.intentIDs[intentID]
}

// writePump pumps messages from the send channel to the websocket connection
func (c *Connection) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Channel closed
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to current message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump pumps messages from the websocket connection to handle client messages
func (c *Connection) readPump() {
	defer func() {
		c.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle client messages (subscribe/unsubscribe requests)
		if err := c.handleClientMessage(message); err != nil {
			log.Printf("Error handling client message: %v", err)
		}
	}
}

// handleClientMessage processes messages from the client
func (c *Connection) handleClientMessage(data []byte) error {
	var clientMessage struct {
		Type     string `json:"type"`
		IntentID string `json:"intent_id"`
	}

	if err := json.Unmarshal(data, &clientMessage); err != nil {
		return fmt.Errorf("invalid message format: %w", err)
	}

	switch clientMessage.Type {
	case "subscribe":
		if clientMessage.IntentID == "" {
			return fmt.Errorf("intent_id is required for subscribe")
		}
		c.AddIntentID(clientMessage.IntentID)
		c.manager.AddSubscription(clientMessage.IntentID, c.id)
		
		// Send subscription confirmation
		response := domain.NewWebSocketMessage("subscribed", clientMessage.IntentID, map[string]string{
			"status": "subscribed to intent",
		})
		c.Send(response)

	case "unsubscribe":
		if clientMessage.IntentID == "" {
			return fmt.Errorf("intent_id is required for unsubscribe")
		}
		c.RemoveIntentID(clientMessage.IntentID)
		c.manager.RemoveSubscription(clientMessage.IntentID, c.id)
		
		// Send unsubscription confirmation
		response := domain.NewWebSocketMessage("unsubscribed", clientMessage.IntentID, map[string]string{
			"status": "unsubscribed from intent",
		})
		c.Send(response)

	case "ping":
		// Respond with pong
		response := domain.NewWebSocketMessage("pong", "", map[string]interface{}{
			"timestamp": time.Now().Unix(),
		})
		c.Send(response)

	default:
		return fmt.Errorf("unknown message type: %s", clientMessage.Type)
	}

	return nil
}

// Start starts the connection's read and write pumps
func (c *Connection) Start() {
	go c.writePump()
	go c.readPump()
}

// SendJSON sends a JSON message directly (utility method)
func (c *Connection) SendJSON(data interface{}) error {
	message := domain.NewWebSocketMessage("data", "", data)
	return c.Send(message)
}