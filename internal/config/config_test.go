package config

import "testing"

func TestConfigValidateRejectsUnsupportedLoggingValues(t *testing.T) {
	cfg := Config{HTTPAddr: ":8080", StoreBackend: "memory", LogLevel: "trace"}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected unsupported log level to fail validation")
	}

	cfg = Config{HTTPAddr: ":8080", StoreBackend: "memory", LogFormat: "yaml"}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected unsupported log format to fail validation")
	}
}

func TestConfigValidateAcceptsSupportedLoggingValues(t *testing.T) {
	cfg := Config{
		HTTPAddr:         ":8080",
		StoreBackend:     "memory",
		LogLevel:         "debug",
		LogFormat:        "json",
		LogRetentionDays: 7,
		LogMaxSizeMB:     100,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected config to validate, got error: %v", err)
	}
}

func TestConfigValidateRejectsInvalidLogPersistenceLimits(t *testing.T) {
	cfg := Config{
		HTTPAddr:         ":8080",
		StoreBackend:     "memory",
		LogRetentionDays: 0,
		LogMaxSizeMB:     100,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected invalid retention to fail validation")
	}

	cfg = Config{
		HTTPAddr:         ":8080",
		StoreBackend:     "memory",
		LogRetentionDays: 7,
		LogMaxSizeMB:     0,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected invalid log max size to fail validation")
	}
}

func TestLoadReadsLogPersistenceEnv(t *testing.T) {
	t.Setenv("LOG_DIR", "/tmp/bktrader-logs")
	t.Setenv("LOG_RETENTION_DAYS", "9")
	t.Setenv("LOG_MAX_SIZE_MB", "64")

	cfg := Load()
	if cfg.LogDir != "/tmp/bktrader-logs" {
		t.Fatalf("expected LOG_DIR to be loaded, got %q", cfg.LogDir)
	}
	if cfg.LogRetentionDays != 9 {
		t.Fatalf("expected LOG_RETENTION_DAYS=9, got %d", cfg.LogRetentionDays)
	}
	if cfg.LogMaxSizeMB != 64 {
		t.Fatalf("expected LOG_MAX_SIZE_MB=64, got %d", cfg.LogMaxSizeMB)
	}
}

func TestBoolFromEnvRecognizesTruthyAndFalsyValues(t *testing.T) {
	t.Setenv("BOOL_TRUE", "yes")
	if !boolFromEnv("BOOL_TRUE", false) {
		t.Fatalf("expected yes to parse as true")
	}

	t.Setenv("BOOL_FALSE", "off")
	if boolFromEnv("BOOL_FALSE", true) {
		t.Fatalf("expected off to parse as false")
	}

	t.Setenv("BOOL_INVALID", "maybe")
	if !boolFromEnv("BOOL_INVALID", true) {
		t.Fatalf("expected invalid value to fall back to default")
	}
}
