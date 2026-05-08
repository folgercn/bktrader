# ETH Pre-Breakout Entry 1s Replay (2026-01-01T00:00:00+00:00 to 2026-04-30T23:59:59+00:00)

Scope: research-only. Entry uses 2025-trained empirical state hit probabilities before the 1h breakout level is touched. Execution and exits use continuous 1s bars and structure trailing.

| Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Candidate Min | Entries | Busy | Pre Fail | Pre Timeout | Already Broken |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|---:|---:|
| `arm_d010_p70` | 232 | -2.9781% | 2.5295% | 0.2892% | 49.57% | -4.0458% | 0.2379 | 1493.50s | 0.4079 | `{'InitialSL': 116, 'BreakevenSL': 94, 'StructureSL': 18, 'NoNewLowExit': 4}` | `{'base': 144, 'strong': 88}` | 896 | 232 | 470 | 180 | 0 | 0 |
| `arm_d010_p80` | 8 | -0.4280% | -0.2686% | -0.3323% | 37.50% | -0.5000% | 0.2000 | 2868.00s | 0.2398 | `{'InitialSL': 5, 'BreakevenSL': 2, 'StructureSL': 1}` | `{'base': 8}` | 11 | 8 | 0 | 2 | 0 | 0 |

## Files

- Summary JSON: `research/eth_2026_jan_apr_prebreakout_armed_entry_replay_summary.json`
- `arm_d010_p70` ledger: `research/tmp_eth_2026_jan_apr_prebreakout_armed_entry_replay_arm_d010_p70_ledger.csv`
- `arm_d010_p80` ledger: `research/tmp_eth_2026_jan_apr_prebreakout_armed_entry_replay_arm_d010_p80_ledger.csv`
