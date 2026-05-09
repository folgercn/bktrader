# Probabilistic V4 Probability Model

范围：仅限 `research`。概率模型只输出 post-touch continuation probability / EV，不直接决定仓位或 live 语义。

- Selection scope: `global`
- Split: `train_ratio=0.7, split_time=2026-03-05T04:28:17+00:00`

## Selected Threshold

- `prob_min`: `0.55`
- `ev_atr_min`: `0.0`

| Events | Success | Net Edge ATR | Avg Prob | Avg Prob EV ATR |
|---:|---:|---:|---:|---:|
| 47 | 72.3404% | 0.173923 | 0.659274 | 0.132974 |

## Top Coefficients

| Feature | Weight |
|---|---:|
| `pullback_60s_atr` | -0.94468295 |
| `pullback_15s_atr` | 0.43767148 |
| `flow_ratio_60s` | -0.29899020 |
| `dwell_30s_pass` | 0.29598753 |
| `close_pos_300s` | -0.26088055 |
| `pullback_30s_atr` | -0.25348259 |
| `speed_60s_atr` | 0.19453372 |
| `markov_llr` | 0.16280987 |
| `speed_300s_atr` | 0.12609126 |
| `markov_prob_success` | 0.07757358 |
| `dwell_15s_pass` | 0.07670633 |
| `dwell_5s_pass` | -0.07499959 |

## Calibration

| Bin | Events | Avg Prob | Realized Success | Net Edge ATR |
|---|---:|---:|---:|---:|
| `(-0.001, 0.2]` | 43 | 0.112031 | 6.9767% | -0.301297 |
| `(0.2, 0.4]` | 141 | 0.293453 | 25.5319% | -0.141831 |
| `(0.4, 0.6]` | 78 | 0.496287 | 51.2821% | 0.026227 |
| `(0.6, 0.8]` | 32 | 0.662665 | 62.5000% | 0.095881 |
| `(0.8, 1.0]` | 4 | 0.893263 | 100.0000% | 0.369956 |
