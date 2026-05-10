# Probabilistic V6 Union Selector Sweep

范围：仅限 `research`。本阶段目标不是继续围绕 1% 边缘优化，而是把概率模型重新用到组合选择和仓位质量控制里，并用事件级 union 回测替代 rows 相加。

## 结论

`union_selector_sweep_markov_cap` 找到一个超过 10% 的候选组合：

| Candidate | Union Return | Trades | Active Months | Worst Silo | Gate Summary |
|---|---:|---:|---:|---:|---|
| `candidate_001` | 10.8467% | 115 | 9 | -1.3744% | `edge>=0.05`, `val_topk_return>=0.5%`, `return/DD<=10`, `0.4<=markov<=0.9` |
| `candidate_002` | 10.6023% | 110 | 8 | -1.3744% | same, `markov<=0.85` |
| `candidate_003` | 10.5743% | 132 | 9 | -1.3744% | same, no Markov lower bound, `markov<=0.9` |
| `candidate_004` | 10.3299% | 127 | 8 | -1.3744% | same, no Markov lower bound, `markov<=0.85` |
| `candidate_005` | 10.3012% | 121 | 10 | -1.3744% | same, `return/DD<=20`, `0.4<=markov<=0.9` |
| `candidate_006` | 10.0568% | 116 | 9 | -1.3744% | same, `return/DD<=20`, `0.4<=markov<=0.85` |
| `candidate_008` | 10.0288% | 138 | 10 | -1.3744% | same, no Markov lower bound, `markov<=0.9` |
| `candidate_007` | 9.7496% | 94 | 8 | -1.3744% | looser edge, stricter `SL<=0.3` |

这说明概率模型不是完全无效，问题更像是 V3/V6 早期组合方式把“概率强弱”和“可交易事件 union”混在一起了。rows 相加会高估，但在 union 级别加入 `validation return/DD` 和 Markov 置信度上限后，真实去重回测仍可站上 10%。

## Candidate 001 明细

| Silo | Return | Trades |
|---|---:|---:|
| `2025-06 ETHUSDT` | 0.1090% | 3 |
| `2025-07 BTCUSDT` | -0.7250% | 18 |
| `2025-08 BTCUSDT` | -1.3744% | 5 |
| `2025-09 ETHUSDT` | 0.2444% | 5 |
| `2025-11 BTCUSDT` | 0.4103% | 6 |
| `2025-12 ETHUSDT` | 0.4398% | 24 |
| `2026-01 BTCUSDT` | 0.4559% | 16 |
| `2026-02 BTCUSDT` | 4.3173% | 15 |
| `2026-02 ETHUSDT` | 2.1529% | 12 |
| `2026-03 ETHUSDT` | 4.8165% | 11 |

关键变化不是简单删掉亏损月，而是用 validation-only 质量约束挡掉了三个明显拖累源：

- `2025-10 ETHUSDT short_speed60_high`：`validation_topk_sized_return_pct=0.3935%`，低于 `0.5%`。
- `2026-04 ETHUSDT short_speed60_high`：`validation_markov=0.9115`，超过 `0.9`，属于置信度过热风险。
- `2026-04 ETHUSDT delay60_feature60`：`validation_return_over_dd=11.2131`，超过 `10`，同样偏过热。

仍保留的主要风险是 `2025-07/2025-08 BTCUSDT`，它们 validation 指标并不差，靠 pass/fail gate 很难过滤。下一阶段应该做概率分数联动仓位，把这类“通过但不强”的 BTC sleeve 降到低仓位，而不是硬删。

## 产物

- 工具：`research/probabilistic_v6_union_selector_sweep.py`
- 主结果：`research/probabilistic_v6_runs/walkforward_2025m06_2026apr_combo_baseline_short_speed/union_selector_sweep_markov_cap/summary.json`
- 可读报告：`research/probabilistic_v6_runs/walkforward_2025m06_2026apr_combo_baseline_short_speed/union_selector_sweep_markov_cap/summary.md`
- 手工预检候选：`research/probabilistic_v6_runs/walkforward_2025m06_2026apr_combo_baseline_short_speed/union_markov_cap_v1/summary.json`

## 注意

- Gate 字段只来自 validation period，`test_edge` 不参与筛选。
- 这仍然是 execute window 上的探索性 post-selection，不能直接视为实盘候选。
- 当前 runner 是 one-shot 1s execution，不是完整 `dir2_zero_initial=true` / `reentry_window` 生命周期；下一步要么补生命周期复测，要么明确作为概率 sleeve 候选继续筛。
- 下一阶段优先做仓位联动：按 validation confidence、Markov score、return/DD 和模型 share 对 `model_notional_share` 做 cap/decay，再用 union runner 复测是否能压低 `2025-07/08 BTC` 的拖累。
