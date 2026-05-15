# ETH Pretouch Timing Live Integration — 设计文档

日期：2026-05-15  
分支：`feature/eth-pretouch-timing-live`  
范围：testnet shadow（`dispatchMode=manual-review`，不自动下单）  
风险等级：L2（涉及 `internal/service/` 新增策略引擎，不修改现有引擎）

## 1. 目标

将 research 验证的 ETH pretouch timing 策略集成到 bkTrader live 系统，作为 testnet shadow 运行：
- 实时检测 ETHUSDT pretouch 事件
- 应用 timing classification（skip/fast/slow）决定是否入场
- 应用 RF probability sizing 决定仓位大小
- 应用 cost_q50_cut050 对高成本事件降仓
- 记录信号和模拟 PnL，不自动执行

## 2. Research 结论摘要

| 指标 | 数值 |
|------|------|
| 事件源 | `pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1` |
| Symbol | ETHUSDT only |
| Full window CS (same_close) | 30.53% |
| 10bps kill stress CS | 23.29% |
| Neg SM (all scenarios) | 0 |
| Worst SM (10bps) | +1.40% |
| Forward CS | 10.97% |
| Timing classifier | DT3, LOOCV=14.59% |
| RF AUC | 0.70 |
| Exit params | SL=0.45 ATR, BE=0.8R, trail_start=1.5R, max_hold=2h |
| Base share | 0.80 (ETH-only) |
| Cost sizing | roundtrip_cost_atr >= train_q50 → multiplier × 0.5 |

## 3. 架构设计

### 3.1 新增组件

```
internal/service/
├── strategy_engine_pretouch_timing.go    # 新策略引擎（核心）
├── pretouch_event_detector.go            # pretouch 事件检测逻辑
├── pretouch_ml_client.go                 # ML sidecar HTTP client
└── pretouch_sizing.go                    # cost sizing + multiplier 应用

internal/domain/
└── pretouch_event.go                     # PretouchEvent 领域模型

scripts/timing_probability_sidecar/       # Python ML sidecar
├── server.py                             # FastAPI 服务
├── inference.py                          # 推理（复用 research 代码）
├── retrain.py                            # rolling 重训
└── models/                               # 持久化模型权重
```

### 3.2 不修改的文件

- `live_execution.go` — 不改执行逻辑
- `execution_strategy.go` — 复用现有 `book-aware-v1`
- `signal_runtime_sessions.go` — 复用现有 runtime 管理
- `signal_runtime_scanner.go` — 复用现有 scanner

### 3.3 数据流

```
Binance WS (1s trade tick + 1h kline)
    ↓
Signal Runtime Session (existing infra)
    ↓
Strategy Engine: bk-live-eth-pretouch-timing
    ↓
┌─────────────────────────────────────────┐
│ 1. Pretouch Event Detector              │
│    - 监控 1h signal bar 内的 1s tick     │
│    - 检测 touch_extension_atr 条件       │
│    - 检测 speed_300s_atr >= threshold    │
│    - 检测 pre_touch_seconds <= 1800      │
│    - 检测 eff_300s <= 1.0               │
│                                         │
│ 2. Timing Classifier (DT3 rules)        │
│    - 从 research 提取的决策规则          │
│    - 输出: skip / fast / slow            │
│    - skip → 不入场                       │
│                                         │
│ 3. Sizing Calculator                    │
│    - RF probability → multiplier        │
│    - cost_q50_cut050 → × 0.5 if high    │
│    - final_size = 0.80 × multiplier     │
│                                         │
│ 4. Signal Intent                        │
│    - action: advance-plan               │
│    - side: long/short                   │
│    - quantity: final_size               │
│    - metadata: timing_regime, prob, etc │
└─────────────────────────────────────────┘
    ↓
Execution Strategy (book-aware-v1, existing)
    ↓
Dispatch (manual-review, 不自动下单)
```

## 4. 核心接口设计

### 4.1 PretouchEvent 领域模型

