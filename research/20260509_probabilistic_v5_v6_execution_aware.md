# Probabilistic V5/V6 执行感知概率模型推进记录

范围：仅限 `research`。本轮没有修改 `live` / `internal`，也不把任何结果解释为实盘候选。

## 本轮目标

用户明确要求概率模型不能只沿着原 V3/Markov 阈值做小修小补；收益在 `1%` 附近反复横跳没有意义，实盘候选至少要看到 `10%~20%` 级别的回测潜力。因此本轮重点改成：

- 概率模型族扩展：`logistic`、`random_forest`、`extra_trees`、`gradient_boosting`、`svm_rbf`。
- Markov 不只做二元质量阈值：加入 `5/15/30/60s` 小窗口 order-flow state LLR / probability 特征。
- 质量阈值联动仓位：输出 `model_notional_share`，用 EV、probability、Markov score 的连续分数做动态仓位。
- 训练目标从 first-edge continuation 推进到 execution-aware label：用同一套 `1s` 执行模型独立标注每个事件的真实执行收益。

## 代码变化

- `research/probabilistic_v5_ml_probability_model.py`
  - 新增多模型 sweep。
  - 新增多窗口 Markov 特征：`markov_llr_5s/15s/30s/60s`、`markov_combo_score`。
  - 新增 `sum_sized_net_edge` selection objective。
  - 新增 `hybrid_markov` sizing：默认 `EV 45% + prob 25% + Markov 30%`。
  - 动态仓位 calibration 改为只用 train+validation selected events，避免用 test 分布做 sizing 泄漏。

- `research/probabilistic_v6_execution_labeler.py`
  - 新增 execution-aware labeler。
  - 对每个 event 独立执行 `delay5 / initial_stop_atr=0.45 / breakeven=0.8R / trail_start=0.9R / max_hold=4h`。
  - 输出 `execution_return_pct`、`execution_win`、`execution_exit_reason`。
  - 为复用 V5 概率模型，把 `outcome` 重写成 execution win/loss，并把 `net_first_edge_atr` 临时作为 execution return target；原始 first-edge 字段保存在 `original_*` 列。

## V5 ML OOS 结果

V5 full-window 动态仓位曾在 Jan-Mar 全段跑出较高结果，但那不是严格 OOS：

| Symbol | Window | Model | Trades | Realistic | PF | DD |
|---|---|---|---:|---:|---:|---:|
| `BTCUSDT` | 2026 Jan-Mar all-window | logistic | 33 | `+3.9386%` | `1.445897` | `-4.6906%` |
| `ETHUSDT` | 2026 Jan-Mar all-window | logistic | 64 | `+4.8828%` | `1.213127` | `-4.7986%` |

严格 OOS 后退化明显：

| OOS | Symbol | Selected | Trades | Realistic | PF | 结论 |
|---|---|---:|---:|---:|---:|---|
| 2026 Mar, Jan train / Feb validation | `BTCUSDT` | 6 | 4 | `-0.5480%` | `0.748220` | 不通过 |
| 2026 Mar, Jan train / Feb validation | `ETHUSDT` | 15 | 9 | `-0.2590%` | `0.935379` | 不通过 |
| 2025 Dec, Oct train / Nov validation | `BTCUSDT` | 4 | 3 | `+0.3649%` | `1.556244` | 样本太小 |
| 2025 Dec, Oct train / Nov validation | `ETHUSDT` | 10 | 10 | `-1.5010%` | `0.593468` | 不通过 |

宽门槛 `hybrid_markov + sum_sized_net_edge` 在 2026 Mar 直接失败：

| Symbol | Selected | Trades | Realistic | PF |
|---|---:|---:|---:|---:|
| `BTCUSDT` | 65 | 31 | `-3.2538%` | 未晋级 |
| `ETHUSDT` | 80 | 31 | `-2.8606%` | 未晋级 |

解释：Markov/ML 能提高 event 层区分度，但如果 selection objective 追求样本量或 validation sum，OOS 会把低质量事件一起放进来；动态仓位会放大这个错误。

## V6 Execution-Aware Label

V6 先把全体可执行 event 标注出来：

