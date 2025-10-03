package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	RedisHost     string
	RedisPort     int
	RedisPassword string
	RedisDB       int
}

type Redis struct {
	conn *redis.Client
}

func NewRedis(cfg RedisConfig) (*Redis, error) {

	url := fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort)
	log.Println("===>Redis URL: ", url)
	conn := redis.NewClient(&redis.Options{
		Addr:     url,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	return &Redis{conn: conn}, nil
}

func (r *Redis) HealthCheck(ctx context.Context) error {
	return r.conn.Ping(ctx).Err()
}

func (r *Redis) GetClient() *redis.Client {
	return r.conn
}

func (r *Redis) Close() error {
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

func (r *Redis) IsConnected(ctx context.Context) bool {
	if r.conn == nil {
		return false
	}
	return r.conn.Ping(ctx).Err() == nil
}

// SetWithExpiration sets a key-value pair with expiration time
func (r *Redis) SetWithExpiration(ctx context.Context, key, value string, expiration time.Duration) error {
	return r.conn.Set(ctx, key, value, expiration).Err()
}

// Get retrieves a value by
func (r *Redis) Get(ctx context.Context, key string) (string, error) {
	return r.conn.Get(ctx, key).Result()
}

// Delete removes keys
func (r *Redis) Delete(ctx context.Context, keys ...string) error {
	return r.conn.Del(ctx, keys...).Err()
}

// Exists checks if keys exist
func (r *Redis) Exists(ctx context.Context, keys ...string) (int64, error) {
	return r.conn.Exists(ctx, keys...).Result()
}

// Set sets a key-value pair
func (r *Redis) Set(ctx context.Context, key, value string, expiration time.Duration) error {
	return r.conn.Set(ctx, key, value, expiration).Err()
}

// Ping tests the connection
func (r *Redis) Ping(ctx context.Context) error {
	return r.conn.Ping(ctx).Err()
}

// Keys returns keys matching pattern
func (r *Redis) Keys(ctx context.Context, pattern string) ([]string, error) {
	return r.conn.Keys(ctx, pattern).Result()
}

// SMembers returns all members of a set
func (r *Redis) SMembers(ctx context.Context, key string) ([]string, error) {
	return r.conn.SMembers(ctx, key).Result()
}

// SAdd adds members to a set
func (r *Redis) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return r.conn.SAdd(ctx, key, members...).Err()
}

// SRem removes members from a set
func (r *Redis) SRem(ctx context.Context, key string, members ...interface{}) error {
	return r.conn.SRem(ctx, key, members...).Err()
}

// SCard returns the number of elements in a set
func (r *Redis) SCard(ctx context.Context, key string) (int64, error) {
	return r.conn.SCard(ctx, key).Result()
}
