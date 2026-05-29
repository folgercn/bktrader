# ETH Pretouch Timing Live 设计说明

分支：`feature/eth-pretouch-timing-live`

## 目标

把 research 验证过的 ETHUSDT 1h pretouch timing 事件接入 live testnet：

- 用 Binance trade tick 触发策略评估。
- 从 `sourceStates` / `signalBarStates` 读取 1h signal bar 历史和当前未闭合 bar。
- 检测 original_t2 pretouch 触达事件。
- 用 Go 原生 DT3 timing classifier 和 RF probability 做 skip/advance-plan 与 sizing。
- 生成 live execution proposal，是否 `manual-review` / `auto-dispatch` 由 live session 的 `dispatchMode` 决定。

## 当前 research lead / testnet shadow target

截至 2026-05-21，后续文档、research 扩展和生产排障中，未特别说明时，`research lead`
默认指当前 risk-on shadow bundle：**原 2026-05-15 ETH pretouch lead + lead `0.20..0.40 ETH`
quantity-band 条件仓位放大 + T3 overlay `2.0x` + T3 专用 RF/cost quality sizing +
T3 deterministic stop-gate lifecycle overlay**。原来的 2026-05-15 production-aligned lead 以后在本文档中
统一称为 `base lead` / `original lead`；只有做历史对照或保守基线时才单独引用它。

| 维度 | 固化口径 |
| --- | --- |
| Current research lead | `lead_q020_q040_overlay_q020_q040_t3_rf_cost_det_stop_gate_20260521` |
| 组合定义 | `base lead` + `pretouchShadowLeadQuantityBandSizing=true` (`0.20..0.40 ETH`) + `pretouchShadowOverlayScale=2.0` + `pretouchShadowOverlayQualitySizing=true` + T3 deterministic stop gate selector |
| Base / original lead | `Timing-Probability Unified Framework / ETH Pretouch Timing` |
| 决策报告 | `research/entry_redesign/scripts/output/timing_probability_unified/20260515_unified_framework_decision_report.md` |
| Canonical event source | `pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1` |
| Canonical CSV | `research/tick_flow_event_sources/20260514_pretouch_full_window/feature_filtered_seed_events/robust_quality/pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1.csv` |
| 训练入口默认配置 | `internal/service/pretouch_trainer.go` / `DefaultPretouchTrainerConfig` |
| Live engine | `internal/service/strategy_engine_pretouch_timing.go` / `bk-live-eth-pretouch-timing` |
| Live detector | `internal/service/pretouch_event_detector.go` / `PretouchEventDetector` |
| Live template | `internal/service/live_launch_templates.go` / `binance-testnet-eth-pretouch-timing` |
| Lead model artifact | `data/pretouch_model.json` |
| Lead model version | `20260515_v1` (`trained_at=2026-05-15T07:05:01Z`) |
| Lead model features | `roundtrip_cost_atr`, `prev1_range_atr`, `prev1_close_pos_side`, `level_to_signal_open_atr`, `touch_extension_atr`, `speed_300s_atr`, `eff_300s`, `pre_touch_seconds` |
| Lead model metrics | `timing_loocv=0.85`, `rf_accuracy=0.7142857142857143` |
| T3 overlay model artifact | `data/pretouch_t3_overlay_rf_model.json` |
| T3 overlay model version | `20260520_t3_overlay_rf_cost_v1` (`trained_at=2026-05-20T00:00:00Z`) |
| T3 overlay model features | `rf_probability`, `speed_300s_abs`, `eff_300s`, `touch_extension_abs`, `pre_touch_seconds`, `roundtrip_cost_atr`, `side_is_short` |
| T3 overlay model metrics | relaxed event pool `events=351`; walk-forward q020-q040 evidence: overlay `91.360615%`, lead q020-q040 + overlay `152.431532%` |
| T3 lifecycle lead | deterministic rule `abs(speed_300s_atr) >= 0.65`、`eff_300s >= 0.85`、`250 <= pre_touch_seconds <= 900`、`abs(touch_extension_atr) <= 0.40`；这个 rule 是选择器，不是出场动作 |
| T3 selected stop action | deterministic rule 命中的 T3 events 使用 `delay_trailing_updates_79m + hard_stop_atr_3.0`；未命中的 T3 events 继续走 PR447 lifecycle |
| T3 lifecycle metrics | relaxed q020-q040 event pool 上 deterministic stop-gate 后 overlay `194.323156%`，相对 relaxed q020-q040 baseline `91.360615%` lift `+102.962541pp` |
| Testnet direct status | 兼容 mode 仍为 `testnet_shadow_collect`，但候选语义是 Binance Futures testnet / `sandbox=true` 下直接真实提交 `0.20..0.40 ETH`，不是 mainnet live candidate |
| Lead sizing | `productionSuggestedQuantity` 先由 RF probability / cost penalty 生成；testnet direct guard 通过时再按 production/max-production quality score 映射到 `0.20..0.40 ETH` |
| T3 overlay sizing | 独立 `entry-t3-overlay` event source，先形成固定基准 `pretouchBaseOrderQuantity * 0.40 * 2.0 = 0.080 ETH`，再由 T3 RF/cost quality 直接映射到绝对数量 `0.20..0.40 ETH` |
| Production live delta | Report spec 的 `trail_start=0.9` 已按 base lead 建议在模板侧固化为 `trail_start_r=1.5`; live template 使用 `max_hold_hours=2.0`、`pretouchBaseShare=0.80` |

当前 `research lead` 的身份由 **base event source + model version + live engine/template 参数 +
testnet direct risk-on 参数 + T3 lifecycle selection rule**共同定义，不由单个收益数字定义。当前生产 testnet direct 对齐的不是
2026-05-13 的 `local_context_event_execution` union lead，也不是旧 V6 `candidate_001` /
`reentry_window` baseline。需要复现这些历史对照组或只复现 `base lead` 时，必须在实验说明中显式标注。

术语边界：

- `deterministic stop gate` 是 **选择器**：只决定哪些 T3 overlay event 有资格切换出场生命周期。
- `delay_trailing_updates_79m + hard_stop_atr_3.0` 是 **被选择后的出场动作**：延迟更新 trailing stop，
  但保留 hard stop；它不是“持仓 79 分钟不许止损”。
- research 产物里出现的 `min_hold_sl_60m` / `PR447 min_hold_sl_60m` 是历史回测命名。实盘对齐时不能把它
  直接翻译成“全 SL 分支 min-hold”，否则会错误阻断 catastrophic / hard stop。

### Research Lead 收益口径锁定

2026-05-21 更新后，`research lead` 默认不再等同于 `base_lead_same_close`、单独的
`base_lead_adverse10_exact`，也不再只等同于 PR447 的 T3 RF/cost quantity-band sizing。当前口径是
`base lead + lead 0.20..0.40 ETH quantity-band shadow sizing + T3 overlay 2.0x +
T3 RF/cost quantity-band shadow sizing + T3 deterministic stop-gate lifecycle overlay`。下表先保留 base lead
作为对照，避免后续把 `research lead`、`same_close`、`adverse10` 和 walk-forward/monthly gate 结果混用：

| 口径 | Calendar sum | Worst month | Negative months | Trades | 说明 |
| --- | ---: | ---: | ---: | ---: | --- |
| `base_lead_same_close` | `30.217222%` | `2.358319%` | `0` | `62` | 原 `base lead` 不加 adverse/slippage 压力的 replay；现在只作为无额外成交压力上界，不再单独代表当前 `research lead`。 |
| `base_lead_adverse10_exact` | `22.971648%` | `1.395821%` | `0` | `62` | 原 `base lead`、同一 production template exit contract 下的 next-second adverse + 10bps 压力口径；作为保守主基线。 |

当前 `research lead` 主收益口径：

| 口径 | Calendar sum | 条件 | 用途 |
| --- | ---: | --- | --- |
| `lead_quantity_0p20_0p40_adverse10` | `61.070916%` | 固定 canonical lead adverse10 ledger，只把 submitted lead quantity 按 live shadow 公式映射到 `0.20..0.40 ETH` | 正式 T2 quantity-band 线性 notional 回测口径；相对 `base_lead_adverse10_exact` lift `+38.099268pp`，相对旧 `lead 1.5x` lift `+26.613444pp`。 |
| `t3_rf_cost_quantity_0p20_0p40_exact_lead` | `68.610750%` | `base_lead_adverse10_exact` + T3 RF/cost walk-forward quantity band `0.20..0.40 ETH` | T3 quantity-band 证据；不是 mainnet promotion 结果，需 testnet fill/depth 验真。 |
| `t3_pr447_min_hold_sl_60m_overlay_q020_q040` | `45.639101%` | PR447 T3 RF/cost quantity-band overlay baseline；研究名保留 `min_hold_sl_60m`，但 live 对齐时不得实现成全局阻断 hard stop 的 min-hold SL | 当前 deterministic lifecycle lead 的对照基线。 |
| `t3_relaxed_deterministic_stop_gate_overlay_q020_q040` | `194.323156%` | relaxed event pool + q020-q040 quantity band；deterministic rule 命中的 24 个 event 切到 `delay_trailing_updates_79m + hard_stop_atr_3.0` | 当前 relaxed T3 lifecycle research lead；相对 relaxed q020-q040 baseline `91.360615%` lift `+102.962541pp`。 |
| `base_lead_adverse10_plus_t3_deterministic_stop_gate` | `78.432435%` | `base_lead_adverse10_exact` + deterministic T3 lifecycle overlay | 保守 base lead + 新 T3 lifecycle 的组合口径；用于和 PR447 `68.610750%` 对照。 |
| `lead_q020_q040_plus_t3_q020_q040_pr447` | `106.710018%` | `lead_quantity_0p20_0p40_adverse10` + PR447 `t3_rf_cost_quantity_0p20_0p40`，按月 additive bundle | PR447 sizing/lifecycle headline；worst month `-1.464655%`、negative month `1`、event-order DD `-4.682954%`，仍未建模更大 submitted quantity 的额外冲击。 |
| `lead_q020_q040_plus_t3_relaxed_deterministic_stop_gate` | `255.394073%` | `lead_quantity_0p20_0p40_adverse10` + relaxed deterministic T3 overlay，按月 additive bundle | 当前 relaxed research headline 口径；overlay worst month `-3.357137%`、negative month `1`、max DD `-7.827585%`；额外 slippage/depth degradation 仍未建模。 |
| `legacy_lead_1p5_overlay_2p0_strict15` | `28.970948%` | 旧 `lead_scale=1.5`、`overlay_scale=2.0`、strict impact proxy、15bp pressure | 历史连续性对照，不再代表当前 `research lead`。 |

