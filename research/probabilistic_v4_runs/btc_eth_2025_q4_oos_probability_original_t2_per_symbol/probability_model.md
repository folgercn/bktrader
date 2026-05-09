# Probabilistic V4 Probability Model

范围：仅限 `research`。概率模型只输出 post-touch continuation probability / EV，不直接决定仓位或 live 语义。

- Selection scope: `per_symbol`
- Split: `train_end=2025-11-30T23:59:59+00:00`

## Selected Thresholds By Symbol

| Symbol | Events | Prob Min | EV Min | Success | Net Edge ATR | Avg Prob | Avg Prob EV ATR |
|---|---:|---:|---:|---:|---:|---:|---:|
| `BTCUSDT` | 27 | 0.4 | 0.0 | 37.0370% | -0.039986 | 0.579386 | 0.068900 |
| `ETHUSDT` | 15 | 0.55 | 0.1 | 73.3333% | 0.219164 | 0.639301 | 0.145213 |

## Top Coefficients By Symbol

### BTCUSDT

| Feature | Weight |
|---|---:|
| `pullback_60s_atr` | -0.89536228 |
| `dwell_30s_pass` | 0.48270451 |
| `dwell_5s_pass` | -0.38766585 |
| `speed_60s_atr` | 0.33385839 |
| `markov_llr` | 0.26577494 |
| `flow_ratio_60s` | 0.19154366 |
| `flow_ratio_120s` | -0.18139779 |
| `speed_15s_atr` | 0.14846788 |
| `speed_300s_atr` | -0.14303599 |
| `close_pos_60s` | -0.08865305 |
| `pullback_30s_atr` | 0.06982861 |
| `eff_300s` | 0.06976021 |

### ETHUSDT

| Feature | Weight |
|---|---:|
| `pullback_60s_atr` | -0.99344705 |
| `speed_15s_atr` | 0.45083008 |
| `dwell_5s_pass` | -0.29008936 |
| `markov_llr` | 0.24663005 |
| `close_pos_300s` | 0.22583040 |
| `pullback_30s_atr` | -0.19379024 |
| `dwell_15s_pass` | 0.18848455 |
| `eff_300s` | -0.14786186 |
| `pullback_15s_atr` | 0.14080075 |
| `flow_ratio_60s` | -0.12520348 |
| `speed_60s_atr` | 0.11588992 |
| `speed_300s_atr` | 0.10491469 |

