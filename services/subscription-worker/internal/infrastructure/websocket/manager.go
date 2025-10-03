package websocket

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/quangdang46/NFT-Marketplace/services/subscription-worker/internal/config"
	"github.com/quangdang46/NFT-Marketplace/services/subscription-worker/internal/domain"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// In production, implement proper origin checking
		return true
	},
}

// Manager manages WebSocket connections and subscriptions
type Manager struct {
	config        config.WebSocketConfig
	connections   map[string]*Connection
	subscriptions map[string]map[string]bool // intentID -> connectionID -> bool
	mu            sync.RWMutex
	server        *http.Server
	isRunning     bool
}

// NewManager creates a new WebSocket manager
func NewManager(config config.WebSocketConfig) *Manager {
	return &Manager{
		config:        config,
		connections:   make(map[string]*Connection),
		subscriptions: make(map[string]map[string]bool),
	}
}

// Start starts the WebSocket manager
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		return fmt.Errorf("WebSocket manager is already running")
	}
	m.isRunning = true
	m.mu.Unlock()

	// Set up HTTP handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", m.handleWebSocket)
	mux.HandleFunc("/health", m.handleHealth)
	mux.HandleFunc("/stats", m.handleStats)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%s", m.config.Host, m.config.Port)
	m.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("Starting WebSocket server on %s", addr)

	// Start cleanup routine
	go m.cleanupRoutine(ctx)

	// Start server
	go func() {
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("WebSocket server error: %v", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	return m.Stop(ctx)
}

// Stop stops the WebSocket manager
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isRunning {
		return nil
	}

	log.Println("Stopping WebSocket manager...")

	// Close all connections
	for _, conn := range m.connections {
		conn.Close()
	}

	// Shutdown HTTP server
	if m.server != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		
		if err := m.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("failed to shutdown WebSocket server: %w", err)
		}
	}

	m.isRunning = false
	log.Println("WebSocket manager stopped")
	return nil
}

// AddConnection adds a new WebSocket connection
func (m *Manager) AddConnection(conn domain.WebSocketConnection) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.connections) >= m.config.MaxConnections {
		return fmt.Errorf("maximum connections reached")
	}

	if wsConn, ok := conn.(*Connection); ok {
		m.connections[conn.GetID()] = wsConn
		log.Printf("Added WebSocket connection: %s", conn.GetID())
		return nil
	}

	return fmt.Errorf("invalid connection type")
}

// RemoveConnection removes a WebSocket connection
func (m *Manager) RemoveConnection(connID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conn, exists := m.connections[connID]; exists {
		// Remove from all subscriptions
		for intentID := range m.subscriptions {
			if m.subscriptions[intentID] != nil {
				delete(m.subscriptions[intentID], connID)
				if len(m.subscriptions[intentID]) == 0 {
					delete(m.subscriptions, intentID)
				}
			}
		}

		delete(m.connections, connID)
		log.Printf("Removed WebSocket connection: %s", connID)
		_ = conn // Avoid unused variable warning
	}
}

// SendToIntent sends a message to all connections subscribed to an intent
func (m *Manager) SendToIntent(intentID string, message *domain.WebSocketMessage) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subscribers, exists := m.subscriptions[intentID]
	if !exists || len(subscribers) == 0 {
		log.Printf("No subscribers for intent: %s", intentID)
		return nil
	}

	var errors []error
	sentCount := 0

	for connID := range subscribers {
		if conn, exists := m.connections[connID]; exists && conn.IsActive() {
			if err := conn.Send(message); err != nil {
				log.Printf("Failed to send message to connection %s: %v", connID, err)
				errors = append(errors, err)
				// Remove failed connection
				go m.RemoveConnection(connID)
			} else {
				sentCount++
			}
		}
	}

	log.Printf("Sent message to %d/%d subscribers for intent %s", sentCount, len(subscribers), intentID)

	if len(errors) > 0 {
		return fmt.Errorf("failed to send to %d connections", len(errors))
	}

	return nil
}

