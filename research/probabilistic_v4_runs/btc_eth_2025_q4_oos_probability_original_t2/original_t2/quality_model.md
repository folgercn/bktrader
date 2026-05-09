# Probabilistic V4 Probability Model

范围：仅限 `research`。概率模型只输出 post-touch continuation probability / EV，不直接决定仓位或 live 语义。

- Selection scope: `global`
- Split: `train_end=2025-11-30T23:59:59+00:00`

## Selected Threshold

- `prob_min`: `0.55`
- `ev_atr_min`: `0.05`

| Events | Success | Net Edge ATR | Avg Prob | Avg Prob EV ATR |
|---:|---:|---:|---:|---:|
| 26 | 61.5385% | 0.097776 | 0.656934 | 0.129067 |

## Top Coefficients

| Feature | Weight |
|---|---:|
| `pullback_60s_atr` | -0.92953526 |
| `dwell_5s_pass` | -0.34344043 |
| `dwell_30s_pass` | 0.31389279 |
| `speed_15s_atr` | 0.29137734 |
| `speed_60s_atr` | 0.24178785 |
| `markov_llr` | 0.18510214 |
| `close_pos_300s` | 0.14195533 |
| `pullback_15s_atr` | 0.09757637 |
| `eff_60s` | 0.06786863 |
| `pullback_30s_atr` | -0.06785613 |
| `flow_ratio_120s` | -0.05493078 |
| `dwell_15s_pass` | 0.05204329 |

## Calibration

| Bin | Events | Avg Prob | Realized Success | Net Edge ATR |
|---|---:|---:|---:|---:|
| `(-0.001, 0.2]` | 60 | 0.125366 | 11.6667% | -0.293384 |
| `(0.2, 0.4]` | 131 | 0.301871 | 35.1145% | -0.109104 |
| `(0.4, 0.6]` | 131 | 0.488230 | 35.8779% | -0.120631 |
| `(0.6, 0.8]` | 19 | 0.682949 | 68.4211% | 0.078322 |
| `(0.8, 1.0]` | 2 | 0.906342 | 50.0000% | -0.019484 |
