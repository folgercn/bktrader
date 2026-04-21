package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	_, _ = store.UpsertNotificationDelivery(notificationID, "telegram", "sent", "")
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
