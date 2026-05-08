# ETH Pre-Breakout Entry 1s Replay (2026-01-01T00:00:00+00:00 to 2026-01-03T23:59:59+00:00)

Scope: research-only. Entry uses 2025-trained empirical state hit probabilities before the 1h breakout level is touched. Execution and exits use continuous 1s bars and structure trailing.

| Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Candidate Min | Entries | Busy | Pre Fail | Pre Timeout | Already Broken |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|---:|---:|
| `arm_d010_p70` | 6 | -0.1207% | 0.0392% | -0.0248% | 66.67% | -0.1267% | 0.2667 | 458.50s | 0.6027 | `{'BreakevenSL': 4, 'InitialSL': 2}` | `{'strong': 4, 'base': 2}` | 15 | 6 | 6 | 3 | 0 | 0 |

## Files

- Summary JSON: `research/tmp_prebreakout_arm_smoke_summary.json`
- `arm_d010_p70` ledger: `research/tmp_prebreakout_arm_smoke_arm_d010_p70_ledger.csv`
