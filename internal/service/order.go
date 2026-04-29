package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	storepkg "github.com/wuyaocheng/bktrader/internal/store"
)

const (
	liveSettlementSyncErrorKey    = "immediateFillSyncError"
	liveSettlementSyncRequiredKey = "immediateFillSyncRequired"
)

// --- 订单管理服务方法 ---

// ListOrders 获取所有订单列表。
func (p *Platform) ListOrders() ([]domain.Order, error) {
	return p.store.ListOrders()
}

// ListOrdersWithLimit 获取限制数量的订单，按时间降序（最新优先）。
func (p *Platform) ListOrdersWithLimit(limit, offset int) ([]domain.Order, error) {
	return p.store.ListOrdersWithLimit(limit, offset)
}

// CountOrders 获取订单总数。
func (p *Platform) CountOrders() (int, error) {
	return p.store.CountOrders()
}

func (p *Platform) GetOrder(orderID string) (domain.Order, error) {
	return p.store.GetOrderByID(orderID)
}

func (p *Platform) ClosePosition(positionID string) (domain.Order, error) {
	position, _, err := p.resolveClosePositionTarget(positionID)
	if err != nil {
		return domain.Order{}, err
	}
	// Manual close stays fail-closed for unresolved reconcile conflicts, but a
	// stale local-only position is allowed one authoritative reconcile self-heal
	// attempt before we block the close path.
	if err := p.ensureLivePositionReconcileGateAllowsExecution(position.AccountID, position.Symbol, position.Quantity > 0); err != nil {
		return domain.Order{}, err
	}
	position, _, err = p.resolveClosePositionTarget(positionID)
	if err != nil {
		return domain.Order{}, err
	}
	order := buildClosePositionOrder(position)
	if session, ok := p.findLiveRecoveryCloseOnlySession(position.AccountID, position.Symbol); ok {
		order.Metadata = cloneMetadata(order.Metadata)
		order.Metadata["skipRuntimeCheck"] = true
		order.Metadata["recoveryMode"] = liveRecoveryModeCloseOnlyTakeover
		order.Metadata["recoveryCloseOnlyTakeover"] = true
		order.Metadata["recoveryTakeoverSessionId"] = session.ID
		order.StrategyVersionID = firstNonEmpty(order.StrategyVersionID, stringValue(session.State["strategyVersionId"]))
	}
	return p.CreateOrder(order)
}

func (p *Platform) ensureLivePositionReconcileGateAllowsExecution(accountID, symbol string, requiresVerification bool) error {
	account, err := p.store.GetAccount(accountID)
	if err != nil {
		return err
	}
	gate := resolveLivePositionReconcileGate(account, symbol, requiresVerification)
	if boolValue(gate["blocking"]) {
		if healedAccount, attempted, healErr := p.attemptLiveAccountReconcileSelfHeal(account, symbol); attempted {
			if healErr != nil {
				return healErr
			}
			account = healedAccount
			gate = resolveLivePositionReconcileGate(account, symbol, requiresVerification)
			if boolValue(gate["blocking"]) &&
				strings.EqualFold(strings.TrimSpace(stringValue(gate["scenario"])), "missing-reconcile-verdict") {
				position, found, findErr := p.store.FindPosition(accountID, symbol)
				if findErr != nil {
					return findErr
				}
				if !found || position.Quantity <= 0 {
					return nil
				}
			}
		}
	}
	if !boolValue(gate["blocking"]) {
		return nil
	}
	return fmt.Errorf(
		"live position %s execution blocked by reconcile gate: %s (%s)",
		NormalizeSymbol(symbol),
		firstNonEmpty(stringValue(gate["status"]), livePositionReconcileGateStatusError),
		firstNonEmpty(stringValue(gate["scenario"]), "unknown"),
	)
}

func buildClosePositionOrder(position domain.Position) domain.Order {
	closeSide := "SELL"
	if strings.EqualFold(strings.TrimSpace(position.Side), "SHORT") {
		closeSide = "BUY"
	}
	return domain.Order{
		AccountID:         position.AccountID,
		StrategyVersionID: position.StrategyVersionID,
		Symbol:            NormalizeSymbol(position.Symbol),
		Side:              closeSide,
		Type:              "MARKET",
		Quantity:          position.Quantity,
		ReduceOnly:        true,
		Metadata: map[string]any{
			"source":           "manual-position-close",
			"positionId":       position.ID,
			"markPrice":        position.MarkPrice,
			"priceHint":        position.MarkPrice,
			"manualAction":     "close-position",
			"skipRuntimeCheck": true,
		},
	}
}

func (p *Platform) findLiveRecoveryCloseOnlySession(accountID, symbol string) (domain.LiveSession, bool) {
	sessions, err := p.ListLiveSessions()
	if err != nil {
		return domain.LiveSession{}, false
	}
	normalizedSymbol := NormalizeSymbol(symbol)
	for _, session := range sessions {
		if session.AccountID != accountID {
			continue
		}
		if !isLiveSessionRecoveryCloseOnlyMode(session.State) {
			continue
		}
		sessionSymbol := NormalizeSymbol(firstNonEmpty(stringValue(session.State["symbol"]), stringValue(session.State["lastSymbol"])))
		if normalizedSymbol != "" && sessionSymbol != "" && sessionSymbol != normalizedSymbol {
			continue
		}
		return session, true
	}
	return domain.LiveSession{}, false
}

// CreateOrder 创建订单。对于 PAPER 模式账户，订单会被立即执行（模拟成交），
// 生成 fill 记录、更新持仓、捕获净值快照。
func (p *Platform) CreateOrder(order domain.Order) (domain.Order, error) {
	order.NormalizeExecutionFlags()
	logger := p.logger("service.order",
		"account_id", order.AccountID,
		"strategy_version_id", order.StrategyVersionID,
		"symbol", NormalizeSymbol(order.Symbol),
		"side", order.Side,
		"type", order.Type,
	)
	logger.Debug("creating order", "quantity", order.Quantity, "price", order.Price)
	account, err := p.prepareOrderAccount(order)
	if err != nil {
		logger.Warn("prepare order account failed", "error", err)
		return domain.Order{}, err
	}
	if err := p.preflightOrderExecution(account, order); err != nil {
		logger.Warn("order preflight failed", "error", err, "account_mode", account.Mode)
		return domain.Order{}, err
	}
	createdOrder, err := p.store.CreateOrder(order)
	if err != nil {
		logger.Error("persist order failed", "error", err)
		return domain.Order{}, err
	}
	logger.Debug("order persisted", "order_id", createdOrder.ID, "account_mode", account.Mode)
	return p.executeCreatedOrder(account, createdOrder)
}

func (p *Platform) prepareOrderAccount(order domain.Order) (domain.Account, error) {
	return p.store.GetAccount(order.AccountID)
}

