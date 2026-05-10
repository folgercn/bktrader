# 2026-05-10 Probabilistic V6 R4 baseline_plus_t3 复盘

## 范围

本轮只覆盖 `research`，不改 `live` / `internal` 执行路径。目标是验证 R4.1：

> 将 `baseline_plus_t3` 事件源接入 full-window execution-labeled pipeline 后，概率模型是否能从更宽事件池里筛出 `10% ~ 20%` 级别候选？

产物：

- labeled dataset: `research/probabilistic_v6_runs/2025m03_2026apr_baseline_plus_t3_delay60/events_execution_labeled.csv`
- validation_best run: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback/summary.md`
- validation_best selector scan: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_postselect_btc_fallback/no_trade_gate_scan_fullgrid.md`
- all-topK matrix: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix/summary.md`
- all-topK selector scan: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_baseline_plus_t3_delay60_feature60_alltopk_matrix/no_trade_gate_scan_alltopk_target.md`

当前 runner 仍是 `Baseline_Derived_Sizing`：event selection + 1s execution + `hybrid_markov` / BTC fixed 20% fallback，不是完整 `dir2_zero_initial=true` + `zero_initial_mode=reentry_window` lifecycle。

## 数据覆盖

`baseline_plus_t3` full-window labeled dataset 共 `1679` 行，覆盖 2025-03 至 2026-04：

| Window | Rows | BTCUSDT | ETHUSDT |
|---|---:|---:|---:|
| 2025-03 ~ 2025-06 | 510 | 288 | 222 |
| 2025-Q3 | 406 | 238 | 168 |
| 2025-Q4 | 369 | 199 | 170 |
| 2026-01 ~ 2026-03 | 264 | 140 | 124 |
| 2026-04 | 130 | 83 | 47 |
| Total | 1679 | 948 | 731 |

事件池比 Original_T2 full-window 的 `1303` 行更宽，但原始标签质量没有同步改善。ETHUSDT 在多个窗口的平均 execution return 为负，说明 `t3_swing` 扩源引入了更多 false breakout。

## validation_best 结果

同 R3 参数口径：

- `entry_delay_seconds=60`
- `feature_horizon_seconds=60`
- `top_k_policy=validation_best`
- `rank_by=prob_ev_atr`
- `validation_topk_gate_stage=post_selection`
- BTC sizing fallback: `fixed20_on_initial_sl`

| Metric | Value |
|---|---:|
| Active_Silo_Sum | -7.5453% |
| Calendar_Normalized_Return | -0.342968% |
| Active Months | 8 |
| Active Silos | 11 |
| Trades | 100 |
| Worst Active Silo | -2.7136% |
| Negative Active Silos | 7 |

按固定 topK 聚合：

| TopK | Active_Silo_Sum | Active Months | Trades | Worst Active Silo |
|---:|---:|---:|---:|---:|
| 5 | -4.7993% | 5 | 23 | -2.6500% |
| 10 | -2.8872% | 3 | 38 | -2.7136% |
| 15 | 0.0000% | 0 | 0 | 0.0000% |
| 20 | +0.1412% | 2 | 39 | -0.6461% |

validation_best 的 no-trade gate 扫描最佳非空结果只有 `+1.2517%`，active months `4`，trades `49`，没有 qualified result，`target_hit=false`。该扫描的 post-hoc positive oracle 也只有 `+4.2515%`，说明这个 validation_best 子池本身很薄。

## all-topK matrix 结果

all-topK 用 `top_k=5/10/15/20` 全展开，观察是否是 validation_best 过早丢掉了可用 sleeve。

结果更差：

| Metric | Value |
|---|---:|
| Active_Silo_Sum | -50.7477% |
| Calendar_Normalized_Return | -2.306714% |
| Active Months | 9 |
| Active Silos | 60 |
| Trades | 560 |
| Worst Active Silo | -6.7528% |
| Negative Active Silos | 39 |

固定 topK 全部为负：

