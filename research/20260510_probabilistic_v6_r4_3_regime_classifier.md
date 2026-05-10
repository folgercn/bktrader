# 2026-05-10 Probabilistic V6 R4.3 Regime Classifier 初探

## 范围

本轮仍只覆盖 `research`。目标是验证 R4.2 失败后的下一条路线：

> 不再继续微调单个 validation gate，而是在 symbol-month / event-cluster 层面先判断是否可交易，再让概率模型决定 topK / sizing。

本阶段新增：

- classifier prototype: `research/probabilistic_v6_regime_classifier.py`
- enhanced all-topK classifier run: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_original_t2_delay60_r4_2_features_alltopk_matrix/regime_classifier/summary.md`
- old all-topK classifier control: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_delay60_feature60_alltopk_matrix/regime_classifier/summary.md`
- enhanced event feature slices: `research/probabilistic_v6_runs/2025m03_2026apr_original_t2_delay60_r4_2_features/r4_3_feature_slices.md`

## Symbol-Row Classifier

`probabilistic_v6_regime_classifier.py` 只消费已有 `symbol_rows.csv`。它使用历史已完成月份训练 `tradeable / no-trade` classifier，用上一个 execute month 做模型和 `prob_min` 选择，再应用到下一个 execute month。当前 execute month 的收益不参与模型或阈值选择。

候选模型：

- logistic
- random_forest
- extra_trees
- gradient_boosting
- svm_rbf

可用输入只包含 execute 前可见字段，例如 `validation_topk_*`、`top_k`、`symbol`、`model_name`、sizing summary；不使用 `test_edge`、`realistic_return_pct`、PF、MaxDD 等 execute label 字段做特征。

### Enhanced R4.2 all-topK

| Metric | Value |
|---|---:|
| Active_Silo_Sum | +2.5444% |
| Active Months | 4 |
| Active Rows | 5 |
| Trades | 40 |
| Worst Month | -1.1777% |
| Target Hit | False |
| Oracle Positive Per Symbol-Month | +9.2568% |

选中 rows：

| Month | Symbol | TopK | Row Model | Regime Prob | Trades | Realistic |
|---|---|---:|---|---:|---:|---:|
| 2025-11 | BTCUSDT | 5 | gradient_boosting | 0.7963 | 5 | -0.2630% |
| 2026-02 | ETHUSDT | 10 | gradient_boosting | 0.7200 | 7 | +2.5318% |
| 2026-03 | BTCUSDT | 10 | extra_trees | 0.9884 | 10 | -1.6595% |
| 2026-03 | ETHUSDT | 10 | logistic | 0.9508 | 9 | +3.1128% |
| 2026-04 | ETHUSDT | 15 | logistic | 0.5476 | 9 | -1.1777% |

### Old all-topK control

| Metric | Value |
|---|---:|
| Active_Silo_Sum | -2.0012% |
| Active Months | 2 |
| Active Rows | 2 |
| Trades | 14 |
| Worst Month | -1.0482% |
| Target Hit | False |
| Oracle Positive Per Symbol-Month | +10.4570% |

结论：只用 `symbol_rows.csv` 这一层的 validation metrics 做 classifier 不够。它能少交易，但不能稳定识别 2026-03 BTCUSDT、2026-04 ETHUSDT 这种 validation 很强但 execute 转负的 regime。

## Event-Level Slice 线索

R4.3 同时用 enhanced event dataset 跑了 feature-slice analyzer。这个扫描使用 execution labels 做假设生成，阈值来自全样本分位数，因此不能直接作为 OOS 规则，但能告诉我们下一步该把 regime label 建在哪里。

最强 pair slices：

| Slice | Events | Months | Label Return | Win | InitialSL | Pos Months | Worst Month |
|---|---:|---:|---:|---:|---:|---:|---:|
| `side=short & speed_60s_atr:q4>0.260001` | 167 | 14 | +8.9587% | 0.5748 | 0.3892 | 8 | -1.9167% |
| `symbol_side=BTCUSDT_short & eff_60s:q2<=0.949966` | 71 | 14 | +8.6851% | 0.7042 | 0.2254 | 11 | -1.2413% |
| `symbol_side=ETHUSDT_short & prev1_range_atr:q4>0.914154` | 63 | 14 | +8.2561% | 0.6032 | 0.3810 | 9 | -2.5000% |
| `symbol_side=ETHUSDT_short & prev1_close_pos_side:q3<=0.837915` | 65 | 14 | +8.2467% | 0.6308 | 0.3385 | 10 | -1.4772% |
| `symbol=BTCUSDT & flow_ratio_15s:q2<=0.914133` | 186 | 14 | +7.4800% | 0.5806 | 0.3656 | 9 | -1.6572% |

这组线索比 symbol-row classifier 更有价值：它们不是在“某个月 topK 是否强”上做判断，而是在事件形成时直接描述了短窗口行为。例如 `BTCUSDT_short & eff_60s` 的 InitialSL rate 只有 `0.2254`，且 14 个月里 11 个 positive months，明显更像可迁移的事件簇。

## 结论

R4.3 第一版结论：

- symbol-row classifier 已实现，但收益只有 `+2.5444%`，不晋级；
- 仅靠 validation topK summary 学不到足够稳定的 no-trade regime；
- event-level slice 暴露了更好的方向：先固定事件簇 / regime label，再回到 V6 walk-forward runner 验证真实 execution、busy/same-bar 约束和 portfolio-level 选择。

下一步不建议继续扩 symbol-row classifier 模型族。应该把 top event slices 固定成 train/validation 内可计算的规则，生成 filtered event datasets，然后按 R3 同口径复跑 V6：

1. `BTCUSDT_short & eff_60s_low`
2. `short & speed_60s_high`
3. `ETHUSDT_short & prev1_close_pos_side_mid_or_lower`
4. `BTCUSDT flow_ratio_15s_low`

只有 filtered runner 的 validation-only selector 达到 `10% ~ 20%`，才值得进入更复杂的 GRU / sequence model。