func (p *Platform) preflightOrderExecution(account domain.Account, order domain.Order) error {
	if err := p.validateReduceOnlyOrder(account, order); err != nil {
		return err
	}
	if account.Mode != "LIVE" {
		return nil
	}
	if account.Status != "CONFIGURED" && account.Status != "READY" {
		return fmt.Errorf("live account %s is not configured", account.ID)
	}
	if _, _, err := p.resolveLiveAdapterForAccount(account); err != nil {
		return err
	}
	if shouldSkipLiveRuntimeCheck(order) {
		return nil
	}
	if _, _, err := p.ensureLiveRuntimeReady(account, order); err != nil {
		return err
	}
	return nil
}

func (p *Platform) resolveClosePositionTarget(positionID string) (domain.Position, domain.Account, error) {
	position, found, err := p.findPositionByID(positionID)
	if err != nil {
		return domain.Position{}, domain.Account{}, err
	}
	if !found {
		return domain.Position{}, domain.Account{}, fmt.Errorf("position not found: %s", positionID)
	}
	account, err := p.store.GetAccount(position.AccountID)
	if err != nil {
		return domain.Position{}, domain.Account{}, err
	}
	if strings.EqualFold(strings.TrimSpace(account.Mode), "LIVE") {
		syncedAccount, err := p.requestLiveAccountSync(account.ID, "resolve-close-position-target")
		if err != nil {
			return domain.Position{}, domain.Account{}, err
		}
		account = syncedAccount
		if healedAccount, attempted, healErr := p.attemptLiveAccountReconcileSelfHeal(account, position.Symbol); attempted {
			if healErr != nil {
				return domain.Position{}, domain.Account{}, healErr
			}
			account = healedAccount
		}
		position, found, err = p.findPositionByID(positionID)
		if err != nil {
			return domain.Position{}, domain.Account{}, err
		}
		if !found {
			return domain.Position{}, domain.Account{}, fmt.Errorf("position not found: %s", positionID)
		}
	}
	if position.Quantity <= 0 {
		return domain.Position{}, domain.Account{}, fmt.Errorf("position has no quantity to close: %s", positionID)
	}
	return position, account, nil
}

func (p *Platform) findPositionByID(positionID string) (domain.Position, bool, error) {
	positions, err := p.store.ListPositions()
	if err != nil {
		return domain.Position{}, false, err
	}
	for _, item := range positions {
		if item.ID == positionID {
			return item, true, nil
		}
	}
	return domain.Position{}, false, nil
}

func (p *Platform) validateReduceOnlyOrder(account domain.Account, order domain.Order) error {
	if !order.EffectiveReduceOnly() && !order.EffectiveClosePosition() {
		return nil
	}
	symbol := NormalizeSymbol(order.Symbol)
	position, found, err := p.resolveReduceOnlyTargetPosition(account.ID, order)
	if err != nil {
		return err
	}
	if !found || position.Quantity <= 0 {
		return fmt.Errorf("reduce-only order requires an open position for %s", symbol)
	}
	expectedSide := "SELL"
	if strings.EqualFold(strings.TrimSpace(position.Side), "SHORT") {
		expectedSide = "BUY"
	}
	if !strings.EqualFold(strings.TrimSpace(order.Side), expectedSide) {
		return fmt.Errorf("reduce-only order side %s does not reduce %s position on %s", order.Side, position.Side, symbol)
	}
	if order.Quantity <= 0 {
		return fmt.Errorf("reduce-only order quantity must be positive for %s", symbol)
	}
	if tradingQuantityExceeds(order.Quantity, position.Quantity) {
		return fmt.Errorf("reduce-only order quantity %.12f exceeds open position quantity %.12f for %s", order.Quantity, position.Quantity, symbol)
	}
	return nil
}

func (p *Platform) resolveReduceOnlyTargetPosition(accountID string, order domain.Order) (domain.Position, bool, error) {
	symbol := NormalizeSymbol(order.Symbol)
	positions, err := p.store.ListPositions()
	if err != nil {
		return domain.Position{}, false, err
	}
	candidates := make([]domain.Position, 0)
	for _, item := range positions {
		if strings.TrimSpace(item.AccountID) != strings.TrimSpace(accountID) {
			continue
		}
		if NormalizeSymbol(item.Symbol) != symbol || item.Quantity <= 0 {
			continue
		}
		candidates = append(candidates, item)
	}
	if len(candidates) == 0 {
		return domain.Position{}, false, nil
	}
	if positionID := strings.TrimSpace(stringValue(mapValue(order.Metadata)["positionId"])); positionID != "" {
		for _, item := range candidates {
			if item.ID == positionID {
				return item, true, nil
			}
		}
		return domain.Position{}, false, fmt.Errorf("reduce-only order target position %s is not open for %s", positionID, symbol)
	}
	if strategyVersionID := strings.TrimSpace(order.StrategyVersionID); strategyVersionID != "" {
		versionMatches := make([]domain.Position, 0, len(candidates))
		for _, item := range candidates {
			if strings.EqualFold(strings.TrimSpace(item.StrategyVersionID), strategyVersionID) {
				versionMatches = append(versionMatches, item)
			}
		}
		switch len(versionMatches) {
		case 0:
			return domain.Position{}, false, fmt.Errorf("reduce-only order requires an open %s position for strategy version %s", symbol, strategyVersionID)
		case 1:
			return versionMatches[0], true, nil
		default:
			return domain.Position{}, false, fmt.Errorf("reduce-only order for %s is ambiguous across %d open positions for strategy version %s; specify positionId", symbol, len(versionMatches), strategyVersionID)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], true, nil
	}
	return domain.Position{}, false, fmt.Errorf("reduce-only order for %s is ambiguous across %d open positions; specify strategyVersionId or positionId", symbol, len(candidates))
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
	runtimeSession := domain.SignalRuntimeSession{}
	sourceGate := map[string]any{"ready": true, "mode": "manual-smoke-test"}
	if !shouldSkipLiveRuntimeCheck(order) {
		runtimeSession, sourceGate, err = p.ensureLiveRuntimeReady(account, order)
		if err != nil {
			return domain.Order{}, err
		}
	}
	order, err = p.prepareLiveOrderForSubmission(account, order, binding)
	if err != nil {
		return p.applyLiveSubmissionResult(order, binding, runtimeSession, sourceGate, LiveOrderSubmission{}, err)
	}
	submission, submitErr := adapter.SubmitOrder(account, order, binding)
	return p.applyLiveSubmissionResult(order, binding, runtimeSession, sourceGate, submission, submitErr)
}

const (
	liveExecutionBoundaryClassNormalEntry           = "normal-entry"
	liveExecutionBoundaryClassNormalExit            = "normal-exit"
	liveExecutionBoundaryClassRecoveredPassiveClose = "recovered-passive-close"
	liveExecutionBoundaryClassVirtualPath           = "virtual-path"
)

