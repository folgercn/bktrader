package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

// LiveRecoveryDiagnoseOptions 诊断选项
type LiveRecoveryDiagnoseOptions struct {
	AccountID     string `json:"accountId"`
	Symbol        string `json:"symbol"`
	SessionID     string `json:"sessionId,omitempty"`
	LookbackHours int    `json:"lookbackHours"`
}

// LiveRecoveryFact 状态事实
type LiveRecoveryFact struct {
	Source        string         `json:"source"` // "exchange" 或 "db"
	Symbol        string         `json:"symbol"`
	Position      map[string]any `json:"position"`
	OpenOrders    []any          `json:"openOrders"`
	RecentOrders  []any          `json:"recentOrders"`
	RecentFills   []any          `json:"recentFills"`
	ReconcileGate map[string]any `json:"reconcileGate,omitempty"`
	SyncedAt      time.Time      `json:"syncedAt"`
}

// LiveRecoveryMismatch 差异分类
type LiveRecoveryMismatch struct {
	Scenario       string   `json:"scenario"` // 例如 "exchange-flat-db-position-present"
	Level          string   `json:"level"`    // "critical", "warning", "info"
	Message        string   `json:"message"`
	MismatchFields []string `json:"mismatchFields,omitempty"`
}

// LiveRecoveryActionCandidate 可选修复动作
type LiveRecoveryActionCandidate struct {
	Action      string         `json:"action"` // "reconcile", "sync-orders", "clear-stale-position", "adopt-exchange-position"
	Label       string         `json:"label"`
	Description string         `json:"description"`
	Allowed     bool           `json:"allowed"`
	BlockedBy   string         `json:"blockedBy,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"` // 执行动作所需的参数
}

// LiveRecoveryDiagnoseResult 诊断结果
type LiveRecoveryDiagnoseResult struct {
	AccountID     string                        `json:"accountId"`
	Symbol        string                        `json:"symbol"`
	ExchangeFact  LiveRecoveryFact              `json:"exchangeFact"`
	DBFact        LiveRecoveryFact              `json:"dbFact"`
	Mismatches    []LiveRecoveryMismatch        `json:"mismatches"`
	Actions       []LiveRecoveryActionCandidate `json:"actions"`
	Authoritative bool                          `json:"authoritative"`
	RuntimeRole   string                        `json:"runtimeRole"` // BKTRADER_ROLE
	DiagnosedAt   time.Time                     `json:"diagnosedAt"`
}

// DiagnoseLiveRecovery 执行实盘恢复诊断
func (p *Platform) DiagnoseLiveRecovery(ctx context.Context, options LiveRecoveryDiagnoseOptions) (LiveRecoveryDiagnoseResult, error) {
	account, err := p.store.GetAccount(options.AccountID)
	if err != nil {
		return LiveRecoveryDiagnoseResult{}, err
	}

	symbol := NormalizeSymbol(options.Symbol)
	lookback := options.LookbackHours
	if lookback <= 0 {
		lookback = 24
	}

	result := LiveRecoveryDiagnoseResult{
		AccountID:   account.ID,
		Symbol:      symbol,
		RuntimeRole: p.processRole,
		DiagnosedAt: time.Now().UTC(),
	}

	// 1. 获取本地数据库事实
	dbFact, err := p.fetchDBFact(account.ID, symbol, lookback)
	if err != nil {
		return result, fmt.Errorf("fetch db fact failed: %w", err)
	}
	result.DBFact = dbFact

	// 2. 获取交易所权威事实
	exchangeFact, err := p.fetchExchangeFact(account, symbol, lookback)
	if err != nil {
		// 如果获取交易所事实失败，标记权威性为 false
		result.Authoritative = false
		return result, fmt.Errorf("fetch exchange fact failed: %w", err)
	}
	result.ExchangeFact = exchangeFact
	result.Authoritative = true // 成功获取 REST 事实即视为权威

	// 3. 评估差异与分类
	result.Mismatches = p.classifyRecoveryMismatches(result.DBFact, result.ExchangeFact)

	// 4. 生成可选修复动作
	result.Actions = p.generateRecoveryActions(account, symbol, result.DBFact, result.ExchangeFact, result.Mismatches)

	return result, nil
}

