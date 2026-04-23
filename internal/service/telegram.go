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
var telegramNow = func() time.Time { return time.Now().UTC() }
var telegramBeijingLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}()

const (
	telegramFlapSendGrace    = 45 * time.Second
	telegramFlapRecoverGrace = 60 * time.Second
)

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
			_, _ = p.store.UpsertNotificationDelivery(item.ID, "telegram", "failed", err.Error(), nil)
			logger.Warn("send telegram notification failed", "error", err)
			return err
		}
		_, _ = p.store.UpsertNotificationDelivery(item.ID, "telegram", "sent", "", map[string]any{"title": item.Alert.Title})
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

	timeoutSec := p.runtimePolicy.TelegramHTTPTimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 8
	}
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
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
	metadata := mapValue(alert.Metadata)
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
	if liveSessionID := strings.TrimSpace(stringValue(metadata["liveSessionId"])); liveSessionID != "" {
		lines = append(lines, fmt.Sprintf("实盘会话: %s", liveSessionID))
	}
	if orderID := strings.TrimSpace(stringValue(metadata["orderId"])); orderID != "" {
		lines = append(lines, fmt.Sprintf("订单: %s", orderID))
	}
	if alert.PaperSessionID != "" {
		lines = append(lines, fmt.Sprintf("模拟盘: %s", alert.PaperSessionID))
	}
	if !alert.EventTime.IsZero() {
		lines = append(lines, fmt.Sprintf("北京时间: %s", formatTelegramBeijingTime(alert.EventTime)))
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
	now := telegramNow()
	deliveryByID := make(map[string]domain.NotificationDelivery, len(deliveries))
	for _, item := range deliveries {
		if strings.EqualFold(item.Channel, "telegram") {
			deliveryByID[item.NotificationID] = item
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
		delivery, hasDelivery := deliveryByID[item.ID]
		if telegramAlertNeedsFlapSuppression(item.Alert) {
			nextDelivery, shouldSend, sendErr := p.advanceTelegramFlapSuppressedActiveDelivery(item, delivery, hasDelivery, now)
			if sendErr != nil {
				_, _ = p.store.UpsertNotificationDelivery(item.ID, "telegram", "failed", sendErr.Error(), nil)
				if firstErr == nil {
					firstErr = sendErr
				}
				p.logger("service.telegram", "notification_id", item.ID).Warn("send telegram notification failed", "error", sendErr)
				continue
			}
			if nextDelivery.NotificationID != "" {
				deliveryByID[item.ID] = nextDelivery
			}
			if strings.EqualFold(nextDelivery.Status, "sent") {
				p.telegramSentAlertCache.Store(item.ID, item.Alert.Title)
			}
			if !shouldSend {
				continue
			}
			sentCount++
			continue
		}
		if hasDelivery && strings.EqualFold(delivery.Status, "sent") {
			// 如果已经发送过，确保缓存中有标题（用于后续恢复）
			p.telegramSentAlertCache.Store(item.ID, item.Alert.Title)
			continue
		}
		if err := p.sendTelegramMessage(formatTelegramNotification(item)); err != nil {
			_, _ = p.store.UpsertNotificationDelivery(item.ID, "telegram", "failed", err.Error(), nil)
			if firstErr == nil {
				firstErr = err
			}
			p.logger("service.telegram", "notification_id", item.ID).Warn("send telegram notification failed", "error", err)
			continue
		}
		// 记录到缓存用于恢复通知
		p.telegramSentAlertCache.Store(item.ID, item.Alert.Title)
		if _, err := p.store.UpsertNotificationDelivery(item.ID, "telegram", "sent", "", map[string]any{"title": item.Alert.Title}); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			p.logger("service.telegram", "notification_id", item.ID).Warn("record telegram delivery failed", "error", err)
			continue
		}
		sentCount++
	}

	tradeEventCount, tradeEventErr := p.DispatchTelegramTradeEvents(deliveryByID)
	if tradeEventErr != nil && firstErr == nil {
		firstErr = tradeEventErr
	}
	positionReportCount, positionReportErr := p.DispatchTelegramPositionReport(deliveryByID, now)
	if positionReportErr != nil && firstErr == nil {
		firstErr = positionReportErr
	}

	// 恢复检测：遍历已发送的 delivery
	recoveredCount := 0
	for _, delivery := range deliveries {
		if !strings.EqualFold(delivery.Channel, "telegram") {
			continue
		}
		if telegramDeliveryIsOneShot(delivery.NotificationID) {
			continue
		}
		if _, isActive := activeNotificationIDs[delivery.NotificationID]; isActive {
			continue
		}
		if telegramDeliveryNeedsFlapSuppression(delivery) {
			nextDelivery, recovered, recoverErr := p.advanceTelegramFlapSuppressedRecoveredDelivery(delivery, now)
			if recoverErr != nil {
				p.logger("service.telegram", "notification_id", delivery.NotificationID).Warn("send telegram recovery notification failed", "error", recoverErr)
				continue
			}
			if nextDelivery.NotificationID != "" {
				deliveryByID[delivery.NotificationID] = nextDelivery
			}
			if recovered {
				recoveredCount++
			}
			continue
		}
		if !strings.EqualFold(delivery.Status, "sent") {
			continue
		}
		titleRaw, ok := p.telegramSentAlertCache.Load(delivery.NotificationID)
		title := "未知告警"
		if ok {
			title = titleRaw.(string)
		} else if delivery.Metadata != nil {
			// 如果内存缓存失效（如重启后），尝试从持久化的 Metadata 中恢复标题
			if persistentTitle, exists := delivery.Metadata["title"]; exists {
				title = fmt.Sprintf("%v", persistentTitle)
			}
		}

		recoveryMsg := fmt.Sprintf("✅ *[已恢复]* %s\n告警已自动解除。ID: %s", title, delivery.NotificationID)
		if err := p.sendTelegramMessage(recoveryMsg); err != nil {
			p.logger("service.telegram", "notification_id", delivery.NotificationID).Warn("send telegram recovery notification failed", "error", err)
			continue
		}

		// 标记为已恢复，防止重复发送
		if _, err := p.store.UpsertNotificationDelivery(delivery.NotificationID, "telegram", "recovered", "", delivery.Metadata); err != nil {
			p.logger("service.telegram", "notification_id", delivery.NotificationID).Warn("record telegram recovery delivery failed", "error", err)
		}
		p.telegramSentAlertCache.Delete(delivery.NotificationID)
		recoveredCount++
	}

	if sentCount > 0 || recoveredCount > 0 || tradeEventCount > 0 || positionReportCount > 0 {
		logger.Debug("telegram dispatch cycle completed", "sent", sentCount, "recovered", recoveredCount, "trade_events", tradeEventCount, "position_reports", positionReportCount, "active", len(notifications))
	}
	return firstErr
}

