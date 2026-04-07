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
			return synced, nil
		}
	}
	return p.syncLiveAccountFromLocalState(account, binding)
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

	intent := cloneMetadata(mapValue(session.State["lastStrategyIntent"]))
	if len(intent) == 0 {
		return domain.Order{}, fmt.Errorf("live session %s has no ready intent", session.ID)
	}
	side := strings.ToUpper(strings.TrimSpace(stringValue(intent["side"])))
	orderType := strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(intent["type"]), "MARKET")))
	symbol := NormalizeSymbol(firstNonEmpty(stringValue(intent["symbol"]), stringValue(session.State["symbol"])))
	priceHint := parseFloatValue(intent["priceHint"])
	quantity := firstPositive(parseFloatValue(intent["quantity"]), firstPositive(parseFloatValue(session.State["defaultOrderQuantity"]), 0.001))

	version, err := p.resolveCurrentStrategyVersion(session.StrategyID)
	if err != nil {
		return domain.Order{}, err
	}
	order := domain.Order{
		AccountID:         session.AccountID,
		StrategyVersionID: version.ID,
		Symbol:            symbol,
		Side:              side,
		Type:              orderType,
		Quantity:          quantity,
		Price:             priceHint,
		Metadata: map[string]any{
			"source":        "live-session-intent",
			"liveSessionId": session.ID,
			"signalKind":    stringValue(intent["signalKind"]),
			"dispatchMode":  stringValue(session.State["dispatchMode"]),
			"intent":        cloneMetadata(intent),
		},
	}
	created, err := p.CreateOrder(order)
	if err != nil {
		return domain.Order{}, err
	}

	state := cloneMetadata(session.State)
	intentSignature := buildLiveIntentSignature(intent)
	dispatchedAt := time.Now().UTC()
	state["lastDispatchedOrderId"] = created.ID
	state["lastDispatchedOrderStatus"] = created.Status
	state["lastDispatchedAt"] = dispatchedAt.Format(time.RFC3339)
	state["lastDispatchedIntent"] = intent
	state["lastDispatchedIntentSignature"] = intentSignature
	state["planIndex"] = resolveNextLivePlanIndex(state)
	state["lastEventTime"] = firstNonEmpty(stringValue(intent["plannedEventAt"]), dispatchedAt.Format(time.RFC3339))
	state["lastEventSide"] = created.Side
	state["lastEventRole"] = stringValue(intent["role"])
	state["lastEventReason"] = stringValue(intent["reason"])
	delete(state, "lastStrategyIntent")
	delete(state, "lastAutoDispatchError")
	appendTimelineEvent(state, "order", dispatchedAt, "live-intent-dispatched", map[string]any{
		"orderId": created.ID,
		"side":    created.Side,
		"symbol":  created.Symbol,
		"price":   created.Price,
		"role":    stringValue(intent["role"]),
		"reason":  stringValue(intent["reason"]),
	})
	updatedSession, _ := p.store.UpdateLiveSessionState(session.ID, state)
	if updatedSession.ID != "" {
		_, _ = p.syncLatestLiveSessionOrder(updatedSession, time.Now().UTC())
	}
	return created, nil
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

	intent := deriveLiveSessionIntent(decision, executionContext.Symbol)
	state["lastStrategyDecision"] = map[string]any{
		"action":   decision.Action,
		"reason":   decision.Reason,
		"metadata": cloneMetadata(decision.Metadata),
	}
	state["lastStrategyEvaluationContext"] = map[string]any{
		"strategyVersionId":   executionContext.StrategyVersionID,
		"signalTimeframe":     executionContext.SignalTimeframe,
		"executionDataSource": executionContext.ExecutionDataSource,
		"symbol":              executionContext.Symbol,
	}
	if intent != nil {
		state["lastStrategyIntent"] = intent
		state["lastStrategyIntentSignature"] = buildLiveIntentSignature(intent)
	} else {
		delete(state, "lastStrategyIntent")
		delete(state, "lastStrategyIntentSignature")
	}
	appendTimelineEvent(state, "strategy", eventTime, "decision", map[string]any{
		"action":        decision.Action,
		"reason":        decision.Reason,
		"decisionState": stringValue(decision.Metadata["decisionState"]),
		"signalKind":    stringValue(decision.Metadata["signalKind"]),
		"intent":        cloneMetadata(intent),
	})
	if intent != nil {
		state["lastStrategyEvaluationStatus"] = "intent-ready"
	} else if decision.Action == "advance-plan" {
		state["lastStrategyEvaluationStatus"] = "monitoring"
	} else {
		state["lastStrategyEvaluationStatus"] = "waiting-decision"
	}
	updatedSession, err := p.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		return err
	}
	if !shouldAutoDispatchLiveIntent(updatedSession, intent, eventTime) {
		return nil
	}
	if _, err := p.dispatchLiveSessionIntent(updatedSession); err != nil {
		state = cloneMetadata(updatedSession.State)
		state["lastAutoDispatchError"] = err.Error()
		state["lastAutoDispatchAttemptAt"] = eventTime.UTC().Format(time.RFC3339)
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
	if isTerminalOrderStatus(order.Status) {
		return session, nil
	}
	syncedOrder, err := p.SyncLiveOrder(order.ID)
	state := cloneMetadata(session.State)
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
	appendTimelineEvent(state, "order", eventTime, "live-order-synced", map[string]any{
		"orderId": syncedOrder.ID,
		"status":  syncedOrder.Status,
		"price":   syncedOrder.Price,
	})
	return p.store.UpdateLiveSessionState(session.ID, state)
}

