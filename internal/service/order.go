package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

// --- 订单管理服务方法 ---

// ListOrders 获取所有订单列表。
func (p *Platform) ListOrders() ([]domain.Order, error) {
	return p.store.ListOrders()
}

func (p *Platform) GetOrder(orderID string) (domain.Order, error) {
	items, err := p.store.ListOrders()
	if err != nil {
		return domain.Order{}, err
	}
	for _, item := range items {
		if item.ID == orderID {
			return item, nil
		}
	}
	return domain.Order{}, fmt.Errorf("order not found: %s", orderID)
}

// CreateOrder 创建订单。对于 PAPER 模式账户，订单会被立即执行（模拟成交），
// 生成 fill 记录、更新持仓、捕获净值快照。
func (p *Platform) CreateOrder(order domain.Order) (domain.Order, error) {
	account, err := p.prepareOrderAccount(order)
	if err != nil {
		return domain.Order{}, err
	}
	if err := p.preflightOrderExecution(account, order); err != nil {
		return domain.Order{}, err
	}
	createdOrder, err := p.store.CreateOrder(order)
	if err != nil {
		return domain.Order{}, err
	}
	return p.executeCreatedOrder(account, createdOrder)
}

func (p *Platform) prepareOrderAccount(order domain.Order) (domain.Account, error) {
	return p.store.GetAccount(order.AccountID)
}

func (p *Platform) preflightOrderExecution(account domain.Account, order domain.Order) error {
	if account.Mode != "LIVE" {
		return nil
	}
	if account.Status != "CONFIGURED" && account.Status != "READY" {
		return fmt.Errorf("live account %s is not configured", account.ID)
	}
	if _, _, err := p.resolveLiveAdapterForAccount(account); err != nil {
		return err
	}
	if _, _, err := p.ensureLiveRuntimeReady(account, order); err != nil {
		return err
	}
	return nil
}

func (p *Platform) executeCreatedOrder(account domain.Account, order domain.Order) (domain.Order, error) {
	switch strings.ToUpper(strings.TrimSpace(account.Mode)) {
	case "LIVE":
		return p.executeLiveOrder(account, order)
	case "PAPER":
		return p.executePaperOrder(account, order)
	default:
		return order, nil
	}
}

func (p *Platform) executePaperOrder(account domain.Account, order domain.Order) (domain.Order, error) {
	executionPrice := resolveExecutionPrice(order)
	fillFee := resolvePaperFillFee(order, executionPrice)
	order.Metadata = cloneMetadata(order.Metadata)
	applyExecutionMetadata(order.Metadata, map[string]any{
		"executionMode": "paper",
		"fillPolicy":    "immediate",
		"feeSource":     "configured-paper-rate",
		"fundingSource": "paper-simulated",
		"orderLifecycle": map[string]any{
			"submitted": false,
			"accepted":  true,
			"synced":    false,
			"filled":    true,
		},
	})
	return p.finalizeExecutedOrder(account, order, []domain.Fill{{
		OrderID:  order.ID,
		Price:    executionPrice,
		Quantity: order.Quantity,
		Fee:      fillFee,
	}})
}

func (p *Platform) executeLiveOrder(account domain.Account, order domain.Order) (domain.Order, error) {
	return p.submitLiveOrder(account, order)
}

func (p *Platform) submitLiveOrder(account domain.Account, order domain.Order) (domain.Order, error) {
	adapter, binding, err := p.resolveLiveAdapterForAccount(account)
	if err != nil {
		return domain.Order{}, err
	}
	runtimeSession, sourceGate, err := p.ensureLiveRuntimeReady(account, order)
	if err != nil {
		return domain.Order{}, err
	}
	submission, submitErr := adapter.SubmitOrder(account, order, binding)
	return p.applyLiveSubmissionResult(order, binding, runtimeSession, sourceGate, submission, submitErr)
}

