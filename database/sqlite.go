// Package database provides SQLite database helpers with WAL mode.
package database

import (
	"context"
	"database/sql"
	"embed"
	"encoding/gob"
	"errors"
	"fmt"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/juanfont/juango/database/sqliteconfig"
	"github.com/juanfont/juango/types"
	"github.com/rs/zerolog/log"
	"github.com/tailscale/squibble"

	_ "modernc.org/sqlite"
)

// Database errors.
var (
	ErrBuildConnectionURL = errors.New("failed to build SQLite connection URL")
	ErrOpenDatabase       = errors.New("failed to open database")
	ErrPingDatabase       = errors.New("failed to ping database")
	ErrApplySchema        = errors.New("failed to apply schema")
	ErrSchemaValidation   = errors.New("schema validation failed")
)

// Database wraps the sqlx database connection.
type Database struct {
	db *sqlx.DB
}

// Config holds database configuration.
type Config struct {
	Path   string
	Schema string
}

// New creates a new Database instance with the given path and schema.
func New(path string, schema string) (*Database, error) {
	// Register types for session serialization
	registerGobTypes()

	log.Debug().Msgf("Opening database: %s", path)
	db, err := openDatabase(path, schema)
	if err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

// NewWithConfig creates a new Database with custom configuration.
func NewWithConfig(cfg *sqliteconfig.Config, schema string) (*Database, error) {
	registerGobTypes()

	connectionURL, err := cfg.ToURL()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBuildConnectionURL, err)
	}

	log.Debug().
		Str("path", cfg.Path).
		Str("config", connectionURL).
		Msg("Opening SQLite database with custom configuration")

	db, err := sqlx.Open("sqlite", connectionURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOpenDatabase, err)
	}

	// SQLite concurrency settings
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("%w: %w", ErrPingDatabase, err)
	}

	// Apply schema if provided
	if schema != "" {
		s := &squibble.Schema{Current: schema}
		if err := s.Apply(context.Background(), db.DB); err != nil {
			db.Close()
			return nil, fmt.Errorf("%w: %w", ErrApplySchema, err)
		}
	}

	return &Database{db: db}, nil
}

// registerGobTypes registers types needed for session serialization.
func registerGobTypes() {
	gob.Register(types.User{})
	gob.Register(types.OIDCClaims{})
	gob.Register(types.AdminModeState{})
	gob.Register(types.ImpersonationState{})
	gob.Register(sql.NullString{})
	gob.Register(sql.NullTime{})
}

func openDatabase(path string, schema string) (*sqlx.DB, error) {
	isNewDatabase := false
	if path != ":memory:" {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			isNewDatabase = true
		}
	}

	cfg := sqliteconfig.Default(path)

	connectionURL, err := cfg.ToURL()
	if err != nil {
		log.Error().
			Str("path", path).
			Err(err).
			Msg("Failed to build SQLite connection URL")
		return nil, fmt.Errorf("%w: %w", ErrBuildConnectionURL, err)
	}

	log.Debug().
		Str("path", path).
		Str("config", connectionURL).
		Bool("new_database", isNewDatabase).
		Msg("Opening SQLite database with optimized configuration")

	db, err := sqlx.Open("sqlite", connectionURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOpenDatabase, err)
	}

	// SQLite concurrency settings: Single connection model
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("%w: %w", ErrPingDatabase, err)
	}

	// Apply schema if provided
	if schema != "" {
		s := &squibble.Schema{Current: schema}
		if err := s.Apply(context.Background(), db.DB); err != nil {
			db.Close()
			return nil, fmt.Errorf("%w: %w", ErrApplySchema, err)
		}
	}

	log.Info().
		Str("path", path).
		Str("config", connectionURL).
		Msg("Database opened successfully")

	return db, nil
}

// Close closes the database connection.
func (d *Database) Close() error {
	return d.db.Close()
}

// DB returns the underlying *sqlx.DB for advanced operations.
func (d *Database) DB() *sqlx.DB {
	return d.db
}

// WithTx executes a function within a database transaction.
func (d *Database) WithTx(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := d.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Migrate runs migrations from an embedded filesystem.
// The migrations should be in a directory structure like:
// migrations/001_initial.sql, migrations/002_add_users.sql, etc.
func Migrate(db *Database, migrations embed.FS, dir string) error {
	entries, err := migrations.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		content, err := migrations.ReadFile(dir + "/" + entry.Name())
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", entry.Name(), err)
		}

		log.Debug().Str("migration", entry.Name()).Msg("Applying migration")

		if _, err := db.db.Exec(string(content)); err != nil {
			return fmt.Errorf("applying migration %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// MigrateWithSquibble runs migrations using squibble schema management.
func MigrateWithSquibble(db *Database, schema *squibble.Schema) error {
	if err := schema.Apply(context.Background(), db.db.DB); err != nil {
		return fmt.Errorf("applying schema: %w", err)
	}
	return nil
}

// BaseSchema returns a minimal base schema for juango applications.
// Applications should extend this with their own tables.
func BaseSchema() string {
	return `
-- Users table
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL DEFAULT '',
    profile_pic_url TEXT NOT NULL DEFAULT '',
    provider_identifier TEXT UNIQUE,
    is_admin INTEGER NOT NULL DEFAULT 0,
    last_login DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    modified_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_provider_identifier ON users(provider_identifier);

-- Audit log table
CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    actor_user_id TEXT,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    changes TEXT,
    ip_address TEXT,
    user_agent TEXT,
    FOREIGN KEY (actor_user_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor ON audit_log(actor_user_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log(action);
CREATE INDEX IF NOT EXISTS idx_audit_log_resource ON audit_log(resource_type, resource_id);

-- Notifications table
CREATE TABLE IF NOT EXISTS notifications (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'info',
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    link TEXT,
    read INTEGER NOT NULL DEFAULT 0,
    read_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_read ON notifications(user_id, read);
`
}
