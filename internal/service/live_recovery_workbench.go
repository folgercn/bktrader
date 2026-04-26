// (省略未改动部分)

// ExecuteLiveRecoveryAction 执行特定的修复动作（安全版）
func (p *Platform) ExecuteLiveRecoveryAction(ctx context.Context, accountID, action string, payload map[string]any) (map[string]any, error) {
	account, err := p.store.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	symbol := NormalizeSymbol(stringValue(payload["symbol"]))

	// 🔥 核心安全：重新诊断 + 校验 action 是否允许
	diag, err := p.DiagnoseLiveRecovery(ctx, LiveRecoveryDiagnoseOptions{
		AccountID: accountID,
		Symbol:    symbol,
	})
	if err != nil {
		return nil, fmt.Errorf("pre-check diagnose failed: %w", err)
	}

	allowed := false
	for _, a := range diag.Actions {
		if a.Action == action && a.Allowed {
			allowed = true
			break
		}
	}

	if !allowed {
		return nil, fmt.Errorf("action %s is not allowed under current state", action)
	}

	p.logger("service.live_recovery_workbench", "account_id", accountID, "action", action, "symbol", symbol).Warn("validated recovery action execution")

	switch action {
	case "reconcile":
		result, err := p.ReconcileLiveAccount(account.ID, LiveAccountReconcileOptions{LookbackHours: 24})
		if err != nil {
			return nil, err
		}
		return map[string]any{"result": result}, nil

	case "sync-orders":
		if symbol == "" {
			return nil, fmt.Errorf("symbol required for sync-orders")
		}
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
		if len(diag.Mismatches) > 0 {
			return nil, fmt.Errorf("cannot reset reconcile gate while mismatches still exist")
		}
		updated, err := p.refreshLiveAccountPositionReconcileGate(account)
		if err != nil {
			return nil, err
		}
		return map[string]any{"account": updated}, nil

	default:
		return nil, fmt.Errorf("unknown recovery action: %s", action)
	}
}
