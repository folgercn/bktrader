package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type LiveLaunchOptions struct {
	StrategyID            string         `json:"strategyId"`
	Binding               map[string]any `json:"binding,omitempty"`
	LiveSessionOverrides  map[string]any `json:"liveSessionOverrides,omitempty"`
	MirrorStrategySignals bool           `json:"mirrorStrategySignals"`
	StartRuntime          bool           `json:"startRuntime"`
	StartSession          bool           `json:"startSession"`
}

type LiveLaunchResult struct {
	Account               domain.Account              `json:"account"`
	RuntimeSession        domain.SignalRuntimeSession `json:"runtimeSession"`
	LiveSession           domain.LiveSession          `json:"liveSession"`
	MirroredBindingCount  int                         `json:"mirroredBindingCount"`
	AccountBindingApplied bool                        `json:"accountBindingApplied"`
	RuntimeSessionCreated bool                        `json:"runtimeSessionCreated"`
	RuntimeSessionStarted bool                        `json:"runtimeSessionStarted"`
	LiveSessionCreated    bool                        `json:"liveSessionCreated"`
	LiveSessionStarted    bool                        `json:"liveSessionStarted"`
}

const (
	liveOrderStatusVirtualInitial = "VIRTUAL_INITIAL"
	liveOrderStatusVirtualExit    = "VIRTUAL_EXIT"
)

func (p *Platform) ListLiveSessions() ([]domain.LiveSession, error) {
	return p.store.ListLiveSessions()
}

func (p *Platform) DeleteLiveSession(sessionID string) error {
	p.logger("service.live", "session_id", sessionID).Info("deleting live session")
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
	account, err := p.store.GetAccount(accountID)
	if err != nil {
		logger.Warn("load live account failed", "error", err)
		return domain.Account{}, err
	}
	if !strings.EqualFold(account.Mode, "LIVE") {
		return domain.Account{}, fmt.Errorf("account %s is not a LIVE account", accountID)
	}
	adapter, binding, err := p.resolveLiveAdapterForAccount(account)
	if err != nil {
		return domain.Account{}, err
	}
	if syncCapable, ok := adapter.(LiveAccountSyncAdapter); ok {
		if synced, syncErr := syncCapable.SyncAccountSnapshot(p, account, binding); syncErr == nil {
			p.syncLiveSessionsForAccountSnapshot(synced)
			logger.Info("live account synced via adapter", "exchange", synced.Exchange, "status", synced.Status)
			return synced, nil
		} else {
			logger.Warn("adapter live account sync failed, falling back to local state", "error", syncErr)
		}
	}
	synced, err := p.syncLiveAccountFromLocalState(account, binding)
	if err == nil {
		p.syncLiveSessionsForAccountSnapshot(synced)
		logger.Debug("live account synced from local state", "status", synced.Status)
	} else {
		logger.Warn("local-state live account sync failed", "error", err)
	}
	return synced, err
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
	account.Metadata["lastLiveSyncAt"] = syncedAt.Format(time.RFC3339)
	return p.store.UpdateAccount(account)
}

