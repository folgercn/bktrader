package config

import (
	"fmt"
	"testing"
)

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
		LogDir:           "/tmp/bktrader-logs",
		LogRetentionDays: 7,
		LogMaxSizeMB:     100,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected config to validate, got error: %v", err)
	}
}

func TestConfigValidateAllowsZeroPersistenceLimitsWhenLogDirDisabled(t *testing.T) {
	cfg := Config{
		HTTPAddr:         ":8080",
		StoreBackend:     "memory",
		LogRetentionDays: 0,
		LogMaxSizeMB:     0,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected disabled log persistence to skip retention validation, got %v", err)
	}
}

func TestConfigValidateRejectsInvalidLogPersistenceLimits(t *testing.T) {
	cfg := Config{
		HTTPAddr:         ":8080",
		StoreBackend:     "memory",
		LogDir:           "/tmp/bktrader-logs",
		LogRetentionDays: 0,
		LogMaxSizeMB:     100,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected invalid retention to fail validation")
	}

	cfg = Config{
		HTTPAddr:         ":8080",
		StoreBackend:     "memory",
		LogDir:           "/tmp/bktrader-logs",
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

func TestConfigValidateAcceptsSignalRuntimeRunnerRole(t *testing.T) {
	cfg := Config{HTTPAddr: ":8080", StoreBackend: "memory", ProcessRole: "signal-runtime-runner"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected signal-runtime-runner role to validate, got %v", err)
	}
	if cfg.RuntimeActionsEnabled() {
		t.Fatal("expected signal-runtime-runner to reject live runtime mutations")
	}
}

func TestConfigValidateAcceptsSupervisorRole(t *testing.T) {
	cfg := Config{HTTPAddr: ":8080", StoreBackend: "memory", ProcessRole: "supervisor"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected supervisor role to validate, got %v", err)
	}
	if cfg.RuntimeActionsEnabled() {
		t.Fatal("expected supervisor role to reject live runtime mutations")
	}
}

func TestConfigValidateSupervisorContainerExecutor(t *testing.T) {
	for _, executor := range []string{"", "noop", " NoOp "} {
		t.Run("accept_"+executor, func(t *testing.T) {
			cfg := Config{HTTPAddr: ":8080", StoreBackend: "memory", SupervisorContainerExecutor: executor}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("expected executor %q to validate, got %v", executor, err)
			}
		})
	}

	cfg := Config{HTTPAddr: ":8080", StoreBackend: "memory", SupervisorContainerExecutor: "docker"}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected unsupported supervisor container executor to fail validation")
	}
}

func TestConfigValidateRejectsDuplicateSupervisorTargetNames(t *testing.T) {
	cfg := Config{
		HTTPAddr:          ":8080",
		StoreBackend:      "memory",
		SupervisorTargets: []string{"api=http://127.0.0.1:8080", "api=http://127.0.0.1:8081"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected duplicate supervisor target name to fail validation")
	}
}

func TestConfigValidateRejectsDuplicateSupervisorTargetBaseURLs(t *testing.T) {
	tests := []struct {
		name    string
		targets []string
	}{
		{
			name:    "named duplicates",
			targets: []string{"api=http://127.0.0.1:8080", "worker=http://127.0.0.1:8080/"},
		},
		{
			name:    "named and unnamed duplicates",
			targets: []string{"api=http://127.0.0.1:8080", "http://127.0.0.1:8080/"},
		},
		{
			name:    "case-normalized duplicates",
			targets: []string{"api=HTTP://PLATFORM-API:8080", "worker=http://platform-api:8080"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{HTTPAddr: ":8080", StoreBackend: "memory", SupervisorTargets: tt.targets}
			if err := cfg.Validate(); err == nil {
				t.Fatalf("expected duplicate supervisor target base URL to fail validation")
			}
		})
	}
}

func TestConfigValidateRejectsMalformedSupervisorTargets(t *testing.T) {
	tests := []struct {
		name    string
		targets []string
	}{
		{
			name:    "empty explicit name",
			targets: []string{"=http://127.0.0.1:8080"},
		},
		{
			name:    "missing explicit base url",
			targets: []string{"api="},
		},
		{
			name:    "blank explicit base url",
			targets: []string{"api= "},
		},
		{
			name:    "explicit base url without scheme",
			targets: []string{"api=platform-api:8080"},
		},
		{
			name:    "explicit base url with unsupported scheme",
			targets: []string{"api=ftp://platform-api:8080"},
		},
		{
			name:    "explicit base url without host",
			targets: []string{"api=http://"},
		},
		{
			name:    "explicit name with whitespace",
			targets: []string{"platform api=http://127.0.0.1:8080"},
		},
		{
			name:    "explicit name with slash",
			targets: []string{"platform/api=http://127.0.0.1:8080"},
		},
		{
			name:    "explicit name with colon",
			targets: []string{"platform:api=http://127.0.0.1:8080"},
		},
		{
			name:    "explicit name with unicode",
			targets: []string{"平台=http://127.0.0.1:8080"},
		},
		{
			name:    "explicit base url with userinfo",
			targets: []string{"api=http://user:pass@127.0.0.1:8080"},
		},
		{
			name:    "explicit base url with query",
			targets: []string{"api=http://127.0.0.1:8080?token=secret"},
		},
		{
			name:    "explicit base url with fragment",
			targets: []string{"api=http://127.0.0.1:8080#runtime"},
		},
		{
			name:    "unnamed base url without scheme",
			targets: []string{"platform-api:8080"},
		},
		{
			name:    "unnamed base url with unsupported scheme",
			targets: []string{"ftp://platform-api:8080"},
		},
		{
			name:    "unnamed base url with query",
			targets: []string{"http://127.0.0.1:8080?token=secret"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{HTTPAddr: ":8080", StoreBackend: "memory", SupervisorTargets: tt.targets}
			if err := cfg.Validate(); err == nil {
				t.Fatalf("expected malformed supervisor target to fail validation")
			}
		})
	}
}

func TestConfigValidateRejectsTooManySupervisorTargets(t *testing.T) {
	targets := make([]string, 0, maxSupervisorTargets+1)
	for i := 0; i < maxSupervisorTargets+1; i++ {
		targets = append(targets, fmt.Sprintf("target-%02d=http://127.0.0.1:8080/%02d", i, i))
	}
	cfg := Config{HTTPAddr: ":8080", StoreBackend: "memory", SupervisorTargets: targets}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected too many supervisor targets to fail validation")
	}
}

