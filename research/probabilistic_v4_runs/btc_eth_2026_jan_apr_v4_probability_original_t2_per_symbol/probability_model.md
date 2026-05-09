# Probabilistic V4 Probability Model

范围：仅限 `research`。概率模型只输出 post-touch continuation probability / EV，不直接决定仓位或 live 语义。

- Selection scope: `per_symbol`
- Split: `train_ratio=0.7, split_time=2026-03-05T04:28:17+00:00`

## Selected Thresholds By Symbol

| Symbol | Events | Prob Min | EV Min | Success | Net Edge ATR | Avg Prob | Avg Prob EV ATR |
|---|---:|---:|---:|---:|---:|---:|---:|
| `BTCUSDT` | 35 | 0.4 | 0.0 | 62.8571% | 0.105279 | 0.604145 | 0.095071 |
| `ETHUSDT` | 23 | 0.6 | -0.05 | 65.2174% | 0.135252 | 0.681072 | 0.157181 |

## Top Coefficients By Symbol

### BTCUSDT

| Feature | Weight |
|---|---:|
| `pullback_60s_atr` | -0.94742193 |
| `close_pos_300s` | -0.47132907 |
| `pullback_15s_atr` | 0.33103412 |
| `pullback_30s_atr` | -0.30382248 |
| `markov_prob_success` | 0.29382636 |
| `flow_ratio_60s` | -0.26540344 |
| `dwell_15s_pass` | 0.23018975 |
| `speed_60s_atr` | 0.21659717 |
| `dwell_5s_pass` | -0.21626907 |
| `dwell_30s_pass` | 0.16247831 |
| `speed_300s_atr` | 0.16030155 |
| `eff_300s` | 0.15103132 |

### ETHUSDT

| Feature | Weight |
|---|---:|
| `pullback_60s_atr` | -0.95304746 |
| `pullback_15s_atr` | 0.54944839 |
| `dwell_30s_pass` | 0.39942765 |
| `flow_ratio_60s` | -0.38572356 |
| `speed_60s_atr` | 0.20661848 |
| `pullback_30s_atr` | -0.20261155 |
| `roundtrip_cost_atr` | 0.15009171 |
| `speed_300s_atr` | 0.14103145 |
| `markov_llr` | 0.11344152 |
| `speed_15s_atr` | -0.09533725 |
| `flow_ratio_120s` | 0.08162777 |
| `markov_prob_success` | 0.07595085 |

