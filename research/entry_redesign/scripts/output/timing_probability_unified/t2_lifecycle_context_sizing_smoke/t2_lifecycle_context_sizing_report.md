# T2 Lifecycle Context Sizing

Research-only strict lifecycle bridge for original_t2 context sizing. Filters scale T2 order size instead of rejecting locks.

- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- Months: 2026-04
- Symbols: ETHUSDT

## Candidate Summary

| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T2 Trades | T2 Net PnL | T3 Net PnL | Size Fails | Filters | Fail Mult |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|
| `strict_baseline` | -1.020000% | -1.020000% | -1.020000% | 1 | 24 | 20 | -0.255950% | -0.052060% | 0 | `{}` | 1.00 |
| `original_t2_ctx4h_scaled025` | -0.570000% | -0.570000% | -0.570000% | 1 | 24 | 20 | -0.110400% | -0.052140% | 128 | `{"ctx_return_lookback_bars": 4, "max_atr_percentile": 40.0, "min_ctx_side_return_atr": 0.0}` | 0.25 |

## Delta Vs Baseline

| Candidate | Calendar Delta | Worst Silo Delta | Trade Delta | T2 Trade Delta | T2 PnL Delta | Neg Silo Delta |
|---|---:|---:|---:|---:|---:|---:|
| `original_t2_ctx4h_scaled025` vs `strict_baseline` | +0.450000% | +0.450000% | +0 | +0 | +0.145550% | +0 |

## Silo Detail

| Candidate | Symbol | Month | Return | Trades | T2 Trades | T2 PnL | T3 PnL | Size Fails | Size Reasons |
|---|---|---|---:|---:|---:|---:|---:|---:|---|
| `strict_baseline` | `ETHUSDT` | 2026-04 | -1.020000% | 24 | 20 | -0.255950% | -0.052060% | 0 | `{}` |
| `original_t2_ctx4h_scaled025` | `ETHUSDT` | 2026-04 | -0.570000% | 24 | 20 | -0.110400% | -0.052140% | 128 | `{"atr_percentile_high": 94, "ctx_side_return_atr": 34}` |