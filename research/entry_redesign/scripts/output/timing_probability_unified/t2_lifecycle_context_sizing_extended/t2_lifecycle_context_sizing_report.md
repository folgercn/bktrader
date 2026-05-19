# T2 Lifecycle Context Sizing

Research-only strict lifecycle bridge for original_t2 context sizing. Filters scale T2 order size instead of rejecting locks.

- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- Months: 2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04
- Symbols: ETHUSDT, BTCUSDT

## Candidate Summary

| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T2 Trades | T2 Net PnL | T3 Net PnL | Size Fails | Filters | Fail Mult |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|
| `strict_baseline` | -30.980000% | -1.408182% | -2.190000% | 22 | 659 | 516 | -7.621320% | -1.600810% | 0 | `{}` | 1.00 |
| `original_t2_ctx4h_scaled025` | -18.050000% | -0.820455% | -1.290000% | 22 | 659 | 516 | -3.596480% | -1.603750% | 2899 | `{"ctx_return_lookback_bars": 4, "max_atr_percentile": 40.0, "min_ctx_side_return_atr": 0.0}` | 0.25 |

## Delta Vs Baseline

| Candidate | Calendar Delta | Worst Silo Delta | Trade Delta | T2 Trade Delta | T2 PnL Delta | Neg Silo Delta |
|---|---:|---:|---:|---:|---:|---:|
| `original_t2_ctx4h_scaled025` vs `strict_baseline` | +12.930000% | +0.900000% | +0 | +0 | +4.024840% | +0 |

## Silo Detail

