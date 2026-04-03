package store

import "github.com/wuyaocheng/bktrader/internal/domain"

type Repository interface {
	ListStrategies() ([]map[string]any, error)
	CreateStrategy(name, description string, parameters map[string]any) (map[string]any, error)

	ListAccounts() ([]domain.Account, error)
	GetAccount(accountID string) (domain.Account, error)
	CreateAccount(name, mode, exchange string) (domain.Account, error)

	ListOrders() ([]domain.Order, error)
	CreateOrder(order domain.Order) (domain.Order, error)
	UpdateOrder(order domain.Order) (domain.Order, error)

	ListFills() ([]domain.Fill, error)
	CreateFill(fill domain.Fill) (domain.Fill, error)

	ListPositions() ([]domain.Position, error)
	FindPosition(accountID, symbol string) (domain.Position, bool, error)
	SavePosition(position domain.Position) (domain.Position, error)
	DeletePosition(positionID string) error

	ListBacktests() ([]domain.BacktestRun, error)
	CreateBacktest(strategyVersionID string, parameters map[string]any) (domain.BacktestRun, error)

	ListPaperSessions() ([]domain.PaperSession, error)
	GetPaperSession(sessionID string) (domain.PaperSession, error)
	CreatePaperSession(accountID, strategyID string, startEquity float64) (domain.PaperSession, error)
	UpdatePaperSessionStatus(sessionID, status string) (domain.PaperSession, error)
	UpdatePaperSessionState(sessionID string, state map[string]any) (domain.PaperSession, error)

	ListAccountEquitySnapshots(accountID string) ([]domain.AccountEquitySnapshot, error)
	CreateAccountEquitySnapshot(snapshot domain.AccountEquitySnapshot) (domain.AccountEquitySnapshot, error)
}
