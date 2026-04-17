package logging

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultSystemLogCapacity   = 2048
	defaultHTTPRequestCapacity = 2048
	defaultQueryLimit          = 100
	maxQueryLimit              = 200
	redactedText               = "[redacted]"
)

// SystemLogEntry 表示可供前端回看的系统日志记录。
type SystemLogEntry struct {
	ID         string         `json:"id"`
	Level      string         `json:"level"`
	Message    string         `json:"message"`
	CreatedAt  time.Time      `json:"createdAt"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// HTTPRequestLogEntry 表示请求日志中间件采集到的结构化访问记录。
type HTTPRequestLogEntry struct {
	ID            string         `json:"id"`
	Level         string         `json:"level"`
	Message       string         `json:"message"`
	Method        string         `json:"method"`
	Path          string         `json:"path"`
	Query         string         `json:"query,omitempty"`
	RemoteAddr    string         `json:"remoteAddr,omitempty"`
	UserAgent     string         `json:"userAgent,omitempty"`
	Status        int            `json:"status"`
	DurationMs    int64          `json:"durationMs"`
	BytesWritten  int            `json:"bytesWritten"`
	ContentLength int64          `json:"contentLength"`
	PanicMessage  string         `json:"panicMessage,omitempty"`
	Stack         string         `json:"stack,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	Attributes    map[string]any `json:"attributes,omitempty"`
}

// SystemLogQuery 定义系统日志检索条件。
type SystemLogQuery struct {
	Level     string
	Component string
	From      time.Time
	To        time.Time
	Cursor    string
	Limit     int
}

// HTTPRequestLogQuery 定义请求日志检索条件。
type HTTPRequestLogQuery struct {
	Level         string
	Method        string
	Path          string
	Status        int
	DurationMinMs int64
	DurationMaxMs int64
	From          time.Time
	To            time.Time
	Cursor        string
	Limit         int
}

// SystemLogPage 表示系统日志分页结果。
type SystemLogPage struct {
	Items      []SystemLogEntry `json:"items"`
	NextCursor string           `json:"nextCursor,omitempty"`
}

// HTTPRequestLogPage 表示请求日志分页结果。
type HTTPRequestLogPage struct {
	Items      []HTTPRequestLogEntry `json:"items"`
	NextCursor string                `json:"nextCursor,omitempty"`
}

// StreamMessage 表示 SSE 日志总线输出的统一消息。
type StreamMessage struct {
	ID         string    `json:"id,omitempty"`
	Source     string    `json:"source"`
	Type       string    `json:"type"`
	Level      string    `json:"level,omitempty"`
	EventTime  time.Time `json:"eventTime"`
	RecordedAt time.Time `json:"recordedAt,omitempty"`
	Payload    any       `json:"payload,omitempty"`
}

// Broker 提供轻量级的非阻塞发布订阅能力，避免慢客户端拖垮后端。
type Broker struct {
	mu     sync.Mutex
	nextID int
	subs   map[int]chan StreamMessage
}

func NewBroker() *Broker {
	return &Broker{subs: make(map[int]chan StreamMessage)}
}

func (b *Broker) Subscribe(buffer int) (int, <-chan StreamMessage) {
	if buffer <= 0 {
		buffer = 32
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	ch := make(chan StreamMessage, buffer)
	b.subs[b.nextID] = ch
	return b.nextID, ch
}

func (b *Broker) Unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch, ok := b.subs[id]
	if !ok {
		return
	}
	delete(b.subs, id)
	close(ch)
}

func (b *Broker) Publish(message StreamMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- message:
		default:
			// 慢订阅者直接丢弃，避免背压传导到请求处理线程。
		}
	}
}

func (b *Broker) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, ch := range b.subs {
		delete(b.subs, id)
		close(ch)
	}
	b.nextID = 0
}

type ringBuffer[T any] struct {
	mu       sync.RWMutex
	capacity int
	items    []T
	head     int
	size     int
}

func newRingBuffer[T any](capacity int) *ringBuffer[T] {
	if capacity < 1 {
		capacity = 1
	}
	return &ringBuffer[T]{
		capacity: capacity,
		items:    make([]T, capacity),
	}
}

func (r *ringBuffer[T]) append(item T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[r.head] = item
	r.head = (r.head + 1) % r.capacity
	if r.size < r.capacity {
		r.size++
	}
}

func (r *ringBuffer[T]) snapshot() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]T, 0, r.size)
	start := r.head - r.size
	if start < 0 {
		start += r.capacity
	}
	for i := 0; i < r.size; i++ {
		index := (start + i) % r.capacity
		out = append(out, r.items[index])
	}
	return out
}