2026-05-26 复验旧 T2 lead-side `reentry_window` + `reentry_size_schedule=[0.20, 0.10]`
语义：用 canonical 62 个 lead adverse10 event 作为外部 breakout lock，禁用 native original_t2 / native
t3_swing lock，只回放 lead-side reentry 生命周期。命令：

```bash
python3 research/entry_redesign/scripts/timing_probability_unified/t2_reentry_schedule_revalidation.py --write-ledgers
```

结果 `canonical_lead_t2_reentry_schedule` calendar sum 为 `-0.122020%`，相对当前
`lead_quantity_0p20_0p40_adverse10` 低 `-61.192937pp`，相对 `base_lead_adverse10_exact`
低 `-23.093668pp`；62 个 event / 62 个 lock 最终只有 10 笔成交，entry reason 为
`Zero-Initial-Reentry=5`、`SL-Reentry=5`，10 笔全部 `SL` 出场。月度结果为 2025-06
`-0.021590%`、2025-07 `0.000000%`、2025-08 `0.000000%`、2025-09
`-0.048050%`、2025-10 `-0.052380%`，negative months `3`。结论：旧 T2
lead-side reentry schedule 不恢复进当前 research lead；当前 T2 lead 继续以 selected-delay
adverse10 ledger + `0.20..0.40 ETH` quantity-band 为主口径，reentry 生命周期只保留为独立
T3 overlay / lifecycle research surface。

2026-05-27 进一步拆解 selected-delay 与 live 可部署 timing policy 的差异。当前
`lead_quantity_0p20_0p40_adverse10=61.070916%` 使用 research selected-delay
评估：`fast` prediction 在 `D0/D5` 中按 replay PnL 取优，`slow` prediction 在
`D10/D15/pullback` 中按 replay PnL 取优；这不是 live 当下可直接知道的未来信息。用同一
canonical lead replay、同一 adverse10 fill scenario 和同一 `0.20..0.40 ETH` quantity-band
重测固定 delay policy 后，62 个 speed-gate pass 事件结果如下：

- `fixed_d0_all_non_skip`：`56.342405%`，相对 selected-delay qband 低 `-4.728512pp`；
  这是当前 live 触达后 immediate-entry 最接近的收益 proxy。
- `fast_d0_slow_pullback`：`58.600160%`，相对 selected-delay qband 低 `-2.470757pp`；
  是本轮固定 policy 最优，但需要 live 能区分/执行 `slow -> pullback`。
- `fixed_d5_all_non_skip`：`53.053083%`，相对 selected-delay qband 低 `-8.017834pp`。

同一 audit 也用 slow-aware live artifact `data/pretouch_model.json` 对这批 canonical lead
events 做了 Go-compatible 推理复算。按全 68 个 lead rows 统计，research ledger timing
counts 为 `fast=60, slow=8`，当前 artifact timing counts 为 `fast=59, skip=6, slow=3`；
其中 62 个 speed-gate pass rows 上，artifact model + fixed D0 为 `56.342405%`，
相对 selected-delay qband 低 `-4.728512pp`；artifact model + `fast -> D0, slow -> pullback`
为 `57.982349%`，相对 selected-delay qband 低 `-3.088568pp`。理想 ledger policy
`fast_d0_slow_pullback` 为 `58.600160%`，说明当前 artifact 已找回 3/8 个 slow case，
但离 research ledger 的 slow 召回仍有 `0.617811pp` 可解释缺口。

live/testnet shadow 侧已补齐可部署 delay policy：只有 `pretouchShadowMode=testnet_shadow_collect`
且 live sandbox REST 语义成立时，`slow` lead event 不再立即开仓，而是进入
`selectedDelay=pullback` 的 `entry-delay-watch`。执行规则与 research pullback 语义对齐：
`touch_time+5s` 后第一笔 tick 作为 reference price，等待 `0.05 * ATR` 反向回调；
60 秒窗口内触发则以当前 tick 继续产生原 `Pretouch-Timing` entry intent，超时则 fallback
以当前 tick 入场。非 sandbox shadow 仍保持原即时行为，避免改变生产主路径。结论：
selected-delay 本身给 T2 lead 带来约 `+2.47pp` 到 `+4.73pp` 收益；本轮已经把可部署
`slow -> pullback` 执行缺口补到 testnet shadow，剩余主要缺口是 artifact 对 ledger slow
case 的召回不足，而不是 live engine 不会执行 slow delay。

可复算 artifact：

- `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_lead_combo_lead_same_close_trades.csv`
- `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_lead_combo_lead_adverse10_trades.csv`
- `research/entry_redesign/scripts/timing_probability_unified/t2_lead_quantity_band_sizing.py`
- `research/entry_redesign/scripts/output/timing_probability_unified/t2_lead_quantity_band_sizing_20260520/t2_lead_quantity_band_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/t2_lead_quantity_band_sizing_20260520/t2_lead_quantity_band_summary.json`
- `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_lead_exact_exposure/lead_exact_adverse10_exposure_windows.csv`
- `research/entry_redesign/scripts/timing_probability_unified/t2_reentry_schedule_revalidation.py`
- `research/entry_redesign/scripts/output/timing_probability_unified/t2_reentry_schedule_revalidation_20260526/t2_reentry_schedule_revalidation_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/t2_reentry_schedule_revalidation_20260526/t2_reentry_schedule_revalidation_summary.json`
- `research/entry_redesign/scripts/timing_probability_unified/t2_delay_policy_alignment_audit.py`
- `research/entry_redesign/scripts/output/timing_probability_unified/t2_delay_policy_alignment_audit_20260527/t2_delay_policy_alignment_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/t2_delay_policy_alignment_audit_20260527/t2_delay_policy_alignment_summary.json`
- `research/entry_redesign/scripts/output/timing_probability_unified/breakout_structure_lead_expansion_combo_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_smart_stop_policy_sweep_20260521/t3_overlay_smart_stop_policy_sweep_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_deterministic_stop_gate_stability_20260521/t3_overlay_deterministic_stop_gate_stability_report.md`
- `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_deterministic_stop_gate_stability_20260521/t3_overlay_deterministic_stop_gate_summary.json`

2026-05-19 已补齐 lead exact exposure ledger：用同一 production-aligned replay 重建 selected
`DelayResult` 的 `entry_time` / `exit_time`，并与 compact adverse10 lead ledger 做 parity 校验。
结果为 62/62 trades 完全对齐，`selected_delay_mismatches=0`，`max_abs_weighted_pnl_diff=0`，
无缺失 entry/exit。Exact lead 持仓统计为 avg `1487.58s`、p50 `924.50s`、p90 `3748.80s`、
max `7199.00s`；因此后续 portfolio/exposure 口径不得再用 `entry_time + 2h` 的近似窗口作为主结论。

T3 overlay 增强的 exact-lead portfolio sensitivity 结果保存在：

- `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_lead_portfolio_sensitivity_exact_lead/`

在 exact lead windows 下，capacity `1.6` 已经不触发缩放，observed peak active notional 为 `1.600`。
`scale_to_available` / capacity `1.6`：0bp 为 `34.374280%`，10bp 为 `27.823672%`，
15bp 为 `24.548369%`，20bp 为 `21.273065%`。因此增强方向的当前边界是：
`10-15bp` extra overlay round-trip slippage 仍可接受，`20bp` 时 overlay leg 转负且组合不再跑赢
`base_lead_adverse10_exact`，应作为 kill-stress。

### 2026-05-22 T2 shadow candidate sweep / 单腿优化推进

本轮只推进 ETHUSDT 1h pretouch T2 standalone 降仓规则搜索，不改变当前
testnet shadow bundle，不把 T3 deterministic stop-gate 或 PR447 overlay 收益作为 T2 pass 依据。

- Runner:
  `research/entry_redesign/scripts/timing_probability_unified/t2_shadow_candidate_sweep.py`
- Output:
  `research/entry_redesign/scripts/output/timing_probability_unified/t2_shadow_candidate_sweep_20260522/t2_shadow_candidate_sweep_report.md`
- Control:
  `base_current`，scenario 固定为 `next_adverse_xslip10bps`。