| TopK | Active_Silo_Sum | Active Months | Trades | Worst Active Silo |
|---:|---:|---:|---:|---:|
| 5 | -10.6403% | 9 | 73 | -4.4448% |
| 10 | -14.8885% | 9 | 132 | -6.0328% |
| 15 | -11.7125% | 9 | 167 | -6.1758% |
| 20 | -13.5064% | 9 | 188 | -6.7528% |

all-topK selector scan：

| Selector | Active_Silo_Sum | Active Months | Trades | Worst Month | Target Hit |
|---|---:|---:|---:|---:|---|
| best non-empty validation-only gate | +5.7066% | 7 | 194 | -2.5470% | False |
| qualified under target constraints | N/A | N/A | N/A | N/A | False |
| oracle positive per symbol-month | +7.6231% | 5 | 47 | +0.0148% | post-hoc only |

关键点：事后 oracle 每个 `(execute_month, symbol)` 只挑正收益 sleeve，仍只有 `+7.6231%`，低于 `10%`。这说明 `baseline_plus_t3` 当前事件源的可交易上限不足，不是单纯 selector 没调好。

## 归因

1. `baseline_plus_t3` 扩大事件数量，但没有提高事件质量；2025-07、2025-08、2025-09、2025-11、2026-01、2026-04 均出现 validation pass / execute loss。
2. Markov 分数仍有诊断价值，但它在坏月也会偏高。例如 ETHUSDT 2025-07 validation edge 和 Markov 都高，execute 却为 `-2.6500%`。
3. BTC fixed fallback 能减亏，但救不了事件池。all-topK 中不少 BTC dynamic loss 被 fixed20 缩小，组合仍大幅为负。
4. 当前特征对“touch 后是假突破还是有效延续”分辨不够，尤其缺少更短窗口的 flow transition、速度衰减、touch 后反抽深度和 regime 状态。

## 结论

`baseline_plus_t3` 不应作为下一轮主事件源继续阈值微调。R4.1 判定为失败：

- validation_best：`-7.5453%`
- all-topK：`-50.7477%`
- 最佳 validation-only gate：`+5.7066%` 且 worst month `< -2%`
- post-hoc oracle：`+7.6231%`，低于 `10%`

概率模型继续保留，但下一阶段重点必须转到输入质量，而不是扩大结构事件源。

## R4.2 已落地的输入增强

已更新：

- `research/probabilistic_v4_event_dataset.py`
- `research/probabilistic_v5_ml_probability_model.py`
- `research/probabilistic_v6_walkforward_runner.py`

新增或接入的特征包括：

- `flow_ratio_5s` / `flow_ratio_15s` / `flow_ratio_30s`
- `flow_delta_5_60s` / `flow_delta_15_60s` / `flow_delta_30_120s`
- `volume_ratio_5s` / `volume_ratio_15s` / `volume_ratio_60s`
- `speed_5s_atr`
- `speed_decay_5_60s_atr` / `speed_decay_15_60s_atr` / `speed_decay_60_300s_atr`
- `eff_15s`
- `close_pos_15s`
- `pullback_5s_atr`

smoke 产物：

- `research/probabilistic_v6_runs/r4_2_feature_smoke/events.csv`
- `research/probabilistic_v6_runs/r4_2_feature_smoke/ml_model.json`

smoke 结果：BTCUSDT 2026-04-01 至 2026-04-03 产出 `14` 个事件，新列正常落盘，V5 模型可训练。

## R4.2 结果更新

R4.2 已用增强特征重建 `original_t2` full-window dataset 并复跑 V6，详见 `research/20260510_probabilistic_v6_r4_2_enhanced_features.md`。

关键结论：

- validation_best Active_Silo_Sum `-0.2361%`；
- all-topK Active_Silo_Sum `-57.7621%`；
- 可交易 validation-only selector 最佳非空 `+8.9897%`，但 active months 只有 `4`；
- qualified selector `+5.7496%`；
- post-hoc oracle positive per symbol-month `+9.2568%`，仍低于 `10%`。

因此 R4.2 也不支持继续阈值微调。下一阶段应进入 R4.3 regime classifier / 标签重构。
