package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store"
)

type Platform struct {
	store store.Repository
}

type pnlState struct {
	netQty      float64
	avgPrice    float64
	realizedPnL float64
}

func NewPlatform(store store.Repository) *Platform {
	return &Platform{store: store}
}

func (p *Platform) SignalSources() []map[string]any {
	return []map[string]any{
		{
			"id":          "signal-source-bk-1d",
			"name":        "BK 1D ATR Reentry",
			"type":        "internal-strategy",
			"status":      "ACTIVE",
			"dedupeKey":   "symbol+strategyVersion+reason+bar",
			"description": "1D signal / 1m execution strategy feed.",
		},
	}
}

func (p *Platform) ListStrategies() ([]map[string]any, error) {
	return p.store.ListStrategies()
}

func (p *Platform) CreateStrategy(name, description string, parameters map[string]any) (map[string]any, error) {
	if parameters == nil {
		parameters = map[string]any{}
	}
	return p.store.CreateStrategy(name, description, parameters)
}

func (p *Platform) ListAccounts() ([]domain.Account, error) {
	return p.store.ListAccounts()
}

func (p *Platform) ListAccountSummaries() ([]domain.AccountSummary, error) {
	accounts, err := p.store.ListAccounts()
	if err != nil {
		return nil, err
	}
	orders, err := p.store.ListOrders()
	if err != nil {
		return nil, err
	}
	fills, err := p.store.ListFills()
	if err != nil {
		return nil, err
	}
	positions, err := p.store.ListPositions()
	if err != nil {
		return nil, err
	}
	paperSessions, err := p.store.ListPaperSessions()
	if err != nil {
		return nil, err
	}

	orderByID := make(map[string]domain.Order, len(orders))
	for _, order := range orders {
		orderByID[order.ID] = order
	}

	startEquityByAccount := make(map[string]float64, len(paperSessions))
	for _, session := range paperSessions {
		startEquityByAccount[session.AccountID] = session.StartEquity
	}

	states := map[string]*pnlState{}
	feesByAccount := map[string]float64{}
	for _, fill := range fills {
		order, ok := orderByID[fill.OrderID]
		if !ok {
			continue
		}
		key := order.AccountID + "|" + order.Symbol
		state := states[key]
		if state == nil {
			state = &pnlState{}
			states[key] = state
		}
		feesByAccount[order.AccountID] += fill.Fee
		applyPnLFill(state, order.Side, fill.Quantity, fill.Price)
	}

	summaries := make([]domain.AccountSummary, 0, len(accounts))
	for _, account := range accounts {
		startEquity := startEquityByAccount[account.ID]
		if startEquity <= 0 && account.Mode == "PAPER" {
			startEquity = 100000
		}

		realized := 0.0
		for key, state := range states {
			if strings.HasPrefix(key, account.ID+"|") {
				realized += state.realizedPnL
			}
		}

		unrealized := 0.0
		exposure := 0.0
		openCount := 0
		for _, position := range positions {
			if position.AccountID != account.ID {
				continue
			}
			openCount++
			exposure += absFloat(position.Quantity * position.MarkPrice)
			switch strings.ToUpper(position.Side) {
			case "LONG":
				unrealized += (position.MarkPrice - position.EntryPrice) * position.Quantity
			case "SHORT":
				unrealized += (position.EntryPrice - position.MarkPrice) * position.Quantity
			}
		}

		fees := feesByAccount[account.ID]
		netEquity := startEquity + realized + unrealized - fees
		summaries = append(summaries, domain.AccountSummary{
			AccountID:         account.ID,
			AccountName:       account.Name,
			Mode:              account.Mode,
			Exchange:          account.Exchange,
			Status:            account.Status,
			StartEquity:       round2(startEquity),
			RealizedPnL:       round2(realized),
			UnrealizedPnL:     round2(unrealized),
			Fees:              round2(fees),
			NetEquity:         round2(netEquity),
			ExposureNotional:  round2(exposure),
			OpenPositionCount: openCount,
			UpdatedAt:         time.Now().UTC(),
		})
	}
	return summaries, nil
}

func (p *Platform) CreateAccount(name, mode, exchange string) (domain.Account, error) {
	return p.store.CreateAccount(name, strings.ToUpper(mode), exchange)
}

