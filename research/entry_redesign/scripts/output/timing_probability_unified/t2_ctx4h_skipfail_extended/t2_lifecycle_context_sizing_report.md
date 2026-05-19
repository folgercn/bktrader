# T2 Lifecycle Context Sizing

Research-only strict lifecycle bridge for original_t2 context sizing. Filters scale T2 order size instead of rejecting locks.

- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- Months: 2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04
- Symbols: ETHUSDT, BTCUSDT

## Candidate Summary

| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T2 Trades | T2 Net PnL | T3 Net PnL | Size Fails | Filters | Fail Mult | Fail Action | T3 Overrides |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---|---|
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | -7.900000% | -0.359091% | -0.740000% | 19 | 267 | 163 | -2.383800% | 3.983430% | 2958 | `{"ctx_return_lookback_bars": 4, "max_atr_percentile": 40.0, "min_ctx_side_return_atr": 0.0}` | 0.00 | `skip_lock` | `{"min_hold_seconds_before_sl": 3600.0}` |

## T2 Size Multiplier Attribution

| Candidate | Bucket | Trades | Avg Mult | Gross PnL | Fee | Net After Fee | Notional |
|---|---|---:|---:|---:|---:|---:|---:|
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `pass_full_or_unfiltered` | 163 | 1.000000 | -2.383797% | 5.329318% | -7.713116% | 2664.659698% |

## Silo Detail

| Candidate | Symbol | Month | Return | Trades | T2 Trades | T2 PnL | T3 PnL | Size Fails | Size Reasons |
|---|---|---|---:|---:|---:|---:|---:|---:|---|
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-06 | -0.680000% | 16 | 12 | -0.143900% | -0.015830% | 129 | `{"atr_percentile_high": 90, "ctx_side_return_atr": 39}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-07 | -0.090000% | 18 | 10 | -0.128370% | 0.657770% | 146 | `{"atr_percentile_high": 117, "ctx_side_return_atr": 29}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-08 | -0.200000% | 8 | 6 | -0.087980% | 0.151730% | 140 | `{"atr_percentile_high": 108, "ctx_side_return_atr": 32}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-09 | 0.600000% | 12 | 4 | -0.046110% | 1.092370% | 159 | `{"atr_percentile_high": 111, "ctx_side_return_atr": 48}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-10 | 0.120000% | 7 | 4 | -0.054770% | 0.412940% | 150 | `{"atr_percentile_high": 98, "ctx_side_return_atr": 52}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-11 | -0.680000% | 14 | 12 | -0.172960% | -0.046700% | 153 | `{"atr_percentile_high": 110, "ctx_side_return_atr": 43}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-12 | -0.720000% | 16 | 11 | -0.098260% | -0.085750% | 133 | `{"atr_percentile_high": 90, "ctx_side_return_atr": 43}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-01 | -0.190000% | 5 | 2 | -0.023470% | 0.012570% | 143 | `{"atr_percentile_high": 112, "ctx_side_return_atr": 31}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-02 | -0.400000% | 22 | 15 | -0.216990% | 0.558520% | 110 | `{"atr_percentile_high": 61, "ctx_side_return_atr": 49}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-03 | 0.000000% | 15 | 6 | -0.078630% | 0.643880% | 115 | `{"atr_percentile_high": 74, "ctx_side_return_atr": 41}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-04 | -0.360000% | 8 | 6 | -0.061780% | -0.034790% | 130 | `{"atr_percentile_high": 95, "ctx_side_return_atr": 35}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-06 | -0.590000% | 12 | 8 | -0.111140% | -0.042310% | 118 | `{"atr_percentile_high": 81, "ctx_side_return_atr": 37}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-07 | -0.500000% | 11 | 8 | -0.126780% | 0.025030% | 130 | `{"atr_percentile_high": 88, "ctx_side_return_atr": 42}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-08 | -0.320000% | 6 | 1 | -0.034570% | -0.041470% | 136 | `{"atr_percentile_high": 103, "ctx_side_return_atr": 33}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-09 | -0.680000% | 14 | 9 | -0.117400% | -0.066750% | 127 | `{"atr_percentile_high": 89, "ctx_side_return_atr": 38}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-10 | -0.610000% | 15 | 11 | -0.315370% | 0.261950% | 139 | `{"atr_percentile_high": 101, "ctx_side_return_atr": 38}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-11 | -0.090000% | 7 | 4 | -0.044690% | 0.217660% | 154 | `{"atr_percentile_high": 113, "ctx_side_return_atr": 41}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-12 | -0.740000% | 15 | 10 | -0.004150% | -0.159350% | 124 | `{"atr_percentile_high": 82, "ctx_side_return_atr": 42}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-01 | -0.260000% | 6 | 5 | -0.106350% | 0.047130% | 137 | `{"atr_percentile_high": 109, "ctx_side_return_atr": 28}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-02 | -0.160000% | 8 | 5 | -0.097670% | 0.214010% | 118 | `{"atr_percentile_high": 80, "ctx_side_return_atr": 38}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-03 | -0.710000% | 18 | 4 | -0.086700% | 0.098140% | 132 | `{"atr_percentile_high": 84, "ctx_side_return_atr": 48}` |
| `original_t2_ctx4h_skipfail_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-04 | -0.640000% | 14 | 10 | -0.225760% | 0.082680% | 135 | `{"atr_percentile_high": 95, "ctx_side_return_atr": 40}` |