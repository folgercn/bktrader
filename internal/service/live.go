package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type LiveLaunchOptions struct {
	StrategyID             string           `json:"strategyId"`
	Binding                map[string]any   `json:"binding,omitempty"`
	StrategySignalBindings []map[string]any `json:"strategySignalBindings,omitempty"`
	LiveSessionOverrides   map[string]any   `json:"liveSessionOverrides,omitempty"`
	LaunchTemplateKey      string           `json:"launchTemplateKey,omitempty"`
	LaunchTemplateName     string           `json:"launchTemplateName,omitempty"`
	// MirrorStrategySignals is retained for backward-compatible launch payloads.
	// It no longer mirrors strategy signal bindings onto account metadata; when true,
	// LaunchLiveFlow only validates that strategy bindings already exist before startup.
	MirrorStrategySignals bool `json:"mirrorStrategySignals"`
	StartRuntime          bool `json:"startRuntime"`
	StartSession          bool `json:"startSession"`
}

type LiveLaunchResult struct {
	Account               domain.Account              `json:"account"`
	RuntimeSession        domain.SignalRuntimeSession `json:"runtimeSession"`
	LiveSession           domain.LiveSession          `json:"liveSession"`
	MirroredBindingCount  int                         `json:"mirroredBindingCount"`
	AccountBindingApplied bool                        `json:"accountBindingApplied"`
	TemplateApplied       bool                        `json:"templateApplied"`
	TemplateBindingCount  int                         `json:"templateBindingCount"`
	// RuntimePlanRefreshed means the stored runtime plan/subscription state was
	// rebuilt from the replacement template bindings after any running runtime
	// for the same account+strategy was stopped. Actual transport subscription
	// preparation still happens on the next StartSignalRuntimeSession call.
	RuntimePlanRefreshed bool `json:"runtimePlanRefreshed"`
	// StoppedLiveSessions counts RUNNING live sessions in the same
	// account+strategy scope whose symbol/timeframe no longer matches the target
	// launch template. Sessions from other accounts or strategies are left alone.
	StoppedLiveSessions   int  `json:"stoppedLiveSessions"`
	RuntimeSessionCreated bool `json:"runtimeSessionCreated"`
	RuntimeSessionStarted bool `json:"runtimeSessionStarted"`
	LiveSessionCreated    bool `json:"liveSessionCreated"`
	LiveSessionStarted    bool `json:"liveSessionStarted"`
}

type LiveAccountReconcileOptions struct {
	LookbackHours int `json:"lookbackHours"`
}

type LiveAccountReconcileResult struct {
	Account           domain.Account `json:"account"`
	AdapterKey        string         `json:"adapterKey"`
	ExecutionMode     string         `json:"executionMode"`
	LookbackHours     int            `json:"lookbackHours"`
	SymbolCount       int            `json:"symbolCount"`
	Symbols           []string       `json:"symbols"`
	OrderCount        int            `json:"orderCount"`
	CreatedOrderCount int            `json:"createdOrderCount"`
	UpdatedOrderCount int            `json:"updatedOrderCount"`
	Notes             []string       `json:"notes,omitempty"`
}

type liveOrderReconcileIndex struct {
	byID              map[string]domain.Order
	byExchangeOrderID map[string]domain.Order
	byClientOrderID   map[string]domain.Order
}

type liveAccountSyncSuccessOwner interface {
	PersistsLiveAccountSyncSuccess() bool
}

const (
	liveOrderStatusVirtualInitial = "VIRTUAL_INITIAL"
	liveOrderStatusVirtualExit    = "VIRTUAL_EXIT"
)

func (p *Platform) ListLiveSessions() ([]domain.LiveSession, error) {
	return p.store.ListLiveSessions()
}

func (p *Platform) DeleteLiveSession(sessionID string) error {
	return p.DeleteLiveSessionWithForce(sessionID, false)
}

func (p *Platform) DeleteLiveSessionWithForce(sessionID string, force bool) error {
	p.logger("service.live", "session_id", sessionID).Info("deleting live session")
	session, err := p.store.GetLiveSession(sessionID)
	if err != nil {
		return err
	}
	if !force {
		if err := p.ensureNoActivePositionsOrOrders(session.AccountID, session.StrategyID); err != nil {
			return err
		}
	}
	return p.store.DeleteLiveSession(sessionID)
}

func (p *Platform) UpdateLiveSession(sessionID, accountID, strategyID string, overrides map[string]any) (domain.LiveSession, error) {
	logger := p.logger("service.live", "session_id", sessionID)
	session, err := p.store.GetLiveSession(sessionID)
	if err != nil {
		logger.Warn("load live session failed", "error", err)
		return domain.LiveSession{}, err
	}
	if strings.TrimSpace(accountID) != "" {
		account, err := p.store.GetAccount(accountID)
		if err != nil {
			return domain.LiveSession{}, err
		}
		if !strings.EqualFold(account.Mode, "LIVE") {
			return domain.LiveSession{}, fmt.Errorf("live session requires a LIVE account: %s", accountID)
		}
		session.AccountID = accountID
	}
	if strings.TrimSpace(strategyID) != "" {
		session.StrategyID = strategyID
	}
	state := cloneMetadata(session.State)
	for key, value := range normalizeLiveSessionOverrides(overrides) {
		state[key] = value
	}
	session.State = state
	session, err = p.store.UpdateLiveSession(session)
	if err != nil {
		logger.Error("update live session failed", "error", err)
		return domain.LiveSession{}, err
	}
	updated, err := p.syncLiveSessionRuntime(session)
	if err != nil {
		logger.Warn("sync live session runtime failed after update", "error", err)
		return domain.LiveSession{}, err
	}
	p.logger("service.live",
		"session_id", updated.ID,
		"account_id", updated.AccountID,
		"strategy_id", updated.StrategyID,
	).Info("live session updated", "override_count", len(overrides))
	return updated, nil
}

func (p *Platform) SyncLiveAccount(accountID string) (domain.Account, error) {
	logger := p.logger("service.live", "account_id", accountID)
	logger.Debug("syncing live account")
	attemptedAt := time.Now().UTC()
	account, err := p.store.GetAccount(accountID)
	if err != nil {
		logger.Warn("load live account failed", "error", err)
		return domain.Account{}, err
	}
	if !strings.EqualFold(account.Mode, "LIVE") {
		return domain.Account{}, fmt.Errorf("account %s is not a LIVE account", accountID)
	}
	previousSuccessAt := parseOptionalRFC3339(stringValue(account.Metadata["lastLiveSyncAt"]))
	adapter, binding, err := p.resolveLiveAdapterForAccount(account)
	if err != nil {
		logger.Warn("resolve live adapter for account sync failed", "error", err)
		account = p.persistLiveAccountSyncFailure(account, attemptedAt, err)
		return account, err
	}
	var adapterSyncErr error
	if syncCapable, ok := adapter.(LiveAccountSyncAdapter); ok {
		if synced, syncErr := syncCapable.SyncAccountSnapshot(p, account, binding); syncErr == nil {
			if healthOwner, ok := adapter.(liveAccountSyncSuccessOwner); !ok || !healthOwner.PersistsLiveAccountSyncSuccess() {
				synced, syncErr = p.persistLiveAccountSyncSuccess(synced, binding, previousSuccessAt)
				if syncErr != nil {
					logger.Warn("persist adapter live account sync success failed", "error", syncErr)
					return synced, syncErr
				}
			}
			p.syncLiveSessionsForAccountSnapshot(synced)
			logger.Info("live account synced via adapter", "exchange", synced.Exchange, "status", synced.Status)
			return synced, nil
		} else {
			logger.Warn("adapter live account sync failed, falling back to local state", "error", syncErr)
			adapterSyncErr = syncErr
		}
	}
	synced, fallbackErr := p.syncLiveAccountFromLocalState(account, binding)
	if fallbackErr == nil {
		p.syncLiveSessionsForAccountSnapshot(synced)
		logger.Debug("live account synced from local state", "status", synced.Status)
		return synced, nil
	}
	logger.Warn("local-state live account sync failed", "error", fallbackErr)
	if adapterSyncErr != nil {
		fallbackErr = fmt.Errorf("adapter sync failed: %v; local fallback failed: %w", adapterSyncErr, fallbackErr)
	}
	account = p.persistLiveAccountSyncFailure(account, attemptedAt, fallbackErr)
	return account, fallbackErr
}

func (p *Platform) ReconcileLiveAccount(accountID string, options LiveAccountReconcileOptions) (LiveAccountReconcileResult, error) {
	options = normalizeLiveAccountReconcileOptions(options)
	result := LiveAccountReconcileResult{
		LookbackHours: options.LookbackHours,
	}

	account, err := p.SyncLiveAccount(accountID)
	if err != nil {
		return result, err
	}
	adapter, binding, err := p.resolveLiveAdapterForAccount(account)
	if err != nil {
		return result, err
	}
	reconcileAdapter, ok := adapter.(LiveAccountReconcileAdapter)
	if !ok {
		return result, fmt.Errorf("live adapter %s does not support account reconcile", normalizeLiveAdapterKey(stringValue(binding["adapterKey"])))
	}

	result.AdapterKey = normalizeLiveAdapterKey(stringValue(binding["adapterKey"]))
	result.ExecutionMode = normalizeLiveExecutionMode(binding["executionMode"], boolValue(binding["sandbox"]))
	if !strings.EqualFold(result.ExecutionMode, "rest") {
		return result, fmt.Errorf("live account reconcile requires executionMode=rest, got %s", firstNonEmpty(result.ExecutionMode, "unknown"))
	}

	symbols, err := p.collectLiveAccountReconcileSymbols(account, options.LookbackHours)
	if err != nil {
		return result, err
	}
	result.Symbols = symbols
	result.SymbolCount = len(symbols)
	if len(symbols) == 0 {
		result.Notes = []string{"no candidate symbols available for reconcile"}
		account, err = p.persistLiveAccountReconcileSummary(account, result)
		if err != nil {
			return result, err
		}
		result.Account = account
		return result, nil
	}

	orders, err := p.store.ListOrders()
	if err != nil {
		return result, err
	}
	index := buildLiveOrderReconcileIndex(orders, account.ID)

	for _, symbol := range symbols {
		exchangeOrders, err := reconcileAdapter.FetchRecentOrders(account, binding, symbol, options.LookbackHours)
		if err != nil {
			return result, fmt.Errorf("reconcile fetch recent orders for %s failed: %w", symbol, err)
		}
		tradeReports, err := reconcileAdapter.FetchRecentTrades(account, binding, symbol, options.LookbackHours)
		if err != nil {
			return result, fmt.Errorf("reconcile fetch recent trades for %s failed: %w", symbol, err)
		}
		tradesByExchangeOrderID := groupTradeReportsByExchangeOrderID(tradeReports)
		sort.Slice(exchangeOrders, func(i, j int) bool {
			return parseFloatValue(exchangeOrders[i]["updateTime"]) < parseFloatValue(exchangeOrders[j]["updateTime"])
		})
		for _, payload := range exchangeOrders {
			exchangeOrderID := normalizeBinanceOrderID(payload["orderId"], payload["clientOrderId"])
			if exchangeOrderID == "" {
				continue
			}
			reconciledOrder, created, err := p.reconcileLiveAccountExchangeOrder(account, binding, payload, tradesByExchangeOrderID[exchangeOrderID], &index)
			if err != nil {
				return result, err
			}
			if reconciledOrder.ID == "" {
				continue
			}
			result.OrderCount++
			if created {
				result.CreatedOrderCount++
			} else {
				result.UpdatedOrderCount++
			}
		}
	}

	account, err = p.store.GetAccount(accountID)
	if err != nil {
		return result, err
	}
	account, err = p.persistLiveAccountReconcileSummary(account, result)
	if err != nil {
		return result, err
	}
	result.Account = account
	return result, nil
}

func (p *Platform) persistLiveAccountSyncFailure(account domain.Account, attemptedAt time.Time, err error) domain.Account {
	if err == nil {
		return account
	}
	updateAccountSyncFailureHealth(&account, attemptedAt, err)
	updated, updateErr := p.store.UpdateAccount(account)
	if updateErr != nil {
		p.logger("service.live", "account_id", account.ID).Warn("persist live account sync failure health failed", "error", updateErr)
		return account
	}
	return updated
}

func (p *Platform) persistLiveAccountSyncSuccess(account domain.Account, binding map[string]any, previousSuccessAt time.Time) (domain.Account, error) {
	account.Metadata = cloneMetadata(account.Metadata)
	snapshot := cloneMetadata(mapValue(account.Metadata["liveSyncSnapshot"]))
	syncedAt := parseOptionalRFC3339(stringValue(account.Metadata["lastLiveSyncAt"]))
	if syncedAt.IsZero() {
		syncedAt = parseOptionalRFC3339(stringValue(snapshot["syncedAt"]))
	}
	if syncedAt.IsZero() {
		syncedAt = time.Now().UTC()
	}

	snapshot["source"] = firstNonEmpty(stringValue(snapshot["source"]), "live-account-adapter")
	snapshot["adapterKey"] = firstNonEmpty(
		normalizeLiveAdapterKey(stringValue(snapshot["adapterKey"])),
		normalizeLiveAdapterKey(stringValue(binding["adapterKey"])),
	)
	snapshot["syncedAt"] = syncedAt.Format(time.RFC3339)
	snapshot["syncStatus"] = firstNonEmpty(stringValue(snapshot["syncStatus"]), "SYNCED")
	snapshot["accountExchange"] = firstNonEmpty(stringValue(snapshot["accountExchange"]), account.Exchange)
	snapshot["bindingMode"] = firstNonEmpty(stringValue(snapshot["bindingMode"]), stringValue(binding["connectionMode"]))
	snapshot["executionMode"] = firstNonEmpty(
		stringValue(snapshot["executionMode"]),
		normalizeLiveExecutionMode(binding["executionMode"], boolValue(binding["sandbox"])),
	)
	snapshot["feeSource"] = firstNonEmpty(stringValue(snapshot["feeSource"]), "exchange")
	snapshot["fundingSource"] = firstNonEmpty(stringValue(snapshot["fundingSource"]), "exchange")

	account.Metadata["liveSyncSnapshot"] = snapshot
	account.Metadata["lastLiveSyncAt"] = syncedAt.Format(time.RFC3339)
	updateAccountSyncSuccessHealth(&account, syncedAt, previousSuccessAt)
	return p.store.UpdateAccount(account)
}

func normalizeLiveAccountReconcileOptions(options LiveAccountReconcileOptions) LiveAccountReconcileOptions {
	if options.LookbackHours <= 0 {
		options.LookbackHours = 24
	}
	if options.LookbackHours > 24*7 {
		options.LookbackHours = 24 * 7
	}
	return options
}

