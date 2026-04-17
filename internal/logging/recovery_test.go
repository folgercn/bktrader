package logging

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
)

func TestConfigurePersistsSystemAndHTTPRequestLogsToDisk(t *testing.T) {
	ResetForTests()
	t.Cleanup(ResetForTests)

	dir := t.TempDir()
	originalLogger := slog.Default()
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	cfg := config.Config{
		AppName:          "bkTrader-test",
		Environment:      "test",
		LogLevel:         "debug",
		LogFormat:        "json",
		LogDir:           dir,
		LogRetentionDays: 7,
		LogMaxSizeMB:     1,
	}
	if err := Configure(cfg); err != nil {
		t.Fatalf("configure logger: %v", err)
	}

	slog.Info("persistent system log", "component", "test.persistence", "attempt", 1)
	RecordHTTPRequest(HTTPRequestLogEntry{
		ID:         "http-log-7",
		Level:      "info",
		Message:    "http request completed",
		Method:     "GET",
		Path:       "/healthz",
		Status:     200,
		DurationMs: 12,
		CreatedAt:  time.Date(2026, time.April, 17, 9, 0, 0, 0, time.UTC),
		Attributes: map[string]any{
			"component": "http",
		},
	})

	systemEntries := readJSONLines[SystemLogEntry](t, filepath.Join(dir, systemLogMirrorFilename))
	if len(systemEntries) == 0 {
		t.Fatalf("expected system log mirror to contain entries")
	}
	foundSystem := false
	for _, entry := range systemEntries {
		if entry.Message != "persistent system log" {
			continue
		}
		foundSystem = true
		if entry.Level != "info" {
			t.Fatalf("expected system log level info, got %q", entry.Level)
		}
		if got := stringValue(entry.Attributes["component"]); got != "test.persistence" {
			t.Fatalf("expected mirrored component, got %q", got)
		}
	}
	if !foundSystem {
		t.Fatalf("expected persistent system log in mirrored file")
	}

	httpEntries := readJSONLines[HTTPRequestLogEntry](t, filepath.Join(dir, httpRequestLogMirrorFilename))
	if len(httpEntries) != 1 {
		t.Fatalf("expected exactly one persisted http request log, got %d", len(httpEntries))
	}
	if httpEntries[0].Path != "/healthz" {
		t.Fatalf("expected mirrored http path /healthz, got %q", httpEntries[0].Path)
	}
}

func TestBootstrapFromDiskRestoresBuffersAndSequences(t *testing.T) {
	ResetForTests()
	t.Cleanup(ResetForTests)

	dir := t.TempDir()
	writeJSONLines(t, filepath.Join(dir, systemLogMirrorFilename), []SystemLogEntry{
		{
			ID:        "system-log-41",
			Level:     "warning",
			Message:   "persisted warning",
			CreatedAt: time.Date(2026, time.April, 17, 8, 0, 0, 0, time.UTC),
			Attributes: map[string]any{
				"component": "bootstrap",
			},
		},
		{
			ID:        "system-log-42",
			Level:     "info",
			Message:   "persisted info",
			CreatedAt: time.Date(2026, time.April, 17, 8, 5, 0, 0, time.UTC),
		},
	})
	writeJSONLines(t, filepath.Join(dir, httpRequestLogMirrorFilename), []HTTPRequestLogEntry{
		{
			ID:         "http-log-9",
			Level:      "info",
			Message:    "http request completed",
			Method:     "GET",
			Path:       "/api/v1/health",
			Status:     200,
			DurationMs: 4,
			CreatedAt:  time.Date(2026, time.April, 17, 8, 10, 0, 0, time.UTC),
		},
	})

	result, err := BootstrapFromDisk(dir)
	if err != nil {
		t.Fatalf("bootstrap from disk: %v", err)
	}
	if result.SystemRecovered != 2 {
		t.Fatalf("expected 2 recovered system logs, got %d", result.SystemRecovered)
	}
	if result.HTTPRecovered != 1 {
		t.Fatalf("expected 1 recovered http log, got %d", result.HTTPRecovered)
	}
	if result.SkippedLines != 0 {
		t.Fatalf("expected no skipped lines, got %d", result.SkippedLines)
	}

	systemPage, err := ListSystemLogs(SystemLogQuery{Limit: 10})
	if err != nil {
		t.Fatalf("list system logs: %v", err)
	}
	if len(systemPage.Items) != 2 {
		t.Fatalf("expected 2 system logs in memory, got %d", len(systemPage.Items))
	}
	if systemPage.Items[0].ID != "system-log-42" {
		t.Fatalf("expected newest recovered system log first, got %q", systemPage.Items[0].ID)
	}

	httpEntry, ok := GetHTTPRequestLog("http-log-9")
	if !ok {
		t.Fatalf("expected recovered http request log to be queryable")
	}
	if httpEntry.Path != "/api/v1/health" {
		t.Fatalf("expected recovered http request path, got %q", httpEntry.Path)
	}

	newSystem := RecordSystemLog(SystemLogEntry{
		Message:   "new system log",
		CreatedAt: time.Date(2026, time.April, 17, 8, 15, 0, 0, time.UTC),
	})
	if newSystem.ID != "system-log-43" {
		t.Fatalf("expected recovered sequence to continue at system-log-43, got %q", newSystem.ID)
	}

	newHTTP := RecordHTTPRequest(HTTPRequestLogEntry{
		Message:    "new http log",
		Method:     "POST",
		Path:       "/api/v1/orders",
		Status:     201,
		DurationMs: 6,
		CreatedAt:  time.Date(2026, time.April, 17, 8, 20, 0, 0, time.UTC),
	})
	if newHTTP.ID != "http-log-10" {
		t.Fatalf("expected recovered sequence to continue at http-log-10, got %q", newHTTP.ID)
	}
}