func (p *Platform) fetchDBFact(accountID, symbol string, lookback int) (LiveRecoveryFact, error) {
	fact := LiveRecoveryFact{
		Source:   "db",
		Symbol:   symbol,
		SyncedAt: time.Now().UTC(),
	}

	// 持仓
	pos, found, err := p.store.FindPosition(accountID, symbol)
	if err != nil {
		return fact, err
	}
	if found && tradingQuantityPositive(pos.Quantity) {
		fact.Position = buildRecoveredLivePositionStateSnapshot(pos)
	} else {
		fact.Position = map[string]any{}
	}

	// 挂单
	orders, err := p.store.QueryOrders(domain.OrderQuery{
		AccountID: accountID,
		Symbols:   []string{symbol},
		Statuses:  []string{"NEW", "PARTIALLY_FILLED"},
	})
	if err == nil {
		fact.OpenOrders = make([]any, len(orders))
		for i, o := range orders {
			fact.OpenOrders[i] = o
		}
	}

	// 最近订单
	since := time.Now().Add(-time.Duration(lookback) * time.Hour)
	recentOrders, err := p.store.QueryOrders(domain.OrderQuery{
		AccountID: accountID,
		Symbols:   []string{symbol},
		Limit:     50,
	})
	if err == nil {
		fact.RecentOrders = make([]any, 0)
		orderIDs := make([]string, 0)
		for _, o := range recentOrders {
			if o.CreatedAt.After(since) {
				fact.RecentOrders = append(fact.RecentOrders, o)
				orderIDs = append(orderIDs, o.ID)
			}
		}

		// 最近成交
		if len(orderIDs) > 0 {
			fills, err := p.store.QueryFills(domain.FillQuery{
				OrderIDs: orderIDs,
			})
			if err == nil {
				fact.RecentFills = make([]any, len(fills))
				for i, f := range fills {
					fact.RecentFills[i] = f
				}
			}
		}
	}

	// 对账门
	account, _ := p.store.GetAccount(accountID)
	if account.ID != "" {
		fact.ReconcileGate = resolveLivePositionReconcileGate(account, symbol, true)
	}

	return fact, nil
}

func (p *Platform) fetchExchangeFact(account domain.Account, symbol string, lookback int) (LiveRecoveryFact, error) {
	fact := LiveRecoveryFact{
		Source:   "exchange",
		Symbol:   symbol,
		SyncedAt: time.Now().UTC(),
	}

	adapter, binding, err := p.resolveLiveAdapterForAccount(account)
	if err != nil {
		return fact, err
	}

	// 仓位风险
	if posAdapter, ok := adapter.(LiveAccountSyncAdapter); ok {
		synced, err := posAdapter.SyncAccountSnapshot(p, account, binding)
		if err == nil {
			snapshot := mapValue(synced.Metadata["liveSyncSnapshot"])
			exchangePositions := metadataList(snapshot["positions"])
			for _, item := range exchangePositions {
				if NormalizeSymbol(stringValue(item["symbol"])) == symbol {
					positionAmt := parseFloatValue(item["positionAmt"])
					if positionAmt != 0 {
						side := "LONG"
						if positionAmt < 0 {
							side = "SHORT"
						}
						fact.Position = map[string]any{
							"symbol":     symbol,
							"side":       side,
							"quantity":   math.Abs(positionAmt),
							"entryPrice": parseFloatValue(item["entryPrice"]),
							"markPrice":  parseFloatValue(item["markPrice"]),
						}
					}
					break
				}
			}
		}
	}
	if fact.Position == nil {
		fact.Position = map[string]any{}
	}

	// 挂单与最近订单
	if recAdapter, ok := adapter.(LiveAccountReconcileAdapter); ok {
		orders, err := recAdapter.FetchRecentOrders(account, binding, symbol, lookback)
		if err == nil {
			fact.RecentOrders = make([]any, len(orders))
			fact.OpenOrders = make([]any, 0)
			for i, o := range orders {
				fact.RecentOrders[i] = o
				status := strings.ToUpper(stringValue(o["status"]))
				if status == "NEW" || status == "PARTIALLY_FILLED" {
					fact.OpenOrders = append(fact.OpenOrders, o)
				}
			}
		}

		trades, err := recAdapter.FetchRecentTrades(account, binding, symbol, lookback)
		if err == nil {
			fact.RecentFills = make([]any, len(trades))
			for i, t := range trades {
				fact.RecentFills[i] = t
			}
		}
	}

	return fact, nil
}