```go
// internal/domain/pretouch_event.go
package domain

import "time"

type PretouchEvent struct {
    EventID           string    `json:"eventId"`
    Symbol            string    `json:"symbol"`
    Side              string    `json:"side"`       // long / short
    TouchTime         time.Time `json:"touchTime"`
    TouchPrice        float64   `json:"touchPrice"`
    Level             float64   `json:"level"`      // breakout level (prev_high_2 / prev_low_2)
    ATR               float64   `json:"atr"`
    TouchExtensionATR float64   `json:"touchExtensionAtr"`
    Speed300sATR      float64   `json:"speed300sAtr"`
    Eff300s           float64   `json:"eff300s"`
    PreTouchSeconds   float64   `json:"preTouchSeconds"`
    RoundtripCostATR  float64   `json:"roundtripCostAtr"`
    SignalBarStart    time.Time `json:"signalBarStart"`
    
    // Timing classification result
    TimingRegime      string    `json:"timingRegime"`  // skip / fast / slow
    
    // Sizing
    RFProbability     float64   `json:"rfProbability"`
    SizingMultiplier  float64   `json:"sizingMultiplier"`
    CostPenalty       float64   `json:"costPenalty"`   // 1.0 or 0.5
    FinalPositionSize float64   `json:"finalPositionSize"`
}
```

### 4.2 策略引擎接口

```go
// internal/service/strategy_engine_pretouch_timing.go
package service

const bkLiveEthPretouchTimingEngineKey = "bk-live-eth-pretouch-timing"

type bkLiveEthPretouchTimingEngine struct {
    platform *Platform
    detector *PretouchEventDetector
}

func (e bkLiveEthPretouchTimingEngine) Key() string {
    return bkLiveEthPretouchTimingEngineKey
}

func (e bkLiveEthPretouchTimingEngine) EvaluateSignal(
    ctx StrategySignalEvaluationContext,
) (StrategySignalDecision, error) {
    // 1. 检查是否为 ETHUSDT
    // 2. 检测 pretouch 事件
    // 3. 应用 timing classification
    // 4. 计算 sizing
    // 5. 产出 signal decision
}
```

### 4.3 Pretouch 事件检测器

```go
// internal/service/pretouch_event_detector.go
package service

type PretouchEventDetector struct {
    // 滚动窗口状态
    recentTicks    []TickData       // 最近 300s 的 tick 数据
    signalBarState *SignalBarState   // 当前 1h signal bar 状态
    
    // 冻结阈值（从 research 固化）
    speedThreshold     float64  // train q10 = 0.228106
    costQ50Threshold   float64  // train q50 = 0.116865
    maxPreTouchSeconds float64  // 1800
    maxEff300s         float64  // 1.0
}

type PretouchDetectionResult struct {
    Detected bool
    Event    domain.PretouchEvent
    Reason   string  // 未检测到的原因
}

func (d *PretouchEventDetector) OnTick(tick TickData) *PretouchDetectionResult {
    // 1. 更新滚动窗口
    // 2. 检查当前 tick 是否触发 breakout level
    // 3. 如果触发，计算 pretouch 特征
    // 4. 验证质量过滤条件
    // 5. 返回检测结果
}
```

### 4.4 Sidecar HTTP Client

```go
// internal/service/pretouch_ml_client.go
package service

import (
    "context"
    "time"
)

const (
    pretouchSidecarURL     = "http://localhost:9101"
    pretouchInferTimeout   = 100 * time.Millisecond
)

type PretouchMLInference struct {
    TimingRegime     string  // "skip" / "fast" / "slow"
    RFProbability    float64
    SizingMultiplier float64
    ModelVersion     string
    ModelAgeHours    float64
    InferenceTimeMs  int
}

type PretouchMLClient struct {
    baseURL string
    client  *http.Client
}

func (c *PretouchMLClient) Infer(
    ctx context.Context,
    event domain.PretouchEvent,
) (*PretouchMLInference, error) {
    // POST /infer with feature vector
    // 超时 100ms，超时返回 error
    // 调用方收到 error → 标记 event 为 skip，不入场
}

func (c *PretouchMLClient) Health(ctx context.Context) error {
    // GET /health，用于启动检查和健康监控
}
```

### 4.5 Sizing Calculator

```go
// internal/service/pretouch_sizing.go
package service

const (
    pretouchBaseShare       = 0.80
    pretouchCostQ50Penalty  = 0.50
    pretouchCostQ50Threshold = 0.116865
)

type PretouchSizingResult struct {
    RFProbability    float64
    BaseMultiplier   float64  // clip(prob × 2, 0, 2)
    CostPenalty      float64  // 1.0 or 0.5
    FinalMultiplier  float64  // base × cost_penalty
    PositionSize     float64  // pretouchBaseShare × finalMultiplier
}

func computePretouchSizing(event domain.PretouchEvent) PretouchSizingResult {
    // 1. RF probability → multiplier = clip(prob × 2, 0, 2)
    //    注：live 阶段 RF probability 需要从预训练模型推理
    //    简化方案：使用 CSV 中已有的 prob_success 列作为 lookup
    //    或者：使用固定 multiplier=1.0（退化为 raw fixed sizing）
    //
    // 2. cost_q50_cut050: if roundtrip_cost_atr >= threshold → × 0.5
    //
    // 3. final = baseShare × multiplier × costPenalty
}
```