func (p *Platform) applyLiveSubmissionResult(
	order domain.Order,
	binding map[string]any,
	runtimeSession domain.SignalRuntimeSession,
	sourceGate map[string]any,
	submission LiveOrderSubmission,
	submitErr error,
) (domain.Order, error) {
	order.Metadata = cloneMetadata(order.Metadata)
	applyExecutionMetadata(order.Metadata, map[string]any{
		"executionMode":    "live",
		"adapterKey":       normalizeLiveAdapterKey(stringValue(binding["adapterKey"])),
		"feeSource":        "exchange",
		"fundingSource":    "exchange",
		"submitMode":       "adapter",
		"runtimeSessionId": runtimeSession.ID,
		"runtimePreflight": sourceGate,
		"orderLifecycle": map[string]any{
			"submitted": true,
			"accepted":  false,
			"synced":    false,
			"filled":    false,
		},
	})
	if submitErr != nil {
		order.Status = "REJECTED"
		order.Metadata["liveSubmitError"] = submitErr.Error()
		markOrderLifecycle(order.Metadata, "accepted", false)
		updatedOrder, updateErr := p.store.UpdateOrder(order)
		if updateErr != nil {
			return domain.Order{}, updateErr
		}
		return updatedOrder, submitErr
	}
	delete(order.Metadata, "liveSubmitError")
	order.Status = firstNonEmpty(submission.Status, "ACCEPTED")
	order.Metadata["exchangeOrderId"] = submission.ExchangeOrderID
	order.Metadata["acceptedAt"] = submission.AcceptedAt
	order.Metadata["adapterSubmission"] = submission.Metadata
	markOrderLifecycle(order.Metadata, "accepted", true)
	return p.store.UpdateOrder(order)
}

func (p *Platform) ensureLiveRuntimeReady(account domain.Account, order domain.Order) (domain.SignalRuntimeSession, map[string]any, error) {
	strategyID, err := p.resolveLiveStrategyIDForOrder(account.ID, order)
	if err != nil {
		return domain.SignalRuntimeSession{}, nil, err
	}
	plan, err := p.BuildSignalRuntimePlan(account.ID, strategyID)
	if err != nil {
		return domain.SignalRuntimeSession{}, nil, err
	}
	if !boolValue(plan["ready"]) {
		return domain.SignalRuntimeSession{}, nil, fmt.Errorf(
			"live runtime plan is not ready for account %s strategy %s: missing=%d",
			account.ID,
			strategyID,
			len(metadataList(plan["missingBindings"])),
		)
	}

	runtimeSession, err := p.resolveLiveRuntimeSession(account.ID, strategyID)
	if err != nil {
		return domain.SignalRuntimeSession{}, nil, err
	}
	if !strings.EqualFold(runtimeSession.Status, "RUNNING") {
		return domain.SignalRuntimeSession{}, nil, fmt.Errorf(
			"live runtime session %s is not running for account %s strategy %s",
			runtimeSession.ID,
			account.ID,
			strategyID,
		)
	}
	if !strings.EqualFold(stringValue(runtimeSession.State["health"]), "healthy") {
		return domain.SignalRuntimeSession{}, nil, fmt.Errorf(
			"live runtime session %s is not healthy for account %s strategy %s",
			runtimeSession.ID,
			account.ID,
			strategyID,
		)
	}

	sourceGate := p.evaluateRuntimeSignalSourceReadiness(strategyID, runtimeSession, time.Now().UTC())
	if !boolValue(sourceGate["ready"]) {
		return domain.SignalRuntimeSession{}, sourceGate, fmt.Errorf(
			"live runtime session %s not ready: missing=%d stale=%d",
			runtimeSession.ID,
			len(metadataList(sourceGate["missing"])),
			len(metadataList(sourceGate["stale"])),
		)
	}
	return runtimeSession, sourceGate, nil
}

