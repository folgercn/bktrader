# T3 Bayesian Event Filter

Walk-forward Bayesian bucket scores for T3 event quality.

- Trades: `/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t3_probability_overlay_extended/t3_probability_overlay_trades.csv`
- Events: `/Users/wuyaocheng/Downloads/bkTrader/research/entry_redesign/scripts/output/timing_probability_unified/t3_probability_overlay_extended/t3_probability_overlay_scored_events.csv`
- Min train months: `3`
- Min group trades: `3`
- Prior strength: `8.0`

## Threshold Summary

| Threshold | Events | Labeled Trades | Label Net | Worst Silo | Neg Silos | Avg Score | Avg Trade | Win Rate |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `bayes_ge_m0p010` | 228 | 51 | -0.126450% | -0.341424% | 7 | 0.008887% | -0.002479% | 43.14% |
| `bayes_ge_0p020` | 44 | 9 | -0.483125% | -0.086235% | 7 | 0.040217% | -0.053681% | 22.22% |
| `bayes_ge_0p000` | 139 | 31 | -0.643123% | -0.285718% | 8 | 0.017946% | -0.020746% | 41.94% |
| `bayes_ge_0p005` | 101 | 26 | -0.754760% | -0.285718% | 7 | 0.023938% | -0.029029% | 38.46% |
| `bayes_ge_0p010` | 78 | 24 | -0.773381% | -0.227837% | 7 | 0.028991% | -0.032224% | 37.50% |

## Read

- This is an out-of-sample label audit for event selection, not a lifecycle result by itself.
- A selected-event CSV must still be replayed with strict lifecycle and adverse/next-second checks.
- Sparse groups back off to broader buckets, so high scores are intentionally conservative.
