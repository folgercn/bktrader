# Probabilistic V6 Combo Union Runner

范围：仅限 `research`。本报告把 gate 扫描选中的多个 sleeve 合并到事件级，再用同一个 1s execution runner 回测。

## Metrics

| Metric | Value |
|---|---:|
| Active_Silo_Sum | 9.7496% |
| Active Months | 8 |
| Trades | 94 |
| Worst Silo | -1.3744% |
| Best Silo | 4.3173% |
| Input Sleeve Rows | 14 |
| Union Groups | 9 |
| Duplicate Events Removed | 387 |

## Groups

| Month | Symbol | Sleeves | Input Events | Union Events | Dups | Trades | Return | Sources |
|---|---|---:|---:|---:|---:|---:|---:|---|
| `2025-07` | `BTCUSDT` | 1 | 221 | 221 | 0 | 18 | -0.7250% | `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback:top20` |
| `2025-08` | `BTCUSDT` | 1 | 175 | 175 | 0 | 5 | -1.3744% | `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback:top5` |
| `2025-09` | `ETHUSDT` | 1 | 85 | 85 | 0 | 5 | 0.2444% | `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10:top10` |
| `2025-11` | `BTCUSDT` | 1 | 113 | 113 | 0 | 6 | 0.4103% | `walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback:top15` |
| `2025-12` | `ETHUSDT` | 1 | 72 | 72 | 0 | 8 | 0.3794% | `walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10:top10` |
| `2026-01` | `BTCUSDT` | 2 | 379 | 291 | 88 | 16 | 0.4559% | `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback:top10, walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback:top20` |
| `2026-02` | `BTCUSDT` | 3 | 528 | 383 | 145 | 15 | 4.3173% | `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback:top10, walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10:top10, walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback:top10` |
| `2026-02` | `ETHUSDT` | 2 | 313 | 268 | 45 | 12 | 2.1529% | `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback:top5, walkforward_2025m06_2026apr_r4_4_short_speed60_high_loose_top10:top10` |
| `2026-03` | `ETHUSDT` | 2 | 372 | 263 | 109 | 9 | 3.8888% | `walkforward_2025m06_2026apr_delay60_feature60_postselect_btc_fallback:top5, walkforward_delay60_original_t2_feature60_postselect_gate_btc_fallback:top10` |
