// Package config provides Viper configuration loading utilities.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

const (
	// JSONLogFormat indicates JSON log format.
	JSONLogFormat = "json"
	// TextLogFormat indicates text log format.
	TextLogFormat = "text"
)

// LogConfig holds logging configuration.
type LogConfig struct {
	Format     string        `mapstructure:"format"`
	Level      zerolog.Level `mapstructure:"level"`
	WithCaller bool          `mapstructure:"with_caller"`
}

// SessionConfig holds session configuration.
type SessionConfig struct {
	AuthenticationKey string        `mapstructure:"authentication_key"`
	EncryptionKey     string        `mapstructure:"encryption_key"`
	CookieName        string        `mapstructure:"cookie_name"`
	CookieExpiry      time.Duration `mapstructure:"cookie_expiry"`
}

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	Path              string `mapstructure:"path"`
	WriteAheadLog     bool   `mapstructure:"write_ahead_log"`
	WALAutoCheckPoint int    `mapstructure:"wal_auto_check_point"`
}

// RedisConfig holds Redis configuration for background tasks.
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// WorkerConfig holds background worker configuration.
type WorkerConfig struct {
	Concurrency int `mapstructure:"concurrency"`
}

// OIDCConfig holds OIDC authentication configuration.
type OIDCConfig struct {
	Issuer       string            `mapstructure:"issuer"`
	ClientID     string            `mapstructure:"client_id"`
	ClientSecret string            `mapstructure:"client_secret"`
	Scopes       []string          `mapstructure:"scopes"`
	ExtraParams  map[string]string `mapstructure:"extra_params"`
	Expiry       time.Duration     `mapstructure:"expiry"`
}

// SMTPConfig holds SMTP email configuration.
type SMTPConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from_address"`
	ReplyTo  string `mapstructure:"reply_to"`
}

