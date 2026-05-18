# ETH Pretouch Timing Live 设计说明

分支：`feature/eth-pretouch-timing-live`

## 目标

把 research 验证过的 ETHUSDT 1h pretouch timing 事件接入 live testnet：

- 用 Binance trade tick 触发策略评估。
- 从 `sourceStates` / `signalBarStates` 读取 1h signal bar 历史和当前未闭合 bar。
- 检测 original_t2 pretouch 触达事件。
- 用 Go 原生 DT3 timing classifier 和 RF probability 做 skip/advance-plan 与 sizing。
- 生成 live execution proposal，是否 `manual-review` / `auto-dispatch` 由 live session 的 `dispatchMode` 决定。

## 当前 production-aligned research lead

截至 2026-05-18，生产代码以 `origin/main` 为准时，当前 live ETH pretouch 策略对齐的是
2026-05-15 的 **Timing-Probability Unified Framework / ETH Pretouch Timing** lead。后续文档、
research 扩展和生产排障中，未特别说明时，`research lead` 默认指本节定义的版本。

| 维度 | 固化口径 |
| --- | --- |
| Research lead | `Timing-Probability Unified Framework / ETH Pretouch Timing` |
| 决策报告 | `research/entry_redesign/scripts/output/timing_probability_unified/20260515_unified_framework_decision_report.md` |
| Canonical event source | `pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1` |
| Canonical CSV | `research/tick_flow_event_sources/20260514_pretouch_full_window/feature_filtered_seed_events/robust_quality/pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1.csv` |
| 训练入口默认配置 | `internal/service/pretouch_trainer.go` / `DefaultPretouchTrainerConfig` |
| Live engine | `internal/service/strategy_engine_pretouch_timing.go` / `bk-live-eth-pretouch-timing` |
| Live detector | `internal/service/pretouch_event_detector.go` / `PretouchEventDetector` |
| Live template | `internal/service/live_launch_templates.go` / `binance-testnet-eth-pretouch-timing` |
| Model artifact | `data/pretouch_model.json` |
| Model version | `20260515_v1` (`trained_at=2026-05-15T07:05:01Z`) |
| Model features | `roundtrip_cost_atr`, `prev1_range_atr`, `prev1_close_pos_side`, `level_to_signal_open_atr`, `touch_extension_atr`, `speed_300s_atr`, `eff_300s`, `pre_touch_seconds` |
| Model metrics | `timing_loocv=0.85`, `rf_accuracy=0.7142857142857143` |
| Production live delta | Report spec 的 `trail_start=0.9` 已按 lead 建议在模板侧固化为 `trail_start_r=1.5`; live template 使用 `max_hold_hours=2.0`、`pretouchBaseShare=0.80` |

这个 lead 的身份由 **event source + model version + live engine/template 参数**共同定义，不由单个收益数字定义。
当前生产对齐的不是 2026-05-13 的 `local_context_event_execution` union lead，也不是旧 V6
`candidate_001` / `reentry_window` baseline。需要复现这些历史对照组时，必须在实验说明中显式标注。

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
| `breakout_shape_tolerance_bps` | `0.5` |
| `positionSizingMode` | `intent_quantity` |
| `dispatchMode` | 模板默认展示 `auto-dispatch`，live session 可显式切回 `manual-review` |

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

## Breakout 结构展开基准

当前生产的 `breakout_shape_tolerance_bps=0.5` 不是“放宽触发容差”，而是要求 `prev_high_2` /
`prev_low_2` 相对上一根 closed bar 有额外分离度：

- Long ready: `prev_high_2 > prev_high_1 * (1 + tolerance_bps / 10000)`
- Short ready: `prev_low_2 < prev_low_1 * (1 - tolerance_bps / 10000)`

因此它比原始 `original_t2` 严格比较更窄。后续如果要捕获 near-equal 结构，例如相邻两根 bar 的
high/low 近乎持平但当前 bar 已经触达 `prev_high_2` / `prev_low_2`，必须在当前 lead 上展开，而不是切回
`local_context_event_execution` 或旧 V6 口径。

建议 research 展开顺序：

1. 固定当前 lead 的其余条件，只扫描 breakout shape：`restrictive_0p5bps`（当前生产）、
   `strict_0bps`（原始 strict compare）、`near_equal_slack_0p25/0p5/1p0bps`。
2. 事件进入后继续沿用 canonical quality chain：`pretouch_small_pullback` seed、RF q50、`speed_300s_atr >= train q10`、
   `pre_touch_seconds <= 1800`、`eff_300s <= 1.0`、`cost_q50_cut050`。