## 5. Launch Template

```go
// 新增到 live_launch_templates.go

func buildEthPretouchTimingTemplate() LiveLaunchTemplate {
    return LiveLaunchTemplate{
        Key:                 "binance-testnet-eth-pretouch-timing",
        Name:                "Binance Testnet ETHUSDT Pretouch Timing",
        Description:         "ETHUSDT 1h pretouch timing 策略：timing classification × RF sizing × cost_q50_cut050。",
        Symbol:              "ETHUSDT",
        SignalTimeframe:     "1h",
        DefaultDispatchMode: "manual-review",
        // ... (signal bindings: kline 1h + trade tick + order book)
        LaunchPayload: LiveLaunchOptions{
            LiveSessionOverrides: map[string]any{
                "symbol":                  "ETHUSDT",
                "signalTimeframe":         "1h",
                "strategyEngine":          "bk-live-eth-pretouch-timing",
                "executionStrategy":       "book-aware-v1",
                "dispatchMode":            "manual-review",
                // Exit params (from research)
                "stop_loss_atr":           0.45,
                "breakeven_at_r":          0.8,
                "trail_start_r":           1.5,
                "trail_buffer_atr":        0.05,
                "max_hold_hours":          2.0,
                // Sizing
                "positionSizingMode":      "pretouch_timing_rf",
                "pretouchBaseShare":       0.80,
                "pretouchCostQ50Threshold": 0.116865,
                "pretouchCostQ50Penalty":  0.50,
                // Speed gate
                "pretouchSpeedThreshold":  0.228106,
                // Quality filters
                "pretouchMaxPreTouchSec":  1800,
                "pretouchMaxEff300s":      1.0,
            },
        },
    }
}
```

## 6. RF Probability + Timing Classifier 推理方案（rolling 重训）

### 6.1 核心约束

**严格对齐 research lead 语义：**
- RF probability：必须用真实模型推理，不退化为 fixed multiplier
- Timing classifier (DT3)：必须支持 rolling 重训，不硬编码规则
- 公式：`multiplier = clip(prob × 2, 0, 2)`

### 6.2 架构选择：Python sidecar（推荐）

Go 主进程通过本地 HTTP 调用 Python sidecar 做模型推理和 rolling 重训。

```
┌──────────────────────┐      HTTP        ┌──────────────────────────┐
│  Go: bkTrader        │ ──────────────→  │  Python: ML sidecar       │
│  - Pretouch detector │  POST /infer     │  - sklearn DT3 + RF       │
│  - Live execution    │  ←─────────────  │  - rolling retrain (cron) │
│                      │  {regime, prob}  │  - feature pipeline       │
└──────────────────────┘                  └──────────────────────────┘
        ↓                                            ↑
   Signal Intent                              事件历史 + bar cache
```

**为什么不选 ONNX：**
- DT3 树结构 + RF 200 棵树的 ONNX 转换可行，但需要在 live 里同时维护 train pipeline 和 inference pipeline，工程复杂度高
- Rolling 重训意味着模型权重每周/每月变化，ONNX 静态化与此冲突
- Python sidecar 让 train 和 inference 共享同一套 sklearn 代码，零代码漂移

**为什么不选 Go 原生实现：**
- Go 没有成熟的 sklearn 等价库
- 自己实现 DT3 + RF 推理 = 维护两套代码 = 必然漂移
- 违反"同一判定只能有一个入口"原则（AGENTS Review 黄金规则 #3）

### 6.3 Sidecar 接口

```python
# scripts/timing_probability_sidecar/server.py
# FastAPI/Flask 单进程，监听本地 9101 端口

POST /infer
Request:
{
    "event": {
        "event_id": "eth_20260515_120300",
        "symbol": "ETHUSDT",
        "side": "long",
        "touch_time": "2026-05-15T12:03:00Z",
        "features": {
            "signal_atr_percentile": 0.45,
            "roundtrip_cost_atr": 0.12,
            "prev1_body_atr": 0.30,
            ...  // 全部 10 个 Original_10_Features
        }
    }
}

Response:
{
    "timing_regime": "fast",        // skip / fast / slow
    "rf_probability": 0.62,
    "sizing_multiplier": 1.24,      // clip(0.62 × 2, 0, 2)
    "model_version": "20260515_v1",
    "model_age_hours": 36.5,
    "inference_time_ms": 8
}
```

