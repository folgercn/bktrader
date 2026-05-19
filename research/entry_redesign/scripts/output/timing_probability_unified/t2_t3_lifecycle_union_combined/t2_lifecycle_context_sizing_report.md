# T2 Lifecycle Context Sizing

Research-only strict lifecycle bridge for original_t2 context sizing. Filters scale T2 order size instead of rejecting locks.

- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- Months: 2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04
- Symbols: ETHUSDT, BTCUSDT

## Candidate Summary

| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T2 Trades | T2 Net PnL | T3 Net PnL | Size Fails | Filters | Fail Mult | T3 Overrides |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---|
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | -11.680000% | -0.530909% | -1.100000% | 21 | 610 | 510 | -3.513910% | 3.854600% | 2895 | `{"ctx_return_lookback_bars": 4, "max_atr_percentile": 40.0, "min_ctx_side_return_atr": 0.0}` | 0.25 | `{"min_hold_seconds_before_sl": 3600.0}` |

## Silo Detail

| Candidate | Symbol | Month | Return | Trades | T2 Trades | T2 PnL | T3 PnL | Size Fails | Size Reasons |
|---|---|---|---:|---:|---:|---:|---:|---:|---|
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-06 | -0.820000% | 37 | 33 | -0.119080% | -0.015830% | 127 | `{"atr_percentile_high": 89, "ctx_side_return_atr": 38}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-07 | -0.320000% | 47 | 39 | -0.176380% | 0.657170% | 144 | `{"atr_percentile_high": 115, "ctx_side_return_atr": 29}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-08 | -0.240000% | 18 | 17 | -0.101670% | 0.168970% | 139 | `{"atr_percentile_high": 107, "ctx_side_return_atr": 32}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-09 | 0.390000% | 31 | 23 | -0.113570% | 1.091550% | 157 | `{"atr_percentile_high": 109, "ctx_side_return_atr": 48}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-10 | -0.010000% | 24 | 21 | -0.042040% | 0.412680% | 146 | `{"atr_percentile_high": 96, "ctx_side_return_atr": 50}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-11 | -0.960000% | 36 | 34 | -0.279610% | -0.046660% | 152 | `{"atr_percentile_high": 109, "ctx_side_return_atr": 43}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-12 | -0.930000% | 40 | 36 | -0.163540% | -0.070020% | 128 | `{"atr_percentile_high": 86, "ctx_side_return_atr": 42}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-01 | -0.430000% | 25 | 22 | -0.098580% | 0.012600% | 139 | `{"atr_percentile_high": 108, "ctx_side_return_atr": 31}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-02 | -0.320000% | 32 | 25 | -0.100040% | 0.559140% | 109 | `{"atr_percentile_high": 60, "ctx_side_return_atr": 49}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-03 | -0.120000% | 29 | 20 | -0.122050% | 0.643870% | 114 | `{"atr_percentile_high": 74, "ctx_side_return_atr": 40}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-04 | -0.510000% | 22 | 20 | -0.110450% | -0.034770% | 128 | `{"atr_percentile_high": 94, "ctx_side_return_atr": 34}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-06 | -0.760000% | 23 | 19 | -0.171450% | -0.042210% | 116 | `{"atr_percentile_high": 79, "ctx_side_return_atr": 37}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-07 | -0.540000% | 15 | 12 | -0.132370% | 0.025040% | 129 | `{"atr_percentile_high": 88, "ctx_side_return_atr": 41}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-08 | -0.370000% | 11 | 6 | -0.043440% | -0.041470% | 135 | `{"atr_percentile_high": 102, "ctx_side_return_atr": 33}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-09 | -0.860000% | 29 | 24 | -0.165000% | -0.066750% | 124 | `{"atr_percentile_high": 86, "ctx_side_return_atr": 38}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-10 | -0.770000% | 30 | 27 | -0.365890% | 0.207320% | 133 | `{"atr_percentile_high": 96, "ctx_side_return_atr": 37}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-11 | -0.330000% | 21 | 18 | -0.166540% | 0.217010% | 150 | `{"atr_percentile_high": 109, "ctx_side_return_atr": 41}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-12 | -1.100000% | 39 | 34 | -0.155520% | -0.159140% | 118 | `{"atr_percentile_high": 78, "ctx_side_return_atr": 40}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-01 | -0.600000% | 29 | 28 | -0.245610% | 0.047080% | 131 | `{"atr_percentile_high": 104, "ctx_side_return_atr": 27}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-02 | -0.340000% | 16 | 13 | -0.203250% | 0.213810% | 115 | `{"atr_percentile_high": 77, "ctx_side_return_atr": 38}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-03 | -0.810000% | 26 | 13 | -0.066420% | -0.007190% | 130 | `{"atr_percentile_high": 82, "ctx_side_return_atr": 48}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-04 | -0.930000% | 30 | 26 | -0.371410% | 0.082400% | 131 | `{"atr_percentile_high": 91, "ctx_side_return_atr": 40}` |