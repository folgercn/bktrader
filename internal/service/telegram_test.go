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

func TestTelegramDispatchSendsFilledTradeEventsWithPnLAndDedup(t *testing.T) {
	messages := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p map[string]any
		json.Unmarshal(body, &p)
		messages = append(messages, p["text"].(string))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	oldURL := telegramBaseURL
	telegramBaseURL = server.URL
	defer func() { telegramBaseURL = oldURL }()

	store := memory.NewStore()
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	_, err := store.CreateOrderExecutionEvent(domain.OrderExecutionEvent{
		ID:              "open-event-1",
		OrderID:         "order-open-1",
		AccountID:       "account-live-1",
		LiveSessionID:   "live-session-1",
		Symbol:          "BTCUSDT",
		Side:            "BUY",
		EventType:       "filled",
		Status:          "FILLED",
		Quantity:        0.2,
		Price:           64000,
		EventTime:       now,
		ExecutionMode:   "live",
		DispatchSummary: map[string]any{"role": "entry"},
	})
	if err != nil {
		t.Fatalf("create open event failed: %v", err)
	}
	_, err = store.CreateOrderExecutionEvent(domain.OrderExecutionEvent{
		ID:              "close-event-1",
		OrderID:         "order-close-1",
		AccountID:       "account-live-1",
		LiveSessionID:   "live-session-1",
		Symbol:          "BTCUSDT",
		Side:            "SELL",
		EventType:       "filled",
		Status:          "FILLED",
		Quantity:        0.1,
		Price:           65000,
		EventTime:       now.Add(time.Minute),
		ExecutionMode:   "live",
		ReduceOnly:      true,
		AdapterSync:     map[string]any{"totalRealizedPnl": 100.5, "totalFee": 1.25},
		DispatchSummary: map[string]any{"role": "exit"},
	})
	if err != nil {
		t.Fatalf("create close event failed: %v", err)
	}

	p := &Platform{
		store: store,
		telegramConfig: domain.TelegramConfig{
			Enabled:                       true,
			BotToken:                      "test-token",
			ChatID:                        "123",
			SendLevels:                    []string{},
			TradeEventsEnabled:            true,
			PositionReportIntervalMinutes: 30,
		},
	}

	if err := p.DispatchTelegramNotifications(); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 trade messages, got %d: %#v", len(messages), messages)
	}
	joined := strings.Join(messages, "\n---\n")
	if !strings.Contains(joined, "*开仓成交* BTCUSDT BUY") {
		t.Fatalf("expected open trade message, got: %s", joined)
	}
	if !strings.Contains(joined, "数量: 0.2") || !strings.Contains(joined, "价格: 64000") {
		t.Fatalf("expected open qty and price, got: %s", joined)
	}
	if !strings.Contains(joined, "*平仓成交* BTCUSDT SELL") || !strings.Contains(joined, "已实现盈亏: +100.5") {
		t.Fatalf("expected close pnl message, got: %s", joined)
	}
	if err := p.DispatchTelegramNotifications(); err != nil {
		t.Fatalf("second dispatch failed: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected trade delivery dedupe, got %d messages", len(messages))
	}
}

func TestTelegramPositionReportUsesThirtyMinuteBucketAndSkipsRecovery(t *testing.T) {
	messages := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p map[string]any
		json.Unmarshal(body, &p)
		messages = append(messages, p["text"].(string))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	oldURL := telegramBaseURL
	telegramBaseURL = server.URL
	defer func() { telegramBaseURL = oldURL }()

	base := time.Date(2026, 4, 22, 10, 5, 0, 0, time.UTC)
	oldNow := telegramNow
	telegramNow = func() time.Time { return base }
	defer func() { telegramNow = oldNow }()

	store := memory.NewStore()
	accounts, err := store.ListAccounts()
	if err != nil {
		t.Fatalf("list accounts failed: %v", err)
	}
	var account domain.Account
	for _, item := range accounts {
		if strings.EqualFold(item.Mode, "LIVE") {
			account = item
			break
		}
	}
	if account.ID == "" {
		t.Fatal("expected default live account")
	}
	account.Metadata = map[string]any{
		"lastLiveSyncAt": base.Format(time.RFC3339),
		"liveSyncSnapshot": map[string]any{
			"syncStatus":            "SYNCED",
			"syncedAt":              base.Format(time.RFC3339),
			"totalMarginBalance":    12000.0,
			"availableBalance":      8000.0,
			"totalWalletBalance":    11900.0,
			"totalUnrealizedProfit": 100.0,
			"positions": []map[string]any{{
				"symbol":           "ETHUSDT",
				"positionAmt":      1.5,
				"entryPrice":       3000.0,
				"markPrice":        3100.0,
				"unrealizedProfit": 150.0,
				"positionSide":     "LONG",
			}},
		},
	}
	if _, err := store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	p := &Platform{
		store: store,
		telegramConfig: domain.TelegramConfig{
			Enabled:                       true,
			BotToken:                      "test-token",
			ChatID:                        "123",
			SendLevels:                    []string{},
			PositionReportEnabled:         true,
			PositionReportIntervalMinutes: 30,
		},
	}

	if err := p.DispatchTelegramNotifications(); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected one position report, got %d: %#v", len(messages), messages)
	}
	if !strings.Contains(messages[0], "*持仓定时播报* 30分钟") || !strings.Contains(messages[0], "ETHUSDT LONG 数量:1.5") || !strings.Contains(messages[0], "浮盈亏:+150") {
		t.Fatalf("unexpected position report: %s", messages[0])
	}
	if err := p.DispatchTelegramNotifications(); err != nil {
		t.Fatalf("second dispatch failed: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected position report delivery dedupe, got %d messages", len(messages))
	}
	telegramNow = func() time.Time { return base.Add(31 * time.Minute) }
	if err := p.DispatchTelegramNotifications(); err != nil {
		t.Fatalf("third dispatch failed: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected next bucket position report, got %d messages", len(messages))
	}
	for _, msg := range messages {
		if strings.Contains(msg, "[已恢复]") {
			t.Fatalf("position report must not trigger recovery message: %s", msg)
		}
	}
}