func (p *Platform) prepareLiveOrderForSubmission(account domain.Account, order domain.Order, binding map[string]any) (domain.Order, error) {
	order.Metadata = cloneMetadata(order.Metadata)
	if order.Metadata == nil {
		order.Metadata = map[string]any{}
	}
	classification := classifyLiveExecutionBoundaryOrder(order)
	order.Metadata["executionBoundaryClass"] = classification
	if classification != liveExecutionBoundaryClassRecoveredPassiveClose {
		return order, nil
	}
	if !order.EffectiveReduceOnly() {
		return order, fmt.Errorf("recovered passive close requires reduceOnly semantics at execution boundary for %s", NormalizeSymbol(order.Symbol))
	}
	position, found, err := p.resolveReduceOnlyTargetPosition(account.ID, order)
	if err != nil {
		return order, err
	}
	if !found || position.Quantity <= 0 {
		return order, fmt.Errorf("recovered passive close requires an open position for %s", NormalizeSymbol(order.Symbol))
	}
	expectedSide := "SELL"
	expectedPositionSide := "LONG"
	if strings.EqualFold(strings.TrimSpace(position.Side), "SHORT") {
		expectedSide = "BUY"
		expectedPositionSide = "SHORT"
	}
	if !strings.EqualFold(strings.TrimSpace(order.Side), expectedSide) {
		return order, fmt.Errorf("recovered passive close side %s does not reduce %s position on %s", order.Side, position.Side, NormalizeSymbol(order.Symbol))
	}
	positionMode := strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(binding["positionMode"]), "ONE_WAY")))
	providedPositionSide := strings.ToUpper(strings.TrimSpace(stringValue(order.Metadata["positionSide"])))
	switch positionMode {
	case "HEDGE":
		if providedPositionSide != "" && providedPositionSide != expectedPositionSide {
			return order, fmt.Errorf("recovered passive close positionSide %s does not match %s position on %s", providedPositionSide, expectedPositionSide, NormalizeSymbol(order.Symbol))
		}
		order.Metadata["positionSide"] = expectedPositionSide
	case "ONE_WAY":
		if providedPositionSide != "" && providedPositionSide != "BOTH" {
			return order, fmt.Errorf("recovered passive close positionSide %s is invalid in ONE_WAY mode for %s", providedPositionSide, NormalizeSymbol(order.Symbol))
		}
		order.Metadata["positionSide"] = "BOTH"
	default:
		return order, fmt.Errorf("unsupported positionMode: %s", positionMode)
	}
	order.Metadata["executionBoundaryGuard"] = liveExecutionBoundaryClassRecoveredPassiveClose
	order.Metadata["executionBoundaryTargetSide"] = strings.ToUpper(strings.TrimSpace(position.Side))
	return order, nil
}

func classifyLiveExecutionBoundaryOrder(order domain.Order) string {
	metadata := mapValue(order.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	if isVirtualExecutionBoundaryOrder(order, metadata) {
		return liveExecutionBoundaryClassVirtualPath
	}
	if isRecoveredPassiveCloseOrder(order, metadata) {
		return liveExecutionBoundaryClassRecoveredPassiveClose
	}
	proposal := mapValue(firstNonEmptyMapValue(metadata["executionProposal"], metadata["intent"]))
	if order.EffectiveReduceOnly() || order.EffectiveClosePosition() || strings.EqualFold(strings.TrimSpace(stringValue(proposal["role"])), "exit") {
		return liveExecutionBoundaryClassNormalExit
	}
	return liveExecutionBoundaryClassNormalEntry
}

func isRecoveredPassiveCloseOrder(order domain.Order, metadata map[string]any) bool {
	if boolValue(metadata["recoveryCloseOnlyTakeover"]) {
		return true
	}
	proposal := mapValue(firstNonEmptyMapValue(metadata["executionProposal"], metadata["intent"]))
	proposalMeta := mapValue(proposal["metadata"])
	if boolValue(metadata["recoveryTriggered"]) || boolValue(proposalMeta["recoveryTriggered"]) {
		return strings.EqualFold(strings.TrimSpace(firstNonEmpty(stringValue(proposal["role"]), stringValue(metadata["role"]), "exit")), "exit")
	}
	return strings.EqualFold(strings.TrimSpace(stringValue(proposal["signalKind"])), "recovery-watchdog") &&
		strings.EqualFold(strings.TrimSpace(firstNonEmpty(stringValue(proposal["role"]), "exit")), "exit")
}

func isVirtualExecutionBoundaryOrder(order domain.Order, metadata map[string]any) bool {
	status := strings.ToLower(strings.TrimSpace(order.Status))
	if strings.HasPrefix(status, "virtual-") {
		return true
	}
	source := strings.ToLower(strings.TrimSpace(stringValue(metadata["source"])))
	return strings.HasPrefix(source, "virtual-")
}

func shouldSkipLiveRuntimeCheck(order domain.Order) bool {
	if order.Metadata == nil {
		return false
	}
	return boolValue(order.Metadata["skipRuntimeCheck"]) || boolValue(order.Metadata["manualTest"])
}

func (p *Platform) applyLiveSubmissionResult(
	order domain.Order,
	binding map[string]any,
	runtimeSession domain.SignalRuntimeSession,
	sourceGate map[string]any,
	submission LiveOrderSubmission,
	submitErr error,
) (domain.Order, error) {
	logger := p.logger("service.order",
		"order_id", order.ID,
		"account_id", order.AccountID,
		"symbol", NormalizeSymbol(order.Symbol),
		"side", order.Side,
		"type", order.Type,
	)
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
		if telemetryErr := p.recordLiveOrderExecutionEvent(updatedOrder, "submitted", time.Now().UTC(), true, submitErr); telemetryErr != nil {
			logger.Warn("record live order submission event failed", "error", telemetryErr)
		}
		logger.Warn("live order submission failed", "error", submitErr)
		return updatedOrder, submitErr
	}
	delete(order.Metadata, "liveSubmitError")
	order.Status = firstNonEmpty(submission.Status, "ACCEPTED")
	adoptNormalizedLiveSubmissionValues(&order, submission.Metadata)
	order.Metadata["exchangeOrderId"] = submission.ExchangeOrderID
	order.Metadata["acceptedAt"] = submission.AcceptedAt
	order.Metadata["adapterSubmission"] = submission.Metadata
	order.Metadata["lastExchangeStatus"] = order.Status
	order.Metadata["lastExchangeUpdateAt"] = firstNonEmpty(submission.AcceptedAt, time.Now().UTC().Format(time.RFC3339))
	markOrderLifecycle(order.Metadata, "accepted", true)
	if strings.EqualFold(order.Status, "FILLED") {
		markOrderLifecycle(order.Metadata, "filled", true)
	}
	logger.Info("live order submitted",
		"status", order.Status,
		"exchange_order_id", submission.ExchangeOrderID,
	)
	updatedOrder, err := p.store.UpdateOrder(order)
	if err != nil {
		return domain.Order{}, err
	}
	if telemetryErr := p.recordLiveOrderExecutionEvent(updatedOrder, "submitted", time.Now().UTC(), false, nil); telemetryErr != nil {
		logger.Warn("record live order submission event failed", "error", telemetryErr)
	}
	if strings.EqualFold(updatedOrder.Status, "FILLED") {
		settledOrder, settleErr := p.settleImmediatelyFilledLiveOrder(updatedOrder)
		if settleErr != nil {
			settledOrder = markLiveSettlementSyncRetry(settledOrder, settleErr)
			persistedOrder, updateErr := p.store.UpdateOrder(settledOrder)
			if updateErr != nil {
				return domain.Order{}, updateErr
			}
			logger.Warn("live order immediate fill sync failed",
				"exchange_order_id", submission.ExchangeOrderID,
				"error", settleErr,
			)
			return persistedOrder, nil
		}
		return settledOrder, nil
	}
	return updatedOrder, nil
}