func telegramDeliveryIsOneShot(notificationID string) bool {
	return strings.HasPrefix(notificationID, "trade-event:") || strings.HasPrefix(notificationID, "position-report:")
}

func (p *Platform) DispatchTelegramTradeEvents(deliveryByID map[string]domain.NotificationDelivery) (int, error) {
	config := p.telegramConfig
	if !config.Enabled || !config.TradeEventsEnabled || strings.TrimSpace(config.BotToken) == "" || strings.TrimSpace(config.ChatID) == "" {
		return 0, nil
	}
	events, err := p.store.ListOrderExecutionEvents("")
	if err != nil {
		return 0, err
	}
	sent := 0
	var firstErr error
	for _, event := range events {
		if !strings.EqualFold(event.EventType, "filled") {
			continue
		}
		if event.Failed || strings.TrimSpace(event.Error) != "" {
			continue
		}
		notificationID := tradeEventNotificationID(event)
		if delivery, ok := deliveryByID[notificationID]; ok && strings.EqualFold(delivery.Status, "sent") {
			continue
		}
		message := formatTelegramTradeEvent(event)
		if err := p.sendTelegramMessage(message); err != nil {
			_, _ = p.store.UpsertNotificationDelivery(notificationID, "telegram", "failed", err.Error(), map[string]any{
				"kind":      "trade-event",
				"eventId":   event.ID,
				"orderId":   event.OrderID,
				"symbol":    event.Symbol,
				"eventTime": event.EventTime.Format(time.RFC3339),
			})
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		delivery, err := p.store.UpsertNotificationDelivery(notificationID, "telegram", "sent", "", map[string]any{
			"kind":      "trade-event",
			"eventId":   event.ID,
			"orderId":   event.OrderID,
			"symbol":    event.Symbol,
			"eventTime": event.EventTime.Format(time.RFC3339),
		})
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		deliveryByID[notificationID] = delivery
		sent++
	}
	return sent, firstErr
}

func tradeEventNotificationID(event domain.OrderExecutionEvent) string {
	anchor := firstNonEmpty(
		strings.TrimSpace(event.OrderID),
		strings.TrimSpace(event.ExchangeOrderID),
		strings.TrimSpace(event.ID),
	)
	eventTime := ""
	if !event.EventTime.IsZero() {
		eventTime = event.EventTime.UTC().Format(time.RFC3339Nano)
	}
	return strings.Join([]string{
		"trade-event",
		anchor,
		NormalizeSymbol(event.Symbol),
		strings.ToUpper(strings.TrimSpace(event.Side)),
		strings.ToLower(strings.TrimSpace(event.EventType)),
		strings.ToUpper(strings.TrimSpace(event.Status)),
		eventTime,
	}, ":")
}

func (p *Platform) DispatchTelegramPositionReport(deliveryByID map[string]domain.NotificationDelivery, now time.Time) (int, error) {
	config := p.telegramConfig
	if !config.Enabled || !config.PositionReportEnabled || strings.TrimSpace(config.BotToken) == "" || strings.TrimSpace(config.ChatID) == "" {
		return 0, nil
	}
	intervalMinutes := normalizeTelegramPositionReportInterval(config.PositionReportIntervalMinutes)
	interval := time.Duration(intervalMinutes) * time.Minute
	bucket := now.UTC().Truncate(interval)
	notificationID := fmt.Sprintf("position-report:%d:%d", intervalMinutes, bucket.Unix())
	if delivery, ok := deliveryByID[notificationID]; ok && strings.EqualFold(delivery.Status, "sent") {
		return 0, nil
	}

	// 仅在时间桶开始后的前 5 分钟内允许发起播报，避免“一开仓就播报”的突兀感。
	if now.UTC().Sub(bucket) > 5*time.Minute {
		p.logger("service.telegram").Debug("skipping position report because it is out of the 5-minute scheduling window",
			"now", now.Format(time.RFC3339),
			"bucket", bucket.Format(time.RFC3339),
			"diff", now.Sub(bucket).String())
		return 0, nil
	}

	accounts, err := p.store.ListAccounts()
	if err != nil {
		return 0, err
	}
	liveAccounts := make([]domain.Account, 0)
	for _, account := range accounts {
		if strings.EqualFold(account.Mode, "LIVE") {
			liveAccounts = append(liveAccounts, account)
		}
	}
	if len(liveAccounts) == 0 {
		return 0, nil
	}
	if !telegramAccountsHaveOpenPosition(liveAccounts) {
		return 0, nil
	}
	summaries, err := p.ListAccountSummaries()
	if err != nil {
		return 0, err
	}
	summaryByAccount := make(map[string]domain.AccountSummary, len(summaries))
	for _, summary := range summaries {
		summaryByAccount[summary.AccountID] = summary
	}
	message := formatTelegramPositionReport(liveAccounts, summaryByAccount, now, intervalMinutes)
	if err := p.sendTelegramMessage(message); err != nil {
		_, _ = p.store.UpsertNotificationDelivery(notificationID, "telegram", "failed", err.Error(), map[string]any{
			"kind":            "position-report",
			"bucket":          bucket.Format(time.RFC3339),
			"intervalMinutes": intervalMinutes,
		})
		return 0, err
	}
	delivery, err := p.store.UpsertNotificationDelivery(notificationID, "telegram", "sent", "", map[string]any{
		"kind":            "position-report",
		"bucket":          bucket.Format(time.RFC3339),
		"intervalMinutes": intervalMinutes,
	})
	if err != nil {
		return 0, err
	}
	deliveryByID[notificationID] = delivery
	return 1, nil
}

func telegramAccountsHaveOpenPosition(accounts []domain.Account) bool {
	for _, account := range accounts {
		for _, position := range metadataList(mapValue(account.Metadata["liveSyncSnapshot"])["positions"]) {
			qty := firstNonZeroFloat(parseFloatValue(position["quantity"]), parseFloatValue(position["positionAmt"]))
			if qty < 0 {
				qty = -qty
			}
			if qty > 0 {
				return true
			}
		}
	}
	return false
}

func formatTelegramTradeEvent(event domain.OrderExecutionEvent) string {
	action := "开仓"
	if telegramTradeEventIsClose(event) {
		action = "平仓"
	}
	price := firstPositiveTelegramFloat(event.Price, event.NormalizedPrice, event.ExpectedPrice, event.RawPriceReference)
	realizedPnL := firstNonZeroFloat(
		parseFloatValue(event.AdapterSync["totalRealizedPnl"]),
		parseFloatValue(event.AdapterSync["realizedPnl"]),
		parseFloatValue(event.DispatchSummary["realizedPnl"]),
	)
	fee := firstNonZeroFloat(parseFloatValue(event.AdapterSync["totalFee"]), parseFloatValue(event.DispatchSummary["fee"]))
	lines := []string{
		fmt.Sprintf("📌 *%s成交* %s %s", action, NormalizeSymbol(event.Symbol), strings.ToUpper(strings.TrimSpace(event.Side))),
		fmt.Sprintf("数量: %s", formatTelegramNumber(event.Quantity)),
		fmt.Sprintf("价格: %s", formatTelegramNumber(price)),
		fmt.Sprintf("订单: %s", event.OrderID),
	}
	if event.ExchangeOrderID != "" {
		lines = append(lines, fmt.Sprintf("交易所订单: %s", event.ExchangeOrderID))
	}
	if action == "平仓" {
		lines = append(lines, fmt.Sprintf("已实现盈亏: %s", formatTelegramSignedNumber(realizedPnL)))
	}
	if fee != 0 {
		lines = append(lines, fmt.Sprintf("手续费: %s", formatTelegramSignedNumber(-fee)))
	}
	if event.AccountID != "" {
		lines = append(lines, fmt.Sprintf("账户: %s", event.AccountID))
	}
	if event.LiveSessionID != "" {
		lines = append(lines, fmt.Sprintf("实盘会话: %s", event.LiveSessionID))
	}
	if !event.EventTime.IsZero() {
		lines = append(lines, fmt.Sprintf("北京时间: %s", formatTelegramBeijingTime(event.EventTime)))
	}
	return strings.Join(lines, "\n")
}

func telegramTradeEventIsClose(event domain.OrderExecutionEvent) bool {
	if event.ReduceOnly {
		return true
	}
	decision := strings.ToLower(strings.TrimSpace(event.ExecutionDecision))
	if strings.Contains(decision, "exit") || strings.Contains(decision, "close") {
		return true
	}
	role := strings.ToLower(firstNonEmpty(stringValue(event.DispatchSummary["role"]), stringValue(event.DispatchSummary["orderRole"]), stringValue(event.Metadata["role"])))
	return strings.Contains(role, "exit") || strings.Contains(role, "close")
}

func formatTelegramPositionReport(accounts []domain.Account, summaries map[string]domain.AccountSummary, bucket time.Time, intervalMinutes int) string {
	lines := []string{
		fmt.Sprintf("📊 *持仓定时播报* %d分钟", intervalMinutes),
		fmt.Sprintf("北京时间: %s", formatTelegramBeijingTime(bucket)),
	}
	for _, account := range accounts {
		summary := summaries[account.ID]
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("账户: %s", firstNonEmpty(account.Name, account.ID)))
		lines = append(lines, fmt.Sprintf("净值: %s 可用: %s 未实现盈亏: %s", formatTelegramNumber(summary.NetEquity), formatTelegramNumber(summary.AvailableBalance), formatTelegramSignedNumber(summary.UnrealizedPnL)))
		snapshot := mapValue(account.Metadata["liveSyncSnapshot"])
		syncStatus := firstNonEmpty(stringValue(snapshot["syncStatus"]), account.Status)
		if syncedAt := firstNonEmpty(stringValue(snapshot["syncedAt"]), stringValue(account.Metadata["lastLiveSyncAt"])); syncedAt != "" {
			displayTime := syncedAt
			if t, err := time.Parse(time.RFC3339, syncedAt); err == nil {
				displayTime = formatTelegramBeijingTime(t)
			}
			lines = append(lines, fmt.Sprintf("同步: %s %s", syncStatus, displayTime))
		} else {
			lines = append(lines, fmt.Sprintf("同步: %s", syncStatus))
		}
		positions := metadataList(snapshot["positions"])
		if len(positions) == 0 {
			lines = append(lines, "持仓: 无")
			continue
		}
		for _, position := range positions {
			symbol := NormalizeSymbol(stringValue(position["symbol"]))
			qty := firstNonZeroFloat(parseFloatValue(position["quantity"]), parseFloatValue(position["positionAmt"]))
			side := firstNonEmpty(stringValue(position["side"]), stringValue(position["positionSide"]))
			if side == "" {
				if qty < 0 {
					side = "SHORT"
				} else {
					side = "LONG"
				}
			}
			if qty < 0 {
				qty = -qty
			}
			entryPrice := parseFloatValue(position["entryPrice"])
			markPrice := parseFloatValue(position["markPrice"])
			unrealized := parseFloatValue(position["unrealizedProfit"])
			if unrealized == 0 && entryPrice > 0 && markPrice > 0 && qty > 0 {
				if strings.EqualFold(side, "SHORT") {
					unrealized = (entryPrice - markPrice) * qty
				} else {
					unrealized = (markPrice - entryPrice) * qty
				}
			}
			lines = append(lines, fmt.Sprintf("%s %s 数量:%s 入场:%s 标记:%s 浮盈亏:%s",
				symbol,
				strings.ToUpper(side),
				formatTelegramNumber(qty),
				formatTelegramNumber(entryPrice),
				formatTelegramNumber(markPrice),
				formatTelegramSignedNumber(unrealized),
			))
		}
	}
	return strings.Join(lines, "\n")
}

