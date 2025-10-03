package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/quangdang46/NFT-Marketplace/shared/contracts"
	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQConfig holds the configuration for RabbitMQ
type RabbitMQConfig struct {
	RabbitMQHost     string `json:"rabbitmq_host"`
	RabbitMQPort     int    `json:"rabbitmq_port"`
	RabbitMQUser     string `json:"rabbitmq_user"`
	RabbitMQPassword string `json:"rabbitmq_password"`
	RabbitMQExchange string `json:"rabbitmq_exchange"`
}

// ExchangeConfig defines exchange configuration
type ExchangeConfig struct {
	Name       string `json:"name"`
	Type       string `json:"type"` // "topic", "direct", "fanout", "headers"
	Durable    bool   `json:"durable"`
	AutoDelete bool   `json:"auto_delete"`
	Internal   bool   `json:"internal"`
	NoWait     bool   `json:"no_wait"`
}

// QueueConfig defines queue configuration
type QueueConfig struct {
	Name       string `json:"name"`
	Durable    bool   `json:"durable"`
	AutoDelete bool   `json:"auto_delete"`
	Exclusive  bool   `json:"exclusive"`
	NoWait     bool   `json:"no_wait"`
	TTL        int64  `json:"ttl,omitempty"`        // Message TTL in milliseconds
	MaxLength  int32  `json:"max_length,omitempty"` // Max queue length
	DLX        string `json:"dlx,omitempty"`        // Dead Letter Exchange
	DLRKey     string `json:"dlr_key,omitempty"`    // Dead Letter Routing Key
}

// BindingConfig defines queue-to-exchange binding
type BindingConfig struct {
	QueueName    string `json:"queue_name"`
	ExchangeName string `json:"exchange_name"`
	RoutingKey   string `json:"routing_key"`
	NoWait       bool   `json:"no_wait"`
}

// PublishConfig defines publishing options
type PublishConfig struct {
	Exchange     string                 `json:"exchange"`
	RoutingKey   string                 `json:"routing_key"`
	Mandatory    bool                   `json:"mandatory"`
	Immediate    bool                   `json:"immediate"`
	ContentType  string                 `json:"content_type"`
	DeliveryMode uint8                  `json:"delivery_mode"` // 1 = non-persistent, 2 = persistent
	Priority     uint8                  `json:"priority"`
	Headers      map[string]interface{} `json:"headers,omitempty"`
}

// MessageHandler defines the signature for message handlers
type MessageHandler func(context.Context, amqp.Delivery) error

// Message represents a message to be published
type Message struct {
	Exchange   string
	RoutingKey string
	Body       []byte
	Headers    map[string]interface{}
	Timestamp  time.Time
	MessageID  string
}

// ToAMQPMessage converts Message to contracts.AMQPMessage
func (m *Message) ToAMQPMessage() contracts.AMQPMessage {
	return contracts.AMQPMessage{
		Exchange:   m.Exchange,
		RoutingKey: m.RoutingKey,
		Body:       m.Body,
		Headers:    m.Headers,
	}
}

// RabbitMQ wraps the AMQP connection and provides high-level operations
type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  RabbitMQConfig
	closed  bool
}

// NewRabbitMQ creates a new RabbitMQ client with configuration
func NewRabbitMQ(config RabbitMQConfig) (*RabbitMQ, error) {
	// Set defaults

	rmq := &RabbitMQ{
		config: config,
	}

	if err := rmq.connect(); err != nil {
		return nil, err
	}

	return rmq, nil
}

// buildURL builds AMQP URL from config components
func (r *RabbitMQ) buildURL() string {
	scheme := "amqp"
	if r.config.RabbitMQPort == 5671 {
		scheme = "amqps"
	}
	return fmt.Sprintf("%s://%s:%s@%s:%d",
		scheme,
		r.config.RabbitMQUser,
		r.config.RabbitMQPassword,
		r.config.RabbitMQHost,
		r.config.RabbitMQPort,
	)
}

