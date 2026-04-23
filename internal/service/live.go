package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"sync"
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
	liveOrderStatusVirtualInitial             = "VIRTUAL_INITIAL"
	liveOrderStatusVirtualExit                = "VIRTUAL_EXIT"
	liveAccountReconcileSelfHealLookbackHours = 8
)

var ErrLiveAccountOperationInProgress = errors.New("live account operation already in progress")

type liveAccountOperationKind string

const (
	liveAccountOperationSync      liveAccountOperationKind = "sync"
	liveAccountOperationReconcile liveAccountOperationKind = "reconcile"
)

type liveAccountSyncState struct {
	mu          sync.Mutex
	running     bool
	done        chan struct{}
	result      domain.Account
	err         error
	completedAt time.Time
}

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

func (p *Platform) UpdateLiveSession(sessionID, alias, accountID, strategyID string, overrides map[string]any) (domain.LiveSession, error) {
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
	// Always assign alias (trimmed) to support clearing an existing alias by providing an empty string.
	session.Alias = strings.TrimSpace(alias)
	state := cloneMetadata(session.State)
	for key, value := range p.canonicalizeLiveSessionOverridesForStrategy(session.StrategyID, overrides) {
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
	return p.requestLiveAccountSync(accountID, "direct")
}

func (p *Platform) requestLiveAccountSync(accountID, trigger string) (domain.Account, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return domain.Account{}, fmt.Errorf("live account id is required")
	}
	normalizedTrigger := firstNonEmpty(strings.TrimSpace(trigger), "unspecified")
	state := p.liveAccountSyncEntry(accountID)
	logger := p.logger("service.live", "account_id", accountID, "trigger", normalizedTrigger)

	state.mu.Lock()
	if state.running {
		done := state.done
		state.mu.Unlock()
		waitStartedAt := time.Now()
		if done != nil {
			<-done
		}
		state.mu.Lock()
		result, err := state.result, state.err
		state.mu.Unlock()
		logger.Info("live account sync request reused in-flight result",
			"coalesced", true,
			"waited_for_inflight", true,
			"wait_ms", time.Since(waitStartedAt).Milliseconds(),
			"reused_recent_result", false,
			"result_error", err != nil,
		)
		return result, err
	}
	if p.liveAccountSyncAllowsRecentReuse(trigger) {
		if reuseWindow := p.liveAccountSyncReuseWindow(); reuseWindow > 0 && !state.completedAt.IsZero() {
			age := time.Since(state.completedAt)
			if age >= 0 && age < reuseWindow {
				result, err := state.result, state.err
				state.mu.Unlock()
				logger.Info("live account sync request reused recent result",
					"coalesced", false,
					"waited_for_inflight", false,
					"reused_recent_result", true,
					"age_ms", age.Milliseconds(),
					"reuse_window_ms", reuseWindow.Milliseconds(),
					"result_error", err != nil,
				)
				return result, err
			}
		}
	}
	done := make(chan struct{})
	state.running = true
	state.done = done
	state.mu.Unlock()

	release, acquired := p.tryStartLiveAccountOperation(accountID, liveAccountOperationSync)
	if !acquired {
		err := fmt.Errorf("%w: sync account=%s", ErrLiveAccountOperationInProgress, accountID)
		state.mu.Lock()
		state.result = domain.Account{}
		state.err = err
		state.running = false
		state.done = nil
		close(done)
		state.mu.Unlock()
		return domain.Account{}, err
	}
	defer release()
	logger.Info("live account sync request executing",
		"coalesced", false,
		"waited_for_inflight", false,
		"reused_recent_result", false,
	)
	result, err := p.syncLiveAccountWithoutGate(accountID)
	state.mu.Lock()
	state.result = result
	state.err = err
	state.completedAt = time.Now().UTC()
	state.running = false
	state.done = nil
	close(done)
	state.mu.Unlock()
	logger.Info("live account sync request completed",
		"coalesced", false,
		"waited_for_inflight", false,
		"reused_recent_result", false,
		"result_error", err != nil,
		"completed_at", state.completedAt.Format(time.RFC3339),
	)
	return result, err
}

func (p *Platform) liveAccountSyncEntry(accountID string) *liveAccountSyncState {
	actual, _ := p.liveAccountSyncState.LoadOrStore(strings.TrimSpace(accountID), &liveAccountSyncState{})
	entry, _ := actual.(*liveAccountSyncState)
	if entry == nil {
		entry = &liveAccountSyncState{}
		p.liveAccountSyncState.Store(strings.TrimSpace(accountID), entry)
	}
	return entry
}

func (p *Platform) liveAccountSyncReuseWindow() time.Duration {
	threshold := time.Duration(p.runtimePolicy.LiveAccountSyncFreshnessSecs) * time.Second
	switch {
	case threshold <= 0:
		return 5 * time.Second
	case threshold < 4*time.Second:
		return threshold
	default:
		window := threshold / 4
		if window > 5*time.Second {
			window = 5 * time.Second
		}
		if window < time.Second {
			window = time.Second
		}
		return window
	}
}

func (p *Platform) liveAccountSyncAllowsRecentReuse(trigger string) bool {
	switch strings.TrimSpace(trigger) {
	case "authoritative-reconcile",
		"recover-live-session",
		"live-immediate-fill-settlement",
		"live-intent-dispatched-filled",
		"live-terminal-order-sync",
		"live-filled-order-sync":
		return false
	default:
		return true
	}
}

func compactSourceGateEntries(entries []map[string]any) []map[string]any {
	if len(entries) == 0 {
		return []map[string]any{}
	}
	items := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		items = append(items, map[string]any{
			"sourceKey":   stringValue(entry["sourceKey"]),
			"role":        stringValue(entry["role"]),
			"streamType":  stringValue(entry["streamType"]),
			"symbol":      stringValue(entry["symbol"]),
			"lastEventAt": stringValue(entry["lastEventAt"]),
			"maxAgeSec":   maxIntValue(entry["maxAgeSec"], 0),
		})
	}
	return items
}

func runtimeSourceGateIssueSignature(sourceGate map[string]any) string {
	payload, err := json.Marshal(map[string]any{
		"missing": compactSourceGateEntries(metadataList(sourceGate["missing"])),
		"stale":   compactSourceGateEntries(metadataList(sourceGate["stale"])),
		"ready":   boolValue(sourceGate["ready"]),
	})
	if err != nil {
		return ""
	}
	return string(payload)
}

func (p *Platform) logRuntimeSourceGateState(strategyID string, runtimeSession domain.SignalRuntimeSession, sourceGate map[string]any, eventTime time.Time) {
	runtimeID := strings.TrimSpace(runtimeSession.ID)
	if runtimeID == "" {
		return
	}
	signature := runtimeSourceGateIssueSignature(sourceGate)
	previous, hadPrevious := p.runtimeSourceGateState.Load(runtimeID)
	previousSignature, _ := previous.(string)
	if boolValue(sourceGate["ready"]) {
		if hadPrevious && previousSignature != "" {
			p.logger("service.runtime_source_gate",
				"runtime_session_id", runtimeID,
				"account_id", runtimeSession.AccountID,
				"strategy_id", strategyID,
			).Info("runtime source gate recovered", "event_time", eventTime.Format(time.RFC3339))
			p.runtimeSourceGateState.Delete(runtimeID)
		}
		return
	}
	if signature != "" && hadPrevious && previousSignature == signature {
		return
	}
	p.runtimeSourceGateState.Store(runtimeID, signature)
	p.logger("service.runtime_source_gate",
		"runtime_session_id", runtimeID,
		"account_id", runtimeSession.AccountID,
		"strategy_id", strategyID,
	).Warn("runtime source gate blocked",
		"event_time", eventTime.Format(time.RFC3339),
		"missing_count", len(metadataList(sourceGate["missing"])),
		"stale_count", len(metadataList(sourceGate["stale"])),
		"missing_sources", compactSourceGateEntries(metadataList(sourceGate["missing"])),
		"stale_sources", compactSourceGateEntries(metadataList(sourceGate["stale"])),
	)
}