func adoptNormalizedLiveSubmissionValues(order *domain.Order, submissionMetadata map[string]any) {
	if order == nil {
		return
	}
	submission := cloneMetadata(submissionMetadata)
	if len(submission) == 0 {
		return
	}
	normalization := mapValue(submission["normalization"])
	if quantity := firstPositive(
		parseFloatValue(submission["normalizedQuantity"]),
		parseFloatValue(normalization["normalizedQuantity"]),
	); quantity > 0 {
		order.Quantity = quantity
	}
	if price := firstPositive(
		parseFloatValue(submission["normalizedPrice"]),
		parseFloatValue(normalization["normalizedPrice"]),
	); price > 0 {
		order.Price = price
	}
}

func markLiveSettlementSyncRetry(order domain.Order, err error) domain.Order {
	order.Metadata = cloneMetadata(order.Metadata)
	order.Metadata[liveSettlementSyncErrorKey] = err.Error()
	if liveOrderFillSettlementComplete(order) && !boolValue(order.Metadata["emptyTradeRetryRequired"]) {
		delete(order.Metadata, liveSettlementSyncRequiredKey)
	} else {
		order.Metadata[liveSettlementSyncRequiredKey] = true
	}
	return order
}

func liveOrderSettlementSyncPending(order domain.Order) bool {
	return boolValue(order.Metadata[liveSettlementSyncRequiredKey]) &&
		!liveOrderFillSettlementComplete(order)
}

func liveOrderFillSettlementComplete(order domain.Order) bool {
	if order.Quantity <= 0 {
		return false
	}
	return !tradingQuantityBelow(parseFloatValue(order.Metadata["filledQuantity"]), order.Quantity)
}

func (p *Platform) settleImmediatelyFilledLiveOrder(order domain.Order) (domain.Order, error) {
	account, accountErr := p.store.GetAccount(order.AccountID)
	if accountErr == nil {
		if settledOrder, attempted, err := p.settleLiveOrderFromSubmission(account, order); attempted {
			if err != nil {
				return order, fmt.Errorf("live order %s submitted as FILLED but submission settlement failed: %w", order.ID, err)
			}
			if _, syncErr := p.requestLiveAccountSync(order.AccountID, "live-immediate-fill-settlement"); syncErr != nil {
				if errors.Is(syncErr, ErrLiveAccountOperationInProgress) {
					return settledOrder, nil
				}
				return settledOrder, fmt.Errorf("live order %s settled but account/session refresh failed: %w", order.ID, syncErr)
			}
			return settledOrder, nil
		}
	}
	settledOrder, err := p.SyncLiveOrder(order.ID)
	if err != nil {
		return order, fmt.Errorf("live order %s submitted as FILLED but settlement sync failed: %w", order.ID, err)
	}
	if _, syncErr := p.requestLiveAccountSync(order.AccountID, "live-immediate-fill-settlement"); syncErr != nil {
		if errors.Is(syncErr, ErrLiveAccountOperationInProgress) {
			return settledOrder, nil
		}
		return settledOrder, fmt.Errorf("live order %s settled but account/session refresh failed: %w", order.ID, syncErr)
	}
	return settledOrder, nil
}

func (p *Platform) settleLiveOrderFromSubmission(account domain.Account, order domain.Order) (domain.Order, bool, error) {
	syncResult, ok := liveOrderSubmissionSettlementSync(order)
	if !ok {
		return order, false, nil
	}
	settledOrder, err := p.applyLiveSyncResult(account, order, syncResult)
	if err != nil {
		return order, true, err
	}
	return settledOrder, true, nil
}

func liveOrderSubmissionSettlementSync(order domain.Order) (LiveOrderSync, bool) {
	submission := cloneMetadata(mapValue(order.Metadata["adapterSubmission"]))
	if len(submission) == 0 {
		return LiveOrderSync{}, false
	}
	status := firstNonEmpty(
		mapBinanceOrderStatus(stringValue(submission["binanceStatus"])),
		stringValue(submission["status"]),
		stringValue(order.Metadata["lastExchangeStatus"]),
		order.Status,
	)
	if !strings.EqualFold(strings.TrimSpace(status), "FILLED") {
		return LiveOrderSync{}, false
	}
	filledQty := firstPositive(
		parseFloatValue(submission["executedQty"]),
		firstPositive(
			parseFloatValue(submission["cumQty"]),
			parseFloatValue(submission["filledQuantity"]),
		),
	)
	if !tradingQuantityPositive(filledQty) {
		return LiveOrderSync{}, false
	}
	submission["source"] = firstNonEmpty(stringValue(submission["source"]), "live-submission-result")
	submission["exchangeOrderId"] = firstNonEmpty(stringValue(submission["exchangeOrderId"]), stringValue(order.Metadata["exchangeOrderId"]))
	submission["clientOrderId"] = firstNonEmpty(stringValue(submission["clientOrderId"]), order.ID)
	submission["executedQty"] = filledQty
	submission["cumQty"] = firstPositive(parseFloatValue(submission["cumQty"]), filledQty)
	syncedAt := firstNonEmpty(
		stringValue(submission["updateTime"]),
		stringValue(order.Metadata["lastExchangeUpdateAt"]),
		stringValue(order.Metadata["acceptedAt"]),
		time.Now().UTC().Format(time.RFC3339),
	)
	return LiveOrderSync{
		Status:     "FILLED",
		SyncedAt:   syncedAt,
		Metadata:   submission,
		Terminal:   true,
		FeeSource:  firstNonEmpty(stringValue(submission["feeSource"]), "exchange"),
		FundingSrc: firstNonEmpty(stringValue(submission["fundingSource"]), "exchange"),
	}, true
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
	logger := p.logger("service.order", "order_id", orderID)
	order, account, adapter, binding, err := p.resolveLiveOrderContext(orderID)
	if err != nil {
		logger.Warn("resolve live order context failed", "error", err)
		return domain.Order{}, err
	}
	syncResult, err := adapter.SyncOrder(account, order, binding)
	if err != nil {
		logger.Warn("sync live order failed", "error", err)
		return domain.Order{}, err
	}
	logger.Debug("live order sync received", "status", syncResult.Status, "fill_count", len(syncResult.Fills))
	return p.applyLiveSyncResult(account, order, syncResult)
}

func (p *Platform) CancelLiveOrder(orderID string) (domain.Order, error) {
	logger := p.logger("service.order", "order_id", orderID)
	order, account, adapter, binding, err := p.resolveLiveOrderContext(orderID)
	if err != nil {
		logger.Warn("resolve live order context failed", "error", err)
		return domain.Order{}, err
	}
	syncResult, err := adapter.CancelOrder(account, order, binding)
	if err != nil {
		logger.Warn("cancel live order failed", "error", err)
		return domain.Order{}, err
	}
	logger.Info("live order cancellation acknowledged", "status", syncResult.Status)
	return p.applyLiveSyncResult(account, order, syncResult)
}

