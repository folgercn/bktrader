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
		HTTPAddr:     ":8080",
		StoreBackend: "memory",
		LogLevel:     "debug",
		LogFormat:    "json",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected config to validate, got error: %v", err)
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