- Input:
  两份 frozen current-lead audit；为对齐 clean T2 base `50.434734%`，只用
  `selector=eff0817385_prevclose_le_0p969489` + `action_policy=buf020` 携带
  `baseline_pnl_pct`，不把 CSV 里的 `selected` 布尔列作为二次过滤标签。
- Allowed features:
  `rf_probability`、`speed_300s_atr`、`eff_300s`、`touch_extension_atr`、
  `prev1_close_pos_side`、`prev1_range_atr`、`side`。
- Sizing action:
  只允许 downsize，scale ladder 为 `1.0/0.75/0.50/0.25/0.0`。
- Anti-leak:
  每个 forward month 只能用 prior months 选阈值规则；不使用 T3 / `combo_pnl_pct`、
  不使用月度收益标签、不使用 future month。

Walk-forward 结果：

| Candidate | 2022-07..2024-12 delta | 2025-06..2026-04 delta | All delta | Avg scale | Neg months | Max DD | 结论 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| `static_quality_downsize` | `+2.990405pp` | `-2.914431pp` | `+0.075974pp` | `0.933219` | `16/19` | `-6.625610%` vs base `-7.409576%` | 长窗口能砍掉坏桶，但 canary 明显低于 base，不能 pass。 |
| `pure_loss_distilled` | `+0.397752pp` | `+0.000000pp` | `+0.397752pp` | `0.955479` | `16/19` | `-7.409576%` vs base `-7.409576%` | 已蒸馏为固定阈值规则，但 long delta 未达到 `+1.0pp`，all delta 也未达 shadow `+2.0pp`。 |

Frozen train-window 诊断进一步确认当前坏桶不适合直接固化成 shadow rule：

| Frozen rule | Train delta | Holdout delta | All delta | 结论 |
| --- | ---: | ---: | ---: | --- |
| `speed_300s_atr<=0.251125 => scale 0.25` | `+6.231559pp` | `-2.914431pp` | `+3.317128pp` | 2022-2024 train 很强，但 2025-2026 holdout 失败，不能 freeze。 |
| `touch_extension_atr<=-0.124973 & speed_300s_atr<=0.251125 => scale 0.00` | `+3.548808pp` | `-3.885908pp` | `-0.337100pp` | pure-loss 蒸馏规则 holdout 更差，不能 freeze。 |

当前结论：**没有 T2 单腿候选通过 testnet shadow gate**。`context_confirmed_downsize`
需要 4h/12h closed-bar context 列；当前 T2 audit 缺少 `ctx4h_side_return_atr` /
`ctx12h_side_return_atr`，本轮只能登记为 research watch unavailable。后续 T2 优化应先重建
closed-bar context 或扩展可解释特征，再重新做 standalone gate；在通过 T2 standalone gate 前，
不做 Go live metadata wiring。

### 2026-05-25 T2 static downsize 候选推进 / under review

本轮在 2026-05-22 runner 上补齐 closed-bar context 后，继续只推进 T2 standalone downsize。
它仍不改变当前 testnet shadow bundle，也不把 T3 overlay / combo 收益作为 pass 依据。

- Runner:
  `research/entry_redesign/scripts/timing_probability_unified/t2_shadow_candidate_sweep.py`
- Deep-dive helper:
  `research/entry_redesign/scripts/timing_probability_unified/t2_static_downsize_deep_dive.py`
- Output:
  `research/entry_redesign/scripts/output/timing_probability_unified/t2_shadow_candidate_sweep_20260525/t2_shadow_candidate_sweep_report.md`
- Candidate:
  `static_optimal_or_doc_a_ctx12h_range_le350_scale025_downsize`
- Rule:
  若未命中 profit-protection，`ctx12h_range_atr <= 3.500000`，并且命中以下任一风险桶，
  则把本次 T2 size 缩到 `0.25`：
  - `eff_300s >= 0.925057 && ctx12h_side_return_atr <= -0.282982`
  - `touch_extension_atr <= -0.112263 && ctx12h_range_atr >= 3.006207`
- Profit-protection:
  `ctx12h_side_return_atr >= 1.45` 或 `ctx4h_range_atr >= 2.36` 或
  `ctx12h_range_atr >= 3.98` 或 `rf_probability >= 0.965` 时，不降仓。

2026-05-25 rolling walk-forward 结果：

| Candidate | 2022-07..2024-12 delta | 2025-06..2026-04 delta | All delta | Avg scale | Selected | Shadow gate | 结论 |
| --- | ---: | ---: | ---: | ---: | ---: | --- | --- |
| `static_optimal_downsize` | `+3.318669pp` | `+0.626243pp` | `+3.944912pp` | `0.979452` | `3/146` | pass | 收益略高，但 canary lift 较弱。 |
| `static_optimal_or_doc_a_scale025_downsize` | `+2.007711pp` | `+1.718344pp` | `+3.726056pp` | `0.974315` | `5/146` | pass | 原 OR 候选，保留为对照。 |
| `static_optimal_or_doc_a_ctx12h_range_le350_scale025_downsize` | `+5.655933pp` | `+1.718344pp` | `+7.374277pp` | `0.958904` | `8/146` | pass | 新主候选；用圆整的 `ctx12h_range_atr <= 3.50` 限制训练期过度命中，恢复 2024-10..12 收益。 |

固定全周期 deep-dive 显示过滤版 `static_optimal OR doc_rule_a` + `ctx12h_range_atr <= 3.50`
的潜在空间为：all `+10.009754pp`、canary `+1.718344pp`、avg scale `0.933219`。正式
rolling runner 只允许 prior months 激活规则，因此 promotion 口径以
`t2_shadow_candidate_sweep_20260525` 为准。

Stability audit 显示 `ctx12h_range_atr <= 3.40/3.45/3.50` 形成同一收益平台，不是单点阈值；
在 `3.25..3.60` 区间内有 `8/19` 个阈值同时满足 all/canary/avg-scale/frozen-holdout 约束。
Frozen train-window 诊断显示过滤版不再是 base passthrough：train `+8.291409pp`、
holdout `+1.718344pp`、all `+10.009754pp`、holdout avg scale `0.956731`，读数为
`holdout non-negative; inspect before promotion`。

追加的 purged nested threshold selection 进一步确认收益不依赖固定 `3.50`：每个 forward month
只用更早月份训练，并空出 1 个 purge month；阈值取训练窗内达到 best delta `95%` 的平台中位数。
该选择器在 `28/40` 个 forward month 激活，自动选择阈值 `3.35..3.80`，all `+7.687331pp`、
long `+5.968986pp`、canary `+1.718344pp`、avg scale `0.953767`、selected `9/146`。因此当前状态从
`research_candidate_under_review` 提升为 `research_candidate_review_ready`；在完成人工 review、
事件级审计和必要的 live 可重建性确认之前，仍不做 Go live metadata wiring。

Selected-event reconstructability audit 已完成第一层确认：

- Script:
  `research/entry_redesign/scripts/timing_probability_unified/t2_downsize_selected_event_reconstructability.py`
- Output:
  `research/entry_redesign/scripts/output/timing_probability_unified/t2_downsize_selected_event_reconstructability_20260525/`
- Result:
  closed-bar context `8/8` 可重建，decision `8/8` 可重建，max feature abs diff
  `4.4408920985e-16`。
- Event branches:
  `static_optimal` `5` 次、`doc_rule_a` `2` 次、`static_optimal+doc_rule_a` `1` 次；
  所有 selected event 的 PP 均为 `False`。

这条证据只覆盖 4h/12h closed-bar context 与静态规则重放；`eff_300s`、`touch_extension_atr`
等 intrabar touch 特征仍依赖既有 event ledger。下一步若要进入 testnet shadow metadata wiring，
必须先确认 live decision metadata 已有同口径字段，或先补 shadow-only telemetry，不得直接改
production/live 默认行为。

Metadata readiness audit 进一步收窄实现边界：

- Script:
  `research/entry_redesign/scripts/timing_probability_unified/t2_downsize_live_metadata_readiness_audit.py`
- Output:
  `research/entry_redesign/scripts/output/timing_probability_unified/t2_downsize_live_metadata_readiness_20260525/`
- Static code read:
  `rf_probability`、`eff_300s`、`touch_extension_atr` 在 advance-plan 路径已有来源；
  `ctx12h_side_return_atr`、`ctx4h_range_atr`、`ctx12h_range_atr` 不是当前 Go live decision metadata 字段。
- Production read-only spot-check:
  `bktrader-ctl order list --limit 20 --json` 抽样显示，最近 ETH entry order 的
  `intent.metadata.pretouchEvent` / `executionProposal.metadata.pretouchEvent`
  含 `eff300s`、`touchExtensionAtr`、`speed300sAtr`、`preTouchSeconds`、`atr`；
  未见 4h/12h context 字段。

2026-05-25 根据人工推进意见，Go 实现边界从 shadow-only metadata 前移为
**testnet shadow submitted quantity downsize**：

- 只在 `pretouchShadowMode=testnet_shadow_collect` 下构造 `t2StaticDownsizeCandidate`。
- 只有 `pretouchShadowT2StaticDownsize=true` 且候选规则 selected 时，才进入 downsize 分支。
- 真实改 `suggestedQuantity` 前必须复用既有 testnet risk-on guard：`submittedRiskOnQuantityEnabled=true`。
  也就是 live 语义、账户 `sandbox=true`、`executionMode=rest`、depth/spread guard 通过后，才允许把
  `submittedQuantityAfterShadow` 乘以 `pretouchShadowT2StaticDownsizeScale`，默认 `0.25`。
