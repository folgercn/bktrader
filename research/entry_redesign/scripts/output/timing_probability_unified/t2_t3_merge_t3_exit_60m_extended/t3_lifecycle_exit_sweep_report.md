# T3 Lifecycle Exit Sweep

Research-only full reentry-window lifecycle sweep. Overrides apply only to T3 positions.

- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- Months: 2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04
- Symbols: ETHUSDT, BTCUSDT

## Candidate Summary

| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T3 Trades | T3 Net PnL | T3 Win Rate | T3 Exit Reasons | Overrides |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `t3_min_hold_sl_60m` | -24.480000% | -1.112727% | -2.150000% | 22 | 610 | 100 | 3.845840% | 47.00% | `{"FinalMarkToMarket": 1, "SL": 99}` | `{"min_hold_seconds_before_sl": 3600.0}` |

## Silo Detail

| Candidate | Symbol | Month | Return | Trades | T3 Trades | T3 Net PnL | T3 Win Rate | T3 Exit Reasons |
|---|---|---|---:|---:|---:|---:|---:|---|
| `t3_min_hold_sl_60m` | `ETHUSDT` | 2025-06 | -1.250000% | 37 | 4 | -0.015850% | 25.00% | `{"SL": 4}` |
| `t3_min_hold_sl_60m` | `ETHUSDT` | 2025-07 | -1.250000% | 47 | 8 | 0.654450% | 75.00% | `{"SL": 8}` |
| `t3_min_hold_sl_60m` | `ETHUSDT` | 2025-08 | -0.530000% | 18 | 1 | 0.168700% | 100.00% | `{"SL": 1}` |
| `t3_min_hold_sl_60m` | `ETHUSDT` | 2025-09 | -0.270000% | 31 | 8 | 1.089100% | 75.00% | `{"SL": 8}` |
| `t3_min_hold_sl_60m` | `ETHUSDT` | 2025-10 | -0.390000% | 24 | 3 | 0.411930% | 100.00% | `{"SL": 3}` |
| `t3_min_hold_sl_60m` | `ETHUSDT` | 2025-11 | -1.790000% | 36 | 2 | -0.046540% | 0.00% | `{"SL": 2}` |
| `t3_min_hold_sl_60m` | `ETHUSDT` | 2025-12 | -1.720000% | 40 | 4 | -0.069590% | 0.00% | `{"SL": 4}` |
| `t3_min_hold_sl_60m` | `ETHUSDT` | 2026-01 | -1.150000% | 25 | 3 | 0.012700% | 33.33% | `{"SL": 3}` |
| `t3_min_hold_sl_60m` | `ETHUSDT` | 2026-02 | -0.360000% | 32 | 7 | 0.559390% | 57.14% | `{"SL": 7}` |
| `t3_min_hold_sl_60m` | `ETHUSDT` | 2026-03 | -0.650000% | 29 | 9 | 0.643810% | 33.33% | `{"SL": 9}` |
| `t3_min_hold_sl_60m` | `ETHUSDT` | 2026-04 | -0.970000% | 22 | 2 | -0.034720% | 0.00% | `{"SL": 2}` |
| `t3_min_hold_sl_60m` | `BTCUSDT` | 2025-06 | -1.250000% | 23 | 4 | -0.041900% | 25.00% | `{"SL": 4}` |
| `t3_min_hold_sl_60m` | `BTCUSDT` | 2025-07 | -0.660000% | 15 | 3 | 0.025060% | 33.33% | `{"SL": 3}` |
| `t3_min_hold_sl_60m` | `BTCUSDT` | 2025-08 | -0.550000% | 11 | 5 | -0.041470% | 20.00% | `{"SL": 5}` |
| `t3_min_hold_sl_60m` | `BTCUSDT` | 2025-09 | -1.390000% | 29 | 5 | -0.066750% | 40.00% | `{"SL": 5}` |
| `t3_min_hold_sl_60m` | `BTCUSDT` | 2025-10 | -1.610000% | 30 | 3 | 0.206560% | 66.67% | `{"SL": 3}` |
| `t3_min_hold_sl_60m` | `BTCUSDT` | 2025-11 | -1.070000% | 21 | 3 | 0.215040% | 66.67% | `{"FinalMarkToMarket": 1, "SL": 2}` |
| `t3_min_hold_sl_60m` | `BTCUSDT` | 2025-12 | -2.150000% | 39 | 5 | -0.158530% | 20.00% | `{"SL": 5}` |
| `t3_min_hold_sl_60m` | `BTCUSDT` | 2026-01 | -1.630000% | 29 | 1 | 0.046950% | 100.00% | `{"SL": 1}` |
| `t3_min_hold_sl_60m` | `BTCUSDT` | 2026-02 | -0.880000% | 16 | 3 | 0.213210% | 100.00% | `{"SL": 3}` |
| `t3_min_hold_sl_60m` | `BTCUSDT` | 2026-03 | -1.170000% | 26 | 13 | -0.007260% | 46.15% | `{"SL": 13}` |
| `t3_min_hold_sl_60m` | `BTCUSDT` | 2026-04 | -1.790000% | 30 | 4 | 0.081550% | 50.00% | `{"SL": 4}` |