```python
GET /health
Response:
{
    "status": "ready",
    "model_version": "20260515_v1",
    "model_trained_at": "2026-05-13T00:00:00Z",
    "model_age_hours": 60.5,
    "next_retrain_at": "2026-05-20T00:00:00Z"
}
```

### 6.4 Rolling 重训策略

**重训触发：**
- 时间触发：每周日 00:00 UTC 自动重训
- 数据触发：累计新 events ≥ 50 时强制重训
- 手动触发：通过 admin API

**重训流程：**
```
1. 加载最近 N 个月的 events_pool（rolling window）
   - DT3: 最近 12 个月
   - RF: 最近 12 个月
2. 运行 timing_probability_unified.unified_runner pipeline
   - 等同于 research pipeline，参数完全一致
3. 训练新模型，验证：
   - DT3 LOOCV calendar_sum > 上一版本 × 0.7（防止退化）
   - RF test AUC > 0.55（防止失效）
4. 验证通过 → 原子替换 production model
   - 旧模型保留 7 天用于回滚
5. 验证失败 → 保留旧模型，发告警
```

**重训失败 fallback：**
- 旧模型继续服务（不会断信号）
- 触发告警通知人工介入
- 连续 2 次重训失败 → 自动暂停 strategy（manual-review 仍然 dispatch=false）

### 6.5 Sidecar 与 Go 的解耦

| 层 | 职责 | 失败处理 |
|---|------|---------|
| Go 主进程 | 事件检测、特征计算、orchestration | 不依赖 sidecar 启动 |
| Python sidecar | ML 推理 + 重训 | 独立进程，crash 不影响 Go |
| Go 调用 sidecar | 超时 100ms，失败 fallback | sidecar 不可用 → 标记 event 为 `skip`，不入场 |

**关键原则：sidecar 不可用 = 不入场，而不是退化为 fixed multiplier。** 这避免了"失败装成功"反模式（AGENTS Review 黄金规则 #2）。

### 6.6 部署

```yaml
# deployments/docker-compose.testnet.yml 新增
services:
  ml-sidecar:
    build:
      context: scripts/timing_probability_sidecar
    ports:
      - "127.0.0.1:9101:9101"  # 仅本地访问
    volumes:
      - ./scripts/timing_probability_sidecar/models:/app/models
      - ./research/tick_flow_event_sources:/app/data:ro
    environment:
      - MODEL_DIR=/app/models
      - DATA_DIR=/app/data
      - RETRAIN_SCHEDULE=weekly
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9101/health"]
      interval: 30s
      timeout: 5s
      retries: 3
```

### 6.7 实施清单

新增组件：

```
scripts/timing_probability_sidecar/
├── Dockerfile
├── requirements.txt
├── server.py                    # FastAPI 服务
├── inference.py                 # 模型推理逻辑
├── retrain.py                   # rolling 重训
├── models/
│   ├── current/
│   │   ├── timing_classifier.pkl
│   │   ├── rf_probability.pkl
│   │   ├── feature_imputer.pkl
│   │   └── metadata.json
│   └── archive/
│       └── 20260515_v1/...
└── tests/
    └── test_inference.py
```

**复用 research 代码：**
- `inference.py` 直接 import `timing_probability_unified.timing_classifier`
- `inference.py` 直接 import `timing_probability_unified.probability_model`
- `retrain.py` 直接调用 `timing_probability_unified.unified_runner.main()`
- 这样确保 sidecar 与 research 100% 一致，零代码漂移

## 8. Pretouch 事件检测逻辑

### 8.1 触发条件（从 research 定义）

一个 pretouch 事件在以下条件同时满足时触发：

1. **Breakout level touch**：当前 1h signal bar 内，1s tick price 首次触及 `prev_high_2`（long）或 `prev_low_2`（short）
2. **pre_touch_seconds <= 1800**：触发发生在 signal bar 开始后 30 分钟内
3. **eff_300s <= 1.0**：触发前 300s 的效率不超过 1.0
4. **speed_300s_atr >= 0.228106**：触发前 300s 的速度达到阈值（train q10）

### 8.2 实时计算需求

| 特征 | 计算方式 | 数据需求 |
|------|---------|---------|
| `touch_extension_atr` | `(touch_price - level) / ATR` | 当前 tick + level + ATR |
| `speed_300s_atr` | `abs(price_change_300s) / ATR` | 最近 300s tick 滚动窗口 |
| `eff_300s` | `abs(net_move) / total_range` over 300s | 最近 300s tick 滚动窗口 |
| `pre_touch_seconds` | `touch_time - signal_bar_start` | 当前 bar 开始时间 |
| `roundtrip_cost_atr` | `(spread + 2×slippage) / ATR` | order book spread + ATR |
| `ATR` | 前 N 根 1h bar 的 range 均值 | 历史 1h bar |
| `level` | `prev_high_2` (long) / `prev_low_2` (short) | 前 2 根 1h bar |