func (p *Platform) resolveLiveStrategyIDForOrder(accountID string, order domain.Order) (string, error) {
	if strings.TrimSpace(order.StrategyVersionID) != "" {
		return p.resolveStrategyIDFromVersionID(order.StrategyVersionID)
	}
	sessions := p.ListSignalRuntimeSessions()
	matches := make([]domain.SignalRuntimeSession, 0)
	for _, session := range sessions {
		if session.AccountID == accountID {
			matches = append(matches, session)
		}
	}
	if len(matches) == 1 {
		return matches[0].StrategyID, nil
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("live order requires strategyVersionId or a linked runtime session")
	}
	return "", fmt.Errorf("live order requires strategyVersionId when multiple runtime sessions are linked to account %s", accountID)
}

func (p *Platform) resolveLiveRuntimeSession(accountID, strategyID string) (domain.SignalRuntimeSession, error) {
	sessions := p.ListSignalRuntimeSessions()
	var fallback *domain.SignalRuntimeSession
	for _, session := range sessions {
		if session.AccountID != accountID || session.StrategyID != strategyID {
			continue
		}
		if strings.EqualFold(session.Status, "RUNNING") {
			return session, nil
		}
		if fallback == nil {
			sessionCopy := session
			fallback = &sessionCopy
		}
	}
	if fallback != nil {
		return *fallback, nil
	}
	return domain.SignalRuntimeSession{}, fmt.Errorf("no runtime session found for account %s strategy %s", accountID, strategyID)
}

func (p *Platform) resolveStrategyIDFromVersionID(strategyVersionID string) (string, error) {
	items, err := p.ListStrategies()
	if err != nil {
		return "", err
	}
	for _, item := range items {
		switch currentVersion := item["currentVersion"].(type) {
		case domain.StrategyVersion:
			if currentVersion.ID == strategyVersionID {
				return stringValue(item["id"]), nil
			}
		case map[string]any:
			if stringValue(currentVersion["id"]) == strategyVersionID {
				return stringValue(item["id"]), nil
			}
		}
	}
	return "", fmt.Errorf("strategy not found for strategy version %s", strategyVersionID)
}

func (p *Platform) SyncLiveOrder(orderID string) (domain.Order, error) {
	order, err := p.GetOrder(orderID)
	if err != nil {
		return domain.Order{}, err
	}
	account, err := p.store.GetAccount(order.AccountID)
	if err != nil {
		return domain.Order{}, err
	}
	if account.Mode != "LIVE" {
		return domain.Order{}, fmt.Errorf("order %s is not a live order", orderID)
	}
	adapter, binding, err := p.resolveLiveAdapterForAccount(account)
	if err != nil {
		return domain.Order{}, err
	}
	syncResult, err := adapter.SyncOrder(account, order, binding)
	if err != nil {
		return domain.Order{}, err
	}
	return p.applyLiveSyncResult(account, order, syncResult)
}

func (p *Platform) applyLiveSyncResult(account domain.Account, order domain.Order, syncResult LiveOrderSync) (domain.Order, error) {
	order.Metadata = cloneMetadata(order.Metadata)
	applyExecutionMetadata(order.Metadata, map[string]any{
		"lastSyncAt":    syncResult.SyncedAt,
		"syncMode":      "adapter",
		"feeSource":     firstNonEmpty(syncResult.FeeSource, "exchange"),
		"fundingSource": firstNonEmpty(syncResult.FundingSrc, "exchange"),
		"adapterSync":   syncResult.Metadata,
	})
	order.Status = firstNonEmpty(syncResult.Status, order.Status)
	markOrderLifecycle(order.Metadata, "synced", true)
	if len(syncResult.Fills) > 0 {
		fills, lastFundingPnL, lastPrice := buildLiveSyncSettlement(order, syncResult)
		order.Price = lastPrice
		order.Metadata["fundingPnL"] = lastFundingPnL
		markOrderLifecycle(order.Metadata, "filled", true)
		return p.finalizeExecutedOrder(account, order, fills)
	}
	return p.store.UpdateOrder(order)
}

