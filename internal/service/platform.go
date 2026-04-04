// Package service 提供 bkTrader 平台的核心业务逻辑。
//
// Platform 是主要的服务门面（Facade），委托给各个子模块:
//   - strategy.go  — 策略管理、账户管理、信号源、回测
//   - order.go     — 订单执行、持仓管理
//   - paper.go     — 模拟交易引擎、统一策略执行计划回放
//   - pnl.go       — PnL 计算、工具函数
//   - chart.go     — K 线数据、图表标注
package service

import (
	"context"
	"sync"

	"github.com/wuyaocheng/bktrader/internal/store"
)

// Platform 是平台服务的核心门面，持有存储接口和运行时状态。
// 所有业务方法通过 Platform 对外暴露，内部拆分到各个子文件中。
type Platform struct {
	store           store.Repository              // 存储层接口（内存 / PostgreSQL）
	mu              sync.Mutex                    // 保护 run map 的并发访问
	run             map[string]context.CancelFunc // 运行中的 paper session -> cancel 函数
	paperPlans      map[string][]paperPlannedOrder
	strategyEngines map[string]StrategyEngine
	liveAdapters    map[string]LiveExecutionAdapter
	signalSources   map[string]SignalSourceProvider
	signalAdapters  map[string]SignalRuntimeAdapter
	manifestMu      sync.Mutex
	once            sync.Once             // 确保 CSV ledger 只加载一次
	ledger          []strategyReplayEvent // 缓存的策略回放账本
	ledgerErr       error                 // 加载账本时的错误
	candleOnce      sync.Once
	candles         []candleBar
	candleErr       error
	tickInterval    int // 模拟盘 Ticker 间隔（秒）
	minuteDataDir   string
	tickDataDir     string
	tickManifest    []tradeArchiveManifestEntry
}

// NewPlatform 创建并初始化平台服务实例。
func NewPlatform(store store.Repository) *Platform {
	platform := &Platform{
		store:           store,
		run:             make(map[string]context.CancelFunc),
		paperPlans:      make(map[string][]paperPlannedOrder),
		strategyEngines: make(map[string]StrategyEngine),
		liveAdapters:    make(map[string]LiveExecutionAdapter),
		signalSources:   make(map[string]SignalSourceProvider),
		signalAdapters:  make(map[string]SignalRuntimeAdapter),
	}
	platform.registerBuiltInStrategyEngines()
	platform.registerBuiltInLiveAdapters()
	platform.registerBuiltInSignalSources()
	platform.registerBuiltInSignalRuntimeAdapters()
	return platform
}

func (p *Platform) SetBacktestDataDirs(minuteDataDir, tickDataDir string) {
	p.minuteDataDir = minuteDataDir
	p.tickDataDir = tickDataDir
	p.manifestMu.Lock()
	p.tickManifest = nil
	p.manifestMu.Unlock()
}
