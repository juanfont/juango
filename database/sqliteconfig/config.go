// Package sqliteconfig provides type-safe configuration for SQLite databases
// with proper enum validation and URL generation for modernc.org/sqlite driver.
package sqliteconfig

import (
	"errors"
	"fmt"
	"strings"
)

// Errors returned by config validation.
var (
	ErrPathEmpty           = errors.New("path cannot be empty")
	ErrBusyTimeoutNegative = errors.New("busy_timeout must be >= 0")
	ErrInvalidJournalMode  = errors.New("invalid journal_mode")
	ErrInvalidAutoVacuum   = errors.New("invalid auto_vacuum")
	ErrWALAutocheckpoint   = errors.New("wal_autocheckpoint must be >= -1")
	ErrInvalidSynchronous  = errors.New("invalid synchronous")
	ErrInvalidTxLock       = errors.New("invalid txlock")
)

const (
	// DefaultBusyTimeout is the default busy timeout in milliseconds.
	DefaultBusyTimeout = 10000
)

// JournalMode represents SQLite journal_mode pragma values.
type JournalMode string

const (
	// JournalModeWAL enables Write-Ahead Logging (RECOMMENDED for production).
	JournalModeWAL JournalMode = "WAL"
	// JournalModeDelete uses traditional rollback journaling.
	JournalModeDelete JournalMode = "DELETE"
	// JournalModeTruncate is like DELETE but with faster cleanup.
	JournalModeTruncate JournalMode = "TRUNCATE"
	// JournalModePersist keeps journal file between transactions.
	JournalModePersist JournalMode = "PERSIST"
	// JournalModeMemory keeps journal in memory (DANGEROUS).
	JournalModeMemory JournalMode = "MEMORY"
	// JournalModeOff disables journaling entirely (EXTREMELY DANGEROUS).
	JournalModeOff JournalMode = "OFF"
)

// IsValid returns true if the JournalMode is valid.
func (j JournalMode) IsValid() bool {
	switch j {
	case JournalModeWAL, JournalModeDelete, JournalModeTruncate,
		JournalModePersist, JournalModeMemory, JournalModeOff:
		return true
	default:
		return false
	}
}

// String returns the string representation.
func (j JournalMode) String() string {
	return string(j)
}

// AutoVacuum represents SQLite auto_vacuum pragma values.
type AutoVacuum string

const (
	// AutoVacuumNone disables automatic space reclamation.
	AutoVacuumNone AutoVacuum = "NONE"
	// AutoVacuumFull immediately reclaims space on every DELETE/DROP.
	AutoVacuumFull AutoVacuum = "FULL"
	// AutoVacuumIncremental reclaims space gradually (RECOMMENDED for production).
	AutoVacuumIncremental AutoVacuum = "INCREMENTAL"
)

// IsValid returns true if the AutoVacuum is valid.
func (a AutoVacuum) IsValid() bool {
	switch a {
	case AutoVacuumNone, AutoVacuumFull, AutoVacuumIncremental:
		return true
	default:
		return false
	}
}

// String returns the string representation.
func (a AutoVacuum) String() string {
	return string(a)
}

// Synchronous represents SQLite synchronous pragma values.
type Synchronous string

const (
	// SynchronousOff disables syncing (DANGEROUS).
	SynchronousOff Synchronous = "OFF"
	// SynchronousNormal provides balanced performance and safety (RECOMMENDED).
	SynchronousNormal Synchronous = "NORMAL"
	// SynchronousFull provides maximum durability with performance cost.
	SynchronousFull Synchronous = "FULL"
	// SynchronousExtra provides paranoid-level data safety (EXTREME).
	SynchronousExtra Synchronous = "EXTRA"
)

// IsValid returns true if the Synchronous is valid.
func (s Synchronous) IsValid() bool {
	switch s {
	case SynchronousOff, SynchronousNormal, SynchronousFull, SynchronousExtra:
		return true
	default:
		return false
	}
}

// String returns the string representation.
func (s Synchronous) String() string {
	return string(s)
}

// TxLock represents SQLite transaction lock mode.
type TxLock string

const (
	// TxLockDeferred acquires locks lazily (SQLite default).
	TxLockDeferred TxLock = "deferred"
	// TxLockImmediate acquires write lock immediately (RECOMMENDED for production).
	TxLockImmediate TxLock = "immediate"
	// TxLockExclusive acquires exclusive lock immediately.
	TxLockExclusive TxLock = "exclusive"
)

