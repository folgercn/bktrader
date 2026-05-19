# T2 Lifecycle Pass-Bucket Gates

Research-only strict lifecycle sweep for shrinking the original_t2 full-size pass bucket.

- Timeframe: `1h`
- Reentry fill policy: `strict_next_second_cross`
- Months: 2025-06, 2025-07, 2025-08, 2025-09, 2025-10, 2025-11, 2025-12, 2026-01, 2026-02, 2026-03, 2026-04
- Symbols: ETHUSDT, BTCUSDT

## Candidate Summary

| Candidate | Calendar Sum | Delta | Worst Silo | Neg Silos | Trades | T2 Trades | T2 Net PnL | T3 Net PnL | Filters | Read |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| `t2_disabled_t3_60m` | -0.190000% | baseline | -0.460000% | 12 | 104 | 0 | 0.000000% | 3.988470% | `{"allowed_sides": []}` | strict lifecycle floor: disable original_t2 and keep only T3 60m behavior |

## Read

- These results are promotion-comparable lifecycle returns, not adverse10 event-ledger returns.
- A candidate only matters if it improves calendar sum without creating a worse worst-silo profile or collapsing T3 contribution.
- Exact low_eff/RF remains a separate hook because lifecycle replay still needs event-time speed/efficiency/RF features.