// SendToConnection sends a message to a specific connection
func (m *Manager) SendToConnection(connID string, message *domain.WebSocketMessage) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if conn, exists := m.connections[connID]; exists {
		return conn.Send(message)
	}

	return fmt.Errorf("connection not found: %s", connID)
}

// GetConnectionCount returns the number of active connections
func (m *Manager) GetConnectionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.connections)
}

// HealthCheck performs a health check
func (m *Manager) HealthCheck() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.isRunning {
		return fmt.Errorf("WebSocket manager is not running")
	}

	return nil
}

// AddSubscription adds a subscription mapping
func (m *Manager) AddSubscription(intentID, connID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.subscriptions[intentID] == nil {
		m.subscriptions[intentID] = make(map[string]bool)
	}
	m.subscriptions[intentID][connID] = true
	log.Printf("Added subscription: intent=%s, connection=%s", intentID, connID)
}

// RemoveSubscription removes a subscription mapping
func (m *Manager) RemoveSubscription(intentID, connID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if subscribers, exists := m.subscriptions[intentID]; exists {
		delete(subscribers, connID)
		if len(subscribers) == 0 {
			delete(m.subscriptions, intentID)
		}
		log.Printf("Removed subscription: intent=%s, connection=%s", intentID, connID)
	}
}

// HTTP handlers

func (m *Manager) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	// Create connection ID
	connID := uuid.New().String()

	// Create connection wrapper
	wsConn := NewConnection(connID, conn, m)

	// Add to manager
	if err := m.AddConnection(wsConn); err != nil {
		log.Printf("Failed to add connection: %v", err)
		conn.Close()
		return
	}

	// Start connection handling
	wsConn.Start()
}

func (m *Manager) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := m.HealthCheck(); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status": "healthy", "connections": %d}`, m.GetConnectionCount())
}

func (m *Manager) handleStats(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	stats := map[string]interface{}{
		"connections":   len(m.connections),
		"subscriptions": len(m.subscriptions),
		"max_connections": m.config.MaxConnections,
		"is_running":    m.isRunning,
	}

	// Count total subscribers
	totalSubscribers := 0
	for _, subscribers := range m.subscriptions {
		totalSubscribers += len(subscribers)
	}
	stats["total_subscribers"] = totalSubscribers
	m.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	// Simple JSON encoding
	fmt.Fprintf(w, `{
		"connections": %d,
		"subscriptions": %d,
		"total_subscribers": %d,
		"max_connections": %d,
		"is_running": %t
	}`, stats["connections"], stats["subscriptions"], stats["total_subscribers"], 
		stats["max_connections"], stats["is_running"])
}

// cleanupRoutine periodically cleans up inactive connections
func (m *Manager) cleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.cleanup()
		}
	}
}

// cleanup removes inactive connections
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	inactiveConnections := make([]string, 0)

	for connID, conn := range m.connections {
		if !conn.IsActive() {
			inactiveConnections = append(inactiveConnections, connID)
		}
	}

	for _, connID := range inactiveConnections {
		delete(m.connections, connID)

		// Remove from subscriptions
		for intentID := range m.subscriptions {
			if m.subscriptions[intentID] != nil {
				delete(m.subscriptions[intentID], connID)
				if len(m.subscriptions[intentID]) == 0 {
					delete(m.subscriptions, intentID)
				}
			}
		}
	}

	if len(inactiveConnections) > 0 {
		log.Printf("Cleaned up %d inactive connections", len(inactiveConnections))
	}
}

// GetSubscriptionInfo returns subscription information for debugging
func (m *Manager) GetSubscriptionInfo() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info := make(map[string]interface{})
	info["total_intents"] = len(m.subscriptions)
	info["total_connections"] = len(m.connections)

	intentInfo := make(map[string]int)
	for intentID, subscribers := range m.subscriptions {
		intentInfo[intentID] = len(subscribers)
	}
	info["intents"] = intentInfo

	return info
}