func firstNonZeroFloat(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func firstPositiveTelegramFloat(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func formatTelegramNumber(value float64) string {
	text := fmt.Sprintf("%.8f", value)
	text = strings.TrimRight(text, "0")
	text = strings.TrimRight(text, ".")
	if text == "" || text == "-0" {
		return "0"
	}
	return text
}

func formatTelegramSignedNumber(value float64) string {
	if value > 0 {
		return "+" + formatTelegramNumber(value)
	}
	return formatTelegramNumber(value)
}

func formatTelegramBeijingTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.In(telegramBeijingLocation).Format("2006-01-02 15:04:05")
}

func telegramAlertNeedsFlapSuppression(alert domain.PlatformAlert) bool {
	return telegramFlapSuppressionKeyForAlert(alert) != ""
}

func telegramDeliveryNeedsFlapSuppression(delivery domain.NotificationDelivery) bool {
	if delivery.NotificationID == "" {
		return false
	}
	if strings.HasPrefix(delivery.NotificationID, "runtime-stale-") {
		return true
	}
	if strings.HasPrefix(delivery.NotificationID, "runtime-recovering-") {
		return true
	}
	if strings.HasPrefix(delivery.NotificationID, "live-preflight-runtime-error-") {
		return true
	}
	if delivery.Metadata == nil {
		return false
	}
	return telegramFlapSuppressionKeyIsKnown(stringValue(delivery.Metadata["flapSuppressionKey"]))
}

func telegramFlapSuppressionKeyForAlert(alert domain.PlatformAlert) string {
	if alert.ID == "" {
		return ""
	}
	if alert.Scope == "runtime" && strings.HasPrefix(alert.ID, "runtime-stale-") {
		return "runtime-stale"
	}
	if alert.Scope == "runtime" && strings.HasPrefix(alert.ID, "runtime-recovering-") {
		return "runtime-recovering"
	}
	if alert.Scope == "live" && strings.HasPrefix(alert.ID, "live-warning-stale-source-states-") {
		return "live-warning-stale-source-states"
	}
	if alert.Scope == "live" && strings.HasPrefix(alert.ID, "live-preflight-runtime-error-") {
		return "live-preflight-runtime-error"
	}
	return ""
}

func telegramFlapSuppressionKeyIsKnown(key string) bool {
	switch key {
	case "runtime-stale", "runtime-recovering", "live-warning-stale-source-states", "live-preflight-runtime-error":
		return true
	default:
		return false
	}
}

func (p *Platform) advanceTelegramFlapSuppressedActiveDelivery(
	item domain.PlatformNotification,
	delivery domain.NotificationDelivery,
	hasDelivery bool,
	now time.Time,
) (domain.NotificationDelivery, bool, error) {
	metadata := cloneMetadata(delivery.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	if !hasDelivery || (!strings.EqualFold(delivery.Status, "pending") &&
		!strings.EqualFold(delivery.Status, "sent") &&
		!strings.EqualFold(delivery.Status, "resolve_pending")) {
		delete(metadata, "firstActiveAt")
	}
	metadata["title"] = item.Alert.Title
	metadata["scope"] = item.Alert.Scope
	metadata["detail"] = item.Alert.Detail
	metadata["firstActiveAt"] = firstNonEmpty(stringValue(metadata["firstActiveAt"]), now.Format(time.RFC3339))
	if key := telegramFlapSuppressionKeyForAlert(item.Alert); key != "" {
		metadata["flapSuppressionKey"] = key
	}
	delete(metadata, "resolveObservedAt")

	if hasDelivery {
		switch {
		case strings.EqualFold(delivery.Status, "sent"):
			return delivery, false, nil
		case strings.EqualFold(delivery.Status, "resolve_pending"):
			nextDelivery, err := p.store.UpsertNotificationDelivery(item.ID, "telegram", "sent", "", metadata)
			return nextDelivery, false, err
		case strings.EqualFold(delivery.Status, "pending"):
			firstActiveAt := parseOptionalRFC3339(stringValue(metadata["firstActiveAt"]))
			if firstActiveAt.IsZero() {
				firstActiveAt = now
				metadata["firstActiveAt"] = now.Format(time.RFC3339)
			}
			if now.Sub(firstActiveAt) < telegramFlapSendGrace {
				nextDelivery, err := p.store.UpsertNotificationDelivery(item.ID, "telegram", "pending", "", metadata)
				return nextDelivery, false, err
			}
		}
	}

	if !hasDelivery || !strings.EqualFold(delivery.Status, "pending") {
		nextDelivery, err := p.store.UpsertNotificationDelivery(item.ID, "telegram", "pending", "", metadata)
		return nextDelivery, false, err
	}
	if err := p.sendTelegramMessage(formatTelegramNotification(item)); err != nil {
		return domain.NotificationDelivery{}, false, err
	}
	nextDelivery, err := p.store.UpsertNotificationDelivery(item.ID, "telegram", "sent", "", metadata)
	return nextDelivery, true, err
}

func (p *Platform) advanceTelegramFlapSuppressedRecoveredDelivery(delivery domain.NotificationDelivery, now time.Time) (domain.NotificationDelivery, bool, error) {
	metadata := cloneMetadata(delivery.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	if strings.EqualFold(delivery.Status, "pending") {
		nextDelivery, err := p.store.UpsertNotificationDelivery(delivery.NotificationID, "telegram", "recovered", "", metadata)
		return nextDelivery, false, err
	}
	if !strings.EqualFold(delivery.Status, "sent") && !strings.EqualFold(delivery.Status, "resolve_pending") {
		return delivery, false, nil
	}
	resolveObservedAt := parseOptionalRFC3339(stringValue(metadata["resolveObservedAt"]))
	if strings.EqualFold(delivery.Status, "sent") || resolveObservedAt.IsZero() {
		metadata["resolveObservedAt"] = now.Format(time.RFC3339)
		nextDelivery, err := p.store.UpsertNotificationDelivery(delivery.NotificationID, "telegram", "resolve_pending", "", metadata)
		return nextDelivery, false, err
	}
	if now.Sub(resolveObservedAt) < telegramFlapRecoverGrace {
		nextDelivery, err := p.store.UpsertNotificationDelivery(delivery.NotificationID, "telegram", "resolve_pending", "", metadata)
		return nextDelivery, false, err
	}

	titleRaw, ok := p.telegramSentAlertCache.Load(delivery.NotificationID)
	title := "未知告警"
	if ok {
		title = titleRaw.(string)
	} else if persistentTitle, exists := metadata["title"]; exists {
		title = fmt.Sprintf("%v", persistentTitle)
	}
	recoveryMsg := fmt.Sprintf("✅ *[已恢复]* %s\n告警已自动解除。ID: %s", title, delivery.NotificationID)
	if err := p.sendTelegramMessage(recoveryMsg); err != nil {
		return domain.NotificationDelivery{}, false, err
	}
	nextDelivery, err := p.store.UpsertNotificationDelivery(delivery.NotificationID, "telegram", "recovered", "", metadata)
	if err != nil {
		return domain.NotificationDelivery{}, false, err
	}
	p.telegramSentAlertCache.Delete(delivery.NotificationID)
	return nextDelivery, true, nil
}