func buildLiveSyncSettlement(order domain.Order, syncResult LiveOrderSync) ([]domain.Fill, float64, float64) {
	fills := make([]domain.Fill, 0, len(syncResult.Fills))
	lastFundingPnL := 0.0
	lastPrice := order.Price
	for _, report := range syncResult.Fills {
		price := report.Price
		if price <= 0 {
			price = resolveExecutionPrice(order)
		}
		lastPrice = price
		lastFundingPnL = report.FundingPnL
		fills = append(fills, domain.Fill{
			OrderID:  order.ID,
			Price:    price,
			Quantity: report.Quantity,
			Fee:      report.Fee - report.FundingPnL,
		})
	}
	return fills, lastFundingPnL, lastPrice
}

func (p *Platform) finalizeExecutedOrder(account domain.Account, order domain.Order, fills []domain.Fill) (domain.Order, error) {
	if len(fills) == 0 {
		return p.store.UpdateOrder(order)
	}
	lastPrice := order.Price
	for _, fill := range fills {
		createdFill, err := p.store.CreateFill(fill)
		if err != nil {
			return domain.Order{}, err
		}
		executionPrice := createdFill.Price
		if executionPrice <= 0 {
			executionPrice = resolveExecutionPrice(order)
		}
		lastPrice = executionPrice
		execOrder := order
		execOrder.Price = executionPrice
		if err := p.applyExecutionFill(account, execOrder, executionPrice); err != nil {
			return domain.Order{}, err
		}
	}
	order.Status = "FILLED"
	order.Price = lastPrice
	markOrderLifecycle(order.Metadata, "filled", true)
	if order.Metadata["acceptedAt"] == nil {
		order.Metadata["acceptedAt"] = time.Now().UTC().Format(time.RFC3339)
	}
	order.Metadata["lastFilledAt"] = time.Now().UTC().Format(time.RFC3339)
	updatedOrder, err := p.store.UpdateOrder(order)
	if err != nil {
		return domain.Order{}, err
	}
	if err := p.captureAccountSnapshot(account.ID); err != nil {
		return domain.Order{}, err
	}
	return updatedOrder, nil
}

func (p *Platform) resolveLiveAdapterForAccount(account domain.Account) (LiveExecutionAdapter, map[string]any, error) {
	binding := resolveLiveBinding(account)
	if len(binding) == 0 {
		return nil, nil, fmt.Errorf("live account %s has no adapter binding", account.ID)
	}
	adapterKey := normalizeLiveAdapterKey(stringValue(binding["adapterKey"]))
	adapter, ok := p.liveAdapters[adapterKey]
	if !ok {
		return nil, nil, fmt.Errorf("live adapter not registered: %s", adapterKey)
	}
	if err := adapter.ValidateAccountConfig(binding); err != nil {
		return nil, nil, err
	}
	return adapter, binding, nil
}

func resolvePaperTradingFeeRate(order domain.Order) float64 {
	if order.Metadata != nil {
		for _, key := range []string{"tradingFeeBps", "feeRateBps", "takerFeeBps"} {
			if value, ok := order.Metadata[key]; ok {
				if bps, ok := toFloat64(value); ok && bps >= 0 {
					return bps / 10000.0
				}
			}
		}
	}
	return 0.001
}

func resolvePaperFillFee(order domain.Order, executionPrice float64) float64 {
	if order.Metadata != nil {
		for _, key := range []string{"paperFeeAmount", "fillFeeAmount"} {
			if value, ok := order.Metadata[key]; ok {
				if fee, ok := toFloat64(value); ok {
					return fee
				}
			}
		}
	}
	fee := executionPrice * order.Quantity * resolvePaperTradingFeeRate(order)
	if order.Metadata != nil {
		if fundingPnL, ok := toFloat64(order.Metadata["fundingPnL"]); ok {
			fee -= fundingPnL
		}
	}
	return fee
}