// IsValid returns true if the TxLock is valid.
func (t TxLock) IsValid() bool {
	switch t {
	case TxLockDeferred, TxLockImmediate, TxLockExclusive, "":
		return true
	default:
		return false
	}
}

// String returns the string representation.
func (t TxLock) String() string {
	return string(t)
}

// Config holds SQLite database configuration with type-safe enums.
type Config struct {
	Path              string      // file path or ":memory:"
	BusyTimeout       int         // milliseconds (0 = default/disabled)
	JournalMode       JournalMode // journal mode (affects concurrency and crash recovery)
	AutoVacuum        AutoVacuum  // auto vacuum mode (affects storage efficiency)
	WALAutocheckpoint int         // pages (-1 = default/not set, 0 = disabled, >0 = enabled)
	Synchronous       Synchronous // synchronous mode (affects durability vs performance)
	ForeignKeys       bool        // enable foreign key constraints (data integrity)
	TxLock            TxLock      // transaction lock mode (affects write concurrency)
}

// Default returns the production configuration optimized for most usage patterns.
func Default(path string) *Config {
	return &Config{
		Path:              path,
		BusyTimeout:       DefaultBusyTimeout,
		JournalMode:       JournalModeWAL,
		AutoVacuum:        AutoVacuumIncremental,
		WALAutocheckpoint: 1000,
		Synchronous:       SynchronousNormal,
		ForeignKeys:       true,
		TxLock:            TxLockImmediate,
	}
}

// Memory returns a configuration for in-memory databases.
func Memory() *Config {
	return &Config{
		Path:              ":memory:",
		WALAutocheckpoint: -1,
		ForeignKeys:       true,
	}
}

// Validate checks if all configuration values are valid.
func (c *Config) Validate() error {
	if c.Path == "" {
		return ErrPathEmpty
	}

	if c.BusyTimeout < 0 {
		return fmt.Errorf("%w, got %d", ErrBusyTimeoutNegative, c.BusyTimeout)
	}

	if c.JournalMode != "" && !c.JournalMode.IsValid() {
		return fmt.Errorf("%w: %s", ErrInvalidJournalMode, c.JournalMode)
	}

	if c.AutoVacuum != "" && !c.AutoVacuum.IsValid() {
		return fmt.Errorf("%w: %s", ErrInvalidAutoVacuum, c.AutoVacuum)
	}

	if c.WALAutocheckpoint < -1 {
		return fmt.Errorf("%w, got %d", ErrWALAutocheckpoint, c.WALAutocheckpoint)
	}

	if c.Synchronous != "" && !c.Synchronous.IsValid() {
		return fmt.Errorf("%w: %s", ErrInvalidSynchronous, c.Synchronous)
	}

	if c.TxLock != "" && !c.TxLock.IsValid() {
		return fmt.Errorf("%w: %s", ErrInvalidTxLock, c.TxLock)
	}

	return nil
}

// ToURL builds a properly encoded SQLite connection string using _pragma parameters.
func (c *Config) ToURL() (string, error) {
	if err := c.Validate(); err != nil {
		return "", fmt.Errorf("invalid config: %w", err)
	}

	var pragmas []string

	if c.BusyTimeout > 0 {
		pragmas = append(pragmas, fmt.Sprintf("busy_timeout=%d", c.BusyTimeout))
	}
	if c.JournalMode != "" {
		pragmas = append(pragmas, fmt.Sprintf("journal_mode=%s", c.JournalMode))
	}
	if c.AutoVacuum != "" {
		pragmas = append(pragmas, fmt.Sprintf("auto_vacuum=%s", c.AutoVacuum))
	}
	if c.WALAutocheckpoint >= 0 {
		pragmas = append(pragmas, fmt.Sprintf("wal_autocheckpoint=%d", c.WALAutocheckpoint))
	}
	if c.Synchronous != "" {
		pragmas = append(pragmas, fmt.Sprintf("synchronous=%s", c.Synchronous))
	}
	if c.ForeignKeys {
		pragmas = append(pragmas, "foreign_keys=ON")
	}

	var baseURL string
	if c.Path == ":memory:" {
		baseURL = ":memory:"
	} else {
		baseURL = "file:" + c.Path
	}

	queryParts := make([]string, 0, 1+len(pragmas))

	if c.TxLock != "" {
		queryParts = append(queryParts, "_txlock="+string(c.TxLock))
	}

	for _, pragma := range pragmas {
		queryParts = append(queryParts, "_pragma="+pragma)
	}

	if len(queryParts) > 0 {
		baseURL += "?" + strings.Join(queryParts, "&")
	}

	return baseURL, nil
}