| Dataset | Symbol | Events | Tradable | Wins | Avg Execution Return/Event |
|---|---|---:|---:|---:|---:|
| 2026 Jan-Mar | `BTCUSDT` | 510 | 242 | 127 | `-0.01896076%` |
| 2026 Jan-Mar | `ETHUSDT` | 483 | 203 | 108 | `-0.02049181%` |
| 2025 Q4 | `BTCUSDT` | 516 | 330 | 157 | `-0.06415811%` |
| 2025 Q4 | `ETHUSDT` | 547 | 304 | 156 | `-0.04087132%` |

也就是说，不筛选时 execution target 本身为负，必须靠概率模型挑子集。

### 2026 Mar OOS, Per-Symbol

训练：Jan；验证：Feb；执行：Mar。

| Symbol | Model | Validation Events | Test Events | Dynamic Realistic | Fixed 20% Realistic | 结论 |
|---|---|---:|---:|---:|---:|---|
| `BTCUSDT` | gradient_boosting | 27 | 35 | `-1.1130%` | `-0.1649%` | selection/sizing 均不通过 |
| `ETHUSDT` | logistic/random_forest family | 27 | 18 | `+1.5450%` | `+0.1438%` | 单窗有效，但未跨期确认 |

ETH 的 `+1.5450%` 来自动态仓位把 trailing winners 放大：

- Trades: `17`
- PF: `1.384505`
- Win: `58.82%`
- DD: `-2.6693%`
- Exit attribution: `TrailingSL +5.616635%`，`InitialSL -3.879278%`

BTC 的问题相反：fixed 20% 已接近打平，但 dynamic sizing 把 InitialSL 放大，说明 BTC 当前不能启用同一套 sizing。

### 2025 Dec OOS

训练：Oct；验证：Nov；测试：Dec。

per-symbol execution-aware 模型没有找到可晋级子集：

| Symbol | Best Objective | Test Events | Test Avg Execution Label | Test Sum Label | 结论 |
|---|---|---:|---:|---:|---|
| `BTCUSDT` | avg/sum 相同 | 17 | `-0.112338%` | `-1.909753` | 不通过 |
| `ETHUSDT` | avg/sum 相同 | 32 | `-0.019174%` | `-0.613579` | 不通过 |

## 结论

概率模型不能丢。它确实把 ETH 2026 Mar 从负收益区拉到了 `+1.5450%` 的单月 OOS；这已经不是原 V4 那种 `1%` 附近固定仓位筛选。但它还没有达到 `10%~20%` 实盘候选门槛：

- 2025 Dec 没过。
- BTC 在 2026 Mar 和 2025 Dec 都不稳定。
- ETH 只有一个 OOS 月份表现好，不能晋级。
- 当前最有价值的方向是 execution-aware probability + per-symbol dynamic sizing，而不是继续调 Markov 阈值。

## 下一步推进

1. 做 walk-forward 月度矩阵，而不是继续只看 Jan/Feb/Mar 或 Q4：
   - train 2 个月，validate 1 个月，execute 1 个月。
   - 至少覆盖 2025 全年 BTC/ETH。
   - 目标不是“某月正”，而是月度组合累计能接近 `10%~20%`，且月度 DD 可控。

2. sizing 需要分 symbol：
   - ETH 可以继续测试 dynamic share。
   - BTC 当前必须默认禁用 dynamic share，甚至在 validation 不满足更强稳定性条件时直接空仓。

3. Markov 继续作为小窗口状态输入：
   - 保留 `5/15/30/60s` state transition LLR。
   - 下一步加入 `state_seq` 的跨窗口 slope / agreement，例如短窗口强但长窗口弱时降低仓位。

4. 目标函数继续贴近执行：
   - 不再只训练 first-edge continuation。
   - 增加 `InitialSL` 风险 classifier 或 multi-task label：`TrailingSL winner`、`InitialSL loser`、`Breakeven/MaxHold`。

5. 如果要冲 `10%~20%`，靠当前 1h original_t2 单结构不够：
   - 需要组合 `original_t2`、`baseline_plus_t3`、可能还有不同 volatility regime 的独立模型。
   - portfolio 层必须做 top-K / risk budget，而不是每个 selected event 都执行。

## 2026-05-09 追加：V7 尝试与 lookahead 清理

