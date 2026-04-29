package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type RemoteFillsResponse struct {
	OrderID           string                `json:"orderId"`
	AccountID         string                `json:"accountId"`
	Exchange          string                `json:"exchange"`
	Symbol            string                `json:"symbol"`
	ExchangeOrderID   string                `json:"exchangeOrderId"`
	Status            string                `json:"status"`
	QueriedAt         string                `json:"queriedAt"`
	RemoteOrder       map[string]any        `json:"remoteOrder"`
	RemoteTrades      []map[string]any      `json:"remoteTrades"`
	NormalizedReports []LiveFillReport      `json:"normalizedReports"`
	LocalFills        []domain.Fill         `json:"localFills"`
	Raw               map[string]any        `json:"raw,omitempty"`
	Diagnostics       RemoteFillDiagnostics `json:"diagnostics"`
}

type RemoteFillDiagnostics struct {
	HasRealTrades           bool   `json:"hasRealTrades"`
	HasSyntheticLocalFill   bool   `json:"hasSyntheticLocalFill"`
	LocalFeeZero            bool   `json:"localFeeZero"`
	CanSettle               bool   `json:"canSettle"`
	Reason                  string `json:"reason"`
	RemoteTradeCount        int    `json:"remoteTradeCount"`
	LocalFillCount          int    `json:"localFillCount"`
	LocalRealFillCount      int    `json:"localRealFillCount"`
	LocalSyntheticFillCount int    `json:"localSyntheticFillCount"`
}

type ManualFillSyncRequest struct {
	Confirm bool   `json:"confirm"`
	Reason  string `json:"reason"`
	DryRun  bool   `json:"dryRun"`
}

type ManualFillSyncResponse struct {
	OrderID     string                `json:"orderId"`
	DryRun      bool                  `json:"dryRun"`
	SyncedAt    string                `json:"syncedAt,omitempty"`
	Result      string                `json:"result"` // settled | no_remote_trades | already_settled | failed
	Before      FillSyncSnapshot      `json:"before"`
	After       FillSyncSnapshot      `json:"after"`
	Diagnostics RemoteFillDiagnostics `json:"diagnostics"`
}

type FillSyncSnapshot struct {
	FillCount          int     `json:"fillCount"`
	RealFillCount      int     `json:"realFillCount"`
	SyntheticFillCount int     `json:"syntheticFillCount"`
	FeeZeroCount       int     `json:"feeZeroCount"`
	FilledQuantity     float64 `json:"filledQuantity"`
	RemainingQuantity  float64 `json:"remainingQuantity"`
}