3. 先用当前 `20260515_v1` model 做 DT3 timing + RF sizing 复评；如果新增事件的 feature 分布明显漂移，再用同一
   `DefaultPretouchTrainerConfig` 克隆训练口径重训一版对照模型。
4. 回测必须同时跑 `same_close`、next-second adverse fill 和 `1/3/5/7/10bps` slippage matrix；不得把 `re_p`
   或任意更乐观成交口径作为主结果。
5. 比较时同时看全量 lead 指标和新增事件增量质量：calendar sum、worst SM、neg SM、trade count、平均/分位 RF probability、
   timing skip 率、成本惩罚占比。
6. 如果增加上下文过滤，长周期最多使用 `4h-12h` 可实时解释的 flow/trend 特征；不要使用月度 gate 预测一个月走势。
7. 在上述矩阵没有证明收益和风险都改善前，不修改 live 默认 `breakout_shape_tolerance_bps`。

### Breakout 结构展开阶段结果

截至 2026-05-18，已完成三层 research-only 展开，均未修改 live 默认参数：

1. `breakout_shape_expansion.py`
   - 直接比较 `restrictive_0p5bps`、`strict_0bps`、`near_equal_slack_0p25/0p5/1p0bps`。
   - 结果：放宽到 strict/slack 只增加 8 到 19 个 model-advance 事件，但新增事件 same-close 增量均为负；
     不支持把 `breakout_shape_tolerance_bps` 直接放宽。
   - 当前 production shape 对 canonical lead 结构覆盖较高：151/154 canonical ETH events ready，
     66/68 canonical unified trades ready；漏掉的 canonical trade weighted pnl 合计约 `0.007939`。

2. `breakout_structure_confirmation_sweep.py`
   - 在当前 production shape 上测试 touch-second close reclaim、0.03/0.05/0.10 ATR follow-through、
     clean pre-confirm adverse 等 post-touch 结构确认；确认信号按确认秒 close 重新定价。
   - 结果：`touch_close_ext_ge_0p03atr` 将宽事件池 same-close calendar sum 从 `-0.319441` 改善到 `-0.121593`，
     但 trade count 只有 151，且 next-second adverse 10bps 仍为 `-0.313283`；不支持单独上线 post-touch confirmation。

3. `breakout_structure_quality_gate_sweep.py`
   - 在 561 个 current-shape model-advance 事件上测试结构质量切片。
   - 候选结果：
     `low_rf_slope_up` same-close `0.122515` / adverse10 `0.033666` / 96 trades；
     `low_eff_low_atr_pct` same-close `0.109478` / adverse10 `0.053389` / 49 trades；
     `level_far_sma_gap_up` same-close `0.103569` / adverse10 `0.047704` / 54 trades。
   - 这些阈值来自当前宽事件池挖掘，样本偏小，只能作为下一阶段 walk-forward / retrain 候选；
     在跨期验证前不得改 live 默认。

4. `breakout_structure_walkforward_validation.py`
   - 对结构质量 gate 做 trailing train / next-month forward 验证，阈值只允许从过去窗口分位数计算。
   - 3 个月训练窗口，8 个 forward 月：动态选择器 same-close `0.171124`，adverse10 `0.059066`，
     85 trades；固定 `low_eff_low_atr_q20_q40` adverse10 `0.072854`，固定 `low_rf_slope_up_q40_q60`
     adverse10 `0.060757`。
   - 4 个月训练窗口，7 个 forward 月：动态选择器 same-close `0.161410`，adverse10 `0.060353`，
     75 trades。
   - 5 个月训练窗口，6 个 forward 月：动态选择器 same-close `0.073731`，adverse10 `0.024575`，
     48 trades；但固定 `low_eff_q20` 在同窗口 same-close `0.184845`，adverse10 `0.070874`。
   - 共同 forward 月 `2025-11..2026-04` 下，3m/4m 动态选择器 adverse10 约 `0.0975/0.0984`，
     fixed `low_eff_q20` adverse10 约 `0.0669/0.0542/0.0709`；说明 `eff_300s` 低分位结构质量更稳定，
     而动态 selector 的选择准则仍需再收敛。

