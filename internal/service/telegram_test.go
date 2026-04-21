package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestTelegramNotificationRecovery(t *testing.T) {
	// 1. 设置 Mock Telegram Server
	var lastMsg string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p map[string]any
		json.Unmarshal(body, &p)
		lastMsg = p["text"].(string)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	// 劫持全局 URL
	oldURL := telegramBaseURL
	telegramBaseURL = server.URL
	defer func() { telegramBaseURL = oldURL }()

	store := memory.NewStore()
	p := &Platform{
		store: store,
		telegramConfig: domain.TelegramConfig{
			Enabled:    true,
			BotToken:   "test-token",
			ChatID:     "123",
			SendLevels: []string{"warning", "error"},
		},
	}

	// 2. 模拟场景：已发送告警，但现在不再活跃
	notificationID := "alert-123"
	alertTitle := "数据同步异常"

	// 模拟核心记录：
	// a. Store 中有已发送记录
	_, _ = store.UpsertNotificationDelivery(notificationID, "telegram", "sent", "", nil)
	// b. 内存缓存中有标题记录
	p.telegramSentAlertCache.Store(notificationID, alertTitle)
	// c. 当前活跃告警列表为空 (ListNotifications 会返回空)

	// 3. 执行调度
	err := p.DispatchTelegramNotifications()
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	// 4. 验证结果
	// a. 应该发送了 [已恢复] 消息
	if !strings.Contains(lastMsg, "✅ *[已恢复]*") {
		t.Fatalf("expected recovery message, got: %s", lastMsg)
	}
	if !strings.Contains(lastMsg, alertTitle) {
		t.Fatalf("expected alert title in recovery msg, got: %s", lastMsg)
	}

	// b. Store 记录应更新为 recovered
	deliveries, _ := store.ListNotificationDeliveries()
	found := false
	for _, d := range deliveries {
		if d.NotificationID == notificationID && d.Status == "recovered" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected delivery status to be 'recovered'")
	}

	// c. 内存缓存应被清理
	if _, ok := p.telegramSentAlertCache.Load(notificationID); ok {
		t.Fatal("expected alert to be removed from memory cache")
	}
}

func TestTelegramNotificationRecoveryRestart(t *testing.T) {
	// 这个测试模拟服务重启：内存缓存为空，但数据库中存有 metadata 标题
	var lastMsg string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p map[string]any
		json.Unmarshal(body, &p)
		lastMsg = p["text"].(string)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	oldURL := telegramBaseURL
	telegramBaseURL = server.URL
	defer func() { telegramBaseURL = oldURL }()

	store := memory.NewStore()
	p := &Platform{
		store: store,
		telegramConfig: domain.TelegramConfig{
			Enabled:    true,
			BotToken:   "test-token",
			ChatID:     "123",
			SendLevels: []string{"warning", "error"},
		},
	}

	notificationID := "alert-restart-456"
	alertTitle := "重启后恢复标题"

	// 1. 模拟场景：
	// a. Store 中有已发送记录，且包含 metadata 标题
	_, _ = store.UpsertNotificationDelivery(notificationID, "telegram", "sent", "", map[string]any{"title": alertTitle})
	// b. 内存缓存为空 (模拟重启)
	p.telegramSentAlertCache.Delete(notificationID)

	// 2. 执行调度
	if err := p.DispatchTelegramNotifications(); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	// 3. 验证回复内容包含持久化的标题
	if !strings.Contains(lastMsg, alertTitle) {
		t.Fatalf("expected persistent alert title in recovery msg, got: %s", lastMsg)
	}
	if !strings.Contains(lastMsg, "✅ *[已恢复]*") {
		t.Fatalf("expected recovery marker, got: %s", lastMsg)
	}
}