func (p *Platform) resolveLiveOrderContext(orderID string) (domain.Order, domain.Account, LiveExecutionAdapter, map[string]any, error) {
	order, err := p.GetOrder(orderID)
	if err != nil {
		return domain.Order{}, domain.Account{}, nil, nil, err
	}
	account, err := p.store.GetAccount(order.AccountID)
	if err != nil {
		return domain.Order{}, domain.Account{}, nil, nil, err
	}
	if account.Mode != "LIVE" {
		return domain.Order{}, domain.Account{}, nil, nil, fmt.Errorf("order %s is not a live order", orderID)
	}
	adapter, binding, err := p.resolveLiveAdapterForAccount(account)
	if err != nil {
		return domain.Order{}, domain.Account{}, nil, nil, err
	}
	return order, account, adapter, binding, nil
}

func (p *Platform) applyLiveSyncResult(account domain.Account, order domain.Order, syncResult LiveOrderSync) (domain.Order, error) {
	logger := p.logger("service.order",
		"order_id", order.ID,
		"account_id", order.AccountID,
		"symbol", NormalizeSymbol(order.Symbol),
	)
	order.Metadata = cloneMetadata(order.Metadata)
	adoptNormalizedLiveSubmissionValues(&order, mapValue(order.Metadata["adapterSubmission"]))
	applyExecutionMetadata(order.Metadata, map[string]any{
		"lastSyncAt":           syncResult.SyncedAt,
		"syncMode":             "adapter",
		"feeSource":            firstNonEmpty(syncResult.FeeSource, "exchange"),
		"fundingSource":        firstNonEmpty(syncResult.FundingSrc, "exchange"),
		"adapterSync":          syncResult.Metadata,
		"lastExchangeStatus":   firstNonEmpty(syncResult.Status, order.Status),
		"lastExchangeUpdateAt": firstNonEmpty(syncResult.SyncedAt, time.Now().UTC().Format(time.RFC3339)),
	})
	if val, ok := syncResult.Metadata["emptyTradeSyncAttempts"]; ok {
		order.Metadata["emptyTradeSyncAttempts"] = val
	}
	if val, ok := syncResult.Metadata["emptyTradeRetryRequired"]; ok {
		order.Metadata["emptyTradeRetryRequired"] = val
	}
	order.Status = firstNonEmpty(syncResult.Status, order.Status)
	adoptTerminalReduceOnlyFilledQuantity(&order, syncResult)
	if strings.EqualFold(order.Status, "CANCELLED") || strings.EqualFold(order.Status, "REJECTED") {
		markOrderLifecycle(order.Metadata, "filled", false)
	}
	markOrderLifecycle(order.Metadata, "synced", true)
	fills, lastFundingPnL, lastPrice := buildLiveSyncSettlement(order, syncResult)
	if len(fills) > 0 {
		order.Price = lastPrice
		order.Metadata["fundingPnL"] = lastFundingPnL
		order.Metadata["fillCount"] = len(fills)
		order.Metadata["lastFillAt"] = firstNonEmpty(syncResult.SyncedAt, time.Now().UTC().Format(time.RFC3339))
		markOrderLifecycle(order.Metadata, "filled", true)
		logger.Info("live order synced with fills",
			"status", order.Status,
			"fill_count", len(fills),
			"last_price", lastPrice,
			"funding_pnl", lastFundingPnL,
		)
		if telemetryErr := p.recordLiveOrderExecutionEvent(order, "synced", parseOptionalRFC3339(firstNonEmpty(syncResult.SyncedAt, stringValue(order.Metadata["lastSyncAt"]))), false, nil); telemetryErr != nil {
			logger.Warn("record live order sync event failed", "error", telemetryErr)
		}
		return p.finalizeExecutedOrder(account, order, fills)
	}
	logger.Debug("live order sync applied", "status", order.Status)
	updatedOrder, err := p.store.UpdateOrder(order)
	if err != nil {
		return domain.Order{}, err
	}
	if telemetryErr := p.recordLiveOrderExecutionEvent(updatedOrder, "synced", parseOptionalRFC3339(firstNonEmpty(syncResult.SyncedAt, stringValue(updatedOrder.Metadata["lastSyncAt"]))), false, nil); telemetryErr != nil {
		logger.Warn("record live order sync event failed", "error", telemetryErr)
	}
	return updatedOrder, nil
}

func adoptTerminalReduceOnlyFilledQuantity(order *domain.Order, syncResult LiveOrderSync) {
	if order == nil || !strings.EqualFold(strings.TrimSpace(syncResult.Status), "FILLED") {
		return
	}
	if !order.EffectiveReduceOnly() && !order.EffectiveClosePosition() {
		return
	}
	terminalQuantity := terminalFilledSyncQuantity(syncResult)
	if !tradingQuantityPositive(terminalQuantity) {
		return
	}
	if order.Quantity <= 0 || !tradingQuantityBelow(terminalQuantity, order.Quantity) {
		return
	}
	if order.Metadata == nil {
		order.Metadata = map[string]any{}
	}
	order.Metadata["terminalSyncOriginalQuantity"] = order.Quantity
	order.Metadata["terminalSyncFilledQuantity"] = terminalQuantity
	order.Metadata["terminalSyncQuantityAdjusted"] = true
	order.Quantity = terminalQuantity
}

func terminalFilledSyncQuantity(syncResult LiveOrderSync) float64 {
	metadata := mapValue(syncResult.Metadata)
	quantity := firstPositive(
		parseFloatValue(metadata["executedQty"]),
		firstPositive(
			parseFloatValue(metadata["cumQty"]),
			firstPositive(
				parseFloatValue(metadata["filledQuantity"]),
				sumLiveFillReportQuantity(syncResult.Fills),
			),
		),
	)
	return firstPositive(quantity, parseFloatValue(metadata["origQty"]))
}

func sumLiveFillReportQuantity(fills []LiveFillReport) float64 {
	total := 0.0
	for _, fill := range fills {
		total += fill.Quantity
	}
	return total
}

func buildLiveSyncSettlement(order domain.Order, syncResult LiveOrderSync) ([]domain.Fill, float64, float64) {
	reports := syncResult.Fills
	if len(reports) == 0 {
		if fallback, ok := buildTerminalFilledFallbackReport(order, syncResult); ok {
			reports = []LiveFillReport{fallback}
		}
	}
	fills := make([]domain.Fill, 0, len(reports))
	lastFundingPnL := 0.0
	lastPrice := order.Price
	for _, report := range reports {
		if report.Quantity <= 0 {
			continue
		}
		price := report.Price
		if price <= 0 {
			price = resolveExecutionPrice(order)
		}
		lastPrice = price
		lastFundingPnL = report.FundingPnL
		fills = append(fills, domain.Fill{
			OrderID:           order.ID,
			ExchangeTradeID:   resolveLiveFillTradeID(report),
			ExchangeTradeTime: resolveLiveFillTradeTime(report),
			DedupFingerprint:  strings.TrimSpace(stringValue(mapValue(report.Metadata)["dedupFingerprint"])),
			Source:            string(resolveLiveFillSource(report)),
			Price:             price,
			Quantity:          report.Quantity,
			Fee:               report.Fee - report.FundingPnL,
		})
	}
	return fills, lastFundingPnL, lastPrice
}

