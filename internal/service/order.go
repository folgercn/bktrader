package service

import (
	"fmt"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

// --- 订单管理服务方法 ---

// ListOrders 获取所有订单列表。
func (p *Platform) ListOrders() ([]domain.Order, error) {
	return p.store.ListOrders()
}

func (p *Platform) GetOrder(orderID string) (domain.Order, error) {
	items, err := p.store.ListOrders()
	if err != nil {
		return domain.Order{}, err
	}
	for _, item := range items {
		if item.ID == orderID {
			return item, nil
		}
	}
	return domain.Order{}, fmt.Errorf("order not found: %s", orderID)
}

// CreateOrder 创建订单。对于 PAPER 模式账户，订单会被立即执行（模拟成交），
// 生成 fill 记录、更新持仓、捕获净值快照。
func (p *Platform) CreateOrder(order domain.Order) (domain.Order, error) {
	account, err := p.store.GetAccount(order.AccountID)
	if err != nil {
		return domain.Order{}, err
	}

	if account.Mode == "LIVE" {
		if account.Status != "CONFIGURED" && account.Status != "READY" {
			return domain.Order{}, fmt.Errorf("live account %s is not configured", account.ID)
		}
		if _, _, err := p.resolveLiveAdapterForAccount(account); err != nil {
			return domain.Order{}, err
		}
	}

	createdOrder, err := p.store.CreateOrder(order)
	if err != nil {
		return domain.Order{}, err
	}

	if account.Mode == "LIVE" {
		return p.submitLiveOrder(account, createdOrder)
	}

	// 非模拟盘账户直接返回，等待真实交易所回报
	if account.Mode != "PAPER" {
		return createdOrder, nil
	}

	// --- 模拟盘立即成交逻辑 ---
	executionPrice := resolveExecutionPrice(createdOrder)
	fillFee := resolvePaperFillFee(createdOrder, executionPrice)
	fill := domain.Fill{
		OrderID:  createdOrder.ID,
		Price:    executionPrice,
		Quantity: createdOrder.Quantity,
		Fee:      fillFee,
	}
	if _, err := p.store.CreateFill(fill); err != nil {
		return domain.Order{}, err
	}

	// 更新持仓
	if err := p.applyPaperFill(account, createdOrder, executionPrice); err != nil {
		return domain.Order{}, err
	}

	// 标记订单为已成交
	createdOrder.Status = "FILLED"
	createdOrder.Price = executionPrice
	createdOrder.Metadata = cloneMetadata(createdOrder.Metadata)
	createdOrder.Metadata["executionMode"] = "paper"
	createdOrder.Metadata["fillPolicy"] = "immediate"
	createdOrder.Metadata["feeSource"] = "configured-paper-rate"
	updatedOrder, err := p.store.UpdateOrder(createdOrder)
	if err != nil {
		return domain.Order{}, err
	}

	// 捕获成交后的净值快照
	if err := p.captureAccountSnapshot(account.ID); err != nil {
		return domain.Order{}, err
	}
	return updatedOrder, nil
}

func (p *Platform) submitLiveOrder(account domain.Account, order domain.Order) (domain.Order, error) {
	adapter, binding, err := p.resolveLiveAdapterForAccount(account)
	if err != nil {
		return domain.Order{}, err
	}

	submission, err := adapter.SubmitOrder(account, order, binding)
	order.Metadata = cloneMetadata(order.Metadata)
	order.Metadata["executionMode"] = "live"
	order.Metadata["adapterKey"] = normalizeLiveAdapterKey(stringValue(binding["adapterKey"]))
	order.Metadata["feeSource"] = "exchange"
	order.Metadata["fundingSource"] = "exchange"
	order.Metadata["submitMode"] = "adapter"
	if err != nil {
		order.Status = "REJECTED"
		order.Metadata["liveSubmitError"] = err.Error()
		updatedOrder, updateErr := p.store.UpdateOrder(order)
		if updateErr != nil {
			return domain.Order{}, updateErr
		}
		return updatedOrder, err
	}

	order.Status = firstNonEmpty(submission.Status, "ACCEPTED")
	order.Metadata["exchangeOrderId"] = submission.ExchangeOrderID
	order.Metadata["acceptedAt"] = submission.AcceptedAt
	order.Metadata["adapterSubmission"] = submission.Metadata
	return p.store.UpdateOrder(order)
}

func (p *Platform) SyncLiveOrder(orderID string) (domain.Order, error) {
	order, err := p.GetOrder(orderID)
	if err != nil {
		return domain.Order{}, err
	}
	account, err := p.store.GetAccount(order.AccountID)
	if err != nil {
		return domain.Order{}, err
	}
	if account.Mode != "LIVE" {
		return domain.Order{}, fmt.Errorf("order %s is not a live order", orderID)
	}
	adapter, binding, err := p.resolveLiveAdapterForAccount(account)
	if err != nil {
		return domain.Order{}, err
	}
	syncResult, err := adapter.SyncOrder(account, order, binding)
	if err != nil {
		return domain.Order{}, err
	}

	order.Metadata = cloneMetadata(order.Metadata)
	order.Metadata["lastSyncAt"] = syncResult.SyncedAt
	order.Metadata["syncMode"] = "adapter"
	order.Metadata["feeSource"] = firstNonEmpty(syncResult.FeeSource, "exchange")
	order.Metadata["fundingSource"] = firstNonEmpty(syncResult.FundingSrc, "exchange")
	order.Metadata["adapterSync"] = syncResult.Metadata
	order.Status = firstNonEmpty(syncResult.Status, order.Status)
	if len(syncResult.Fills) > 0 {
		for _, report := range syncResult.Fills {
			if _, err := p.store.CreateFill(domain.Fill{
				OrderID:  order.ID,
				Price:    report.Price,
				Quantity: report.Quantity,
				Fee:      report.Fee - report.FundingPnL,
			}); err != nil {
				return domain.Order{}, err
			}
			liveExecutionPrice := report.Price
			if liveExecutionPrice <= 0 {
				liveExecutionPrice = resolveExecutionPrice(order)
			}
			execOrder := order
			execOrder.Price = liveExecutionPrice
			if execOrder.Metadata == nil {
				execOrder.Metadata = map[string]any{}
			}
			execOrder.Metadata["fundingPnL"] = report.FundingPnL
			if err := p.applyPaperFill(account, execOrder, liveExecutionPrice); err != nil {
				return domain.Order{}, err
			}
		}
		order.Price = syncResult.Fills[len(syncResult.Fills)-1].Price
	}
	updatedOrder, err := p.store.UpdateOrder(order)
	if err != nil {
		return domain.Order{}, err
	}
	if err := p.captureAccountSnapshot(account.ID); err != nil {
		return domain.Order{}, err
	}
	return updatedOrder, nil
}

func (p *Platform) resolveLiveAdapterForAccount(account domain.Account) (LiveExecutionAdapter, map[string]any, error) {
	binding := resolveLiveBinding(account)
	if len(binding) == 0 {
		return nil, nil, fmt.Errorf("live account %s has no adapter binding", account.ID)
	}
	adapterKey := normalizeLiveAdapterKey(stringValue(binding["adapterKey"]))
	adapter, ok := p.liveAdapters[adapterKey]
	if !ok {
		return nil, nil, fmt.Errorf("live adapter not registered: %s", adapterKey)
	}
	if err := adapter.ValidateAccountConfig(binding); err != nil {
		return nil, nil, err
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
	return p.store.ListPositions()
}

// ListFills 获取所有成交记录列表。
func (p *Platform) ListFills() ([]domain.Fill, error) {
	return p.store.ListFills()
}

// applyPaperFill 根据模拟成交更新仓位。
// 包含开仓、增仓、部分平仓、全部平仓、反向开仓等场景。
func (p *Platform) applyPaperFill(account domain.Account, order domain.Order, executionPrice float64) error {
	position, exists, err := p.store.FindPosition(account.ID, order.Symbol)
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
		_, err := p.store.SavePosition(domain.Position{
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
		_, err := p.store.SavePosition(position)
		return err
	}

	// 反方向 → 部分平仓
	if order.Quantity < position.Quantity {
		position.Quantity = position.Quantity - order.Quantity
		position.MarkPrice = executionPrice
		_, err := p.store.SavePosition(position)
		return err
	}

	// 反方向 → 全部平仓
	if order.Quantity == position.Quantity {
		return p.store.DeletePosition(position.ID)
	}

	// 反方向 → 平仓后反向开仓
	remaining := order.Quantity - position.Quantity
	position.Side = targetSide
	position.Quantity = remaining
	position.EntryPrice = executionPrice
	position.MarkPrice = executionPrice
	position.StrategyVersionID = firstNonEmpty(order.StrategyVersionID, position.StrategyVersionID)
	_, err = p.store.SavePosition(position)
	return err
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
