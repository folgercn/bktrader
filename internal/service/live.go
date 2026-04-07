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

func (p *Platform) SyncLiveAccount(accountID string) (domain.Account, error) {
	account, err := p.store.GetAccount(accountID)
	if err != nil {
		return domain.Account{}, err
	}
	if !strings.EqualFold(account.Mode, "LIVE") {
		return domain.Account{}, fmt.Errorf("account %s is not a LIVE account", accountID)
	}
	_, binding, err := p.resolveLiveAdapterForAccount(account)
	if err != nil {
		return domain.Account{}, err
	}
	if normalizeLiveExecutionMode(binding["executionMode"], boolValue(binding["sandbox"])) == "rest" {
		if synced, restErr := p.syncLiveAccountFromBinance(account, binding); restErr == nil {
			p.syncLiveSessionsForAccountSnapshot(synced)
			return synced, nil
		}
	}
	synced, err := p.syncLiveAccountFromLocalState(account, binding)
	if err == nil {
		p.syncLiveSessionsForAccountSnapshot(synced)
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
		symbol := NormalizeSymbol(stringValue(session.State["symbol"]))
		if symbol == "" {
			continue
		}
		positionSnapshot, foundPosition, positionErr := p.resolvePaperSessionPositionSnapshot(account.ID, symbol)
		if positionErr != nil {
			continue
		}
		state := cloneMetadata(session.State)
		state["recoveredPosition"] = positionSnapshot
		state["hasRecoveredPosition"] = foundPosition
		state["lastRecoveredPositionAt"] = time.Now().UTC().Format(time.RFC3339)
		state["positionRecoverySource"] = "live-account-sync"
		if foundPosition {
			state["positionRecoveryStatus"] = "monitoring-open-position"
		} else {
			state["positionRecoveryStatus"] = "flat"
		}
		_, _ = p.store.UpdateLiveSessionState(session.ID, state)
	}
}