func (p *Platform) classifyRecoveryMismatches(dbFact, exFact LiveRecoveryFact) []LiveRecoveryMismatch {
	mismatches := make([]LiveRecoveryMismatch, 0)

	dbQty := parseFloatValue(dbFact.Position["quantity"])
	exQty := parseFloatValue(exFact.Position["quantity"])
	dbSide := strings.ToUpper(stringValue(dbFact.Position["side"]))
	exSide := strings.ToUpper(stringValue(exFact.Position["side"]))

	// 场景 1: 交易所已平仓，本地仍有持仓 (Stale Position)
	if !tradingQuantityPositive(exQty) && tradingQuantityPositive(dbQty) {
		mismatches = append(mismatches, LiveRecoveryMismatch{
			Scenario: "exchange-flat-db-position-present",
			Level:    "critical",
			Message:  "交易所已无持仓，但本地数据库仍显示活跃仓位（陈旧数据）。",
		})
	}

	// 场景 2: 交易所持仓活跃，本地无持仓 (Missing Position)
	if tradingQuantityPositive(exQty) && !tradingQuantityPositive(dbQty) {
		mismatches = append(mismatches, LiveRecoveryMismatch{
			Scenario: "exchange-position-db-missing",
			Level:    "critical",
			Message:  "交易所存在活跃持仓，但本地数据库中缺失该仓位。",
		})
	}

	// 场景 3: 数量不匹配 (Quantity Mismatch)
	if tradingQuantityPositive(exQty) && tradingQuantityPositive(dbQty) && tradingQuantityDiffers(exQty, dbQty) {
		mismatches = append(mismatches, LiveRecoveryMismatch{
			Scenario:       "quantity-mismatch",
			Level:          "warning",
			Message:        fmt.Sprintf("仓位数量不匹配：交易所 %.4f vs 本地 %.4f。", exQty, dbQty),
			MismatchFields: []string{"quantity"},
		})
	}

	// 场景 4: 方向冲突 (Side Conflict)
	if tradingQuantityPositive(exQty) && tradingQuantityPositive(dbQty) && dbSide != exSide {
		mismatches = append(mismatches, LiveRecoveryMismatch{
			Scenario:       "side-conflict",
			Level:          "critical",
			Message:        fmt.Sprintf("仓位方向冲突：交易所 %s vs 本地 %s。", exSide, dbSide),
			MismatchFields: []string{"side"},
		})
	}

	// 场景 5: 对账门阻塞
	if boolValue(dbFact.ReconcileGate["blocking"]) {
		mismatches = append(mismatches, LiveRecoveryMismatch{
			Scenario: "reconcile-gate-blocked",
			Level:    "warning",
			Message:  fmt.Sprintf("对账门已阻塞：%s (%s)。", stringValue(dbFact.ReconcileGate["status"]), stringValue(dbFact.ReconcileGate["scenario"])),
		})
	}

	// 场景 6: 存在未结订单 (Working Orders)
	if len(exFact.OpenOrders) > 0 || len(dbFact.OpenOrders) > 0 {
		mismatches = append(mismatches, LiveRecoveryMismatch{
			Scenario: "working-orders-present",
			Level:    "info",
			Message:  fmt.Sprintf("检测到未结订单：交易所 %d 笔，本地 %d 笔。", len(exFact.OpenOrders), len(dbFact.OpenOrders)),
		})
	}

	return mismatches
}

func (p *Platform) generateRecoveryActions(account domain.Account, symbol string, dbFact, exFact LiveRecoveryFact, mismatches []LiveRecoveryMismatch) []LiveRecoveryActionCandidate {
	actions := make([]LiveRecoveryActionCandidate, 0)

	// 通用动作：重新对账
	actions = append(actions, LiveRecoveryActionCandidate{
		Action:      "reconcile",
		Label:       "重新对账账户",
		Description: "触发完整的账户级对账，从交易所拉取最近订单和成交并更新本地数据库状态。",
		Allowed:     true,
	})

	// 通用动作：同步终端订单
	actions = append(actions, LiveRecoveryActionCandidate{
		Action:      "sync-orders",
		Label:       "同步终端订单",
		Description: "同步该交易对下所有非终端状态的订单。",
		Allowed:     true,
	})

	// 特殊动作：清除陈旧仓位
	hasStale := false
	for _, m := range mismatches {
		if m.Scenario == "exchange-flat-db-position-present" {
			hasStale = true
			break
		}
	}

	clearAllowed := hasStale
	clearBlockedBy := ""
	if clearAllowed {
		// 阻断条件：必须没有交易所挂单
		if len(exFact.OpenOrders) > 0 {
			clearAllowed = false
			clearBlockedBy = "exchange-open-orders-present"
		}
		// 阻断条件：必须没有待结算订单
		pendingSettlementSymbols, _ := p.liveSettlementPendingOrderSymbols(account.ID)
		if _, pending := pendingSettlementSymbols[symbol]; pending {
			clearAllowed = false
			clearBlockedBy = "pending-settlement"
		}
	}

	actions = append(actions, LiveRecoveryActionCandidate{
		Action:      "clear-stale-position",
		Label:       "清除本地陈旧持仓",
		Description: "当交易所已完全平仓且无挂单时，安全地删除本地冗余的活跃持仓记录。",
		Allowed:     clearAllowed,
		BlockedBy:   clearBlockedBy,
		Payload: map[string]any{
			"positionId": stringValue(dbFact.Position["id"]),
			"symbol":     symbol,
		},
	})

	// 特殊动作：采纳交易所仓位
	hasMissing := false
	for _, m := range mismatches {
		if m.Scenario == "exchange-position-db-missing" {
			hasMissing = true
			break
		}
	}

	adoptAllowed := hasMissing
	adoptBlockedBy := ""
	if adoptAllowed {
		if len(dbFact.OpenOrders) > 0 {
			adoptAllowed = false
			adoptBlockedBy = "db-working-orders-present"
		}
	}

	actions = append(actions, LiveRecoveryActionCandidate{
		Action:      "adopt-exchange-position",
		Label:       "采纳交易所仓位",
		Description: "当交易所存在仓位导致本地缺失时，将交易所的仓位信息导入本地数据库。",
		Allowed:     adoptAllowed,
		BlockedBy:   adoptBlockedBy,
		Payload: map[string]any{
			"symbol": symbol,
		},
	})

	// 特殊动作：重置对账门
	actions = append(actions, LiveRecoveryActionCandidate{
		Action:      "reset-reconcile-gate",
		Label:       "重置对账门状态",
		Description: "在完成上述修复后，强制刷新账户的对账门状态，使其恢复为就绪（Verified）。",
		Allowed:     true,
	})

	return actions
}