func (p *Platform) collectLiveAccountReconcileSymbols(account domain.Account, lookbackHours int) ([]string, error) {
	symbolSet := make(map[string]struct{})
	cutoff := time.Now().UTC().Add(-time.Duration(maxInt(lookbackHours, 1)) * time.Hour)

	orders, err := p.store.ListOrders()
	if err != nil {
		return nil, err
	}
	for _, order := range orders {
		if order.AccountID != account.ID {
			continue
		}
		status := strings.ToUpper(strings.TrimSpace(order.Status))
		if order.CreatedAt.After(cutoff) || !isTerminalOrderStatus(status) {
			addLiveAccountReconcileSymbol(symbolSet, order.Symbol)
		}
	}

	positions, err := p.store.ListPositions()
	if err != nil {
		return nil, err
	}
	for _, position := range positions {
		if position.AccountID != account.ID || position.Quantity <= 0 {
			continue
		}
		addLiveAccountReconcileSymbol(symbolSet, position.Symbol)
	}

	snapshot := cloneMetadata(mapValue(account.Metadata["liveSyncSnapshot"]))
	for _, item := range metadataList(snapshot["positions"]) {
		addLiveAccountReconcileSymbol(symbolSet, stringValue(item["symbol"]))
	}
	for _, item := range metadataList(snapshot["openOrders"]) {
		addLiveAccountReconcileSymbol(symbolSet, stringValue(item["symbol"]))
	}

	sessions, err := p.store.ListLiveSessions()
	if err != nil {
		return nil, err
	}
	for _, session := range sessions {
		if session.AccountID != account.ID {
			continue
		}
		addLiveAccountReconcileSymbol(symbolSet, stringValue(session.State["symbol"]))
	}

	symbols := make([]string, 0, len(symbolSet))
	for symbol := range symbolSet {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	return symbols, nil
}

func addLiveAccountReconcileSymbol(symbols map[string]struct{}, raw string) {
	symbol := NormalizeSymbol(raw)
	if symbol == "" {
		return
	}
	symbols[symbol] = struct{}{}
}

func buildLiveOrderReconcileIndex(orders []domain.Order, accountID string) liveOrderReconcileIndex {
	index := liveOrderReconcileIndex{
		byID:              make(map[string]domain.Order),
		byExchangeOrderID: make(map[string]domain.Order),
		byClientOrderID:   make(map[string]domain.Order),
	}
	for _, order := range orders {
		if order.AccountID != accountID {
			continue
		}
		index.put(order)
	}
	return index
}

func (i *liveOrderReconcileIndex) put(order domain.Order) {
	if order.ID == "" {
		return
	}
	i.byID[order.ID] = order
	i.byClientOrderID[order.ID] = order
	if exchangeOrderID := strings.TrimSpace(stringValue(order.Metadata["exchangeOrderId"])); exchangeOrderID != "" {
		i.byExchangeOrderID[exchangeOrderID] = order
	}
	if clientOrderID := strings.TrimSpace(stringValue(mapValue(order.Metadata["adapterSubmission"])["clientOrderId"])); clientOrderID != "" {
		i.byClientOrderID[clientOrderID] = order
	}
	if clientOrderID := strings.TrimSpace(stringValue(order.Metadata["exchangeClientOrderId"])); clientOrderID != "" {
		i.byClientOrderID[clientOrderID] = order
	}
}

func (i *liveOrderReconcileIndex) match(exchangeOrderID, clientOrderID string) (domain.Order, bool) {
	if exchangeOrderID != "" {
		if order, ok := i.byExchangeOrderID[exchangeOrderID]; ok {
			return order, true
		}
	}
	if clientOrderID != "" {
		if order, ok := i.byClientOrderID[clientOrderID]; ok {
			return order, true
		}
		if order, ok := i.byID[clientOrderID]; ok {
			return order, true
		}
	}
	return domain.Order{}, false
}

func groupTradeReportsByExchangeOrderID(reports []LiveFillReport) map[string][]LiveFillReport {
	grouped := make(map[string][]LiveFillReport)
	for _, report := range reports {
		exchangeOrderID := strings.TrimSpace(stringValue(mapValue(report.Metadata)["exchangeOrderId"]))
		if exchangeOrderID == "" {
			continue
		}
		grouped[exchangeOrderID] = append(grouped[exchangeOrderID], report)
	}
	return grouped
}

func (p *Platform) reconcileLiveAccountExchangeOrder(account domain.Account, binding map[string]any, payload map[string]any, tradeReports []LiveFillReport, index *liveOrderReconcileIndex) (domain.Order, bool, error) {
	exchangeOrderID := normalizeBinanceOrderID(payload["orderId"], payload["clientOrderId"])
	clientOrderID := strings.TrimSpace(stringValue(payload["clientOrderId"]))
	if exchangeOrderID == "" {
		return domain.Order{}, false, nil
	}
	order, found := index.match(exchangeOrderID, clientOrderID)
	created := false
	var err error
	if !found {
		order, err = p.createRecoveredLiveOrderFromExchange(account, binding, payload)
		if err != nil {
			return domain.Order{}, false, err
		}
		created = true
	}
	order, err = p.enrichLiveOrderFromExchangePayload(account.ID, order, binding, payload, created)
	if err != nil {
		return domain.Order{}, false, err
	}
	syncResult := buildLiveReconcileSyncResult(order, payload, tradeReports)
	if isTerminalOrderStatus(order.Status) && !isTerminalOrderStatus(syncResult.Status) {
		syncResult.Status = order.Status
	}
	updated, err := p.applyLiveSyncResult(account, order, syncResult)
	if err != nil {
		return domain.Order{}, false, err
	}
	index.put(updated)
	return updated, created, nil
}

func (p *Platform) createRecoveredLiveOrderFromExchange(account domain.Account, binding map[string]any, payload map[string]any) (domain.Order, error) {
	symbol := NormalizeSymbol(stringValue(payload["symbol"]))
	quantity := firstPositive(parseFloatValue(payload["origQty"]), parseFloatValue(payload["executedQty"]))
	price := firstPositive(parseFloatValue(payload["avgPrice"]), parseFloatValue(payload["price"]))
	base := domain.Order{
		AccountID:         account.ID,
		StrategyVersionID: p.inferReconcileStrategyVersionID(account.ID, symbol),
		Symbol:            symbol,
		Side:              strings.ToUpper(strings.TrimSpace(stringValue(payload["side"]))),
		Type:              strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(payload["origType"]), stringValue(payload["type"]), "MARKET"))),
		Quantity:          quantity,
		Price:             price,
		ReduceOnly:        boolValue(payload["reduceOnly"]),
		ClosePosition:     boolValue(payload["closePosition"]),
		Metadata: map[string]any{
			"source":             "live-account-reconcile",
			"reconcileRecovered": true,
			"executionMode":      "live",
			"adapterKey":         normalizeLiveAdapterKey(stringValue(binding["adapterKey"])),
			"feeSource":          "exchange",
			"fundingSource":      "exchange",
			"orderLifecycle": map[string]any{
				"submitted": true,
				"accepted":  true,
				"synced":    false,
				"filled":    false,
			},
		},
	}
	return p.store.CreateOrder(base)
}

func (p *Platform) enrichLiveOrderFromExchangePayload(accountID string, order domain.Order, binding map[string]any, payload map[string]any, recovered bool) (domain.Order, error) {
	status := firstNonEmpty(mapBinanceOrderStatus(stringValue(payload["status"])), order.Status, "ACCEPTED")
	exchangeOrderID := normalizeBinanceOrderID(payload["orderId"], payload["clientOrderId"])
	clientOrderID := strings.TrimSpace(stringValue(payload["clientOrderId"]))
	syncedAt := firstNonEmpty(parseBinanceMillisToRFC3339(payload["updateTime"]), parseBinanceMillisToRFC3339(payload["time"]), time.Now().UTC().Format(time.RFC3339))
	acceptedAt := firstNonEmpty(parseBinanceMillisToRFC3339(payload["time"]), syncedAt)

	order.Metadata = cloneMetadata(order.Metadata)
	applyExecutionMetadata(order.Metadata, map[string]any{
		"executionMode": "live",
		"adapterKey":    normalizeLiveAdapterKey(stringValue(binding["adapterKey"])),
		"feeSource":     "exchange",
		"fundingSource": "exchange",
	})
	if recovered {
		order.Metadata["source"] = "live-account-reconcile"
		order.Metadata["reconcileRecovered"] = true
	}
	order.Symbol = NormalizeSymbol(firstNonEmpty(stringValue(payload["symbol"]), order.Symbol))
	order.Side = strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(payload["side"]), order.Side)))
	order.Type = strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(payload["origType"]), stringValue(payload["type"]), order.Type)))
	order.Quantity = firstPositive(parseFloatValue(payload["origQty"]), firstPositive(parseFloatValue(payload["executedQty"]), order.Quantity))
	order.Price = firstPositive(parseFloatValue(payload["avgPrice"]), firstPositive(parseFloatValue(payload["price"]), order.Price))
	order.ReduceOnly = order.ReduceOnly || boolValue(payload["reduceOnly"])
	order.ClosePosition = order.ClosePosition || boolValue(payload["closePosition"])
	if strings.TrimSpace(order.StrategyVersionID) == "" {
		order.StrategyVersionID = p.inferReconcileStrategyVersionID(accountID, order.Symbol)
	}
	order.Metadata["exchangeOrderId"] = exchangeOrderID
	if clientOrderID != "" {
		order.Metadata["exchangeClientOrderId"] = clientOrderID
	}
	if strings.TrimSpace(stringValue(order.Metadata["acceptedAt"])) == "" && acceptedAt != "" {
		order.Metadata["acceptedAt"] = acceptedAt
	}
	order.Metadata["lastExchangeStatus"] = status
	order.Metadata["lastExchangeUpdateAt"] = syncedAt
	submission := cloneMetadata(mapValue(order.Metadata["adapterSubmission"]))
	if submission == nil {
		submission = map[string]any{}
	}
	submission["adapterMode"] = "rest-reconcile"
	submission["executionMode"] = normalizeLiveExecutionMode(binding["executionMode"], boolValue(binding["sandbox"]))
	submission["exchangeOrderId"] = exchangeOrderID
	submission["clientOrderId"] = clientOrderID
	submission["binanceStatus"] = stringValue(payload["status"])
	submission["origQty"] = parseFloatValue(payload["origQty"])
	submission["executedQty"] = parseFloatValue(payload["executedQty"])
	submission["price"] = parseFloatValue(payload["price"])
	submission["avgPrice"] = parseFloatValue(payload["avgPrice"])
	submission["timeInForce"] = stringValue(payload["timeInForce"])
	submission["updateTime"] = syncedAt
	order.Metadata["adapterSubmission"] = submission
	markOrderLifecycle(order.Metadata, "submitted", true)
	markOrderLifecycle(order.Metadata, "accepted", !strings.EqualFold(status, "REJECTED"))
	return order, nil
}

func buildLiveReconcileSyncResult(order domain.Order, payload map[string]any, tradeReports []LiveFillReport) LiveOrderSync {
	status := firstNonEmpty(mapBinanceOrderStatus(stringValue(payload["status"])), order.Status, "ACCEPTED")
	syncedAt := firstNonEmpty(parseBinanceMillisToRFC3339(payload["updateTime"]), parseBinanceMillisToRFC3339(payload["time"]), time.Now().UTC().Format(time.RFC3339))
	exchangeOrderID := normalizeBinanceOrderID(payload["orderId"], payload["clientOrderId"])
	clientOrderID := strings.TrimSpace(stringValue(payload["clientOrderId"]))
	terminal := isTerminalOrderStatus(status)
	filledQty := parseFloatValue(payload["executedQty"])
	avgPrice := firstPositive(parseFloatValue(payload["avgPrice"]), parseFloatValue(payload["price"]))
	fills := make([]LiveFillReport, 0, len(tradeReports))
	if terminal && len(tradeReports) > 0 {
		fills = append(fills, tradeReports...)
	} else if terminal && filledQty > 0 && strings.EqualFold(status, "FILLED") {
		fills = append(fills, LiveFillReport{
			Price:    avgPrice,
			Quantity: filledQty,
			Fee:      0,
			Metadata: map[string]any{
				"source":          "binance-all-orders",
				"exchangeOrderId": exchangeOrderID,
				"clientOrderId":   clientOrderID,
				"tradeTime":       syncedAt,
				"executionMode":   "rest",
			},
		})
	}
	return LiveOrderSync{
		Status:   status,
		SyncedAt: syncedAt,
		Fills:    fills,
		Metadata: map[string]any{
			"adapterMode":     "rest-reconcile",
			"executionMode":   "rest",
			"exchangeOrderId": exchangeOrderID,
			"clientOrderId":   clientOrderID,
			"binanceStatus":   stringValue(payload["status"]),
			"origQty":         parseFloatValue(payload["origQty"]),
			"executedQty":     filledQty,
			"avgPrice":        avgPrice,
			"price":           parseFloatValue(payload["price"]),
			"updateTime":      syncedAt,
		},
		Terminal:   terminal,
		FeeSource:  "exchange",
		FundingSrc: "exchange",
	}
}

func (p *Platform) inferReconcileStrategyVersionID(accountID, symbol string) string {
	if strings.TrimSpace(symbol) == "" {
		return ""
	}
	sessions, err := p.store.ListLiveSessions()
	if err != nil {
		return ""
	}
	var strategyID string
	for _, session := range sessions {
		if session.AccountID != accountID {
			continue
		}
		if NormalizeSymbol(stringValue(session.State["symbol"])) != NormalizeSymbol(symbol) {
			continue
		}
		if strategyID != "" && strategyID != session.StrategyID {
			return ""
		}
		strategyID = session.StrategyID
	}
	if strategyID == "" {
		return ""
	}
	version, err := p.resolveCurrentStrategyVersion(strategyID)
	if err != nil {
		return ""
	}
	return version.ID
}

func (p *Platform) persistLiveAccountReconcileSummary(account domain.Account, result LiveAccountReconcileResult) (domain.Account, error) {
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["lastLiveReconcileAt"] = time.Now().UTC().Format(time.RFC3339)
	account.Metadata["lastLiveReconcile"] = map[string]any{
		"adapterKey":        result.AdapterKey,
		"executionMode":     result.ExecutionMode,
		"lookbackHours":     result.LookbackHours,
		"symbolCount":       result.SymbolCount,
		"symbols":           result.Symbols,
		"orderCount":        result.OrderCount,
		"createdOrderCount": result.CreatedOrderCount,
		"updatedOrderCount": result.UpdatedOrderCount,
		"notes":             result.Notes,
	}
	return p.store.UpdateAccount(account)
}

func (p *Platform) syncLiveSessionsForAccountSnapshot(account domain.Account) {
	sessions, err := p.ListLiveSessions()
	if err != nil {
		return
	}
	for _, session := range sessions {
		if session.AccountID != account.ID {
			continue
		}
		_, _ = p.refreshLiveSessionPositionContext(session, time.Now().UTC(), "live-account-sync")
	}
}

