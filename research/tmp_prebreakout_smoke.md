# ETH Pre-Breakout Hitting Probability (2026-01-01T00:00:00+00:00 to 2026-01-03T23:59:59+00:00)

Scope: research-only. Empirical Markov-style state table using 1m bars before the 1h breakout level is touched. Outcome is whether price hits the breakout level before an adverse fail move within the configured horizon.

- Horizon: 60m
- Max distance: 0.4 ATR
- Fail distance: 0.2 ATR
- Candidates: 257

## Top States

| Distance | Speed5 | Eff15 | Samples | Hit | Fail | Timeout | Median Min | Avg Dist | Avg Speed5 | Avg Eff |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|

## Side Distance Summary

| Side | Distance | Samples | Hit | Fail | Median Min |
|---|---|---:|---:|---:|---:|
| `long` | `0-0.10` | 43 | 81.40% | 18.60% | 1.00 |
| `long` | `0.10-0.20` | 45 | 73.33% | 24.44% | 3.00 |
| `long` | `0.20-0.30` | 71 | 29.58% | 60.56% | 5.00 |
| `long` | `0.30-0.40` | 98 | 26.53% | 60.20% | 12.00 |

## Files

- Summary JSON: `research/tmp_prebreakout_smoke_summary.json`
- Candidates CSV: `research/tmp_prebreakout_smoke_candidates.csv`