- 非 sandbox / mainnet / guard 未通过时，只记录 would-downsize 与 block reason，不改变 submitted quantity、
  dispatchMode 或 production suggested quantity。

仓位放大口径：

- `2.5x` T3 overlay（`[0.50,0.25]`）已完成 actual lifecycle replay 和 exact-lead portfolio：
  0bp `37.262579%`、10bp `29.058168%`、15bp `24.955963%`、20bp `20.853758%`。
  相比 `2.0x` overlay，15bp 只多 `0.407594pp`，但 DD 从 `-2.105913%` 恶化到
  `-2.613241%`；20bp 反而低于 `2.0x`，所以它只能作为 aggressive research row。
- 旧一轮线性 lead-scale what-if 显示，`1.25x` lead scale + 当前 `2.0x` overlay、capacity `2.0`
  在 10bp/15bp/20bp 下分别为 `33.566584%`、`30.291281%`、`27.015977%`。
  但这是基于 exact lead ledger 的线性 notional/PnL 诊断，没有重放更大订单的盘口冲击；
  不得直接当作 live sizing 依据。
- 当时更干净的下一步不是继续拉大 T3 overlay，而是对 `1.25x` lead-scale + 2.0x overlay 做
  stricter order-book impact model。

2026-05-19 已补充 order-book impact proxy（非真实 depth replay）：

- 产物：
  `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_orderbook_impact_decision_report.md`
- 当前 `2.0x` overlay + `1.25x` lead scale + capacity `2.0`：
  - `moderate_top1p6_active0p5` / 15bp: `28.817057%`，DD `-2.121955%`
  - `strict_top1p2_active1p0` / 15bp: `25.554185%`，DD `-2.121955%`
  - `strict_top1p2_active1p0` / 20bp: `22.278881%`，低于 `base_lead_adverse10_exact`，按 kill-stress 失败
  - `severe_top1p0_active2p0` / 15bp: `20.291321%`，失败
- `2.5x` overlay 在 strict 15bp 下只有 `25.961779%`，比 `2.0x` overlay 多 `0.407594pp`，
  但 DD 恶化到 `-2.629283%`；strict 20bp 为 `21.859574%`，不通过 kill-stress。

因此该轮仓位提升的判断是：不得无条件放大默认仓位；只保留“depth/impact gate 通过时，把
lead scale 临时提升到 `1.25x`”作为下一轮 research candidate。后续 risk-appetite sweep 曾把
shadow target 更新为 `lead 1.5x / overlay 2.0x`；若盘口处于 severe profile，仓位提升仍必须阻断
并回落到当前 sizing。

Conditional lead-scale runner 已验证策略形态：

- 产物：
  `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_conditional_lead_scale_size2p0/`
- `strict_top1p2_active1p0` profile 下，gate `8bp` / overlay 15bp 只放大 38 笔、阻断 24 笔，
  result 为 `23.687991%`；gate `10bp` 放大 62/62 笔，result 为 `25.554185%`。
- `severe_top1p0_active2p0` / gate `10bp` / overlay 15bp 只放大 35 笔、阻断 27 笔，但 result
  仍只有 `20.482920%`，说明 thin-book 条件下不只是“不要放大”，而是整组 candidate 需要更强执行 gate。

结论：conditional scaling 是正确形态，但 gate 阈值对 8-10bp 非常敏感，不能用 proxy 直接定 live
参数。下一步必须用真实 depth 数据或生产 decision telemetry 校准。

2026-05-19 已补充生产 decision/order telemetry 校准（仍是 research-only，不保存原始生产 JSON）：

- 产物：
  `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_live_depth_calibration_20260519/`
- 数据源：
  `bktrader-ctl order list --limit 200 --json`，过滤 `ETHUSDT` /
  `strategy-version-bk-eth-pretouch-timing-v010` / `signalKind=entry` /
  `reduceOnly=false`，得到 6 笔 live testnet pretouch entry。
- 当前观测 entry quality：max spread `7.320544bp`、max book age `413.137042ms`、
  max source divergence `0.320167bp`、min top-depth coverage `9335.605263`、
  max actual adverse fill drift `5.204792bp`、p90 adverse drift `2.791609bp`。
- 在仅用当时 live pre-submit guard 重算的数量缩放矩阵里，`1.5x` lead scale shadow target 为
  `6/6` combined pass，min scaled top-depth coverage `6223.736842`，worst
  8bp slippage headroom `2.795208bp`。`2.0x/2.5x` 也因 testnet top-book coverage 极大而通过 guard，
  但当前 research 已证明 `2.0x` lead target 在 strict 15bp 下不如 `1.5x`。

这份 telemetry 的用途是校准 proxy 的方向，不是覆盖风险结论。它支持“`1.5x` lead scale 可以进入
testnet shadow 采样”，但样本只有 6 笔且来自 testnet；同时最新一笔 entry 已出现 `5.204792bp`
adverse fill drift，距离 live `8bp` slippage guard 只剩 `2.795208bp`。因此不得因为 top-depth coverage
看起来宽就无条件放大真实 submitted quantity，更不能把 `2.0x` overlay 推成 mainnet/default live event source。
本提交只允许它作为 guarded testnet shadow entry proposal。下一步应继续累计生产 entry
遥测或补历史真实 depth replay，再把 conditional gate 固化为“spread/book age/source divergence/top-depth
coverage/fill drift”联合判定。

2026-05-19 已把 sizing 放大结论机械化成 readiness gate：

- 产物：
  `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_sizing_readiness_gate_risk_appetite_20260519/`
- 当前状态：
  `research_continue_collect_live_depth`
- Gate 证据：
  当时 `lead 1.5x / overlay 2.0x` shadow target 的 live telemetry 为 `6/6` pass，
  min scaled top-depth coverage `6223.736842`，worst 8bp slippage headroom `2.795208bp`；
  strict 15bp proxy `28.970948%` 跑赢 `base_lead_adverse10_exact`，strict 20bp proxy
  `22.420341%` 保持 kill-stress 失败，severe 15bp proxy `21.231073%` 失败。
- 阻塞项：
  live entry 样本只有 `6` 笔，低于 readiness gate 的 `30` 笔 human-review 门槛。

因此当前不是 `live_candidate_ready_for_human_review`。后续除非样本数达到门槛且继续保持
`100%` live combined pass、worst slippage headroom `>=2bp`，同时 proxy 的 strict/severe 分离不变，
否则只能继续作为 research candidate。

2026-05-19 按更高风险偏好重新打开 sizing sweep：

- 产物：
  `research/entry_redesign/breakout_structure_risk_appetite_20260519.md`
- Harness 变化：
  `t3_overlay_orderbook_impact_sensitivity.py` 新增 `--overlay-scales`，可以联合扫描
  `lead_scale × overlay_scale × capacity × overlay slippage`。默认 `overlay_scale=1.0`，旧命令口径不变。
- Aggressive matrix:
  `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_aggressive_risk_appetite_sweep_20260519/`
- 结论：
  之前 harness 对 research 风险偏好确实偏保守，因为只系统性扫了 lead scale，没有把 overlay scale 和
  lead scale 联合放大。打开后，收益/回撤前沿变清楚：
  - `strict_top1p2_active1p0` / 10bp 下，`lead_scale=1.5`、`overlay_scale=2.0`、capacity `>=2.5`
    为 `35.521555%`，DD `-3.582801%`。
  - 同一行在 `strict` / 15bp 下为 `28.970948%`，DD `-4.179741%`。
  - `moderate` / 15bp 可以到 `38.856778%`，但依赖更乐观 impact profile。
  - `severe` / 15bp 最好只有 `22.222649%`，低于 `base_lead_adverse10_exact`，仍必须作为 thin-book kill stress。
- 当前 risk-on research row 改为：
  `lead_scale=1.5`、`overlay_scale=2.0`、capacity `>=2.5`。这不是 live 默认 sizing 候选；
  它只是下一步真实 depth / 生产 telemetry 校准前的进攻型 research candidate。
- Conditional risk-on harness:
  `t3_overlay_conditional_lead_scale.py` 已从 lead-only 条件放大扩成 lead / overlay 分腿条件放大，
  并支持多 capacity sweep。产物：
  `t3_overlay_conditional_risk_appetite_1p5x2p0_20260519/` 和
  `t3_overlay_conditional_risk_appetite_2p0x2p0_20260519/`。
- Conditional 结果：
  - `lead 1.5x / overlay 2.0x` 在 `strict` / 15bp 最优为 `28.970948%`，DD `-4.179741%`；
    在 `strict` / 10bp 为 `35.521555%`，DD `-3.582801%`。
  - `lead 2.0x / overlay 2.0x` 在 `strict` / 15bp 最优只有 `24.473835%`，DD `-4.288626%`，
    说明进攻档不应继续推高 lead target。
  - `severe` / 15bp 最优为 `21.231073%`，仍低于 `base_lead_adverse10_exact`，因此 severe/thin-book
    必须阻断 risk-on sizing。

Telemetry 累积刷新流程已固化：