func (p *Platform) RecoverLiveTradingOnStartup(ctx context.Context) {
	logger := p.logger("service.live")
	logger.Info("starting live trading recovery")
	accounts, err := p.ListAccounts()
	if err != nil {
		logger.Warn("list accounts failed during live recovery", "error", err)
		return
	}
	syncedAccounts := 0
	for _, account := range accounts {
		if ctx != nil {
			select {
			case <-ctx.Done():
				logger.Info("live trading recovery cancelled", "synced_account_count", syncedAccounts)
				return
			default:
			}
		}
		if !strings.EqualFold(account.Mode, "LIVE") {
			continue
		}
		syncedAccount, syncErr := p.SyncLiveAccount(account.ID)
		if syncErr == nil {
			account = syncedAccount
			syncedAccounts++
		} else {
			p.logger("service.live", "account_id", account.ID).Warn("live account sync failed during recovery", "error", syncErr)
		}
	}

	sessions, err := p.ListLiveSessions()
	if err != nil {
		logger.Warn("list live sessions failed during recovery", "error", err)
		return
	}
	recoveredSessions := 0
	for _, session := range sessions {
		if ctx != nil {
			select {
			case <-ctx.Done():
				logger.Info("live trading recovery cancelled",
					"synced_account_count", syncedAccounts,
					"recovered_session_count", recoveredSessions,
				)
				return
			default:
			}
		}
		if !strings.EqualFold(session.Status, "RUNNING") {
			continue
		}
		recovered, recoverErr := p.recoverRunningLiveSession(session)
		if recoverErr != nil {
			p.logger("service.live", "session_id", session.ID).Warn("recover running live session failed", "error", recoverErr)
			state := cloneMetadata(session.State)
			state["lastRecoveryError"] = recoverErr.Error()
			state["lastRecoveryAttemptAt"] = time.Now().UTC().Format(time.RFC3339)
			_, _ = p.store.UpdateLiveSessionState(session.ID, state)
			continue
		}
		recoveredSessions++
		state := cloneMetadata(recovered.State)
		delete(state, "lastRecoveryError")
		state["lastRecoveryAttemptAt"] = time.Now().UTC().Format(time.RFC3339)
		state["lastRecoveryStatus"] = "recovered"
		_, _ = p.store.UpdateLiveSessionState(recovered.ID, state)
	}
	logger.Info("live trading recovery completed",
		"synced_account_count", syncedAccounts,
		"recovered_session_count", recoveredSessions,
	)
}

func (p *Platform) syncLiveAccountFromLocalState(account domain.Account, binding map[string]any) (domain.Account, error) {
	previousSuccessAt := parseOptionalRFC3339(stringValue(account.Metadata["lastLiveSyncAt"]))
	orders, err := p.store.ListOrders()
	if err != nil {
		return domain.Account{}, err
	}
	fills, err := p.store.ListFills()
	if err != nil {
		return domain.Account{}, err
	}
	positions, err := p.store.ListPositions()
	if err != nil {
		return domain.Account{}, err
	}

	filteredOrders := make([]domain.Order, 0)
	orderByID := make(map[string]domain.Order)
	for _, order := range orders {
		if order.AccountID != account.ID {
			continue
		}
		filteredOrders = append(filteredOrders, order)
		orderByID[order.ID] = order
	}
	filteredFills := make([]domain.Fill, 0)
	for _, fill := range fills {
		if _, ok := orderByID[fill.OrderID]; ok {
			filteredFills = append(filteredFills, fill)
		}
	}
	filteredPositions := make([]domain.Position, 0)
	for _, position := range positions {
		if position.AccountID == account.ID {
			filteredPositions = append(filteredPositions, position)
		}
	}

	syncedAt := time.Now().UTC()
	openOrders := 0
	for _, order := range filteredOrders {
		status := strings.ToUpper(strings.TrimSpace(order.Status))
		if status != "FILLED" && status != "CANCELLED" && status != "REJECTED" {
			openOrders++
		}
	}

	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveSyncSnapshot"] = map[string]any{
		"source":          "platform-live-reconciliation",
		"adapterKey":      normalizeLiveAdapterKey(stringValue(binding["adapterKey"])),
		"syncedAt":        syncedAt.Format(time.RFC3339),
		"orderCount":      len(filteredOrders),
		"fillCount":       len(filteredFills),
		"positionCount":   len(filteredPositions),
		"openOrderCount":  openOrders,
		"latestOrder":     summarizeLiveAccountLatestOrder(filteredOrders),
		"latestFill":      summarizeLiveAccountLatestFill(filteredFills, orderByID),
		"positions":       summarizeLiveAccountPositions(filteredPositions),
		"bindingMode":     stringValue(binding["connectionMode"]),
		"executionMode":   normalizeLiveExecutionMode(binding["executionMode"], boolValue(binding["sandbox"])),
		"feeSource":       "exchange",
		"fundingSource":   "exchange",
		"syncStatus":      "SYNCED",
		"accountExchange": account.Exchange,
	}
	return p.persistLiveAccountSyncSuccess(account, binding, previousSuccessAt)
}

func (p *Platform) syncLiveAccountFromBinance(account domain.Account, binding map[string]any) (domain.Account, error) {
	previousSuccessAt := parseOptionalRFC3339(stringValue(account.Metadata["lastLiveSyncAt"]))
	resolved, err := resolveBinanceRESTCredentials(binding)
	if err != nil {
		return domain.Account{}, err
	}
	accountPayload, err := binanceSignedGET(resolved, "/fapi/v3/account", map[string]string{
		"timestamp":  fmt.Sprintf("%d", time.Now().UTC().UnixMilli()),
		"recvWindow": fmt.Sprintf("%d", maxIntValue(binding["recvWindowMs"], 5000)),
	})
	if err != nil {
		return domain.Account{}, fmt.Errorf("binance account sync failed: %w", err)
	}
	positionRiskPayload, err := binanceSignedGET(resolved, "/fapi/v2/positionRisk", map[string]string{
		"timestamp":  fmt.Sprintf("%d", time.Now().UTC().UnixMilli()),
		"recvWindow": fmt.Sprintf("%d", maxIntValue(binding["recvWindowMs"], 5000)),
	})
	if err != nil {
		return domain.Account{}, fmt.Errorf("binance position risk sync failed: %w", err)
	}
	openOrdersPayload, err := binanceSignedGET(resolved, "/fapi/v1/openOrders", map[string]string{
		"timestamp":  fmt.Sprintf("%d", time.Now().UTC().UnixMilli()),
		"recvWindow": fmt.Sprintf("%d", maxIntValue(binding["recvWindowMs"], 5000)),
	})
	if err != nil {
		return domain.Account{}, fmt.Errorf("binance open orders sync failed: %w", err)
	}

	var accountBody map[string]any
	if err := json.Unmarshal(accountPayload, &accountBody); err != nil {
		return domain.Account{}, err
	}
	var positionRiskBody []map[string]any
	if err := json.Unmarshal(positionRiskPayload, &positionRiskBody); err != nil {
		return domain.Account{}, err
	}
	var openOrdersBody []map[string]any
	if err := json.Unmarshal(openOrdersPayload, &openOrdersBody); err != nil {
		return domain.Account{}, err
	}

	positions := metadataList(accountBody["positions"])
	assets := metadataList(accountBody["assets"])
	positionRiskIndex := make(map[string]map[string]any, len(positionRiskBody))
	for _, item := range positionRiskBody {
		key := strings.ToUpper(strings.TrimSpace(stringValue(item["symbol"]))) + "|" + strings.ToUpper(strings.TrimSpace(stringValue(item["positionSide"])))
		positionRiskIndex[key] = item
	}

	openPositions := make([]map[string]any, 0)
	for _, item := range positions {
		positionAmt := parseFloatValue(item["positionAmt"])
		if positionAmt == 0 {
			continue
		}
		riskKey := strings.ToUpper(strings.TrimSpace(stringValue(item["symbol"]))) + "|" + strings.ToUpper(strings.TrimSpace(stringValue(item["positionSide"])))
		risk := positionRiskIndex[riskKey]
		openPositions = append(openPositions, map[string]any{
			"symbol":           stringValue(item["symbol"]),
			"positionAmt":      positionAmt,
			"entryPrice":       parseFloatValue(item["entryPrice"]),
			"markPrice":        firstPositive(parseFloatValue(risk["markPrice"]), parseFloatValue(item["markPrice"])),
			"unrealizedProfit": firstPositive(parseFloatValue(risk["unRealizedProfit"]), parseFloatValue(item["unrealizedProfit"])),
			"liquidationPrice": parseFloatValue(risk["liquidationPrice"]),
			"notional":         parseFloatValue(risk["notional"]),
			"isolatedMargin":   parseFloatValue(risk["isolatedMargin"]),
			"leverage":         firstNonEmpty(stringValue(risk["leverage"]), stringValue(item["leverage"])),
			"marginType":       firstNonEmpty(stringValue(risk["marginType"]), stringValue(item["marginType"])),
			"positionSide":     firstNonEmpty(stringValue(risk["positionSide"]), stringValue(item["positionSide"])),
			"breakEvenPrice":   parseFloatValue(risk["breakEvenPrice"]),
			"maxNotionalValue": parseFloatValue(risk["maxNotionalValue"]),
		})
	}
	assetSummaries := make([]map[string]any, 0)
	for _, item := range assets {
		walletBalance := parseFloatValue(item["walletBalance"])
		if walletBalance == 0 && parseFloatValue(item["availableBalance"]) == 0 {
			continue
		}
		assetSummaries = append(assetSummaries, map[string]any{
			"asset":              stringValue(item["asset"]),
			"walletBalance":      walletBalance,
			"availableBalance":   parseFloatValue(item["availableBalance"]),
			"crossWalletBalance": parseFloatValue(item["crossWalletBalance"]),
			"crossUnPnl":         parseFloatValue(item["crossUnPnl"]),
		})
	}
	openOrders := make([]map[string]any, 0, len(openOrdersBody))
	for _, item := range openOrdersBody {
		openOrders = append(openOrders, map[string]any{
			"symbol":        stringValue(item["symbol"]),
			"orderId":       stringValue(item["orderId"]),
			"clientOrderId": stringValue(item["clientOrderId"]),
			"status":        mapBinanceOrderStatus(stringValue(item["status"])),
			"side":          stringValue(item["side"]),
			"type":          stringValue(item["type"]),
			"origType":      stringValue(item["origType"]),
			"origQty":       parseFloatValue(item["origQty"]),
			"executedQty":   parseFloatValue(item["executedQty"]),
			"price":         parseFloatValue(item["price"]),
			"avgPrice":      parseFloatValue(item["avgPrice"]),
			"stopPrice":     parseFloatValue(item["stopPrice"]),
			"workingType":   stringValue(item["workingType"]),
			"positionSide":  stringValue(item["positionSide"]),
			"reduceOnly":    item["reduceOnly"],
			"closePosition": item["closePosition"],
			"timeInForce":   stringValue(item["timeInForce"]),
			"updateTime":    parseBinanceMillisToRFC3339(item["updateTime"]),
		})
	}
	syncedAt := time.Now().UTC()
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveSyncSnapshot"] = map[string]any{
		"source":                "binance-rest-account-v3",
		"adapterKey":            normalizeLiveAdapterKey(stringValue(binding["adapterKey"])),
		"syncedAt":              syncedAt.Format(time.RFC3339),
		"bindingMode":           stringValue(binding["connectionMode"]),
		"executionMode":         "rest",
		"accountExchange":       account.Exchange,
		"feeSource":             "exchange",
		"fundingSource":         "exchange",
		"syncStatus":            "SYNCED",
		"feeTier":               accountBody["feeTier"],
		"canTrade":              accountBody["canTrade"],
		"canDeposit":            accountBody["canDeposit"],
		"canWithdraw":           accountBody["canWithdraw"],
		"totalWalletBalance":    parseFloatValue(accountBody["totalWalletBalance"]),
		"totalUnrealizedProfit": parseFloatValue(accountBody["totalUnrealizedProfit"]),
		"totalMarginBalance":    parseFloatValue(accountBody["totalMarginBalance"]),
		"availableBalance":      parseFloatValue(accountBody["availableBalance"]),
		"maxWithdrawAmount":     parseFloatValue(accountBody["maxWithdrawAmount"]),
		"positionCount":         len(openPositions),
		"positions":             openPositions,
		"assets":                assetSummaries,
		"openOrderCount":        len(openOrders),
		"openOrders":            openOrders,
		"apiKeyRef":             resolved.APIKeyRef,
		"restBaseUrl":           resolved.BaseURL,
	}
	account, err = p.persistLiveAccountSyncSuccess(account, binding, previousSuccessAt)
	if err != nil {
		return domain.Account{}, err
	}
	if reconcileErr := p.reconcileLiveAccountPositions(account, openPositions); reconcileErr != nil {
		account.Metadata = cloneMetadata(account.Metadata)
		account.Metadata["lastLivePositionSyncError"] = reconcileErr.Error()
		account, _ = p.store.UpdateAccount(account)
		return account, reconcileErr
	}
	account.Metadata = cloneMetadata(account.Metadata)
	delete(account.Metadata, "lastLivePositionSyncError")
	account.Metadata["lastLivePositionSyncAt"] = time.Now().UTC().Format(time.RFC3339)
	return p.store.UpdateAccount(account)
}

func (p *Platform) CreateLiveSession(accountID, strategyID string, overrides map[string]any) (domain.LiveSession, error) {
	logger := p.logger("service.live", "account_id", accountID, "strategy_id", strategyID)
	account, err := p.store.GetAccount(accountID)
	if err != nil {
		logger.Warn("load account for live session failed", "error", err)
		return domain.LiveSession{}, err
	}
	if !strings.EqualFold(account.Mode, "LIVE") {
		return domain.LiveSession{}, fmt.Errorf("live session requires a LIVE account: %s", accountID)
	}

	session, err := p.store.CreateLiveSession(accountID, strategyID)
	if err != nil {
		logger.Error("create live session failed", "error", err)
		return domain.LiveSession{}, err
	}
	if len(overrides) > 0 {
		state := cloneMetadata(session.State)
		for key, value := range normalizeLiveSessionOverrides(overrides) {
			state[key] = value
		}
		session, err = p.store.UpdateLiveSessionState(session.ID, state)
		if err != nil {
			p.logger("service.live", "session_id", session.ID).Error("apply live session overrides failed", "error", err)
			return domain.LiveSession{}, err
		}
	}
	session, err = p.syncLiveSessionRuntime(session)
	if err != nil {
		p.logger("service.live", "session_id", session.ID).Warn("sync live session runtime failed", "error", err)
		return domain.LiveSession{}, err
	}
	p.logger("service.live",
		"session_id", session.ID,
		"account_id", session.AccountID,
		"strategy_id", session.StrategyID,
	).Info("live session created", "override_count", len(overrides))
	return session, nil
}

