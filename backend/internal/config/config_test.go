package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("DefaultConfig failed validation: %v", err)
	}

	if cfg.Server.Port != 8443 {
		t.Errorf("expected default port 8443, got %d", cfg.Server.Port)
	}
	if cfg.Provider.DefaultProvider != "incus" {
		t.Errorf("expected default provider incus, got %s", cfg.Provider.DefaultProvider)
	}
}

func TestLoadFromEnvOverrides(t *testing.T) {
	os.Setenv("MYSTIC_SERVER_PORT", "9443")
	os.Setenv("MYSTIC_DEFAULT_PROVIDER", "lxc")
	os.Setenv("MYSTIC_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("MYSTIC_SERVER_PORT")
		os.Unsetenv("MYSTIC_DEFAULT_PROVIDER")
		os.Unsetenv("MYSTIC_LOG_LEVEL")
	}()

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}

	if cfg.Server.Port != 9443 {
		t.Errorf("expected port override 9443, got %d", cfg.Server.Port)
	}
	if cfg.Provider.DefaultProvider != "lxc" {
		t.Errorf("expected provider override lxc, got %s", cfg.Provider.DefaultProvider)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level override debug, got %s", cfg.Logging.Level)
	}
}

func TestConfigValidationErrors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Port = 999999
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid port, got nil")
	}

	cfg = DefaultConfig()
	cfg.Provider.DefaultProvider = "docker"
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for unsupported provider, got nil")
	}
}
