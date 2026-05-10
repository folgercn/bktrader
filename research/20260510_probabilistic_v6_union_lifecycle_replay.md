# Probabilistic V6 Union Lifecycle Replay

范围：仅限 `research`。本阶段承接 `candidate_001` union sizing calibration，把概率模型真正接到完整 `dir2_zero_initial=true` / `zero_initial_mode=reentry_window` 生命周期里。

## 结论

概率模型不能丢，而且这次终于不是 1% 附近打转了。

在完整 `reentry_window` 生命周期下，两个候选都进入 30%+ active silo sum 区间：

| 配置 | Lifecycle Return | One-shot Return | Trades | Active Silos | Worst Silo | Negative Silos |
|---|---:|---:|---:|---:|---:|---:|
| `quality_edge_return_mult_1p20_cap_1p80` | 33.4100% | 12.8486% | 310 | 10 | -0.4400% | 1 |
| `power0_fixed_1p30` | 33.0200% | 13.4179% | 310 | 10 | -0.4000% | 1 |

这说明上一阶段的 caveat 是实质性的：one-shot 1s execution 低估了概率筛选事件在完整 `Zero-Initial-Reentry` + `SL-Reentry` 生命周期里的扩展收益。概率模型在这里的角色不是直接预测每一笔能不能赚，而是：

1. 作为 breakout-lock gate：只允许被概率 union 选中的 `original_t2` 事件进入 reentry-window 生命周期。
2. 作为风险预算倍率：`model_notional_share` 不再当成单笔绝对仓位，而是作为 baseline schedule 的倍率，例如 `1.30 => [0.26, 0.13]`。

## 明细

`power0_fixed_1p30` 的主要贡献：

| Silo | Return | Trades |
|---|---:|---:|
| `2025-06 ETHUSDT` | 0.84% | 6 |
| `2025-07 BTCUSDT` | 2.36% | 62 |
| `2025-08 BTCUSDT` | -0.40% | 18 |
| `2025-09 ETHUSDT` | 0.73% | 11 |
| `2025-11 BTCUSDT` | 2.16% | 12 |
| `2025-12 ETHUSDT` | 7.91% | 60 |
| `2026-01 BTCUSDT` | 1.45% | 45 |
| `2026-02 BTCUSDT` | 5.58% | 42 |
| `2026-02 ETHUSDT` | 8.12% | 30 |
| `2026-03 ETHUSDT` | 4.27% | 24 |

`quality_edge_return_mult_1p20_cap_1p80` 的主要贡献：

| Silo | Return | Trades | Mean Scale |
|---|---:|---:|---:|
| `2025-06 ETHUSDT` | 0.50% | 6 | 0.793373 |
| `2025-07 BTCUSDT` | 2.46% | 62 | 1.323754 |
| `2025-08 BTCUSDT` | -0.44% | 18 | 1.395229 |
| `2025-09 ETHUSDT` | 0.97% | 11 | 1.592907 |
| `2025-11 BTCUSDT` | 1.71% | 12 | 0.825342 |
| `2025-12 ETHUSDT` | 8.98% | 60 | 1.298735 |
| `2026-01 BTCUSDT` | 0.92% | 45 | 0.993852 |
| `2026-02 BTCUSDT` | 5.20% | 42 | 1.211847 |
| `2026-02 ETHUSDT` | 8.39% | 30 | 1.359652 |
| `2026-03 ETHUSDT` | 4.72% | 24 | 1.464222 |

## 关键观察

`2025-07 BTCUSDT` 在 one-shot 里是负贡献，但生命周期里变成正贡献：`power0_fixed_1p30` 为 `+2.36%`，`quality_edge_return_mult_1p20_cap_1p80` 为 `+2.46%`。这说明单独按事件一次性执行会错过 reentry-window 的主要收益形态。

`2025-08 BTCUSDT` 仍是唯一负 silo，但亏损已压到 `-0.40%/-0.44%`。这比 one-shot sizing calibration 里的 `-1.6084%/-1.5747%` 明显更可接受。

质量 sizing 没有显著改善 worst silo，但总收益略高于 fixed：`33.4100%` vs `33.0200%`。它主要把收益从 `2026-01 BTC` 转移到 `2025-12/2026-02/2026-03 ETH`，更像 regime risk budget，而不是单纯防守。

## 产物

- Runner hook：`research/eth_q1_breakout_t3_shape_compare.py`
  - 新增可选 `breakout_gate`
  - 不传 gate 时默认行为不变
  - ledger 增加 `notional_share` / `sizing_scale`
- Lifecycle runner：`research/probabilistic_v6_union_lifecycle_runner.py`
  - 消费 union sizing sweep 输出
  - 支持现有 `flow_1s.pkl` cache 拼接，避免反复解压 tick
  - 输出完整 per-config/per-silo lifecycle summary
- 主结果：`research/probabilistic_v6_runs/walkforward_2025m06_2026apr_combo_baseline_short_speed/union_lifecycle_reentry_window_candidate_001/summary.json`
- 可读结果：`research/probabilistic_v6_runs/walkforward_2025m06_2026apr_combo_baseline_short_speed/union_lifecycle_reentry_window_candidate_001/summary.md`

## 下一步

1. 优先把 `quality_edge_return_mult_1p20_cap_1p80` 和 `power0_fixed_1p30` 做 cross-year / cross-asset 复测，不能只信 2025-06 到 2026-03 这一个 post-selection 窗口。
2. 给 lifecycle runner 增加 non-selected calendar silos 报告，把空仓月份按 0% 纳入固定网格，避免 active silo sum 被误读成组合净值曲线。
3. 如果 cross-year 仍能站在 10-20% 以上，再做参数收敛：`scale_cap`、BTC/ETH 分资产倍率、质量 sizing 的 `edge/return/DD/markov` 权重。

## 注意

- 当前结果仍是 research post-selection，不是实盘候选。
- `33%` 是 active silo sum，不是 live portfolio equity curve。
- 本轮只接了 `original_t2` 1h intrabar breakout；没有把 `baseline_plus_t3` 的 t3_swing 事件纳入生命周期候选。
