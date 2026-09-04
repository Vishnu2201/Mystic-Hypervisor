package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// Config represents the root application configuration structure.
type Config struct {
	Server     ServerConfig     `json:"server"`
	Database   DatabaseConfig   `json:"database"`
	Auth       AuthConfig       `json:"auth"`
	Provider   ProviderConfig   `json:"provider"`
	Networking NetworkingConfig `json:"networking"`
	Monitoring MonitoringConfig `json:"monitoring"`
	TLS        TLSConfig        `json:"tls"`
	Logging    LoggingConfig    `json:"logging"`
}

type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type DatabaseConfig struct {
	Driver string `json:"driver"`
	Path   string `json:"path"`
}

type AuthConfig struct {
	JWTSecret            string `json:"jwt_secret"`
	TokenExpiryHours     int    `json:"token_expiry_hours"`
	SessionTimeoutMinute int    `json:"session_timeout_minutes"`
}

type ProviderConfig struct {
	DefaultProvider string `json:"default_provider"` // incus, lxc, kvm
	IncusSocket     string `json:"incus_socket"`
	LXCSocket       string `json:"lxc_socket"`
	KVMPath         string `json:"kvm_path"`
}

type NetworkingConfig struct {
	DefaultBridge string `json:"default_bridge"`
	EnableNAT     bool   `json:"enable_nat"`
	SubnetCIDR    string `json:"subnet_cidr"`
}

type MonitoringConfig struct {
	IntervalSeconds int `json:"interval_seconds"`
	RetentionDays   int `json:"retention_days"`
}

type TLSConfig struct {
	Enabled  bool   `json:"enabled"`
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
}

type LoggingConfig struct {
	Level  string `json:"level"`  // debug, info, warn, error
	Format string `json:"format"` // json, text
}

// DefaultConfig returns safe defaults for Mystic Hypervisor.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8443,
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			Path:   "/var/lib/mystic/mystic.db",
		},
		Auth: AuthConfig{
			JWTSecret:            "", // Must be provided via env or loaded securely
			TokenExpiryHours:     24,
			SessionTimeoutMinute: 60,
		},
		Provider: ProviderConfig{
			DefaultProvider: "incus",
			IncusSocket:     "/var/lib/incus/unix.socket",
			LXCSocket:       "/var/lib/lxc/unix.socket",
			KVMPath:         "/usr/bin/qemu-system-x86_64",
		},
		Networking: NetworkingConfig{
			DefaultBridge: "mysticbr0",
			EnableNAT:     true,
			SubnetCIDR:    "10.250.0.0/24",
		},
		Monitoring: MonitoringConfig{
			IntervalSeconds: 5,
			RetentionDays:   7,
		},
		TLS: TLSConfig{
			Enabled:  true,
			CertFile: "/etc/mystic/certs/server.crt",
			KeyFile:  "/etc/mystic/certs/server.key",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

// LoadFromEnv loads configuration overrides from environment variables.
func LoadFromEnv() (*Config, error) {
	cfg := DefaultConfig()

	if host := os.Getenv("MYSTIC_SERVER_HOST"); host != "" {
		cfg.Server.Host = host
	}
	if portStr := os.Getenv("MYSTIC_SERVER_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			cfg.Server.Port = port
		}
	}
	if dbDriver := os.Getenv("MYSTIC_DB_DRIVER"); dbDriver != "" {
		cfg.Database.Driver = dbDriver
	}
	if dbPath := os.Getenv("MYSTIC_DB_PATH"); dbPath != "" {
		cfg.Database.Path = dbPath
	}
	if secret := os.Getenv("MYSTIC_JWT_SECRET"); secret != "" {
		cfg.Auth.JWTSecret = secret
	}
	if provider := os.Getenv("MYSTIC_DEFAULT_PROVIDER"); provider != "" {
		cfg.Provider.DefaultProvider = provider
	}
	if logLevel := os.Getenv("MYSTIC_LOG_LEVEL"); logLevel != "" {
		cfg.Logging.Level = logLevel
	}

	return cfg, cfg.Validate()
}

// Validate checks the configuration for required settings and security boundaries.
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	if c.Database.Path == "" {
		return errors.New("database path must not be empty")
	}
	if c.Provider.DefaultProvider == "" {
		return errors.New("default provider must be specified")
	}
	switch c.Provider.DefaultProvider {
	case "incus", "lxc", "kvm":
		// valid provider
	default:
		return fmt.Errorf("unsupported default provider: %s", c.Provider.DefaultProvider)
	}
	return nil
}