func (p *Platform) LaunchLiveFlow(accountID string, options LiveLaunchOptions) (LiveLaunchResult, error) {
	logger := p.logger("service.live", "account_id", accountID)
	account, err := p.store.GetAccount(accountID)
	if err != nil {
		logger.Warn("load account for live launch failed", "error", err)
		return LiveLaunchResult{}, err
	}
	if !strings.EqualFold(account.Mode, "LIVE") {
		return LiveLaunchResult{}, fmt.Errorf("account %s is not a LIVE account", accountID)
	}
	strategyID := strings.TrimSpace(options.StrategyID)
	if strategyID == "" {
		return LiveLaunchResult{}, fmt.Errorf("strategyId is required")
	}
	if !options.MirrorStrategySignals {
		options.MirrorStrategySignals = true
	}
	if !options.StartRuntime {
		options.StartRuntime = true
	}
	if !options.StartSession {
		options.StartSession = true
	}
	templateContext := liveLaunchTemplateContextFromLaunchOptions(options)
	logger.Info("launching live flow",
		"strategy_id", strings.TrimSpace(options.StrategyID),
		"mirror_strategy_signals", options.MirrorStrategySignals,
		"start_runtime", options.StartRuntime,
		"start_session", options.StartSession,
		"has_binding", len(options.Binding) > 0,
		"has_template_bindings", len(options.StrategySignalBindings) > 0,
		"has_overrides", len(options.LiveSessionOverrides) > 0,
	)

	result := LiveLaunchResult{Account: account}

	if account.Status != "CONFIGURED" && account.Status != "READY" {
		if len(options.Binding) == 0 {
			return LiveLaunchResult{}, fmt.Errorf("live account %s requires binding before launch", account.ID)
		}
		account, err = p.BindLiveAccount(account.ID, options.Binding)
		if err != nil {
			return LiveLaunchResult{}, err
		}
		result.Account = account
		result.AccountBindingApplied = true
	}

	if len(options.StrategySignalBindings) > 0 {
		// Launch templates are exclusive within the current account+strategy:
		// we quiesce the runtime, stop non-target RUNNING live sessions in that
		// same scope, replace bindings, then rebuild runtime state from the new
		// template. We do not hot-swap subscriptions under a still-running runtime.
		if err := p.ensureNoActivePositionsOrOrders(account.ID, strategyID); err != nil {
			return LiveLaunchResult{}, fmt.Errorf("launch template switch blocked by active positions or orders: %w", err)
		}
		if runtimeSession, found := p.findLiveRuntimeSession(account.ID, strategyID); found {
			if strings.EqualFold(runtimeSession.Status, "RUNNING") {
				if _, err := p.StopSignalRuntimeSession(runtimeSession.ID); err != nil {
					return LiveLaunchResult{}, fmt.Errorf("stop existing signal runtime before template switch failed: %w", err)
				}
			}
		}
		stoppedLiveSessions, err := p.stopConflictingLaunchLiveSessions(account.ID, strategyID, templateContext.Symbol, templateContext.SignalTimeframe)
		if err != nil {
			return LiveLaunchResult{}, fmt.Errorf("stop conflicting live sessions before template switch failed: %w", err)
		}
		if _, err := p.replaceStrategySignalSources(strategyID, options.StrategySignalBindings); err != nil {
			return LiveLaunchResult{}, fmt.Errorf("apply launch template bindings failed: %w", err)
		}
		result.TemplateApplied = true
		result.TemplateBindingCount = len(options.StrategySignalBindings)
		result.StoppedLiveSessions = stoppedLiveSessions
		if runtimeSession, found := p.findLiveRuntimeSession(account.ID, strategyID); found {
			runtimeSession, err = p.syncSignalRuntimeSessionPlan(runtimeSession.ID)
			if err != nil {
				return LiveLaunchResult{}, fmt.Errorf("refresh signal runtime plan after template switch failed: %w", err)
			}
			result.RuntimeSession = runtimeSession
			result.RuntimePlanRefreshed = true
		}
	}

	if options.MirrorStrategySignals {
		// Compatibility gate only: strategy bindings are now the runtime source of truth.
		// We no longer mirror them onto account signal bindings during launch.
		strategyBindings, err := p.ListStrategySignalBindings(strategyID)
		if err != nil {
			return LiveLaunchResult{}, err
		}
		if len(strategyBindings) == 0 {
			return LiveLaunchResult{}, fmt.Errorf("strategy %s has no signal bindings", strategyID)
		}
	}

	runtimeSession, runtimeCreated, err := p.ensureLaunchRuntimeSession(account.ID, strategyID)
	if err != nil {
		return LiveLaunchResult{}, err
	}
	result.RuntimeSession = runtimeSession
	result.RuntimeSessionCreated = runtimeCreated
	if options.StartRuntime && !strings.EqualFold(runtimeSession.Status, "RUNNING") {
		runtimeSession, err = p.StartSignalRuntimeSession(runtimeSession.ID)
		if err != nil {
			return LiveLaunchResult{}, err
		}
		result.RuntimeSession = runtimeSession
		result.RuntimeSessionStarted = true
	}

	liveSession, liveCreated, err := p.ensureLaunchLiveSession(account.ID, strategyID, options.LiveSessionOverrides)
	if err != nil {
		return LiveLaunchResult{}, err
	}
	result.LiveSession = liveSession
	result.LiveSessionCreated = liveCreated
	if options.StartSession && !strings.EqualFold(liveSession.Status, "RUNNING") {
		liveSession, err = p.StartLiveSession(liveSession.ID)
		if err != nil {
			return LiveLaunchResult{}, err
		}
		result.LiveSession = liveSession
		result.LiveSessionStarted = true
	}
	if templateContext.hasMetadata() {
		if updatedRuntime, updateErr := p.updateSignalRuntimeLaunchTemplateContext(result.RuntimeSession.ID, templateContext); updateErr == nil && updatedRuntime.ID != "" {
			result.RuntimeSession = updatedRuntime
		}
		if updatedLive, updateErr := p.updateLiveSessionLaunchTemplateContext(result.LiveSession.ID, templateContext); updateErr == nil && updatedLive.ID != "" {
			result.LiveSession = updatedLive
		}
	}

	account, err = p.store.GetAccount(account.ID)
	if err == nil {
		result.Account = account
	}
	logger.Info("live flow launched",
		"strategy_id", strategyID,
		"mirrored_binding_count", result.MirroredBindingCount,
		"account_binding_applied", result.AccountBindingApplied,
		"template_applied", result.TemplateApplied,
		"template_binding_count", result.TemplateBindingCount,
		"runtime_plan_refreshed", result.RuntimePlanRefreshed,
		"stopped_live_sessions", result.StoppedLiveSessions,
		"runtime_session_created", result.RuntimeSessionCreated,
		"runtime_session_started", result.RuntimeSessionStarted,
		"live_session_created", result.LiveSessionCreated,
		"live_session_started", result.LiveSessionStarted,
	)
	return result, nil
}

type liveLaunchTemplateContext struct {
	Key             string
	Name            string
	Symbol          string
	SignalTimeframe string
}

func liveLaunchTemplateContextFromLaunchOptions(options LiveLaunchOptions) liveLaunchTemplateContext {
	context := liveLaunchTemplateContext{
		Key:             strings.TrimSpace(options.LaunchTemplateKey),
		Name:            strings.TrimSpace(options.LaunchTemplateName),
		Symbol:          NormalizeSymbol(stringValue(options.LiveSessionOverrides["symbol"])),
		SignalTimeframe: normalizeSignalBarInterval(stringValue(options.LiveSessionOverrides["signalTimeframe"])),
	}
	if context.Symbol != "" && context.SignalTimeframe != "" {
		return context
	}
	for _, binding := range options.StrategySignalBindings {
		symbol := NormalizeSymbol(stringValue(binding["symbol"]))
		timeframe := signalBindingTimeframe(stringValue(binding["sourceKey"]), metadataValue(binding["options"]))
		if context.Symbol == "" && symbol != "" {
			context.Symbol = symbol
		}
		if context.SignalTimeframe == "" && timeframe != "" {
			context.SignalTimeframe = timeframe
		}
		if context.Symbol != "" && context.SignalTimeframe != "" {
			break
		}
	}
	return context
}

func (c liveLaunchTemplateContext) hasMetadata() bool {
	return strings.TrimSpace(c.Key) != "" || strings.TrimSpace(c.Name) != "" || c.Symbol != "" || c.SignalTimeframe != ""
}

func (p *Platform) findLiveRuntimeSession(accountID, strategyID string) (domain.SignalRuntimeSession, bool) {
	var fallback domain.SignalRuntimeSession
	found := false
	for _, session := range p.ListSignalRuntimeSessions() {
		if session.AccountID != accountID || session.StrategyID != strategyID {
			continue
		}
		if !found {
			fallback = session
			found = true
		}
		if strings.EqualFold(session.Status, "RUNNING") {
			return session, true
		}
	}
	if found {
		return fallback, true
	}
	return domain.SignalRuntimeSession{}, false
}

// stopConflictingLaunchLiveSessions enforces the current "template exclusive"
// boundary: within the same account+strategy, any RUNNING live session whose
// symbol/timeframe does not match the target template scope is stopped before
// bindings are replaced. Other accounts and strategies are intentionally
// untouched so the blast radius stays inside the launch target.
func (p *Platform) stopConflictingLaunchLiveSessions(accountID, strategyID, targetSymbol, targetTimeframe string) (int, error) {
	sessions, err := p.ListLiveSessions()
	if err != nil {
		return 0, err
	}
	stopped := 0
	now := time.Now().UTC()
	for _, session := range sessions {
		if session.AccountID != accountID || session.StrategyID != strategyID || !strings.EqualFold(session.Status, "RUNNING") {
			continue
		}
		if liveSessionMatchesLaunchScope(session, targetSymbol, targetTimeframe) {
			continue
		}
		updated, err := p.store.UpdateLiveSessionStatus(session.ID, "STOPPED")
		if err != nil {
			return stopped, err
		}
		state := cloneMetadata(updated.State)
		state["signalRuntimeStatus"] = "STOPPED"
		state["lastTemplateSwitchAt"] = now.Format(time.RFC3339)
		state["lastTemplateSwitchReason"] = "launch-template-switch"
		if _, err := p.store.UpdateLiveSessionState(updated.ID, state); err != nil {
			return stopped, err
		}
		p.mu.Lock()
		delete(p.livePlans, updated.ID)
		p.mu.Unlock()
		stopped++
	}
	return stopped, nil
}

func liveSessionMatchesLaunchScope(session domain.LiveSession, targetSymbol, targetTimeframe string) bool {
	if targetSymbol == "" && targetTimeframe == "" {
		return false
	}
	sessionSymbol := NormalizeSymbol(firstNonEmpty(stringValue(session.State["symbol"]), stringValue(session.State["lastSymbol"])))
	if targetSymbol != "" && sessionSymbol != targetSymbol {
		return false
	}
	sessionTimeframe := normalizeSignalBarInterval(firstNonEmpty(stringValue(session.State["signalTimeframe"]), stringValue(session.State["timeframe"])))
	if targetTimeframe != "" && sessionTimeframe != targetTimeframe {
		return false
	}
	return true
}

func applyLaunchTemplateContext(state map[string]any, context liveLaunchTemplateContext) {
	if !context.hasMetadata() {
		return
	}
	if strings.TrimSpace(context.Key) != "" {
		state["launchTemplateKey"] = context.Key
	}
	if strings.TrimSpace(context.Name) != "" {
		state["launchTemplateName"] = context.Name
	}
	if context.Symbol != "" {
		state["launchTemplateSymbol"] = context.Symbol
	}
	if context.SignalTimeframe != "" {
		state["launchTemplateTimeframe"] = context.SignalTimeframe
	}
	state["launchTemplateAppliedAt"] = time.Now().UTC().Format(time.RFC3339)
}

func (p *Platform) updateSignalRuntimeLaunchTemplateContext(sessionID string, context liveLaunchTemplateContext) (domain.SignalRuntimeSession, error) {
	if strings.TrimSpace(sessionID) == "" || !context.hasMetadata() {
		return domain.SignalRuntimeSession{}, nil
	}
	if err := p.updateSignalRuntimeSessionState(sessionID, func(session *domain.SignalRuntimeSession) {
		state := cloneMetadata(session.State)
		applyLaunchTemplateContext(state, context)
		session.State = state
		session.UpdatedAt = time.Now().UTC()
	}); err != nil {
		return domain.SignalRuntimeSession{}, err
	}
	return p.GetSignalRuntimeSession(sessionID)
}

func (p *Platform) updateLiveSessionLaunchTemplateContext(sessionID string, context liveLaunchTemplateContext) (domain.LiveSession, error) {
	if strings.TrimSpace(sessionID) == "" || !context.hasMetadata() {
		return domain.LiveSession{}, nil
	}
	session, err := p.store.GetLiveSession(sessionID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	state := cloneMetadata(session.State)
	applyLaunchTemplateContext(state, context)
	return p.store.UpdateLiveSessionState(sessionID, state)
}

func (p *Platform) ensureLaunchRuntimeSession(accountID, strategyID string) (domain.SignalRuntimeSession, bool, error) {
	for _, session := range p.ListSignalRuntimeSessions() {
		if session.AccountID == accountID && session.StrategyID == strategyID {
			return session, false, nil
		}
	}
	session, err := p.CreateSignalRuntimeSession(accountID, strategyID)
	return session, true, err
}

func (p *Platform) ensureLaunchLiveSession(accountID, strategyID string, overrides map[string]any) (domain.LiveSession, bool, error) {
	normalizedOverrides := normalizeLiveSessionOverrides(overrides)
	targetSymbol := NormalizeSymbol(stringValue(normalizedOverrides["symbol"]))
	targetTimeframe := normalizeSignalBarInterval(stringValue(normalizedOverrides["signalTimeframe"]))
	sessions, err := p.ListLiveSessions()
	if err != nil {
		return domain.LiveSession{}, false, err
	}
	for _, session := range sessions {
		if session.AccountID != accountID || session.StrategyID != strategyID {
			continue
		}
		if targetSymbol != "" && NormalizeSymbol(stringValue(session.State["symbol"])) != targetSymbol {
			continue
		}
		if targetTimeframe != "" && normalizeSignalBarInterval(stringValue(session.State["signalTimeframe"])) != targetTimeframe {
			continue
		}
		if len(normalizedOverrides) == 0 {
			return session, false, nil
		}
		state := cloneMetadata(session.State)
		for key, value := range normalizedOverrides {
			state[key] = value
		}
		updated, err := p.store.UpdateLiveSessionState(session.ID, state)
		if err != nil {
			return domain.LiveSession{}, false, err
		}
		synced, err := p.syncLiveSessionRuntime(updated)
		return synced, false, err
	}
	session, err := p.CreateLiveSession(accountID, strategyID, normalizedOverrides)
	return session, true, err
}

func (p *Platform) StartLiveSession(sessionID string) (domain.LiveSession, error) {
	logger := p.logger("service.live", "session_id", sessionID)
	session, err := p.store.GetLiveSession(sessionID)
	if err != nil {
		logger.Warn("load live session failed", "error", err)
		return domain.LiveSession{}, err
	}
	account, err := p.store.GetAccount(session.AccountID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	if !strings.EqualFold(account.Mode, "LIVE") {
		return domain.LiveSession{}, fmt.Errorf("live session %s is not bound to a LIVE account", session.ID)
	}
	if account.Status != "CONFIGURED" && account.Status != "READY" {
		return domain.LiveSession{}, fmt.Errorf("live account %s is not configured", account.ID)
	}
	if _, _, err := p.resolveLiveAdapterForAccount(account); err != nil {
		logger.Warn("resolve live adapter failed", "error", err)
		return domain.LiveSession{}, err
	}

	session, err = p.syncLiveSessionRuntime(session)
	if err != nil {
		logger.Warn("sync live session runtime failed", "error", err)
		return domain.LiveSession{}, err
	}
	session, err = p.ensureLiveSessionSignalRuntimeStarted(session)
	if err != nil {
		logger.Warn("ensure live signal runtime failed", "error", err)
		return domain.LiveSession{}, err
	}
	session, err = p.store.UpdateLiveSessionStatus(sessionID, "RUNNING")
	if err != nil {
		logger.Error("mark live session running failed", "error", err)
		return domain.LiveSession{}, err
	}
	p.logger("service.live",
		"session_id", session.ID,
		"account_id", session.AccountID,
		"strategy_id", session.StrategyID,
	).Info("live session started")
	return session, nil
}

func (p *Platform) StartLiveSyncDispatcher(ctx context.Context) {
	if ctx == nil {
		return
	}
	logger := p.logger("service.live_sync_dispatcher")
	logger.Info("live sync dispatcher started")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("live sync dispatcher stopped")
			return
		case <-ticker.C:
			now := time.Now().UTC()
			if err := p.syncActiveLiveSessions(now); err != nil {
				logger.Warn("sync active live sessions failed", "error", err)
			}
			if err := p.syncActiveLiveAccounts(now); err != nil {
				logger.Warn("sync active live accounts failed", "error", err)
			}
		}
	}
}