func (r *ringBuffer[T]) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	var zero T
	for i := range r.items {
		r.items[i] = zero
	}
	r.head = 0
	r.size = 0
}

type systemLogStore struct {
	sequence atomic.Int64
	buffer   *ringBuffer[SystemLogEntry]
}

type httpRequestLogStore struct {
	sequence atomic.Int64
	buffer   *ringBuffer[HTTPRequestLogEntry]
}

type timeCursor struct {
	Time string `json:"time"`
	ID   string `json:"id"`
}

var (
	defaultSystemLogStore   = &systemLogStore{buffer: newRingBuffer[SystemLogEntry](defaultSystemLogCapacity)}
	defaultHTTPRequestStore = &httpRequestLogStore{buffer: newRingBuffer[HTTPRequestLogEntry](defaultHTTPRequestCapacity)}
	defaultSystemBroker     = NewBroker()
	defaultHTTPBroker       = NewBroker()
)

func SystemBroker() *Broker {
	return defaultSystemBroker
}

func HTTPBroker() *Broker {
	return defaultHTTPBroker
}

func RecordSystemLog(entry SystemLogEntry) SystemLogEntry {
	recorded := defaultSystemLogStore.add(entry)
	_ = defaultDiskMirror.writeSystem(recorded)
	defaultSystemBroker.Publish(StreamMessage{
		ID:         recorded.ID,
		Source:     "system",
		Type:       "system-log",
		Level:      recorded.Level,
		EventTime:  recorded.CreatedAt,
		RecordedAt: recorded.CreatedAt,
		Payload:    recorded,
	})
	return recorded
}

func RecordHTTPRequest(entry HTTPRequestLogEntry) HTTPRequestLogEntry {
	recorded := defaultHTTPRequestStore.add(entry)
	_ = defaultDiskMirror.writeHTTPRequest(recorded)
	defaultHTTPBroker.Publish(StreamMessage{
		ID:         recorded.ID,
		Source:     "http",
		Type:       "http-request",
		Level:      recorded.Level,
		EventTime:  recorded.CreatedAt,
		RecordedAt: recorded.CreatedAt,
		Payload:    sanitizeHTTPRequestLogEntry(recorded),
	})
	return recorded
}

func ListSystemLogs(query SystemLogQuery) (SystemLogPage, error) {
	cursor, err := decodeTimeCursor(query.Cursor)
	if err != nil {
		return SystemLogPage{}, err
	}
	return defaultSystemLogStore.query(query, cursor), nil
}

func ListHTTPRequestLogs(query HTTPRequestLogQuery) (HTTPRequestLogPage, error) {
	cursor, err := decodeTimeCursor(query.Cursor)
	if err != nil {
		return HTTPRequestLogPage{}, err
	}
	return defaultHTTPRequestStore.query(query, cursor), nil
}

func GetHTTPRequestLog(id string) (HTTPRequestLogEntry, bool) {
	return defaultHTTPRequestStore.get(strings.TrimSpace(id))
}

// ResetForTests 清理全局日志缓冲，仅供测试使用。
func ResetForTests() {
	defaultSystemLogStore.reset()
	defaultHTTPRequestStore.reset()
	defaultSystemBroker.reset()
	defaultHTTPBroker.reset()
	defaultDiskMirror.reset()
}

func newSystemCaptureHandler() slog.Handler {
	return &systemCaptureHandler{}
}

type systemCaptureHandler struct {
	attrs  []slog.Attr
	groups []string
}

func (h *systemCaptureHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *systemCaptureHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := make(map[string]any)
	for _, attr := range h.attrs {
		appendSlogAttr(attrs, h.groups, attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		appendSlogAttr(attrs, h.groups, attr)
		return true
	})
	RecordSystemLog(SystemLogEntry{
		Level:      normalizeStoredLevel(record.Level.String()),
		Message:    record.Message,
		CreatedAt:  normalizeTime(record.Time),
		Attributes: attrs,
	})
	return nil
}

func (h *systemCaptureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &systemCaptureHandler{
		attrs:  append(append([]slog.Attr{}, h.attrs...), attrs...),
		groups: append([]string{}, h.groups...),
	}
}

func (h *systemCaptureHandler) WithGroup(name string) slog.Handler {
	if strings.TrimSpace(name) == "" {
		return &systemCaptureHandler{
			attrs:  append([]slog.Attr{}, h.attrs...),
			groups: append([]string{}, h.groups...),
		}
	}
	return &systemCaptureHandler{
		attrs:  append([]slog.Attr{}, h.attrs...),
		groups: append(append([]string{}, h.groups...), name),
	}
}