// FetchRemoteFills pulls the remote details of an order from the exchange, without modifying local DB.
func (p *Platform) FetchRemoteFills(orderID string) (RemoteFillsResponse, error) {
	order, account, adapter, binding, err := p.resolveLiveOrderContext(orderID)
	if err != nil {
		return RemoteFillsResponse{}, err
	}
	if account.Mode != "LIVE" {
		return RemoteFillsResponse{}, errors.New("remote fills fetching is only supported for LIVE accounts")
	}

	reconcileAdapter, ok := adapter.(LiveAccountReconcileAdapter)
	if !ok {
		return RemoteFillsResponse{}, errors.New("adapter does not support fetching recent trades")
	}

	syncResult, err := adapter.SyncOrder(account, order, binding)
	if err != nil {
		return RemoteFillsResponse{}, fmt.Errorf("failed to sync remote order: %w", err)
	}

	remoteTrades, err := reconcileAdapter.FetchRecentTrades(account, binding, order.Symbol, 0)
	if err != nil {
		return RemoteFillsResponse{}, fmt.Errorf("failed to fetch remote trades: %w", err)
	}

	// Filter remoteTrades to only include trades for this order
	var matchedTrades []LiveFillReport
	var matchedRawTrades []map[string]any

	exchangeOrderID := order.Metadata["exchangeOrderId"]
	if exchangeOrderID == nil && syncResult.Metadata != nil {
		exchangeOrderID = syncResult.Metadata["exchangeOrderId"]
	}

	for _, trade := range remoteTrades {
		tradeExchangeOrderID := ""
		tradeClientOrderID := ""
		if trade.Metadata != nil {
			if eoid, ok := trade.Metadata["exchangeOrderId"]; ok {
				tradeExchangeOrderID = fmt.Sprintf("%v", eoid)
			}
			if coid, ok := trade.Metadata["clientOrderId"]; ok {
				tradeClientOrderID = fmt.Sprintf("%v", coid)
			}
			if coid, ok := trade.Metadata["orderId"]; ok && tradeClientOrderID == "" {
				tradeClientOrderID = fmt.Sprintf("%v", coid)
			}
		}

		if (exchangeOrderID != nil && tradeExchangeOrderID == fmt.Sprintf("%v", exchangeOrderID)) || tradeClientOrderID == order.ID {
			matchedTrades = append(matchedTrades, trade)
			if trade.Metadata != nil && trade.Metadata["raw"] != nil {
				if rawMap, ok := trade.Metadata["raw"].(map[string]any); ok {
					matchedRawTrades = append(matchedRawTrades, rawMap)
				}
			}
		}
	}

	localFills, err := p.store.QueryFills(domain.FillQuery{OrderIDs: []string{orderID}})
	if err != nil {
		return RemoteFillsResponse{}, fmt.Errorf("failed to query local fills: %w", err)
	}

	var raw map[string]any
	rawOrderMap, ok := syncResult.Metadata["raw"].(map[string]any)
	if ok || len(matchedRawTrades) > 0 {
		raw = make(map[string]any)
		if ok {
			raw["order"] = maskSensitiveData(rawOrderMap)
		}
		if len(matchedRawTrades) > 0 {
			var maskedTrades []map[string]any
			for _, t := range matchedRawTrades {
				maskedTrades = append(maskedTrades, maskSensitiveData(t))
			}
			raw["trades"] = maskedTrades
		}
	}

	var remoteOrderMap map[string]any
	if syncResult.Status != "" {
		remoteOrderMap = map[string]any{
			"status":          syncResult.Status,
			"exchangeOrderId": exchangeOrderID,
			"syncedAt":        syncResult.SyncedAt,
		}
	}

	diagnostics := buildRemoteFillDiagnostics(localFills, matchedTrades)

	return RemoteFillsResponse{
		OrderID:           orderID,
		AccountID:         account.ID,
		Exchange:          account.Exchange,
		Symbol:            order.Symbol,
		ExchangeOrderID:   fmt.Sprintf("%v", exchangeOrderID),
		Status:            order.Status,
		QueriedAt:         time.Now().UTC().Format(time.RFC3339),
		RemoteOrder:       remoteOrderMap,
		RemoteTrades:      matchedRawTrades,
		NormalizedReports: matchedTrades,
		LocalFills:        localFills,
		Raw:               raw,
		Diagnostics:       diagnostics,
	}, nil
}

