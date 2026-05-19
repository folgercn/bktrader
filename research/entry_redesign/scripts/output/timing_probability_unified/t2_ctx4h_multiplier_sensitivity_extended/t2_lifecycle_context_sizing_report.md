# T2 Lifecycle Context Sizing

Research-only strict lifecycle bridge for original_t2 context sizing. Filters scale T2 order size instead of rejecting locks.

- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- Months: 2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04
- Symbols: ETHUSDT, BTCUSDT

## Candidate Summary

| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T2 Trades | T2 Net PnL | T3 Net PnL | Size Fails | Filters | Fail Mult | T3 Overrides |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---|
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | -9.130000% | -0.415000% | -0.880000% | 20 | 610 | 510 | -2.720670% | 3.856350% | 2895 | `{"ctx_return_lookback_bars": 4, "max_atr_percentile": 40.0, "min_ctx_side_return_atr": 0.0}` | 0.10 | `{"min_hold_seconds_before_sl": 3600.0}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | -7.410000% | -0.336818% | -0.740000% | 19 | 255 | 510 | -2.191290% | 3.857520% | 2895 | `{"ctx_return_lookback_bars": 4, "max_atr_percentile": 40.0, "min_ctx_side_return_atr": 0.0}` | 0.00 | `{"min_hold_seconds_before_sl": 3600.0}` |

## Delta Vs Baseline

| Candidate | Calendar Delta | Worst Silo Delta | Trade Delta | T2 Trade Delta | T2 PnL Delta | Neg Silo Delta |
|---|---:|---:|---:|---:|---:|---:|
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` vs `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | +1.720000% | +0.140000% | -355 | +0 | +0.529380% | -1 |

## T2 Size Multiplier Attribution

| Candidate | Bucket | Trades | Avg Mult | Gross PnL | Fee | Net After Fee | Notional |
|---|---|---:|---:|---:|---:|---:|---:|
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `fail_scaled_0.10` | 355 | 0.100000 | -0.530647% | 1.188129% | -1.718776% | 594.065124% |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `pass_full_or_unfiltered` | 155 | 1.000000 | -2.190017% | 5.067947% | -7.257968% | 2533.974249% |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `fail_zero` | 355 | 0.000000 | 0.000000% | 0.000000% | 0.000000% | 0.000000% |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `pass_full_or_unfiltered` | 155 | 1.000000 | -2.191278% | 5.070651% | -7.261931% | 2535.326443% |

## Silo Detail

| Candidate | Symbol | Month | Return | Trades | T2 Trades | T2 PnL | T3 PnL | Size Fails | Size Reasons |
|---|---|---|---:|---:|---:|---:|---:|---:|---|
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-06 | -0.740000% | 37 | 33 | -0.133950% | -0.015830% | 127 | `{"atr_percentile_high": 89, "ctx_side_return_atr": 38}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-07 | -0.130000% | 47 | 39 | -0.132930% | 0.657710% | 144 | `{"atr_percentile_high": 115, "ctx_side_return_atr": 29}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-08 | -0.180000% | 18 | 17 | -0.093490% | 0.169030% | 139 | `{"atr_percentile_high": 107, "ctx_side_return_atr": 32}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-09 | 0.520000% | 31 | 23 | -0.073110% | 1.092040% | 157 | `{"atr_percentile_high": 109, "ctx_side_return_atr": 48}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-10 | 0.070000% | 24 | 21 | -0.049660% | 0.412840% | 146 | `{"atr_percentile_high": 96, "ctx_side_return_atr": 50}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-11 | -0.790000% | 36 | 34 | -0.215650% | -0.046680% | 152 | `{"atr_percentile_high": 109, "ctx_side_return_atr": 43}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-12 | -0.770000% | 40 | 36 | -0.124430% | -0.070100% | 128 | `{"atr_percentile_high": 86, "ctx_side_return_atr": 42}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-01 | -0.290000% | 25 | 22 | -0.053540% | 0.012580% | 139 | `{"atr_percentile_high": 108, "ctx_side_return_atr": 31}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-02 | -0.310000% | 32 | 25 | -0.148840% | 0.559090% | 109 | `{"atr_percentile_high": 60, "ctx_side_return_atr": 49}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-03 | -0.010000% | 29 | 20 | -0.086730% | 0.643880% | 114 | `{"atr_percentile_high": 74, "ctx_side_return_atr": 40}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-04 | -0.420000% | 22 | 20 | -0.081260% | -0.034780% | 128 | `{"atr_percentile_high": 94, "ctx_side_return_atr": 34}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-06 | -0.660000% | 23 | 19 | -0.135270% | -0.042270% | 116 | `{"atr_percentile_high": 79, "ctx_side_return_atr": 37}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-07 | -0.520000% | 15 | 12 | -0.129010% | 0.025030% | 129 | `{"atr_percentile_high": 88, "ctx_side_return_atr": 41}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-08 | -0.340000% | 11 | 6 | -0.038120% | -0.041470% | 135 | `{"atr_percentile_high": 102, "ctx_side_return_atr": 33}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-09 | -0.750000% | 29 | 24 | -0.136450% | -0.066750% | 124 | `{"atr_percentile_high": 86, "ctx_side_return_atr": 38}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-10 | -0.600000% | 30 | 27 | -0.289290% | 0.207470% | 133 | `{"atr_percentile_high": 96, "ctx_side_return_atr": 37}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-11 | -0.190000% | 21 | 18 | -0.093460% | 0.217400% | 150 | `{"atr_percentile_high": 109, "ctx_side_return_atr": 41}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-12 | -0.880000% | 39 | 34 | -0.064760% | -0.159270% | 118 | `{"atr_percentile_high": 78, "ctx_side_return_atr": 40}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-01 | -0.400000% | 29 | 28 | -0.162100% | 0.047110% | 131 | `{"atr_percentile_high": 104, "ctx_side_return_atr": 27}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-02 | -0.240000% | 16 | 13 | -0.139920% | 0.213930% | 115 | `{"atr_percentile_high": 77, "ctx_side_return_atr": 38}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-03 | -0.740000% | 26 | 13 | -0.054640% | -0.007180% | 130 | `{"atr_percentile_high": 82, "ctx_side_return_atr": 48}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-04 | -0.760000% | 30 | 26 | -0.284060% | 0.082570% | 131 | `{"atr_percentile_high": 91, "ctx_side_return_atr": 40}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-06 | -0.680000% | 16 | 33 | -0.143900% | -0.015830% | 127 | `{"atr_percentile_high": 89, "ctx_side_return_atr": 38}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-07 | -0.010000% | 16 | 39 | -0.103920% | 0.658080% | 144 | `{"atr_percentile_high": 115, "ctx_side_return_atr": 29}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-08 | -0.140000% | 7 | 17 | -0.088030% | 0.169060% | 139 | `{"atr_percentile_high": 107, "ctx_side_return_atr": 32}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-09 | 0.600000% | 12 | 23 | -0.046110% | 1.092370% | 157 | `{"atr_percentile_high": 109, "ctx_side_return_atr": 48}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-10 | 0.120000% | 7 | 21 | -0.054770% | 0.412940% | 146 | `{"atr_percentile_high": 96, "ctx_side_return_atr": 50}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-11 | -0.680000% | 14 | 34 | -0.172960% | -0.046700% | 152 | `{"atr_percentile_high": 109, "ctx_side_return_atr": 43}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `ETHUSDT` | 2025-12 | -0.670000% | 15 | 36 | -0.098320% | -0.070160% | 128 | `{"atr_percentile_high": 86, "ctx_side_return_atr": 42}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-01 | -0.190000% | 5 | 22 | -0.023470% | 0.012570% | 139 | `{"atr_percentile_high": 108, "ctx_side_return_atr": 31}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-02 | -0.300000% | 20 | 25 | -0.181390% | 0.559060% | 109 | `{"atr_percentile_high": 60, "ctx_side_return_atr": 49}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-03 | 0.060000% | 14 | 20 | -0.063170% | 0.643890% | 114 | `{"atr_percentile_high": 74, "ctx_side_return_atr": 40}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-04 | -0.360000% | 8 | 20 | -0.061780% | -0.034790% | 128 | `{"atr_percentile_high": 94, "ctx_side_return_atr": 34}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-06 | -0.590000% | 12 | 19 | -0.111140% | -0.042310% | 116 | `{"atr_percentile_high": 79, "ctx_side_return_atr": 37}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-07 | -0.500000% | 11 | 12 | -0.126780% | 0.025030% | 129 | `{"atr_percentile_high": 88, "ctx_side_return_atr": 41}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-08 | -0.320000% | 6 | 6 | -0.034570% | -0.041470% | 135 | `{"atr_percentile_high": 102, "ctx_side_return_atr": 33}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-09 | -0.680000% | 14 | 24 | -0.117400% | -0.066750% | 124 | `{"atr_percentile_high": 86, "ctx_side_return_atr": 38}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-10 | -0.490000% | 12 | 27 | -0.238170% | 0.207570% | 133 | `{"atr_percentile_high": 96, "ctx_side_return_atr": 37}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-11 | -0.090000% | 7 | 18 | -0.044690% | 0.217660% | 150 | `{"atr_percentile_high": 109, "ctx_side_return_atr": 41}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `BTCUSDT` | 2025-12 | -0.740000% | 15 | 34 | -0.004150% | -0.159350% | 118 | `{"atr_percentile_high": 78, "ctx_side_return_atr": 40}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-01 | -0.260000% | 6 | 28 | -0.106350% | 0.047130% | 131 | `{"atr_percentile_high": 104, "ctx_side_return_atr": 27}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-02 | -0.160000% | 8 | 13 | -0.097670% | 0.214010% | 115 | `{"atr_percentile_high": 77, "ctx_side_return_atr": 38}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-03 | -0.690000% | 16 | 13 | -0.046790% | -0.007170% | 130 | `{"atr_percentile_high": 82, "ctx_side_return_atr": 48}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-04 | -0.640000% | 14 | 26 | -0.225760% | 0.082680% | 131 | `{"atr_percentile_high": 91, "ctx_side_return_atr": 40}` |