# Probabilistic V4 Probability Model

范围：仅限 `research`。概率模型只输出 post-touch continuation probability / EV，不直接决定仓位或 live 语义。

- Selection scope: `global`
- Split: `train_ratio=0.6, split_time=2026-01-04T23:01:47+00:00`

## Selected Threshold

- `prob_min`: `0.65`
- `ev_atr_min`: `0.0`

| Events | Success | Net Edge ATR | Avg Prob | Avg Prob EV ATR |
|---:|---:|---:|---:|---:|
| 5 | 40.0000% | -0.106355 | 0.816777 | 0.190602 |

## Top Coefficients

| Feature | Weight |
|---|---:|
| `dwell_5s_pass` | -1.59255101 |
| `touch_extension_atr` | 1.49199129 |
| `pullback_15s_atr` | -1.22685976 |
| `markov_llr` | 1.11844706 |
| `markov_prob_success` | 1.11377036 |
| `dwell_30s_pass` | 1.08146172 |
| `eff_60s` | 0.78494018 |
| `close_pos_60s` | -0.69676117 |
| `dwell_15s_pass` | 0.54043047 |
| `roundtrip_cost_atr` | 0.50140700 |
| `pullback_30s_atr` | 0.41050202 |
| `eff_300s` | 0.38562361 |

## Calibration

| Bin | Events | Avg Prob | Realized Success | Net Edge ATR |
|---|---:|---:|---:|---:|
| `(-0.001, 0.2]` | 10 | 0.040712 | 40.0000% | -0.093378 |
| `(0.4, 0.6]` | 3 | 0.463890 | 0.0000% | -0.395208 |
| `(0.6, 0.8]` | 5 | 0.717545 | 20.0000% | -0.270073 |
| `(0.8, 1.0]` | 2 | 0.934894 | 50.0000% | -0.029651 |
