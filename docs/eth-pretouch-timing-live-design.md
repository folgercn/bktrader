# ETH Pretouch Timing Live 设计说明

分支：`feature/eth-pretouch-timing-live`

## 目标

把 research 验证过的 ETHUSDT 1h pretouch timing 事件接入 live testnet shadow：

- 用 Binance trade tick 触发策略评估。
- 从 `sourceStates` / `signalBarStates` 读取 1h signal bar 历史和当前未闭合 bar。
- 检测 original_t2 pretouch 触达事件。
- 用 Go 原生 DT3 timing classifier 和 RF probability 做 skip/advance-plan 与 sizing。
- 只生成 `manual-review` proposal，不默认自动下单。

## Research 固化参数

| 项 | 值 |
| --- | --- |
| Symbol | `ETHUSDT` |
| Signal timeframe | `1h` |
| Pretouch event source | `pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1` |
| `pretouchMaxPreTouchSec` | `1800` |
| `pretouchMaxEff300s` | `1.0` |
| `pretouchSpeedThreshold` | `0.228106` |
| `pretouchCostQ50Threshold` | `0.116865` |
| `pretouchCostQ50Penalty` | `0.50` |
| `pretouchBaseShare` | `0.80` |
| `positionSizingMode` | `intent_quantity` |

## Runtime 架构

```text
Binance kline 1h  -> signal bar state/history
Binance trade tick -> EvaluateSignal trigger
Binance order book -> spread/cost feature

bk-live-eth-pretouch-timing
  -> PretouchEventDetector
  -> PretouchModelBundle (Go-native JSON model)
  -> StrategySignalDecision(action=advance-plan)
  -> deriveLiveSignalIntent
  -> book-aware-v1 execution proposal
```

当前实现文件：

- `internal/service/strategy_engine_pretouch_timing.go`
- `internal/service/pretouch_event_detector.go`
- `internal/service/pretouch_tree.go`
- `internal/service/pretouch_trainer.go`
- `cmd/pretouch-train/main.go`
- `internal/domain/pretouch_event.go`
- `data/pretouch_model.json`

## 模型加载与重训

live 进程默认加载 `data/pretouch_model.json`。部署环境可通过 `BK_PRETOUCH_MODEL_PATH` 覆盖路径。

模型不可用、JSON 不合法、feature 数量不匹配或 tree feature index 越界时，策略视为 `no_model_loaded`，不入场，也不会退化为 fixed sizing。

重训入口：

```bash
go run ./cmd/pretouch-train \
  --events-csv research/tick_flow_event_sources/.../pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1.csv \
  --out data/pretouch_model.json \
  --forward-start 2025-11-01 \
  --train-ratio 0.6 \
  --dt-depth 3 \
  --rf-estimators 200 \
  --seed 42
```

训练输入 CSV 目前不在本 PR 内归档；上线候选前必须补齐训练输入 provenance/hash 或把训练数据作为可审计 artifact 管理。

## Live 决策语义

策略只处理 `ETHUSDT`。每个 trigger tick 执行：

1. 从 runtime signal bar state 同步最近 closed bars 和当前 1h bar。
2. 更新 300s tick window。
3. 检查 original_t2 long/short 结构 ready，使用 `breakout_shape_tolerance_bps`，默认 `0.5` bps。
4. 当前 tick 触达 `prev_high_2` / `prev_low_2` 后计算特征。
5. 质量过滤未通过则返回 `wait`，理由写入 `Reason`。
6. timing classifier 返回 `skip` 或未知 regime 时返回 `wait/timing_skip`。
7. 正常事件返回 `advance-plan`，metadata 包含 `signalBarDecision`、`nextPlanned*`、`suggestedQuantity` 和 order book 字段。

`suggestedQuantity = pretouchBaseOrderQuantity × pretouchBaseShare × clip(rf_probability × 2, 0, 2) × costPenalty`

执行层通过 `positionSizingMode=intent_quantity` 使用 intent 里的 `suggestedQuantity`。模板仍保留 `defaultOrderQuantity=0.100` 作为人工可见的基础数量，但 pretouch entry 的实际 proposal 数量以 intent sizing 为准。

## 安全边界

- 模板默认 `dispatchMode=manual-review`。
- `sandbox=true`，只面向 Binance Futures testnet。
- 不修改现有 BTC baseline/live 策略语义。
- 模型失败、feature 缺失、未知 timing regime、无 base quantity 均不入场。
- 同一 1h signal bar 内 detector 只触发一次 pretouch event。

## 验证范围

本 PR 最小验证应覆盖：

- `PretouchEventDetector` 正常 long/short 检测、去重、bar history 不足、quality gate 和 bid/ask cost。
- `TreeNode` / `RandomForest` 推理边界、坏 feature 长度不 panic、模型 Save/Load 与 legacy `rf_auc` 兼容。
- `bkLiveEthPretouchTimingEngine.EvaluateSignal` 的 `no_model_loaded`、`timing_skip`、正常 `advance-plan` 和 live-compatible metadata。
- `intent_quantity` sizing contract，确认 RF/cost sizing 后的 intent quantity 不被 fixed quantity 覆盖。
- launch template 不包含外部 Python 推理服务依赖说明。
