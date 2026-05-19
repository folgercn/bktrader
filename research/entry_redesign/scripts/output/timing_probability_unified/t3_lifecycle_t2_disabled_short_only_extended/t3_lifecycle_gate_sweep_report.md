# T3 Lifecycle Gate Sweep

Research-only full reentry-window lifecycle sweep. Missing symbol-months are counted as 0.0.

- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- Disable original_t2: `True`
- T3 exit overrides: `{"min_hold_seconds_before_sl": 3600.0}`
- Months: 2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04
- Symbols: ETHUSDT, BTCUSDT

## Candidate Summary

| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T2 Trades | T3 Trades | T3 Net PnL | T3 Rejects | Filters |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `pre900_short_only` | 0.460000% | 0.020909% | -0.090000% | 9 | 33 | 0 | 33 | 1.772430% | 1028 | `{"allowed_sides": ["short"], "max_pre_touch_seconds": 900.0}` |

## Silo Detail

| Candidate | Symbol | Month | Return | Trades | T2 Trades | T3 Trades | T3 PnL | T3 Rejects | Reject Reasons |
|---|---|---|---:|---:|---:|---:|---:|---:|---|
| `pre900_short_only` | `ETHUSDT` | 2025-06 | -0.050000% | 2 | 0 | 2 | 0.025910% | 49 | `{"long": {"side": 27}, "short": {"pre_touch_seconds": 22}}` |
| `pre900_short_only` | `ETHUSDT` | 2025-07 | 0.060000% | 3 | 0 | 3 | 0.182810% | 54 | `{"long": {"side": 35}, "short": {"pre_touch_seconds": 19}}` |
| `pre900_short_only` | `ETHUSDT` | 2025-08 | 0.070000% | 2 | 0 | 2 | 0.151880% | 49 | `{"long": {"side": 23}, "short": {"pre_touch_seconds": 26}}` |
| `pre900_short_only` | `ETHUSDT` | 2025-09 | 0.030000% | 3 | 0 | 3 | 0.147760% | 43 | `{"long": {"side": 19}, "short": {"pre_touch_seconds": 24}}` |
| `pre900_short_only` | `ETHUSDT` | 2025-10 | 0.000000% | 0 | 0 | 0 | 0.000000% | 39 | `{"long": {"side": 21}, "short": {"pre_touch_seconds": 18}}` |
| `pre900_short_only` | `ETHUSDT` | 2025-11 | 0.000000% | 0 | 0 | 0 | 0.000000% | 32 | `{"long": {"side": 13}, "short": {"pre_touch_seconds": 19}}` |
| `pre900_short_only` | `ETHUSDT` | 2025-12 | -0.060000% | 1 | 0 | 1 | -0.015880% | 58 | `{"long": {"side": 33}, "short": {"pre_touch_seconds": 25}}` |
| `pre900_short_only` | `ETHUSDT` | 2026-01 | -0.060000% | 1 | 0 | 1 | -0.016110% | 60 | `{"long": {"side": 37}, "short": {"pre_touch_seconds": 23}}` |
| `pre900_short_only` | `ETHUSDT` | 2026-02 | 0.000000% | 0 | 0 | 0 | 0.000000% | 51 | `{"long": {"side": 29}, "short": {"pre_touch_seconds": 22}}` |
| `pre900_short_only` | `ETHUSDT` | 2026-03 | 0.610000% | 4 | 0 | 4 | 0.765630% | 60 | `{"long": {"side": 30}, "short": {"pre_touch_seconds": 30}}` |
| `pre900_short_only` | `ETHUSDT` | 2026-04 | -0.060000% | 1 | 0 | 1 | -0.017880% | 46 | `{"long": {"side": 22}, "short": {"pre_touch_seconds": 24}}` |
| `pre900_short_only` | `BTCUSDT` | 2025-06 | -0.090000% | 1 | 0 | 1 | -0.050130% | 46 | `{"long": {"side": 23}, "short": {"pre_touch_seconds": 23}}` |
| `pre900_short_only` | `BTCUSDT` | 2025-07 | -0.020000% | 2 | 0 | 2 | 0.062270% | 38 | `{"long": {"side": 20}, "short": {"pre_touch_seconds": 18}}` |
| `pre900_short_only` | `BTCUSDT` | 2025-08 | 0.000000% | 0 | 0 | 0 | 0.000000% | 38 | `{"long": {"side": 20}, "short": {"pre_touch_seconds": 18}}` |
| `pre900_short_only` | `BTCUSDT` | 2025-09 | -0.030000% | 1 | 0 | 1 | 0.007460% | 46 | `{"long": {"side": 25}, "short": {"pre_touch_seconds": 21}}` |
| `pre900_short_only` | `BTCUSDT` | 2025-10 | -0.060000% | 2 | 0 | 2 | 0.017270% | 43 | `{"long": {"side": 20}, "short": {"pre_touch_seconds": 23}}` |
| `pre900_short_only` | `BTCUSDT` | 2025-11 | 0.100000% | 3 | 0 | 3 | 0.218020% | 37 | `{"long": {"side": 16}, "short": {"pre_touch_seconds": 21}}` |
| `pre900_short_only` | `BTCUSDT` | 2025-12 | -0.080000% | 1 | 0 | 1 | -0.041460% | 52 | `{"long": {"side": 27}, "short": {"pre_touch_seconds": 25}}` |
| `pre900_short_only` | `BTCUSDT` | 2026-01 | 0.010000% | 1 | 0 | 1 | 0.047130% | 43 | `{"long": {"side": 18}, "short": {"pre_touch_seconds": 25}}` |
| `pre900_short_only` | `BTCUSDT` | 2026-02 | 0.000000% | 0 | 0 | 0 | 0.000000% | 35 | `{"long": {"side": 17}, "short": {"pre_touch_seconds": 18}}` |
| `pre900_short_only` | `BTCUSDT` | 2026-03 | 0.090000% | 5 | 0 | 5 | 0.287750% | 56 | `{"long": {"side": 28}, "short": {"pre_touch_seconds": 28}}` |
| `pre900_short_only` | `BTCUSDT` | 2026-04 | 0.000000% | 0 | 0 | 0 | 0.000000% | 53 | `{"long": {"side": 27}, "short": {"pre_touch_seconds": 26}}` |