| Candidate | Symbol | Month | Return | Trades | T2 Trades | T2 PnL | T3 PnL | Size Fails | Size Reasons |
|---|---|---|---:|---:|---:|---:|---:|---:|---|
| `strict_baseline` | `ETHUSDT` | 2025-06 | -1.290000% | 38 | 33 | -0.045850% | -0.028080% | 0 | `{}` |
| `strict_baseline` | `ETHUSDT` | 2025-07 | -2.120000% | 53 | 39 | -0.390580% | -0.108730% | 0 | `{}` |
| `strict_baseline` | `ETHUSDT` | 2025-08 | -0.820000% | 20 | 17 | -0.142210% | -0.060200% | 0 | `{}` |
| `strict_baseline` | `ETHUSDT` | 2025-09 | -1.400000% | 34 | 23 | -0.312780% | -0.030020% | 0 | `{}` |
| `strict_baseline` | `ETHUSDT` | 2025-10 | -0.960000% | 27 | 23 | -0.043170% | -0.063110% | 0 | `{}` |
| `strict_baseline` | `ETHUSDT` | 2025-11 | -1.830000% | 37 | 34 | -0.597660% | -0.059770% | 0 | `{}` |
| `strict_baseline` | `ETHUSDT` | 2025-12 | -1.800000% | 43 | 36 | -0.358020% | -0.096510% | 0 | `{}` |
| `strict_baseline` | `ETHUSDT` | 2026-01 | -1.280000% | 28 | 22 | -0.322520% | -0.064770% | 0 | `{}` |
| `strict_baseline` | `ETHUSDT` | 2026-02 | -0.750000% | 36 | 25 | 0.143790% | 0.222370% | 0 | `{}` |
| `strict_baseline` | `ETHUSDT` | 2026-03 | -1.760000% | 38 | 20 | -0.295030% | -0.298600% | 0 | `{}` |
| `strict_baseline` | `ETHUSDT` | 2026-04 | -1.020000% | 24 | 20 | -0.255950% | -0.052060% | 0 | `{}` |
| `strict_baseline` | `BTCUSDT` | 2025-06 | -1.380000% | 24 | 19 | -0.351290% | -0.138740% | 0 | `{}` |
| `strict_baseline` | `BTCUSDT` | 2025-07 | -0.860000% | 17 | 12 | -0.149030% | -0.134270% | 0 | `{}` |
| `strict_baseline` | `BTCUSDT` | 2025-08 | -0.670000% | 13 | 8 | -0.138670% | -0.048430% | 0 | `{}` |
| `strict_baseline` | `BTCUSDT` | 2025-09 | -1.390000% | 29 | 24 | -0.307260% | -0.068320% | 0 | `{}` |
| `strict_baseline` | `BTCUSDT` | 2025-10 | -1.770000% | 32 | 27 | -0.746720% | 0.086560% | 0 | `{}` |
| `strict_baseline` | `BTCUSDT` | 2025-11 | -1.490000% | 23 | 18 | -0.529960% | -0.161920% | 0 | `{}` |
| `strict_baseline` | `BTCUSDT` | 2025-12 | -2.190000% | 40 | 34 | -0.606530% | -0.179130% | 0 | `{}` |
| `strict_baseline` | `BTCUSDT` | 2026-01 | -1.630000% | 29 | 28 | -0.661130% | 0.046950% | 0 | `{}` |
| `strict_baseline` | `BTCUSDT` | 2026-02 | -0.990000% | 17 | 13 | -0.519010% | 0.147960% | 0 | `{}` |
| `strict_baseline` | `BTCUSDT` | 2026-03 | -1.640000% | 27 | 15 | -0.185870% | -0.425550% | 0 | `{}` |
| `strict_baseline` | `BTCUSDT` | 2026-04 | -1.940000% | 30 | 26 | -0.805870% | -0.086440% | 0 | `{}` |
| `original_t2_ctx4h_scaled025` | `ETHUSDT` | 2025-06 | -0.850000% | 38 | 33 | -0.119050% | -0.028120% | 127 | `{"atr_percentile_high": 89, "ctx_side_return_atr": 38}` |
| `original_t2_ctx4h_scaled025` | `ETHUSDT` | 2025-07 | -1.200000% | 53 | 39 | -0.175420% | -0.108960% | 144 | `{"atr_percentile_high": 115, "ctx_side_return_atr": 29}` |
| `original_t2_ctx4h_scaled025` | `ETHUSDT` | 2025-08 | -0.530000% | 20 | 17 | -0.101460% | -0.060300% | 139 | `{"atr_percentile_high": 107, "ctx_side_return_atr": 32}` |
| `original_t2_ctx4h_scaled025` | `ETHUSDT` | 2025-09 | -0.750000% | 34 | 23 | -0.112740% | -0.030030% | 158 | `{"atr_percentile_high": 110, "ctx_side_return_atr": 48}` |
| `original_t2_ctx4h_scaled025` | `ETHUSDT` | 2025-10 | -0.510000% | 27 | 23 | -0.051560% | -0.063200% | 147 | `{"atr_percentile_high": 97, "ctx_side_return_atr": 50}` |
| `original_t2_ctx4h_scaled025` | `ETHUSDT` | 2025-11 | -0.990000% | 37 | 34 | -0.279530% | -0.059920% | 152 | `{"atr_percentile_high": 109, "ctx_side_return_atr": 43}` |
| `original_t2_ctx4h_scaled025` | `ETHUSDT` | 2025-12 | -1.020000% | 43 | 36 | -0.163490% | -0.097090% | 128 | `{"atr_percentile_high": 86, "ctx_side_return_atr": 42}` |
| `original_t2_ctx4h_scaled025` | `ETHUSDT` | 2026-01 | -0.570000% | 28 | 22 | -0.098470% | -0.064920% | 139 | `{"atr_percentile_high": 108, "ctx_side_return_atr": 31}` |
| `original_t2_ctx4h_scaled025` | `ETHUSDT` | 2026-02 | -0.710000% | 36 | 25 | -0.099890% | 0.222130% | 109 | `{"atr_percentile_high": 60, "ctx_side_return_atr": 49}` |
| `original_t2_ctx4h_scaled025` | `ETHUSDT` | 2026-03 | -1.230000% | 38 | 20 | -0.120780% | -0.298790% | 114 | `{"atr_percentile_high": 74, "ctx_side_return_atr": 40}` |
| `original_t2_ctx4h_scaled025` | `ETHUSDT` | 2026-04 | -0.570000% | 24 | 20 | -0.110400% | -0.052140% | 128 | `{"atr_percentile_high": 94, "ctx_side_return_atr": 34}` |
| `original_t2_ctx4h_scaled025` | `BTCUSDT` | 2025-06 | -0.890000% | 24 | 19 | -0.171230% | -0.139080% | 116 | `{"atr_percentile_high": 79, "ctx_side_return_atr": 37}` |
| `original_t2_ctx4h_scaled025` | `BTCUSDT` | 2025-07 | -0.740000% | 17 | 12 | -0.132340% | -0.134350% | 129 | `{"atr_percentile_high": 88, "ctx_side_return_atr": 41}` |
| `original_t2_ctx4h_scaled025` | `BTCUSDT` | 2025-08 | -0.390000% | 13 | 8 | -0.060650% | -0.048480% | 135 | `{"atr_percentile_high": 102, "ctx_side_return_atr": 33}` |
| `original_t2_ctx4h_scaled025` | `BTCUSDT` | 2025-09 | -0.860000% | 29 | 24 | -0.165000% | -0.068330% | 124 | `{"atr_percentile_high": 86, "ctx_side_return_atr": 38}` |
| `original_t2_ctx4h_scaled025` | `BTCUSDT` | 2025-10 | -0.930000% | 32 | 27 | -0.365380% | 0.086960% | 133 | `{"atr_percentile_high": 96, "ctx_side_return_atr": 37}` |
| `original_t2_ctx4h_scaled025` | `BTCUSDT` | 2025-11 | -0.750000% | 23 | 18 | -0.166280% | -0.162170% | 150 | `{"atr_percentile_high": 109, "ctx_side_return_atr": 41}` |
| `original_t2_ctx4h_scaled025` | `BTCUSDT` | 2025-12 | -1.140000% | 40 | 34 | -0.155430% | -0.179830% | 118 | `{"atr_percentile_high": 78, "ctx_side_return_atr": 40}` |
| `original_t2_ctx4h_scaled025` | `BTCUSDT` | 2026-01 | -0.600000% | 29 | 28 | -0.245610% | 0.047080% | 131 | `{"atr_percentile_high": 104, "ctx_side_return_atr": 27}` |
| `original_t2_ctx4h_scaled025` | `BTCUSDT` | 2026-02 | -0.450000% | 17 | 13 | -0.203250% | 0.148200% | 116 | `{"atr_percentile_high": 78, "ctx_side_return_atr": 38}` |
| `original_t2_ctx4h_scaled025` | `BTCUSDT` | 2026-03 | -1.290000% | 27 | 15 | -0.127320% | -0.425600% | 130 | `{"atr_percentile_high": 82, "ctx_side_return_atr": 48}` |
| `original_t2_ctx4h_scaled025` | `BTCUSDT` | 2026-04 | -1.080000% | 30 | 26 | -0.371200% | -0.086810% | 132 | `{"atr_percentile_high": 91, "ctx_side_return_atr": 41}` |