func buildTerminalFilledFallbackReport(order domain.Order, syncResult LiveOrderSync) (LiveFillReport, bool) {
	if !strings.EqualFold(strings.TrimSpace(syncResult.Status), "FILLED") {
		return LiveFillReport{}, false
	}
	metadata := cloneMetadata(syncResult.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	if boolValue(metadata["emptyTradeRetryRequired"]) {
		return LiveFillReport{}, false
	}
	totalFilledQty := firstPositive(
		parseFloatValue(metadata["executedQty"]),
		firstPositive(
			parseFloatValue(metadata["cumQty"]),
			firstPositive(
				parseFloatValue(metadata["filledQuantity"]),
				order.Quantity,
			),
		),
	)
	if totalFilledQty <= 0 {
		return LiveFillReport{}, false
	}
	alreadyFilledQty := parseFloatValue(order.Metadata["filledQuantity"])
	fallbackQty := totalFilledQty - alreadyFilledQty
	if order.Quantity > 0 && tradingQuantityExceeds(fallbackQty, order.Quantity-alreadyFilledQty) {
		fallbackQty = order.Quantity - alreadyFilledQty
	}
	if !tradingQuantityPositive(fallbackQty) {
		return LiveFillReport{}, false
	}
	price := firstPositive(
		parseFloatValue(metadata["avgPrice"]),
		firstPositive(
			parseFloatValue(metadata["price"]),
			firstPositive(order.Price, resolveExecutionPrice(order)),
		),
	)
	tradeTime := firstNonEmpty(
		stringValue(metadata["updateTime"]),
		syncResult.SyncedAt,
		stringValue(order.Metadata["lastExchangeUpdateAt"]),
		stringValue(order.Metadata["acceptedAt"]),
		time.Now().UTC().Format(time.RFC3339),
	)
	exchangeOrderID := firstNonEmpty(
		stringValue(metadata["exchangeOrderId"]),
		stringValue(order.Metadata["exchangeOrderId"]),
	)
	reportSource := firstNonEmpty(stringValue(metadata["reportSource"]), stringValue(metadata["source"]), "terminal-filled-order-fallback")
	metadata["source"] = firstNonEmpty(stringValue(metadata["source"]), reportSource)
	metadata["reportSource"] = reportSource
	metadata["exchangeOrderId"] = exchangeOrderID
	metadata["clientOrderId"] = firstNonEmpty(stringValue(metadata["clientOrderId"]), order.ID)
	metadata["tradeTime"] = tradeTime
	metadata["syntheticFill"] = true
	metadata["dedupFingerprint"] = terminalFilledFallbackDedupFingerprint(order, syncResult, totalFilledQty)
	return LiveFillReport{
		Price:    price,
		Quantity: fallbackQty,
		Fee:      0,
		Source:   FillSourceSynthetic,
		Metadata: metadata,
	}, true
}

func terminalFilledFallbackDedupFingerprint(order domain.Order, syncResult LiveOrderSync, totalFilledQty float64) string {
	metadata := mapValue(syncResult.Metadata)
	return strings.Join([]string{
		"terminal-filled-order-fallback",
		strings.TrimSpace(order.ID),
		firstNonEmpty(stringValue(metadata["exchangeOrderId"]), stringValue(order.Metadata["exchangeOrderId"])),
		fmt.Sprintf("%.12f", totalFilledQty),
	}, "|")
}

func (p *Platform) finalizeExecutedOrder(account domain.Account, order domain.Order, fills []domain.Fill) (domain.Order, error) {
	if len(fills) == 0 {
		return p.store.UpdateOrder(order)
	}

	var updatedOrder domain.Order
	var newFills []domain.Fill
	if err := p.store.WithFillSettlementTx(order.ID, func(tx storepkg.FillSettlementStore) error {
		filteredFills, err := filterExistingExecutionFillsWithStore(tx, order.ID, fills)
		if err != nil {
			return err
		}
		newFills = filteredFills

		existingFills, err := tx.QueryFills(domain.FillQuery{OrderIDs: []string{order.ID}})
		if err != nil {
			return err
		}
		existingInputs, err := fillReconciliationInputsFromStoredFills(existingFills)
		if err != nil {
			return err
		}
		incomingInputs, err := fillReconciliationInputsFromIncomingFills(order, newFills)
		if err != nil {
			return err
		}
		plan, err := BuildFillReconciliationPlan(order, existingInputs, incomingInputs, FillReconcilePolicy{AllowSyntheticFallback: true})
		if err != nil {
			return err
		}

		if len(plan.DeleteFillIDs) > 0 {
			if _, err := tx.DeleteFillsByID(plan.DeleteFillIDs); err != nil {
				return fmt.Errorf("failed to delete synthetic fills before upgrade: %w", err)
			}
		}

		lastPrice := order.Price
		for _, fill := range plan.CreateFills {
			createdFill, err := tx.CreateFill(fill)
			if err != nil {
				return err
			}
			executionPrice := createdFill.Price
			if executionPrice <= 0 {
				executionPrice = resolveExecutionPrice(order)
			}
			lastPrice = executionPrice
		}

		for _, fill := range plan.ApplyPositionFills {
			executionPrice := fill.Price
			if executionPrice <= 0 {
				executionPrice = resolveExecutionPrice(order)
			}
			execOrder := order
			execOrder.Quantity = fill.Quantity
			execOrder.Price = executionPrice
			if err := p.applyExecutionFillWithStore(tx, account, execOrder, executionPrice); err != nil {
				return err
			}
		}

		filledQuantity, err := tx.TotalFilledQuantityForOrder(order.ID)
		if err != nil {
			return err
		}
		order.Metadata["filledQuantity"] = filledQuantity
		remainingQuantity := order.Quantity - filledQuantity
		if remainingQuantity < 0 {
			remainingQuantity = 0
		}
		order.Metadata["remainingQuantity"] = remainingQuantity

		orderCompletelyFilled := !tradingQuantityBelow(filledQuantity, order.Quantity)
		if orderCompletelyFilled {
			order.Status = "FILLED"
			if !boolValue(order.Metadata["emptyTradeRetryRequired"]) {
				delete(order.Metadata, liveSettlementSyncRequiredKey)
			}
			delete(order.Metadata, liveSettlementSyncErrorKey)
		} else if isTerminalOrderStatus(order.Status) && !strings.EqualFold(order.Status, "FILLED") {
			if !boolValue(order.Metadata["emptyTradeRetryRequired"]) {
				delete(order.Metadata, liveSettlementSyncRequiredKey)
			}
			delete(order.Metadata, liveSettlementSyncErrorKey)
		} else if filledQuantity > 0 {
			order.Status = "PARTIALLY_FILLED"
		}
		order.Price = lastPrice
		markOrderLifecycle(order.Metadata, "filled", orderCompletelyFilled)
		if order.Metadata["acceptedAt"] == nil {
			order.Metadata["acceptedAt"] = time.Now().UTC().Format(time.RFC3339)
		}
		if (len(newFills) > 0 || strings.TrimSpace(stringValue(order.Metadata["lastFilledAt"])) == "") && filledQuantity > 0 {
			filledAt := time.Now().UTC()
			if latestTradeTime := latestFillExchangeTradeTime(newFills); !latestTradeTime.IsZero() {
				filledAt = latestTradeTime
			}
			order.Metadata["lastFilledAt"] = filledAt.Format(time.RFC3339)
		}
		updated, err := tx.UpdateOrder(order)
		if err != nil {
			return err
		}
		updatedOrder = updated
		return nil
	}); err != nil {
		return domain.Order{}, err
	}
	if strings.EqualFold(account.Mode, "LIVE") && len(newFills) > 0 {
		if telemetryErr := p.recordLiveOrderExecutionEvent(updatedOrder, "filled", parseOptionalRFC3339(stringValue(updatedOrder.Metadata["lastFilledAt"])), false, nil); telemetryErr != nil {
			p.logger("service.order", "order_id", updatedOrder.ID).Warn("record live order fill event failed", "error", telemetryErr)
		}
	}
	if err := p.captureAccountSnapshot(account.ID); err != nil {
		return domain.Order{}, err
	}
	levelLogger := p.logger("service.order",
		"order_id", updatedOrder.ID,
		"account_id", updatedOrder.AccountID,
		"symbol", NormalizeSymbol(updatedOrder.Symbol),
	)
	if strings.EqualFold(account.Mode, "LIVE") {
		if strings.EqualFold(updatedOrder.Status, "FILLED") {
			levelLogger.Info("order filled", "fill_count", len(newFills), "price", updatedOrder.Price)
		} else {
			levelLogger.Info("order partially filled", "fill_count", len(newFills), "price", updatedOrder.Price)
		}
	} else {
		levelLogger.Debug("paper order filled", "fill_count", len(newFills), "price", updatedOrder.Price)
	}
	return updatedOrder, nil
}

func fillReconciliationInputsFromStoredFills(fills []domain.Fill) ([]FillReconciliationInput, error) {
	inputs := make([]FillReconciliationInput, 0, len(fills))
	for _, fill := range fills {
		source, ok := fillReconciliationSourceFromStoredFill(fill)
		if !ok {
			return nil, fmt.Errorf("stored fill %s has ambiguous source", fill.ID)
		}
		inputs = append(inputs, FillReconciliationInput{Fill: fill, Source: source})
	}
	return inputs, nil
}

func fillReconciliationInputsFromIncomingFills(order domain.Order, fills []domain.Fill) ([]FillReconciliationInput, error) {
	inputs := make([]FillReconciliationInput, 0, len(fills))
	for _, fill := range fills {
		if strings.TrimSpace(fill.OrderID) == "" {
			fill.OrderID = order.ID
		}
		source := FillSourceReal
		if strings.TrimSpace(fill.Source) != "" {
			source = FillSource(strings.TrimSpace(fill.Source))
		} else if strings.TrimSpace(fill.ExchangeTradeID) == "" {
			fill.DedupFingerprint = strings.TrimSpace(fill.DedupFingerprint)
			if fill.DedupFingerprint == "" {
				fill.DedupFingerprint = fill.FallbackFingerprint()
			}
			source = FillSourceSynthetic
			if strings.HasPrefix(fill.DedupFingerprint, syntheticRemainderFingerprintPrefix) {
				source = FillSourceRemainder
			}
		}
		inputs = append(inputs, FillReconciliationInput{Fill: fill, Source: source})
	}
	return inputs, nil
}

func fillReconciliationSourceFromStoredFill(fill domain.Fill) (FillSource, bool) {
	if source := strings.TrimSpace(fill.Source); source != "" {
		switch FillSource(source) {
		case FillSourceReal, FillSourceSynthetic, FillSourceRemainder:
			return FillSource(source), true
		default:
			return "", false
		}
	}
	if strings.TrimSpace(fill.ExchangeTradeID) != "" {
		return FillSourceReal, true
	}
	fingerprint := strings.TrimSpace(fill.DedupFingerprint)
	if fingerprint == "" {
		return "", false
	}
	if strings.HasPrefix(fingerprint, syntheticRemainderFingerprintPrefix) {
		return FillSourceRemainder, true
	}
	return FillSourceSynthetic, true
}

func (p *Platform) filterExistingExecutionFills(orderID string, fills []domain.Fill) ([]domain.Fill, error) {
	return filterExistingExecutionFillsWithStore(p.store, orderID, fills)
}

type fillQueryStore interface {
	QueryFills(query domain.FillQuery) ([]domain.Fill, error)
}

func filterExistingExecutionFillsWithStore(store fillQueryStore, orderID string, fills []domain.Fill) ([]domain.Fill, error) {
	if len(fills) == 0 {
		return nil, nil
	}
	existing, err := store.QueryFills(domain.FillQuery{OrderIDs: []string{orderID}})
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(existing)+len(fills))
	globalTradeSeen := make(map[string]domain.Fill, len(existing)+len(fills))
	for _, item := range existing {
		if item.ExchangeTradeID != "" {
			globalTradeSeen[strings.TrimSpace(item.ExchangeTradeID)] = item
		}
		if item.OrderID != orderID {
			continue
		}
		key := buildFillDedupKey(item)
		if key == "" {
			continue
		}
		seen[key] = struct{}{}
	}
	filtered := make([]domain.Fill, 0, len(fills))
	for _, fill := range fills {
		fill.OrderID = orderID
		if strings.TrimSpace(fill.ExchangeTradeID) == "" && strings.TrimSpace(fill.DedupFingerprint) == "" {
			fill.DedupFingerprint = fill.FallbackFingerprint()
		}
		if tradeID := strings.TrimSpace(fill.ExchangeTradeID); tradeID != "" {
			if existingFill, exists := globalTradeSeen[tradeID]; exists {
				if fill.Fee == 0 || tradingQuantityEqual(fill.Fee, existingFill.Fee) {
					continue
				}
				filtered = append(filtered, fill)
				continue
			} else {
				globalTradeSeen[tradeID] = fill
			}
		}
		key := buildFillDedupKey(fill)
		if key == "" {
			filtered = append(filtered, fill)
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, fill)
	}
	return filtered, nil
}