func (p *Platform) RecoverLiveTradingOnStartup(ctx context.Context) {
	accounts, err := p.ListAccounts()
	if err != nil {
		return
	}
	for _, account := range accounts {
		if ctx != nil {
			select {
			case <-ctx.Done():
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
		}
	}

	sessions, err := p.ListLiveSessions()
	if err != nil {
		return
	}
	for _, session := range sessions {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
		if !strings.EqualFold(session.Status, "RUNNING") {
			continue
		}
		recovered, recoverErr := p.recoverRunningLiveSession(session)
		if recoverErr != nil {
			state := cloneMetadata(session.State)
			state["lastRecoveryError"] = recoverErr.Error()
			state["lastRecoveryAttemptAt"] = time.Now().UTC().Format(time.RFC3339)
			_, _ = p.store.UpdateLiveSessionState(session.ID, state)
			continue
		}
		state := cloneMetadata(recovered.State)
		delete(state, "lastRecoveryError")
		state["lastRecoveryAttemptAt"] = time.Now().UTC().Format(time.RFC3339)
		state["lastRecoveryStatus"] = "recovered"
		_, _ = p.store.UpdateLiveSessionState(recovered.ID, state)
	}
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
	account, err := p.store.GetAccount(accountID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	if !strings.EqualFold(account.Mode, "LIVE") {
		return domain.LiveSession{}, fmt.Errorf("live session requires a LIVE account: %s", accountID)
	}

	session, err := p.store.CreateLiveSession(accountID, strategyID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	if len(overrides) > 0 {
		state := cloneMetadata(session.State)
		for key, value := range normalizeLiveSessionOverrides(overrides) {
			state[key] = value
		}
		session, err = p.store.UpdateLiveSessionState(session.ID, state)
		if err != nil {
			return domain.LiveSession{}, err
		}
	}
	return p.syncLiveSessionRuntime(session)
}

func (p *Platform) LaunchLiveFlow(accountID string, options LiveLaunchOptions) (LiveLaunchResult, error) {
	account, err := p.store.GetAccount(accountID)
	if err != nil {
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
	session, err := p.store.GetLiveSession(sessionID)
	if err != nil {
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
		return domain.LiveSession{}, err
	}

	session, err = p.syncLiveSessionRuntime(session)
	if err != nil {
		return domain.LiveSession{}, err
	}
	session, err = p.ensureLiveSessionSignalRuntimeStarted(session)
	if err != nil {
		return domain.LiveSession{}, err
	}
	return p.store.UpdateLiveSessionStatus(sessionID, "RUNNING")
}

func (p *Platform) StartLiveSyncDispatcher(ctx context.Context) {
	if ctx == nil {
		return
	}
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = p.syncActiveLiveSessions(time.Now().UTC())
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
	session, _ = p.refreshLiveSessionProtectionState(session)
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

func (p *Platform) refreshLiveSessionProtectionState(session domain.LiveSession) (domain.LiveSession, error) {
	account, err := p.store.GetAccount(session.AccountID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	snapshot := cloneMetadata(mapValue(account.Metadata["liveSyncSnapshot"]))
	openOrders := metadataList(snapshot["openOrders"])
	sessionSymbol := NormalizeSymbol(firstNonEmpty(stringValue(session.State["symbol"]), stringValue(session.State["lastSymbol"])))
	position, found, err := p.resolvePaperSessionPositionSnapshot(session.AccountID, sessionSymbol)
	if err != nil {
		return domain.LiveSession{}, err
	}

	protectedOrders := make([]map[string]any, 0)
	stopOrders := make([]map[string]any, 0)
	takeProfitOrders := make([]map[string]any, 0)
	for _, item := range openOrders {
		if sessionSymbol != "" && NormalizeSymbol(stringValue(item["symbol"])) != sessionSymbol {
			continue
		}
		if !isProtectionOrder(item) {
			continue
		}
		protectedOrders = append(protectedOrders, cloneMetadata(item))
		if isStopProtectionOrder(item) {
			stopOrders = append(stopOrders, cloneMetadata(item))
		}
		if isTakeProfitProtectionOrder(item) {
			takeProfitOrders = append(takeProfitOrders, cloneMetadata(item))
		}
	}

	state := cloneMetadata(session.State)
	state["recoveredPosition"] = position
	state["hasRecoveredPosition"] = found
	state["recoveredProtectionOrders"] = protectedOrders
	state["recoveredProtectionCount"] = len(protectedOrders)
	state["recoveredStopOrderCount"] = len(stopOrders)
	state["recoveredTakeProfitOrderCount"] = len(takeProfitOrders)
	state["lastProtectionRecoveryAt"] = time.Now().UTC().Format(time.RFC3339)
	state["lastProtectionRecoverySymbol"] = sessionSymbol
	state["recoveredStopOrder"] = firstMetadataOrEmpty(stopOrders)
	state["recoveredTakeProfitOrder"] = firstMetadataOrEmpty(takeProfitOrders)

	status := "flat"
	switch {
	case found && len(protectedOrders) > 0:
		status = "protected-open-position"
	case found:
		status = "unprotected-open-position"
	}
	state["positionRecoveryStatus"] = status
	state["protectionRecoveryStatus"] = status
	if found {
		appendTimelineEvent(state, "recovery", time.Now().UTC(), "live-position-recovered", map[string]any{
			"symbol":               sessionSymbol,
			"protectionCount":      len(protectedOrders),
			"stopOrderCount":       len(stopOrders),
			"takeProfitOrderCount": len(takeProfitOrders),
			"status":               status,
		})
	}
	return p.store.UpdateLiveSessionState(session.ID, state)
}

func isProtectionOrder(order map[string]any) bool {
	orderType := strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(order["origType"]), stringValue(order["type"]))))
	if boolValue(order["reduceOnly"]) || boolValue(order["closePosition"]) {
		return true
	}
	return strings.Contains(orderType, "STOP") || strings.Contains(orderType, "TAKE_PROFIT")
}

func isStopProtectionOrder(order map[string]any) bool {
	orderType := strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(order["origType"]), stringValue(order["type"]))))
	return strings.Contains(orderType, "STOP")
}

func isTakeProfitProtectionOrder(order map[string]any) bool {
	orderType := strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(order["origType"]), stringValue(order["type"]))))
	return strings.Contains(orderType, "TAKE_PROFIT")
}

func firstMetadataOrEmpty(items []map[string]any) map[string]any {
	if len(items) == 0 {
		return map[string]any{}
	}
	return cloneMetadata(items[0])
}