func TestTelegramPositionReportSkipsEmptyPositions(t *testing.T) {
	messages := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var p map[string]any
		json.Unmarshal(body, &p)
		messages = append(messages, p["text"].(string))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	oldURL := telegramBaseURL
	telegramBaseURL = server.URL
	defer func() { telegramBaseURL = oldURL }()

	base := time.Date(2026, 4, 22, 10, 5, 0, 0, time.UTC)
	oldNow := telegramNow
	telegramNow = func() time.Time { return base }
	defer func() { telegramNow = oldNow }()

	store := memory.NewStore()
	accounts, err := store.ListAccounts()
	if err != nil {
		t.Fatalf("list accounts failed: %v", err)
	}
	var account domain.Account
	for _, item := range accounts {
		if strings.EqualFold(item.Mode, "LIVE") {
			account = item
			break
		}
	}
	if account.ID == "" {
		t.Fatal("expected default live account")
	}
	account.Metadata = map[string]any{
		"lastLiveSyncAt": base.Format(time.RFC3339),
		"liveSyncSnapshot": map[string]any{
			"syncStatus":            "SYNCED",
			"syncedAt":              base.Format(time.RFC3339),
			"totalMarginBalance":    12000.0,
			"availableBalance":      8000.0,
			"totalWalletBalance":    12000.0,
			"totalUnrealizedProfit": 0.0,
			"positions": []map[string]any{{
				"symbol":       "ETHUSDT",
				"positionAmt":  0.0,
				"entryPrice":   0.0,
				"markPrice":    0.0,
				"positionSide": "BOTH",
			}},
		},
	}
	if _, err := store.UpdateAccount(account); err != nil {
		t.Fatalf("update account failed: %v", err)
	}

	p := &Platform{
		store: store,
		telegramConfig: domain.TelegramConfig{
			Enabled:                       true,
			BotToken:                      "test-token",
			ChatID:                        "123",
			SendLevels:                    []string{},
			PositionReportEnabled:         true,
			PositionReportIntervalMinutes: 30,
		},
	}

	if err := p.DispatchTelegramNotifications(); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected empty position report to be skipped, got %#v", messages)
	}
	deliveries, err := store.ListNotificationDeliveries()
	if err != nil {
		t.Fatalf("list deliveries failed: %v", err)
	}
	for _, delivery := range deliveries {
		if strings.HasPrefix(delivery.NotificationID, "position-report:") {
			t.Fatalf("expected no position report delivery for empty positions, got %#v", delivery)
		}
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

func TestTelegramFlapSuppressionResetsFirstActiveAtAfterRecoveredDelivery(t *testing.T) {
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
	oldActiveAt := time.Date(2026, 4, 22, 3, 59, 25, 0, time.UTC)
	newActiveAt := oldActiveAt.Add(6 * time.Minute)
	item := domain.PlatformNotification{
		ID:     "runtime-stale-signal-runtime-1",
		Status: "active",
		Alert: domain.PlatformAlert{
			ID:               "runtime-stale-signal-runtime-1",
			Scope:            "runtime",
			Level:            "warning",
			Title:            "数据源过期",
			Detail:           "1 个数据源状态已陈旧",
			RuntimeSessionID: "signal-runtime-1",
			EventTime:        newActiveAt,
		},
		UpdatedAt: newActiveAt,
	}
	recoveredDelivery := domain.NotificationDelivery{
		NotificationID: item.ID,
		Channel:        "telegram",
		Status:         "recovered",
		Metadata: map[string]any{
			"firstActiveAt": oldActiveAt.Format(time.RFC3339),
			"title":         "数据源过期",
		},
	}

	pending, shouldSend, err := p.advanceTelegramFlapSuppressedActiveDelivery(item, recoveredDelivery, true, newActiveAt)
	if err != nil {
		t.Fatalf("reactivate recovered delivery failed: %v", err)
	}
	if shouldSend {
		t.Fatal("expected reactivated recovered delivery to reset grace window")
	}
	if pending.Status != "pending" {
		t.Fatalf("expected pending status, got %s", pending.Status)
	}
	if got := stringValue(pending.Metadata["firstActiveAt"]); got != newActiveAt.Format(time.RFC3339) {
		t.Fatalf("expected firstActiveAt reset to new active time, got %s", got)
	}

	pending, shouldSend, err = p.advanceTelegramFlapSuppressedActiveDelivery(item, pending, true, newActiveAt.Add(30*time.Second))
	if err != nil {
		t.Fatalf("refresh reactivated pending delivery failed: %v", err)
	}
	if shouldSend {
		t.Fatal("expected reactivated alert to remain pending before fresh grace window")
	}
	if len(messages) != 0 {
		t.Fatalf("expected no telegram message before fresh grace window, got %#v", messages)
	}

	_, shouldSend, err = p.advanceTelegramFlapSuppressedActiveDelivery(item, pending, true, newActiveAt.Add(50*time.Second))
	if err != nil {
		t.Fatalf("send reactivated pending delivery failed: %v", err)
	}
	if !shouldSend {
		t.Fatal("expected reactivated alert to send after fresh grace window")
	}
}

func TestTelegramDispatchSuppressesTransientRuntimeRecoveringAlert(t *testing.T) {
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

	store := memory.NewStore()
	p := &Platform{store: store}
	base := time.Date(2026, 4, 22, 3, 51, 53, 0, time.UTC)
	item := domain.PlatformNotification{
		ID:     "runtime-recovering-signal-runtime-1",
		Status: "active",
		Alert: domain.PlatformAlert{
			ID:               "runtime-recovering-signal-runtime-1",
			Scope:            "runtime",
			Level:            "warning",
			Title:            "运行时恢复中",
			Detail:           "尝试次数 1/3: unexpected EOF",
			RuntimeSessionID: "signal-runtime-1",
			EventTime:        base,
		},
		UpdatedAt: base,
	}

	pending, shouldSend, err := p.advanceTelegramFlapSuppressedActiveDelivery(item, domain.NotificationDelivery{}, false, base)
	if err != nil {
		t.Fatalf("seed runtime recovering pending delivery failed: %v", err)
	}
	if shouldSend {
		t.Fatal("expected transient runtime recovering alert to stay pending")
	}

	recovered, sentRecovery, err := p.advanceTelegramFlapSuppressedRecoveredDelivery(pending, base.Add(15*time.Second))
	if err != nil {
		t.Fatalf("recover pending runtime recovering delivery failed: %v", err)
	}
	if sentRecovery {
		t.Fatal("expected unsent pending runtime recovering alert to recover silently")
	}
	if recovered.Status != "recovered" {
		t.Fatalf("expected recovered status, got %s", recovered.Status)
	}
	if len(messages) != 0 {
		t.Fatalf("expected no telegram messages for transient runtime recovering flap, got %#v", messages)
	}
}

func TestTelegramAlertNeedsFlapSuppressionUsesStableLiveWarningID(t *testing.T) {
	alert := domain.PlatformAlert{
		ID:     "live-warning-stale-source-states-account-1",
		Scope:  "live",
		Title:  "任意文案",
		Detail: "任意细节",
	}
	if !telegramAlertNeedsFlapSuppression(alert) {
		t.Fatal("expected live stale-source warning id prefix to enable flap suppression")
	}

	alert.ID = "live-warning-account-1"
	if telegramAlertNeedsFlapSuppression(alert) {
		t.Fatal("expected generic live warning id not to enable flap suppression")
	}
}

func TestTelegramAlertNeedsFlapSuppressionForTransientRuntimeRecoveryIDs(t *testing.T) {
	cases := []domain.PlatformAlert{
		{ID: "runtime-recovering-signal-runtime-1", Scope: "runtime"},
		{ID: "live-preflight-runtime-error-account-1", Scope: "live"},
	}
	for _, alert := range cases {
		if !telegramAlertNeedsFlapSuppression(alert) {
			t.Fatalf("expected %s to enable flap suppression", alert.ID)
		}
	}

	nonSuppressed := domain.PlatformAlert{ID: "live-preflight-account-1", Scope: "live"}
	if telegramAlertNeedsFlapSuppression(nonSuppressed) {
		t.Fatal("expected generic live preflight alert not to enable flap suppression")
	}
}