func (s *systemLogStore) add(entry SystemLogEntry) SystemLogEntry {
	entry.Level = normalizeStoredLevel(entry.Level)
	entry.CreatedAt = normalizeTime(entry.CreatedAt)
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("system-log-%d", s.sequence.Add(1))
	} else {
		bumpAtomicSequence(&s.sequence, entry.ID, "system-log-")
	}
	if len(entry.Attributes) == 0 {
		entry.Attributes = nil
	}
	s.buffer.append(entry)
	return entry
}

func (s *systemLogStore) query(query SystemLogQuery, cursor *timeCursor) SystemLogPage {
	items := s.buffer.snapshot()
	limit := normalizeLimit(query.Limit)
	level := normalizeLevelFilter(query.Level)
	component := strings.TrimSpace(query.Component)
	out := make([]SystemLogEntry, 0, limit+1)
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if cursor != nil && !isBeforeCursor(item.CreatedAt, item.ID, *cursor) {
			continue
		}
		if level != "" && item.Level != level {
			continue
		}
		if component != "" && strings.TrimSpace(stringValue(item.Attributes["component"])) != component {
			continue
		}
		if !query.From.IsZero() && item.CreatedAt.Before(query.From.UTC()) {
			continue
		}
		if !query.To.IsZero() && item.CreatedAt.After(query.To.UTC()) {
			continue
		}
		out = append(out, item)
		if len(out) >= limit+1 {
			break
		}
	}
	page := SystemLogPage{Items: out}
	if len(out) > limit {
		page.NextCursor = encodeTimeCursor(out[limit-1].CreatedAt, out[limit-1].ID)
		page.Items = out[:limit]
	}
	return page
}

func (s *systemLogStore) reset() {
	s.sequence.Store(0)
	s.buffer.reset()
}

func (s *httpRequestLogStore) add(entry HTTPRequestLogEntry) HTTPRequestLogEntry {
	entry.Level = normalizeStoredLevel(firstNonEmpty(entry.Level, HTTPLevel(entry.Status).String()))
	entry.CreatedAt = normalizeTime(entry.CreatedAt)
	entry.Method = strings.ToUpper(strings.TrimSpace(entry.Method))
	entry.Path = strings.TrimSpace(entry.Path)
	entry.Query = strings.TrimSpace(entry.Query)
	entry.RemoteAddr = strings.TrimSpace(entry.RemoteAddr)
	entry.UserAgent = strings.TrimSpace(entry.UserAgent)
	entry.Message = strings.TrimSpace(entry.Message)
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("http-log-%d", s.sequence.Add(1))
	} else {
		bumpAtomicSequence(&s.sequence, entry.ID, "http-log-")
	}
	if len(entry.Attributes) == 0 {
		entry.Attributes = nil
	}
	s.buffer.append(entry)
	return entry
}

func (s *httpRequestLogStore) query(query HTTPRequestLogQuery, cursor *timeCursor) HTTPRequestLogPage {
	items := s.buffer.snapshot()
	limit := normalizeLimit(query.Limit)
	level := normalizeLevelFilter(query.Level)
	method := strings.ToUpper(strings.TrimSpace(query.Method))
	path := strings.TrimSpace(query.Path)
	out := make([]HTTPRequestLogEntry, 0, limit+1)
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if cursor != nil && !isBeforeCursor(item.CreatedAt, item.ID, *cursor) {
			continue
		}
		if level != "" && item.Level != level {
			continue
		}
		if method != "" && item.Method != method {
			continue
		}
		if path != "" && !strings.Contains(item.Path, path) {
			continue
		}
		if query.Status > 0 && item.Status != query.Status {
			continue
		}
		if query.DurationMinMs > 0 && item.DurationMs < query.DurationMinMs {
			continue
		}
		if query.DurationMaxMs > 0 && item.DurationMs > query.DurationMaxMs {
			continue
		}
		if !query.From.IsZero() && item.CreatedAt.Before(query.From.UTC()) {
			continue
		}
		if !query.To.IsZero() && item.CreatedAt.After(query.To.UTC()) {
			continue
		}
		out = append(out, sanitizeHTTPRequestLogEntry(item))
		if len(out) >= limit+1 {
			break
		}
	}
	page := HTTPRequestLogPage{Items: out}
	if len(out) > limit {
		page.NextCursor = encodeTimeCursor(out[limit-1].CreatedAt, out[limit-1].ID)
		page.Items = out[:limit]
	}
	return page
}

func (s *httpRequestLogStore) reset() {
	s.sequence.Store(0)
	s.buffer.reset()
}

func (s *httpRequestLogStore) get(id string) (HTTPRequestLogEntry, bool) {
	if id == "" {
		return HTTPRequestLogEntry{}, false
	}
	items := s.buffer.snapshot()
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].ID == id {
			return items[i], true
		}
	}
	return HTTPRequestLogEntry{}, false
}

func normalizeLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultQueryLimit
	case limit > maxQueryLimit:
		return maxQueryLimit
	default:
		return limit
	}
}

func normalizeStoredLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "warn":
		return "warning"
	case "info", "warning", "error", "debug", "critical":
		return strings.ToLower(strings.TrimSpace(level))
	case "":
		return "info"
	default:
		return strings.ToLower(strings.TrimSpace(level))
	}
}

func normalizeLevelFilter(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "all":
		return ""
	case "warn":
		return "warning"
	default:
		return strings.ToLower(strings.TrimSpace(level))
	}
}

func encodeTimeCursor(ts time.Time, id string) string {
	payload, _ := json.Marshal(timeCursor{
		Time: normalizeTime(ts).Format(time.RFC3339Nano),
		ID:   id,
	})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeTimeCursor(raw string) (*timeCursor, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	var cursor timeCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	if strings.TrimSpace(cursor.ID) == "" {
		return nil, fmt.Errorf("invalid cursor")
	}
	if _, err := time.Parse(time.RFC3339Nano, cursor.Time); err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	return &cursor, nil
}

func isBeforeCursor(ts time.Time, id string, cursor timeCursor) bool {
	cursorTime, err := time.Parse(time.RFC3339Nano, cursor.Time)
	if err != nil {
		return false
	}
	switch {
	case ts.Before(cursorTime):
		return true
	case ts.After(cursorTime):
		return false
	default:
		return id < cursor.ID
	}
}

func normalizeTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}

func sanitizeHTTPRequestLogEntry(entry HTTPRequestLogEntry) HTTPRequestLogEntry {
	entry.Query = redactQuery(entry.Query)
	entry.RemoteAddr = maskRemoteAddr(entry.RemoteAddr)
	entry.PanicMessage = redactSensitiveText(entry.PanicMessage)
	entry.Stack = redactSensitiveText(entry.Stack)
	return entry
}

func redactQuery(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, "&")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key := part
		if index := strings.Index(part, "="); index >= 0 {
			key = part[:index]
		}
		key = strings.TrimSpace(key)
		if key == "" {
			parts[i] = redactedText
			continue
		}
		parts[i] = key + "=REDACTED"
	}
	return strings.Join(parts, "&")
}

func maskRemoteAddr(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	host := raw
	if parsedHost, _, err := net.SplitHostPort(raw); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			return fmt.Sprintf("%d.%d.x.x", ipv4[0], ipv4[1])
		}
		parts := strings.Split(ip.String(), ":")
		for len(parts) < 2 {
			parts = append(parts, "*")
		}
		return parts[0] + ":" + parts[1] + ":*:*:*:*:*:*"
	}
	labels := strings.Split(host, ".")
	if len(labels) > 1 && strings.TrimSpace(labels[0]) != "" {
		return labels[0] + ".***"
	}
	if len(host) <= 2 {
		return "*"
	}
	return host[:1] + "***"
}

func redactSensitiveText(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	return redactedText
}

func appendSlogAttr(target map[string]any, groups []string, attr slog.Attr) {
	if attr.Equal(slog.Attr{}) {
		return
	}
	if attr.Value.Kind() == slog.KindGroup {
		nextGroups := append([]string{}, groups...)
		if attr.Key != "" {
			nextGroups = append(nextGroups, attr.Key)
		}
		for _, child := range attr.Value.Group() {
			appendSlogAttr(target, nextGroups, child)
		}
		return
	}
	if strings.TrimSpace(attr.Key) == "" {
		return
	}
	key := attr.Key
	if len(groups) > 0 {
		key = strings.Join(append(append([]string{}, groups...), attr.Key), ".")
	}
	target[key] = slogValueToAny(attr.Value)
}

func slogValueToAny(value slog.Value) any {
	switch value.Kind() {
	case slog.KindString:
		return value.String()
	case slog.KindInt64:
		return value.Int64()
	case slog.KindUint64:
		return value.Uint64()
	case slog.KindFloat64:
		return value.Float64()
	case slog.KindBool:
		return value.Bool()
	case slog.KindDuration:
		return value.Duration().Milliseconds()
	case slog.KindTime:
		return value.Time().UTC()
	case slog.KindAny:
		raw := value.Any()
		if err, ok := raw.(error); ok {
			return err.Error()
		}
		return raw
	default:
		return value.Any()
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func bumpAtomicSequence(sequence *atomic.Int64, id, prefix string) {
	raw := strings.TrimSpace(strings.TrimPrefix(id, prefix))
	if raw == "" {
		return
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return
	}
	for {
		current := sequence.Load()
		if current >= value {
			return
		}
		if sequence.CompareAndSwap(current, value) {
			return
		}
	}
}