// ManualSyncFills performs a manual sync of fills, either as a dry run or full settlement.
func (p *Platform) ManualSyncFills(orderID string, req ManualFillSyncRequest) (ManualFillSyncResponse, error) {
	order, account, adapter, binding, err := p.resolveLiveOrderContext(orderID)
	if err != nil {
		return ManualFillSyncResponse{}, err
	}
	if account.Mode != "LIVE" {
		return ManualFillSyncResponse{}, errors.New("manual sync is only supported for LIVE accounts")
	}

	reconcileAdapter, ok := adapter.(LiveAccountReconcileAdapter)
	if !ok {
		return ManualFillSyncResponse{}, errors.New("adapter does not support fetching recent trades")
	}

	if !req.DryRun {
		if !req.Confirm {
			return ManualFillSyncResponse{}, errors.New("confirmation required for manual sync")
		}
		if strings.TrimSpace(req.Reason) == "" {
			return ManualFillSyncResponse{}, errors.New("reason required for manual sync")
		}
	}

	localFillsBefore, err := p.store.QueryFills(domain.FillQuery{OrderIDs: []string{orderID}})
	if err != nil {
		return ManualFillSyncResponse{}, err
	}
	beforeSnapshot := buildFillSyncSnapshot(order, localFillsBefore)

	if req.DryRun {
		// Just simulate by fetching remote trades
		syncResult, err := adapter.SyncOrder(account, order, binding)
		if err != nil {
			return ManualFillSyncResponse{}, fmt.Errorf("dry-run failed to sync remote order: %w", err)
		}
		remoteTrades, err := reconcileAdapter.FetchRecentTrades(account, binding, order.Symbol, 0)
		if err != nil {
			return ManualFillSyncResponse{}, fmt.Errorf("dry-run failed to fetch remote trades: %w", err)
		}

		exchangeOrderID := order.Metadata["exchangeOrderId"]
		if exchangeOrderID == nil && syncResult.Metadata != nil {
			exchangeOrderID = syncResult.Metadata["exchangeOrderId"]
		}

		var matchedTrades []LiveFillReport
		for _, trade := range remoteTrades {
			tradeExchangeOrderID := ""
			tradeClientOrderID := ""
			if trade.Metadata != nil {
				if eoid, ok := trade.Metadata["exchangeOrderId"]; ok {
					tradeExchangeOrderID = fmt.Sprintf("%v", eoid)
				}
				if coid, ok := trade.Metadata["clientOrderId"]; ok {
					tradeClientOrderID = fmt.Sprintf("%v", coid)
				}
				if coid, ok := trade.Metadata["orderId"]; ok && tradeClientOrderID == "" {
					tradeClientOrderID = fmt.Sprintf("%v", coid)
				}
			}
			if (exchangeOrderID != nil && tradeExchangeOrderID == fmt.Sprintf("%v", exchangeOrderID)) || tradeClientOrderID == order.ID {
				matchedTrades = append(matchedTrades, trade)
			}
		}

		diagnostics := buildRemoteFillDiagnostics(localFillsBefore, matchedTrades)

		afterSnapshot := beforeSnapshot
		result := "dry_run_no_changes"
		if diagnostics.CanSettle {
			afterSnapshot.FillCount = len(matchedTrades)
			afterSnapshot.RealFillCount = len(matchedTrades)
			afterSnapshot.SyntheticFillCount = 0
			result = "dry_run_can_settle"
		} else if !diagnostics.HasRealTrades {
			result = "dry_run_no_remote_trades"
		} else {
			result = "dry_run_already_settled"
		}

		return ManualFillSyncResponse{
			OrderID:     orderID,
			DryRun:      true,
			Result:      result,
			Before:      beforeSnapshot,
			After:       afterSnapshot,
			Diagnostics: diagnostics,
		}, nil
	}

	// Actual Sync: use existing SyncLiveOrder, but first we can annotate metadata
	if order.Metadata == nil {
		order.Metadata = make(map[string]any)
	}
	order.Metadata["manualFillSync"] = map[string]any{
		"reason": req.Reason,
		"time":   time.Now().UTC().Format(time.RFC3339),
	}
	_, err = p.store.UpdateOrder(order)
	if err != nil {
		return ManualFillSyncResponse{}, fmt.Errorf("failed to save manual sync metadata: %w", err)
	}

	syncedOrder, err := p.SyncLiveOrder(orderID)
	if err != nil {
		return ManualFillSyncResponse{}, fmt.Errorf("manual sync failed during SyncLiveOrder: %w", err)
	}

	localFillsAfter, err := p.store.QueryFills(domain.FillQuery{OrderIDs: []string{orderID}})
	if err != nil {
		return ManualFillSyncResponse{}, err
	}
	afterSnapshot := buildFillSyncSnapshot(syncedOrder, localFillsAfter)

	remoteTrades, _ := reconcileAdapter.FetchRecentTrades(account, binding, syncedOrder.Symbol, 0)
	var matchedTrades []LiveFillReport
	exchangeOrderID := syncedOrder.Metadata["exchangeOrderId"]
	for _, trade := range remoteTrades {
		tradeExchangeOrderID := ""
		tradeClientOrderID := ""
		if trade.Metadata != nil {
			if eoid, ok := trade.Metadata["exchangeOrderId"]; ok {
				tradeExchangeOrderID = fmt.Sprintf("%v", eoid)
			}
			if coid, ok := trade.Metadata["clientOrderId"]; ok {
				tradeClientOrderID = fmt.Sprintf("%v", coid)
			}
			if coid, ok := trade.Metadata["orderId"]; ok && tradeClientOrderID == "" {
				tradeClientOrderID = fmt.Sprintf("%v", coid)
			}
		}
		if (exchangeOrderID != nil && tradeExchangeOrderID == fmt.Sprintf("%v", exchangeOrderID)) || tradeClientOrderID == syncedOrder.ID {
			matchedTrades = append(matchedTrades, trade)
		}
	}
	diagnostics := buildRemoteFillDiagnostics(localFillsAfter, matchedTrades)

	result := "settled"
	if afterSnapshot.RealFillCount == beforeSnapshot.RealFillCount && afterSnapshot.SyntheticFillCount == beforeSnapshot.SyntheticFillCount {
		result = "already_settled"
		if !diagnostics.HasRealTrades {
			result = "no_remote_trades"
		}
	}

	return ManualFillSyncResponse{
		OrderID:     orderID,
		DryRun:      false,
		SyncedAt:    time.Now().UTC().Format(time.RFC3339),
		Result:      result,
		Before:      beforeSnapshot,
		After:       afterSnapshot,
		Diagnostics: diagnostics,
	}, nil
}

