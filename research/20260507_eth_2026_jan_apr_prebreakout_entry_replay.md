# ETH Pre-Breakout Entry 1s Replay (2026-01-01T00:00:00+00:00 to 2026-04-30T23:59:59+00:00)

Scope: research-only. Entry uses 2025-trained empirical state hit probabilities before the 1h breakout level is touched. Execution and exits use continuous 1s bars and structure trailing.

| Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Candidate Min | Entries | Busy | Already Broken |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|
| `pre_d005_p70` | 127 | -0.1004% | 3.0869% | 1.7994% | 50.39% | -3.1728% | 0.2472 | 1922.00s | 0.4032 | `{'InitialSL': 63, 'BreakevenSL': 49, 'StructureSL': 12, 'NoNewLowExit': 2, 'NoNewHighExit': 1}` | `{'base': 67, 'strong': 60}` | 205 | 127 | 64 | 3 |
| `pre_d010_p75` | 178 | -0.9053% | 2.9438% | 1.3861% | 47.75% | -2.6732% | 0.2140 | 1615.00s | 0.3759 | `{'InitialSL': 92, 'BreakevenSL': 69, 'StructureSL': 14, 'NoNewHighExit': 2, 'NoNewLowExit': 1}` | `{'base': 153, 'strong': 25}` | 441 | 178 | 238 | 1 |
| `pre_d010_p80` | 9 | -0.5351% | -0.3558% | -0.4276% | 33.33% | -0.6166% | 0.2000 | 3341.00s | 0.2366 | `{'InitialSL': 6, 'BreakevenSL': 2, 'StructureSL': 1}` | `{'base': 9}` | 11 | 9 | 1 | 0 |

## Files

- Summary JSON: `research/eth_2026_jan_apr_prebreakout_entry_replay_summary.json`
- `pre_d005_p70` ledger: `research/tmp_eth_2026_jan_apr_prebreakout_entry_replay_pre_d005_p70_ledger.csv`
- `pre_d010_p75` ledger: `research/tmp_eth_2026_jan_apr_prebreakout_entry_replay_pre_d010_p75_ledger.csv`
- `pre_d010_p80` ledger: `research/tmp_eth_2026_jan_apr_prebreakout_entry_replay_pre_d010_p80_ledger.csv`
