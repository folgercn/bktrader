package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

var telegramBaseURL = "https://api.telegram.org"

func (p *Platform) SendNotificationToTelegram(notificationID string) error {
	logger := p.logger("service.telegram", "notification_id", strings.TrimSpace(notificationID))
	notifications, err := p.ListNotifications(true)
	if err != nil {
		logger.Warn("list notifications failed", "error", err)
		return err
	}
	for _, item := range notifications {
		if item.ID != strings.TrimSpace(notificationID) {
			continue
		}
		if err := p.sendTelegramMessage(formatTelegramNotification(item)); err != nil {
			_, _ = p.store.UpsertNotificationDelivery(item.ID, "telegram", "failed", err.Error())
			logger.Warn("send telegram notification failed", "error", err)
			return err
		}
		_, _ = p.store.UpsertNotificationDelivery(item.ID, "telegram", "sent", "")
		logger.Info("telegram notification sent", "level", item.Alert.Level)
		return nil
	}
	logger.Warn("telegram notification not found")
	return fmt.Errorf("notification not found: %s", notificationID)
}

func (p *Platform) SendTelegramTestMessage() error {
	p.logger("service.telegram").Info("sending telegram test message")
	return p.sendTelegramMessage("bkTrader Telegram channel test\n\nTelegram 通知通道已连通。")
}

func (p *Platform) sendTelegramMessage(text string) error {
	config := p.telegramConfig
	if !config.Enabled {
		return fmt.Errorf("telegram channel is disabled")
	}
	if strings.TrimSpace(config.BotToken) == "" || strings.TrimSpace(config.ChatID) == "" {
		return fmt.Errorf("telegram bot token or chat id is missing")
	}

	payload := map[string]any{
		"chat_id": config.ChatID,
		"text":    text,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/bot%s/sendMessage", telegramBaseURL, config.BotToken), bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram send failed: status=%s", resp.Status)
	}
	return nil
}

func formatTelegramNotification(item domain.PlatformNotification) string {
	alert := item.Alert
	lines := []string{
		fmt.Sprintf("[%s] %s", strings.ToUpper(alert.Level), alert.Title),
		alert.Detail,
	}
	if alert.Scope != "" {
		lines = append(lines, fmt.Sprintf("范围: %s", alert.Scope))
	}
	if alert.AccountName != "" || alert.AccountID != "" {
		lines = append(lines, fmt.Sprintf("账户: %s", firstNonEmpty(alert.AccountName, alert.AccountID)))
	}
	if alert.StrategyName != "" || alert.StrategyID != "" {
		lines = append(lines, fmt.Sprintf("策略: %s", firstNonEmpty(alert.StrategyName, alert.StrategyID)))
	}
	if alert.RuntimeSessionID != "" {
		lines = append(lines, fmt.Sprintf("运行时: %s", alert.RuntimeSessionID))
	}
	if alert.PaperSessionID != "" {
		lines = append(lines, fmt.Sprintf("模拟盘: %s", alert.PaperSessionID))
	}
	if !alert.EventTime.IsZero() {
		lines = append(lines, fmt.Sprintf("时间: %s", alert.EventTime.Format(time.RFC3339)))
	}
	return strings.Join(lines, "\n")
}

func (p *Platform) StartTelegramDispatcher(ctx context.Context) {
	p.logger("service.telegram").Info("starting telegram dispatcher")
	go p.runTelegramDispatcher(ctx)
}

func (p *Platform) runTelegramDispatcher(ctx context.Context) {
	logger := p.logger("service.telegram")
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("telegram dispatcher stopped")
			return
		case <-ticker.C:
			_ = p.DispatchTelegramNotifications()
		}
	}
}

