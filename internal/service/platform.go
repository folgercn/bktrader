package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

type Platform struct {
	store *memory.Store
}

func NewPlatform(store *memory.Store) *Platform {
	return &Platform{store: store}
}

func (p *Platform) SignalSources() []map[string]any {
	return p.store.SignalSources()
}

func (p *Platform) ListStrategies() []map[string]any {
	return p.store.ListStrategies()
}

func (p *Platform) CreateStrategy(name, description string, parameters map[string]any) map[string]any {
	if parameters == nil {
		parameters = map[string]any{}
	}
	return p.store.CreateStrategy(name, description, parameters)
}

func (p *Platform) ListAccounts() []domain.Account {
	return p.store.ListAccounts()
}

func (p *Platform) CreateAccount(name, mode, exchange string) domain.Account {
	return p.store.CreateAccount(name, strings.ToUpper(mode), exchange)
}

func (p *Platform) ListOrders() []domain.Order {
	return p.store.ListOrders()
}

func (p *Platform) CreateOrder(order domain.Order) domain.Order {
	return p.store.CreateOrder(order)
}

func (p *Platform) ListPositions() []domain.Position {
	return p.store.ListPositions()
}

func (p *Platform) ListBacktests() []domain.BacktestRun {
	return p.store.ListBacktests()
}

func (p *Platform) CreateBacktest(strategyVersionID string, parameters map[string]any) domain.BacktestRun {
	return p.store.CreateBacktest(strategyVersionID, parameters)
}

func (p *Platform) ListPaperSessions() []map[string]any {
	return p.store.ListPaperSessions()
}

func (p *Platform) CreatePaperSession(accountID, strategyID string, startEquity float64) map[string]any {
	return p.store.CreatePaperSession(accountID, strategyID, startEquity)
}

func (p *Platform) ListAnnotations(symbol string) []domain.ChartAnnotation {
	return p.store.ListAnnotations(symbol)
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