func (p *Platform) SyncLiveSession(sessionID string) (domain.LiveSession, error) {
	session, err := p.store.GetLiveSession(sessionID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	return p.syncLatestLiveSessionOrder(session, time.Now().UTC())
}

func (p *Platform) syncActiveLiveSessions(eventTime time.Time) error {
	sessions, err := p.ListLiveSessions()
	if err != nil {
		return err
	}
	for _, session := range sessions {
		if !strings.EqualFold(session.Status, "RUNNING") {
			continue
		}
		if strings.TrimSpace(stringValue(session.State["lastDispatchedOrderId"])) == "" {
			continue
		}
		_, _ = p.syncLatestLiveSessionOrder(session, eventTime)
	}
	return nil
}

func (p *Platform) DispatchLiveSessionIntent(sessionID string) (domain.Order, error) {
	session, err := p.store.GetLiveSession(sessionID)
	if err != nil {
		return domain.Order{}, err
	}
	return p.dispatchLiveSessionIntent(session)
}

func (p *Platform) dispatchLiveSessionIntent(session domain.LiveSession) (domain.Order, error) {
	if !strings.EqualFold(session.Status, "RUNNING") && !strings.EqualFold(session.Status, "READY") {
		return domain.Order{}, fmt.Errorf("live session %s is not dispatchable in status %s", session.ID, session.Status)
	}

	proposalMap := cloneMetadata(mapValue(firstNonEmptyMapValue(session.State["lastExecutionProposal"], session.State["lastStrategyIntent"])))
	if len(proposalMap) == 0 {
		return domain.Order{}, fmt.Errorf("live session %s has no execution proposal", session.ID)
	}
	proposal := executionProposalFromMap(proposalMap)
	if !strings.EqualFold(proposal.Status, "dispatchable") {
		return domain.Order{}, fmt.Errorf("live session %s execution proposal is not dispatchable: %s", session.ID, firstNonEmpty(proposal.Status, "unknown"))
	}

	version, err := p.resolveCurrentStrategyVersion(session.StrategyID)
	if err != nil {
		return domain.Order{}, err
	}
	order := buildLiveOrderFromExecutionProposal(session, version.ID, proposal, proposalMap)
	created, createErr := p.CreateOrder(order)
	if createErr != nil && created.ID == "" {
		return domain.Order{}, createErr
	}

	state := cloneMetadata(session.State)
	intentSignature := buildLiveIntentSignature(proposalMap)
	dispatchedAt := time.Now().UTC()
	state["lastDispatchedOrderId"] = created.ID
	state["lastDispatchedOrderStatus"] = created.Status
	if isTerminalOrderStatus(created.Status) {
		state["lastSyncedOrderId"] = created.ID
		state["lastSyncedOrderStatus"] = created.Status
	}
	state["lastDispatchedAt"] = dispatchedAt.Format(time.RFC3339)
	state["lastDispatchedIntent"] = proposalMap
	state["lastDispatchedIntentSignature"] = intentSignature
	delete(state, "lastExecutionTimeoutAt")
	delete(state, "lastExecutionTimeoutReason")
	delete(state, "lastExecutionTimeoutIntentSignature")
	if shouldAdvanceLivePlanForOrderStatus(created.Status) {
		state["planIndex"] = resolveNextLivePlanIndex(state)
		state["lastEventTime"] = firstNonEmpty(stringValue(proposalMap["plannedEventAt"]), dispatchedAt.Format(time.RFC3339))
		state["lastEventSide"] = created.Side
		state["lastEventRole"] = proposal.Role
		state["lastEventReason"] = proposal.Reason
		delete(state, "lastStrategyIntent")
		delete(state, "lastExecutionProposal")
	} else {
		state["lastDispatchRejectedAt"] = dispatchedAt.Format(time.RFC3339)
		state["lastDispatchRejectedStatus"] = created.Status
		if shouldMarkLiveExecutionFallback(created) {
			state["lastExecutionTimeoutAt"] = dispatchedAt.Format(time.RFC3339)
			state["lastExecutionTimeoutReason"] = "maker-rejected-post-only"
			state["lastExecutionTimeoutIntentSignature"] = intentSignature
		}
	}
	if createErr != nil {
		state["lastAutoDispatchError"] = createErr.Error()
		state["lastAutoDispatchAttemptAt"] = dispatchedAt.Format(time.RFC3339)
	} else {
		delete(state, "lastAutoDispatchError")
	}
	appendTimelineEvent(state, "order", dispatchedAt, "live-intent-dispatched", map[string]any{
		"orderId": created.ID,
		"side":    created.Side,
		"symbol":  created.Symbol,
		"price":   created.Price,
		"role":    proposal.Role,
		"reason":  proposal.Reason,
	})
	if strings.EqualFold(created.Status, "FILLED") {
		if _, syncErr := p.SyncLiveAccount(session.AccountID); syncErr != nil {
			state["lastSyncError"] = syncErr.Error()
		} else if positionSnapshot, foundPosition, positionErr := p.resolvePaperSessionPositionSnapshot(session.AccountID, created.Symbol); positionErr == nil {
			state["recoveredPosition"] = positionSnapshot
			state["hasRecoveredPosition"] = foundPosition
			state["lastRecoveredPositionAt"] = dispatchedAt.Format(time.RFC3339)
			state["positionRecoverySource"] = "live-order-fill-sync"
			if foundPosition {
				state["positionRecoveryStatus"] = "monitoring-open-position"
			} else {
				state["positionRecoveryStatus"] = "flat"
			}
		}
	}
	updatedSession, _ := p.store.UpdateLiveSessionState(session.ID, state)
	if updatedSession.ID != "" {
		_, _ = p.syncLatestLiveSessionOrder(updatedSession, time.Now().UTC())
	}
	if createErr != nil {
		return created, createErr
	}
	return created, nil
}

func buildLiveOrderFromExecutionProposal(session domain.LiveSession, strategyVersionID string, proposal ExecutionProposal, proposalMap map[string]any) domain.Order {
	orderType := strings.ToUpper(strings.TrimSpace(firstNonEmpty(proposal.Type, "MARKET")))
	quantity := firstPositive(parseFloatValue(session.State["defaultOrderQuantity"]), firstPositive(proposal.Quantity, 0.001))
	price := proposal.PriceHint
	if orderType != "MARKET" {
		price = firstPositive(proposal.LimitPrice, proposal.PriceHint)
	}
	return domain.Order{
		AccountID:         session.AccountID,
		StrategyVersionID: strategyVersionID,
		Symbol:            NormalizeSymbol(firstNonEmpty(proposal.Symbol, stringValue(session.State["symbol"]))),
		Side:              strings.ToUpper(strings.TrimSpace(proposal.Side)),
		Type:              orderType,
		Quantity:          quantity,
		Price:             price,
		Metadata: map[string]any{
			"source":             "live-session-intent",
			"liveSessionId":      session.ID,
			"signalKind":         proposal.SignalKind,
			"dispatchMode":       stringValue(session.State["dispatchMode"]),
			"timeInForce":        proposal.TimeInForce,
			"postOnly":           proposal.PostOnly,
			"reduceOnly":         proposal.ReduceOnly,
			"executionStrategy":  proposal.ExecutionStrategy,
			"executionExpiresAt": stringValue(proposal.Metadata["executionExpiresAt"]),
			"executionProposal":  cloneMetadata(proposalMap),
			"intent":             cloneMetadata(proposalMap),
		},
	}
}

func (p *Platform) applyLiveVirtualInitialEvent(session domain.LiveSession, proposalMap map[string]any, eventTime time.Time) (domain.LiveSession, error) {
	proposal := executionProposalFromMap(proposalMap)
	state := cloneMetadata(session.State)
	intentSignature := buildLiveIntentSignature(proposalMap)
	entryPrice := firstPositive(
		parseFloatValue(proposalMap["plannedPrice"]),
		firstPositive(
			parseFloatValue(proposalMap["priceHint"]),
			firstPositive(
				parseFloatValue(mapValue(proposalMap["metadata"])["bestAsk"]),
				parseFloatValue(mapValue(proposalMap["metadata"])["bestBid"]),
			),
		),
	)
	virtualSide := "LONG"
	if strings.EqualFold(proposal.Side, "SELL") || strings.EqualFold(proposal.Side, "SHORT") {
		virtualSide = "SHORT"
	}
	state["lastDispatchedAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastDispatchedIntent"] = cloneMetadata(proposalMap)
	state["lastDispatchedIntentSignature"] = intentSignature
	state["lastDispatchedOrderStatus"] = liveOrderStatusVirtualInitial
	state["lastSyncedOrderStatus"] = liveOrderStatusVirtualInitial
	state["lastVirtualSignalAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastVirtualSignalType"] = "initial"
	state["virtualPosition"] = map[string]any{
		"found":      true,
		"virtual":    true,
		"symbol":     NormalizeSymbol(proposal.Symbol),
		"side":       virtualSide,
		"quantity":   0.0,
		"entryPrice": entryPrice,
		"markPrice":  entryPrice,
		"reason":     proposal.Reason,
		"recordedAt": eventTime.UTC().Format(time.RFC3339),
	}
	state["planIndex"] = resolveNextLivePlanIndex(state)
	state["lastEventTime"] = firstNonEmpty(stringValue(proposalMap["plannedEventAt"]), eventTime.UTC().Format(time.RFC3339))
	state["lastEventSide"] = proposal.Side
	state["lastEventRole"] = proposal.Role
	state["lastEventReason"] = proposal.Reason
	delete(state, "lastStrategyIntent")
	delete(state, "lastExecutionProposal")
	delete(state, "lastAutoDispatchError")
	appendTimelineEvent(state, "strategy", eventTime, "live-virtual-initial-recorded", map[string]any{
		"side":       proposal.Side,
		"symbol":     proposal.Symbol,
		"entryPrice": entryPrice,
		"reason":     proposal.Reason,
	})
	return p.store.UpdateLiveSessionState(session.ID, state)
}

func (p *Platform) applyLiveVirtualExitEvent(session domain.LiveSession, proposalMap map[string]any, eventTime time.Time) (domain.LiveSession, error) {
	proposal := executionProposalFromMap(proposalMap)
	state := cloneMetadata(session.State)
	intentSignature := buildLiveIntentSignature(proposalMap)
	state["lastDispatchedAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastDispatchedIntent"] = cloneMetadata(proposalMap)
	state["lastDispatchedIntentSignature"] = intentSignature
	state["lastDispatchedOrderStatus"] = liveOrderStatusVirtualExit
	state["lastSyncedOrderStatus"] = liveOrderStatusVirtualExit
	state["lastVirtualSignalAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastVirtualSignalType"] = "exit"
	delete(state, "virtualPosition")
	state["planIndex"] = resolveNextLivePlanIndex(state)
	state["lastEventTime"] = firstNonEmpty(stringValue(proposalMap["plannedEventAt"]), eventTime.UTC().Format(time.RFC3339))
	state["lastEventSide"] = proposal.Side
	state["lastEventRole"] = proposal.Role
	state["lastEventReason"] = proposal.Reason
	delete(state, "lastStrategyIntent")
	delete(state, "lastExecutionProposal")
	delete(state, "lastAutoDispatchError")
	appendTimelineEvent(state, "strategy", eventTime, "live-virtual-exit-recorded", map[string]any{
		"side":   proposal.Side,
		"symbol": proposal.Symbol,
		"reason": proposal.Reason,
	})
	return p.store.UpdateLiveSessionState(session.ID, state)
}

func (p *Platform) StopLiveSession(sessionID string) (domain.LiveSession, error) {
	session, err := p.store.UpdateLiveSessionStatus(sessionID, "STOPPED")
	if err != nil {
		return domain.LiveSession{}, err
	}
	_, _ = p.stopLinkedLiveSignalRuntime(session)
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
	if signalIntent != nil {
		state["lastSignalIntent"] = signalIntentToMap(*signalIntent)
		proposal, proposalErr := p.buildLiveExecutionProposal(session, executionContext, summary, sourceStates, eventTime, *signalIntent)
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
		intent = executionProposal
		state["lastStrategyIntent"] = executionProposal
		state["lastStrategyIntentSignature"] = buildLiveIntentSignature(executionProposal)
	} else {
		delete(state, "lastSignalIntent")
		delete(state, "lastExecutionProposal")
		delete(state, "lastStrategyIntent")
		delete(state, "lastStrategyIntentSignature")
	}
	appendTimelineEvent(state, "strategy", eventTime, "decision", map[string]any{
		"action":        decision.Action,
		"reason":        decision.Reason,
		"decisionState": stringValue(decision.Metadata["decisionState"]),
		"signalKind":    stringValue(decision.Metadata["signalKind"]),
		"signalIntent":  cloneMetadata(mapValue(state["lastSignalIntent"])),
		"intent":        cloneMetadata(intent),
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

func (p *Platform) syncLatestLiveSessionOrder(session domain.LiveSession, eventTime time.Time) (domain.LiveSession, error) {
	orderID := stringValue(session.State["lastDispatchedOrderId"])
	if strings.TrimSpace(orderID) == "" {
		return session, nil
	}
	order, err := p.GetOrder(orderID)
	if err != nil {
		return session, err
	}
	state := cloneMetadata(session.State)
	if isTerminalOrderStatus(order.Status) {
		state["lastSyncedOrderId"] = order.ID
		state["lastSyncedOrderStatus"] = order.Status
		state["lastDispatchedOrderStatus"] = order.Status
		if strings.EqualFold(order.Status, "FILLED") {
			if _, syncErr := p.SyncLiveAccount(session.AccountID); syncErr == nil {
				if positionSnapshot, foundPosition, positionErr := p.resolvePaperSessionPositionSnapshot(session.AccountID, order.Symbol); positionErr == nil {
					state["recoveredPosition"] = positionSnapshot
					state["hasRecoveredPosition"] = foundPosition
					state["lastRecoveredPositionAt"] = eventTime.UTC().Format(time.RFC3339)
					state["positionRecoverySource"] = "live-order-sync"
					if foundPosition {
						state["positionRecoveryStatus"] = "monitoring-open-position"
					} else {
						state["positionRecoveryStatus"] = "flat"
					}
				}
			}
		}
		return p.store.UpdateLiveSessionState(session.ID, state)
	}
	if shouldCancelLiveOrderForExecutionTimeout(order, eventTime) {
		cancelledOrder, cancelErr := p.CancelLiveOrder(order.ID)
		state["lastSyncAttemptAt"] = eventTime.UTC().Format(time.RFC3339)
		if cancelErr != nil {
			state["lastSyncError"] = cancelErr.Error()
			appendTimelineEvent(state, "order", eventTime, "live-order-cancel-error", map[string]any{
				"orderId": order.ID,
				"error":   cancelErr.Error(),
			})
			updated, updateErr := p.store.UpdateLiveSessionState(session.ID, state)
			if updateErr != nil {
				return domain.LiveSession{}, updateErr
			}
			return updated, cancelErr
		}
		delete(state, "lastSyncError")
		state["lastSyncedOrderId"] = cancelledOrder.ID
		state["lastSyncedOrderStatus"] = cancelledOrder.Status
		state["lastDispatchedOrderStatus"] = cancelledOrder.Status
		state["lastSyncedAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastExecutionTimeoutAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastExecutionTimeoutReason"] = "resting-order-expired"
		timeoutSignature := buildLiveIntentSignature(mapValue(order.Metadata["executionProposal"]))
		if timeoutSignature == "" {
			timeoutSignature = buildLiveIntentSignature(mapValue(order.Metadata["intent"]))
		}
		if timeoutSignature != "" {
			state["lastExecutionTimeoutIntentSignature"] = timeoutSignature
		}
		appendTimelineEvent(state, "order", eventTime, "live-order-cancelled-timeout", map[string]any{
			"orderId":     cancelledOrder.ID,
			"status":      cancelledOrder.Status,
			"expiredAt":   stringValue(order.Metadata["executionExpiresAt"]),
			"orderType":   cancelledOrder.Type,
			"postOnly":    boolValue(order.Metadata["postOnly"]),
			"timeInForce": stringValue(order.Metadata["timeInForce"]),
		})
		return p.store.UpdateLiveSessionState(session.ID, state)
	}
	syncedOrder, err := p.SyncLiveOrder(order.ID)
	state["lastSyncAttemptAt"] = eventTime.UTC().Format(time.RFC3339)
	if err != nil {
		state["lastSyncError"] = err.Error()
		appendTimelineEvent(state, "order", eventTime, "live-order-sync-error", map[string]any{
			"orderId": order.ID,
			"error":   err.Error(),
		})
		updated, updateErr := p.store.UpdateLiveSessionState(session.ID, state)
		if updateErr != nil {
			return domain.LiveSession{}, updateErr
		}
		return updated, err
	}
	delete(state, "lastSyncError")
	state["lastSyncedOrderId"] = syncedOrder.ID
	state["lastSyncedOrderStatus"] = syncedOrder.Status
	state["lastDispatchedOrderStatus"] = syncedOrder.Status
	state["lastSyncedAt"] = time.Now().UTC().Format(time.RFC3339)
	if strings.EqualFold(syncedOrder.Status, "FILLED") {
		if _, syncErr := p.SyncLiveAccount(session.AccountID); syncErr == nil {
			if positionSnapshot, foundPosition, positionErr := p.resolvePaperSessionPositionSnapshot(session.AccountID, syncedOrder.Symbol); positionErr == nil {
				state["recoveredPosition"] = positionSnapshot
				state["hasRecoveredPosition"] = foundPosition
				state["lastRecoveredPositionAt"] = eventTime.UTC().Format(time.RFC3339)
				state["positionRecoverySource"] = "live-order-sync"
				if foundPosition {
					state["positionRecoveryStatus"] = "monitoring-open-position"
				} else {
					state["positionRecoveryStatus"] = "flat"
				}
			}
		}
	}
	appendTimelineEvent(state, "order", eventTime, "live-order-synced", map[string]any{
		"orderId": syncedOrder.ID,
		"status":  syncedOrder.Status,
		"price":   syncedOrder.Price,
	})
	return p.store.UpdateLiveSessionState(session.ID, state)
}

func isTerminalOrderStatus(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "FILLED", "CANCELLED", "REJECTED", liveOrderStatusVirtualInitial, liveOrderStatusVirtualExit:
		return true
	default:
		return false
	}
}

func shouldCancelLiveOrderForExecutionTimeout(order domain.Order, eventTime time.Time) bool {
	if isTerminalOrderStatus(order.Status) {
		return false
	}
	expiresAt := parseOptionalRFC3339(stringValue(order.Metadata["executionExpiresAt"]))
	if expiresAt.IsZero() {
		return false
	}
	return !eventTime.UTC().Before(expiresAt.UTC())
}

func shouldAdvanceLivePlanForOrderStatus(status string) bool {
	return !strings.EqualFold(strings.TrimSpace(status), "REJECTED")
}

func firstNonEmptyMapValue(values ...any) map[string]any {
	for _, value := range values {
		if resolved := cloneMetadata(mapValue(value)); len(resolved) > 0 {
			return resolved
		}
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
	state["lastRecoveredPositionAt"] = time.Now().UTC().Format(time.RFC3339)
	state["positionRecoverySource"] = "platform-position-store"
	state["positionRecoveryStatus"] = "flat"
	if foundPosition {
		state["positionRecoveryStatus"] = "monitoring-open-position"
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
	if !found || (parseFloatValue(position["quantity"]) <= 0 && !virtualFound) {
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
	livePositionState := cloneMetadata(mapValue(session.State["livePositionState"]))
	if len(livePositionState) > 0 {
		liveSymbol := NormalizeSymbol(firstNonEmpty(stringValue(livePositionState["symbol"]), symbol))
		if liveSymbol == NormalizeSymbol(symbol) {
			mergedPosition := cloneMetadata(positionSnapshot)
			for key, value := range livePositionState {
				mergedPosition[key] = value
			}
			positionSnapshot = mergedPosition
		}
	}
	if foundPosition || parseFloatValue(positionSnapshot["quantity"]) > 0 {
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
	virtualPosition["found"] = true
	virtualPosition["virtual"] = true
	virtualPosition["symbol"] = virtualSymbol
	return virtualPosition, true, nil
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
		},
	}
}

func (p *Platform) buildLiveExecutionProposal(session domain.LiveSession, executionContext StrategyExecutionContext, summary map[string]any, sourceStates map[string]any, eventTime time.Time, intent SignalIntent) (ExecutionProposal, error) {
	strategy, _, err := p.resolveExecutionStrategy(executionContext.Parameters)
	if err != nil {
		return ExecutionProposal{}, err
	}
	proposal, err := strategy.BuildProposal(ExecutionPlanningContext{
		Session:        session,
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
	if strings.TrimSpace(stringValue(session.State["dispatchMode"])) != "auto-dispatch" {
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
