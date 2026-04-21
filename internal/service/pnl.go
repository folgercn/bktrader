package service

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

// pnlState 跟踪单个交易对的净持仓方向、均价和已实现盈亏。
type pnlState struct {
	netQty      float64 // 净持仓数量（正为多头，负为空头）
	avgPrice    float64 // 加权平均入场价
	realizedPnL float64 // 累计已实现盈亏
}

// buildAccountSummary 基于持仓、PnL 状态和费用数据构建账户汇总。
func buildAccountSummary(
	account domain.Account,
	positions []domain.Position,
	startEquityByAccount map[string]float64,
	states map[string]*pnlState,
	feesByAccount map[string]float64,
) domain.AccountSummary {
	startEquity := startEquityByAccount[account.ID]
	if startEquity <= 0 && account.Mode == "PAPER" {
		startEquity = 100000 // 模拟盘默认初始权益 10 万
	}

	// 汇总已实现盈亏
	realized := 0.0
	for key, state := range states {
		if strings.HasPrefix(key, account.ID+"|") {
			realized += state.realizedPnL
		}
	}

	// 计算未实现盈亏和风险敞口
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
	return domain.AccountSummary{
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
	}
}

// applyPnLFill 根据一笔成交更新 PnL 状态。
// 处理同方向加仓（加权均价）和反方向平仓/反转的已实现盈亏计算。
func applyPnLFill(state *pnlState, side string, qty, price float64) {
	signedQty := qty
	if strings.ToUpper(strings.TrimSpace(side)) == "SELL" {
		signedQty = -qty
	}

	// 同方向加仓：更新加权平均价
	if state.netQty == 0 || sameSign(state.netQty, signedQty) {
		totalQty := absFloat(state.netQty) + absFloat(signedQty)
		if totalQty > 0 {
			state.avgPrice = ((state.avgPrice * absFloat(state.netQty)) + (price * absFloat(signedQty))) / totalQty
		}
		state.netQty += signedQty
		return
	}

	// 反方向：计算平仓盈亏
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

	// 平仓后反向开仓
	if signedQty > 0 {
		state.netQty = remaining
	} else {
		state.netQty = -remaining
	}
	state.avgPrice = price
}

// --- 通用数学/工具函数 ---

// sameSign 判断两个浮点数是否同号。
func sameSign(a, b float64) bool {
	return (a > 0 && b > 0) || (a < 0 && b < 0)
}

// absFloat 返回浮点数的绝对值。
func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// round2 四舍五入到小数点后两位。
func round2(v float64) float64 {
	return float64(int(v*100)) / 100
}

// maxFloat 返回两个浮点数中较大的一个。
func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// minFloat 返回两个浮点数中较小的一个。
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// firstNonEmpty 返回第一个非空字符串。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// cloneMetadata 深拷贝 metadata map，避免引用共享导致数据污染。
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

// ToFloat64 将 any 类型值转换为 float64，支持常见数值类型和字符串（导出版本）。
func ToFloat64(value any) (float64, bool) {
	return toFloat64(value)
}

// toFloat64 将 any 类型值转换为 float64，支持常见数值类型和字符串。
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