func (p *Platform) ListOrders() ([]domain.Order, error) {
	return p.store.ListOrders()
}

func (p *Platform) CreateOrder(order domain.Order) (domain.Order, error) {
	account, err := p.store.GetAccount(order.AccountID)
	if err != nil {
		return domain.Order{}, err
	}

	createdOrder, err := p.store.CreateOrder(order)
	if err != nil {
		return domain.Order{}, err
	}

	if account.Mode != "PAPER" {
		return createdOrder, nil
	}

	executionPrice := p.resolveExecutionPrice(createdOrder)
	fill := domain.Fill{
		OrderID:  createdOrder.ID,
		Price:    executionPrice,
		Quantity: createdOrder.Quantity,
		Fee:      executionPrice * createdOrder.Quantity * 0.001,
	}
	if _, err := p.store.CreateFill(fill); err != nil {
		return domain.Order{}, err
	}

	if err := p.applyPaperFill(account, createdOrder, executionPrice); err != nil {
		return domain.Order{}, err
	}

	createdOrder.Status = "FILLED"
	createdOrder.Price = executionPrice
	createdOrder.Metadata = cloneMetadata(createdOrder.Metadata)
	createdOrder.Metadata["executionMode"] = "paper"
	createdOrder.Metadata["fillPolicy"] = "immediate"
	return p.store.UpdateOrder(createdOrder)
}

func (p *Platform) ListPositions() ([]domain.Position, error) {
	return p.store.ListPositions()
}

func (p *Platform) ListFills() ([]domain.Fill, error) {
	return p.store.ListFills()
}

func (p *Platform) ListBacktests() ([]domain.BacktestRun, error) {
	return p.store.ListBacktests()
}

func (p *Platform) CreateBacktest(strategyVersionID string, parameters map[string]any) (domain.BacktestRun, error) {
	return p.store.CreateBacktest(strategyVersionID, parameters)
}

func (p *Platform) ListPaperSessions() ([]domain.PaperSession, error) {
	return p.store.ListPaperSessions()
}

func (p *Platform) CreatePaperSession(accountID, strategyID string, startEquity float64) (domain.PaperSession, error) {
	return p.store.CreatePaperSession(accountID, strategyID, startEquity)
}

