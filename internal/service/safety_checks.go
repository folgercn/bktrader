package service

import (
	"errors"
	"strings"
)

var ErrActivePositionsOrOrders = errors.New("active positions or orders exist")

type activePositionsOrOrdersError struct{}

func (activePositionsOrOrdersError) Error() string {
	return "存在活动中的订单或未平仓头寸，无法直接停用/删除"
}

func (activePositionsOrOrdersError) Unwrap() error {
	return ErrActivePositionsOrOrders
}

func (p *Platform) HasActivePositionsOrOrders(accountID, strategyID string) (bool, error) {
	positions, err := p.store.ListPositions()
	if err != nil {
		return false, err
	}
	for _, item := range positions {
		if strings.TrimSpace(item.AccountID) != strings.TrimSpace(accountID) || item.Quantity <= 0 {
			continue
		}
		matches, matchErr := p.matchesStrategyVersionToStrategy(item.StrategyVersionID, strategyID)
		if matchErr != nil {
			return false, matchErr
		}
		if matches {
			return true, nil
		}
	}

	orders, err := p.store.ListOrders()
	if err != nil {
		return false, err
	}
	for _, item := range orders {
		if strings.TrimSpace(item.AccountID) != strings.TrimSpace(accountID) {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(item.Status)) {
		case "NEW", "PARTIALLY_FILLED", "ACCEPTED":
		default:
			continue
		}
		matches, matchErr := p.matchesStrategyVersionToStrategy(item.StrategyVersionID, strategyID)
		if matchErr != nil {
			return false, matchErr
		}
		if matches {
			return true, nil
		}
	}
	return false, nil
}

func (p *Platform) ensureNoActivePositionsOrOrders(accountID, strategyID string) error {
	active, err := p.HasActivePositionsOrOrders(accountID, strategyID)
	if err != nil {
		return err
	}
	if active {
		return activePositionsOrOrdersError{}
	}
	return nil
}

func (p *Platform) matchesStrategyVersionToStrategy(strategyVersionID, strategyID string) (bool, error) {
	if strings.TrimSpace(strategyVersionID) == "" || strings.TrimSpace(strategyID) == "" {
		return false, nil
	}
	resolvedStrategyID, err := p.resolveStrategyIDFromVersionID(strategyVersionID)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(resolvedStrategyID), strings.TrimSpace(strategyID)), nil
}