func (p *Platform) DispatchTelegramNotifications() error {
	logger := p.logger("service.telegram")
	config := p.telegramConfig
	if !config.Enabled || strings.TrimSpace(config.BotToken) == "" || strings.TrimSpace(config.ChatID) == "" {
		logger.Debug("telegram dispatcher skipped because channel is not configured")
		return nil
	}
	notifications, err := p.ListNotifications(false)
	if err != nil {
		logger.Warn("list notifications failed", "error", err)
		return err
	}
	deliveries, err := p.store.ListNotificationDeliveries()
	if err != nil {
		logger.Warn("list notification deliveries failed", "error", err)
		return err
	}
	delivered := make(map[string]struct{}, len(deliveries))
	for _, item := range deliveries {
		if strings.EqualFold(item.Channel, "telegram") && strings.EqualFold(item.Status, "sent") {
			delivered[item.NotificationID] = struct{}{}
		}
	}
	allowedLevels := make(map[string]struct{}, len(config.SendLevels))
	for _, level := range config.SendLevels {
		allowedLevels[strings.ToLower(strings.TrimSpace(level))] = struct{}{}
	}
	var firstErr error
	sentCount := 0
	activeNotificationIDs := make(map[string]struct{}, len(notifications))
	for _, item := range notifications {
		activeNotificationIDs[item.ID] = struct{}{}
		level := strings.ToLower(strings.TrimSpace(item.Alert.Level))
		if _, ok := allowedLevels[level]; !ok {
			continue
		}
		if _, ok := delivered[item.ID]; ok {
			// 如果已经发送过，确保缓存中有标题（用于后续恢复）
			p.telegramSentAlertCache.Store(item.ID, item.Alert.Title)
			continue
		}
		if err := p.sendTelegramMessage(formatTelegramNotification(item)); err != nil {
			_, _ = p.store.UpsertNotificationDelivery(item.ID, "telegram", "failed", err.Error())
			if firstErr == nil {
				firstErr = err
			}
			p.logger("service.telegram", "notification_id", item.ID).Warn("send telegram notification failed", "error", err)
			continue
		}
		// 记录到缓存用于恢复通知
		p.telegramSentAlertCache.Store(item.ID, item.Alert.Title)
		if _, err := p.store.UpsertNotificationDelivery(item.ID, "telegram", "sent", ""); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			p.logger("service.telegram", "notification_id", item.ID).Warn("record telegram delivery failed", "error", err)
			continue
		}
		sentCount++
	}

	// 恢复检测：遍历已发送的 delivery
	recoveredCount := 0
	for _, delivery := range deliveries {
		if !strings.EqualFold(delivery.Channel, "telegram") || !strings.EqualFold(delivery.Status, "sent") {
			continue
		}
		// 如果该告警 ID 不再活跃，说明已恢复
		if _, isActive := activeNotificationIDs[delivery.NotificationID]; !isActive {
			titleRaw, ok := p.telegramSentAlertCache.Load(delivery.NotificationID)
			title := "未知告警"
			if ok {
				title = titleRaw.(string)
			}

			recoveryMsg := fmt.Sprintf("✅ *[已恢复]* %s\n告警已自动解除。ID: %s", title, delivery.NotificationID)
			if err := p.sendTelegramMessage(recoveryMsg); err != nil {
				p.logger("service.telegram", "notification_id", delivery.NotificationID).Warn("send telegram recovery notification failed", "error", err)
				continue
			}

			// 标记为已恢复，防止重复发送
			if _, err := p.store.UpsertNotificationDelivery(delivery.NotificationID, "telegram", "recovered", ""); err != nil {
				p.logger("service.telegram", "notification_id", delivery.NotificationID).Warn("record telegram recovery delivery failed", "error", err)
			}
			p.telegramSentAlertCache.Delete(delivery.NotificationID)
			recoveredCount++
		}
	}

	if sentCount > 0 || recoveredCount > 0 {
		logger.Debug("telegram dispatch cycle completed", "sent", sentCount, "recovered", recoveredCount, "active", len(notifications))
	}
	return firstErr
}