// ExecuteLiveRecoveryAction 执行特定的修复动作
func (p *Platform) ExecuteLiveRecoveryAction(ctx context.Context, accountID, action string, payload map[string]any) (map[string]any, error) {
	account, err := p.store.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	symbol := NormalizeSymbol(stringValue(payload["symbol"]))
	p.logger("service.live_recovery_workbench", "account_id", accountID, "action", action, "symbol", symbol).Info("executing recovery action")

	switch action {
	case "reconcile":
		lookbackHours := 24
		if h, ok := payload["lookbackHours"].(int); ok && h > 0 {
			lookbackHours = h
		}
		result, err := p.ReconcileLiveAccount(account.ID, LiveAccountReconcileOptions{LookbackHours: lookbackHours})
		if err != nil {
			return nil, err
		}
		return map[string]any{"result": result}, nil

	case "sync-orders":
		// 同步该交易对下的所有订单
		orders, err := p.store.QueryOrders(domain.OrderQuery{
			AccountID: accountID,
			Symbols:   []string{symbol},
		})
		if err != nil {
			return nil, err
		}
		count := 0
		for _, o := range orders {
			if !isTerminalOrderStatus(o.Status) {
				_, err := p.SyncLiveOrder(o.ID)
				if err == nil {
					count++
				}
			}
		}
		return map[string]any{"syncedCount": count}, nil

	case "clear-stale-position":
		return p.executeClearStalePosition(account, symbol, payload)

	case "adopt-exchange-position":
		return p.executeAdoptExchangePosition(account, symbol, payload)

	case "reset-reconcile-gate":
		updated, err := p.refreshLiveAccountPositionReconcileGate(account)
		if err != nil {
			return nil, err
		}
		return map[string]any{"account": updated}, nil

	default:
		return nil, fmt.Errorf("unknown recovery action: %s", action)
	}
}

func (p *Platform) executeClearStalePosition(account domain.Account, symbol string, payload map[string]any) (map[string]any, error) {
	// 二次校验：再次从交易所获取事实
	exFact, err := p.fetchExchangeFact(account, symbol, 1)
	if err != nil {
		return nil, fmt.Errorf("double-check exchange fact failed: %w", err)
	}

	// 强制安全检查
	if tradingQuantityPositive(parseFloatValue(exFact.Position["quantity"])) {
		return nil, fmt.Errorf("safety check failed: exchange position is still active for %s", symbol)
	}
	if len(exFact.OpenOrders) > 0 {
		return nil, fmt.Errorf("safety check failed: exchange open orders still exist for %s", symbol)
	}

	pos, found, err := p.store.FindPosition(account.ID, symbol)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("local position for %s not found", symbol)
	}

	// 确认交易所无持仓后，直接删除本地持仓记录，保持 DB 洁净
	if err := p.store.DeletePosition(pos.ID); err != nil {
		return nil, err
	}
	p.logger("service.live_recovery_workbench", "account_id", account.ID, "symbol", symbol).Warn("stale position deleted manually via workbench", "position_id", pos.ID)

	// 自动刷新对账门
	p.refreshLiveAccountPositionReconcileGate(account)
	return map[string]any{"action": "clear-stale-position", "symbol": symbol, "status": "deleted"}, nil
}

func (p *Platform) executeAdoptExchangePosition(account domain.Account, symbol string, payload map[string]any) (map[string]any, error) {
	// 重新执行对账即可实现采纳（reconcile 会自动 SavePosition）
	result, err := p.ReconcileLiveAccount(account.ID, LiveAccountReconcileOptions{LookbackHours: 2})
	if err != nil {
		return nil, err
	}

	p.logger("service.live_recovery_workbench", "account_id", account.ID, "symbol", symbol).Info("exchange position adopted via reconcile")

	return map[string]any{"adopted": true, "reconcileResult": result}, nil
}