func (p *Platform) syncActiveLiveAccounts(eventTime time.Time) error {
	sessions, err := p.ListLiveSessions()
	if err != nil {
		return err
	}
	seen := make(map[string]struct{})
	var syncErrs []error
	for _, session := range sessions {
		if !strings.EqualFold(session.Status, "RUNNING") {
			continue
		}
		if _, ok := seen[session.AccountID]; ok {
			continue
		}
		seen[session.AccountID] = struct{}{}
		account, accountErr := p.store.GetAccount(session.AccountID)
		if accountErr != nil {
			continue
		}
		if !p.shouldRefreshLiveAccountSync(account, eventTime) {
			continue
		}
		if _, syncErr := p.SyncLiveAccount(account.ID); syncErr != nil {
			syncErrs = append(syncErrs, fmt.Errorf("live account %s sync failed: %w", account.ID, syncErr))
		}
	}
	return errors.Join(syncErrs...)
}

func (p *Platform) shouldRefreshLiveAccountSync(account domain.Account, eventTime time.Time) bool {
	threshold := time.Duration(p.runtimePolicy.LiveAccountSyncFreshnessSecs) * time.Second
	if threshold <= 0 {
		return false
	}
	lastSyncActivityAt := parseOptionalRFC3339(stringValue(account.Metadata["lastLiveSyncAt"]))
	accountSync := mapValue(mapValue(account.Metadata["healthSummary"])["accountSync"])
	if attemptedAt := parseOptionalRFC3339(stringValue(accountSync["lastAttemptAt"])); attemptedAt.After(lastSyncActivityAt) {
		lastSyncActivityAt = attemptedAt
	}
	if lastSyncActivityAt.IsZero() {
		return true
	}
	return eventTime.Sub(lastSyncActivityAt) >= threshold
}

func (p *Platform) recoverRunningLiveSession(session domain.LiveSession) (domain.LiveSession, error) {
	account, err := p.store.GetAccount(session.AccountID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	if _, syncErr := p.SyncLiveAccount(account.ID); syncErr != nil {
		// Keep recovery moving so runtime monitoring can still come back.
	}
	session, err = p.syncLiveSessionRuntime(session)
	if err != nil {
		return domain.LiveSession{}, err
	}
	session, err = p.ensureLiveSessionSignalRuntimeStarted(session)
	if err != nil {
		return domain.LiveSession{}, err
	}
	if strings.TrimSpace(stringValue(session.State["lastDispatchedOrderId"])) != "" {
		session, _ = p.syncLatestLiveSessionOrder(session, time.Now().UTC())
	}
	session, _ = p.refreshLiveSessionPositionContext(session, time.Now().UTC(), "live-startup-recovery")
	return p.store.UpdateLiveSessionStatus(session.ID, "RUNNING")
}

func (p *Platform) reconcileLiveAccountPositions(account domain.Account, exchangePositions []map[string]any) error {
	existing, err := p.store.ListPositions()
	if err != nil {
		return err
	}
	existingBySymbol := make(map[string]domain.Position)
	for _, position := range existing {
		if position.AccountID != account.ID {
			continue
		}
		existingBySymbol[NormalizeSymbol(position.Symbol)] = position
	}

	seenSymbols := make(map[string]struct{}, len(exchangePositions))
	for _, item := range exchangePositions {
		symbol := NormalizeSymbol(stringValue(item["symbol"]))
		if symbol == "" {
			continue
		}
		positionAmt := parseFloatValue(item["positionAmt"])
		if positionAmt == 0 {
			continue
		}
		seenSymbols[symbol] = struct{}{}
		side := "LONG"
		if positionAmt < 0 {
			side = "SHORT"
		}
		quantity := math.Abs(positionAmt)
		entryPrice := parseFloatValue(item["entryPrice"])
		markPrice := firstPositive(parseFloatValue(item["markPrice"]), entryPrice)
		strategyVersionID := p.resolveLivePositionStrategyVersionID(account.ID, symbol)
		position := existingBySymbol[symbol]
		position.AccountID = account.ID
		position.StrategyVersionID = firstNonEmpty(strategyVersionID, position.StrategyVersionID)
		position.Symbol = symbol
		position.Side = side
		position.Quantity = quantity
		position.EntryPrice = entryPrice
		position.MarkPrice = markPrice
		if _, err := p.store.SavePosition(position); err != nil {
			return err
		}
	}

	for symbol, position := range existingBySymbol {
		if _, ok := seenSymbols[symbol]; ok {
			continue
		}
		if err := p.store.DeletePosition(position.ID); err != nil {
			return err
		}
	}
	return nil
}

func (p *Platform) resolveLivePositionStrategyVersionID(accountID, symbol string) string {
	sessions, err := p.ListLiveSessions()
	if err == nil {
		for _, session := range sessions {
			if session.AccountID != accountID {
				continue
			}
			sessionSymbol := NormalizeSymbol(firstNonEmpty(stringValue(session.State["symbol"]), stringValue(session.State["lastSymbol"])))
			if sessionSymbol != "" && sessionSymbol != symbol {
				continue
			}
			version, versionErr := p.resolveCurrentStrategyVersion(session.StrategyID)
			if versionErr == nil {
				return version.ID
			}
		}
	}
	return ""
}

func (p *Platform) StopLiveSession(sessionID string) (domain.LiveSession, error) {
	return p.StopLiveSessionWithForce(sessionID, false)
}

func (p *Platform) StopLiveSessionWithForce(sessionID string, force bool) (domain.LiveSession, error) {
	logger := p.logger("service.live", "session_id", sessionID)
	existing, err := p.store.GetLiveSession(sessionID)
	if err != nil {
		logger.Error("load live session before stop failed", "error", err)
		return domain.LiveSession{}, err
	}
	if !force {
		if err := p.ensureNoActivePositionsOrOrders(existing.AccountID, existing.StrategyID); err != nil {
			logger.Warn("stop live session blocked by active positions or orders", "error", err)
			return domain.LiveSession{}, err
		}
	}
	session, err := p.store.UpdateLiveSessionStatus(sessionID, "STOPPED")
	if err != nil {
		logger.Error("stop live session failed", "error", err)
		return domain.LiveSession{}, err
	}
	p.mu.Lock()
	delete(p.livePlans, session.ID)
	p.mu.Unlock()
	_, _ = p.stopLinkedLiveSignalRuntime(session)
	p.logger("service.live",
		"session_id", session.ID,
		"account_id", session.AccountID,
		"strategy_id", session.StrategyID,
	).Info("live session stopped")
	return session, nil
}

func (p *Platform) triggerLiveSessionFromSignal(sessionID, runtimeSessionID string, summary map[string]any, eventTime time.Time) error {
	session, err := p.store.GetLiveSession(sessionID)
	if err != nil {
		return err
	}
	if session.Status != "RUNNING" {
		return nil
	}

	// Symbol mismatch guard — secondary defense against cross-symbol contamination
	triggerSymbol := signalRuntimeSummarySymbol(summary)
	sessionSymbol := NormalizeSymbol(firstNonEmpty(
		stringValue(session.State["symbol"]),
		stringValue(session.State["lastSymbol"]),
	))
	if triggerSymbol != "" && sessionSymbol != "" && triggerSymbol != sessionSymbol {
		p.logger("service.live",
			"session_id", sessionID,
			"trigger_symbol", triggerSymbol,
			"session_symbol", sessionSymbol,
		).Warn("signal symbol mismatch in triggerLiveSessionFromSignal, skipping")
		recordSignalSymbolMismatch(session.State, triggerSymbol, sessionSymbol, eventTime)
		return nil
	}

	state := cloneMetadata(session.State)
	state["lastSignalRuntimeEventAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastSignalRuntimeEvent"] = cloneMetadata(summary)
	state["lastSignalRuntimeSessionId"] = runtimeSessionID
	recordStrategyTriggerHealth(state, summary, eventTime)
	updatedSession, err := p.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		return err
	}
	if err := p.evaluateLiveSessionOnSignal(updatedSession, runtimeSessionID, summary, eventTime); err != nil {
		state = cloneMetadata(updatedSession.State)
		state["lastStrategyTriggerError"] = err.Error()
		state["lastStrategyTriggerErrorAt"] = eventTime.UTC().Format(time.RFC3339)
		_, _ = p.store.UpdateLiveSessionState(updatedSession.ID, state)
		return err
	}
	return nil
}