func (p *Platform) syncLiveAccountWithoutGate(accountID string) (domain.Account, error) {
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

func (p *Platform) tryStartLiveAccountOperation(accountID string, kind liveAccountOperationKind) (func(), bool) {
	if strings.TrimSpace(accountID) == "" {
		return func() {}, false
	}
	actual, _ := p.liveAccountOpMu.LoadOrStore(accountID, &sync.Mutex{})
	mu, _ := actual.(*sync.Mutex)
	if mu == nil || !mu.TryLock() {
		p.logger("service.live", "account_id", accountID, "operation", string(kind)).Debug("skip live account operation while another operation is in progress")
		return func() {}, false
	}
	return mu.Unlock, true
}

func liveAccountPositionReconcilePending(account domain.Account) bool {
	requiredAt := parseOptionalRFC3339(stringValue(account.Metadata["livePositionReconcileRequiredAt"]))
	if requiredAt.IsZero() {
		return false
	}
	lastSyncAt := parseOptionalRFC3339(stringValue(account.Metadata["lastLivePositionSyncAt"]))
	return lastSyncAt.IsZero() || requiredAt.After(lastSyncAt)
}

func clearLiveAccountPositionReconcileRequirement(metadata map[string]any) {
	if metadata == nil {
		return
	}
	delete(metadata, "livePositionReconcileRequiredAt")
	delete(metadata, "livePositionReconcileTrigger")
}

func (p *Platform) markLiveAccountPositionReconcileRequired(account domain.Account, trigger string, eventTime time.Time) (domain.Account, error) {
	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["livePositionReconcileRequiredAt"] = eventTime.UTC().Format(time.RFC3339)
	account.Metadata["livePositionReconcileTrigger"] = firstNonEmpty(strings.TrimSpace(trigger), "reconcile-required")
	return p.store.UpdateAccount(account)
}

func (p *Platform) triggerAuthoritativeLiveAccountReconcile(accountID, trigger string, eventTime time.Time) (domain.Account, error) {
	account, err := p.store.GetAccount(accountID)
	if err != nil {
		return domain.Account{}, err
	}
	if !strings.EqualFold(account.Mode, "LIVE") {
		return domain.Account{}, fmt.Errorf("account %s is not a LIVE account", accountID)
	}
	binding := cloneMetadata(mapValue(account.Metadata["liveBinding"]))
	if normalizeLiveExecutionMode(binding["executionMode"], boolValue(binding["sandbox"])) != "rest" {
		return account, nil
	}
	account, err = p.markLiveAccountPositionReconcileRequired(account, trigger, eventTime)
	if err != nil {
		return domain.Account{}, err
	}
	p.syncLiveSessionsForAccountSnapshot(account)

	synced, syncErr := p.requestLiveAccountSync(accountID, "authoritative-reconcile")
	if syncErr != nil {
		latest, latestErr := p.store.GetAccount(accountID)
		if latestErr == nil {
			return latest, syncErr
		}
		return account, syncErr
	}
	if liveAccountPositionReconcilePending(synced) {
		return synced, fmt.Errorf("live account %s reconcile is still pending after %s", accountID, firstNonEmpty(strings.TrimSpace(trigger), "reconcile"))
	}
	return synced, nil
}

func (p *Platform) ReconcileLiveAccount(accountID string, options LiveAccountReconcileOptions) (LiveAccountReconcileResult, error) {
	options = normalizeLiveAccountReconcileOptions(options)
	result := LiveAccountReconcileResult{
		LookbackHours: options.LookbackHours,
	}

	release, acquired := p.tryStartLiveAccountOperation(accountID, liveAccountOperationReconcile)
	if !acquired {
		return result, fmt.Errorf("live account %s sync/reconcile already in progress", accountID)
	}
	defer release()

	account, err := p.syncLiveAccountWithoutGate(accountID)
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
	snapshotPositions := liveSyncSnapshotPositionAmounts(account)

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
			reconciledOrder, created, err := p.reconcileLiveAccountExchangeOrder(account, binding, payload, tradesByExchangeOrderID[exchangeOrderID], snapshotPositions, &index)
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
	account, err = p.refreshLiveAccountPositionReconcileGate(account)
	if err != nil {
		return result, err
	}
	p.syncLiveSessionsForAccountSnapshot(account)
	result.Account = account
	return result, nil
}

func livePositionReconcileGateCanSelfHeal(gate map[string]any) bool {
	return strings.EqualFold(strings.TrimSpace(stringValue(gate["status"])), livePositionReconcileGateStatusStale) &&
		strings.EqualFold(strings.TrimSpace(stringValue(gate["scenario"])), "db-position-exchange-missing")
}

func (p *Platform) supportsLiveAccountReconcile(account domain.Account) bool {
	if !strings.EqualFold(strings.TrimSpace(account.Mode), "LIVE") {
		return false
	}
	binding := cloneMetadata(mapValue(account.Metadata["liveBinding"]))
	if normalizeLiveExecutionMode(binding["executionMode"], boolValue(binding["sandbox"])) != "rest" {
		return false
	}
	adapter, _, err := p.resolveLiveAdapterForAccount(account)
	if err != nil {
		return false
	}
	_, ok := adapter.(LiveAccountReconcileAdapter)
	return ok
}

func (p *Platform) attemptLiveAccountReconcileSelfHeal(account domain.Account, symbol string) (domain.Account, bool, error) {
	symbol = NormalizeSymbol(symbol)
	if symbol == "" {
		return account, false, nil
	}
	gate := resolveLivePositionReconcileGate(account, symbol, true)
	if !livePositionReconcileGateCanSelfHeal(gate) || !p.supportsLiveAccountReconcile(account) {
		return account, false, nil
	}
	result, err := p.ReconcileLiveAccount(account.ID, LiveAccountReconcileOptions{
		LookbackHours: liveAccountReconcileSelfHealLookbackHours,
	})
	if err != nil {
		return account, true, err
	}
	return result.Account, true, nil
}

func (p *Platform) attemptLiveExposureReconcileSelfHeal(accountID string) (bool, error) {
	account, err := p.store.GetAccount(accountID)
	if err != nil {
		return false, err
	}
	if !p.supportsLiveAccountReconcile(account) {
		return false, nil
	}
	_, err = p.ReconcileLiveAccount(accountID, LiveAccountReconcileOptions{
		LookbackHours: liveAccountReconcileSelfHealLookbackHours,
	})
	if err != nil {
		return true, err
	}
	return true, nil
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

func (p *Platform) refreshLiveAccountPositionReconcileGate(account domain.Account) (domain.Account, error) {
	snapshot := cloneMetadata(mapValue(account.Metadata["liveSyncSnapshot"]))
	exchangePositions := metadataList(snapshot["positions"])
	reconcileGate, reconcileErr := p.reconcileLiveAccountPositions(account, exchangePositions)
	if reconcileErr != nil {
		account.Metadata = cloneMetadata(account.Metadata)
		account.Metadata["lastLivePositionSyncError"] = reconcileErr.Error()
		account, _ = p.store.UpdateAccount(account)
		return account, reconcileErr
	}
	account.Metadata = cloneMetadata(account.Metadata)
	delete(account.Metadata, "lastLivePositionSyncError")
	account.Metadata["lastLivePositionSyncAt"] = time.Now().UTC().Format(time.RFC3339)
	account.Metadata["livePositionReconcileGate"] = reconcileGate
	clearLiveAccountPositionReconcileRequirement(account.Metadata)
	return p.store.UpdateAccount(account)
}

func normalizeLiveAccountReconcileOptions(options LiveAccountReconcileOptions) LiveAccountReconcileOptions {
	if options.LookbackHours <= 0 {
		options.LookbackHours = 4
	}
	if options.LookbackHours > 48 {
		options.LookbackHours = 48
	}
	return options
}

func (p *Platform) collectLiveAccountReconcileSymbols(account domain.Account, lookbackHours int) ([]string, error) {
	symbolSet := make(map[string]struct{})

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
		if session.AccountID != account.ID || !strings.EqualFold(strings.TrimSpace(session.Status), "RUNNING") {
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

func liveSyncSnapshotPositionAmounts(account domain.Account) map[string]float64 {
	snapshot := cloneMetadata(mapValue(account.Metadata["liveSyncSnapshot"]))
	amounts := make(map[string]float64)
	for _, item := range metadataList(snapshot["positions"]) {
		symbol := NormalizeSymbol(stringValue(item["symbol"]))
		if symbol == "" {
			continue
		}
		amount := parseFloatValue(item["positionAmt"])
		if amount == 0 {
			amount = parseFloatValue(item["quantity"])
		}
		amounts[symbol] = math.Abs(amount)
	}
	return amounts
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

func (p *Platform) reconcileLiveAccountExchangeOrder(account domain.Account, binding map[string]any, payload map[string]any, tradeReports []LiveFillReport, snapshotPositions map[string]float64, index *liveOrderReconcileIndex) (domain.Order, bool, error) {
	exchangeOrderID := normalizeBinanceOrderID(payload["orderId"], payload["clientOrderId"])
	clientOrderID := strings.TrimSpace(stringValue(payload["clientOrderId"]))
	if exchangeOrderID == "" {
		return domain.Order{}, false, nil
	}
	order, found := index.match(exchangeOrderID, clientOrderID)
	created := false
	var err error
	if !found {
		if skipReason := classifyLiveReconcileOrderSkip(payload, clientOrderID, snapshotPositions, index); skipReason != "" {
			p.logger("service.live",
				"account_id", account.ID,
				"symbol", NormalizeSymbol(stringValue(payload["symbol"])),
				"exchange_order_id", exchangeOrderID,
				"client_order_id", clientOrderID,
				"skip_reason", skipReason,
			).Warn("skip reconcile exchange order")
			return domain.Order{}, false, nil
		}
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

func classifyLiveReconcileOrderSkip(payload map[string]any, clientOrderID string, snapshotPositions map[string]float64, index *liveOrderReconcileIndex) string {
	status := strings.ToUpper(strings.TrimSpace(stringValue(payload["status"])))
	terminal := isTerminalOrderStatus(mapBinanceOrderStatus(status)) || isTerminalOrderStatus(status)
	executedQty := parseFloatValue(payload["executedQty"])
	isSystemOrder := strings.HasPrefix(clientOrderID, "order-")
	if !isSystemOrder && clientOrderID != "" && index != nil {
		_, isSystemOrder = index.byClientOrderID[clientOrderID]
	}
	if !isSystemOrder {
		return "non-system-order"
	}
	symbol := NormalizeSymbol(stringValue(payload["symbol"]))
	if terminal && !tradingQuantityPositive(snapshotPositions[symbol]) {
		return "closed-position-historical-order"
	}
	if (strings.EqualFold(status, "CANCELED") || strings.EqualFold(status, "CANCELLED") || strings.EqualFold(status, "REJECTED")) && !tradingQuantityPositive(executedQty) {
		return "no-fill-cancelled"
	}
	if !terminal && !tradingQuantityPositive(snapshotPositions[symbol]) {
		return "no-live-position"
	}
	return ""
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
		p.refreshLiveSessionForAccountSnapshot(session, time.Now().UTC())
	}
}

func (p *Platform) refreshLiveSessionForAccountSnapshot(session domain.LiveSession, refreshedAt time.Time) domain.LiveSession {
	updated := session
	if completed, _, _, completeErr := p.completeRecoveredLiveSessionMetadata(updated); completeErr == nil {
		updated = completed
	}
	// refreshLiveSessionPositionContext persists its own state updates; we retain
	// the returned snapshot here only to make the refresh sequence explicit.
	if refreshed, refreshErr := p.refreshLiveSessionPositionContext(updated, refreshedAt, "live-account-sync"); refreshErr == nil {
		updated = refreshed
	}
	return updated
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
		syncedAccount, syncErr := p.requestLiveAccountSync(account.ID, "recover-live-session")
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
		if isLiveSessionRecoveryCloseOnlyMode(state) {
			state["lastRecoveryStatus"] = liveRecoveryModeCloseOnlyTakeover
		} else {
			state["lastRecoveryStatus"] = "recovered"
		}
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
	accountPayload, err := binanceSignedGETWithCategory(resolved, "/fapi/v3/account", map[string]string{
		"timestamp":  fmt.Sprintf("%d", time.Now().UTC().UnixMilli()),
		"recvWindow": fmt.Sprintf("%d", maxIntValue(binding["recvWindowMs"], 5000)),
	}, binanceRESTCategoryAccountSync)
	if err != nil {
		return domain.Account{}, fmt.Errorf("binance account sync failed: %w", err)
	}
	positionRiskPayload, err := binanceSignedGETWithCategory(resolved, "/fapi/v2/positionRisk", map[string]string{
		"timestamp":  fmt.Sprintf("%d", time.Now().UTC().UnixMilli()),
		"recvWindow": fmt.Sprintf("%d", maxIntValue(binding["recvWindowMs"], 5000)),
	}, binanceRESTCategoryAccountSync)
	if err != nil {
		return domain.Account{}, fmt.Errorf("binance position risk sync failed: %w", err)
	}
	openOrdersPayload, err := binanceSignedGETWithCategory(resolved, "/fapi/v1/openOrders", map[string]string{
		"timestamp":  fmt.Sprintf("%d", time.Now().UTC().UnixMilli()),
		"recvWindow": fmt.Sprintf("%d", maxIntValue(binding["recvWindowMs"], 5000)),
	}, binanceRESTCategoryAccountSync)
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
		entryPrice := resolveRecoveredLiveEntryPrice(
			parseFloatValue(item["entryPrice"]),
			parseFloatValue(risk["entryPrice"]),
			parseFloatValue(risk["breakEvenPrice"]),
		)
		openPositions = append(openPositions, map[string]any{
			"symbol":           stringValue(item["symbol"]),
			"positionAmt":      positionAmt,
			"entryPrice":       entryPrice,
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
	return p.refreshLiveAccountPositionReconcileGate(account)
}

func (p *Platform) CreateLiveSession(alias, accountID, strategyID string, overrides map[string]any) (domain.LiveSession, error) {
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
	if strings.TrimSpace(alias) != "" {
		session.Alias = alias
		session, err = p.store.UpdateLiveSession(session)
		if err != nil {
			logger.Error("set live session alias failed", "error", err)
			return domain.LiveSession{}, err
		}
	}
	if len(overrides) > 0 {
		state := cloneMetadata(session.State)
		for key, value := range p.canonicalizeLiveSessionOverridesForStrategy(strategyID, overrides) {
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
	normalizedOverrides := p.canonicalizeLiveSessionOverridesForStrategy(strategyID, overrides)
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
		rawSessionSymbol := strings.TrimSpace(firstNonEmpty(stringValue(session.State["symbol"]), stringValue(session.State["lastSymbol"])))
		rawSessionTimeframe := strings.TrimSpace(firstNonEmpty(stringValue(session.State["signalTimeframe"]), stringValue(session.State["timeframe"])))
		sessionSymbol := normalizeOptionalLiveScopeSymbol(rawSessionSymbol)
		sessionTimeframe := normalizeSignalBarInterval(rawSessionTimeframe)
		if rawSessionSymbol != "" || rawSessionTimeframe != "" {
			sessionScope := p.canonicalizeLiveSessionOverridesForStrategy(strategyID, map[string]any{
				"symbol":          rawSessionSymbol,
				"signalTimeframe": rawSessionTimeframe,
			})
			sessionSymbol = normalizeOptionalLiveScopeSymbol(firstNonEmpty(stringValue(sessionScope["symbol"]), rawSessionSymbol))
			sessionTimeframe = normalizeSignalBarInterval(firstNonEmpty(stringValue(sessionScope["signalTimeframe"]), rawSessionTimeframe))
		}
		if targetSymbol != "" && sessionSymbol != targetSymbol {
			continue
		}
		if targetTimeframe != "" && sessionTimeframe != targetTimeframe {
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
	session, err := p.CreateLiveSession("", accountID, strategyID, normalizedOverrides)
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
	if syncedAccount, reconcileErr := p.triggerAuthoritativeLiveAccountReconcile(account.ID, "historical-takeover-activation", time.Now().UTC()); reconcileErr == nil {
		account = syncedAccount
	} else {
		if latestAccount, latestErr := p.store.GetAccount(account.ID); latestErr == nil {
			account = latestAccount
		}
		if liveAccountPositionReconcilePending(account) {
			logger.Warn("authoritative live account reconcile pending on start", "error", reconcileErr)
			symbol := NormalizeSymbol(firstNonEmpty(stringValue(session.State["symbol"]), stringValue(session.State["lastSymbol"])))
			if symbol != "" {
				if position, found, findErr := p.store.FindPosition(session.AccountID, symbol); findErr == nil && found && position.Quantity > 0 {
					gate := resolveLivePositionReconcileGate(account, symbol, true)
					blocked, blockErr := p.enterRecoveredLiveSessionReconcileGateBlocked(session, position, gate)
					if blockErr == nil {
						return blocked, fmt.Errorf("live session %s requires authoritative reconcile before historical takeover activation", session.ID)
					}
					return domain.LiveSession{}, blockErr
				}
			}
		}
	}
	if strings.TrimSpace(stringValue(session.State["lastDispatchedOrderId"])) != "" {
		if syncedSession, syncErr := p.syncLatestLiveSessionOrder(session, time.Now().UTC()); syncErr == nil {
			session = syncedSession
		}
	}
	session, recoveredPosition, incompleteRecoveryMetadata, err := p.completeRecoveredLiveSessionMetadata(session)
	if err != nil {
		logger.Warn("complete recovered live session metadata failed", "error", err)
		return domain.LiveSession{}, err
	}
	if incompleteRecoveryMetadata {
		session, err = p.enterRecoveredLiveSessionCloseOnlyMode(session, recoveredPosition, "missing-strategy-version", "recovered position is missing strategyVersionId")
		if err != nil {
			logger.Warn("enter close-only takeover mode failed", "error", err)
			return domain.LiveSession{}, err
		}
		return domain.LiveSession{}, fmt.Errorf("live session %s is blocked in %s mode", session.ID, liveRecoveryModeCloseOnlyTakeover)
	}
	if isLiveSessionRecoveryCloseOnlyMode(session.State) {
		return domain.LiveSession{}, fmt.Errorf("live session %s is blocked in %s mode", session.ID, liveRecoveryModeCloseOnlyTakeover)
	}
	gate := resolveLivePositionReconcileGate(account, recoveredPosition.Symbol, recoveredPosition.Quantity > 0)
	if boolValue(gate["blocking"]) {
		if healedAccount, attempted, healErr := p.attemptLiveAccountReconcileSelfHeal(account, recoveredPosition.Symbol); attempted {
			if healErr != nil {
				logger.Warn("reconcile self-heal failed before start gate evaluation", "error", healErr)
			} else {
				account = healedAccount
				session, recoveredPosition, incompleteRecoveryMetadata, err = p.completeRecoveredLiveSessionMetadata(session)
				if err != nil {
					logger.Warn("complete recovered live session metadata failed after self-heal", "error", err)
					return domain.LiveSession{}, err
				}
				if incompleteRecoveryMetadata {
					session, err = p.enterRecoveredLiveSessionCloseOnlyMode(session, recoveredPosition, "missing-strategy-version", "recovered position is missing strategyVersionId")
					if err != nil {
						logger.Warn("enter close-only takeover mode failed after self-heal", "error", err)
						return domain.LiveSession{}, err
					}
					return domain.LiveSession{}, fmt.Errorf("live session %s is blocked in %s mode", session.ID, liveRecoveryModeCloseOnlyTakeover)
				}
				if isLiveSessionRecoveryCloseOnlyMode(session.State) {
					return domain.LiveSession{}, fmt.Errorf("live session %s is blocked in %s mode", session.ID, liveRecoveryModeCloseOnlyTakeover)
				}
				gate = resolveLivePositionReconcileGate(account, recoveredPosition.Symbol, recoveredPosition.Quantity > 0)
			}
		}
	}
	if boolValue(gate["blocking"]) {
		session, err = p.enterRecoveredLiveSessionReconcileGateBlocked(session, recoveredPosition, gate)
		if err != nil {
			logger.Warn("enter reconcile gate blocked mode failed", "error", err)
			return domain.LiveSession{}, err
		}
		return domain.LiveSession{}, fmt.Errorf("live session %s is blocked in %s mode", session.ID, liveRecoveryModeReconcileGateBlocked)
	}
	if isLiveSessionRecoveryReconcileGateBlocked(session.State) {
		return domain.LiveSession{}, fmt.Errorf("live session %s is blocked in %s mode", session.ID, liveRecoveryModeReconcileGateBlocked)
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
		if _, syncErr := p.requestLiveAccountSync(account.ID, "sync-active-live-accounts"); syncErr != nil {
			syncErrs = append(syncErrs, fmt.Errorf("live account %s sync failed: %w", account.ID, syncErr))
		}
	}
	return errors.Join(syncErrs...)
}

func (p *Platform) shouldRefreshLiveAccountSync(account domain.Account, eventTime time.Time) bool {
	if liveAccountPositionReconcilePending(account) {
		return true
	}
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
	if syncedAccount, syncErr := p.triggerAuthoritativeLiveAccountReconcile(account.ID, "startup-recovery", time.Now().UTC()); syncErr != nil {
		latestAccount, latestErr := p.store.GetAccount(account.ID)
		if latestErr == nil {
			account = latestAccount
		}
	} else {
		account = syncedAccount
	}
	if strings.TrimSpace(stringValue(session.State["lastDispatchedOrderId"])) != "" {
		if syncedSession, syncErr := p.syncLatestLiveSessionOrder(session, time.Now().UTC()); syncErr == nil {
			session = syncedSession
		}
	}
	session, recoveredPosition, incompleteRecoveryMetadata, err := p.completeRecoveredLiveSessionMetadata(session)
	if err != nil {
		return domain.LiveSession{}, err
	}
	if incompleteRecoveryMetadata {
		session, err = p.enterRecoveredLiveSessionCloseOnlyMode(session, recoveredPosition, "missing-strategy-version", "recovered position is missing strategyVersionId")
		if err != nil {
			return domain.LiveSession{}, err
		}
		session, _ = p.refreshLiveSessionPositionContext(session, time.Now().UTC(), "live-startup-recovery-close-only")
		return session, nil
	}
	gate := resolveLivePositionReconcileGate(account, recoveredPosition.Symbol, recoveredPosition.Quantity > 0)
	if boolValue(gate["blocking"]) {
		if healedAccount, attempted, healErr := p.attemptLiveAccountReconcileSelfHeal(account, recoveredPosition.Symbol); attempted {
			if healErr == nil {
				account = healedAccount
				session, recoveredPosition, incompleteRecoveryMetadata, err = p.completeRecoveredLiveSessionMetadata(session)
				if err != nil {
					return domain.LiveSession{}, err
				}
				if incompleteRecoveryMetadata {
					session, err = p.enterRecoveredLiveSessionCloseOnlyMode(session, recoveredPosition, "missing-strategy-version", "recovered position is missing strategyVersionId")
					if err != nil {
						return domain.LiveSession{}, err
					}
					session, _ = p.refreshLiveSessionPositionContext(session, time.Now().UTC(), "live-startup-recovery-close-only")
					return session, nil
				}
				gate = resolveLivePositionReconcileGate(account, recoveredPosition.Symbol, recoveredPosition.Quantity > 0)
			}
		}
	}
	if boolValue(gate["blocking"]) {
		return p.enterRecoveredLiveSessionReconcileGateBlocked(session, recoveredPosition, gate)
	}
	session, err = p.syncLiveSessionRuntime(session)
	if err != nil {
		if recoveredPosition.Quantity > 0 {
			return p.enterRecoveredLiveSessionCloseOnlyMode(session, recoveredPosition, "runtime-linkage-unavailable", err.Error())
		}
		return domain.LiveSession{}, err
	}
	session, err = p.ensureLiveSessionSignalRuntimeStarted(session)
	if err != nil {
		if recoveredPosition.Quantity > 0 {
			return p.enterRecoveredLiveSessionCloseOnlyMode(session, recoveredPosition, "runtime-linkage-unavailable", err.Error())
		}
		return domain.LiveSession{}, err
	}
	if strings.TrimSpace(stringValue(session.State["lastDispatchedOrderId"])) != "" {
		session, _ = p.syncLatestLiveSessionOrder(session, time.Now().UTC())
	}
	session, _ = p.refreshLiveSessionPositionContext(session, time.Now().UTC(), "live-startup-recovery")
	if isLiveSessionRecoveryCloseOnlyMode(session.State) {
		return session, nil
	}
	return p.store.UpdateLiveSessionStatus(session.ID, "RUNNING")
}

func (p *Platform) reconcileLiveAccountPositions(account domain.Account, exchangePositions []map[string]any) (map[string]any, error) {
	existing, err := p.store.ListPositions()
	if err != nil {
		return nil, err
	}
	existingBySymbol := make(map[string]domain.Position)
	for _, position := range existing {
		if position.AccountID != account.ID {
			continue
		}
		existingBySymbol[NormalizeSymbol(position.Symbol)] = position
	}
	pendingSettlementSymbols, err := p.liveSettlementPendingOrderSymbols(account.ID)
	if err != nil {
		return nil, err
	}

	syncedAt := time.Now().UTC()
	symbols := make(map[string]any)
	blockingCount := 0
	previousSymbols := make(map[string]struct{})
	for symbol := range mapValue(mapValue(account.Metadata["livePositionReconcileGate"])["symbols"]) {
		normalized := NormalizeSymbol(symbol)
		if normalized == "" {
			continue
		}
		previousSymbols[normalized] = struct{}{}
	}
	recordGate := func(symbol string, gate map[string]any) {
		if symbol == "" || gate == nil {
			return
		}
		gate = cloneMetadata(gate)
		gate["symbol"] = symbol
		gate["comparedAt"] = firstNonEmpty(stringValue(gate["comparedAt"]), syncedAt.Format(time.RFC3339))
		if boolValue(gate["blocking"]) {
			blockingCount++
		}
		symbols[symbol] = gate
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
		strategyVersionID := p.resolveLivePositionStrategyVersionID(account.ID, symbol)
		position := existingBySymbol[symbol]
		entryPrice := resolveRecoveredLiveEntryPrice(
			parseFloatValue(item["entryPrice"]),
			parseFloatValue(item["breakEvenPrice"]),
			position.EntryPrice,
		)
		markPrice := firstPositive(parseFloatValue(item["markPrice"]), entryPrice)
		exchangeSnapshot := map[string]any{
			"symbol":     symbol,
			"side":       side,
			"quantity":   quantity,
			"entryPrice": entryPrice,
			"markPrice":  markPrice,
		}
		if _, pending := pendingSettlementSymbols[symbol]; pending {
			recordGate(symbol, map[string]any{
				"status":           livePositionReconcileGateStatusStale,
				"scenario":         "order-settlement-pending",
				"blocking":         true,
				"dbPosition":       buildRecoveredLivePositionStateSnapshot(position),
				"exchangePosition": exchangeSnapshot,
			})
			continue
		}
		if position.ID == "" || position.Quantity <= 0 {
			position.AccountID = account.ID
			position.StrategyVersionID = firstNonEmpty(strategyVersionID, position.StrategyVersionID)
			position.Symbol = symbol
			position.Side = side
			position.Quantity = quantity
			position.EntryPrice = entryPrice
			position.MarkPrice = markPrice
			if _, err := p.store.SavePosition(position); err != nil {
				return nil, err
			}
			recordGate(symbol, map[string]any{
				"status":           livePositionReconcileGateStatusAdopted,
				"scenario":         "exchange-position-db-missing",
				"blocking":         false,
				"dbPosition":       map[string]any{},
				"exchangePosition": exchangeSnapshot,
			})
			continue
		}
		dbSnapshot := buildRecoveredLivePositionStateSnapshot(position)
		mismatchFields := make([]any, 0, 3)
		if !strings.EqualFold(strings.TrimSpace(position.Side), side) {
			mismatchFields = append(mismatchFields, "side")
		}
		if tradingQuantityDiffers(position.Quantity, quantity) {
			mismatchFields = append(mismatchFields, "quantity")
		}
		if tradingPriceDiffers(position.EntryPrice, entryPrice) {
			mismatchFields = append(mismatchFields, "entryPrice")
		}
		if len(mismatchFields) > 0 {
			recordGate(symbol, map[string]any{
				"status":           livePositionReconcileGateStatusConflict,
				"scenario":         classifyLivePositionReconcileScenario(mismatchFields),
				"blocking":         true,
				"mismatchFields":   mismatchFields,
				"dbPosition":       dbSnapshot,
				"exchangePosition": exchangeSnapshot,
			})
			continue
		}
		position.AccountID = account.ID
		position.StrategyVersionID = firstNonEmpty(strategyVersionID, position.StrategyVersionID)
		position.Symbol = symbol
		position.Side = side
		position.Quantity = quantity
		position.EntryPrice = entryPrice
		position.MarkPrice = markPrice
		if _, err := p.store.SavePosition(position); err != nil {
			return nil, err
		}
		recordGate(symbol, map[string]any{
			"status":           livePositionReconcileGateStatusVerified,
			"scenario":         "db-position-matches-exchange",
			"blocking":         false,
			"dbPosition":       dbSnapshot,
			"exchangePosition": exchangeSnapshot,
		})
	}

	for symbol, position := range existingBySymbol {
		if _, ok := seenSymbols[symbol]; ok {
			continue
		}
		if position.Quantity <= 0 {
			continue
		}
		recordGate(symbol, map[string]any{
			"status":           livePositionReconcileGateStatusStale,
			"scenario":         "db-position-exchange-missing",
			"blocking":         true,
			"dbPosition":       buildRecoveredLivePositionStateSnapshot(position),
			"exchangePosition": map[string]any{},
		})
	}
	for symbol := range previousSymbols {
		if _, ok := symbols[symbol]; ok {
			continue
		}
		if _, ok := seenSymbols[symbol]; ok {
			continue
		}
		if position, ok := existingBySymbol[symbol]; ok && position.Quantity > 0 {
			continue
		}
		recordGate(symbol, map[string]any{
			"status":           livePositionReconcileGateStatusVerified,
			"scenario":         "exchange-flat",
			"blocking":         false,
			"dbPosition":       map[string]any{},
			"exchangePosition": map[string]any{},
		})
	}
	return map[string]any{
		"source":              "binance-position-reconcile",
		"syncedAt":            syncedAt.Format(time.RFC3339),
		"authoritative":       true,
		"blockingSymbolCount": blockingCount,
		"symbols":             symbols,
	}, nil
}

func (p *Platform) liveSettlementPendingOrderSymbols(accountID string) (map[string]struct{}, error) {
	orders, err := p.store.ListOrders()
	if err != nil {
		return nil, err
	}
	symbols := make(map[string]struct{})
	for _, order := range orders {
		if order.AccountID != accountID || !liveOrderSettlementSyncPending(order) {
			continue
		}
		if symbol := NormalizeSymbol(order.Symbol); symbol != "" {
			symbols[symbol] = struct{}{}
		}
	}
	return symbols, nil
}

func resolveRecoveredLiveEntryPrice(primary, secondary, fallback float64) float64 {
	return firstPositive(primary, firstPositive(secondary, fallback))
}

func classifyLivePositionReconcileScenario(mismatchFields []any) string {
	if len(mismatchFields) == 1 {
		switch stringValue(mismatchFields[0]) {
		case "side":
			return "side-mismatch"
		case "quantity":
			return "quantity-mismatch"
		case "entryPrice":
			return "entry-price-mismatch"
		}
	}
	return "multi-field-mismatch"
}

func (p *Platform) resolveLivePositionStrategyVersionID(accountID, symbol string) string {
	return p.inferReconcileStrategyVersionID(accountID, symbol)
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
	recoveryActions := currentLiveRecoveryActionMatrix(session.State)

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
	if signalBarDecision := cloneMetadata(mapValue(decision.Metadata["signalBarDecision"])); len(signalBarDecision) > 0 {
		state["lastStrategyEvaluationSignalBarDecision"] = signalBarDecision
	} else {
		delete(state, "lastStrategyEvaluationSignalBarDecision")
	}
	if signalBarStateKey := stringValue(decision.Metadata["signalBarStateKey"]); signalBarStateKey != "" {
		state["lastStrategyEvaluationSignalBarStateKey"] = signalBarStateKey
	} else {
		delete(state, "lastStrategyEvaluationSignalBarStateKey")
	}
	recordLatestBreakoutSignal(state, decision, eventTime)
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
	if executionContext.SignalTimeframe != "" {
		state["signalTimeframe"] = executionContext.SignalTimeframe
	}
	if executionContext.Symbol != "" {
		state["symbol"] = executionContext.Symbol
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
	if len(executionProposal) > 0 {
		action := resolveLiveRecoveryIntentAction(executionProposal)
		if !liveRecoveryIntentActionAllowed(recoveryActions, action) {
			delete(state, "lastExecutionProposal")
			delete(state, "lastExecutionProfile")
			delete(state, "lastStrategyIntent")
			delete(state, "lastStrategyIntentSignature")
			state["lastStrategyEvaluationStatus"] = "recovery-manual-review"
			state["lastRecoveryBlockedAction"] = action
			state["lastRecoveryBlockedAt"] = eventTime.UTC().Format(time.RFC3339)
			appendTimelineEvent(state, "recovery", eventTime, "recovery-action-blocked", map[string]any{
				"takeoverState": stringValue(state["recoveryTakeoverState"]),
				"action":        action,
			})
			intent = nil
			executionProposal = nil
		}
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
	} else if strings.EqualFold(stringValue(state["lastStrategyEvaluationStatus"]), "recovery-manual-review") {
		// Keep the takeover/manual-review verdict instead of downgrading back to a normal monitoring state.
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
	breakoutPrice, breakoutPriceSource := pickSignalBreakoutPrice(summary, sourceStates)
	updatedState, nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason := prepareLivePlanStepForSignalEvaluation(
		session.State,
		executionContext.Parameters,
		signalBarStates,
		executionContext.Symbol,
		executionContext.SignalTimeframe,
		currentPosition,
		eventTime,
		breakoutPrice,
		breakoutPriceSource,
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
		SessionState:      cloneMetadata(updatedState),
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
	breakoutPrice float64,
	breakoutPriceSource string,
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
	gate := evaluateSignalBarGate(signalBarState, "", "entry", "", breakoutPrice, breakoutPriceSource)
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
	if isLiveSessionRecoveryCloseOnlyMode(state) {
		state["runtimeMode"] = liveRecoveryModeCloseOnlyTakeover
		state["signalRuntimeMode"] = liveRecoveryModeCloseOnlyTakeover
		state["signalRuntimeRequired"] = false
		state["signalRuntimeReady"] = false
		delete(state, "signalRuntimeSessionId")
		delete(state, "signalRuntimeStatus")
		return p.store.UpdateLiveSessionState(session.ID, state)
	}
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

	requiredBindings := metadataList(plan["requiredBindings"])
	state = applyCanonicalLiveSignalScope(state, requiredBindings)
	required := len(requiredBindings) > 0
	state["signalRuntimePlan"] = plan
	state["signalRuntimeMode"] = "linked"
	state["signalRuntimeRequired"] = required
	state["signalRuntimeReady"] = boolValue(plan["ready"])
	if stringValue(state["dispatchMode"]) == "" {
		state["dispatchMode"] = "manual-review"
	}
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
	session, recoveredPosition, incompleteRecoveryMetadata, err := p.completeRecoveredLiveSessionMetadata(session)
	if err != nil {
		return domain.LiveSession{}, nil, err
	}
	if incompleteRecoveryMetadata {
		session, err = p.enterRecoveredLiveSessionCloseOnlyMode(session, recoveredPosition, "missing-strategy-version", "recovered position is missing strategyVersionId")
		if err != nil {
			return domain.LiveSession{}, nil, err
		}
		return session, nil, fmt.Errorf("live session %s is in close-only takeover mode", session.ID)
	}
	if isLiveSessionRecoveryCloseOnlyMode(session.State) {
		return session, nil, fmt.Errorf("live session %s is in close-only takeover mode", session.ID)
	}
	if isLiveSessionRecoveryReconcileGateBlocked(session.State) {
		return session, nil, fmt.Errorf("live session %s is blocked by reconcile gate", session.ID)
	}

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

	session, err = p.syncLiveSessionRuntime(session)
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
	state["positionRecoveryStatus"] = normalizedRecoveredPositionStatus(
		stringValue(state["positionRecoveryStatus"]),
		foundPosition,
		boolValue(positionSnapshot["virtual"]),
	)
	takeoverMatrix := applyLiveRecoveryTakeoverState(state, boolValue(state["recoveryTakeoverActive"]))
	if nextIndex, adjusted := reconcileLivePlanIndexWithPosition(plan, resolveLivePlanIndex(state), positionSnapshot, foundPosition); adjusted && takeoverMatrix.AllowPlanProgression {
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

const (
	liveRecoveryModeCloseOnlyTakeover       = "close-only-takeover"
	liveRecoveryModeReconcileGateBlocked    = "reconcile-gate-blocked"
	liveRecoveryMetadataStatusComplete      = "complete"
	liveRecoveryMetadataStatusIncomplete    = "incomplete"
	liveRecoveryTakeoverStateMonitoring     = "recovery-monitoring"
	liveRecoveryTakeoverStateUnprotected    = "unprotected-open-position"
	liveRecoveryTakeoverStateStaleSync      = "stale-sync"
	liveRecoveryTakeoverStateConflict       = "recovery-conflict"
	liveRecoveryTakeoverStateError          = "error"
	livePositionReconcileGateStatusAdopted  = "adopted"
	livePositionReconcileGateStatusVerified = "verified"
	livePositionReconcileGateStatusStale    = "stale"
	livePositionReconcileGateStatusConflict = "conflict"
	livePositionReconcileGateStatusError    = "error"
)

type liveRecoveryActionMatrix struct {
	OpenNewPosition       bool
	CloseExistingPosition bool
	PlaceProtectionOrders bool
	AutoDispatch          bool
	ManualReviewRequired  bool
	AllowPlanProgression  bool
}

// Recovery/takeover action matrix.
// State                     open close protect auto-dispatch manual-review plan-progress
// close-only-takeover       no   yes   no      no            yes           no
// recovery-monitoring       no   yes   yes     no            yes           yes
// unprotected-open-position no   yes   yes     no            yes           no
// stale-sync                no   no    no      no            yes           no
// recovery-conflict         no   no    no      no            yes           no
// error                     no   no    no      no            yes           no
func liveRecoveryActionMatrixForState(state string) liveRecoveryActionMatrix {
	switch strings.TrimSpace(state) {
	case liveRecoveryModeCloseOnlyTakeover:
		return liveRecoveryActionMatrix{
			CloseExistingPosition: true,
			ManualReviewRequired:  true,
		}
	case liveRecoveryTakeoverStateMonitoring:
		return liveRecoveryActionMatrix{
			CloseExistingPosition: true,
			PlaceProtectionOrders: true,
			ManualReviewRequired:  true,
			AllowPlanProgression:  true,
		}
	case liveRecoveryTakeoverStateUnprotected:
		return liveRecoveryActionMatrix{
			CloseExistingPosition: true,
			PlaceProtectionOrders: true,
			ManualReviewRequired:  true,
		}
	case liveRecoveryTakeoverStateStaleSync, liveRecoveryTakeoverStateConflict, liveRecoveryTakeoverStateError:
		return liveRecoveryActionMatrix{ManualReviewRequired: true}
	default:
		return liveRecoveryActionMatrix{
			OpenNewPosition:       true,
			CloseExistingPosition: true,
			PlaceProtectionOrders: true,
			AutoDispatch:          true,
			AllowPlanProgression:  true,
		}
	}
}

func hasRecoveredLiveRealPosition(state map[string]any) bool {
	return boolValue(state["hasRecoveredPosition"]) ||
		boolValue(state["hasRecoveredRealPosition"]) ||
		tradingQuantityPositive(math.Abs(parseFloatValue(mapValue(state["recoveredPosition"])["quantity"])))
}

func shouldActivateLiveRecoveryTakeover(source string, state map[string]any) bool {
	if boolValue(state["recoveryTakeoverActive"]) {
		return true
	}
	return strings.HasPrefix(strings.TrimSpace(source), "live-startup-recovery")
}

func resolveLiveRecoveryTakeoverState(state map[string]any, active bool) string {
	if !active {
		return ""
	}
	if isLiveSessionRecoveryCloseOnlyMode(state) ||
		strings.EqualFold(stringValue(state["positionRecoveryStatus"]), liveRecoveryModeCloseOnlyTakeover) {
		return liveRecoveryModeCloseOnlyTakeover
	}
	switch strings.TrimSpace(stringValue(state["positionReconcileGateStatus"])) {
	case livePositionReconcileGateStatusStale:
		return liveRecoveryTakeoverStateStaleSync
	case livePositionReconcileGateStatusConflict:
		return liveRecoveryTakeoverStateConflict
	case livePositionReconcileGateStatusError:
		return liveRecoveryTakeoverStateError
	}
	switch strings.TrimSpace(stringValue(state["positionRecoveryStatus"])) {
	case liveRecoveryTakeoverStateUnprotected:
		return liveRecoveryTakeoverStateUnprotected
	case "protected-open-position", "monitoring-open-position", "monitoring-virtual-position", livePositionRecoveryStatusClosingPending:
		return liveRecoveryTakeoverStateMonitoring
	case livePositionReconcileGateStatusStale:
		return liveRecoveryTakeoverStateStaleSync
	case livePositionReconcileGateStatusConflict:
		return liveRecoveryTakeoverStateConflict
	case livePositionReconcileGateStatusError:
		return liveRecoveryTakeoverStateError
	}
	if hasRecoveredLiveRealPosition(state) {
		return liveRecoveryTakeoverStateMonitoring
	}
	if strings.TrimSpace(stringValue(state["positionRecoveryStatus"])) == "" ||
		strings.EqualFold(stringValue(state["positionRecoveryStatus"]), "flat") {
		return ""
	}
	return liveRecoveryTakeoverStateError
}

func applyLiveRecoveryTakeoverState(state map[string]any, active bool) liveRecoveryActionMatrix {
	if state == nil {
		return liveRecoveryActionMatrixForState("")
	}
	takeoverState := resolveLiveRecoveryTakeoverState(state, active)
	if takeoverState == "" {
		delete(state, "recoveryTakeoverActive")
		delete(state, "recoveryTakeoverState")
		delete(state, "recoveryActionMatrix")
		delete(state, "recoveryManualReviewRequired")
		return liveRecoveryActionMatrixForState("")
	}
	matrix := liveRecoveryActionMatrixForState(takeoverState)
	state["recoveryTakeoverActive"] = true
	state["recoveryTakeoverState"] = takeoverState
	state["recoveryManualReviewRequired"] = matrix.ManualReviewRequired
	state["recoveryActionMatrix"] = map[string]any{
		"openNewPosition":       matrix.OpenNewPosition,
		"closeExistingPosition": matrix.CloseExistingPosition,
		"placeProtectionOrders": matrix.PlaceProtectionOrders,
		"autoDispatch":          matrix.AutoDispatch,
		"manualReviewRequired":  matrix.ManualReviewRequired,
		"allowPlanProgression":  matrix.AllowPlanProgression,
	}
	return matrix
}

func currentLiveRecoveryActionMatrix(state map[string]any) liveRecoveryActionMatrix {
	return liveRecoveryActionMatrixForState(resolveLiveRecoveryTakeoverState(state, boolValue(state["recoveryTakeoverActive"])))
}

func normalizedRecoveredPositionStatus(currentStatus string, foundPosition, virtualPosition bool) string {
	switch strings.TrimSpace(currentStatus) {
	case "protected-open-position", "unprotected-open-position", livePositionRecoveryStatusClosingPending:
		if foundPosition {
			return currentStatus
		}
	}
	switch {
	case foundPosition:
		return "monitoring-open-position"
	case virtualPosition:
		return "monitoring-virtual-position"
	default:
		return "flat"
	}
}

func resolveLiveRecoveryIntentAction(intent map[string]any) string {
	if len(intent) == 0 {
		return ""
	}
	orderType := strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(intent["type"]), stringValue(mapValue(intent["metadata"])["orderType"]))))
	role := strings.ToLower(strings.TrimSpace(stringValue(intent["role"])))
	if strings.Contains(orderType, "STOP") || strings.Contains(orderType, "TAKE_PROFIT") {
		return "place-protection-orders"
	}
	if role == "exit" || boolValue(intent["reduceOnly"]) || boolValue(intent["closePosition"]) {
		return "close-existing-position"
	}
	return "open-new-position"
}

func liveRecoveryIntentActionAllowed(matrix liveRecoveryActionMatrix, action string) bool {
	switch action {
	case "open-new-position":
		return matrix.OpenNewPosition
	case "close-existing-position":
		return matrix.CloseExistingPosition
	case "place-protection-orders":
		return matrix.PlaceProtectionOrders
	default:
		return true
	}
}

func isLiveSessionRecoveryCloseOnlyMode(state map[string]any) bool {
	return strings.EqualFold(strings.TrimSpace(stringValue(state["recoveryMode"])), liveRecoveryModeCloseOnlyTakeover)
}

func isLiveSessionRecoveryReconcileGateBlocked(state map[string]any) bool {
	return strings.EqualFold(strings.TrimSpace(stringValue(state["recoveryMode"])), liveRecoveryModeReconcileGateBlocked)
}

func isLiveSessionBlockedByPositionReconcileGate(state map[string]any) bool {
	if boolValue(state["positionReconcileGateBlocking"]) {
		return true
	}
	switch strings.TrimSpace(stringValue(state["positionReconcileGateStatus"])) {
	case livePositionReconcileGateStatusStale, livePositionReconcileGateStatusConflict, livePositionReconcileGateStatusError:
		return true
	}
	return isLiveSessionRecoveryReconcileGateBlocked(state)
}

func livePositionReconcileGateRequired(snapshot map[string]any) bool {
	return strings.EqualFold(strings.TrimSpace(stringValue(snapshot["executionMode"])), "rest")
}

func resolveLivePositionReconcileGate(account domain.Account, symbol string, requiresVerification bool) map[string]any {
	symbol = NormalizeSymbol(symbol)
	snapshot := cloneMetadata(mapValue(account.Metadata["liveSyncSnapshot"]))
	gate := map[string]any{
		"symbol":        symbol,
		"status":        livePositionReconcileGateStatusVerified,
		"blocking":      false,
		"source":        stringValue(snapshot["source"]),
		"authoritative": liveProtectionSnapshotIsAuthoritative(snapshot),
		"required":      livePositionReconcileGateRequired(snapshot),
	}
	if !requiresVerification || symbol == "" || !boolValue(gate["required"]) {
		return gate
	}
	if liveAccountPositionReconcilePending(account) {
		gate["status"] = livePositionReconcileGateStatusStale
		gate["blocking"] = true
		gate["scenario"] = firstNonEmpty(
			strings.TrimSpace(stringValue(account.Metadata["livePositionReconcileTrigger"])),
			"reconcile-required",
		)
		gate["requiredAt"] = stringValue(account.Metadata["livePositionReconcileRequiredAt"])
		gate["takeoverState"] = liveRecoveryTakeoverStateStaleSync
		return gate
	}
	if !boolValue(gate["authoritative"]) {
		gate["status"] = livePositionReconcileGateStatusError
		gate["blocking"] = true
		gate["scenario"] = "exchange-truth-unavailable"
		gate["takeoverState"] = liveRecoveryTakeoverStateError
		return gate
	}
	symbolGate := cloneMetadata(mapValue(mapValue(mapValue(account.Metadata["livePositionReconcileGate"])["symbols"])[symbol]))
	if len(symbolGate) == 0 {
		gate["status"] = livePositionReconcileGateStatusError
		gate["blocking"] = true
		gate["scenario"] = "missing-reconcile-verdict"
		gate["takeoverState"] = liveRecoveryTakeoverStateError
		return gate
	}
	for key, value := range symbolGate {
		gate[key] = value
	}
	gate["symbol"] = symbol
	gate["blocking"] = boolValue(gate["blocking"])
	gate["status"] = firstNonEmpty(stringValue(gate["status"]), livePositionReconcileGateStatusVerified)
	switch strings.TrimSpace(stringValue(gate["status"])) {
	case livePositionReconcileGateStatusStale:
		gate["takeoverState"] = liveRecoveryTakeoverStateStaleSync
	case livePositionReconcileGateStatusConflict:
		gate["takeoverState"] = liveRecoveryTakeoverStateConflict
	case livePositionReconcileGateStatusError:
		gate["takeoverState"] = liveRecoveryTakeoverStateError
	default:
		gate["takeoverState"] = ""
	}
	return gate
}

func applyLivePositionReconcileGateState(state map[string]any, gate map[string]any) {
	if state == nil {
		return
	}
	gate = cloneMetadata(gate)
	state["positionReconcileGateStatus"] = firstNonEmpty(stringValue(gate["status"]), livePositionReconcileGateStatusVerified)
	state["positionReconcileGateBlocking"] = boolValue(gate["blocking"])
	state["positionReconcileGateScenario"] = stringValue(gate["scenario"])
	state["positionReconcileGateComparedAt"] = stringValue(gate["comparedAt"])
	state["positionReconcileGateSource"] = stringValue(gate["source"])
	state["positionReconcileGateDBPosition"] = cloneMetadata(mapValue(gate["dbPosition"]))
	state["positionReconcileGateExchangePosition"] = cloneMetadata(mapValue(gate["exchangePosition"]))
	if mismatchFields := metadataList(gate["mismatchFields"]); len(mismatchFields) > 0 {
		state["positionReconcileGateMismatchFields"] = mismatchFields
	} else {
		delete(state, "positionReconcileGateMismatchFields")
	}
	if boolValue(gate["blocking"]) {
		state["positionRecoveryStatus"] = firstNonEmpty(stringValue(gate["status"]), livePositionReconcileGateStatusError)
		state["lastStrategyEvaluationStatus"] = liveRecoveryModeReconcileGateBlocked
	}
}

func buildRecoveredLivePositionStateSnapshot(position domain.Position) map[string]any {
	if position.Quantity <= 0 {
		return map[string]any{}
	}
	return map[string]any{
		"id":                position.ID,
		"symbol":            NormalizeSymbol(position.Symbol),
		"side":              position.Side,
		"quantity":          position.Quantity,
		"entryPrice":        position.EntryPrice,
		"markPrice":         position.MarkPrice,
		"strategyVersionId": position.StrategyVersionID,
		"updatedAt":         position.UpdatedAt.UTC().Format(time.RFC3339),
		"found":             true,
	}
}

func (p *Platform) completeRecoveredLiveSessionMetadata(session domain.LiveSession) (domain.LiveSession, domain.Position, bool, error) {
	symbol := NormalizeSymbol(firstNonEmpty(stringValue(session.State["symbol"]), stringValue(session.State["lastSymbol"])))
	if symbol == "" {
		return session, domain.Position{}, false, nil
	}
	position, found, err := p.store.FindPosition(session.AccountID, symbol)
	if err != nil {
		return domain.LiveSession{}, domain.Position{}, false, err
	}
	if !found || position.Quantity <= 0 {
		state := cloneMetadata(session.State)
		delete(state, "recoveryMetadataStatus")
		delete(state, "recoveryMetadataMissing")
		delete(state, "recoveryMetadataCompletedAt")
		delete(state, "recoveryMode")
		delete(state, "recoveryCloseOnlyReason")
		delete(state, "recoveryCloseOnlyDetail")
		delete(state, "recoveryCloseOnlyAt")
		delete(state, "recoveryBlockedReason")
		delete(state, "recoveryBlockedDetail")
		delete(state, "recoveryBlockedAt")
		delete(state, "runtimeMode")
		delete(state, "signalRuntimeMode")
		// Preserve the current runtime linkage gate for RUNNING sessions.
		// Clearing these flags while flat can silently detach runtime fanout
		// from an otherwise healthy live session until some later resync
		// happens to rebuild the linkage state.
		if strings.EqualFold(stringValue(state["lastStrategyEvaluationStatus"]), liveRecoveryModeCloseOnlyTakeover) {
			delete(state, "lastStrategyEvaluationStatus")
		}
		if strings.EqualFold(stringValue(state["lastStrategyEvaluationStatus"]), liveRecoveryModeReconcileGateBlocked) {
			delete(state, "lastStrategyEvaluationStatus")
		}
		if strings.EqualFold(stringValue(state["positionRecoveryStatus"]), liveRecoveryModeCloseOnlyTakeover) {
			state["positionRecoveryStatus"] = "flat"
		}
		if isLiveSessionRecoveryReconcileGateBlocked(state) || strings.EqualFold(stringValue(state["positionRecoveryStatus"]), livePositionReconcileGateStatusStale) ||
			strings.EqualFold(stringValue(state["positionRecoveryStatus"]), livePositionReconcileGateStatusConflict) ||
			strings.EqualFold(stringValue(state["positionRecoveryStatus"]), livePositionReconcileGateStatusError) {
			state["positionRecoveryStatus"] = "flat"
		}
		delete(state, "recoveryMode")
		delete(state, "positionReconcileGateStatus")
		delete(state, "positionReconcileGateBlocking")
		delete(state, "positionReconcileGateScenario")
		delete(state, "positionReconcileGateComparedAt")
		delete(state, "positionReconcileGateSource")
		delete(state, "positionReconcileGateMismatchFields")
		delete(state, "positionReconcileGateDBPosition")
		delete(state, "positionReconcileGateExchangePosition")
		applyLiveRecoveryTakeoverState(state, false)
		if !metadataEqual(state, session.State) {
			session, err = p.store.UpdateLiveSessionState(session.ID, state)
			if err != nil {
				return domain.LiveSession{}, domain.Position{}, false, err
			}
		}
		return session, domain.Position{}, false, nil
	}

	state := cloneMetadata(session.State)
	if strings.TrimSpace(position.StrategyVersionID) == "" {
		position.StrategyVersionID = p.resolveLivePositionStrategyVersionID(session.AccountID, symbol)
		if strings.TrimSpace(position.StrategyVersionID) != "" {
			position, err = p.store.SavePosition(position)
			if err != nil {
				return domain.LiveSession{}, domain.Position{}, false, err
			}
		}
	}
	if strings.TrimSpace(position.StrategyVersionID) == "" {
		state["recoveryMetadataStatus"] = liveRecoveryMetadataStatusIncomplete
		state["recoveryMetadataMissing"] = []any{"strategyVersionId"}
		if !metadataEqual(state, session.State) {
			session, err = p.store.UpdateLiveSessionState(session.ID, state)
			if err != nil {
				return domain.LiveSession{}, domain.Position{}, false, err
			}
		}
		return session, position, true, nil
	}

	state["strategyVersionId"] = position.StrategyVersionID
	state["recoveryMetadataStatus"] = liveRecoveryMetadataStatusComplete
	state["recoveryMetadataCompletedAt"] = time.Now().UTC().Format(time.RFC3339)
	delete(state, "recoveryMetadataMissing")
	delete(state, "recoveryMode")
	delete(state, "recoveryCloseOnlyReason")
	delete(state, "recoveryCloseOnlyDetail")
	delete(state, "recoveryCloseOnlyAt")
	delete(state, "recoveryBlockedReason")
	delete(state, "recoveryBlockedDetail")
	delete(state, "recoveryBlockedAt")
	applyLiveRecoveryTakeoverState(state, boolValue(state["recoveryTakeoverActive"]))
	if !metadataEqual(state, session.State) {
		session, err = p.store.UpdateLiveSessionState(session.ID, state)
		if err != nil {
			return domain.LiveSession{}, domain.Position{}, false, err
		}
	}
	return session, position, false, nil
}

func (p *Platform) enterRecoveredLiveSessionCloseOnlyMode(session domain.LiveSession, position domain.Position, reason, detail string) (domain.LiveSession, error) {
	state := cloneMetadata(session.State)
	state["recoveryMode"] = liveRecoveryModeCloseOnlyTakeover
	state["runtimeMode"] = liveRecoveryModeCloseOnlyTakeover
	state["signalRuntimeMode"] = liveRecoveryModeCloseOnlyTakeover
	state["signalRuntimeRequired"] = false
	state["signalRuntimeReady"] = false
	state["recoveryMetadataStatus"] = liveRecoveryMetadataStatusIncomplete
	state["recoveryCloseOnlyReason"] = reason
	state["recoveryCloseOnlyDetail"] = detail
	state["recoveryCloseOnlyAt"] = time.Now().UTC().Format(time.RFC3339)
	state["lastStrategyEvaluationStatus"] = liveRecoveryModeCloseOnlyTakeover
	state["positionRecoveryStatus"] = liveRecoveryModeCloseOnlyTakeover
	state["recoveredPosition"] = buildRecoveredLivePositionStateSnapshot(position)
	state["hasRecoveredPosition"] = position.Quantity > 0
	state["hasRecoveredRealPosition"] = position.Quantity > 0
	state["hasRecoveredVirtualPosition"] = false
	delete(state, "signalRuntimeSessionId")
	delete(state, "signalRuntimeStatus")
	delete(state, "lastExecutionProposal")
	delete(state, "lastStrategyIntent")
	delete(state, "lastSignalIntent")
	delete(state, "lastStrategyIntentSignature")
	applyLiveRecoveryTakeoverState(state, true)
	session, err := p.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		return domain.LiveSession{}, err
	}
	return p.store.UpdateLiveSessionStatus(session.ID, "BLOCKED")
}

func (p *Platform) enterRecoveredLiveSessionReconcileGateBlocked(session domain.LiveSession, position domain.Position, gate map[string]any) (domain.LiveSession, error) {
	state := cloneMetadata(session.State)
	state["recoveryMode"] = liveRecoveryModeReconcileGateBlocked
	state["runtimeMode"] = liveRecoveryModeReconcileGateBlocked
	state["signalRuntimeMode"] = liveRecoveryModeReconcileGateBlocked
	state["signalRuntimeRequired"] = false
	state["signalRuntimeReady"] = false
	state["recoveryBlockedReason"] = firstNonEmpty(stringValue(gate["status"]), livePositionReconcileGateStatusError)
	state["recoveryBlockedDetail"] = firstNonEmpty(stringValue(gate["scenario"]), "position-reconcile-gate-blocked")
	state["recoveryBlockedAt"] = time.Now().UTC().Format(time.RFC3339)
	state["recoveredPosition"] = buildRecoveredLivePositionStateSnapshot(position)
	state["hasRecoveredPosition"] = position.Quantity > 0
	state["hasRecoveredRealPosition"] = position.Quantity > 0
	state["hasRecoveredVirtualPosition"] = false
	delete(state, "signalRuntimeSessionId")
	delete(state, "signalRuntimeStatus")
	delete(state, "lastExecutionProposal")
	delete(state, "lastStrategyIntent")
	delete(state, "lastSignalIntent")
	delete(state, "lastStrategyIntentSignature")
	applyLivePositionReconcileGateState(state, gate)
	applyLiveRecoveryTakeoverState(state, true)
	session, err := p.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		return domain.LiveSession{}, err
	}
	return p.store.UpdateLiveSessionStatus(session.ID, "BLOCKED")
}

func metadataEqual(left, right map[string]any) bool {
	return reflect.DeepEqual(left, right)
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
	state["recoveredPosition"] = positionSnapshot
	state["hasRecoveredPosition"] = foundPosition
	state["hasRecoveredRealPosition"] = foundPosition
	state["hasRecoveredVirtualPosition"] = boolValue(positionSnapshot["virtual"])
	takeoverMatrix := applyLiveRecoveryTakeoverState(state, boolValue(state["recoveryTakeoverActive"]))

	nextIndex, adjusted := reconcileLivePlanIndexWithPosition(plan, currentIndex, positionSnapshot, foundPosition)
	planLengthAdjusted := maxIntValue(state["planLength"], -1) != len(plan)
	if !adjusted && !planLengthAdjusted {
		if metadataEqual(state, session.State) {
			return session, nil
		}
		return p.store.UpdateLiveSessionState(session.ID, state)
	}

	state["planLength"] = len(plan)
	if nextIndex < len(plan) {
		delete(state, "completedAt")
	}
	if !adjusted || !takeoverMatrix.AllowPlanProgression {
		return p.store.UpdateLiveSessionState(session.ID, state)
	}
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
	hasRealPosition := foundPosition || tradingQuantityPositive(math.Abs(parseFloatValue(positionSnapshot["quantity"])))
	livePositionState := cloneMetadata(mapValue(session.State["livePositionState"]))
	if len(livePositionState) > 0 && hasRealPosition {
		liveSymbol := NormalizeSymbol(firstNonEmpty(stringValue(livePositionState["symbol"]), symbol))
		if liveSymbol == NormalizeSymbol(symbol) && livePositionStateMatchesPositionSnapshot(positionSnapshot, livePositionState) {
			positionSnapshot = mergeLivePositionRiskState(positionSnapshot, livePositionState)
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

func livePositionStateMatchesPositionSnapshot(positionSnapshot map[string]any, livePositionState map[string]any) bool {
	if len(positionSnapshot) == 0 || len(livePositionState) == 0 {
		return false
	}
	positionSymbol := NormalizeSymbol(stringValue(positionSnapshot["symbol"]))
	liveSymbol := NormalizeSymbol(stringValue(livePositionState["symbol"]))
	if liveSymbol != "" && positionSymbol != "" && liveSymbol != positionSymbol {
		return false
	}
	positionSide := strings.ToUpper(strings.TrimSpace(stringValue(positionSnapshot["side"])))
	liveSide := strings.ToUpper(strings.TrimSpace(stringValue(livePositionState["side"])))
	if liveSide != "" && positionSide != "" && liveSide != positionSide {
		return false
	}
	positionEntry := parseFloatValue(positionSnapshot["entryPrice"])
	liveEntry := parseFloatValue(livePositionState["entryPrice"])
	if liveEntry > 0 && positionEntry > 0 && tradingPriceDiffers(liveEntry, positionEntry) {
		return false
	}
	positionKey := buildLivePositionWatermarkKey(positionSnapshot)
	if positionKey == "" {
		return false
	}
	liveKey := strings.TrimSpace(stringValue(livePositionState["watermarkPositionKey"]))
	if liveKey == "" {
		return false
	}
	if liveKey == positionKey {
		return true
	}
	if strings.TrimSpace(stringValue(positionSnapshot["id"])) != "" {
		return liveKey == buildLegacyPrefixedLivePositionWatermarkKey(positionSnapshot)
	}
	return liveKey == buildLivePositionWatermarkBaseKey(positionSnapshot) ||
		liveKey == buildLegacyLivePositionWatermarkKey(positionSnapshot)
}

func mergeLivePositionRiskState(positionSnapshot map[string]any, livePositionState map[string]any) map[string]any {
	mergedPosition := cloneMetadata(positionSnapshot)
	for _, key := range []string{
		"baseStopLoss",
		"stopLoss",
		"stopLossSource",
		"trailingStopConfigured",
		"trailingStopActive",
		"trailingActivationArmed",
		"trailingStopCandidate",
		"protected",
		"protectionTrigger",
		"prevHigh1",
		"prevLow1",
		"atr14",
		"profitProtectATR",
		"hwm",
		"lwm",
		"watermarkPositionKey",
	} {
		if value, ok := livePositionState[key]; ok {
			mergedPosition[key] = value
		}
	}
	return mergedPosition
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
	parameters = applyCanonicalLiveSignalScope(parameters, resolveStrategySignalBindings(parameters))
	parameters = applyLiveSafeStopDefaults(parameters)
	return NormalizeBacktestParameters(parameters)
}

func (p *Platform) canonicalizeLiveSessionOverridesForStrategy(strategyID string, overrides map[string]any) map[string]any {
	normalized := normalizeLiveSessionOverrides(overrides)
	if len(normalized) == 0 || strings.TrimSpace(strategyID) == "" {
		return normalized
	}
	version, err := p.resolveCurrentStrategyVersion(strategyID)
	if err != nil {
		return normalized
	}
	return applyCanonicalLiveSignalScope(normalized, resolveStrategySignalBindings(version.Parameters))
}

func applyCanonicalLiveSignalScope(scope map[string]any, bindings []map[string]any) map[string]any {
	normalized := cloneMetadata(scope)
	if normalized == nil {
		normalized = map[string]any{}
	}
	symbol, timeframe := resolveCanonicalLiveSignalScope(bindings, normalized)
	if symbol != "" {
		normalized["symbol"] = symbol
	}
	if timeframe != "" {
		normalized["signalTimeframe"] = timeframe
	}
	return normalized
}

func resolveCanonicalLiveSignalScope(bindings []map[string]any, scope map[string]any) (string, string) {
	currentSymbol := normalizeOptionalLiveScopeSymbol(scope["symbol"])
	currentTimeframe := normalizeSignalBarInterval(stringValue(scope["signalTimeframe"]))
	candidates := collectLiveSignalBarBindings(bindings)
	if len(candidates) == 0 {
		return currentSymbol, currentTimeframe
	}

	symbolScopeResolved := false
	if currentSymbol != "" {
		matched := make([]map[string]any, 0, len(candidates))
		for _, binding := range candidates {
			if liveSignalBindingSymbol(binding) == currentSymbol {
				matched = append(matched, binding)
			}
		}
		if len(matched) > 0 {
			candidates = matched
			symbolScopeResolved = true
		} else {
			return currentSymbol, currentTimeframe
		}
	} else if uniqueSymbol := uniqueLiveSignalBindingValue(candidates, liveSignalBindingSymbol); uniqueSymbol != "" {
		currentSymbol = uniqueSymbol
		filtered := make([]map[string]any, 0, len(candidates))
		for _, binding := range candidates {
			if liveSignalBindingSymbol(binding) == uniqueSymbol {
				filtered = append(filtered, binding)
			}
		}
		if len(filtered) > 0 {
			candidates = filtered
		}
		symbolScopeResolved = true
	}

	if currentTimeframe != "" {
		matched := false
		for _, binding := range candidates {
			if liveSignalBindingTimeframe(binding) == currentTimeframe {
				matched = true
				break
			}
		}
		if !matched && symbolScopeResolved {
			if uniqueTimeframe := uniqueLiveSignalBindingValue(candidates, liveSignalBindingTimeframe); uniqueTimeframe != "" {
				currentTimeframe = uniqueTimeframe
			}
		}
	} else if uniqueTimeframe := uniqueLiveSignalBindingValue(candidates, liveSignalBindingTimeframe); uniqueTimeframe != "" {
		currentTimeframe = uniqueTimeframe
	}

	return currentSymbol, currentTimeframe
}

func collectLiveSignalBarBindings(bindings []map[string]any) []map[string]any {
	collected := make([]map[string]any, 0, len(bindings))
	for _, binding := range bindings {
		if normalizeSignalSourceRole(stringValue(binding["role"])) != "signal" {
			continue
		}
		streamType := strings.ToLower(strings.TrimSpace(stringValue(binding["streamType"])))
		if streamType == "" && normalizeSignalSourceKey(stringValue(binding["sourceKey"])) == "binance-kline" {
			streamType = "signal_bar"
		}
		if streamType != "signal_bar" {
			continue
		}
		collected = append(collected, binding)
	}
	return collected
}

func uniqueLiveSignalBindingValue(bindings []map[string]any, extract func(map[string]any) string) string {
	unique := ""
	for _, binding := range bindings {
		value := strings.TrimSpace(extract(binding))
		if value == "" {
			continue
		}
		if unique == "" {
			unique = value
			continue
		}
		if unique != value {
			return ""
		}
	}
	return unique
}

func liveSignalBindingSymbol(binding map[string]any) string {
	return normalizeOptionalLiveScopeSymbol(binding["symbol"])
}

func liveSignalBindingTimeframe(binding map[string]any) string {
	if timeframe := normalizeSignalBarInterval(stringValue(binding["timeframe"])); timeframe != "" {
		return timeframe
	}
	return signalBindingTimeframe(stringValue(binding["sourceKey"]), metadataValue(binding["options"]))
}

func normalizeOptionalLiveScopeSymbol(value any) string {
	symbol := strings.TrimSpace(stringValue(value))
	if symbol == "" {
		return ""
	}
	return NormalizeSymbol(symbol)
}

const (
	liveDefaultTrailingStopATR           = 0.3
	liveDefaultDelayedTrailingActivation = 0.5
)

func applyLiveSafeStopDefaults(parameters map[string]any) map[string]any {
	normalized := cloneMetadata(parameters)
	if normalized == nil {
		normalized = map[string]any{}
	}

	trailingStopATR, trailingConfigured := normalized["trailing_stop_atr"]
	resolvedTrailingStopATR := parseFloatValue(trailingStopATR)
	if !trailingConfigured || resolvedTrailingStopATR <= 0 {
		resolvedTrailingStopATR = liveDefaultTrailingStopATR
		normalized["trailing_stop_atr"] = resolvedTrailingStopATR
	}

	delayedActivationATR, delayedConfigured := normalized["delayed_trailing_activation_atr"]
	resolvedDelayedActivationATR := parseFloatValue(delayedActivationATR)
	if !delayedConfigured || resolvedDelayedActivationATR <= 0 {
		normalized["delayed_trailing_activation_atr"] = liveDefaultDelayedTrailingActivation
	}

	stopLossATR, stopConfigured := normalized["stop_loss_atr"]
	if !stopConfigured || parseFloatValue(stopLossATR) <= 0 {
		normalized["stop_loss_atr"] = resolvedTrailingStopATR
	}

	return normalized
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
	signalBarTradeLimitKey := stringValue(meta[liveSignalBarTradeLimitKeyField])
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
	currentPosition := cloneMetadata(mapValue(meta["currentPosition"]))
	if role == "exit" {
		nextSide = normalizeLiveExitIntentSide(nextSide, currentPosition)
		quantity = firstPositive(math.Abs(parseFloatValue(currentPosition["quantity"])), quantity)
	}

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
			"signalBarStateKey":             signalBarStateKey,
			liveSignalBarTradeLimitKeyField: signalBarTradeLimitKey,
			"entryProximityBps":             entryProximityBps,
			"spreadBps":                     spreadBps,
			"ma20":                          ma20,
			"atr14":                         atr14,
			"liquidityBias":                 liquidityBias,
			"biasActionable":                biasActionable,
			"bestBid":                       bestBid,
			"bestAsk":                       bestAsk,
			"bestBidQty":                    bestBidQty,
			"bestAskQty":                    bestAskQty,
			"currentPosition":               currentPosition,
			"bookImbalance":                 parseFloatValue(meta["bookImbalance"]),
		},
	}
}

func normalizeLiveExitIntentSide(plannedSide string, currentPosition map[string]any) string {
	positionSide := strings.ToUpper(strings.TrimSpace(stringValue(currentPosition["side"])))
	switch positionSide {
	case "LONG":
		return "SELL"
	case "SHORT":
		return "BUY"
	default:
		return plannedSide
	}
}

func recordLatestBreakoutSignal(state map[string]any, decision StrategySignalDecision, eventTime time.Time) {
	if state == nil {
		return
	}
	breakout := deriveBreakoutSignalSnapshot(decision, eventTime)
	if len(breakout) == 0 {
		return
	}
	state["lastBreakoutSignal"] = breakout
	history := metadataList(state["breakoutHistory"])
	signature := stringValue(breakout["signature"])
	if len(history) > 0 {
		last := mapValue(history[len(history)-1])
		if stringValue(last["signature"]) == signature {
			state["breakoutHistory"] = history
			return
		}
	}
	history = append(history, breakout)
	if len(history) > 24 {
		history = history[len(history)-24:]
	}
	state["breakoutHistory"] = history
}

func deriveBreakoutSignalSnapshot(decision StrategySignalDecision, eventTime time.Time) map[string]any {
	meta := cloneMetadata(decision.Metadata)
	signalBarDecision := mapValue(meta["signalBarDecision"])
	if len(signalBarDecision) == 0 {
		return nil
	}
	current := mapValue(signalBarDecision["current"])
	prevBar2 := mapValue(signalBarDecision["prevBar2"])
	side := ""
	level := 0.0
	switch {
	case boolValue(signalBarDecision["longBreakoutPatternReady"]):
		side = "BUY"
		level = parseFloatValue(prevBar2["high"])
	case boolValue(signalBarDecision["shortBreakoutPatternReady"]):
		side = "SELL"
		level = parseFloatValue(prevBar2["low"])
	default:
		return nil
	}
	if level <= 0 {
		return nil
	}
	barTime := resolveBreakoutSignalTime(current["barStart"], eventTime)
	return map[string]any{
		"signature":         fmt.Sprintf("%s|%s|%.8f", side, barTime.Format(time.RFC3339), level),
		"side":              side,
		"level":             level,
		"barTime":           barTime.Format(time.RFC3339),
		"eventAt":           eventTime.UTC().Format(time.RFC3339),
		"price":             parseFloatValue(signalBarDecision["breakoutPrice"]),
		"priceSource":       stringValue(signalBarDecision["breakoutPriceSource"]),
		"close":             parseFloatValue(current["close"]),
		"timeframe":         stringValue(signalBarDecision["timeframe"]),
		"signalBarStateKey": stringValue(meta["signalBarStateKey"]),
		"source":            "signal-breakout-price",
	}
}

func resolveBreakoutSignalTime(raw any, fallback time.Time) time.Time {
	if numeric, ok := toFloat64(raw); ok && numeric > 0 {
		return time.UnixMilli(int64(numeric)).UTC()
	}
	if parsed := parseOptionalRFC3339(stringValue(raw)); !parsed.IsZero() {
		return parsed.UTC()
	}
	return fallback.UTC()
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
	proposal = adjustLiveExecutionProposalForVirtualSemantics(session, executionContext.Parameters, proposal)
	proposalMap := assembleLiveExecutionProposalMetadata(session, executionContext.StrategyVersionID, executionProposalToMap(proposal))
	return executionProposalFromMap(proposalMap), nil
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
	hasRecoveredRealPosition := boolValue(session.State["hasRecoveredPosition"]) ||
		boolValue(session.State["hasRecoveredRealPosition"]) ||
		tradingQuantityPositive(math.Abs(parseFloatValue(mapValue(session.State["recoveredPosition"])["quantity"])))
	if strings.EqualFold(proposal.Role, "exit") &&
		boolValue(mapValue(session.State["virtualPosition"])["virtual"]) &&
		!hasRecoveredRealPosition {
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
	if shouldBlockAutoDispatchForRecoveryIntent(session, intent) {
		return false
	}
	if shouldBlockAutoDispatchForLiveEntryTradeLimit(session, intent) {
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
