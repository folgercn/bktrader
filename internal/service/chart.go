package service

import (
	"strconv"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

// --- K 线数据和图表标注服务方法 ---

// ListAnnotations 返回图表标注数据。
// 当前为硬编码示例数据（TODO: 从回测/订单数据动态生成）。
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

// CandleSeries 生成 K 线数据序列，用于 TradingView 图表展示。
// 当前为模拟数据（TODO: 对接真实行情数据源，如 Binance API）。
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

	// 计算时间范围
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

	// 生成伪造 K 线数据（基于公式的确定性波动）
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

// resolutionToDuration 将 K 线分辨率字符串转换为 time.Duration。
// 支持: "1"(1分钟), "5", "15", "60", "240", "1D"/"D"(日线)。
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