func (p *Platform) evaluateLiveSessionOnSignal(session domain.LiveSession, runtimeSessionID string, summary map[string]any, eventTime time.Time) error {
	session, err := p.syncLiveSessionRuntime(session)
	if err != nil {
		return err
	}
	session, _ = p.syncLatestLiveSessionOrder(session, eventTime)
	session, plan, err := p.ensureLiveExecutionPlan(session)
	if err != nil {
		return err
	}

	state := cloneMetadata(session.State)
	state["strategyEvaluationMode"] = "signal-runtime-heartbeat"
	state["lastStrategyEvaluationAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastStrategyEvaluationTrigger"] = cloneMetadata(summary)
	state["lastStrategyEvaluationTriggerSource"] = buildStrategyEvaluationTriggerSource(summary)
	state["lastStrategyEvaluationStatus"] = "evaluated"
	state["lastStrategyEvaluationPlanLength"] = len(plan)
	index := resolveLivePlanIndex(state)
	state["lastStrategyEvaluationRemaining"] = maxInt(len(plan)-index, 0)
	var nextPlannedEvent time.Time
	var nextPlannedPrice float64
	var nextPlannedSide string
	var nextPlannedRole string
	var nextPlannedReason string
	if index >= 0 && index < len(plan) {
		step := plan[index]
		nextPlannedEvent = step.EventTime
		nextPlannedPrice = step.Price
		nextPlannedSide = step.Side
		nextPlannedRole = step.Role
		nextPlannedReason = step.Reason
		state["lastStrategyEvaluationNextPlannedEventAt"] = formatOptionalRFC3339(nextPlannedEvent)
		state["lastStrategyEvaluationNextPlannedPrice"] = nextPlannedPrice
		state["lastStrategyEvaluationNextPlannedSide"] = nextPlannedSide
		state["lastStrategyEvaluationNextPlannedRole"] = nextPlannedRole
		state["lastStrategyEvaluationNextPlannedReason"] = nextPlannedReason
	} else {
		state["lastStrategyEvaluationStatus"] = "plan-exhausted"
		delete(state, "lastStrategyIntent")
		delete(state, "lastStrategyIntentSignature")
		appendTimelineEvent(state, "strategy", eventTime, "plan-exhausted", map[string]any{
			"planLength": len(plan),
		})
		return p.finalizeLiveSessionPlanExhausted(session, state, plan, eventTime)
	}

	sourceGate := map[string]any{
		"ready":   false,
		"missing": []any{},
		"stale":   []any{},
	}
	sourceStates := map[string]any{}
	signalBarStates := map[string]any{}
	evalSymbol := NormalizeSymbol(firstNonEmpty(stringValue(state["symbol"]), stringValue(state["lastSymbol"])))
	if runtimeSession, runtimeErr := p.GetSignalRuntimeSession(firstNonEmpty(runtimeSessionID, stringValue(state["signalRuntimeSessionId"]))); runtimeErr == nil {
		state["lastSignalRuntimeStatus"] = runtimeSession.Status
		sourceStates = cloneMetadata(mapValue(runtimeSession.State["sourceStates"]))
		if sourceStates == nil {
			sourceStates = map[string]any{}
		}
		signalBarStates = cloneMetadata(mapValue(runtimeSession.State["signalBarStates"]))
		if signalBarStates == nil {
			signalBarStates = map[string]any{}
		}
		// Per-session symbol scoping: filter out data from other symbols
		sourceStates = filterSourceStatesBySymbol(sourceStates, evalSymbol)
		signalBarStates = filterSignalBarStatesBySymbol(signalBarStates, evalSymbol)
		state["lastStrategyEvaluationSourceStates"] = sourceStates
		state["lastStrategyEvaluationSignalBarStates"] = signalBarStates
		state["lastStrategyEvaluationSignalBarStateCount"] = len(signalBarStates)
		state["lastStrategyEvaluationSourceStateCount"] = len(sourceStates)
		state["lastStrategyEvaluationRuntimeSummary"] = cloneMetadata(mapValue(runtimeSession.State["lastEventSummary"]))
		sourceGate = p.evaluateRuntimeSignalSourceReadiness(session.StrategyID, runtimeSession, eventTime)
		state["lastStrategyEvaluationSourceGate"] = sourceGate
		recordStrategySourceGateHealth(state, sourceGate, eventTime)
	}
	if len(signalBarStates) == 0 {
		bootstrapStates, bootstrapErr := p.liveSignalBarStates(stringValue(state["symbol"]), stringValue(state["signalTimeframe"]))
		if bootstrapErr == nil && len(bootstrapStates) > 0 {
			signalBarStates = bootstrapStates
			state["lastStrategyEvaluationSignalBarStates"] = signalBarStates
			state["lastStrategyEvaluationSignalBarStateCount"] = len(signalBarStates)
			state["lastStrategyEvaluationSignalBarBootstrap"] = "market-cache"
		}
	}
	if !boolValue(sourceGate["ready"]) {
		state["lastStrategyEvaluationStatus"] = "waiting-source-states"
		appendTimelineEvent(state, "strategy", eventTime, "waiting-source-states", map[string]any{
			"missing": len(metadataList(sourceGate["missing"])),
			"stale":   len(metadataList(sourceGate["stale"])),
		})
		_, err := p.store.UpdateLiveSessionState(session.ID, state)
		return err
	}

	evaluationSession := session
	evaluationSession.State = cloneMetadata(state)
	executionContext, decision, updatedDecisionState, err := p.evaluateLiveSignalDecision(evaluationSession, summary, sourceStates, signalBarStates, eventTime, nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason)
	if err != nil {
		state["lastStrategyEvaluationStatus"] = "decision-error"
		state["lastStrategyDecision"] = map[string]any{
			"action": "error",
			"reason": err.Error(),
		}
		appendTimelineEvent(state, "strategy", eventTime, "decision-error", map[string]any{"error": err.Error()})
		recordStrategyDecisionErrorHealth(state, eventTime, err)
		_, updateErr := p.store.UpdateLiveSessionState(session.ID, state)
		if updateErr != nil {
			return updateErr
		}
		return err
	}
	state = updatedDecisionState

	signalIntent := deriveLiveSignalIntent(decision, executionContext.Symbol)
	var intent map[string]any
	var executionProposal map[string]any
	state["lastStrategyDecision"] = map[string]any{
		"action":   decision.Action,
		"reason":   decision.Reason,
		"metadata": cloneMetadata(decision.Metadata),
	}
	recordStrategyDecisionHealth(state, decision, eventTime)
	if livePositionState := cloneMetadata(mapValue(decision.Metadata["livePositionState"])); len(livePositionState) > 0 {
		state["lastLivePositionState"] = livePositionState
		symbol := NormalizeSymbol(firstNonEmpty(stringValue(livePositionState["symbol"]), stringValue(state["symbol"])))
		livePositionState["symbol"] = symbol
		if virtualPosition := cloneMetadata(mapValue(state["virtualPosition"])); len(virtualPosition) > 0 && NormalizeSymbol(stringValue(virtualPosition["symbol"])) == symbol {
			for key, value := range livePositionState {
				virtualPosition[key] = value
			}
			state["virtualPosition"] = virtualPosition
		} else {
			state["livePositionState"] = livePositionState
		}
	}
	state["lastStrategyEvaluationContext"] = map[string]any{
		"strategyVersionId":   executionContext.StrategyVersionID,
		"signalTimeframe":     executionContext.SignalTimeframe,
		"executionDataSource": executionContext.ExecutionDataSource,
		"symbol":              executionContext.Symbol,
	}
	// P0-3: Inject ATR14 from signal bar state for volatility-adjusted sizing
	if signalBarState := mapValue(decision.Metadata["signalBarState"]); len(signalBarState) > 0 {
		if atr14 := parseFloatValue(signalBarState["atr14"]); atr14 > 0 {
			state["atr14"] = atr14
		}
	}
	if signalIntent != nil {
		state["lastSignalIntent"] = signalIntentToMap(*signalIntent)
		planningSession := session
		planningSession.State = cloneMetadata(state)
		proposal, proposalErr := p.buildLiveExecutionProposal(planningSession, executionContext, summary, sourceStates, eventTime, *signalIntent)
		if proposalErr != nil {
			state["lastStrategyEvaluationStatus"] = "execution-planning-error"
			state["lastExecutionProposalError"] = proposalErr.Error()
			recordExecutionPlanningErrorHealth(state, eventTime, proposalErr)
			appendTimelineEvent(state, "strategy", eventTime, "execution-planning-error", map[string]any{"error": proposalErr.Error()})
			_, updateErr := p.store.UpdateLiveSessionState(session.ID, state)
			if updateErr != nil {
				return updateErr
			}
			return proposalErr
		}
		delete(state, "lastExecutionProposalError")
		executionProposal = executionProposalToMap(proposal)
		state["lastExecutionProposal"] = executionProposal
		state["lastExecutionProfile"] = executionProposalSummary(executionProposal)
		recordExecutionPlanningHealth(state, executionProposal, eventTime)
		state["lastExecutionTelemetry"] = map[string]any{
			"evaluatedAt":     stringValue(mapValue(executionProposal["metadata"])["executionEvaluatedAt"]),
			"decision":        stringValue(mapValue(executionProposal["metadata"])["executionDecision"]),
			"book":            cloneMetadata(mapValue(mapValue(executionProposal["metadata"])["orderBookSnapshot"])),
			"decisionContext": cloneMetadata(mapValue(mapValue(executionProposal["metadata"])["executionDecisionContext"])),
			"profile":         cloneMetadata(mapValue(state["lastExecutionProfile"])),
		}
		updateExecutionEventStats(state, executionProposal, nil)
		intent = executionProposal
		state["lastStrategyIntent"] = executionProposal
		state["lastStrategyIntentSignature"] = buildLiveIntentSignature(executionProposal)
	} else {
		delete(state, "lastSignalIntent")
		delete(state, "lastExecutionProposal")
		delete(state, "lastExecutionProfile")
		delete(state, "lastStrategyIntent")
		delete(state, "lastStrategyIntentSignature")
	}
	decisionEvent, decisionEventErr := p.recordStrategyDecisionEvent(
		session,
		firstNonEmpty(runtimeSessionID, stringValue(state["signalRuntimeSessionId"])),
		eventTime,
		summary,
		sourceStates,
		signalBarStates,
		sourceGate,
		executionContext,
		decision,
		cloneMetadata(mapValue(state["lastSignalIntent"])),
		executionProposal,
	)
	if decisionEventErr != nil {
		state["lastStrategyDecisionEventError"] = decisionEventErr.Error()
	} else {
		delete(state, "lastStrategyDecisionEventError")
		state["lastStrategyDecisionEventId"] = decisionEvent.ID
		if len(executionProposal) > 0 {
			executionProposal["decisionEventId"] = decisionEvent.ID
			proposalMetadata := cloneMetadata(mapValue(executionProposal["metadata"]))
			proposalMetadata["decisionEventId"] = decisionEvent.ID
			executionProposal["metadata"] = proposalMetadata
			intent = executionProposal
			state["lastExecutionProposal"] = executionProposal
			state["lastStrategyIntent"] = executionProposal
		}
	}
	appendTimelineEvent(state, "strategy", eventTime, "decision", map[string]any{
		"action":            decision.Action,
		"reason":            decision.Reason,
		"decisionState":     stringValue(decision.Metadata["decisionState"]),
		"signalKind":        stringValue(decision.Metadata["signalKind"]),
		"signalIntent":      cloneMetadata(mapValue(state["lastSignalIntent"])),
		"intent":            cloneMetadata(intent),
		"executionStrategy": stringValue(executionProposal["executionStrategy"]),
		"executionProfile":  stringValue(mapValue(executionProposal["metadata"])["executionProfile"]),
		"executionDecision": stringValue(mapValue(executionProposal["metadata"])["executionDecision"]),
		"executionMode":     stringValue(mapValue(executionProposal["metadata"])["executionMode"]),
		"reduceOnly":        boolValue(executionProposal["reduceOnly"]),
		"fallback":          boolValue(mapValue(executionProposal["metadata"])["fallbackFromTimeout"]),
		"book":              cloneMetadata(mapValue(mapValue(executionProposal["metadata"])["orderBookSnapshot"])),
	})
	if executionProposal != nil && strings.EqualFold(stringValue(executionProposal["status"]), "dispatchable") {
		state["lastStrategyEvaluationStatus"] = "intent-ready"
	} else if executionProposal != nil {
		state["lastStrategyEvaluationStatus"] = "waiting-execution"
	} else if decision.Action == "advance-plan" {
		state["lastStrategyEvaluationStatus"] = "monitoring"
	} else {
		state["lastStrategyEvaluationStatus"] = "waiting-decision"
	}
	updatedSession, err := p.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		return err
	}
	if executionProposal != nil {
		status := strings.TrimSpace(stringValue(executionProposal["status"]))
		switch status {
		case "virtual-initial":
			_, err = p.applyLiveVirtualInitialEvent(updatedSession, executionProposal, eventTime)
			return err
		case "virtual-exit":
			_, err = p.applyLiveVirtualExitEvent(updatedSession, executionProposal, eventTime)
			return err
		}
	}
	if !shouldAutoDispatchLiveIntent(updatedSession, intent, eventTime) {
		return nil
	}
	if _, err := p.dispatchLiveSessionIntent(updatedSession); err != nil {
		latestSession, latestErr := p.store.GetLiveSession(updatedSession.ID)
		if latestErr == nil {
			state = cloneMetadata(latestSession.State)
		} else {
			state = cloneMetadata(updatedSession.State)
		}
		if strings.TrimSpace(stringValue(state["lastDispatchedAt"])) == "" {
			state["lastDispatchedAt"] = eventTime.UTC().Format(time.RFC3339)
		}
		if strings.TrimSpace(stringValue(state["lastDispatchedIntentSignature"])) == "" {
			state["lastDispatchedIntentSignature"] = buildLiveIntentSignature(intent)
		}
		state["lastAutoDispatchError"] = err.Error()
		state["lastAutoDispatchAttemptAt"] = eventTime.UTC().Format(time.RFC3339)
		if strings.TrimSpace(stringValue(state["lastDispatchRejectedAt"])) == "" {
			state["lastDispatchRejectedAt"] = eventTime.UTC().Format(time.RFC3339)
		}
		if strings.TrimSpace(stringValue(state["lastDispatchRejectedStatus"])) == "" {
			state["lastDispatchRejectedStatus"] = "DISPATCH_ERROR"
		}
		appendTimelineEvent(state, "order", eventTime, "live-auto-dispatch-error", map[string]any{
			"error": err.Error(),
		})
		_, _ = p.store.UpdateLiveSessionState(updatedSession.ID, state)
		return err
	}
	return nil
}

func (p *Platform) finalizeLiveSessionPlanExhausted(session domain.LiveSession, state map[string]any, plan []paperPlannedOrder, eventTime time.Time) error {
	if state == nil {
		state = cloneMetadata(session.State)
	}
	state["planIndex"] = len(plan)
	state["planLength"] = len(plan)
	state["completedAt"] = eventTime.UTC().Format(time.RFC3339)

	updatedSession, err := p.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		return err
	}
	if !strings.EqualFold(updatedSession.Status, "RUNNING") {
		return nil
	}
	if boolValue(updatedSession.State["hasRecoveredPosition"]) || boolValue(updatedSession.State["hasRecoveredVirtualPosition"]) {
		return nil
	}
	if err := p.ensureNoActivePositionsOrOrders(updatedSession.AccountID, updatedSession.StrategyID); err != nil {
		if errors.Is(err, ErrActivePositionsOrOrders) {
			return nil
		}
		return err
	}
	if err := p.rolloverLiveSessionPlan(updatedSession, eventTime); err != nil {
		return err
	}
	return nil
}

func (p *Platform) rolloverLiveSessionPlan(session domain.LiveSession, eventTime time.Time) error {
	state := cloneMetadata(session.State)
	state["planIndex"] = 0
	state["planLength"] = 0
	state["lastPlanRolloverAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastPlanRolloverReason"] = "plan-exhausted"
	delete(state, "planReadyAt")
	delete(state, "planIndexRecoveredFromPosition")
	delete(state, "recoveredPlanIndex")
	delete(state, "lastStrategyEvaluationNextPlannedEventAt")
	delete(state, "lastStrategyEvaluationNextPlannedPrice")
	delete(state, "lastStrategyEvaluationNextPlannedSide")
	delete(state, "lastStrategyEvaluationNextPlannedRole")
	delete(state, "lastStrategyEvaluationNextPlannedReason")
	appendTimelineEvent(state, "strategy", eventTime, "plan-rollover-scheduled", map[string]any{
		"reason": "plan-exhausted",
	})
	if _, err := p.store.UpdateLiveSessionState(session.ID, state); err != nil {
		return err
	}
	p.mu.Lock()
	delete(p.livePlans, session.ID)
	p.mu.Unlock()
	return nil
}

func (p *Platform) evaluateLiveSignalDecision(session domain.LiveSession, summary map[string]any, sourceStates map[string]any, signalBarStates map[string]any, eventTime time.Time, nextPlannedEvent time.Time, nextPlannedPrice float64, nextPlannedSide, nextPlannedRole, nextPlannedReason string) (StrategyExecutionContext, StrategySignalDecision, map[string]any, error) {
	version, err := p.resolveCurrentStrategyVersion(session.StrategyID)
	if err != nil {
		return StrategyExecutionContext{}, StrategySignalDecision{}, cloneMetadata(session.State), err
	}
	parameters, err := p.resolveLiveSessionParameters(session, version)
	if err != nil {
		return StrategyExecutionContext{}, StrategySignalDecision{}, cloneMetadata(session.State), err
	}
	engine, engineKey, err := p.resolveStrategyEngine(version.ID, parameters)
	if err != nil {
		return StrategyExecutionContext{}, StrategySignalDecision{}, cloneMetadata(session.State), err
	}
	executionContext := StrategyExecutionContext{
		StrategyEngineKey:   engineKey,
		StrategyVersionID:   version.ID,
		SignalTimeframe:     stringValue(parameters["signalTimeframe"]),
		ExecutionDataSource: stringValue(parameters["executionDataSource"]),
		Symbol:              stringValue(parameters["symbol"]),
		From:                parseOptionalRFC3339(stringValue(parameters["from"])),
		To:                  parseOptionalRFC3339(stringValue(parameters["to"])),
		Parameters:          parameters,
		Semantics:           defaultExecutionSemantics(ExecutionModeLive, parameters),
	}
	evaluator, ok := engine.(SignalEvaluatingStrategyEngine)
	if !ok {
		return executionContext, StrategySignalDecision{
			Action: "wait",
			Reason: "engine-has-no-signal-evaluator",
		}, cloneMetadata(session.State), nil
	}
	currentPosition, _, err := p.resolveLiveSessionPositionSnapshot(session, executionContext.Symbol)
	if err != nil {
		return executionContext, StrategySignalDecision{}, cloneMetadata(session.State), err
	}
	updatedState, nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason := prepareLivePlanStepForSignalEvaluation(
		session.State,
		executionContext.Parameters,
		signalBarStates,
		executionContext.Symbol,
		executionContext.SignalTimeframe,
		currentPosition,
		eventTime,
		nextPlannedEvent,
		nextPlannedPrice,
		nextPlannedSide,
		nextPlannedRole,
		nextPlannedReason,
	)
	decision, err := evaluator.EvaluateSignal(StrategySignalEvaluationContext{
		ExecutionContext:  executionContext,
		TriggerSummary:    cloneMetadata(summary),
		SourceStates:      cloneMetadata(sourceStates),
		SignalBarStates:   cloneMetadata(signalBarStates),
		CurrentPosition:   currentPosition,
		SessionState:      cloneMetadata(session.State),
		EventTime:         eventTime.UTC(),
		NextPlannedEvent:  nextPlannedEvent.UTC(),
		NextPlannedPrice:  nextPlannedPrice,
		NextPlannedSide:   nextPlannedSide,
		NextPlannedRole:   nextPlannedRole,
		NextPlannedReason: nextPlannedReason,
	})
	if err != nil {
		return executionContext, StrategySignalDecision{}, updatedState, err
	}
	if strings.TrimSpace(decision.Action) == "" {
		decision.Action = "wait"
	}
	if strings.TrimSpace(decision.Reason) == "" {
		decision.Reason = "unspecified"
	}
	return executionContext, decision, updatedState, nil
}

func alignLivePlanStepToCurrentMarket(
	signalBarStates map[string]any,
	signalTimeframe string,
	currentPosition map[string]any,
	eventTime time.Time,
	nextPlannedEvent time.Time,
	nextPlannedPrice float64,
	nextPlannedSide, nextPlannedRole, nextPlannedReason string,
) (time.Time, float64, string, string, string) {
	if hasActiveLivePositionSnapshot(currentPosition) {
		return nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason
	}
	if !isLivePlanStepStale(nextPlannedEvent, signalTimeframe, eventTime) {
		return nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason
	}
	signalBarState, _ := pickSignalBarState(signalBarStates, NormalizeSymbol(stringValue(currentPosition["symbol"])), signalTimeframe)
	if signalBarState == nil {
		return nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason
	}
	gate := evaluateSignalBarGate(signalBarState, "", "entry", "")
	longReady := boolValue(gate["longReady"])
	shortReady := boolValue(gate["shortReady"])
	if longReady == shortReady {
		return nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason
	}
	current := mapValue(signalBarState["current"])
	price := parseFloatValue(current["close"])
	if price <= 0 {
		price = nextPlannedPrice
	}
	side := "BUY"
	if shortReady {
		side = "SELL"
	}
	return eventTime.UTC(), price, side, "entry", "LiveSignalBootstrap"
}

func isLivePlanStepStale(nextPlannedEvent time.Time, signalTimeframe string, now time.Time) bool {
	if nextPlannedEvent.IsZero() {
		return true
	}
	resolution := liveSignalResolution(signalTimeframe)
	step := resolutionToDuration(resolution)
	if step <= 0 {
		step = 4 * time.Hour
	}
	return now.UTC().After(nextPlannedEvent.UTC().Add(step))
}

