# Probabilistic V6 Union Lifecycle Replay

范围：仅限 `research`。本阶段把概率 union 选中的 breakout 作为完整 reentry-window 生命周期的 breakout-lock gate。

## Setup

- Source sweep: `research/probabilistic_v6_runs/walkforward_2025m06_2026apr_combo_baseline_short_speed/union_sizing_calibration_sweep_candidate_001`
- Signal timeframe: `1h`
- Breakout shape: `original_t2`
- Lifecycle: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`, `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`
- Sizing: `model_notional_share` 作为 baseline schedule 的倍率，例如 `1.30 => [0.26, 0.13]`，不是单笔 130% 仓位。

## Config Metrics

| Config | Lifecycle Return | One-shot Return | Trades | Active Silos | Worst Silo | Negative Silos |
|---|---:|---:|---:|---:|---:|---:|
| `power0_fixed_1p30` | 33.0200% | 13.4179% | 310 | 10 | -0.4000% | 1 |
| `quality_edge_return_mult_1p20_cap_1p80` | 33.4100% | 12.8486% | 310 | 10 | -0.4400% | 1 |

## Group Rows

| Config | Month | Symbol | Events | Gate Keys | Allowed Locks | Rejected Locks | Trades | Return | Max DD | Entry Reasons |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---|
| `power0_fixed_1p30` | `2025-06` | `ETHUSDT` | 3 | 3 | 3 | 179 | 6 | 0.84% | -0.03% | `{'Zero-Initial-Reentry': 3, 'SL-Reentry': 3}` |
| `power0_fixed_1p30` | `2025-07` | `BTCUSDT` | 19 | 19 | 18 | 162 | 62 | 2.36% | -0.24% | `{'SL-Reentry': 44, 'Zero-Initial-Reentry': 18}` |
| `power0_fixed_1p30` | `2025-08` | `BTCUSDT` | 5 | 5 | 5 | 175 | 18 | -0.40% | -0.42% | `{'SL-Reentry': 13, 'Zero-Initial-Reentry': 5}` |
| `power0_fixed_1p30` | `2025-09` | `ETHUSDT` | 5 | 5 | 5 | 198 | 11 | 0.73% | -0.04% | `{'SL-Reentry': 6, 'Zero-Initial-Reentry': 5}` |
| `power0_fixed_1p30` | `2025-11` | `BTCUSDT` | 6 | 6 | 6 | 197 | 12 | 2.16% | -0.07% | `{'Zero-Initial-Reentry': 6, 'SL-Reentry': 6}` |
| `power0_fixed_1p30` | `2025-12` | `ETHUSDT` | 25 | 25 | 25 | 160 | 60 | 7.91% | -0.24% | `{'SL-Reentry': 35, 'Zero-Initial-Reentry': 25}` |
| `power0_fixed_1p30` | `2026-01` | `BTCUSDT` | 17 | 17 | 16 | 168 | 45 | 1.45% | -0.59% | `{'SL-Reentry': 29, 'Zero-Initial-Reentry': 16}` |
| `power0_fixed_1p30` | `2026-02` | `BTCUSDT` | 15 | 15 | 15 | 162 | 42 | 5.58% | -0.15% | `{'SL-Reentry': 27, 'Zero-Initial-Reentry': 15}` |
| `power0_fixed_1p30` | `2026-02` | `ETHUSDT` | 12 | 12 | 12 | 154 | 30 | 8.12% | -0.03% | `{'SL-Reentry': 18, 'Zero-Initial-Reentry': 12}` |
| `power0_fixed_1p30` | `2026-03` | `ETHUSDT` | 11 | 11 | 11 | 153 | 24 | 4.27% | -0.03% | `{'SL-Reentry': 13, 'Zero-Initial-Reentry': 11}` |
| `quality_edge_return_mult_1p20_cap_1p80` | `2025-06` | `ETHUSDT` | 3 | 3 | 3 | 179 | 6 | 0.50% | -0.02% | `{'Zero-Initial-Reentry': 3, 'SL-Reentry': 3}` |
| `quality_edge_return_mult_1p20_cap_1p80` | `2025-07` | `BTCUSDT` | 19 | 19 | 18 | 162 | 62 | 2.46% | -0.24% | `{'SL-Reentry': 44, 'Zero-Initial-Reentry': 18}` |
| `quality_edge_return_mult_1p20_cap_1p80` | `2025-08` | `BTCUSDT` | 5 | 5 | 5 | 175 | 18 | -0.44% | -0.44% | `{'SL-Reentry': 13, 'Zero-Initial-Reentry': 5}` |
| `quality_edge_return_mult_1p20_cap_1p80` | `2025-09` | `ETHUSDT` | 5 | 5 | 5 | 198 | 11 | 0.97% | -0.05% | `{'SL-Reentry': 6, 'Zero-Initial-Reentry': 5}` |
| `quality_edge_return_mult_1p20_cap_1p80` | `2025-11` | `BTCUSDT` | 6 | 6 | 6 | 197 | 12 | 1.71% | -0.04% | `{'Zero-Initial-Reentry': 6, 'SL-Reentry': 6}` |
| `quality_edge_return_mult_1p20_cap_1p80` | `2025-12` | `ETHUSDT` | 25 | 25 | 25 | 160 | 60 | 8.98% | -0.27% | `{'SL-Reentry': 35, 'Zero-Initial-Reentry': 25}` |
| `quality_edge_return_mult_1p20_cap_1p80` | `2026-01` | `BTCUSDT` | 17 | 17 | 16 | 168 | 45 | 0.92% | -0.56% | `{'SL-Reentry': 29, 'Zero-Initial-Reentry': 16}` |
| `quality_edge_return_mult_1p20_cap_1p80` | `2026-02` | `BTCUSDT` | 15 | 15 | 15 | 162 | 42 | 5.20% | -0.15% | `{'SL-Reentry': 27, 'Zero-Initial-Reentry': 15}` |
| `quality_edge_return_mult_1p20_cap_1p80` | `2026-02` | `ETHUSDT` | 12 | 12 | 12 | 154 | 30 | 8.39% | -0.03% | `{'SL-Reentry': 18, 'Zero-Initial-Reentry': 12}` |
| `quality_edge_return_mult_1p20_cap_1p80` | `2026-03` | `ETHUSDT` | 11 | 11 | 11 | 153 | 24 | 4.72% | -0.04% | `{'SL-Reentry': 13, 'Zero-Initial-Reentry': 11}` |

## Read

这轮复测解决的是上一阶段的最大 caveat：不再只看 one-shot 1s execution，而是让概率模型先决定哪些 breakout lock 可以进入完整 reentry-window 生命周期。

结果仍然是 research post-selection，尚未做 cross-year / cross-asset 外推；不能视为实盘候选。