本轮继续按“概率模型必须真正用起来”的方向推进，但重点改成先清理研究语义里的未来信息，再谈收益。

新增/修改：

- `research/probabilistic_v5_ml_probability_model.py`
  - 增加合法 point-in-time 特征：`touch_body_so_far_atr`、`touch_range_so_far_atr`、`touch_close_pos_so_far_side`、`touch_progress`、`level_to_signal_open_atr`、`touch_close_to_level_atr`、时间周期特征。
  - 增加已闭合 bar 上下文特征入口：`prev1_body_atr`、`prev1_range_atr`、`prev1_close_pos_side`、`prev_sma5_gap_atr`、`prev_sma5_slope_atr`、`level_to_prev_close_atr`。
  - 保留 `prob_initial_sl` 与 `sizing_sl_score`，但 InitialSL classifier 当前不能单独作为收益放大依据。

- `research/probabilistic_v6_walkforward_runner.py`
  - 增加 `--top-k-policy validation_best`，先用 validation 的 execution label 选择单个 top-K，再执行测试月。
  - 记录 `validation_topk_sized_return_pct`、`validation_topk_initial_sl_rate`、`validation_topk_max_dd_pct`，避免只看模型阈值。

- `research/probabilistic_v4_event_dataset.py`
  - 默认用上一根已闭合 bar 的 `prev_atr_1` / `prev_atr_percentile_1` 做 ATR 归一化，避免当前未闭合 signal bar 的完整 range 泄漏到事件与执行止损。
  - 输出上一根已闭合 bar 的 body/range/close-position/SMA gap 等上下文。

- `research/probabilistic_v4_execution_runner.py`、`research/order_flow_imbalance_breakout.py`
  - 增加缺失旧 research helper 时的兼容 fallback，保证当前 research harness 可复跑。

曾经用 `signal_close/signal_high/signal_low` 做上下文时，V7 在三段样本上出现过非常高的结果：

| Run | Execute | Portfolio Silo Sum | 结论 |
|---|---|---:|---|
| `walkforward_2025_q3_v7_signalctx_valbest_probev` | 2025-09 | `+4.8130%` | 作废：使用当前 signal bar 完整 OHLC，存在 lookahead |
| `walkforward_2025_q4_v7_signalctx_valbest_probev` | 2025-12 | `+13.7941%` | 作废：同上 |
| `walkforward_2026_q1_v7_signalctx_valbest_probev` | 2026-03 | `+12.7492%` | 作废：同上 |

清理为 point-in-time 后，结果明显回落：

| Run | Execute | Active Result | 结论 |
|---|---|---:|---|
| `walkforward_2025_q3_v7_pointintime_valbest_probev` | 2025-09 | ETH top15 `-1.9905%` | 不通过 |
| `walkforward_2025_q3_v7_pointintime_valbest_markov` | 2025-09 | ETH top15 `-4.0719%` | 不通过 |
| `walkforward_2026_q1_v7_pointintime_valbest_probev` | 2026-03 | BTC top20 `-1.4667%`, ETH top15 `+1.4629%` | 组合约打平，不通过 |
| `walkforward_2025_q4_v7_pointintime_valbest_probev` | 2025-12 | BTC top15 `-3.7057%`, ETH gated out | 不通过 |
| `walkforward_2025_q3_v7_prevatr_valbest_probev` | 2025-09 | ETH top20 `-2.6501%` | `prev_atr_1` 修正后仍不通过 |

当前结论：

- 概率模型不能丢，但原 `original_t2` 单结构在严格 point-in-time 语义下还没有接近 `10%~20%` 候选。
- 高收益版本主要来自当前 signal bar 完整 OHLC / ATR 的未来信息，必须从候选中剔除。
- validation top-K 与 InitialSL gate 能减少部分坏样本，但不能识别 2025-09 / 2025-12 这类 regime shift。
- 下一步如果继续冲收益，应换“事件来源 + regime 组合”，而不是继续在同一批 `original_t2` events 上调模型阈值：优先做 `baseline_plus_t3` / volatility-regime 独立模型 / portfolio-level no-trade gate。

## 2026-05-09 追加：baseline_plus_t3 与 delay60 概率路线

用户提醒 `baseline_plus_t3` 之前已经判断偏弱，所以本轮只验证“概率模型是否能从弱信号源中挑出可交易子集”，不把它当作更强 baseline。