func (p *Platform) syncLiveSessionRuntime(session domain.LiveSession) (domain.LiveSession, error) {
	state := cloneMetadata(session.State)
	plan, err := p.BuildSignalRuntimePlan(session.AccountID, session.StrategyID)
	if err != nil {
		state["signalRuntimeMode"] = "detached"
		state["signalRuntimeRequired"] = false
		state["signalRuntimeStatus"] = "ERROR"
		state["signalRuntimeError"] = err.Error()
		updated, updateErr := p.store.UpdateLiveSessionState(session.ID, state)
		if updateErr != nil {
			return domain.LiveSession{}, updateErr
		}
		return updated, err
	}

	required := len(metadataList(plan["requiredBindings"])) > 0
	state["signalRuntimePlan"] = plan
	state["signalRuntimeMode"] = "linked"
	state["signalRuntimeRequired"] = required
	state["signalRuntimeReady"] = boolValue(plan["ready"])
	state["dispatchMode"] = firstNonEmpty(stringValue(state["dispatchMode"]), "manual-review")
	if _, ok := state["planIndex"]; !ok {
		state["planIndex"] = 0
	}

	runtimeSessionID := stringValue(state["signalRuntimeSessionId"])
	if runtimeSessionID != "" {
		runtimeSession, getErr := p.GetSignalRuntimeSession(runtimeSessionID)
		if getErr == nil {
			state["signalRuntimeStatus"] = runtimeSession.Status
		} else {
			// 如果在内存中找不到该 signalRuntimeSession（例如系统发生重启后内存缓存被清空），
			// 则立刻抹除这个失效的 state ID，阻止崩溃向后传播，并在下方的必须条件分支中触发重新创建。
			runtimeSessionID = ""
			delete(state, "signalRuntimeSessionId")
			delete(state, "signalRuntimeStatus")
		}
	}

	if runtimeSessionID == "" && required {
		runtimeSession, resolveErr := p.resolveLiveRuntimeSession(session.AccountID, session.StrategyID)
		if resolveErr != nil {
			var createErr error
			runtimeSession, createErr = p.CreateSignalRuntimeSession(session.AccountID, session.StrategyID)
			if createErr != nil {
				state["signalRuntimeStatus"] = "ERROR"
				state["signalRuntimeError"] = createErr.Error()
				updated, updateErr := p.store.UpdateLiveSessionState(session.ID, state)
				if updateErr != nil {
					return domain.LiveSession{}, updateErr
				}
				return updated, createErr
			}
		}
		runtimeSessionID = runtimeSession.ID
		state["signalRuntimeSessionId"] = runtimeSession.ID
		state["signalRuntimeStatus"] = runtimeSession.Status
	}

	updated, err := p.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		return domain.LiveSession{}, err
	}
	return updated, nil
}

func (p *Platform) ensureLiveSessionSignalRuntimeStarted(session domain.LiveSession) (domain.LiveSession, error) {
	if !boolValue(session.State["signalRuntimeRequired"]) {
		return session, nil
	}
	if !boolValue(session.State["signalRuntimeReady"]) {
		return session, fmt.Errorf("live session %s signal runtime plan is not ready", session.ID)
	}
	runtimeSessionID := stringValue(session.State["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		return domain.LiveSession{}, fmt.Errorf("live session %s has no linked signal runtime session", session.ID)
	}
	runtimeSession, err := p.StartSignalRuntimeSession(runtimeSessionID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	state := cloneMetadata(session.State)
	state["signalRuntimeStatus"] = runtimeSession.Status
	state["signalRuntimeSessionId"] = runtimeSession.ID
	session, err = p.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		return domain.LiveSession{}, err
	}
	return p.awaitLiveSignalRuntimeReadiness(session, runtimeSession.ID, time.Duration(p.runtimePolicy.PaperStartReadinessTimeoutSecs)*time.Second)
}

func (p *Platform) awaitLiveSignalRuntimeReadiness(session domain.LiveSession, runtimeSessionID string, timeout time.Duration) (domain.LiveSession, error) {
	deadline := time.Now().Add(timeout)
	lastGate := map[string]any{}
	for time.Now().Before(deadline) || time.Now().Equal(deadline) {
		runtimeSession, err := p.GetSignalRuntimeSession(runtimeSessionID)
		if err != nil {
			return domain.LiveSession{}, err
		}
		lastGate = p.evaluateRuntimeSignalSourceReadiness(session.StrategyID, runtimeSession, time.Now().UTC())
		if boolValue(lastGate["ready"]) {
			state := cloneMetadata(session.State)
			state["signalRuntimeStatus"] = runtimeSession.Status
			state["signalRuntimeStartReadiness"] = lastGate
			state["signalRuntimeLastCheckedAt"] = time.Now().UTC().Format(time.RFC3339)
			return p.store.UpdateLiveSessionState(session.ID, state)
		}
		time.Sleep(250 * time.Millisecond)
	}
	state := cloneMetadata(session.State)
	state["signalRuntimeStartReadiness"] = lastGate
	state["signalRuntimeLastCheckedAt"] = time.Now().UTC().Format(time.RFC3339)
	updated, err := p.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		return domain.LiveSession{}, err
	}
	return updated, fmt.Errorf("live session %s runtime readiness timed out", session.ID)
}

func (p *Platform) stopLinkedLiveSignalRuntime(session domain.LiveSession) (domain.SignalRuntimeSession, error) {
	runtimeSessionID := stringValue(session.State["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		return domain.SignalRuntimeSession{}, fmt.Errorf("live session %s has no linked signal runtime session", session.ID)
	}
	runtimeSession, err := p.StopSignalRuntimeSession(runtimeSessionID)
	if err != nil {
		return domain.SignalRuntimeSession{}, err
	}
	state := cloneMetadata(session.State)
	state["signalRuntimeStatus"] = runtimeSession.Status
	_, _ = p.store.UpdateLiveSessionState(session.ID, state)
	return runtimeSession, nil
}

func (p *Platform) ensureLiveExecutionPlan(session domain.LiveSession) (domain.LiveSession, []paperPlannedOrder, error) {
	p.mu.Lock()
	if plan, ok := p.livePlans[session.ID]; ok {
		p.mu.Unlock()
		reconciled, err := p.reconcileLiveSessionPlanIndex(session, plan, time.Now().UTC(), "live-plan-cache-reconcile")
		if err != nil {
			return domain.LiveSession{}, nil, err
		}
		return reconciled, plan, nil
	}
	p.mu.Unlock()

	session, err := p.syncLiveSessionRuntime(session)
	if err != nil {
		return domain.LiveSession{}, nil, err
	}

	version, err := p.resolveCurrentStrategyVersion(session.StrategyID)
	if err != nil {
		return domain.LiveSession{}, nil, err
	}
	parameters, err := p.resolveLiveSessionParameters(session, version)
	if err != nil {
		return domain.LiveSession{}, nil, err
	}
	engine, engineKey, err := p.resolveStrategyEngine(version.ID, parameters)
	if err != nil {
		return domain.LiveSession{}, nil, err
	}

	semantics := defaultExecutionSemantics(ExecutionModeLive, parameters)
	plan, err := p.buildLiveExecutionPlanFromMarketData(session, version, engine, engineKey, parameters, semantics)
	if err != nil {
		return domain.LiveSession{}, nil, err
	}
	for i := range plan {
		plan[i].Metadata = cloneMetadata(plan[i].Metadata)
		if plan[i].Metadata == nil {
			plan[i].Metadata = map[string]any{}
		}
		plan[i].Metadata["source"] = "live-session-strategy-engine"
		plan[i].Metadata["liveSessionId"] = session.ID
		delete(plan[i].Metadata, "paperSession")
	}

	p.mu.Lock()
	p.livePlans[session.ID] = plan
	p.mu.Unlock()

	state := cloneMetadata(session.State)
	state["runner"] = "strategy-engine"
	state["runtimeMode"] = "canonical-strategy-engine"
	state["strategyVersionId"] = version.ID
	state["strategyEngine"] = engineKey
	state["signalTimeframe"] = stringValue(parameters["signalTimeframe"])
	state["executionDataSource"] = stringValue(parameters["executionDataSource"])
	state["symbol"] = stringValue(parameters["symbol"])
	state["executionMode"] = string(semantics.Mode)
	state["slippageMode"] = string(semantics.SlippageMode)
	state["tradingFeeBps"] = semantics.TradingFeeBps
	state["fundingRateBps"] = semantics.FundingRateBps
	state["fundingIntervalHours"] = semantics.FundingIntervalHours
	state["planLength"] = len(plan)
	state["planReadyAt"] = time.Now().UTC().Format(time.RFC3339)
	delete(state, "completedAt")
	if _, ok := state["planIndex"]; !ok {
		state["planIndex"] = 0
	}
	positionSnapshot, foundPosition, positionErr := p.resolveLiveSessionPositionSnapshot(session, stringValue(parameters["symbol"]))
	if positionErr != nil {
		return domain.LiveSession{}, nil, positionErr
	}
	state["recoveredPosition"] = positionSnapshot
	state["hasRecoveredPosition"] = foundPosition
	state["hasRecoveredRealPosition"] = foundPosition
	state["hasRecoveredVirtualPosition"] = boolValue(positionSnapshot["virtual"])
	state["lastRecoveredPositionAt"] = time.Now().UTC().Format(time.RFC3339)
	state["positionRecoverySource"] = "platform-position-store"
	state["positionRecoveryStatus"] = "flat"
	if foundPosition {
		state["positionRecoveryStatus"] = "monitoring-open-position"
	} else if boolValue(positionSnapshot["virtual"]) {
		state["positionRecoveryStatus"] = "monitoring-virtual-position"
	}
	if nextIndex, adjusted := reconcileLivePlanIndexWithPosition(plan, resolveLivePlanIndex(state), positionSnapshot, foundPosition); adjusted {
		state["planIndex"] = nextIndex
		state["planIndexRecoveredFromPosition"] = true
		state["recoveredPlanIndex"] = nextIndex
	} else {
		delete(state, "planIndexRecoveredFromPosition")
	}
	updatedSession, err := p.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		return domain.LiveSession{}, nil, err
	}
	return updatedSession, plan, nil
}

func (p *Platform) reconcileLiveSessionPlanIndex(session domain.LiveSession, plan []paperPlannedOrder, recoveredAt time.Time, source string) (domain.LiveSession, error) {
	if len(plan) == 0 || strings.TrimSpace(session.ID) == "" {
		return session, nil
	}

	state := cloneMetadata(session.State)
	currentIndex := resolveLivePlanIndex(state)
	symbol := NormalizeSymbol(firstNonEmpty(stringValue(state["symbol"]), stringValue(state["lastSymbol"])))
	if symbol == "" {
		if maxIntValue(state["planLength"], -1) != len(plan) {
			state["planLength"] = len(plan)
			return p.store.UpdateLiveSessionState(session.ID, state)
		}
		return session, nil
	}

	positionSnapshot, foundPosition, err := p.resolveLiveSessionPositionSnapshot(session, symbol)
	if err != nil {
		return domain.LiveSession{}, err
	}

	nextIndex, adjusted := reconcileLivePlanIndexWithPosition(plan, currentIndex, positionSnapshot, foundPosition)
	planLengthAdjusted := maxIntValue(state["planLength"], -1) != len(plan)
	if !adjusted && !planLengthAdjusted {
		return session, nil
	}

	state["planLength"] = len(plan)
	if nextIndex < len(plan) {
		delete(state, "completedAt")
	}
	if !adjusted {
		return p.store.UpdateLiveSessionState(session.ID, state)
	}
	state["recoveredPosition"] = positionSnapshot
	state["hasRecoveredPosition"] = foundPosition
	state["hasRecoveredRealPosition"] = foundPosition
	state["hasRecoveredVirtualPosition"] = boolValue(positionSnapshot["virtual"])
	state["lastRecoveredPositionAt"] = recoveredAt.UTC().Format(time.RFC3339)
	state["positionRecoverySource"] = firstNonEmpty(source, "live-plan-cache-reconcile")
	state["planIndex"] = nextIndex
	state["planIndexRecoveredFromPosition"] = true
	state["recoveredPlanIndex"] = nextIndex

	return p.store.UpdateLiveSessionState(session.ID, state)
}

func reconcileLivePlanIndexWithPosition(plan []paperPlannedOrder, currentIndex int, position map[string]any, found bool) (int, bool) {
	if len(plan) == 0 || currentIndex < 0 {
		return currentIndex, false
	}
	virtualFound := boolValue(position["virtual"])
	flatPosition := (!found && !virtualFound) || (parseFloatValue(position["quantity"]) <= 0 && !virtualFound)
	normalizedIndex := false
	if currentIndex > len(plan) {
		if flatPosition {
			return len(plan), true
		}
		currentIndex = len(plan) - 1
		normalizedIndex = true
	}
	if currentIndex >= len(plan) {
		if flatPosition {
			return currentIndex, false
		}
		currentIndex = len(plan) - 1
		normalizedIndex = true
	}
	if flatPosition {
		if strings.EqualFold(plan[currentIndex].Role, "exit") {
			for i := currentIndex; i >= 0; i-- {
				if strings.EqualFold(plan[i].Role, "entry") {
					return i, true
				}
			}
		}
		return currentIndex, normalizedIndex
	}
	if strings.EqualFold(plan[currentIndex].Role, "entry") {
		for i := currentIndex; i < len(plan); i++ {
			if strings.EqualFold(plan[i].Role, "exit") {
				return i, true
			}
		}
	}
	return currentIndex, normalizedIndex
}

func resolveLivePlanIndex(state map[string]any) int {
	if value, ok := toFloat64(state["planIndex"]); ok && value >= 0 {
		return int(value)
	}
	return 0
}

func resolveNextLivePlanIndex(state map[string]any) int {
	return resolveLivePlanIndex(state) + 1
}

func (p *Platform) resolveLiveSessionPositionSnapshot(session domain.LiveSession, symbol string) (map[string]any, bool, error) {
	positionSnapshot, foundPosition, err := p.resolvePaperSessionPositionSnapshot(session.AccountID, symbol)
	if err != nil {
		return nil, false, err
	}
	hasRealPosition := foundPosition || math.Abs(parseFloatValue(positionSnapshot["quantity"])) > 0
	livePositionState := cloneMetadata(mapValue(session.State["livePositionState"]))
	if len(livePositionState) > 0 && hasRealPosition {
		liveSymbol := NormalizeSymbol(firstNonEmpty(stringValue(livePositionState["symbol"]), symbol))
		if liveSymbol == NormalizeSymbol(symbol) {
			mergedPosition := cloneMetadata(positionSnapshot)
			for key, value := range livePositionState {
				mergedPosition[key] = value
			}
			positionSnapshot = mergedPosition
		}
	}
	if hasRealPosition {
		return positionSnapshot, foundPosition, nil
	}
	virtualPosition := cloneMetadata(mapValue(session.State["virtualPosition"]))
	if len(virtualPosition) == 0 {
		return positionSnapshot, foundPosition, nil
	}
	virtualSymbol := NormalizeSymbol(firstNonEmpty(stringValue(virtualPosition["symbol"]), symbol))
	if NormalizeSymbol(symbol) != "" && virtualSymbol != NormalizeSymbol(symbol) {
		return positionSnapshot, foundPosition, nil
	}
	virtualPosition["found"] = false
	virtualPosition["hasRealPosition"] = false
	virtualPosition["hasVirtualPosition"] = true
	virtualPosition["virtual"] = true
	virtualPosition["symbol"] = virtualSymbol
	return virtualPosition, false, nil
}

