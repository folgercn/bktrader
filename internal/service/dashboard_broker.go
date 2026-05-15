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

// DashboardDomain identifies a dashboard snapshot domain.
type DashboardDomain string

const (
	DashboardDomainLiveSessions          DashboardDomain = "live-sessions"
	DashboardDomainSignalRuntimeSessions DashboardDomain = "signal-runtime-sessions"
	DashboardDomainPositions             DashboardDomain = "positions"
	DashboardDomainOrders                DashboardDomain = "orders"
	DashboardDomainFills                 DashboardDomain = "fills"
	DashboardDomainAlerts                DashboardDomain = "alerts"
	DashboardDomainNotifications         DashboardDomain = "notifications"
	DashboardDomainMonitorHealth         DashboardDomain = "monitor-health"
)

type dashboardChange struct {
	Domain DashboardDomain
	Reason string
}

// DashboardBroker 负责仪表盘实时数据的事件分发。
// 采用"变更通知合并 + snapshot 推送"模式，保留轮询作为兜底触发源。
type DashboardBroker struct {
	mu             sync.RWMutex
	subscribers    map[int]chan DashboardEvent
	nextID         int
	seq            int64
	lastHashes     map[string]string
	platform       *Platform
	changes        chan dashboardChange
	fetchFuncs     map[DashboardDomain]func() (any, error)
	coalesceWindow time.Duration
}

func NewDashboardBroker(platform *Platform) *DashboardBroker {
	b := &DashboardBroker{
		subscribers:    make(map[int]chan DashboardEvent),
		lastHashes:     make(map[string]string),
		platform:       platform,
		changes:        make(chan dashboardChange, 128),
		fetchFuncs:     make(map[DashboardDomain]func() (any, error)),
		coalesceWindow: 300 * time.Millisecond,
	}
	b.registerDefaultFetchFuncs()
	return b
}

func (b *DashboardBroker) registerDefaultFetchFuncs() {
	if b.platform == nil {
		return
	}
	b.RegisterDashboardFetchFunc(DashboardDomainLiveSessions, func() (any, error) {
		return b.platform.ListLiveSessionsSummary()
	})
	b.RegisterDashboardFetchFunc(DashboardDomainSignalRuntimeSessions, func() (any, error) {
		return b.platform.ListSignalRuntimeSessionsSummary(), nil
	})
	b.RegisterDashboardFetchFunc(DashboardDomainPositions, func() (any, error) {
		return b.platform.ListPositions()
	})
	b.RegisterDashboardFetchFunc(DashboardDomainOrders, func() (any, error) {
		return b.platform.ListOrdersWithLimit(50, 0)
	})
	b.RegisterDashboardFetchFunc(DashboardDomainFills, func() (any, error) {
		return b.platform.ListFillsWithLimit(50, 0)
	})
	b.RegisterDashboardFetchFunc(DashboardDomainAlerts, func() (any, error) {
		return b.platform.ListAlerts()
	})
	b.RegisterDashboardFetchFunc(DashboardDomainNotifications, func() (any, error) {
		return b.platform.ListNotifications(true)
	})
	b.RegisterDashboardFetchFunc(DashboardDomainMonitorHealth, func() (any, error) {
		return b.platform.HealthSnapshot()
	})
}

// RegisterDashboardFetchFunc installs or replaces the snapshot fetcher for a dashboard domain.
func (b *DashboardBroker) RegisterDashboardFetchFunc(domain DashboardDomain, fetchData func() (any, error)) {
	if domain == "" || fetchData == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.fetchFuncs[domain] = fetchData
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

// NotifyChanged schedules a dashboard domain snapshot check without blocking the caller.
func (b *DashboardBroker) NotifyChanged(domain DashboardDomain, reason string) {
	select {
	case b.changes <- dashboardChange{Domain: domain, Reason: reason}:
	default:
	}
}

// StartEventLoop 合并短时间窗口内的 dashboard 变更信号并发布 snapshot。
func (b *DashboardBroker) StartEventLoop(ctx context.Context) {
	timer := time.NewTimer(b.coalesceWindow)
	if !timer.Stop() {
		<-timer.C
	}
	pending := make(map[DashboardDomain]struct{})
	timerActive := false

	flush := func() {
		for domain := range pending {
			b.publishSnapshotForDomain(domain)
			delete(pending, domain)
		}
	}

	for {
		select {
		case <-ctx.Done():
			if timerActive && !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case change := <-b.changes:
			if change.Domain == "" {
				continue
			}
			pending[change.Domain] = struct{}{}
			if !timerActive {
				timer.Reset(b.coalesceWindow)
				timerActive = true
			}
		case <-timer.C:
			timerActive = false
			flush()
		}
	}
}

// publishSnapshotForDomain 检测指定 domain 数据变更并推送 snapshot。
func (b *DashboardBroker) publishSnapshotForDomain(domain DashboardDomain) {
	b.mu.RLock()
	hasSubscribers := len(b.subscribers) > 0
	fetchData := b.fetchFuncs[domain]
	b.mu.RUnlock()

	if !hasSubscribers || fetchData == nil {
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

	eventType := string(domain)
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
	startTicker := func(intervalMs int, domain DashboardDomain) {
		interval := time.Duration(intervalMs) * time.Millisecond
		ticker := time.NewTicker(interval)
		go func() {
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					b.NotifyChanged(domain, "polling")
				}
			}
		}()
	}

	startTicker(cfg.DashboardLiveSessionsPollMs, DashboardDomainLiveSessions)
	startTicker(cfg.DashboardLiveSessionsPollMs, DashboardDomainSignalRuntimeSessions)
	startTicker(cfg.DashboardPositionsPollMs, DashboardDomainPositions)
	startTicker(cfg.DashboardOrdersPollMs, DashboardDomainOrders)
	startTicker(cfg.DashboardFillsPollMs, DashboardDomainFills)
	startTicker(cfg.DashboardAlertsPollMs, DashboardDomainAlerts)
	startTicker(cfg.DashboardNotificationsPollMs, DashboardDomainNotifications)
	startTicker(cfg.DashboardMonitorHealthPollMs, DashboardDomainMonitorHealth)
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
	b.pushLatestIfAvailable(DashboardDomainLiveSessions, ch)
	b.pushLatestIfAvailable(DashboardDomainSignalRuntimeSessions, ch)
	b.pushLatestIfAvailable(DashboardDomainPositions, ch)
	b.pushLatestIfAvailable(DashboardDomainOrders, ch)
	b.pushLatestIfAvailable(DashboardDomainFills, ch)
	b.pushLatestIfAvailable(DashboardDomainAlerts, ch)
	b.pushLatestIfAvailable(DashboardDomainNotifications, ch)
	b.pushLatestIfAvailable(DashboardDomainMonitorHealth, ch)
}

func (b *DashboardBroker) pushLatestIfAvailable(domain DashboardDomain, ch chan<- DashboardEvent) {
	b.mu.RLock()
	fetchData := b.fetchFuncs[domain]
	b.mu.RUnlock()
	if fetchData == nil {
		return
	}

	data, err := fetchData()
	if err != nil {
		return
	}

	eventType := string(domain)

	b.mu.Lock()
	b.seq++
	seq := b.seq
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