```bash
bktrader-ctl order list --limit 500 --json | \
  PYTHONPATH=research:research/entry_redesign/scripts \
  python3 research/entry_redesign/scripts/timing_probability_unified/t3_overlay_live_depth_calibration.py \
    --orders-json - \
    --history-csv research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_live_depth_calibration_20260519/t3_overlay_live_depth_entry_samples.csv \
    --output-dir research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_live_depth_calibration_accumulated_YYYYMMDD \
    --source-note "bktrader-ctl order list --limit 500 --json + sanitized history"

PYTHONPATH=research:research/entry_redesign/scripts \
  python3 research/entry_redesign/scripts/timing_probability_unified/t3_overlay_sizing_readiness_gate.py \
    --live-summary research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_live_depth_calibration_accumulated_YYYYMMDD/t3_overlay_live_depth_calibration_summary.json \
    --output-dir research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_sizing_readiness_gate_accumulated_YYYYMMDD
```

2026-05-19 dry run 产物：

- `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_live_depth_calibration_accumulated_20260519/`
- `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_sizing_readiness_gate_accumulated_20260519/`

该 dry run 的 `current_entry_count=6`、`history_entry_count=6`、`deduped_entry_count=6`，证明同一
`order_id` 会去重，累计流程不会把旧订单重复计入样本。后续只归档脱敏后的
`t3_overlay_live_depth_entry_samples.csv` / summary / report，不归档原始 `bktrader-ctl` JSON。

### Testnet shadow 推进契约

当前结论是：**可以进入 testnet shadow risk-on 采样；不能进入 live candidate，也不能修改默认
sizing 或 mainnet 行为**。这一阶段的目标不是证明最终收益，而是用真实 testnet order/fill/depth
遥测验证 sizing-lift 的执行风险是否可控。

Shadow 分三层，任何 Go 实现都必须保持这三层状态可区分：

| 状态 | 允许行为 | 不允许行为 | 退出条件 |
| --- | --- | --- | --- |
| `research_candidate` | 离线回测、impact proxy、readiness gate | 写入 live 默认、改变模板默认仓位 | 形成明确 shadow contract |
| `testnet_shadow_collect` | `sandbox=true`，ETHUSDT 1h，允许显式 risk-on shadow 提交 `0.20..0.40 ETH` lead quantity，并允许 T3 overlay 生成独立 `0.20..0.40 ETH` testnet entry proposal，收集 decision/order/fill/depth telemetry | mainnet、全局默认 sizing 放大、跨品种推广、把 T3 overlay 当成 mainnet/live candidate、一次 decision 多订单派发 | live samples `>=30` 或触发 kill gate |
| `live_candidate_ready_for_human_review` | 只允许进入人工评审，不代表自动上线 | 自动切 mainnet、自动改默认 dispatch/sizing | 双人 review 后另开实现 PR |

#### 本次提交版本接入状态

本 research 分支提交的是“ETH pretouch timing 当前 live 版本 + research/shadow 证据包”，不是把所有
research 增强一次性接进 Go live。提交版本的边界如下：

| 模块 | 提交版本状态 | 说明 |
| --- | --- | --- |
| `breakout_shape_tolerance_bps` | **已接入当前 ETH pretouch live** | `PretouchEventDetector` 使用 `StructureToleranceBps` 判断 `prev_high_2` / `prev_low_2` 是否 ready；默认来自 `defaultT2BreakoutShapeToleranceBps=0.5`，session 参数可覆盖非负值。 |
| Breakout 容差语义 | **near-equal shape tolerance** | 当前 `0.5` bps 是 original_t2 结构容差：long 需要 `prev_high_2 >= prev_high_1 * (1 - 0.5/10000)`，short 需要 `prev_low_2 <= prev_low_1 * (1 + 0.5/10000)`；breakout level 仍锁定 `prev_high_2` / `prev_low_2`，不额外放宽触价门槛。 |
| T3 结构 / `t3_swing` | **已接入 testnet direct event source** | Go live engine 在 original_t2 未触发时检测 T3 swing：relaxed 口径为 long `prev_high_3 > prev_high_2 && prev_high_3 > prev_high_1`，short 对称；仍由兼容参数 `testnet_shadow_collect` 启用，不改变 mainnet production lead。 |
| `2.0x T3 overlay` | **已接入 testnet direct 真实 entry proposal** | T3 overlay 使用 `pretouchShadowOverlayBaseShare=0.40`、`pretouchShadowOverlayScale=2.0`，默认 base quantity `0.100` 时 initial overlay order 为 `0.080` ETH；仅当 live 语义、`sandbox=true`、`executionMode=rest`、speed/depth/spread guard 通过时生成 `entry-t3-overlay` proposal。scale/share 和最终 submitted quantity 均有硬上限。 |
| T3 RF/cost quality sizing | **已接入 testnet direct artifact + absolute quantity band** | `data/pretouch_t3_overlay_rf_model.json` 使用 T3 专用 RF 估计事件胜率，再把 overlay 直接映射到 `0.20..0.40` ETH absolute quantity band；默认 `0.080` ETH fixed overlay 等价于 `2.5..5.0x` quality multiplier。模型缺失或 feature build failed 时默认阻断 overlay submit 并写 `model_missing` / `feature_build_failed` metadata，避免污染 q020/q040 candidate 样本；只有显式 `pretouchShadowOverlayQualityFallbackSubmit=true` 才允许 fixed overlay fallback submit。 |
| T2/lead quantity band | **已接入 testnet direct 真实提交数量** | testnet direct 默认启用 risk-on lead sizing，可用 `pretouchShadowSubmitRiskOnQuantity=false` 显式关闭；只有 live 语义、账户 binding `sandbox=true`、`executionMode=rest` 且 depth/spread guard 通过时，`suggestedQuantity` 才从 production sizing 映射到 `0.20..0.40 ETH`。旧 `1.5x` lead scale 仍保留为无 band 参数时的兼容 fallback。 |
| T2 static downsize | **保留代码路径但模板默认关闭** | `static_optimal_or_doc_a_ctx12h_range_le350_scale025_downsize` 仍可显式开启；当前 testnet direct 模板默认 `pretouchShadowT2StaticDownsize=false`，不再把 `0.20..0.40 ETH` 提交量缩小到 25%。 |
| 当前 submitted sizing | **testnet direct 全量 quantity band** | Lead base `productionSuggestedQuantity = pretouchBaseOrderQuantity * pretouchBaseShare * clip(rf_probability * 2, 0, 2) * costPenalty` 仍先按 production RF/cost 生成；通过 risk-on guard 后再按 `productionSuggestedQuantity / maxProductionQuantity` 的质量分数映射到 `0.20..0.40 ETH`，默认不再 downsize。T3 overlay 为独立 `entry-t3-overlay` intent，T3 RF/cost quality 直接映射到 `0.20..0.40 ETH`，并受 `pretouchShadowMaxSubmittedQuantity=0.40` 限制。 |
| Testnet direct | **允许进入全量 testnet 执行阶段** | 用真实 `0.20..0.40 ETH` lead quantity 和 T3 overlay `0.20..0.40 ETH` entry proposal 采集 testnet decision/order/fill/depth telemetry；mainnet 仍被 sandbox/account guard 阻断。 |

#### Testnet Direct 策略细节

| 项 | Shadow 口径 |
| --- | --- |
| Symbol / timeframe | `ETHUSDT` / `1h` |
| Event source | Lead 仍为 production-aligned `pretouch_small_pullback_rf_q50_speed300_ge_q10_touch30m_eff300le1`；overlay 仅为 testnet shadow `t3_swing` |
| Breakout shape | Lead 使用 live `breakout_shape_tolerance_bps=0.5` near-equal original_t2 结构容差；T3 overlay 使用严格三根 bar swing 结构，不放宽 near-equal tolerance |
| Pretouch window | Lead 与 overlay 均默认 `pre_touch_seconds <= 1800` |
| Quality gates | Lead: `eff_300s <= 1.0`、`speed_300s_atr >= 0.228106`、`cost_q50 <= 0.116865` 后按 `pretouchCostQ50Penalty=0.50` 惩罚；T3 overlay: `abs(speed_300s_atr) >= 0.35`、`eff_300s <= 1.0` |
| Model | Lead 使用 `data/pretouch_model.json` / `20260515_v1` 的 Go-native DT3 + RF；T3 overlay 使用 `data/pretouch_t3_overlay_rf_model.json` / `20260520_t3_overlay_rf_cost_v1` 只做 quality sizing，不做 timing skip。 |
| Model features | Lead: `roundtrip_cost_atr`、`prev1_range_atr`、`prev1_close_pos_side`、`level_to_signal_open_atr`、`touch_extension_atr`、`speed_300s_atr`、`eff_300s`、`pre_touch_seconds`；T3: `rf_probability`、`speed_300s_abs`、`eff_300s`、`touch_extension_abs`、`pre_touch_seconds`、`roundtrip_cost_atr`、`side_is_short` |
| Entry action | Lead: `timing=advance-plan` 且 RF/cost sizing 有效时才生成 proposal；`skip`、未知 regime、模型缺失均 wait。T3 overlay: original_t2 未触发、T3 swing 触达且 overlay guard 通过时生成 `entry-t3-overlay` proposal。 |
| Execution profile | 只允许 `sandbox=true`、`executionMode=rest` 的 Binance Futures testnet；若要观测真实 fill drift，需要显式 `auto-dispatch` 的 testnet shadow session |
| Submitted size | Lead: risk-on 未显式关闭且 shadow pre-submit guard 通过时，按 production/max-production quality score 映射到 `pretouchShadowLeadQuantityMinQuantity..MaxQuantity`，默认 `0.20..0.40 ETH`；否则回落到 production sizing。T3 overlay: 未显式关闭且 overlay pre-submit guard 通过时，T3 RF/cost 将 probability 映射到 `pretouchShadowOverlayQualityMinQuantity..MaxQuantity`，默认 `0.20..0.40 ETH`。两条路径最终 shadow submitted quantity 都受 `pretouchShadowMaxSubmittedQuantity` 限制。 |
| Shadow hard caps | `pretouchShadowLeadQuantityMaxQuantity <= 0.40`、`pretouchShadowLeadScale <= 1.5`、`pretouchShadowOverlayScale <= 2.0`、`pretouchShadowOverlayBaseShare <= 0.40`、`pretouchShadowOverlayQualityMaxMultiplier <= 5.0`、`pretouchShadowOverlayQualityMaxQuantity <= 0.40`、`pretouchShadowMaxSubmittedQuantity <= 0.40`；`testnet_shadow_collect` 下 `pretouchBaseOrderQuantity` 也会被 cap 到 max submitted quantity。 |
| Shadow metadata | Metadata 记录 production quantity、submitted before/after shadow、lead quantity band score/min/max、legacy `1.5x` lead quantity、T3 overlay base/share/scale/quality multiplier/submitted quantity、max submitted quantity、是否 capped、depth/spread guard 和 block reason |
| Dispatch boundary | 模板不硬编码 `dispatchMode`；调用方/前端显式传入，系统空值兜底仍为 `manual-review` |