### baseline_plus_t3 Q3 复核

重建 Q3 `baseline_plus_t3` point-in-time ATR 事件：

- `research/probabilistic_v6_runs/2025_q3_baseline_plus_t3_pointintime_atr/events.csv`
- rows: `1303`
- execution-labeled tradable rows: `846`

事件分布：

| Symbol | Shape | continuation | fail | timeout |
|---|---|---:|---:|---:|
| `BTCUSDT` | `original_t2` | 164 | 316 | 15 |
| `BTCUSDT` | `t3_swing` | 38 | 86 | 2 |
| `ETHUSDT` | `original_t2` | 181 | 338 | 9 |
| `ETHUSDT` | `t3_swing` | 51 | 103 | 0 |

execution label 摘要：

| Symbol | Shape | Tradable | Wins | Avg Execution Return |
|---|---|---:|---:|---:|
| `BTCUSDT` | `original_t2` | 367 | 165 | `-0.071362%` |
| `BTCUSDT` | `t3_swing` | 91 | 42 | `-0.060973%` |
| `ETHUSDT` | `original_t2` | 314 | 151 | `-0.109697%` |
| `ETHUSDT` | `t3_swing` | 74 | 40 | `-0.041829%` |

对应 walk-forward：

- Run: `research/probabilistic_v6_runs/walkforward_2025_q3_baseline_plus_t3_v7_prevatr_valbest_probev`
- BTC: validation edge 为负，被 gate 掉。
- ETH: validation 选择 top15，但 2025-09 真实执行 `-3.9517%`，PF `0.328805`，DD `-4.7858%`。

结论：`baseline_plus_t3` 原始信号确实偏弱；概率模型不能从这批 Q3 事件里稳定救出收益。继续在这条弱事件源上调阈值意义不大。

### entry5 下的 pullback60 诊断作废

9 个月 execution label 诊断曾发现 `side=short & pullback_60s_atr:q2` label 累计 `+9.7049%`，但这是 `entry_delay=5s` 语义下的未来信息：`pullback_60s_atr` 需要 touch 后 60 秒才知道，不能用于 5 秒入场决策。

把该 slice 固定后用真实 fixed20 执行验证：

| Window | BTC | ETH | Silo Sum |
|---|---:|---:|---:|
| 2025 Q3 | `-1.6215%` | `+0.1592%` | `-1.4623%` |
| 2025 Q4 | `+0.0378%` | `-0.3733%` | `-0.3355%` |
| 2026 Jan-Mar | `+0.2788%` | `+0.6634%` | `+0.9422%` |

结论：这个方向不能作为 5 秒入场收益证据。

### 合法 delay60 概率路线

为合法使用 touch 后 60 秒确认特征，新增/调整：

- `research/probabilistic_v5_ml_probability_model.py`
  - 在 `--feature-horizon-seconds >= 60` 时纳入 `dwell_60s_*` 与 `pullback_60s_atr`。
- `research/probabilistic_v6_walkforward_runner.py`
  - 新增 `--feature-horizon-seconds` 并强制 `feature_horizon_seconds <= entry_delay_seconds`，避免 post-entry leakage。
- `research/probabilistic_v6_feature_slice_analyzer.py`
  - 用 execution labels 扫描单特征/组合特征 slice，只作为假设生成器。
- `research/probabilistic_v6_no_trade_gate_analyzer.py`
  - 扫描 validation-only no-trade gate，并加入 overfit 抑制字段。

重建 `original_t2 + dwell_60s` 事件与 label：

| Dataset | Events | Delay60 Tradable Labels |
|---|---:|---:|
| 2025 Q3 | 1023 | 314 |
| 2025 Q4 | 1063 | 293 |
| 2026 Jan-Apr | 993 | 194 |

delay60 label 诊断的最佳合法 slice：

- Run: `research/probabilistic_v6_runs/feature_slice_original_t2_delay60_10m`
- Slice: `side=short & speed_60s_atr:q4>0.259108`
- Events: `99`
- Label sum: `+10.1355%`
- Positive months: `7/9`
- InitialSL rate: `33.33%`
- Worst month: `-1.9167%`

但固定规则真实执行仍不够：

