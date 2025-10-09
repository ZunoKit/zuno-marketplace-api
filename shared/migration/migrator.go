package migration

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

// Migrator handles database migrations
type Migrator struct {
	db         *sql.DB
	migrations embed.FS
	service    string
	schemaName string
}

// Config holds migration configuration
type Config struct {
	DatabaseURL string
	Service     string
	SchemaName  string
	Migrations  embed.FS
}

// NewMigrator creates a new migrator
func NewMigrator(config *Config) (*Migrator, error) {
	// Open database connection
	db, err := sql.Open("postgres", config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Migrator{
		db:         db,
		migrations: config.Migrations,
		service:    config.Service,
		schemaName: config.SchemaName,
	}, nil
}

// Migrate runs all pending migrations
func (m *Migrator) Migrate() error {
	log.Printf("Starting migrations for service: %s, schema: %s", m.service, m.schemaName)

	// Create schema if it doesn't exist
	if err := m.createSchema(); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Set search path to the schema
	if err := m.setSearchPath(); err != nil {
		return fmt.Errorf("failed to set search path: %w", err)
	}

	// Create migration instance
	migration, err := m.createMigration()
	if err != nil {
		return fmt.Errorf("failed to create migration: %w", err)
	}

	// Run migrations
	if err := migration.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	version, _, _ := migration.Version()
	log.Printf("Migrations completed successfully. Current version: %d", version)

	return nil
}

// MigrateUp runs n up migrations
func (m *Migrator) MigrateUp(n int) error {
	migration, err := m.createMigration()
	if err != nil {
		return err
	}

	if err := migration.Steps(n); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run %d up migrations: %w", n, err)
	}

	return nil
}

// MigrateDown runs n down migrations
func (m *Migrator) MigrateDown(n int) error {
	migration, err := m.createMigration()
	if err != nil {
		return err
	}

	if err := migration.Steps(-n); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run %d down migrations: %w", n, err)
	}

	return nil
}

// Version returns the current migration version
func (m *Migrator) Version() (uint, bool, error) {
	migration, err := m.createMigration()
	if err != nil {
		return 0, false, err
	}

	return migration.Version()
}

// Force sets a specific migration version
func (m *Migrator) Force(version int) error {
	migration, err := m.createMigration()
	if err != nil {
		return err
	}

	if err := migration.Force(version); err != nil {
		return fmt.Errorf("failed to force version %d: %w", version, err)
	}

	return nil
}

// createSchema creates the schema if it doesn't exist
func (m *Migrator) createSchema() error {
	query := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", m.schemaName)
	_, err := m.db.Exec(query)
	return err
}

// setSearchPath sets the search path to the schema
func (m *Migrator) setSearchPath() error {
	query := fmt.Sprintf("SET search_path TO %s", m.schemaName)
	_, err := m.db.Exec(query)
	return err
}

// createMigration creates a migration instance
func (m *Migrator) createMigration() (*migrate.Migrate, error) {
	// Create source driver from embedded files
	sourceDriver, err := iofs.New(m.migrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to create source driver: %w", err)
	}

	// Create database driver
	dbDriver, err := postgres.WithInstance(m.db, &postgres.Config{
		SchemaName:      m.schemaName,
		MigrationsTable: fmt.Sprintf("%s_migrations", strings.ReplaceAll(m.service, "-", "_")),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create database driver: %w", err)
	}

	// Create migration instance
	migration, err := migrate.NewWithInstance("iofs", sourceDriver, m.schemaName, dbDriver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migration instance: %w", err)
	}

	return migration, nil
}

// Close closes the database connection
func (m *Migrator) Close() error {
	return m.db.Close()
}
