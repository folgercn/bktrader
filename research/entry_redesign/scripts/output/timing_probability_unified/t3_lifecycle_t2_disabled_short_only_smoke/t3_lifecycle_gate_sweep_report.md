# T3 Lifecycle Gate Sweep

Research-only full reentry-window lifecycle sweep. Missing symbol-months are counted as 0.0.

- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- Disable original_t2: `True`
- T3 exit overrides: `{"min_hold_seconds_before_sl": 3600.0}`
- Months: 2026-02
- Symbols: ETHUSDT

## Candidate Summary

| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T2 Trades | T3 Trades | T3 Net PnL | T3 Rejects | Filters |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `pre900` | 0.280000% | 0.280000% | 0.280000% | 0 | 7 | 0 | 7 | 0.560720% | 50 | `{"max_pre_touch_seconds": 900.0}` |
| `pre900_short_only` | 0.000000% | 0.000000% | 0.000000% | 0 | 0 | 0 | 0 | 0.000000% | 51 | `{"allowed_sides": ["short"], "max_pre_touch_seconds": 900.0}` |

## Delta Vs Baseline

| Candidate | Calendar Delta | Worst Silo Delta | Trade Delta | T3 Trade Delta | T3 PnL Delta | T3 Reject Delta | Neg Silo Delta |
|---|---:|---:|---:|---:|---:|---:|---:|
| `pre900_short_only` vs `pre900` | -0.280000% | -0.280000% | -7 | -7 | -0.560720% | +1 | +0 |

## Silo Detail

| Candidate | Symbol | Month | Return | Trades | T2 Trades | T3 Trades | T3 PnL | T3 Rejects | Reject Reasons |
|---|---|---|---:|---:|---:|---:|---:|---:|---|
| `pre900` | `ETHUSDT` | 2026-02 | 0.280000% | 7 | 0 | 7 | 0.560720% | 50 | `{"long": {"pre_touch_seconds": 28}, "short": {"pre_touch_seconds": 22}}` |
| `pre900_short_only` | `ETHUSDT` | 2026-02 | 0.000000% | 0 | 0 | 0 | 0.000000% | 51 | `{"long": {"side": 29}, "short": {"pre_touch_seconds": 22}}` |