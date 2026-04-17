package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

const (
	systemLogMirrorFilename            = "system-log.jsonl"
	httpRequestLogMirrorFilename       = "http-request-log.jsonl"
	bootstrapTailReadBytes       int64 = 4 << 20
)

type BootstrapResult struct {
	SystemRecovered int
	HTTPRecovered   int
	SkippedLines    int
}

type diskMirror struct {
	mu         sync.Mutex
	systemFile io.WriteCloser
	httpFile   io.WriteCloser
}

var defaultDiskMirror diskMirror

func configureDiskMirror(cfg config.Config) error {
	return defaultDiskMirror.configure(cfg.LogDir, cfg.LogMaxSizeMB, cfg.LogRetentionDays)
}

func (m *diskMirror) configure(logDir string, maxSizeMB, retentionDays int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closeLocked()
	if strings.TrimSpace(logDir) == "" {
		return nil
	}
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	m.systemFile = &lumberjack.Logger{
		Filename:   filepath.Join(logDir, systemLogMirrorFilename),
		MaxSize:    maxSizeMB,
		MaxAge:     retentionDays,
		MaxBackups: 0,
		LocalTime:  false,
		Compress:   false,
	}
	m.httpFile = &lumberjack.Logger{
		Filename:   filepath.Join(logDir, httpRequestLogMirrorFilename),
		MaxSize:    maxSizeMB,
		MaxAge:     retentionDays,
		MaxBackups: 0,
		LocalTime:  false,
		Compress:   false,
	}
	return nil
}

func (m *diskMirror) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeLocked()
}

func (m *diskMirror) closeLocked() {
	if m.systemFile != nil {
		_ = m.systemFile.Close()
		m.systemFile = nil
	}
	if m.httpFile != nil {
		_ = m.httpFile.Close()
		m.httpFile = nil
	}
}

func (m *diskMirror) writeSystem(entry SystemLogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.systemFile == nil {
		return nil
	}
	return writeJSONLine(m.systemFile, normalizePersistedSystemLog(entry))
}

func (m *diskMirror) writeHTTPRequest(entry HTTPRequestLogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.httpFile == nil {
		return nil
	}
	return writeJSONLine(m.httpFile, normalizePersistedHTTPRequestLog(entry))
}

func BootstrapFromDisk(logDir string) (BootstrapResult, error) {
	logDir = strings.TrimSpace(logDir)
	if logDir == "" {
		return BootstrapResult{}, nil
	}
	if _, err := os.Stat(logDir); err != nil {
		if os.IsNotExist(err) {
			return BootstrapResult{}, nil
		}
		return BootstrapResult{}, fmt.Errorf("stat log dir: %w", err)
	}

	systemEntries, skipped, err := loadPersistedTail[SystemLogEntry](filepath.Join(logDir, "system-log*.jsonl"), defaultSystemLogCapacity)
	if err != nil {
		return BootstrapResult{}, err
	}
	httpEntries, httpSkipped, err := loadPersistedTail[HTTPRequestLogEntry](filepath.Join(logDir, "http-request-log*.jsonl"), defaultHTTPRequestCapacity)
	if err != nil {
		return BootstrapResult{}, err
	}
	// Bootstrap intentionally restores via store-only helpers so historical lines
	// are not re-published to brokers or appended back into the mirror files.
	for _, entry := range systemEntries {
		restoreSystemLog(entry)
	}
	for _, entry := range httpEntries {
		restoreHTTPRequest(entry)
	}
	return BootstrapResult{
		SystemRecovered: len(systemEntries),
		HTTPRecovered:   len(httpEntries),
		SkippedLines:    skipped + httpSkipped,
	}, nil
}

func loadPersistedTail[T any](pattern string, limit int) ([]T, int, error) {
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, 0, fmt.Errorf("glob log files: %w", err)
	}
	paths = filterExistingFiles(paths)
	if len(paths) == 0 {
		return nil, 0, nil
	}
	sort.Strings(paths)

	reversedLines := make([]string, 0, limit)
	for i := len(paths) - 1; i >= 0 && len(reversedLines) < limit; i-- {
		lines, err := readTailLines(paths[i], limit-len(reversedLines))
		if err != nil {
			return nil, 0, fmt.Errorf("read persisted log %s: %w", paths[i], err)
		}
		for j := len(lines) - 1; j >= 0 && len(reversedLines) < limit; j-- {
			reversedLines = append(reversedLines, lines[j])
		}
	}
	reverseStrings(reversedLines)

	items := make([]T, 0, len(reversedLines))
	skipped := 0
	for _, line := range reversedLines {
		var item T
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			skipped++
			continue
		}
		items = append(items, item)
	}
	return items, skipped, nil
}

func filterExistingFiles(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		out = append(out, path)
	}
	return out
}

func readTailLines(path string, maxLines int) ([]string, error) {
	if maxLines <= 0 {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	// Bootstrap only needs enough tail data to repopulate the in-memory buffers after restart.
	// This 4MB window is a best-effort bound so startup stays predictable even for large log files.
	offset := info.Size() - bootstrapTailReadBytes
	if offset < 0 {
		offset = 0
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	payload, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if index := bytes.IndexByte(payload, '\n'); index >= 0 {
			payload = payload[index+1:]
		} else {
			return nil, nil
		}
	}
	rawLines := bytes.Split(payload, []byte{'\n'})
	lines := make([]string, 0, len(rawLines))
	for _, raw := range rawLines {
		line := strings.TrimSpace(string(raw))
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, nil
}

func reverseStrings(values []string) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func writeJSONLine(writer io.Writer, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	_, err = writer.Write(body)
	return err
}

func normalizePersistedSystemLog(entry SystemLogEntry) SystemLogEntry {
	entry.CreatedAt = normalizeTime(entry.CreatedAt)
	entry.Attributes = normalizePersistedMap(entry.Attributes)
	return entry
}

func normalizePersistedHTTPRequestLog(entry HTTPRequestLogEntry) HTTPRequestLogEntry {
	entry.CreatedAt = normalizeTime(entry.CreatedAt)
	entry.Attributes = normalizePersistedMap(entry.Attributes)
	return entry
}

func normalizePersistedMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = normalizePersistedValue(value)
	}
	return out
}

func normalizePersistedValue(value any) any {
	switch typed := value.(type) {
	case nil,
		bool,
		string,
		float32,
		float64,
		int,
		int8,
		int16,
		int32,
		int64,
		uint,
		uint8,
		uint16,
		uint32,
		uint64,
		json.Number:
		return typed
	case time.Time:
		return typed.UTC()
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizePersistedValue(item))
		}
		return out
	case map[string]any:
		return normalizePersistedMap(typed)
	case error:
		return typed.Error()
	case fmt.Stringer:
		return typed.String()
	default:
		if _, err := json.Marshal(typed); err == nil {
			return typed
		}
		return fmt.Sprintf("%v", typed)
	}
}
