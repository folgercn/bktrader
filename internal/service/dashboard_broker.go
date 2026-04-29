package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
)

// DashboardEvent 代表推送到仪表盘的 SSE 事件
type DashboardEvent struct {
	Seq       int64     `json:"seq"`
	Type      string    `json:"type"`
	Action    string    `json:"action"`
	Payload   any       `json:"payload"`
	Timestamp time.Time `json:"timestamp"`
}

// DashboardBroker 负责仪表盘实时数据的事件分发。
// 采用"轮询检测变更 + 增量推送"模式。
type DashboardBroker struct {
	mu          sync.RWMutex
	subscribers map[int]chan DashboardEvent
	nextID      int
	seq         int64
	lastHashes  map[string]string
	platform    *Platform
}

func NewDashboardBroker(platform *Platform) *DashboardBroker {
	return &DashboardBroker{
		subscribers: make(map[int]chan DashboardEvent),
		lastHashes:  make(map[string]string),
		platform:    platform,
	}
}

func (b *DashboardBroker) Subscribe(buffer int) (int, <-chan DashboardEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	id := b.nextID
	ch := make(chan DashboardEvent, buffer)
	b.subscribers[id] = ch
	return id, ch
}

func (b *DashboardBroker) Unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.subscribers[id]; ok {
		delete(b.subscribers, id)
		close(ch)
	}
}

func (b *DashboardBroker) publish(eventType string, action string, payload any) {
	b.mu.Lock()
	b.seq++
	event := DashboardEvent{
		Seq:       b.seq,
		Type:      eventType,
		Action:    action,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}
	subscribers := make([]chan DashboardEvent, 0, len(b.subscribers))
	for _, ch := range b.subscribers {
		subscribers = append(subscribers, ch)
	}
	b.mu.Unlock()

	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
			// drop if full
		}
	}
}

// hashPayload 计算数据的 SHA256 哈希值
func hashPayload(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// checkAndPublish 检测数据变更并推送
func (b *DashboardBroker) checkAndPublish(eventType string, fetchData func() (any, error)) {
	b.mu.RLock()
	hasSubscribers := len(b.subscribers) > 0
	b.mu.RUnlock()

	if !hasSubscribers {
		return
	}

	data, err := fetchData()
	if err != nil {
		return
	}
	hash := hashPayload(data)
	if hash == "" {
		return
	}

	b.mu.RLock()
	lastHash := b.lastHashes[eventType]
	b.mu.RUnlock()

	if lastHash != hash {
		b.publish(eventType, "snapshot", data)
		b.mu.Lock()
		b.lastHashes[eventType] = hash
		b.mu.Unlock()
	}
}

// StartPolling 启动轮询检查
func (b *DashboardBroker) StartPolling(ctx context.Context, cfg config.Config) {
	startTicker := func(intervalMs int, eventType string, fetchData func() (any, error)) {
		interval := time.Duration(intervalMs) * time.Millisecond
		ticker := time.NewTicker(interval)
		go func() {
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					b.checkAndPublish(eventType, fetchData)
				}
			}
		}()
	}

	startTicker(cfg.DashboardLiveSessionsPollMs, "live-sessions", func() (any, error) {
		return b.platform.ListLiveSessionsSummary()
	})
	startTicker(cfg.DashboardLiveSessionsPollMs, "signal-runtime-sessions", func() (any, error) {
		return b.platform.ListSignalRuntimeSessionsSummary(), nil
	})
	startTicker(cfg.DashboardPositionsPollMs, "positions", func() (any, error) {
		return b.platform.ListPositions()
	})
	startTicker(cfg.DashboardOrdersPollMs, "orders", func() (any, error) {
		return b.platform.ListOrdersWithLimit(50, 0)
	})
	startTicker(cfg.DashboardFillsPollMs, "fills", func() (any, error) {
		return b.platform.ListFillsWithLimit(50, 0)
	})
	startTicker(cfg.DashboardAlertsPollMs, "alerts", func() (any, error) {
		return b.platform.ListAlerts()
	})
	startTicker(cfg.DashboardNotificationsPollMs, "notifications", func() (any, error) {
		return b.platform.ListNotifications(true)
	})
	startTicker(cfg.DashboardMonitorHealthPollMs, "monitor-health", func() (any, error) {
		return b.platform.HealthSnapshot()
	})
}

// PushInitialSnapshot 手动为某个连接推送当前最新数据
func (b *DashboardBroker) PushInitialSnapshot(id int) {
	b.mu.RLock()
	ch, ok := b.subscribers[id]
	b.mu.RUnlock()
	if !ok {
		return
	}

	// Push latest state to new subscriber
	// This ensures they don't have to wait for the next change
	b.pushLatestIfAvailable("live-sessions", func() (any, error) { return b.platform.ListLiveSessionsSummary() }, ch)
	b.pushLatestIfAvailable("signal-runtime-sessions", func() (any, error) {
		return b.platform.ListSignalRuntimeSessionsSummary(), nil
	}, ch)
	b.pushLatestIfAvailable("positions", func() (any, error) { return b.platform.ListPositions() }, ch)
	b.pushLatestIfAvailable("orders", func() (any, error) { return b.platform.ListOrdersWithLimit(50, 0) }, ch)
	b.pushLatestIfAvailable("fills", func() (any, error) { return b.platform.ListFillsWithLimit(50, 0) }, ch)
	b.pushLatestIfAvailable("alerts", func() (any, error) { return b.platform.ListAlerts() }, ch)
	b.pushLatestIfAvailable("notifications", func() (any, error) { return b.platform.ListNotifications(true) }, ch)
	b.pushLatestIfAvailable("monitor-health", func() (any, error) { return b.platform.HealthSnapshot() }, ch)
}

func (b *DashboardBroker) pushLatestIfAvailable(eventType string, fetchData func() (any, error), ch chan<- DashboardEvent) {
	data, err := fetchData()
	if err != nil {
		return
	}

	hash := hashPayload(data)

	b.mu.Lock()
	b.seq++
	seq := b.seq
	if hash != "" {
		b.lastHashes[eventType] = hash
	}
	b.mu.Unlock()

	event := DashboardEvent{
		Seq:       seq,
		Type:      eventType,
		Action:    "snapshot",
		Payload:   data,
		Timestamp: time.Now().UTC(),
	}
	select {
	case ch <- event:
	default:
	}
}
