# Probabilistic V6 Combo Union Runner

范围：仅限 `research`。本报告把 gate 扫描选中的多个 sleeve 合并到事件级，再用同一个 1s execution runner 回测。

## Metrics

| Metric | Value |
|---|---:|
| Active_Silo_Sum | 11.9370% |
| Active Months | 9 |
| Trades | 115 |
| Worst Silo | -1.5113% |
| Best Silo | 5.3086% |
| Input Sleeve Rows | 17 |
| Union Groups | 10 |
| Duplicate Events Removed | 447 |
| Sizing Profile | `none` |
| Share Multiplier | 1.1000 |
| Share Power | 1.0000 |
| Share Cap | 0.0000 |

## Groups

| Month | Symbol | Sleeves | Input Events | Union Events | Dups | Trades | Return | Mean Share | Mean Scale | Sources |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `2025-06` | `ETHUSDT` | 1 | 70 | 70 | 0 | 3 | 0.1195% | 0.9700 | 1.0000 | `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10:top10` |
| `2025-07` | `BTCUSDT` | 1 | 221 | 221 | 0 | 18 | -0.7987% | 1.1431 | 1.0000 | `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback:top20` |
| `2025-08` | `BTCUSDT` | 1 | 175 | 175 | 0 | 5 | -1.5113% | 1.1531 | 1.0000 | `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback:top5` |
| `2025-09` | `ETHUSDT` | 1 | 85 | 85 | 0 | 5 | 0.2686% | 1.2897 | 1.0000 | `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10:top10` |
| `2025-11` | `BTCUSDT` | 1 | 113 | 113 | 0 | 6 | 0.4509% | 1.1044 | 1.0000 | `walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback:top15` |
| `2025-12` | `ETHUSDT` | 2 | 241 | 210 | 31 | 24 | 0.4801% | 1.1375 | 1.0000 | `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback:top20, walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10:top10` |
| `2026-01` | `BTCUSDT` | 2 | 379 | 291 | 88 | 16 | 0.5011% | 1.0451 | 1.0000 | `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback:top10, walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback:top20` |
| `2026-02` | `BTCUSDT` | 3 | 528 | 383 | 145 | 15 | 4.7548% | 1.0751 | 1.0000 | `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback:top10, walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10:top10, walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback:top10` |
| `2026-02` | `ETHUSDT` | 2 | 313 | 268 | 45 | 12 | 2.3634% | 1.0826 | 1.0000 | `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback:top5, walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10:top10` |
| `2026-03` | `ETHUSDT` | 3 | 430 | 292 | 138 | 11 | 5.3086% | 1.0958 | 1.0000 | `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback:top5, walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10:top10, walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback:top10` |