func TestTelegramDispatchSuppressesFlappingRuntimeStaleAlerts(t *testing.T) {
	var messages []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p map[string]any
		_ = json.Unmarshal(body, &p)
		messages = append(messages, p["text"].(string))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	oldURL := telegramBaseURL
	telegramBaseURL = server.URL
	defer func() { telegramBaseURL = oldURL }()

	oldNow := telegramNow
	base := time.Date(2026, 4, 21, 13, 0, 0, 0, time.UTC)
	now := base
	telegramNow = func() time.Time { return now }
	defer func() { telegramNow = oldNow }()

	store := memory.NewStore()
	p := &Platform{
		store: store,
		telegramConfig: domain.TelegramConfig{
			Enabled:    true,
			BotToken:   "test-token",
			ChatID:     "123",
			SendLevels: []string{"warning", "error"},
		},
	}

	item := domain.PlatformNotification{
		ID:     "runtime-stale-signal-runtime-1",
		Status: "active",
		Alert: domain.PlatformAlert{
			ID:               "runtime-stale-signal-runtime-1",
			Scope:            "runtime",
			Level:            "warning",
			Title:            "数据源过期",
			Detail:           "1 个数据源状态已陈旧",
			AccountName:      "Binance Testnet",
			StrategyID:       "2",
			RuntimeSessionID: "signal-runtime-1",
			EventTime:        base,
		},
		UpdatedAt: base,
	}

	pending, shouldSend, err := p.advanceTelegramFlapSuppressedActiveDelivery(item, domain.NotificationDelivery{}, false, now)
	if err != nil {
		t.Fatalf("seed pending delivery failed: %v", err)
	}
	if shouldSend {
		t.Fatal("expected first observation to stay pending")
	}
	if pending.Status != "pending" {
		t.Fatalf("expected pending status, got %s", pending.Status)
	}
	if len(messages) != 0 {
		t.Fatalf("expected no telegram message on first observation, got %#v", messages)
	}

	now = base.Add(30 * time.Second)
	pending, shouldSend, err = p.advanceTelegramFlapSuppressedActiveDelivery(item, pending, true, now)
	if err != nil {
		t.Fatalf("refresh pending delivery failed: %v", err)
	}
	if shouldSend {
		t.Fatal("expected flap-suppressed alert to stay pending before grace window")
	}
	if len(messages) != 0 {
		t.Fatalf("expected no telegram message before grace window, got %#v", messages)
	}

	now = base.Add(50 * time.Second)
	sent, shouldSend, err := p.advanceTelegramFlapSuppressedActiveDelivery(item, pending, true, now)
	if err != nil {
		t.Fatalf("send suppressed alert failed: %v", err)
	}
	if !shouldSend {
		t.Fatal("expected alert to send after grace window")
	}
	if sent.Status != "sent" {
		t.Fatalf("expected sent status, got %s", sent.Status)
	}
	if len(messages) != 1 || !strings.Contains(messages[0], "数据源过期") {
		t.Fatalf("expected one alert message after grace window, got %#v", messages)
	}

	now = base.Add(70 * time.Second)
	resolvePending, recovered, err := p.advanceTelegramFlapSuppressedRecoveredDelivery(sent, now)
	if err != nil {
		t.Fatalf("mark resolve pending failed: %v", err)
	}
	if recovered {
		t.Fatal("expected recovery to wait for stabilization window")
	}
	if resolvePending.Status != "resolve_pending" {
		t.Fatalf("expected resolve_pending status, got %s", resolvePending.Status)
	}

	now = base.Add(135 * time.Second)
	_, recovered, err = p.advanceTelegramFlapSuppressedRecoveredDelivery(resolvePending, now)
	if err != nil {
		t.Fatalf("send stabilized recovery failed: %v", err)
	}
	if !recovered {
		t.Fatal("expected stabilized recovery message after grace window")
	}
	if len(messages) != 2 || !strings.Contains(messages[1], "✅ *[已恢复]*") {
		t.Fatalf("expected recovery message after stabilization window, got %#v", messages)
	}
}
