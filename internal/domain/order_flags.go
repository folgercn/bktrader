package domain

import "strings"

func (o Order) EffectiveReduceOnly() bool {
	return o.ReduceOnly || orderFlagValue(o.Metadata["reduceOnly"])
}

func (o Order) EffectiveClosePosition() bool {
	return o.ClosePosition || orderFlagValue(o.Metadata["closePosition"])
}

func (o *Order) NormalizeExecutionFlags() {
	if o == nil {
		return
	}
	if o.Metadata == nil {
		o.Metadata = map[string]any{}
	}
	if orderFlagValue(o.Metadata["reduceOnly"]) {
		o.ReduceOnly = true
	}
	if orderFlagValue(o.Metadata["closePosition"]) {
		o.ClosePosition = true
	}
	if o.ReduceOnly {
		o.Metadata["reduceOnly"] = true
	}
	if o.ClosePosition {
		o.Metadata["closePosition"] = true
	}
}

func orderFlagValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}