func buildFillDedupKey(fill domain.Fill) string {
	orderID := strings.TrimSpace(fill.OrderID)
	if orderID == "" {
		return ""
	}
	if exchangeTradeID := strings.TrimSpace(fill.ExchangeTradeID); exchangeTradeID != "" {
		return orderID + "|trade|" + exchangeTradeID
	}
	fingerprint := strings.TrimSpace(fill.DedupFingerprint)
	if fingerprint == "" {
		fingerprint = fill.FallbackFingerprint()
	}
	return orderID + "|fallback|" + fingerprint
}

func limitExecutionFillsToRemainingQuantity(fills []domain.Fill, remainingQuantity float64) []domain.Fill {
	if len(fills) == 0 || !tradingQuantityPositive(remainingQuantity) {
		return nil
	}
	limited := make([]domain.Fill, 0, len(fills))
	remaining := remainingQuantity
	for _, fill := range fills {
		if !tradingQuantityPositive(fill.Quantity) || !tradingQuantityPositive(remaining) {
			continue
		}
		if tradingQuantityExceeds(fill.Quantity, remaining) {
			ratio := remaining / fill.Quantity
			fill.Quantity = remaining
			fill.Fee *= ratio
		}
		limited = append(limited, fill)
		remaining -= fill.Quantity
		if remaining < 0 && !tradingQuantityExceeds(-remaining, 0) {
			remaining = 0
		}
	}
	return limited
}