// ListPositions 获取所有持仓列表。
func (p *Platform) ListPositions() ([]domain.Position, error) {
	return p.store.ListPositions()
}

// ListFills 获取所有成交记录列表。
func (p *Platform) ListFills() ([]domain.Fill, error) {
	return p.store.ListFills()
}

// applyExecutionFill 根据已确认成交更新仓位。
// 它是 paper/live 共用的持仓落账逻辑，只处理 canonical fill 之后的仓位变更。
func (p *Platform) applyExecutionFill(account domain.Account, order domain.Order, executionPrice float64) error {
	position, exists, err := p.store.FindPosition(account.ID, order.Symbol)
	if err != nil {
		return err
	}

	orderSide := strings.ToUpper(strings.TrimSpace(order.Side))
	targetSide := "LONG"
	if orderSide == "SELL" {
		targetSide = "SHORT"
	}

	// 无现有持仓 → 新开仓
	if !exists {
		_, err := p.store.SavePosition(domain.Position{
			AccountID:         account.ID,
			StrategyVersionID: order.StrategyVersionID,
			Symbol:            order.Symbol,
			Side:              targetSide,
			Quantity:          order.Quantity,
			EntryPrice:        executionPrice,
			MarkPrice:         executionPrice,
		})
		return err
	}

	// 同方向 → 增仓（加权平均入场价）
	if position.Side == targetSide {
		totalQty := position.Quantity + order.Quantity
		position.EntryPrice = ((position.EntryPrice * position.Quantity) + (executionPrice * order.Quantity)) / totalQty
		position.Quantity = totalQty
		position.MarkPrice = executionPrice
		position.StrategyVersionID = firstNonEmpty(order.StrategyVersionID, position.StrategyVersionID)
		_, err := p.store.SavePosition(position)
		return err
	}

	// 反方向 → 部分平仓
	if order.Quantity < position.Quantity {
		position.Quantity = position.Quantity - order.Quantity
		position.MarkPrice = executionPrice
		_, err := p.store.SavePosition(position)
		return err
	}

	// 反方向 → 全部平仓
	if order.Quantity == position.Quantity {
		return p.store.DeletePosition(position.ID)
	}

	// 反方向 → 平仓后反向开仓
	remaining := order.Quantity - position.Quantity
	position.Side = targetSide
	position.Quantity = remaining
	position.EntryPrice = executionPrice
	position.MarkPrice = executionPrice
	position.StrategyVersionID = firstNonEmpty(order.StrategyVersionID, position.StrategyVersionID)
	_, err = p.store.SavePosition(position)
	return err
}

func applyExecutionMetadata(target map[string]any, updates map[string]any) {
	if target == nil || len(updates) == 0 {
		return
	}
	for key, value := range updates {
		target[key] = value
	}
}

func markOrderLifecycle(metadata map[string]any, key string, value bool) {
	if metadata == nil {
		return
	}
	lifecycle := cloneMetadata(mapValue(metadata["orderLifecycle"]))
	if lifecycle == nil {
		lifecycle = map[string]any{}
	}
	lifecycle[key] = value
	metadata["orderLifecycle"] = lifecycle
}

// resolveExecutionPrice 确定订单的执行价格。
// 优先级：订单指定价格 > metadata 中的标记价格 > 默认硬编码价格。
func resolveExecutionPrice(order domain.Order) float64 {
	if order.Price > 0 {
		return order.Price
	}
	if order.Metadata != nil {
		for _, key := range []string{"markPrice", "lastPrice", "closePrice"} {
			if value, ok := order.Metadata[key]; ok {
				if price, ok := toFloat64(value); ok && price > 0 {
					return price
				}
			}
		}
	}
	// 默认价格（临时方案，后续对接实时行情）
	switch order.Symbol {
	case "ETHUSDT":
		return 3450
	default:
		return 68000
	}
}
