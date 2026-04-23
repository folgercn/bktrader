package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestTelegramPositionReportWindowComprehensive(t *testing.T) {
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
	account, _ := store.GetAccount("live-main")
	// 初始状态：13:00 时有持仓
	account.Metadata = map[string]any{
		"liveSyncSnapshot": map[string]any{
			"syncStatus": "SYNCED",
			"syncedAt":   "2026-04-23T13:00:00Z",
			"positions": []map[string]any{{
				"symbol":      "BTCUSDT",
				"positionAmt": 0.013,
			}},
		},
	}
	store.UpdateAccount(account)

	p := &Platform{
		store: store,
		telegramConfig: domain.TelegramConfig{
			Enabled:                       true,
			BotToken:                      "test-token",
			ChatID:                        "123",
			PositionReportEnabled:         true,
			PositionReportIntervalMinutes: 30,
		},
	}

	deliveries := make(map[string]domain.NotificationDelivery)

	// 1. 窗口内: 13:04 (UTC) -> 应该发送 (5分钟窗口内)
	now1 := time.Date(2026, 4, 23, 13, 4, 0, 0, time.UTC)
	count1, _ := p.DispatchTelegramPositionReport(deliveries, now1)
	if count1 != 1 {
		t.Errorf("Expected report at 13:04 (within 5m window), got %d", count1)
	}
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	messages = nil // clear

	// 2. 窗口外: 13:10 (UTC) -> 假设这是下一个尝试，但不应该再发（即使 deliveries 里没有该桶）
	now2 := time.Date(2026, 4, 23, 13, 10, 0, 0, time.UTC)
	emptyDeliveries := make(map[string]domain.NotificationDelivery)
	count2, _ := p.DispatchTelegramPositionReport(emptyDeliveries, now2)
	if count2 != 0 {
		t.Errorf("Expected NO report at 13:10 (outside 5m window), got %d", count2)
	}
	if len(messages) != 0 {
		t.Errorf("Expected 0 messages at 13:10, got %d", len(messages))
	}

	// 3. 下一个周期: 13:31 (UTC) -> 应该恢复发送
	now3 := time.Date(2026, 4, 23, 13, 31, 0, 0, time.UTC)
	count3, _ := p.DispatchTelegramPositionReport(emptyDeliveries, now3)
	if count3 != 1 {
		t.Errorf("Expected report at 13:31 (next bucket), got %d", count3)
	}
	if len(messages) != 1 {
		t.Errorf("Expected 1 message at 13:31, got %d", len(messages))
	}
}