func isTerminalOrderStatus(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "FILLED", "CANCELLED", "REJECTED":
		return true
	default:
		return false
	}
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
	currentPosition, _, err := p.resolvePaperSessionPositionSnapshot(session.AccountID, executionContext.Symbol)
	if err != nil {
		return executionContext, StrategySignalDecision{}, err
	}
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

	if !p.hasExecutionDataset(stringValue(parameters["executionDataSource"]), stringValue(parameters["symbol"])) {
		return domain.LiveSession{}, nil, fmt.Errorf("no %s dataset found for symbol %s", stringValue(parameters["executionDataSource"]), stringValue(parameters["symbol"]))
	}

	semantics := defaultExecutionSemantics(ExecutionModeLive, parameters)
	result, err := engine.Run(StrategyExecutionContext{
		StrategyEngineKey:   engineKey,
		StrategyVersionID:   version.ID,
		SignalTimeframe:     stringValue(parameters["signalTimeframe"]),
		ExecutionDataSource: stringValue(parameters["executionDataSource"]),
		Symbol:              stringValue(parameters["symbol"]),
		From:                parseOptionalRFC3339(stringValue(parameters["from"])),
		To:                  parseOptionalRFC3339(stringValue(parameters["to"])),
		Parameters:          parameters,
		Semantics:           semantics,
	})
	if err != nil {
		return domain.LiveSession{}, nil, err
	}

	trades, err := executionTradesFromResult(result)
	if err != nil {
		return domain.LiveSession{}, nil, err
	}
	plan, err := buildPaperExecutionPlan(domain.PaperSession{ID: session.ID, StrategyID: session.StrategyID}, version, engineKey, semantics, trades)
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
	positionSnapshot, foundPosition, positionErr := p.resolvePaperSessionPositionSnapshot(session.AccountID, stringValue(parameters["symbol"]))
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
	if !found || parseFloatValue(position["quantity"]) <= 0 {
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

func deriveLiveSessionIntent(decision StrategySignalDecision, symbol string) map[string]any {
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

	return map[string]any{
		"action":            role,
		"role":              role,
		"reason":            reason,
		"side":              nextSide,
		"type":              "MARKET",
		"symbol":            NormalizeSymbol(symbol),
		"priceHint":         marketPrice,
		"priceSource":       marketSource,
		"quantity":          quantity,
		"signalKind":        signalKind,
		"decisionState":     decisionState,
		"signalBarStateKey": signalBarStateKey,
		"entryProximityBps": entryProximityBps,
		"spreadBps":         spreadBps,
		"ma20":              ma20,
		"atr14":             atr14,
		"liquidityBias":     liquidityBias,
		"biasActionable":    biasActionable,
		"bestBid":           bestBid,
		"bestAsk":           bestAsk,
		"plannedEventAt":    stringValue(meta["nextPlannedEvent"]),
		"plannedPrice":      parseFloatValue(meta["nextPlannedPrice"]),
	}
}

func normalizeLiveSessionOverrides(overrides map[string]any) map[string]any {
	normalized := normalizePaperSessionOverrides(overrides)
	if normalized == nil {
		normalized = map[string]any{}
	}
	if quantity := parseFloatValue(overrides["defaultOrderQuantity"]); quantity > 0 {
		normalized["defaultOrderQuantity"] = quantity
	}
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
		lastDispatchedAt := parseOptionalRFC3339(stringValue(session.State["lastDispatchedAt"]))
		cooldown := time.Duration(maxIntValue(session.State["dispatchCooldownSeconds"], 30)) * time.Second
		if !lastDispatchedAt.IsZero() && eventTime.Sub(lastDispatchedAt) < cooldown {
			return false
		}
	}
	return true
}