5. `breakout_structure_lead_expansion_combo.py`
   - 保留 canonical lead trades 不动，只加入与 canonical event source 不重叠的 current-shape live-like 结构质量事件；
     overlap 以 `(signal_start, side)` 在 154 个 canonical ETH events 上去重。
   - 当前 production template replay 口径下（`trail_start_r=1.5`、`max_hold_hours=2.0`），lead gate-on
     same-close 为 `0.302172`，lead adverse10 为 `0.229716`。现有 CSV artifact 口径为
     `0.305333` / `0.232881`，差异来自 artifact 使用 `max_hold_hours=4.0`；这与
     `20260515_unified_framework_decision_report.md` 正文中的 `22.38%/15.17%` 表格也不一致，
     后续归档前需要统一该手写决策报告的 provenance。
   - 组合结果显示有叠加空间：
     `wf3_low_eff_low_atr` 在修正 walk-forward 条件字符串精度后，source 42 个事件、去掉 1 个 overlap、
     加 41 个非重叠事件，same-close `0.402453`，exact adverse10 `0.282598`，
     adverse10 lift `0.052882`，worst SM `-0.010290`，negative SM `2`；
     `wf5_low_eff_q20` 加 71 个非重叠事件，same-close `0.462950`，exact adverse10 `0.285020`，
     adverse10 worst SM `-0.019114`，negative SM `2`；
     `static_low_eff_low_atr_pct` 加 48 个非重叠事件，same-close `0.389906`，exact adverse10 `0.263133`，
     adverse10 worst SM `-0.010290`，negative SM `1`，且 same-close worst SM 仍为正。
   - adverse10 组合已重建 canonical lead per-trade/month ledger，并按 production template exit contract
     与 expansion leg 合并；combo adverse worst-month / negative-month 可以按统一逐笔账本解读。

6. `breakout_structure_model_retrain_validation.py`
   - 不复用 expansion event CSV 中旧的 `timing_prediction` / `rf_probability`，而是按 production template exit
     (`trail_start_r=1.5`、`max_hold_hours=2.0`) 重新 simulate delays、重训 DT timing + RF probability，
     并在 `2025-11..2026-04` forward split 上评估。
   - `production8`（当前 Go live trainer 特征合同）优于扩大后的 `structure13`，所以暂不建议扩大 live 模型特征面。
   - Forward adverse10 结果：
     canonical-only `0.299716`，worst SM `0.002688`，negative SM `0`，78 trades；
     `combo_wf3_low_eff_low_atr` `0.408512`，worst SM `-0.002244`，negative SM `1`，115 trades；
     `combo_wf5_low_eff_q20` `0.423931`，worst SM `-0.019235`，negative SM `2`，149 trades。
   - 需要注意：`wf5_low_eff_q20` 的 expansion events 全部在 forward 窗口，模型训练仍只来自 canonical 训练窗；
     它验证的是“canonical 模型可以泛化到该结构扩展事件源”，不是“扩展事件参与训练后提高模型”。

7. `breakout_structure_candidate_artifact.py`
   - 将 `wf3_low_eff_low_atr` 单独整理为可审计候选 artifact：
     `breakout_structure_wf3_candidate_events.csv`、`breakout_structure_wf3_candidate_expansion_events.csv`、
     `breakout_structure_wf3_candidate_manifest.json`、`breakout_structure_wf3_candidate_promotion_report.md`。
   - 候选 event source 组合为 canonical 154 + wf3 extra 41 = 195 个事件；split 为 train 42（全 canonical）、
     test 29（canonical 26 + wf3 3）、forward 124（canonical 86 + wf3 38）。
   - Artifact 显式记录输入 CSV SHA256、walk-forward 月度 gate 条件、production replay / retrain 指标、
     以及旧 6 位小数 gate 字符串回放需要 `5e-7` 数值容差的原因。
   - 当前推荐：`wf3_low_eff_low_atr` 可以作为下一轮 cross-asset / 更长历史验证对象，但还不是 live 默认策略候选。
     阻塞项包括 exact event-source 仅覆盖 `2025-06..2026-04`，BTC 扩展 path 尚未重建，长历史 bars cache
     尚未生成同口径 pretouch + current-shape + wf3 事件源。

