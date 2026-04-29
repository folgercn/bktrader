package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	storepkg "github.com/wuyaocheng/bktrader/internal/store"
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
	Result      string                `json:"result"` // settled | already_settled | failed | dry_run_can_settle | ...
	Before      FillSyncSnapshot      `json:"before"`
	After       FillSyncSnapshot      `json:"after"`
	Diagnostics RemoteFillDiagnostics `json:"diagnostics"`
	Changes     FillRebuildChanges    `json:"changes,omitempty"`
}

type FillSyncSnapshot struct {
	FillCount          int     `json:"fillCount"`
	RealFillCount      int     `json:"realFillCount"`
	SyntheticFillCount int     `json:"syntheticFillCount"`
	FeeZeroCount       int     `json:"feeZeroCount"`
	FilledQuantity     float64 `json:"filledQuantity"`
	RemainingQuantity  float64 `json:"remainingQuantity"`
}

type FillRebuildChanges struct {
	DeletedSyntheticCount int      `json:"deletedSyntheticCount"`
	AddedRealCount        int      `json:"addedRealCount"`
	DuplicateTradeIDs     []string `json:"duplicateTradeIds,omitempty"`
	NewTradeIDs           []string `json:"newTradeIds,omitempty"`
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

	exchangeOrderID := order.Metadata["exchangeOrderId"]
	if exchangeOrderID == nil && syncResult.Metadata != nil {
		exchangeOrderID = syncResult.Metadata["exchangeOrderId"]
	}

	matchedTrades := matchRemoteTrades(order.ID, fmt.Sprintf("%v", exchangeOrderID), remoteTrades)

	localFills, err := p.store.QueryFills(domain.FillQuery{OrderIDs: []string{orderID}})
	if err != nil {
		return RemoteFillsResponse{}, fmt.Errorf("failed to query local fills: %w", err)
	}

	// Prepare raw and masked data
	var raw map[string]any
	var maskedTrades []map[string]any
	var matchedRawTrades []map[string]any

	for _, t := range matchedTrades {
		if t.Metadata != nil && t.Metadata["raw"] != nil {
			if rawMap, ok := t.Metadata["raw"].(map[string]any); ok {
				matchedRawTrades = append(matchedRawTrades, rawMap)
				maskedTrades = append(maskedTrades, maskSensitiveData(rawMap))
			}
		}
	}

	rawOrderMap, ok := syncResult.Metadata["raw"].(map[string]any)
	if ok || len(maskedTrades) > 0 {
		raw = make(map[string]any)
		if ok {
			raw["order"] = maskSensitiveData(rawOrderMap)
		}
		if len(maskedTrades) > 0 {
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
		RemoteTrades:      maskedTrades, // Use masked versions for response
		NormalizedReports: maskReportsSensitiveData(matchedTrades),
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

	// Fetch remote data first
	syncResult, err := adapter.SyncOrder(account, order, binding)
	if err != nil {
		return ManualFillSyncResponse{}, fmt.Errorf("failed to sync remote order: %w", err)
	}
	remoteTrades, err := reconcileAdapter.FetchRecentTrades(account, binding, order.Symbol, 0)
	if err != nil {
		return ManualFillSyncResponse{}, fmt.Errorf("failed to fetch remote trades: %w", err)
	}

	exchangeOrderID := order.Metadata["exchangeOrderId"]
	if exchangeOrderID == nil && syncResult.Metadata != nil {
		exchangeOrderID = syncResult.Metadata["exchangeOrderId"]
	}

	matchedTrades := matchRemoteTrades(order.ID, fmt.Sprintf("%v", exchangeOrderID), remoteTrades)

	// Perform rebuild (dry-run or real)
	resp, err := p.RebuildOrderFills(order.ID, matchedTrades, req.Reason, req.DryRun)
	if err != nil {
		return ManualFillSyncResponse{}, err
	}

	return *resp, nil
}

// RebuildOrderFills performs an atomic rebuild of fills for an order based on remote trades.
func (p *Platform) RebuildOrderFills(orderID string, matchedTrades []LiveFillReport, reason string, dryRun bool) (*ManualFillSyncResponse, error) {
	order, _, _, _, err := p.resolveLiveOrderContext(orderID)
	if err != nil {
		return nil, err
	}

	localFillsBefore, err := p.store.QueryFills(domain.FillQuery{OrderIDs: []string{orderID}})
	if err != nil {
		return nil, err
	}
	beforeSnapshot := buildFillSyncSnapshot(order, localFillsBefore)
	diagnostics := buildRemoteFillDiagnostics(localFillsBefore, matchedTrades)

	var changes FillRebuildChanges
	var localFillsAfter []domain.Fill
	var syncedOrder domain.Order
	var totalFilledAfter float64

	if dryRun {
		// Simulate rebuild
		changes.DeletedSyntheticCount = diagnostics.LocalSyntheticFillCount
		existingRealMap := make(map[string]bool)
		for _, f := range localFillsBefore {
			if f.Source == string(FillSourceReal) && f.ExchangeTradeID != "" {
				existingRealMap[f.ExchangeTradeID] = true
			}
		}

		localFillsAfter = make([]domain.Fill, 0)
		// Keep existing real ones
		for _, f := range localFillsBefore {
			if f.Source == string(FillSourceReal) {
				localFillsAfter = append(localFillsAfter, f)
			}
		}
		// Add new ones from matched trades
		for _, t := range matchedTrades {
			tradeID := fmt.Sprintf("%v", t.Metadata["exchangeTradeId"])
			if existingRealMap[tradeID] {
				changes.DuplicateTradeIDs = append(changes.DuplicateTradeIDs, tradeID)
			} else {
				changes.AddedRealCount++
				changes.NewTradeIDs = append(changes.NewTradeIDs, tradeID)
				// Mock a fill for snapshot
				localFillsAfter = append(localFillsAfter, domain.Fill{
					Quantity: t.Quantity,
					Price:    t.Price,
					Fee:      t.Fee,
					Source:   string(FillSourceReal),
				})
			}
		}
		syncedOrder = order
		totalFilledAfter = 0
		for _, f := range localFillsAfter {
			totalFilledAfter += f.Quantity
		}
	} else {
		// Real rebuild in transaction
		err = p.store.WithFillSettlementTx(orderID, func(tx storepkg.FillSettlementStore) error {
			// 1. Delete synthetic
			deletedQty, err := tx.DeleteSyntheticFillsForOrder(orderID)
			if err != nil {
				return err
			}
			_ = deletedQty
			changes.DeletedSyntheticCount = diagnostics.LocalSyntheticFillCount

			// 2. Query remaining real fills to check duplicates
			existingFills, err := tx.QueryFills(domain.FillQuery{OrderIDs: []string{orderID}})
			if err != nil {
				return err
			}
			existingRealMap := make(map[string]bool)
			for _, f := range existingFills {
				if f.ExchangeTradeID != "" {
					existingRealMap[f.ExchangeTradeID] = true
				}
			}

			// 3. Create real fills
			for _, t := range matchedTrades {
				tradeID := fmt.Sprintf("%v", t.Metadata["exchangeTradeId"])
				if existingRealMap[tradeID] {
					changes.DuplicateTradeIDs = append(changes.DuplicateTradeIDs, tradeID)
					continue
				}

				changes.AddedRealCount++
				changes.NewTradeIDs = append(changes.NewTradeIDs, tradeID)

				tradeTime := time.Now().UTC()
				if tt, ok := t.Metadata["exchangeTradeTime"].(time.Time); ok {
					tradeTime = tt
				}

				_, err = tx.CreateFill(domain.Fill{
					OrderID:           orderID,
					ExchangeTradeID:   tradeID,
					ExchangeTradeTime: &tradeTime,
					Price:             t.Price,
					Quantity:          t.Quantity,
					Fee:               t.Fee,
					Source:            string(FillSourceReal),
				})
				if err != nil {
					return err
				}
			}

			// 4. Update Order Metadata Audit
			totalFilledAfter, err = tx.TotalFilledQuantityForOrder(orderID)
			if err != nil {
				return err
			}

			if order.Metadata == nil {
				order.Metadata = make(map[string]any)
			}
			// Update audit history
			historyEntry := map[string]any{
				"time":    time.Now().UTC().Format(time.RFC3339),
				"reason":  reason,
				"changes": changes,
				"before":  beforeSnapshot,
			}
			historyRaw := order.Metadata["manualFillSyncHistory"]
			history, _ := historyRaw.([]any)
			history = append(history, historyEntry)
			order.Metadata["manualFillSyncHistory"] = history
			order.Metadata["manualFillSync"] = historyEntry // Backward compatibility for single last op

			syncedOrder, err = tx.UpdateOrder(order)
			if err != nil {
				return err
			}

			// 5. Update Position
			pos, exists, err := tx.FindPosition(order.AccountID, order.Symbol)
			if err != nil {
				return err
			}
			if exists {
				delta := totalFilledAfter - beforeSnapshot.FilledQuantity
				if delta != 0 {
					if order.Side == "BUY" {
						pos.Quantity += delta
					} else {
						pos.Quantity -= delta
					}
					_, err = tx.SavePosition(pos)
					if err != nil {
						return err
					}
				}
			}

			localFillsAfter, err = tx.QueryFills(domain.FillQuery{OrderIDs: []string{orderID}})
			return err
		})
		if err != nil {
			return nil, err
		}
	}

	afterSnapshot := buildFillSyncSnapshot(syncedOrder, localFillsAfter)
	result := "settled"
	if dryRun {
		result = "dry_run_can_settle"
		if !diagnostics.CanSettle {
			result = "dry_run_no_changes"
		}
	} else if changes.AddedRealCount == 0 && changes.DeletedSyntheticCount == 0 {
		result = "already_settled"
	}

	return &ManualFillSyncResponse{
		OrderID:     orderID,
		DryRun:      dryRun,
		SyncedAt:    time.Now().UTC().Format(time.RFC3339),
		Result:      result,
		Before:      beforeSnapshot,
		After:       afterSnapshot,
		Diagnostics: diagnostics,
		Changes:     changes,
	}, nil
}

func matchRemoteTrades(orderID, exchangeOrderID string, remoteTrades []LiveFillReport) []LiveFillReport {
	var matched []LiveFillReport
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

		if (exchangeOrderID != "" && tradeExchangeOrderID == exchangeOrderID) || tradeClientOrderID == orderID {
			matched = append(matched, trade)
		}
	}
	return matched
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

func maskReportsSensitiveData(reports []LiveFillReport) []LiveFillReport {
	if reports == nil {
		return nil
	}
	masked := make([]LiveFillReport, len(reports))
	for i, r := range reports {
		r.Metadata = maskSensitiveData(r.Metadata)
		masked[i] = r
	}
	return masked
}

func maskSensitiveData(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	masked := make(map[string]any)
	for k, v := range data {
		lowerK := strings.ToLower(k)
		// More aggressive masking based on common sensitive keys
		if strings.Contains(lowerK, "key") || strings.Contains(lowerK, "secret") || strings.Contains(lowerK, "signature") ||
			strings.Contains(lowerK, "token") || strings.Contains(lowerK, "auth") || strings.Contains(lowerK, "password") {
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
