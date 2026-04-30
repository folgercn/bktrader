package domain

import "strings"

// OrderIntent 交易意图语义分类。
// 这是判断"一笔订单到底是开多还是平空"的唯一事实源。
// 所有需要判断订单方向语义的地方（service/、http/、前端），
// 都必须消费此分类结果，禁止自行组合 side + reduceOnly 推断。
type OrderIntent string

const (
	OrderIntentOpenLong   OrderIntent = "OPEN_LONG"   // BUY + !reduceOnly → 开多
	OrderIntentOpenShort  OrderIntent = "OPEN_SHORT"  // SELL + !reduceOnly → 开空
	OrderIntentCloseLong  OrderIntent = "CLOSE_LONG"  // SELL + reduceOnly → 平多
	OrderIntentCloseShort OrderIntent = "CLOSE_SHORT" // BUY + reduceOnly → 平空
	OrderIntentUnknown    OrderIntent = "UNKNOWN"
)

// ClassifyOrderIntent 是判断订单交易语义的唯一入口。
// 所有需要判断"这笔订单是开多还是平空"的地方，都必须调用此函数。
// 禁止在 service/、http/、前端 中自行组合 side + reduceOnly 推断。
//
// 当前基于 One-way Mode（positionSide = BOTH）。
// 如果未来需要支持 Hedge Mode（positionSide = LONG/SHORT），
// 在此函数中扩展，不在调用方各自处理。
func ClassifyOrderIntent(o Order) OrderIntent {
	side := strings.ToUpper(strings.TrimSpace(o.Side))
	isExit := o.EffectiveReduceOnly() || o.EffectiveClosePosition()

	switch {
	case side == "BUY" && !isExit:
		return OrderIntentOpenLong
	case side == "SELL" && !isExit:
		return OrderIntentOpenShort
	case side == "SELL" && isExit:
		return OrderIntentCloseLong
	case side == "BUY" && isExit:
		return OrderIntentCloseShort
	default:
		return OrderIntentUnknown
	}
}

// IntentLabel 返回用于 UI 展示的中文标签。
func (i OrderIntent) IntentLabel() string {
	switch i {
	case OrderIntentOpenLong:
		return "开多"
	case OrderIntentOpenShort:
		return "开空"
	case OrderIntentCloseLong:
		return "平多"
	case OrderIntentCloseShort:
		return "平空"
	default:
		return "未知"
	}
}

// IsEntry 返回该意图是否为开仓。
func (i OrderIntent) IsEntry() bool {
	return i == OrderIntentOpenLong || i == OrderIntentOpenShort
}

// IsExit 返回该意图是否为平仓。
func (i OrderIntent) IsExit() bool {
	return i == OrderIntentCloseLong || i == OrderIntentCloseShort
}
