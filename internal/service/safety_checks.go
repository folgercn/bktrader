package service

import (
	"fmt"
	"strings"
)

// HasActivePositionsOrOrders 检查指定账户和策略下是否存在活跃的持仓或订单。
func (p *Platform) HasActivePositionsOrOrders(accountID, strategyID string) (bool, string) {
	// 1. 检查持仓
	positions, err := p.store.ListPositions()
	if err == nil {
		for _, pos := range positions {
			if pos.AccountID == accountID && pos.Quantity > 0 {
				// 获取策略ID，如果是具体的策略版本，需要向上溯源
				posStrategyID, _ := p.resolveStrategyIDFromVersionID(pos.StrategyVersionID)
				if posStrategyID == strategyID || strategyID == "" {
					return true, fmt.Sprintf("存在活跃持仓: %s %v %s", pos.Symbol, pos.Quantity, pos.Side)
				}
			}
		}
	}

	// 2. 检查待处理订单
	orders, err := p.store.ListOrders()
	if err == nil {
		for _, ord := range orders {
			if ord.AccountID == accountID {
				status := strings.ToUpper(ord.Status)
				// 活跃状态：NEW, ACCEPTED, PARTIALLY_FILLED
				if status == "NEW" || status == "ACCEPTED" || status == "PARTIALLY_FILLED" {
					ordStrategyID, _ := p.resolveStrategyIDFromVersionID(ord.StrategyVersionID)
					if ordStrategyID == strategyID || strategyID == "" {
						return true, fmt.Sprintf("存在待处理订单: %s %s %s %v %s", ord.ID, ord.Symbol, ord.Side, ord.Quantity, ord.Status)
					}
				}
			}
		}
	}

	return false, ""
}
