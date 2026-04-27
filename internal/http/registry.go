package http

// RouteEntry 代表一个 API 路由及其元数据
type RouteEntry struct {
	Path        string
	Methods     []string
	Module      string
	Description string
	CLICommand  string
	Idempotent  bool
	RiskLevel   string
}

// APIRegistry 是 bktrader 全量 API 的声明式清单
var APIRegistry = []RouteEntry{
	{Path: "/healthz", Methods: []string{"GET"}, Module: "system", CLICommand: "status", Idempotent: true, RiskLevel: "L0"},
	{Path: "/api/v1/overview", Methods: []string{"GET"}, Module: "system", CLICommand: "status", Idempotent: true, RiskLevel: "L0"},

	// Auth
	{Path: "/api/v1/auth/login", Methods: []string{"POST"}, Module: "auth", CLICommand: "auth login", Idempotent: false, RiskLevel: "L1"},
	{Path: "/api/v1/auth/me", Methods: []string{"GET"}, Module: "auth", CLICommand: "auth me", Idempotent: true, RiskLevel: "L0"},

	// Accounts
	{Path: "/api/v1/accounts", Methods: []string{"GET"}, Module: "accounts", CLICommand: "account list", Idempotent: true, RiskLevel: "L0"},
	{Path: "/api/v1/account-summaries", Methods: []string{"GET"}, Module: "accounts", CLICommand: "account summary", Idempotent: true, RiskLevel: "L0"},
	{Path: "/api/v1/account-equity-snapshots", Methods: []string{"GET"}, Module: "accounts", CLICommand: "account equity", Idempotent: true, RiskLevel: "L0"},
	{Path: "/api/v1/live/accounts/{id}/sync", Methods: []string{"POST"}, Module: "accounts", CLICommand: "account sync", Idempotent: false, RiskLevel: "L1"},
	{Path: "/api/v1/live/accounts/{id}/reconcile", Methods: []string{"POST"}, Module: "accounts", CLICommand: "account reconcile", Idempotent: false, RiskLevel: "L2"},
	{Path: "/api/v1/live/accounts/{id}/binding", Methods: []string{"POST"}, Module: "accounts", CLICommand: "account bind", Idempotent: false, RiskLevel: "L2"},

	// Orders & Fills
	{Path: "/api/v1/orders", Methods: []string{"GET"}, Module: "orders", CLICommand: "order list", Idempotent: true, RiskLevel: "L0"},
	{Path: "/api/v1/orders/{id}", Methods: []string{"GET"}, Module: "orders", CLICommand: "order get", Idempotent: true, RiskLevel: "L0"},
	{Path: "/api/v1/orders/{id}/cancel", Methods: []string{"POST"}, Module: "orders", CLICommand: "order cancel", Idempotent: false, RiskLevel: "L1"},
	{Path: "/api/v1/fills", Methods: []string{"GET"}, Module: "orders", CLICommand: "fill list", Idempotent: true, RiskLevel: "L0"},

	// Live Sessions
	{Path: "/api/v1/live/sessions", Methods: []string{"GET"}, Module: "live", CLICommand: "live list", Idempotent: true, RiskLevel: "L0"},
	{Path: "/api/v1/live/sessions/{id}/detail", Methods: []string{"GET"}, Module: "live", CLICommand: "live get", Idempotent: true, RiskLevel: "L0"},
	{Path: "/api/v1/live/sessions/{id}/start", Methods: []string{"POST"}, Module: "live", CLICommand: "live start", Idempotent: false, RiskLevel: "L1"},
	{Path: "/api/v1/live/sessions/{id}/stop", Methods: []string{"POST"}, Module: "live", CLICommand: "live stop", Idempotent: false, RiskLevel: "L1"},
	{Path: "/api/v1/live/sessions/{id}/dispatch", Methods: []string{"POST"}, Module: "live", CLICommand: "live dispatch", Idempotent: false, RiskLevel: "L2"},

	// Logs & Charts
	{Path: "/api/v1/logs/system", Methods: []string{"GET"}, Module: "logs", CLICommand: "logs system", Idempotent: true, RiskLevel: "L0"},
	{Path: "/api/v1/logs/events", Methods: []string{"GET"}, Module: "logs", CLICommand: "logs event", Idempotent: true, RiskLevel: "L0"},
	{Path: "/api/v1/logs/stream", Methods: []string{"GET"}, Module: "logs", CLICommand: "logs stream", Idempotent: true, RiskLevel: "L0"},
	{Path: "/api/v1/chart/candles", Methods: []string{"GET"}, Module: "chart", CLICommand: "chart candles", Idempotent: true, RiskLevel: "L0"},
}