func (p *Platform) syncLiveAccountFromBinance(account domain.Account, binding map[string]any) (domain.Account, error) {
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
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveSyncSnapshot"] = map[string]any{
		"source":                "binance-rest-account-v3",
		"adapterKey":            normalizeLiveAdapterKey(stringValue(binding["adapterKey"])),
		"syncedAt":              time.Now().UTC().Format(time.RFC3339),
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
	account.Metadata["lastLiveSyncAt"] = time.Now().UTC().Format(time.RFC3339)
	account, err = p.store.UpdateAccount(account)
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
	logger.Info("launching live flow",
		"strategy_id", strings.TrimSpace(options.StrategyID),
		"mirror_strategy_signals", options.MirrorStrategySignals,
		"start_runtime", options.StartRuntime,
		"start_session", options.StartSession,
		"has_binding", len(options.Binding) > 0,
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

	if options.MirrorStrategySignals {
		strategyBindings, err := p.ListStrategySignalBindings(strategyID)
		if err != nil {
			return LiveLaunchResult{}, err
		}
		accountBindings, err := p.ListAccountSignalBindings(account.ID)
		if err != nil {
			return LiveLaunchResult{}, err
		}
		if len(strategyBindings) == 0 && len(accountBindings) == 0 {
			return LiveLaunchResult{}, fmt.Errorf("strategy %s has no signal bindings to mirror", strategyID)
		}
		for _, binding := range strategyBindings {
			exists := false
			for _, item := range accountBindings {
				if item.SourceKey == binding.SourceKey && item.Role == binding.Role && item.Symbol == binding.Symbol {
					exists = true
					break
				}
			}
			if exists {
				continue
			}
			account, err = p.BindAccountSignalSource(account.ID, map[string]any{
				"sourceKey": binding.SourceKey,
				"role":      binding.Role,
				"symbol":    binding.Symbol,
				"options":   binding.Options,
			})
			if err != nil {
				return LiveLaunchResult{}, err
			}
			result.Account = account
			result.MirroredBindingCount++
			accountBindings = append(accountBindings, binding)
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

	account, err = p.store.GetAccount(account.ID)
	if err == nil {
		result.Account = account
	}
	logger.Info("live flow launched",
		"strategy_id", strategyID,
		"mirrored_binding_count", result.MirroredBindingCount,
		"account_binding_applied", result.AccountBindingApplied,
		"runtime_session_created", result.RuntimeSessionCreated,
		"runtime_session_started", result.RuntimeSessionStarted,
		"live_session_created", result.LiveSessionCreated,
		"live_session_started", result.LiveSessionStarted,
	)
	return result, nil
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
	sessions, err := p.ListLiveSessions()
	if err != nil {
		return domain.LiveSession{}, false, err
	}
	for _, session := range sessions {
		if session.AccountID != accountID || session.StrategyID != strategyID {
			continue
		}
		if len(overrides) == 0 {
			return session, false, nil
		}
		state := cloneMetadata(session.State)
		for key, value := range normalizeLiveSessionOverrides(overrides) {
			state[key] = value
		}
		updated, err := p.store.UpdateLiveSessionState(session.ID, state)
		if err != nil {
			return domain.LiveSession{}, false, err
		}
		synced, err := p.syncLiveSessionRuntime(updated)
		return synced, false, err
	}
	session, err := p.CreateLiveSession(accountID, strategyID, overrides)
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
			if err := p.syncActiveLiveSessions(time.Now().UTC()); err != nil {
				logger.Warn("sync active live sessions failed", "error", err)
			}
		}
	}
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
	logger := p.logger("service.live", "session_id", sessionID)
	session, err := p.store.UpdateLiveSessionStatus(sessionID, "STOPPED")
	if err != nil {
		logger.Error("stop live session failed", "error", err)
		return domain.LiveSession{}, err
	}
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

	state := cloneMetadata(session.State)
	state["lastSignalRuntimeEventAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastSignalRuntimeEvent"] = cloneMetadata(summary)
	state["lastSignalRuntimeSessionId"] = runtimeSessionID
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
		_, err := p.store.UpdateLiveSessionState(session.ID, state)
		return err
	}

	sourceGate := map[string]any{
		"ready":   false,
		"missing": []any{},
		"stale":   []any{},
	}
	sourceStates := map[string]any{}
	signalBarStates := map[string]any{}
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
		state["lastStrategyEvaluationSourceStates"] = sourceStates
		state["lastStrategyEvaluationSignalBarStates"] = signalBarStates
		state["lastStrategyEvaluationSignalBarStateCount"] = len(signalBarStates)
		state["lastStrategyEvaluationSourceStateCount"] = len(sourceStates)
		state["lastStrategyEvaluationRuntimeSummary"] = cloneMetadata(mapValue(runtimeSession.State["lastEventSummary"]))
		sourceGate = p.evaluateRuntimeSignalSourceReadiness(session.StrategyID, runtimeSession, eventTime)
		state["lastStrategyEvaluationSourceGate"] = sourceGate
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

	executionContext, decision, err := p.evaluateLiveSignalDecision(session, summary, sourceStates, signalBarStates, eventTime, nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason)
	if err != nil {
		state["lastStrategyEvaluationStatus"] = "decision-error"
		state["lastStrategyDecision"] = map[string]any{
			"action": "error",
			"reason": err.Error(),
		}
		appendTimelineEvent(state, "strategy", eventTime, "decision-error", map[string]any{"error": err.Error()})
		_, updateErr := p.store.UpdateLiveSessionState(session.ID, state)
		if updateErr != nil {
			return updateErr
		}
		return err
	}

	signalIntent := deriveLiveSignalIntent(decision, executionContext.Symbol)
	var intent map[string]any
	var executionProposal map[string]any
	state["lastStrategyDecision"] = map[string]any{
		"action":   decision.Action,
		"reason":   decision.Reason,
		"metadata": cloneMetadata(decision.Metadata),
	}
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

func (p *Platform) evaluateLiveSignalDecision(session domain.LiveSession, summary map[string]any, sourceStates map[string]any, signalBarStates map[string]any, eventTime time.Time, nextPlannedEvent time.Time, nextPlannedPrice float64, nextPlannedSide, nextPlannedRole, nextPlannedReason string) (StrategyExecutionContext, StrategySignalDecision, error) {
	version, err := p.resolveCurrentStrategyVersion(session.StrategyID)
	if err != nil {
		return StrategyExecutionContext{}, StrategySignalDecision{}, err
	}
	parameters, err := p.resolveLiveSessionParameters(session, version)
	if err != nil {
		return StrategyExecutionContext{}, StrategySignalDecision{}, err
	}
	engine, engineKey, err := p.resolveStrategyEngine(version.ID, parameters)
	if err != nil {
		return StrategyExecutionContext{}, StrategySignalDecision{}, err
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
		}, nil
	}
	currentPosition, _, err := p.resolveLiveSessionPositionSnapshot(session, executionContext.Symbol)
	if err != nil {
		return executionContext, StrategySignalDecision{}, err
	}
	nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason = alignLivePlanStepToCurrentMarket(
		signalBarStates,
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
		return executionContext, StrategySignalDecision{}, err
	}
	if strings.TrimSpace(decision.Action) == "" {
		decision.Action = "wait"
	}
	if strings.TrimSpace(decision.Reason) == "" {
		decision.Reason = "unspecified"
	}
	return executionContext, decision, nil
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
	if boolValue(currentPosition["found"]) || parseFloatValue(currentPosition["quantity"]) > 0 {
		return nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason
	}
	if !isLivePlanStepStale(nextPlannedEvent, signalTimeframe, eventTime) {
		return nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason
	}
	signalBarState, _ := pickSignalBarState(signalBarStates, NormalizeSymbol(stringValue(currentPosition["symbol"])), signalTimeframe)
	if signalBarState == nil {
		return nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason
	}
	gate := evaluateSignalBarGate(signalBarState, "", "entry")
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
	resolution := "240"
	if strings.EqualFold(strings.TrimSpace(signalTimeframe), "1d") {
		resolution = "1D"
	}
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
	} else if runtimeSessionID != "" {
		runtimeSession, getErr := p.GetSignalRuntimeSession(runtimeSessionID)
		if getErr == nil {
			state["signalRuntimeStatus"] = runtimeSession.Status
		}
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
		return session, plan, nil
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

func reconcileLivePlanIndexWithPosition(plan []paperPlannedOrder, currentIndex int, position map[string]any, found bool) (int, bool) {
	if len(plan) == 0 || currentIndex < 0 {
		return currentIndex, false
	}
	if currentIndex >= len(plan) {
		currentIndex = len(plan) - 1
	}
	virtualFound := boolValue(position["virtual"])
	if (!found && !virtualFound) || (parseFloatValue(position["quantity"]) <= 0 && !virtualFound) {
		if strings.EqualFold(plan[currentIndex].Role, "exit") {
			for i := currentIndex; i >= 0; i-- {
				if strings.EqualFold(plan[i].Role, "entry") {
					return i, true
				}
			}
		}
		return currentIndex, false
	}
	if strings.EqualFold(plan[currentIndex].Role, "entry") {
		for i := currentIndex; i < len(plan); i++ {
			if strings.EqualFold(plan[i].Role, "exit") {
				return i, true
			}
		}
	}
	return currentIndex, false
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
	if strings.EqualFold(proposal.Role, "entry") && zeroInitial {
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
