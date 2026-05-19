# T2 Lifecycle Context Sizing

Research-only strict lifecycle bridge for original_t2 context sizing. Filters scale T2 order size instead of rejecting locks.

- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- Months: 2026-04
- Symbols: ETHUSDT, BTCUSDT

## Candidate Summary

| Candidate | Calendar Sum | Avg/Symbol-Month | Worst Silo | Neg Silos | Trades | T2 Trades | T2 Net PnL | T3 Net PnL | Size Fails | Filters | Fail Mult | T3 Overrides |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---|
| `original_t2_ctx4h_scaled100_t3_min_hold_sl_60m` | -2.760000% | -1.380000% | -1.790000% | 2 | 52 | 46 | -1.062300% | 0.046830% | 259 | `{"ctx_return_lookback_bars": 4, "max_atr_percentile": 40.0, "min_ctx_side_return_atr": 0.0}` | 1.00 | `{"min_hold_seconds_before_sl": 3600.0}` |
| `original_t2_ctx4h_scaled050_t3_min_hold_sl_60m` | -1.880000% | -0.940000% | -1.220000% | 2 | 52 | 46 | -0.675760% | 0.047360% | 259 | `{"ctx_return_lookback_bars": 4, "max_atr_percentile": 40.0, "min_ctx_side_return_atr": 0.0}` | 0.50 | `{"min_hold_seconds_before_sl": 3600.0}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | -1.440000% | -0.720000% | -0.930000% | 2 | 52 | 46 | -0.481860% | 0.047630% | 259 | `{"ctx_return_lookback_bars": 4, "max_atr_percentile": 40.0, "min_ctx_side_return_atr": 0.0}` | 0.25 | `{"min_hold_seconds_before_sl": 3600.0}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | -1.180000% | -0.590000% | -0.760000% | 2 | 52 | 46 | -0.365320% | 0.047790% | 259 | `{"ctx_return_lookback_bars": 4, "max_atr_percentile": 40.0, "min_ctx_side_return_atr": 0.0}` | 0.10 | `{"min_hold_seconds_before_sl": 3600.0}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | -1.000000% | -0.500000% | -0.640000% | 2 | 22 | 46 | -0.287540% | 0.047890% | 259 | `{"ctx_return_lookback_bars": 4, "max_atr_percentile": 40.0, "min_ctx_side_return_atr": 0.0}` | 0.00 | `{"min_hold_seconds_before_sl": 3600.0}` |

## Delta Vs Baseline

| Candidate | Calendar Delta | Worst Silo Delta | Trade Delta | T2 Trade Delta | T2 PnL Delta | Neg Silo Delta |
|---|---:|---:|---:|---:|---:|---:|
| `original_t2_ctx4h_scaled050_t3_min_hold_sl_60m` vs `original_t2_ctx4h_scaled100_t3_min_hold_sl_60m` | +0.880000% | +0.570000% | +0 | +0 | +0.386540% | +0 |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` vs `original_t2_ctx4h_scaled100_t3_min_hold_sl_60m` | +1.320000% | +0.860000% | +0 | +0 | +0.580440% | +0 |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` vs `original_t2_ctx4h_scaled100_t3_min_hold_sl_60m` | +1.580000% | +1.030000% | +0 | +0 | +0.696980% | +0 |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` vs `original_t2_ctx4h_scaled100_t3_min_hold_sl_60m` | +1.760000% | +1.150000% | -30 | +0 | +0.774760% | +0 |

## T2 Size Multiplier Attribution

| Candidate | Bucket | Trades | Avg Mult | Gross PnL | Fee | Net After Fee | Notional |
|---|---|---:|---:|---:|---:|---:|---:|
| `original_t2_ctx4h_scaled100_t3_min_hold_sl_60m` | `pass_full_or_unfiltered` | 46 | 1.000000 | -1.062294% | 1.508702% | -2.570996% | 754.350938% |
| `original_t2_ctx4h_scaled050_t3_min_hold_sl_60m` | `fail_scaled_0.50` | 30 | 0.500000 | -0.389212% | 0.497728% | -0.886939% | 248.863729% |
| `original_t2_ctx4h_scaled050_t3_min_hold_sl_60m` | `pass_full_or_unfiltered` | 16 | 1.000000 | -0.286545% | 0.517080% | -0.803626% | 258.540015% |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `fail_scaled_0.25` | 30 | 0.250000 | -0.194816% | 0.249153% | -0.443971% | 124.576827% |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `pass_full_or_unfiltered` | 16 | 1.000000 | -0.287044% | 0.517842% | -0.804885% | 258.920724% |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `fail_scaled_0.10` | 30 | 0.100000 | -0.077978% | 0.099731% | -0.177708% | 49.865566% |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `pass_full_or_unfiltered` | 16 | 1.000000 | -0.287342% | 0.518299% | -0.805641% | 259.149447% |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `fail_zero` | 30 | 0.000000 | 0.000000% | 0.000000% | 0.000000% | 0.000000% |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `pass_full_or_unfiltered` | 16 | 1.000000 | -0.287542% | 0.518604% | -0.806147% | 259.302053% |

## Silo Detail

| Candidate | Symbol | Month | Return | Trades | T2 Trades | T2 PnL | T3 PnL | Size Fails | Size Reasons |
|---|---|---|---:|---:|---:|---:|---:|---:|---|
| `original_t2_ctx4h_scaled100_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-04 | -0.970000% | 22 | 20 | -0.256060% | -0.034720% | 128 | `{"atr_percentile_high": 94, "ctx_side_return_atr": 34}` |
| `original_t2_ctx4h_scaled100_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-04 | -1.790000% | 30 | 26 | -0.806240% | 0.081550% | 131 | `{"atr_percentile_high": 91, "ctx_side_return_atr": 40}` |
| `original_t2_ctx4h_scaled050_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-04 | -0.660000% | 22 | 20 | -0.159050% | -0.034750% | 128 | `{"atr_percentile_high": 94, "ctx_side_return_atr": 34}` |
| `original_t2_ctx4h_scaled050_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-04 | -1.220000% | 30 | 26 | -0.516710% | 0.082110% | 131 | `{"atr_percentile_high": 91, "ctx_side_return_atr": 40}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-04 | -0.510000% | 22 | 20 | -0.110450% | -0.034770% | 128 | `{"atr_percentile_high": 94, "ctx_side_return_atr": 34}` |
| `original_t2_ctx4h_scaled025_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-04 | -0.930000% | 30 | 26 | -0.371410% | 0.082400% | 131 | `{"atr_percentile_high": 91, "ctx_side_return_atr": 40}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-04 | -0.420000% | 22 | 20 | -0.081260% | -0.034780% | 128 | `{"atr_percentile_high": 94, "ctx_side_return_atr": 34}` |
| `original_t2_ctx4h_scaled010_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-04 | -0.760000% | 30 | 26 | -0.284060% | 0.082570% | 131 | `{"atr_percentile_high": 91, "ctx_side_return_atr": 40}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `ETHUSDT` | 2026-04 | -0.360000% | 8 | 20 | -0.061780% | -0.034790% | 128 | `{"atr_percentile_high": 94, "ctx_side_return_atr": 34}` |
| `original_t2_ctx4h_scaled000_t3_min_hold_sl_60m` | `BTCUSDT` | 2026-04 | -0.640000% | 14 | 26 | -0.225760% | 0.082680% | 131 | `{"atr_percentile_high": 91, "ctx_side_return_atr": 40}` |