func TestConfigValidateAllowsUnnamedSupervisorTargets(t *testing.T) {
	cfg := Config{
		HTTPAddr:          ":8080",
		StoreBackend:      "memory",
		SupervisorTargets: []string{"http://127.0.0.1:8080", "https://platform-api.example"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected unnamed supervisor targets to validate, got %v", err)
	}
}

func TestConfigValidateAllowsHTTPSNamedSupervisorTargets(t *testing.T) {
	cfg := Config{
		HTTPAddr:          ":8080",
		StoreBackend:      "memory",
		SupervisorTargets: []string{"api=https://platform-api.example/supervisor", "signal_runtime.worker-1=http://127.0.0.1:8081"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected named supervisor targets with http(s) URLs to validate, got %v", err)
	}
}

func TestLoadReadsSupervisorEnv(t *testing.T) {
	t.Setenv("SUPERVISOR_TARGETS", "api=http://127.0.0.1:8080, http://127.0.0.1:8081")
	t.Setenv("SUPERVISOR_BEARER_TOKEN", " supervisor-token ")
	t.Setenv("SUPERVISOR_POLL_INTERVAL_SECONDS", "45")
	t.Setenv("SUPERVISOR_HTTP_TIMEOUT_SECONDS", "3")
	t.Setenv("SUPERVISOR_APPLICATION_RESTART_ENABLED", "true")
	t.Setenv("SUPERVISOR_SERVICE_FAILURE_THRESHOLD", "4")
	t.Setenv("SUPERVISOR_CONTAINER_RESTART_ENABLED", "true")
	t.Setenv("SUPERVISOR_CONTAINER_EXECUTOR", " NoOp ")

	cfg := Load()
	if len(cfg.SupervisorTargets) != 2 {
		t.Fatalf("expected two supervisor targets, got %#v", cfg.SupervisorTargets)
	}
	if cfg.SupervisorTargets[0] != "api=http://127.0.0.1:8080" || cfg.SupervisorTargets[1] != "http://127.0.0.1:8081" {
		t.Fatalf("unexpected supervisor targets %#v", cfg.SupervisorTargets)
	}
	if cfg.SupervisorBearerToken != "supervisor-token" {
		t.Fatalf("expected trimmed supervisor bearer token, got %q", cfg.SupervisorBearerToken)
	}
	if cfg.SupervisorPollIntervalSeconds != 45 {
		t.Fatalf("expected supervisor poll interval 45, got %d", cfg.SupervisorPollIntervalSeconds)
	}
	if cfg.SupervisorHTTPTimeoutSeconds != 3 {
		t.Fatalf("expected supervisor HTTP timeout 3, got %d", cfg.SupervisorHTTPTimeoutSeconds)
	}
	if !cfg.SupervisorAppRestartEnabled {
		t.Fatal("expected supervisor application restart to be enabled")
	}
	if cfg.SupervisorServiceFailThreshold != 4 {
		t.Fatalf("expected supervisor service failure threshold 4, got %d", cfg.SupervisorServiceFailThreshold)
	}
	if !cfg.SupervisorContainerRestart {
		t.Fatal("expected supervisor container restart opt-in to be enabled")
	}
	if cfg.SupervisorContainerExecutor != "noop" {
		t.Fatalf("expected supervisor container executor noop, got %q", cfg.SupervisorContainerExecutor)
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