当前 live sizing 仍以 intent 为唯一入口。base production sizing 为：

```text
suggestedQuantity =
  pretouchBaseOrderQuantity
  * pretouchBaseShare
  * clip(rf_probability * 2, 0, 2)
  * costPenalty
```

其中 `pretouchBaseShare=0.80`，`positionSizingMode=intent_quantity`。Testnet direct risk-on 阶段在
`sandbox=true`、`executionMode=rest`、risk-on 未显式关闭、depth/spread guard
同时满足时，实际提交的 `suggestedQuantity` 会把 production quantity 相对 theoretical max
`pretouchBaseOrderQuantity * pretouchBaseShare * 2.0` 的质量分数映射到 `0.20..0.40 ETH`。若任一条件不满足，metadata
会写入 `submittedRiskOnQuantityBlockReason`，并保持 production sizing；`defaultOrderQuantity` 不得覆盖
intent quantity。`pretouchShadowLeadQuantityBandSizing=true` 只在 testnet direct guard 通过后生效；
旧 `pretouchShadowLeadScale=1.5` 仍保留为未带 quantity-band 参数的兼容 fallback。testnet path 的最终
submitted quantity 被 `pretouchShadowMaxSubmittedQuantity` 硬限制，模板默认值为 `0.40` ETH。
是否提交仍由 sandbox / REST / depth / spread guard 决定。

T3 overlay 是单独的 testnet direct event source，不把 lead RF probability 直接当仓位；它会把
lead RF probability 作为 T3 专用 RF model 的一个输入特征。默认参数为
`pretouchShadowOverlayBaseShare=0.40`、`pretouchShadowOverlayScale=2.0`、
`pretouchShadowOverlaySpeedThreshold=0.35`、`pretouchShadowOverlayQualitySizing=true`、
`pretouchShadowOverlayQualityMinMultiplier=2.50`、`pretouchShadowOverlayQualityMaxMultiplier=5.00`、
`pretouchShadowOverlayQualityMinQuantity=0.20`、`pretouchShadowOverlayQualityMaxQuantity=0.40`。
所以模板默认 `pretouchBaseOrderQuantity=0.100` 时，overlay 固定基准仍为 `0.080` ETH，但 RF/cost
quality 后 intent quantity 直接使用 `0.20..0.40` ETH absolute band。若 `pretouchShadowSubmitOverlayOrder=false`、
非 live 语义、非 sandbox、非 REST、depth/spread guard 失败，策略只记录
`pretouchShadowOverlaySizing.submittedOverlayOrderBlockReason`，不生成 live intent。T3 overlay 的
`signalBarTradeLimitKey` 追加 `entry-t3-overlay` 后缀，避免与 lead entry 共用同一 trade-limit identity；
基础 `signalBarStateKey` 仍保持 `symbol|timeframe|barStart`，用于窗口状态解析。
若 T3 RF artifact 缺失或 lead RF probability 特征不可用，T3 overlay 默认阻断提交，在
`pretouchShadowOverlayQualitySizing.status` 写入 `model_missing` / `feature_build_failed`，并在
`submittedOverlayOrderBlockReason` 写入 `overlay_quality_model_missing` /
`overlay_quality_feature_build_failed`。只有显式设置 `pretouchShadowOverlayQualityFallbackSubmit=true`
时，才允许同一 guard 下提交 fixed `0.080` ETH overlay fallback；默认模板保持 false，避免同一
candidateID 混入 fixed overlay 与 RF/cost quantity-band 样本。历史参数名仍沿用 `pretouchShadow*`，
但当前 testnet 候选语义是直接真实提交 `0.20..0.40` ETH，不再默认 downsize 或 shadow-only。

#### T3 deterministic stop-gate lifecycle overlay

2026-05-29 relaxed event pool 复算后，deterministic stop-gate 在 q020-q040 quantity-band 上的
overlay 为 `194.323156%`，相对 relaxed q020-q040 baseline `91.360615%` 是 `+102.962541pp`；
对应 lead q020-q040 + overlay 为 `255.394073%`。artifact:
`research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_deterministic_stop_gate_relaxed_prev3_dominates_20260529/t3_overlay_deterministic_stop_gate_summary.json`。

2026-05-21 的 T3 lifecycle research lead 不是新增 entry event source，也不是替换 T3 RF/cost
quality sizing。它只是在 T3 overlay event 已经入场后，用一个可解释 deterministic selector 决定是否切换
出场生命周期：

```text
abs(speed_300s_atr) >= 0.65
eff_300s >= 0.85
250 <= pre_touch_seconds <= 900
abs(touch_extension_atr) <= 0.40
```

组合方式：

- Gate 未命中：T3 overlay 继续走 PR447 lifecycle baseline。
- Gate 命中：只对该 T3 event 切换到 `delay_trailing_updates_79m + hard_stop_atr_3.0`。
- `delay_trailing_updates_79m` 的含义是延迟 trailing-stop 更新，不是阻断 hard stop；`hard_stop_atr_3.0`
  从入场后就作为 catastrophic / hard stop 生效。
- 不把 research 文件名里的 `min_hold_sl_60m` 直译成 live 里的全 SL min-hold。实盘对齐时必须保留 hard stop
  可立即触发，只延后 trailing ratchet。
- RF 1.0 gate、ExtraTrees gate 和 dynamic hard-stop schedule 都保留为 research 对照，不作为当前 live 对齐 lead。

当前证据：

| Policy | Overlay | Worst Month | Neg Months | Max DD | Hard3 Events | Worst Trade | P90 Hold | Worst MAE |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| PR447 lifecycle baseline | `45.639101%` | `-3.088880%` | `3` | `-4.602568%` | `0` | `-0.557532%` | `4359.80s` | `-276.224267bp` |
| RF 1.0 + selected hard-stop action | `52.291662%` | `-3.088880%` | `3` | `-4.602568%` | `5` | `-0.557532%` | `4740.00s` | `-276.224267bp` |
| Deterministic gate + selected hard-stop action | `55.460787%` | `-3.088880%` | `2` | `-4.602568%` | `5` | `-0.557532%` | `4755.60s` | `-276.224267bp` |

Stability checks:

- 49 条 threshold-neighborhood rules 中，31 条在保持 PR447 headline DD / worst-trade / MAE 的同时跑赢 PR447。
- Chronological split 仍双边改善：early `+5.382401pp`，late `+4.439285pp`。
- Leave-one-month-out 全部跑赢 PR447；最弱 delta 是 drop 2026-02 后仍 `+5.709936pp`。
- Dynamic hard-stop 当前拒绝：全局应用会牺牲太多 realized return；作为 gated replacement 虽略增 overlay
  到 `55.866415%`，但 worst trade 从 `-0.557532%` 恶化到 `-2.249838%`。

因此当前 live 对齐前的 research lead 是 **deterministic selector + selected hard-stop action**：
selector 是 deterministic gate，action 是 `delay_trailing_updates_79m + hard_stop_atr_3.0`。

#### 预期收益与风险预算

这些收益数字是 **research / proxy 预期**，不是 testnet shadow 实盘收益承诺：