func TestBootstrapFromDiskDoesNotRewritePersistedLogs(t *testing.T) {
	ResetForTests()
	t.Cleanup(ResetForTests)

	dir := t.TempDir()
	writeJSONLines(t, filepath.Join(dir, systemLogMirrorFilename), []SystemLogEntry{
		{
			ID:        "system-log-1",
			Level:     "info",
			Message:   "persisted system log",
			CreatedAt: time.Date(2026, time.April, 17, 10, 0, 0, 0, time.UTC),
		},
	})
	writeJSONLines(t, filepath.Join(dir, httpRequestLogMirrorFilename), []HTTPRequestLogEntry{
		{
			ID:         "http-log-1",
			Level:      "info",
			Message:    "persisted http log",
			Method:     "GET",
			Path:       "/readyz",
			Status:     200,
			DurationMs: 3,
			CreatedAt:  time.Date(2026, time.April, 17, 10, 0, 1, 0, time.UTC),
		},
	})

	cfg := config.Config{
		AppName:          "bkTrader-test",
		Environment:      "test",
		LogLevel:         "info",
		LogFormat:        "json",
		LogDir:           dir,
		LogRetentionDays: 7,
		LogMaxSizeMB:     1,
	}
	if err := Configure(cfg); err != nil {
		t.Fatalf("configure logger: %v", err)
	}

	systemBefore, err := os.ReadFile(filepath.Join(dir, systemLogMirrorFilename))
	if err != nil {
		t.Fatalf("read persisted system log before bootstrap: %v", err)
	}
	httpBefore, err := os.ReadFile(filepath.Join(dir, httpRequestLogMirrorFilename))
	if err != nil {
		t.Fatalf("read persisted http log before bootstrap: %v", err)
	}

	if _, err := BootstrapFromDisk(dir); err != nil {
		t.Fatalf("first bootstrap from disk: %v", err)
	}
	if _, err := BootstrapFromDisk(dir); err != nil {
		t.Fatalf("second bootstrap from disk: %v", err)
	}

	systemAfter, err := os.ReadFile(filepath.Join(dir, systemLogMirrorFilename))
	if err != nil {
		t.Fatalf("read persisted system log after bootstrap: %v", err)
	}
	httpAfter, err := os.ReadFile(filepath.Join(dir, httpRequestLogMirrorFilename))
	if err != nil {
		t.Fatalf("read persisted http log after bootstrap: %v", err)
	}

	if !bytes.Equal(systemBefore, systemAfter) {
		t.Fatalf("expected bootstrap to avoid rewriting persisted system logs")
	}
	if !bytes.Equal(httpBefore, httpAfter) {
		t.Fatalf("expected bootstrap to avoid rewriting persisted http logs")
	}
}

func writeJSONLines[T any](t *testing.T, path string, entries []T) {
	t.Helper()
	var buffer bytes.Buffer
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("marshal json line: %v", err)
		}
		buffer.Write(line)
		buffer.WriteByte('\n')
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0o644); err != nil {
		t.Fatalf("write json lines: %v", err)
	}
}

func readJSONLines[T any](t *testing.T, path string) []T {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open mirrored log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	items := make([]T, 0)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var item T
		if err := json.Unmarshal(line, &item); err != nil {
			t.Fatalf("decode mirrored log line: %v", err)
		}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan mirrored log file: %v", err)
	}
	return items
}