func buildFillSyncSnapshot(order domain.Order, fills []domain.Fill) FillSyncSnapshot {
	var snap FillSyncSnapshot
	snap.FillCount = len(fills)
	for _, f := range fills {
		snap.FilledQuantity += f.Quantity
		if f.Source == string(FillSourceReal) {
			snap.RealFillCount++
		} else if f.Source == string(FillSourceSynthetic) {
			snap.SyntheticFillCount++
		}
		if f.Fee == 0 {
			snap.FeeZeroCount++
		}
	}
	snap.RemainingQuantity = order.Quantity - snap.FilledQuantity
	return snap
}

func buildRemoteFillDiagnostics(localFills []domain.Fill, remoteTrades []LiveFillReport) RemoteFillDiagnostics {
	var diag RemoteFillDiagnostics
	diag.LocalFillCount = len(localFills)
	diag.RemoteTradeCount = len(remoteTrades)
	diag.HasRealTrades = len(remoteTrades) > 0

	for _, f := range localFills {
		if f.Source == string(FillSourceSynthetic) {
			diag.HasSyntheticLocalFill = true
			diag.LocalSyntheticFillCount++
		} else if f.Source == string(FillSourceReal) {
			diag.LocalRealFillCount++
		}
		if f.Fee == 0 {
			diag.LocalFeeZero = true
		}
	}

	if diag.HasRealTrades && (diag.HasSyntheticLocalFill || diag.LocalFeeZero || diag.LocalRealFillCount < diag.RemoteTradeCount) {
		diag.CanSettle = true
		diag.Reason = "remote trades available to replace synthetic or incomplete fills"
	} else if !diag.HasRealTrades {
		diag.Reason = "no remote trades found"
	} else {
		diag.Reason = "local fills already fully settled with remote trades"
	}

	return diag
}

func maskSensitiveData(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	masked := make(map[string]any)
	for k, v := range data {
		lowerK := strings.ToLower(k)
		if strings.Contains(lowerK, "key") || strings.Contains(lowerK, "secret") || strings.Contains(lowerK, "signature") || strings.Contains(lowerK, "token") {
			masked[k] = "***"
		} else if m, ok := v.(map[string]any); ok {
			masked[k] = maskSensitiveData(m)
		} else if arr, ok := v.([]any); ok {
			var newArr []any
			for _, item := range arr {
				if mItem, okItem := item.(map[string]any); okItem {
					newArr = append(newArr, maskSensitiveData(mItem))
				} else {
					newArr = append(newArr, item)
				}
			}
			masked[k] = newArr
		} else {
			masked[k] = v
		}
	}
	return masked
}