| 组合 | 0bp | 10bp | 15bp | 20bp | 结论 |
| --- | ---: | ---: | ---: | ---: | --- |
| Base lead same-close 上界 | `30.217222%` | - | - | - | 只作无额外成交压力上界 |
| Base lead exact adverse10 | - | `22.971648%` | - | - | 保守主基线 |
| 2.0x T3 overlay + exact lead | `34.374280%` | `27.823672%` | `24.548369%` | `21.273065%` | 10-15bp 仍有 lift，20bp kill-stress 失败 |
| T3 RF/cost 0.75-1.25x overlay + exact lead | - | `35.945786%` | - | - | 历史小 band 对照；T3 专用 walk-forward RF/cost sizing 比 fixed overlay `+1.571506pp`，但数量只有 `0.06..0.10` ETH，已不是当前 risk-on shadow target。 |
| T3 RF/cost 0.20-0.40 ETH overlay + exact lead | - | `68.610750%` | - | - | 用户确认可承受更大回撤后的 aggressive shadow sizing；overlay leg `45.639102%`，delta vs fixed `+34.236470pp`，worst month `-3.088880%`，DD `-4.602568%`。 |
| T3 deterministic stop gate + exact lead | - | `78.432435%` | - | - | 在 PR447 T3 RF/cost quantity-band 基础上，只替换 deterministic gate 命中的 5 个 T3 event lifecycle；相对 PR447 `68.610750%` lift `+9.821686pp`。 |
| T2 lead 0.20-0.40 ETH quantity-band | - | `61.070916%` | - | - | 正式 T2 quantity-band adverse10 线性 notional 口径；相对 base `+38.099268pp`，相对旧 `lead 1.5x` `+26.613444pp`。 |
| T2 lead 0.20-0.40 + T3 RF/cost 0.20-0.40 | - | `106.710018%` | - | - | PR447 risk-on shadow headline；按月 additive bundle，worst month `-1.464655%`，event-order DD `-4.682954%`。 |
| T2 lead 0.20-0.40 + relaxed T3 deterministic stop gate | - | `255.394073%` | - | - | 当前 relaxed research headline；T3 overlay `194.323156%`，相对 relaxed q020-q040 baseline lift `+102.962541pp`，overlay worst month `-3.357137%`、negative month `1`。 |
| 1.5x lead scale + 2.0x overlay strict impact proxy | - | `35.521555%` | `28.970948%` | `22.420341%` | 上一轮 lead-scale reference；15bp 跑赢基线，20bp 回到基线附近；当前只作历史连续性对照。 |
| 2.0x lead scale + 2.0x overlay strict impact proxy | - | `31.024442%` | `24.473835%` | `17.923227%` | 不如 1.5x，不能继续推高 lead target |
| Severe thin-book proxy (`1.5x/2.0x`) | - | `27.781680%` | `21.231073%` | `14.680465%` | severe 15/20bp 必须阻断放大 |

Testnet direct 阶段的目标收益区间可以写成：

- 主候选不是追求 base lead 的 `30%+` same-close，而是在 adverse10 / quantity-band 线性 notional
  口径下确认风险偏好放大后仍保留足够收益缓冲。
- T2/lead `0.20..0.40 ETH` quantity-band 的正式回测为 `61.070916%`，相对
  `base_lead_adverse10_exact=22.971648%` 提升 `+38.099268pp`，相对旧 `lead 1.5x`
  提升 `+26.613444pp`；这是进入 testnet direct risk-on 执行的主依据。
- T3 overlay 走 `0.20..0.40 ETH` absolute quantity band 的 walk-forward evidence 为
  `base_lead_adverse10_exact + overlay = 68.610750%`，但 worst month 扩大到 `-3.088880%`、
  DD 扩大到 `-4.602568%`。
- Relaxed deterministic stop-gate lifecycle overlay 把 T3 q020-q040 overlay 从 `91.360615%` 提升到
  `194.323156%`，selector 命中 24 个 active events，lift `+102.962541pp`。
- 当前 combined headline 为 `lead_q020_q040_plus_t3_relaxed_deterministic_stop_gate = 255.394073%`。
  T3 overlay worst month `-3.357137%`、negative month `1`、max DD `-7.827585%`。它仍是线性 notional
  口径，没有额外建模更大 submitted quantity 的 slippage/depth degradation，因此只适合
  testnet direct 验真，不得跳过 fill/depth telemetry 直接成为 mainnet candidate。
- 若真实 fill/depth 校准后 15bp strict proxy 回落到 `base_lead_adverse10_exact` 以下，或者 20bp /
  severe profile 不能作为有效 kill-stress，该 sizing-lift 路线应停止。

#### Testnet Direct readiness gate

进入 testnet direct 的 gate：

| Gate | 当前状态 | 结论 |
| --- | --- | --- |
| Research lead 身份已锁定 | base event/model + lead `0.20..0.40 ETH` quantity band + T3 overlay `0.20..0.40 ETH` RF/cost quality artifact + testnet direct guard 已写入本文档 | pass |
| 保守基线可复算 | `base_lead_adverse10_exact=22.971648%`，62 trades，0 negative months | pass |
| Quantity-band formal report 有 lift | T2/lead qband `61.070916%` > baseline；PR447 bundle `106.710018%` > `base lead + T3 qband` | pass |
| Deterministic lifecycle gate 有 lift | relaxed T3 overlay `194.323156%` > relaxed q020-q040 baseline `91.360615%`，lift `+102.962541pp` | pass |
| Kill-stress 没被误通过 | severe 15bp `21.231073%` fail，strict 20bp `22.420341%` 不再提供足够 lift | pass |
| Live telemetry 方向 | 旧 `1.5x` 样本 `6/6` combined pass；`0.20..0.40 ETH` qband 需要重新累计 | pass for testnet direct, blocks mainnet candidate |
| Live telemetry 样本数 | `6 < 30` | blocks live candidate |

进入 `live_candidate_ready_for_human_review` 的 gate：

1. 累计 live/testnet pretouch entry samples `>=30`，且按 `order_id` 去重。
2. `0.20..0.40 ETH` shadow-computed quantity 的 combined pass ratio 必须保持 `100%`。
3. Worst `8bp` slippage headroom 必须 `>=2bp`；若出现 `adverse_fill_drift_bps >= 8`，直接 kill。
4. Strict 15bp proxy 继续高于 `base_lead_adverse10_exact`；strict 20bp 和 severe 15bp 继续作为 kill-stress 失败。
5. telemetry 中不得出现 lead `no_model_loaded`、T3 `model_missing` / `feature_build_failed` fallback、fixed default 覆盖 intent quantity、非 sandbox 下单。
6. 至少完成一次 `bktrader-ctl order list --json` 累积刷新和 readiness gate 复算，并归档脱敏 CSV / summary / report。

Kill gate：

- 任一 testnet shadow order 的 `sandbox=false`，立即停止本路线并排查 session/template。
- `suggestedQuantity` 缺失或被 `defaultOrderQuantity` 覆盖，停止推进，先修 sizing contract。
- `source_divergence_bps`、book age 或 spread 触发 live pre-submit guard 却仍生成可执行 proposal，停止推进。
- 样本达到 30 后 combined pass ratio 低于 `100%`，或 worst slippage headroom 低于 `2bp`，不进入 live candidate。

#### Go 实现边界

当前阶段实现 testnet shadow risk-on lead sizing 和 T3 overlay entry proposal，不实现 mainnet sizing 放大：

- 增加或复用 metadata 字段记录 current submitted quantity、lead quantity band score/min/max、legacy shadow `1.5x` quantity、T3 overlay base/share/scale/quality multiplier/submitted quantity、scaled top-depth coverage、
  slippage headroom、spread/book-age/source-divergence、submitted before/after shadow 和 block reason。
- 仅当 risk-on 未显式关闭、live 语义、账户 binding `sandbox=true`、`executionMode=rest`、
  shadow pre-submit guard 通过时，submitted proposal quantity 才进入 `0.20..0.40 ETH` lead quantity band；否则等于当前 production sizing。
- T3 overlay 只在 original_t2 未触发时检测；通过 `pretouchShadowSubmitOverlayOrder=false` 可显式关闭。T3 RF/cost quality sizing 可用 `pretouchShadowOverlayQualitySizing=false` 显式关闭。它不会在同一次 decision 里派发第二张订单，也不修改 lead 的 production semantics；overlay 使用独立 trade-limit key suffix 但保留同一基础 signal bar state key。
- T3 overlay shadow 当前只接 initial entry proposal；research 中的 T3 reentry schedule 后续腿仍未提升为 Go live 行为。
  T3 deterministic stop-gate lifecycle overlay 是下一步 live 对齐候选，但实现时必须保持 selector/action 分层：
  deterministic gate 只选择 event，selected action 只延迟 trailing updates 并保留 hard stop 立即可触发。
- 保持 `sandbox=true` testnet 范围；不改 mainnet、不改全局默认 `dispatchMode`、不改 BTC 策略语义。
- 若需要 `auto-dispatch` 采集真实 fill drift，必须由 session 创建参数显式指定；模板或 runtime 不得静默升级。
- 实现 PR 必须覆盖 `no_model_loaded`、fallback sizing、intent quantity 不被 fixed quantity 覆盖、sandbox
  阻断、thin-book shadow 阻断等失败路径。

重要边界：

- 当前 live engine 没有月度 gate，也不会按月份判断是否允许交易。`strategy_engine_pretouch_timing.go`
  只做 symbol、pretouch detector、DT3 timing、RF probability、cost penalty 和 intent sizing；
  `pretouch_event_detector.go` 只做 original_t2 structure、`pre_touch_seconds <= 1800`、
  `eff_300s <= 1.0`、`speed_300s_atr >= 0.228106`、cost threshold、单 bar 去重等实时条件。
- `pretouch_trainer.go` 里的 `ForwardStart=2025-11-01` 只用于训练/验证切分，不是 live runtime gate。
- walk-forward/monthly gate 是 breakout 结构扩展研究里的校验或候选过滤口径，不属于当前 research lead runtime contract。
- `same_close` 不是 live 成交等价物；live template 使用 MARKET entry/exit、`book-aware-v1`、
  `executionEntryMaxSlippageBps=8` 和 order-book freshness/coverage/source-divergence gate。
  因此上线候选比较时，主结果继续以 adverse/slippage matrix 为准，`30.217222%` 只能作为无额外成交压力的上界参考。