| Window | BTC | ETH | Silo Sum |
|---|---:|---:|---:|
| 2025 Q3 | `-0.1663%` | `+0.0021%` | `-0.1642%` |
| 2025 Q4 | `-0.2659%` | `+0.4241%` | `+0.1582%` |
| 2026 Jan-Apr | `+0.5526%` | `+0.2901%` | `+0.8427%` |

因此继续接回概率模型做 walk-forward：

- Run: `research/probabilistic_v6_runs/walkforward_delay60_original_t2_feature60_valbest`
- Setup: train 2 months / validation 1 month / execute 1 month
- `entry_delay_seconds=60`
- `feature_horizon_seconds=60`
- `top_k_policy=validation_best`
- `top_k_selection_metric=return_over_drawdown`
- sizing: `hybrid_markov`

active 结果：

| Execute Month | Active Silo Return |
|---|---:|
| 2025-11 | `+0.4103%` |
| 2025-12 | `-0.7900%` |
| 2026-01 | `+0.1137%` |
| 2026-02 | `+2.1110%` |
| 2026-03 | `+1.7533%` |

合计：

- Active rows: `8`
- Trades: `69`
- Silo sum: `+3.5983%`
- BTC contribution: `+1.5930%`
- ETH contribution: `+2.0053%`

再用 validation-only no-trade gate 做 post-hoc 诊断：

- Baseline: `+3.5983%`
- Best non-empty gate: `+6.0939%`
- Gate 大意：保留 validation return/DD >= 1，且 validation topK sized return <= `7%`，用于挡掉 validation 过热的 BTC 2026-03；同时挡掉 ETH 2026-01/02 的弱 sleeve。

随后把该 gate 正式接入 V6 runner：

- `--min-validation-topk-return-over-dd`
- `--max-validation-topk-return-over-dd`
- `--max-validation-topk-return-pct`
- `--validation-topk-gate-stage`

这里分两种语义：

| Gate Stage | 语义 | Run | Silo Sum | 结论 |
|---|---|---|---:|---|
| `candidate_filter` | 先过滤候选 topK，再允许 fallback 到另一个 topK | `walkforward_delay60_original_t2_feature60_formal_gate` | `+4.7201%` | BTC 2026-03 从 top15 fallback 到 top10，仍亏 `-1.3738%` |
| `post_selection` | 先按原规则选 topK，再对被选中的 topK 做 gate，失败则空仓 | `walkforward_delay60_original_t2_feature60_postselect_gate` | `+6.0939%` | 与 post-hoc 诊断一致 |

`post_selection` active rows：

| Execute Month | Symbol | TopK | Trades | Realistic | Gate Detail |
|---|---|---:|---:|---:|---|
| 2025-11 | `BTCUSDT` | 15 | 6 | `+0.4103%` | pass |
| 2025-12 | `BTCUSDT` | 20 | 11 | `-0.7900%` | pass，validation 不过热 |
| 2026-01 | `BTCUSDT` | 20 | 16 | `+0.3375%` | pass |
| 2026-02 | `BTCUSDT` | 10 | 10 | `+3.0090%` | pass |
| 2026-03 | `ETHUSDT` | 10 | 8 | `+3.1271%` | pass |

被挡掉的关键 sleeve：

- ETH 2026-01 / 2026-02：`validation_topk_return_over_dd < 1.0`
- BTC 2026-03：`validation_topk_return > 7.0`

当前仍无法挡掉 BTC 2025-12：它的 validation topK return `1.494166%`、return/DD `2.429064`、InitialSL rate `0.25`，看起来并不过热，但 execute 月真实执行 `-0.7900%`。这类更像 regime shift，需要额外市场状态特征，而不是继续只调 validation topK 字段。

当前结论更新：

- 概率模型确实有用：合法 delay60 + feature60 + dynamic sizing 已从负/1% 区间推进到 `+3.5983%`，post-hoc gate 可到 `+6.0939%`。
- 仍不到 `10%~20%` 实盘候选门槛。
- 最值得继续的是 `delay60` 路线的 regime gate，而不是继续调原 entry5 阈值。
- `baseline_plus_t3` 暂不晋级，除非后续在 delay60 或 regime-specific 事件源上重新证明。
