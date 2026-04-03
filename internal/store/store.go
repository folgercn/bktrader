package store

import "github.com/wuyaocheng/bktrader/internal/domain"

type Repository interface {
	ListStrategies() ([]map[string]any, error)
	CreateStrategy(name, description string, parameters map[string]any) (map[string]any, error)

	ListAccounts() ([]domain.Account, error)
	CreateAccount(name, mode, exchange string) (domain.Account, error)

	ListOrders() ([]domain.Order, error)
	CreateOrder(order domain.Order) (domain.Order, error)

	ListPositions() ([]domain.Position, error)

	ListBacktests() ([]domain.BacktestRun, error)
	CreateBacktest(strategyVersionID string, parameters map[string]any) (domain.BacktestRun, error)

	ListPaperSessions() ([]map[string]any, error)
	CreatePaperSession(accountID, strategyID string, startEquity float64) (map[string]any, error)
}