- 旧 `20260515_unified_framework_decision_report.md` 中 `22.38%/15.17%` 使用的是当时 report spec
  `trail_start_r=0.9` 的手写决策口径；当前 base lead / shadow bundle 使用 live template 固化的
  `trail_start_r=1.5`、`max_hold_hours=2.0`，以上表和 rebuilt ledgers 为准。

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
| `pretouchShadowLeadQuantityBandSizing` | `true` in testnet shadow template |
| `pretouchShadowLeadQuantityMinQuantity` / `MaxQuantity` | `0.20` / `0.40` ETH |
| `pretouchShadowOverlayQualitySizing` | `true` in testnet shadow template |
| `pretouchShadowOverlayQualityFallbackSubmit` | `false` in testnet shadow template；只有显式打开时才允许 T3 quality model 缺失/feature failed 后提交 fixed overlay fallback |
| `pretouchShadowOverlayQualityMinMultiplier` / `MaxMultiplier` | `2.50` / `5.00`，等价于 fixed `0.080` ETH overlay 的 `0.20..0.40` ETH absolute quantity band |
| `pretouchShadowOverlayQualityMinQuantity` / `MaxQuantity` | `0.20` / `0.40` ETH |
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
  -> PretouchModelBundle (lead Go-native JSON model)
  -> PretouchModelBundle (T3 overlay RF/cost JSON model, testnet shadow only)
  -> StrategySignalDecision(action=advance-plan)
  -> deriveLiveSignalIntent
  -> book-aware-v1 execution proposal
```

当前实现文件：

- `internal/service/strategy_engine_pretouch_timing.go`
- `internal/service/pretouch_event_detector.go`
- `internal/service/pretouch_tree.go`
- `internal/service/pretouch_trainer.go`
- `internal/service/pretouch_t3_overlay_trainer.go`
- `internal/service/pretouch_model_scheduler.go`
- `cmd/pretouch-train/main.go`
- `internal/domain/pretouch_event.go`
- `data/pretouch_model.json`
- `data/pretouch_t3_overlay_rf_model.json`

## 模型加载与重训

live 进程默认加载 lead 模型 `data/pretouch_model.json`。部署环境可通过 `BK_PRETOUCH_MODEL_PATH` 覆盖路径。
T3 overlay quality model 默认加载 `data/pretouch_t3_overlay_rf_model.json`。部署环境可通过
`BK_PRETOUCH_T3_OVERLAY_MODEL_PATH` 覆盖路径。

`live-runner` / `monolith` 会启动 pretouch model scheduler：

- hot reload 默认开启：`BK_PRETOUCH_MODEL_HOT_RELOAD_ENABLED=true`，每
  `BK_PRETOUCH_MODEL_RELOAD_INTERVAL_SECONDS=30` 秒检查 lead / T3 artifact 的 size + mtime。
- retrain 默认开启：`BK_PRETOUCH_MODEL_RETRAIN_ENABLED=true`，每
  `BK_PRETOUCH_MODEL_RETRAIN_INTERVAL_SECONDS=86400` 秒尝试重训。
- lead 重训开关：`BK_PRETOUCH_LEAD_RETRAIN_ENABLED=true`，训练输入来自
  `BK_PRETOUCH_RETRAIN_EVENTS_CSV` 和 `BK_PRETOUCH_TIMING_LABELS_CSV`。timing labels
  还受 `BK_PRETOUCH_TIMING_LABELS_MAX_AGE_HOURS=48` freshness guard 约束；若 CSV
  不存在、为空或超过 freshness window，重训跳过并保留当前模型，避免 scheduler 每天拿旧
  research ledger 反复产出看似“新”的 artifact。
- T3 overlay 重训开关：`BK_PRETOUCH_T3_OVERLAY_RETRAIN_ENABLED=true`，训练输入来自
  `BK_PRETOUCH_T3_OVERLAY_RETRAIN_TRADES_CSV`，默认随 runtime image 携带
  `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_rf_cost_sizing_20260520/t3_overlay_rf_cost_base_trades.csv`。
- 重训写 artifact 使用 temp file + atomic rename；hot reload 只在 JSON 加载和 bundle 校验成功后替换内存模型。
- 已运行 session 不需要重建才能吃到新模型：`EvaluateSignal` 每次评估从 engine 的 atomic model pointer 读取当前模型快照。坏文件、半写入文件或校验失败不会覆盖上一版可用模型。

Lead 模型不可用、JSON 不合法、feature 数量不匹配或 tree feature index 越界时，lead 策略视为
`no_model_loaded`，不入场，也不会退化为 fixed sizing。T3 overlay quality 模型不可用或 feature build
failed 时，默认阻断 overlay submit，并在 metadata 标记 `model_missing` / `feature_build_failed`；
只有显式 `pretouchShadowOverlayQualityFallbackSubmit=true` 才允许 fixed overlay fallback submit。

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

T3 overlay artifact 生成入口：

```bash
PYTHONPATH=research:research/entry_redesign/scripts \
  python -m timing_probability_unified.t3_overlay_rf_model_export --replay-if-missing
```

该脚本优先读取 `t3_overlay_rf_cost_base_trades.csv`，缺失时重放 T3 overlay lifecycle，训练 240-tree T3 RF，并导出
`data/pretouch_t3_overlay_rf_model.json`。当前 artifact 的 sklearn-vs-Go JSON export 概率校验
`max_abs_diff=6.66e-16`。

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

### Live Exit Contract

2026-05-19 对齐点：ETH pretouch live 模板里的 V4 exit 参数需要进入 Go live exit path，
不能只停留在 research replay 文档里。

- 初始止损：live 继续使用 `stop_loss_atr`；当参数只提供 research 命名的 `initial_stop_atr`
  时，归一化会把它作为 `stop_loss_atr` fallback。
- Breakeven：当浮盈达到 `breakeven_at_r × initial risk` 后，SL 会推进到 entry 加/减
  `cost_lock_bps`，用于锁住成本。该判断使用 HWM/LWM 计算 MFE R 倍数，所以属于不可逆
  ratchet：一旦触发，不会因为价格回撤而撤销。
- Trailing：当同时提供 `trail_start_r` 和 `trail_buffer_atr` 时，live 使用 research 的 R-multiple
  语义，即浮盈达到 `trail_start_r × initial risk` 才启用 trailing，止损价为 HWM/LWM 回撤
  `trail_buffer_atr × ATR`。这会优先于 legacy `trailing_stop_atr + delayed_trailing_activation_atr`
  的 ATR-only 语义；没有 V4 参数时 legacy 行为保持不变。V4 exit contract 存在时，live safe
  defaults 不再注入 legacy trailing 默认值，避免 dashboard/审计展示与实际生效参数不一致。
- `min_stop_bps`、`stop_buffer_atr`、`stop_cap_atr` 当前只在参数归一化中保留，尚未进入 live
  adaptive initial-stop 计算；这属于分阶段实施，后续若接入需要单独补 entry signal high/low 与 stop cap
  parity 测试。
- `max_hold_hours` 当前在模板、参数归一化和 research artifact 中保留，但 live runtime 暂未新增
  定时强平路径；如果要把 max-hold 也接入实盘，需要单独 PR 处理 entry time 来源、重启恢复和
  reduce-only MARKET exit 审计。

2026-05-21 T3 deterministic stop-gate 对齐点：

- 这不是新的 entry gate；entry 仍由 T3 swing + T3 RF/cost quality sizing 决定。
- deterministic gate 只在 T3 overlay position 的 exit profile 选择阶段使用，输入为 live 已有/应记录的
  `speed_300s_atr`、`eff_300s`、`pre_touch_seconds`、`touch_extension_atr`。
- Gate 未命中时，exit profile 不变，继续使用 PR447 lifecycle baseline。
- Gate 命中时，exit profile 切换为 `hard_stop_atr=3.0` 且 `min_hold_seconds_before_trailing_sl=4740`。
  这个参数名应表达“只延迟 trailing-stop 更新”，不得复用会阻断 hard stop 的 `min_hold_seconds_before_sl`
  语义。
- Hard stop 必须从 position open 起立即有效；任何 live 实现都不得把 79m 解释成全止损 min-hold。
- Metadata/trace 需要记录 deterministic gate 的 pass/fail、四个 feature 值、阈值、selected exit profile，
  以及是否 fallback 到 PR447 baseline，便于 testnet/prod 逐单审计。

## Breakout 结构展开基准

当前生产的 `breakout_shape_tolerance_bps=0.5` 是 original_t2 结构容差，不是更高的开仓门槛。它只允许
`prev_high_2` / `prev_low_2` 与上一根 closed bar 在 0.5bps 内近似持平；breakout level 仍锁定
`prev_high_2` / `prev_low_2`，当前价格必须真实触达该 level：

- Long ready: `prev_high_2 >= prev_high_1 * (1 - tolerance_bps / 10000)`
- Short ready: `prev_low_2 <= prev_low_1 * (1 + tolerance_bps / 10000)`

2026-05-25 生产排查发现 live `PretouchEventDetector` 曾把该容差反向实现成 restrictive separation；
修复后它与 `strategy_registry.go` / replay 里的 T2 helper 语义保持一致。

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
- Lead `0.20..0.40 ETH` quantity band 只允许在 testnet shadow risk-on guard 通过时真实提交；mainnet 和非 sandbox 必须保持 production sizing。
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