// BaseConfig holds common configuration fields used by juango applications.
type BaseConfig struct {
	ListenAddr       string        `mapstructure:"listen_addr"`
	AdvertiseURL     string        `mapstructure:"advertise_url"`
	AdminModeTimeout time.Duration `mapstructure:"admin_mode_timeout"`

	Session  SessionConfig  `mapstructure:"session"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Worker   WorkerConfig   `mapstructure:"worker"`
	OIDC     OIDCConfig     `mapstructure:"oidc"`
	Logging  LogConfig      `mapstructure:"logging"`
	SMTP     SMTPConfig     `mapstructure:"smtp"`
}

// LoaderConfig holds configuration for the config loader.
type LoaderConfig struct {
	// EnvPrefix is the prefix for environment variables (e.g., "MYAPP" -> MYAPP_LISTEN_ADDR).
	EnvPrefix string

	// ConfigPaths is a list of directories to search for config files.
	ConfigPaths []string

	// ConfigName is the name of the config file (without extension).
	ConfigName string

	// Defaults is a map of default values.
	Defaults map[string]interface{}
}

// DefaultLoaderConfig returns default loader configuration.
func DefaultLoaderConfig(envPrefix string) *LoaderConfig {
	return &LoaderConfig{
		EnvPrefix:  envPrefix,
		ConfigName: "config",
		ConfigPaths: []string{
			fmt.Sprintf("/etc/%s/", strings.ToLower(envPrefix)),
			fmt.Sprintf("$HOME/.%s", strings.ToLower(envPrefix)),
			".",
		},
		Defaults: map[string]interface{}{
			"admin_mode_timeout":      30 * time.Minute,
			"database.write_ahead_log": true,
			"database.wal_autocheckpoint": 1000,
			"redis.addr":              "localhost:6379",
			"redis.password":          "",
			"redis.db":                0,
			"worker.concurrency":      10,
			"logging.level":           "info",
			"logging.format":          TextLogFormat,
			"logging.with_caller":     false,
		},
	}
}

// Load reads configuration from file and environment variables.
// If configPath is empty, it searches in default paths.
// If isFile is true, configPath is treated as a direct file path.
func Load(configPath string, isFile bool, cfg *LoaderConfig) error {
	if cfg == nil {
		cfg = DefaultLoaderConfig("app")
	}

	log.Debug().Msg("Loading configuration")

	if isFile {
		viper.SetConfigFile(configPath)
	} else {
		viper.SetConfigName(cfg.ConfigName)
		if configPath == "" {
			for _, path := range cfg.ConfigPaths {
				viper.AddConfigPath(path)
			}
		} else {
			viper.AddConfigPath(configPath)
		}
	}

	// Environment variable configuration
	viper.SetEnvPrefix(cfg.EnvPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Set defaults
	for key, value := range cfg.Defaults {
		viper.SetDefault(key, value)
	}

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	log.Debug().
		Str("config_file", viper.ConfigFileUsed()).
		Msg("Configuration loaded")

	return nil
}

// GetLogConfig returns the logging configuration from Viper.
func GetLogConfig() LogConfig {
	logLevelStr := viper.GetString("logging.level")
	logLevel, err := zerolog.ParseLevel(logLevelStr)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}

	logFormatOpt := viper.GetString("logging.format")
	var logFormat string
	switch logFormatOpt {
	case JSONLogFormat:
		logFormat = JSONLogFormat
	case TextLogFormat:
		logFormat = TextLogFormat
	case "":
		logFormat = TextLogFormat
	default:
		log.Warn().
			Str("format", logFormatOpt).
			Msg("Invalid log format, using text")
		logFormat = TextLogFormat
	}

	return LogConfig{
		Format:     logFormat,
		Level:      logLevel,
		WithCaller: viper.GetBool("logging.with_caller"),
	}
}

// GetBaseConfig returns the base configuration from Viper.
// Applications should call this after Load() and extend with their own fields.
func GetBaseConfig() *BaseConfig {
	logConfig := GetLogConfig()
	zerolog.SetGlobalLevel(logConfig.Level)

	return &BaseConfig{
		ListenAddr:       viper.GetString("listen_addr"),
		AdvertiseURL:     viper.GetString("advertise_url"),
		AdminModeTimeout: viper.GetDuration("admin_mode_timeout"),
		Logging:          logConfig,
		Database: DatabaseConfig{
			Path:              viper.GetString("database.path"),
			WriteAheadLog:     viper.GetBool("database.write_ahead_log"),
			WALAutoCheckPoint: viper.GetInt("database.wal_auto_check_point"),
		},
		Redis: RedisConfig{
			Addr:     viper.GetString("redis.addr"),
			Password: viper.GetString("redis.password"),
			DB:       viper.GetInt("redis.db"),
		},
		Worker: WorkerConfig{
			Concurrency: viper.GetInt("worker.concurrency"),
		},
		Session: SessionConfig{
			CookieName:        viper.GetString("session.cookie_name"),
			CookieExpiry:      viper.GetDuration("session.cookie_expiry"),
			AuthenticationKey: viper.GetString("session.authentication_key"),
			EncryptionKey:     viper.GetString("session.encryption_key"),
		},
		OIDC: OIDCConfig{
			ClientID:     viper.GetString("oidc.client_id"),
			ClientSecret: viper.GetString("oidc.client_secret"),
			Issuer:       viper.GetString("oidc.issuer"),
			Scopes:       viper.GetStringSlice("oidc.scopes"),
		},
		SMTP: SMTPConfig{
			Host:     viper.GetString("smtp.host"),
			Port:     viper.GetInt("smtp.port"),
			User:     viper.GetString("smtp.user"),
			Password: viper.GetString("smtp.password"),
			From:     viper.GetString("smtp.from_address"),
			ReplyTo:  viper.GetString("smtp.reply_to"),
		},
	}
}

// ValidateRequired checks that required configuration fields are set.
func ValidateRequired(fields map[string]string) error {
	var missing []string
	for field, description := range fields {
		if viper.GetString(field) == "" {
			missing = append(missing, fmt.Sprintf("%s (%s)", field, description))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}
	return nil
}

// ValidateSessionKeys validates that session keys are the correct length.
func ValidateSessionKeys() error {
	authKey := viper.GetString("session.authentication_key")
	encKey := viper.GetString("session.encryption_key")

	if len(authKey) != 32 {
		return fmt.Errorf("session.authentication_key must be 32 bytes, got %d", len(authKey))
	}
	if len(encKey) != 32 {
		return fmt.Errorf("session.encryption_key must be 32 bytes, got %d", len(encKey))
	}
	return nil
}