// connect establishes connection to RabbitMQ
func (r *RabbitMQ) connect() error {
	url := r.buildURL()

	log.Println("===>RabbitMQ URL: ", url)
	conn, err := amqp.DialConfig(url, amqp.Config{
		Heartbeat: 10,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create channel: %w", err)
	}

	// Set QoS
	if err := ch.Qos(10, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	r.conn = conn
	r.channel = ch
	r.closed = false

	return nil
}

// DeclareExchange declares an exchange
func (r *RabbitMQ) DeclareExchange(config ExchangeConfig) error {
	return r.channel.ExchangeDeclare(
		config.Name,
		config.Type,
		config.Durable,
		config.AutoDelete,
		config.Internal,
		config.NoWait,
		nil,
	)
}

// DeclareQueue declares a queue
func (r *RabbitMQ) DeclareQueue(config QueueConfig) (amqp.Queue, error) {
	args := amqp.Table{}

	if config.TTL > 0 {
		args["x-message-ttl"] = config.TTL
	}
	if config.MaxLength > 0 {
		args["x-max-length"] = config.MaxLength
	}
	if config.DLX != "" {
		args["x-dead-letter-exchange"] = config.DLX
	}
	if config.DLRKey != "" {
		args["x-dead-letter-routing-key"] = config.DLRKey
	}

	return r.channel.QueueDeclare(
		config.Name,
		config.Durable,
		config.AutoDelete,
		config.Exclusive,
		config.NoWait,
		args,
	)
}

// BindQueue binds a queue to an exchange
func (r *RabbitMQ) BindQueue(config BindingConfig) error {
	return r.channel.QueueBind(
		config.QueueName,
		config.RoutingKey,
		config.ExchangeName,
		config.NoWait,
		nil,
	)
}

// Publish publishes a message using the contracts.AMQPMessage interface
func (r *RabbitMQ) Publish(ctx context.Context, message contracts.AMQPMessage) error {
	if r.closed {
		return fmt.Errorf("connection is closed")
	}

	// Convert headers to amqp.Table
	headers := make(amqp.Table)
	for k, v := range message.Headers {
		headers[k] = v
	}

	// Set default content type if not specified
	contentType := "application/json"
	if ct, ok := headers["content-type"]; ok {
		if ctStr, ok := ct.(string); ok {
			contentType = ctStr
		}
	}

	// Set default delivery mode to persistent
	deliveryMode := uint8(2)
	if dm, ok := headers["delivery-mode"]; ok {
		if dmUint, ok := dm.(uint8); ok {
			deliveryMode = dmUint
		}
	}

	return r.channel.PublishWithContext(
		ctx,
		message.Exchange,
		message.RoutingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			Headers:      headers,
			ContentType:  contentType,
			DeliveryMode: deliveryMode,
			Timestamp:    time.Now(),
			Body:         message.Body,
		},
	)
}

// PublishWithConfig publishes a message with detailed configuration
func (r *RabbitMQ) PublishWithConfig(ctx context.Context, body []byte, config PublishConfig) error {
	if r.closed {
		return fmt.Errorf("connection is closed")
	}

	// Convert headers to amqp.Table
	headers := make(amqp.Table)
	for k, v := range config.Headers {
		headers[k] = v
	}

	// Set defaults
	contentType := config.ContentType
	if contentType == "" {
		contentType = "application/json"
	}

	deliveryMode := config.DeliveryMode
	if deliveryMode == 0 {
		deliveryMode = 2 // persistent
	}

	return r.channel.PublishWithContext(
		ctx,
		config.Exchange,
		config.RoutingKey,
		config.Mandatory,
		config.Immediate,
		amqp.Publishing{
			Headers:      headers,
			ContentType:  contentType,
			DeliveryMode: deliveryMode,
			Priority:     config.Priority,
			Timestamp:    time.Now(),
			Body:         body,
		},
	)
}

// PublishJSON publishes a JSON message
func (r *RabbitMQ) PublishJSON(ctx context.Context, exchange, routingKey string, data interface{}) error {
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return r.Publish(ctx, contracts.AMQPMessage{
		Exchange:   exchange,
		RoutingKey: routingKey,
		Body:       body,
		Headers: map[string]interface{}{
			"content-type": "application/json",
		},
	})
}

// Consume starts consuming messages from a queue
func (r *RabbitMQ) Consume(queueName, consumerTag string, handler MessageHandler) error {
	if r.closed {
		return fmt.Errorf("connection is closed")
	}

	msgs, err := r.channel.Consume(
		queueName,
		consumerTag,
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	go func() {
		ctx := context.Background()
		for msg := range msgs {
			if err := handler(ctx, msg); err != nil {
				log.Printf("Message handler error: %v", err)
				// Reject and requeue the message
				msg.Nack(false, true)
			} else {
				// Acknowledge the message
				msg.Ack(false)
			}
		}
	}()

	return nil
}

// SetupInfrastructure sets up exchanges, queues, and bindings
func (r *RabbitMQ) SetupInfrastructure(exchanges []ExchangeConfig, queues []QueueConfig, bindings []BindingConfig) error {
	// Declare exchanges
	for _, exchange := range exchanges {
		if err := r.DeclareExchange(exchange); err != nil {
			return fmt.Errorf("failed to declare exchange %s: %w", exchange.Name, err)
		}
		log.Printf("Declared exchange: %s", exchange.Name)
	}

	// Declare queues
	for _, queue := range queues {
		if _, err := r.DeclareQueue(queue); err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", queue.Name, err)
		}
		log.Printf("Declared queue: %s", queue.Name)
	}

	// Create bindings
	for _, binding := range bindings {
		if err := r.BindQueue(binding); err != nil {
			return fmt.Errorf("failed to bind queue %s to exchange %s: %w",
				binding.QueueName, binding.ExchangeName, err)
		}
		log.Printf("Bound queue %s to exchange %s with key %s",
			binding.QueueName, binding.ExchangeName, binding.RoutingKey)
	}

	return nil
}

// IsConnected checks if the connection is alive
func (r *RabbitMQ) IsConnected() bool {
	return !r.closed && r.conn != nil && !r.conn.IsClosed()
}

// Reconnect attempts to reconnect to RabbitMQ
func (r *RabbitMQ) Reconnect() error {
	r.Close()
	return r.connect()
}

// GetConnection returns the underlying AMQP connection
func (r *RabbitMQ) GetConnection() *amqp.Connection {
	return r.conn
}

// GetExchange returns the configured exchange name
func (r *RabbitMQ) GetExchange() string {
	return r.config.RabbitMQExchange
}

// Close closes the connection
func (r *RabbitMQ) Close() error {
	r.closed = true

	if r.channel != nil {
		if err := r.channel.Close(); err != nil {
			log.Printf("Error closing channel: %v", err)
		}
	}

	if r.conn != nil {
		if err := r.conn.Close(); err != nil {
			log.Printf("Error closing connection: %v", err)
			return err
		}
	}

	return nil
}
