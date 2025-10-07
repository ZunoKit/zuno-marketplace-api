package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

type PostgresConfig struct {
	PostgresHost     string
	PostgresPort     int
	PostgresUser     string
	PostgresPassword string
	PostgresDatabase string
	PostgresSSLMode  string
}

type Postgres struct {
	conn *sql.DB
}

func NewPostgres(cfg PostgresConfig) (*Postgres, error) {
	dsn := buildDSN(cfg)
	log.Println("===>Postgres DSN: ", dsn)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return &Postgres{conn: db}, nil
}

func (p *Postgres) HealthCheck(ctx context.Context) error {
	return p.conn.PingContext(ctx)
}

func (p *Postgres) Close() error {
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

func (p *Postgres) Ping(ctx context.Context) error {
	return p.conn.PingContext(ctx)
}

func buildDSN(cfg PostgresConfig) string {
	if cfg.PostgresSSLMode == "" {
		cfg.PostgresSSLMode = "disable"
	}
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresDatabase,
		cfg.PostgresSSLMode,
	)
}

func (p *Postgres) GetClient() *sql.DB {
	return p.conn
}

// NewPostgresWithDB creates a Postgres instance with an existing database connection
// This is useful for testing with sqlmock
func NewPostgresWithDB(db *sql.DB) *Postgres {
	return &Postgres{conn: db}
}