func (p *Platform) ListAnnotations(symbol string) []domain.ChartAnnotation {
	items := []domain.ChartAnnotation{
		{
			ID:     "anno-1",
			Source: "backtest",
			Type:   "entry_long",
			Symbol: "BTCUSDT",
			Time:   time.Date(2024, 2, 5, 14, 21, 0, 0, time.UTC),
			Price:  43125.0,
			Label:  "SL-Reentry",
		},
		{
			ID:     "anno-2",
			Source: "backtest",
			Type:   "exit_tp",
			Symbol: "BTCUSDT",
			Time:   time.Date(2024, 2, 17, 10, 12, 0, 0, time.UTC),
			Price:  52520.0,
			Label:  "PT",
		},
	}
	if symbol == "" {
		return items
	}
	filtered := make([]domain.ChartAnnotation, 0, len(items))
	for _, item := range items {
		if item.Symbol == symbol {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (p *Platform) CandleSeries(symbol string, resolution string, from int64, to int64, limit int) []map[string]any {
	if limit <= 0 {
		limit = 200
	}
	if resolution == "" {
		resolution = "1"
	}
	step := resolutionToDuration(resolution)
	if step == 0 {
		step = time.Minute
	}
	end := time.Now().UTC()
	if to > 0 {
		end = time.Unix(to, 0).UTC()
	}
	start := end.Add(-time.Duration(limit-1) * step)
	if from > 0 {
		start = time.Unix(from, 0).UTC()
	}
	if start.After(end) {
		start = end.Add(-time.Duration(limit-1) * step)
	}

	candles := make([]map[string]any, 0, limit)
	base := 68000.0
	index := 0
	for current := start; !current.After(end) && len(candles) < limit; current = current.Add(step) {
		wave := float64((index%17)-8) * 18
		drift := float64(index) * 2.5
		open := base + drift + wave
		close := open + float64((index%5)-2)*12
		high := maxFloat(open, close) + 22
		low := minFloat(open, close) - 20
		candles = append(candles, map[string]any{
			"symbol":     symbol,
			"resolution": resolution,
			"time":       current,
			"open":       round2(open),
			"high":       round2(high),
			"low":        round2(low),
			"close":      round2(close),
			"volume":     100 + (index % 19 * 7),
		})
		index++
	}
	return candles
}

func resolutionToDuration(resolution string) time.Duration {
	switch resolution {
	case "1":
		return time.Minute
	case "5":
		return 5 * time.Minute
	case "15":
		return 15 * time.Minute
	case "60":
		return time.Hour
	case "240":
		return 4 * time.Hour
	case "1D", "D":
		return 24 * time.Hour
	default:
		if minutes, err := strconv.Atoi(resolution); err == nil && minutes > 0 {
			return time.Duration(minutes) * time.Minute
		}
		return 0
	}
}

func round2(v float64) float64 {
	return float64(int(v*100)) / 100
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func NormalizeSymbol(symbol string) string {
	if symbol == "" {
		return "BTCUSDT"
	}
	return strings.ToUpper(strings.TrimSpace(symbol))
}

func ValidateRequired(values map[string]string, fields ...string) error {
	for _, field := range fields {
		if strings.TrimSpace(values[field]) == "" {
			return fmt.Errorf("%s is required", field)
		}
	}
	return nil
}

func (p *Platform) resolveExecutionPrice(order domain.Order) float64 {
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
	switch order.Symbol {
	case "ETHUSDT":
		return 3450
	default:
		return 68000
	}
}

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

	if position.Side == targetSide {
		totalQty := position.Quantity + order.Quantity
		position.EntryPrice = ((position.EntryPrice * position.Quantity) + (executionPrice * order.Quantity)) / totalQty
		position.Quantity = totalQty
		position.MarkPrice = executionPrice
		position.StrategyVersionID = firstNonEmpty(order.StrategyVersionID, position.StrategyVersionID)
		_, err := p.store.SavePosition(position)
		return err
	}

	if order.Quantity < position.Quantity {
		position.Quantity = position.Quantity - order.Quantity
		position.MarkPrice = executionPrice
		_, err := p.store.SavePosition(position)
		return err
	}

	if order.Quantity == position.Quantity {
		return p.store.DeletePosition(position.ID)
	}

	remaining := order.Quantity - position.Quantity
	position.Side = targetSide
	position.Quantity = remaining
	position.EntryPrice = executionPrice
	position.MarkPrice = executionPrice
	position.StrategyVersionID = firstNonEmpty(order.StrategyVersionID, position.StrategyVersionID)
	_, err = p.store.SavePosition(position)
	return err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(metadata))
	for key, value := range metadata {
		out[key] = value
	}
	return out
}

func toFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func applyPnLFill(state *pnlState, side string, qty, price float64) {
	signedQty := qty
	if strings.ToUpper(strings.TrimSpace(side)) == "SELL" {
		signedQty = -qty
	}

	if state.netQty == 0 || sameSign(state.netQty, signedQty) {
		totalQty := absFloat(state.netQty) + absFloat(signedQty)
		if totalQty > 0 {
			state.avgPrice = ((state.avgPrice * absFloat(state.netQty)) + (price * absFloat(signedQty))) / totalQty
		}
		state.netQty += signedQty
		return
	}

	closingQty := minFloat(absFloat(state.netQty), absFloat(signedQty))
	if state.netQty > 0 {
		state.realizedPnL += (price - state.avgPrice) * closingQty
	} else {
		state.realizedPnL += (state.avgPrice - price) * closingQty
	}

	remaining := absFloat(signedQty) - closingQty
	if remaining == 0 {
		if absFloat(state.netQty) == closingQty {
			state.netQty = 0
			state.avgPrice = 0
		} else {
			if state.netQty > 0 {
				state.netQty -= closingQty
			} else {
				state.netQty += closingQty
			}
		}
		return
	}

	if signedQty > 0 {
		state.netQty = remaining
	} else {
		state.netQty = -remaining
	}
	state.avgPrice = price
}

func sameSign(a, b float64) bool {
	return (a > 0 && b > 0) || (a < 0 && b < 0)
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
