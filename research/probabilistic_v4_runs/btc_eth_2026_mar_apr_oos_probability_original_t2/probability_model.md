# Probabilistic V4 Probability Model

范围：仅限 `research`。概率模型只输出 post-touch continuation probability / EV，不直接决定仓位或 live 语义。

- Selection scope: `global`
- Split: `train_end=2026-02-28T23:59:59+00:00`

## Selected Threshold

- `prob_min`: `0.55`
- `ev_atr_min`: `0.05`

| Events | Success | Net Edge ATR | Avg Prob | Avg Prob EV ATR |
|---:|---:|---:|---:|---:|
| 41 | 70.7317% | 0.176427 | 0.654364 | 0.143605 |

## Top Coefficients

| Feature | Weight |
|---|---:|
| `pullback_60s_atr` | -0.91999401 |
| `pullback_15s_atr` | 0.44644432 |
| `flow_ratio_60s` | -0.32756675 |
| `dwell_30s_pass` | 0.30392701 |
| `speed_60s_atr` | 0.25566228 |
| `pullback_30s_atr` | -0.25160499 |
| `close_pos_300s` | -0.19576601 |
| `markov_llr` | 0.17507632 |
| `speed_300s_atr` | 0.12488164 |
| `eff_300s` | 0.08639516 |
| `dwell_15s_pass` | 0.08390848 |
| `roundtrip_cost_atr` | 0.06320065 |

## Calibration

| Bin | Events | Avg Prob | Realized Success | Net Edge ATR |
|---|---:|---:|---:|---:|
| `(-0.001, 0.2]` | 51 | 0.119028 | 7.8431% | -0.281520 |
| `(0.2, 0.4]` | 157 | 0.294935 | 26.1146% | -0.134439 |
| `(0.4, 0.6]` | 101 | 0.496769 | 50.4950% | 0.026730 |
| `(0.6, 0.8]` | 25 | 0.662116 | 68.0000% | 0.142991 |
| `(0.8, 1.0]` | 4 | 0.864603 | 100.0000% | 0.369956 |