func resolveLiveFillTradeID(report LiveFillReport) string {
	metadata := mapValue(report.Metadata)
	return strings.TrimSpace(firstNonEmpty(
		stringifyBinanceID(metadata["tradeId"]),
		stringifyBinanceID(metadata["exchangeTradeId"]),
	))
}

func resolveLiveFillSource(report LiveFillReport) FillSource {
	if report.Source != "" {
		return report.Source
	}
	metadata := mapValue(report.Metadata)
	if boolValue(metadata["syntheticFill"]) {
		return FillSourceSynthetic
	}
	if strings.TrimSpace(stringValue(metadata["dedupFingerprint"])) != "" {
		return FillSourceSynthetic
	}
	if resolveLiveFillTradeID(report) != "" {
		return FillSourceReal
	}
	return ""
}

func resolveLiveFillTradeTime(report LiveFillReport) *time.Time {
	metadata := mapValue(report.Metadata)
	tradeTime := parseOptionalRFC3339(stringValue(metadata["tradeTime"]))
	if tradeTime.IsZero() {
		return nil
	}
	resolved := tradeTime.UTC()
	return &resolved
}

func latestFillExchangeTradeTime(fills []domain.Fill) time.Time {
	var latest time.Time
	for _, fill := range fills {
		if fill.ExchangeTradeTime == nil || fill.ExchangeTradeTime.IsZero() {
			continue
		}
		if fill.ExchangeTradeTime.After(latest) {
			latest = fill.ExchangeTradeTime.UTC()
		}
	}
	return latest
}

func (p *Platform) resolveLiveAdapterForAccount(account domain.Account) (LiveExecutionAdapter, map[string]any, error) {
	binding := resolveLiveBinding(account)
	if len(binding) == 0 {
		return nil, nil, liveControlConfigErrorf("live account %s has no adapter binding", account.ID)
	}
	adapterKey := normalizeLiveAdapterKey(stringValue(binding["adapterKey"]))
	adapter, ok := p.liveAdapters[adapterKey]
	if !ok {
		return nil, nil, liveControlConfigErrorf("live adapter not registered: %s", adapterKey)
	}
	if err := adapter.ValidateAccountConfig(binding); err != nil {
		return nil, nil, wrapLiveControlConfigError(err)
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
	positions, err := p.store.ListPositions()
	if err != nil {
		return nil, err
	}
	active := make([]domain.Position, 0, len(positions))
	for _, position := range positions {
		if position.Quantity <= 0 {
			continue
		}
		active = append(active, position)
	}
	return active, nil
}

// ListFills 获取所有成交记录列表。
func (p *Platform) ListFills() ([]domain.Fill, error) {
	return p.store.ListFills()
}

// ListFillsWithLimit 获取限制数量的成交记录，按时间降序（最新优先）。
func (p *Platform) ListFillsWithLimit(limit, offset int) ([]domain.Fill, error) {
	return p.store.ListFillsWithLimit(limit, offset)
}

// CountFills 获取成交总数。
func (p *Platform) CountFills() (int, error) {
	return p.store.CountFills()
}

// applyExecutionFill 根据已确认成交更新仓位。
// 它是 paper/live 共用的持仓落账逻辑，只处理 canonical fill 之后的仓位变更。
func (p *Platform) applyExecutionFill(account domain.Account, order domain.Order, executionPrice float64) error {
	return p.applyExecutionFillWithStore(p.store, account, order, executionPrice)
}

func (p *Platform) applyExecutionFillWithStore(settlementStore storepkg.FillSettlementStore, account domain.Account, order domain.Order, executionPrice float64) error {
	if boolValue(order.Metadata["reconcileRecovered"]) {
		snapshotQty := liveSyncSnapshotPositionAmounts(account)[NormalizeSymbol(order.Symbol)]
		if !tradingQuantityPositive(snapshotQty) {
			p.logger("service.order",
				"account_id", account.ID,
				"order_id", order.ID,
				"symbol", NormalizeSymbol(order.Symbol),
			).Warn("skip reconcile fill position apply without authoritative live position")
			return nil
		}
	}
	position, exists, err := settlementStore.FindPosition(account.ID, order.Symbol)
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
		_, err := settlementStore.SavePosition(domain.Position{
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
		_, err := settlementStore.SavePosition(position)
		return err
	}

	// 反方向 → 部分平仓
	if tradingQuantityBelow(order.Quantity, position.Quantity) {
		position.Quantity = position.Quantity - order.Quantity
		if position.Quantity <= 0 {
			return settlementStore.DeletePosition(position.ID)
		}
		position.MarkPrice = executionPrice
		_, err := settlementStore.SavePosition(position)
		return err
	}

	// 反方向 → 全部平仓
	if tradingQuantityEqual(order.Quantity, position.Quantity) {
		err := settlementStore.DeletePosition(position.ID)
		if err == nil && strings.EqualFold(account.Mode, "LIVE") {
			liveSessionID := stringValue(order.Metadata["liveSessionId"])
			decisionEventID := stringValue(order.Metadata["decisionEventId"])
			if liveSessionID != "" {
				strategyID := ""
				if resolved, resolveErr := p.resolveLiveStrategyIDForOrder(account.ID, order); resolveErr == nil {
					strategyID = resolved
				}
				// NOTE(audit): OrderCloseVerification is designed as an append-only log.
				// By default, the latest event (ordered by EventTime desc) determines the authoritative verification state
				// for a given order in `enrichLiveTradePairs`.
				// While `ws-sync` is an optimistic source, subsequent reconcile events can append a newer record
				// with `VerifiedClosed=false` if residual position is detected.
				_, _ = settlementStore.CreateOrderCloseVerification(domain.OrderCloseVerification{
					LiveSessionID:        liveSessionID,
					OrderID:              order.ID,
					DecisionEventID:      decisionEventID,
					AccountID:            account.ID,
					StrategyID:           strategyID,
					Symbol:               position.Symbol,
					VerifiedClosed:       true,
					RemainingPositionQty: 0,
					VerificationSource:   "ws-sync",
					EventTime:            time.Now().UTC(),
				})
			}
		}
		return err
	}

	// 反方向 → 平仓后反向开仓
	remaining := order.Quantity - position.Quantity
	position.Side = targetSide
	position.Quantity = remaining
	position.EntryPrice = executionPrice
	position.MarkPrice = executionPrice
	position.StrategyVersionID = firstNonEmpty(order.StrategyVersionID, position.StrategyVersionID)
	_, err = settlementStore.SavePosition(position)
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