func (p *Platform) resolveLiveSessionParameters(session domain.LiveSession, version domain.StrategyVersion) (map[string]any, error) {
	parameters := cloneMetadata(version.Parameters)
	if parameters == nil {
		parameters = map[string]any{}
	}
	if stringValue(parameters["signalTimeframe"]) == "" {
		parameters["signalTimeframe"] = normalizePaperSignalTimeframe(version.SignalTimeframe)
	}
	if stringValue(parameters["executionDataSource"]) == "" {
		parameters["executionDataSource"] = normalizePaperExecutionSource(version.ExecutionTimeframe)
	}
	if stringValue(parameters["symbol"]) == "" {
		parameters["symbol"] = resolvePaperPlanSymbol(version)
	}
	state := cloneMetadata(session.State)
	for _, key := range []string{
		"signalTimeframe",
		"executionDataSource",
		"executionStrategy",
		"executionOrderType",
		"executionTimeInForce",
		"executionPostOnly",
		"executionWideSpreadMode",
		"executionRestingTimeoutSeconds",
		"executionTimeoutFallbackOrderType",
		"executionTimeoutFallbackTimeInForce",
		"executionMaxSpreadBps",
		"symbol",
		"from",
		"to",
		"strategyEngine",
		"fixed_slippage",
		"stop_mode",
		"stop_loss_atr",
		"profit_protect_atr",
		"long_reentry_atr",
		"short_reentry_atr",
		"reentry_size_schedule",
		"trailing_stop_atr",
		"delayed_trailing_activation_atr",
		"reentry_decay_factor",
		"max_trades_per_bar",
		"dir2_zero_initial",
		"zero_initial_mode",
	} {
		if value, ok := state[key]; ok {
			parameters[key] = value
		}
	}
	return NormalizeBacktestParameters(parameters)
}

func deriveLiveSignalIntent(decision StrategySignalDecision, symbol string) *SignalIntent {
	meta := cloneMetadata(decision.Metadata)
	signalBarDecision := mapValue(meta["signalBarDecision"])
	if strings.TrimSpace(decision.Action) != "advance-plan" || signalBarDecision == nil {
		return nil
	}
	nextSide := strings.ToUpper(strings.TrimSpace(stringValue(meta["nextPlannedSide"])))
	if nextSide == "" {
		longReady := boolValue(signalBarDecision["longReady"])
		shortReady := boolValue(signalBarDecision["shortReady"])
		switch {
		case longReady && !shortReady:
			nextSide = "BUY"
		case shortReady && !longReady:
			nextSide = "SELL"
		default:
			return nil
		}
	}
	marketPrice := parseFloatValue(meta["marketPrice"])
	marketSource := stringValue(meta["marketSource"])
	signalKind := stringValue(meta["signalKind"])
	decisionState := stringValue(meta["decisionState"])
	signalBarStateKey := stringValue(meta["signalBarStateKey"])
	entryProximityBps := parseFloatValue(meta["entryProximityBps"])
	spreadBps := parseFloatValue(meta["spreadBps"])
	ma20 := parseFloatValue(signalBarDecision["ma20"])
	atr14 := parseFloatValue(signalBarDecision["atr14"])
	liquidityBias := stringValue(meta["liquidityBias"])
	biasActionable := boolValue(meta["biasActionable"])
	bestBid := parseFloatValue(meta["bestBid"])
	bestAsk := parseFloatValue(meta["bestAsk"])
	bestBidQty := parseFloatValue(meta["bestBidQty"])
	bestAskQty := parseFloatValue(meta["bestAskQty"])
	quantity := firstPositive(parseFloatValue(meta["suggestedQuantity"]), 0.001)
	role := strings.ToLower(strings.TrimSpace(firstNonEmpty(stringValue(meta["nextPlannedRole"]), "entry")))
	reason := stringValue(meta["nextPlannedReason"])

	return &SignalIntent{
		Action:         role,
		Role:           role,
		Reason:         reason,
		Side:           nextSide,
		Symbol:         NormalizeSymbol(symbol),
		SignalKind:     signalKind,
		DecisionState:  decisionState,
		PlannedEventAt: stringValue(meta["nextPlannedEvent"]),
		PlannedPrice:   parseFloatValue(meta["nextPlannedPrice"]),
		PriceHint:      marketPrice,
		PriceSource:    marketSource,
		Quantity:       quantity,
		Metadata: map[string]any{
			"signalBarStateKey": signalBarStateKey,
			"entryProximityBps": entryProximityBps,
			"spreadBps":         spreadBps,
			"ma20":              ma20,
			"atr14":             atr14,
			"liquidityBias":     liquidityBias,
			"biasActionable":    biasActionable,
			"bestBid":           bestBid,
			"bestAsk":           bestAsk,
			"bestBidQty":        bestBidQty,
			"bestAskQty":        bestAskQty,
			"bookImbalance":     parseFloatValue(meta["bookImbalance"]),
		},
	}
}

func (p *Platform) buildLiveExecutionProposal(session domain.LiveSession, executionContext StrategyExecutionContext, summary map[string]any, sourceStates map[string]any, eventTime time.Time, intent SignalIntent) (ExecutionProposal, error) {
	strategy, _, err := p.resolveExecutionStrategy(executionContext.Parameters)
	if err != nil {
		return ExecutionProposal{}, err
	}
	account, _ := p.store.GetAccount(session.AccountID)
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session:        session,
		Account:        account,
		Execution:      executionContext,
		TriggerSummary: cloneMetadata(summary),
		SourceStates:   cloneMetadata(sourceStates),
		EventTime:      eventTime.UTC(),
		Intent:         intent,
	})
	if err != nil {
		return ExecutionProposal{}, err
	}
	return adjustLiveExecutionProposalForVirtualSemantics(session, executionContext.Parameters, proposal), nil
}

func adjustLiveExecutionProposalForVirtualSemantics(session domain.LiveSession, parameters map[string]any, proposal ExecutionProposal) ExecutionProposal {
	reasonTag := normalizeStrategyReasonTag(proposal.Reason)
	zeroInitial := true
	if _, ok := parameters["dir2_zero_initial"]; ok {
		zeroInitial = boolValue(parameters["dir2_zero_initial"])
	}
	zeroInitialMode := resolveStrategyZeroInitialMode(zeroInitial, parameters["zero_initial_mode"])
	if strings.EqualFold(proposal.Role, "entry") && zeroInitial && zeroInitialMode == strategyZeroInitialModePosition {
		if reasonTag == "initial" || reasonTag == "livesignalbootstrap" {
			proposal.Status = "virtual-initial"
			proposal.Metadata = cloneMetadata(proposal.Metadata)
			proposal.Metadata["virtualPosition"] = true
			proposal.Metadata["virtualReason"] = "dir2-zero-initial"
			return proposal
		}
	}
	if strings.EqualFold(proposal.Role, "exit") && boolValue(mapValue(session.State["virtualPosition"])["virtual"]) {
		proposal.Status = "virtual-exit"
		proposal.Metadata = cloneMetadata(proposal.Metadata)
		proposal.Metadata["virtualExit"] = true
		return proposal
	}
	return proposal
}

func normalizeLiveSessionOverrides(overrides map[string]any) map[string]any {
	normalized := normalizePaperSessionOverrides(overrides)
	if normalized == nil {
		normalized = map[string]any{}
	}
	normalizeExecutionProfileOverrides := func(prefix string) {
		if orderType := strings.TrimSpace(stringValue(overrides[prefix+"OrderType"])); orderType != "" {
			normalized[prefix+"OrderType"] = strings.ToUpper(orderType)
		}
		if tif := strings.TrimSpace(stringValue(overrides[prefix+"TimeInForce"])); tif != "" {
			normalized[prefix+"TimeInForce"] = strings.ToUpper(tif)
		}
		if _, ok := overrides[prefix+"PostOnly"]; ok {
			normalized[prefix+"PostOnly"] = boolValue(overrides[prefix+"PostOnly"])
		}
		if maxSpread := parseFloatValue(overrides[prefix+"MaxSpreadBps"]); maxSpread > 0 {
			normalized[prefix+"MaxSpreadBps"] = maxSpread
		}
		if mode := strings.TrimSpace(stringValue(overrides[prefix+"WideSpreadMode"])); mode != "" {
			normalized[prefix+"WideSpreadMode"] = mode
		}
		if seconds := maxIntValue(overrides[prefix+"RestingTimeoutSeconds"], 0); seconds > 0 {
			normalized[prefix+"RestingTimeoutSeconds"] = seconds
		}
		if orderType := strings.TrimSpace(stringValue(overrides[prefix+"TimeoutFallbackOrderType"])); orderType != "" {
			normalized[prefix+"TimeoutFallbackOrderType"] = strings.ToUpper(orderType)
		}
		if tif := strings.TrimSpace(stringValue(overrides[prefix+"TimeoutFallbackTimeInForce"])); tif != "" {
			normalized[prefix+"TimeoutFallbackTimeInForce"] = strings.ToUpper(tif)
		}
	}
	if quantity := parseFloatValue(overrides["defaultOrderQuantity"]); quantity > 0 {
		normalized["defaultOrderQuantity"] = quantity
	}
	if _, ok := overrides["positionSizingMode"]; ok {
		if mode := normalizePositionSizingMode(overrides["positionSizingMode"]); mode != "" {
			normalized["positionSizingMode"] = mode
		}
	}
	if fraction := parseFloatValue(overrides["defaultOrderFraction"]); fraction > 0 {
		normalized["defaultOrderFraction"] = fraction
	}
	if strategy := strings.TrimSpace(stringValue(overrides["executionStrategy"])); strategy != "" {
		normalized["executionStrategy"] = strategy
	}
	if orderType := strings.TrimSpace(stringValue(overrides["executionOrderType"])); orderType != "" {
		normalized["executionOrderType"] = orderType
	}
	if tif := strings.TrimSpace(stringValue(overrides["executionTimeInForce"])); tif != "" {
		normalized["executionTimeInForce"] = strings.ToUpper(tif)
	}
	if _, ok := overrides["executionPostOnly"]; ok {
		normalized["executionPostOnly"] = boolValue(overrides["executionPostOnly"])
	}
	if mode := strings.TrimSpace(stringValue(overrides["executionWideSpreadMode"])); mode != "" {
		normalized["executionWideSpreadMode"] = mode
	}
	if seconds := maxIntValue(overrides["executionRestingTimeoutSeconds"], 0); seconds > 0 {
		normalized["executionRestingTimeoutSeconds"] = seconds
	}
	if orderType := strings.TrimSpace(stringValue(overrides["executionTimeoutFallbackOrderType"])); orderType != "" {
		normalized["executionTimeoutFallbackOrderType"] = strings.ToUpper(orderType)
	}
	if tif := strings.TrimSpace(stringValue(overrides["executionTimeoutFallbackTimeInForce"])); tif != "" {
		normalized["executionTimeoutFallbackTimeInForce"] = strings.ToUpper(tif)
	}
	if maxSpread := parseFloatValue(overrides["executionMaxSpreadBps"]); maxSpread > 0 {
		normalized["executionMaxSpreadBps"] = maxSpread
	}
	normalizeExecutionProfileOverrides("executionEntry")
	normalizeExecutionProfileOverrides("executionPTExit")
	normalizeExecutionProfileOverrides("executionSLExit")
	if mode := strings.TrimSpace(stringValue(overrides["dispatchMode"])); mode != "" {
		normalized["dispatchMode"] = mode
	}
	if seconds := maxIntValue(overrides["dispatchCooldownSeconds"], 0); seconds > 0 {
		normalized["dispatchCooldownSeconds"] = seconds
	}
	return normalized
}

func summarizeLiveAccountLatestOrder(orders []domain.Order) map[string]any {
	if len(orders) == 0 {
		return map[string]any{}
	}
	latest := orders[0]
	for _, item := range orders[1:] {
		if item.CreatedAt.After(latest.CreatedAt) {
			latest = item
		}
	}
	return map[string]any{
		"id":        latest.ID,
		"symbol":    latest.Symbol,
		"side":      latest.Side,
		"type":      latest.Type,
		"status":    latest.Status,
		"quantity":  latest.Quantity,
		"price":     latest.Price,
		"createdAt": latest.CreatedAt.Format(time.RFC3339),
	}
}

func summarizeLiveAccountLatestFill(fills []domain.Fill, orderByID map[string]domain.Order) map[string]any {
	if len(fills) == 0 {
		return map[string]any{}
	}
	latest := fills[0]
	for _, item := range fills[1:] {
		if item.CreatedAt.After(latest.CreatedAt) {
			latest = item
		}
	}
	order := orderByID[latest.OrderID]
	return map[string]any{
		"orderId":    latest.OrderID,
		"symbol":     order.Symbol,
		"side":       order.Side,
		"price":      latest.Price,
		"quantity":   latest.Quantity,
		"fee":        latest.Fee,
		"createdAt":  latest.CreatedAt.Format(time.RFC3339),
		"orderState": order.Status,
	}
}

func summarizeLiveAccountPositions(positions []domain.Position) []map[string]any {
	items := make([]map[string]any, 0, len(positions))
	for _, position := range positions {
		items = append(items, map[string]any{
			"id":         position.ID,
			"symbol":     position.Symbol,
			"side":       position.Side,
			"quantity":   position.Quantity,
			"entryPrice": position.EntryPrice,
			"markPrice":  position.MarkPrice,
			"updatedAt":  position.UpdatedAt.Format(time.RFC3339),
		})
	}
	return items
}

func buildLiveIntentSignature(intent map[string]any) string {
	return strings.Join([]string{
		stringValue(intent["action"]),
		stringValue(intent["side"]),
		NormalizeSymbol(stringValue(intent["symbol"])),
		stringValue(intent["signalKind"]),
		stringValue(intent["signalBarStateKey"]),
	}, "|")
}

func shouldAutoDispatchLiveIntent(session domain.LiveSession, intent map[string]any, eventTime time.Time) bool {
	if len(intent) == 0 {
		return false
	}
	if strings.TrimSpace(stringValue(session.State["dispatchMode"])) != "auto-"+"dispatch" {
		return false
	}
	currentOrderStatus := strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(session.State["lastSyncedOrderStatus"]), stringValue(session.State["lastDispatchedOrderStatus"]))))
	if currentOrderStatus != "" && !isTerminalOrderStatus(currentOrderStatus) {
		return false
	}
	signature := buildLiveIntentSignature(intent)
	if signature == "" {
		return false
	}
	lastSignature := stringValue(session.State["lastDispatchedIntentSignature"])
	if signature != "" && signature == lastSignature {
		if strings.EqualFold(stringValue(session.State["lastExecutionTimeoutIntentSignature"]), signature) &&
			isTerminalOrderStatus(currentOrderStatus) {
			return true
		}
		if currentOrderStatus != "" && !isTerminalOrderStatus(currentOrderStatus) {
			return false
		}
		lastDispatchedAt := parseOptionalRFC3339(stringValue(session.State["lastDispatchedAt"]))
		cooldown := time.Duration(maxIntValue(session.State["dispatchCooldownSeconds"], 30)) * time.Second
		if !lastDispatchedAt.IsZero() && eventTime.Sub(lastDispatchedAt) < cooldown {
			return false
		}
	}
	return true
}

func shouldMarkLiveExecutionFallback(order domain.Order) bool {
	if !strings.EqualFold(order.Status, "REJECTED") {
		return false
	}
	liveSubmitError := strings.ToLower(strings.TrimSpace(stringValue(order.Metadata["liveSubmitError"])))
	return strings.Contains(liveSubmitError, "\"code\":-5022") ||
		strings.Contains(liveSubmitError, "could not be executed as maker")
}