8. `breakout_structure_cross_asset_validation.py`
   - 新增 symbol-specific cross-asset runner，输出不会覆盖 ETH 主产物；支持 `flow/plain` 1s OHLCV cache、
     自定义 `eval_start/eval_end`，用于验证 `low_eff_low_atr_q20_q40` 是否跨资产泛化。
   - ETH sanity（`2025-06..2026-04`，3m train / next-month forward）复现 walk-forward 固定 gate：
     baseline current-shape adverse10 `-0.530945`、7 个负月；`low_eff_low_atr_q20_q40` adverse10 `0.072854`、
     worst month `-0.012840`、2 个负月、42 trades。
   - BTC 同窗口同口径不通过跨资产验真：baseline adverse10 `-0.622999`、8 个负月；
     `low_eff_low_atr_q20_q40` 虽显著减亏到 adverse10 `-0.059469`，但 worst month `-0.020220`、
     6 个负月、38 trades，仍不是可推广 alpha。
   - BTC 更早历史（`research/historical_extension/bars_cache`，plain 1s，`2025-01..2025-04`）继续显示同样模式：
     3m train 只有 2025-04 一个 forward 月，baseline adverse10 `-0.114854`，
     gate 后 `-0.006364`；1m train 的 2025-03..2025-04 两个月 baseline adverse10 `-0.198943`，
     gate 后 `-0.031058`。结论仍是“过滤减亏”，不是正收益确认。

9. `breakout_structure_cross_asset_gate_search.py`
   - 在 cross-asset current-shape event pool 上扫描一组小而可解释的 structure gates：
     low/high `eff_300s`、ATR percentile、speed、RF、level distance、wick touch、side-normalized SMA gap/slope
     及二元组合。所有阈值只从 trailing train 窗口分位数计算，forward 月才计入结果。
   - BTC `2025-06..2026-04`、3m train、`min_train_trades=5`：所有候选 adverse10 仍为负。
     最好的是 `low_eff_high_speed_q20_q60`，12 trades，adverse10 `-0.033263`、7 个负月；
     `low_eff_low_atr_q20_q40` 为 38 trades，adverse10 `-0.059469`、6 个负月。动态 selector 只在
     2026-03 选中一次 `high_speed_q80`，整体 adverse10 `-0.558606`，基本等同失败。
   - ETH 同口径 gate search：固定 `low_eff_low_atr_q20_q40` 仍是最强候选，42 trades，adverse10 `0.072854`、
     worst month `-0.012840`、2 个负月；次优 `low_eff_low_atr_q30_q50` adverse10 `0.048193`，
     `low_eff_q20` adverse10 `0.039887` 但负月更多。动态 selector 被 train 噪声带偏，forward adverse10
     只有 `-0.357363`，不应作为下一阶段方向。
   - ETH 更早历史 `2025-03..2025-06` flow 1s cache 可用。固定 `low_eff_low_atr_q20_q40` 在 1m train
     的 forward `2025-04..2025-06` 只有 17 trades，adverse10 `-0.045447`，其中 2025-04/05 仍亏；
     2m train 的 `2025-05..2025-06` adverse10 `-0.044765`；3m train 只有 2025-06 一个 forward 月，
     adverse10 `-0.001875`。这说明 `wf3` 不是全历史稳健正收益，只是在早期明显减亏。
   - 按“长周期最多 4h-12h”约束新增 closed-bar context features：
     `ctx4h_side_return_atr` / `ctx12h_side_return_atr`，只使用 signal bar 前已闭合 1h 数据。
     后期 `2025-06..2026-04` 中，`low_eff_low_atr_ctx4h_up` 为 16 trades，adverse10 `0.066304`，
     worst month `-0.001849`、1 个负月；相对原 `low_eff_low_atr_q20_q40` 的 `0.072854` 收益略低，
     但 worst month 明显更稳。早期 `2025-04..2025-06` 中，`low_eff_low_atr_ctx12h_up` 只有 5 trades，
     adverse10 `0.002595`、worst month `-0.003435`，样本过小，只能作为风险收缩 overlay 苗头。

当前判断：breakout alpha 的瓶颈不在 near-equal tolerance，而在宽 live-like 事件池缺少 canonical 上游质量收敛。
`eff_300s` 低分位、低 ATR percentile、`prev_sma5_slope_atr`、`level_to_signal_open_atr`、
`prev_sma5_gap_atr` 这些可实时解释的结构质量特征已通过第一轮 walk-forward 验真；目前 promotion 排序更偏向
`wf3_low_eff_low_atr` 作为 ETH-local 稳健候选、`wf5_low_eff_q20` 作为 ETH-local 进攻候选。BTC cross-asset
结果已把推广风险抬高：下一步应优先做 ETH 更长历史 event-source 重建或寻找 BTC 专属上游质量链，而不是把
当前 `wf3` 直接推成 multi-asset/live 默认 breakout 结构。

## 安全边界

- 模板不在 `LaunchPayload.LiveSessionOverrides` 中硬编码 `dispatchMode`；前端/调用方必须在创建 live session 时显式传入实际模式。
- 当前推荐模板默认模式为 `auto-dispatch`，但系统全局空值兜底仍是 `manual-review`。
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
