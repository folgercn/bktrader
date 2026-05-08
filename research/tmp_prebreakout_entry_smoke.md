# ETH Pre-Breakout Entry 1s Replay (2026-01-01T00:00:00+00:00 to 2026-01-03T23:59:59+00:00)

Scope: research-only. Entry uses 2025-trained empirical state hit probabilities before the 1h breakout level is touched. Execution and exits use continuous 1s bars and structure trailing.

| Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Candidate Min | Entries | Busy | Already Broken |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|
| `pre_d010_p70` | 8 | -0.2717% | -0.0621% | -0.1460% | 50.00% | -0.2777% | 0.2625 | 615.00s | 0.3143 | `{'BreakevenSL': 4, 'InitialSL': 3, 'FinalMarkToMarket': 1}` | `{'strong': 5, 'base': 3}` | 15 | 8 | 7 | 0 |

## Files

- Summary JSON: `research/tmp_prebreakout_entry_smoke_summary.json`
- `pre_d010_p70` ledger: `research/tmp_prebreakout_entry_smoke_pre_d010_p70_ledger.csv`