### 8.3 状态管理

```go
type PretouchDetectorState struct {
    // 1h bar 历史（用于 ATR 和 level 计算）
    hourlyBars     []HourlyBar  // 最近 24 根
    currentBar     *HourlyBar   // 当前未闭合 bar
    
    // 300s 滚动窗口
    tickWindow     *RollingTickWindow  // 最近 300s 的 tick
    
    // 事件去重
    lastTouchTime  time.Time    // 防止同一 bar 内重复触发
    touchedThisBar bool         // 当前 bar 是否已触发过
}
```

## 9. Exit 管理

Exit 逻辑复用现有 live session 的 trailing stop / breakeven / max hold 机制：

| 参数 | 值 | 对应 live session override |
|------|---|--------------------------|
| Initial SL | 0.45 ATR | `stop_loss_atr=0.45` |
| Breakeven trigger | 0.8R | `breakeven_at_r=0.8` |
| Trail start | 1.5R | `trail_start_r=1.5` |
| Trail buffer | 0.05 ATR | `trail_buffer_atr=0.05` |
| Max hold | 2h | `max_hold_hours=2.0` |

这些参数通过 `LiveSessionOverrides` 传入，现有 live execution 逻辑已支持。

## 10. 风险评估

| 风险 | 等级 | 缓解措施 |
|------|------|---------|
| 新策略引擎 bug 导致误下单 | 高 | `dispatchMode=manual-review` 强制人工审核 |
| Pretouch 检测逻辑与 research 不一致 | 中 | 首期只记录信号不执行，对比 research 回测 |
| 滚动窗口内存泄漏 | 低 | 固定 300s 窗口 + 定期清理 |
| ATR/level 计算精度差异 | 中 | 使用与 research 相同的计算逻辑 |
| RF probability 缺失 | 低 | 首期退化为 multiplier=1.0 |

## 11. 验证计划

### Phase 1：信号对比（1-2 周）

- 运行 testnet shadow，记录所有检测到的 pretouch 事件
- 与 research 的 `events_pool.csv` 对比：
  - 事件数量是否接近（预期 ~15-20 events/month for ETH）
  - touch_time 是否一致
  - timing regime 分类是否一致
- 不执行任何交易

### Phase 2：模拟执行（2-4 周）

- 开启 `auto-dispatch`（仍在 testnet）
- 记录实际成交价 vs research 的 `same_close` / `next_adverse` 假设
- 计算 live fill 与 research 的 parity error
- 目标：live fill 在 research `next_adverse_xslip3bps` 口径以内

### Phase 3：评估决策

- 如果 Phase 2 的 live fill parity < 5bps → 可以考虑 mainnet shadow
- 如果 parity > 10bps → 需要调整执行参数或放弃

## 12. 实施步骤

| 步骤 | 文件 | 描述 | 风险 |
|------|------|------|------|
| 1 | `internal/domain/pretouch_event.go` | 新增领域模型 | L0 |
| 2 | `internal/service/pretouch_event_detector.go` | 事件检测器 | L1 |
| 3 | `internal/service/pretouch_timing_classifier.go` | Timing 规则 | L1 |
| 4 | `internal/service/pretouch_sizing.go` | Sizing 计算 | L1 |
| 5 | `internal/service/strategy_engine_pretouch_timing.go` | 策略引擎 | L2 |
| 6 | `internal/service/live_launch_templates.go` | 新增模板 | L1 |
| 7 | `internal/service/strategy_registry.go` | 注册引擎 | L1 |
| 8 | 测试 | Unit tests for detector + classifier | L0 |

## 13. 开放问题

1. **RF probability 推理**：首期用 multiplier=1.0 还是实现简化规则？
2. **ATR 计算周期**：用最近 14 根 1h bar 的 range 均值，还是用 CSV 中已有的 ATR？
3. **事件去重**：同一 1h bar 内只允许一次 pretouch 触发？
4. **与现有 breakout 信号的关系**：pretouch 事件是否会与现有 T2 breakout 信号冲突？
   - 建议：pretouch timing 作为独立 session 运行，不与现有 BTC 30m session 共享

## 14. 依赖

- 不引入新的外部依赖
- 不修改 `go.mod`
- 不修改现有策略引擎代码
- 不修改 `live_execution.go`
- 不修改数据库 schema（pretouch 事件记录在 session state 中）
