# ETH Pre-Breakout Entry 1s Replay (2025-01-01T00:00:00+00:00 to 2025-12-31T23:59:59+00:00)

Scope: research-only. Entry uses 2025-trained empirical state hit probabilities before the 1h breakout level is touched. Execution and exits use continuous 1s bars and structure trailing.

| Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Candidate Min | Entries | Busy | Pre Fail | Pre Timeout | Already Broken |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|---:|---:|
| `arm_d010_p70` | 929 | -8.7797% | 13.3942% | 3.9427% | 50.16% | -17.1432% | 0.2342 | 1645.00s | 0.4461 | `{'InitialSL': 457, 'BreakevenSL': 387, 'StructureSL': 72, 'NoNewHighExit': 8, 'NoNewLowExit': 4, 'EMA8Exit': 1}` | `{'base': 611, 'strong': 318}` | 4307 | 929 | 2797 | 579 | 1 | 0 |
| `arm_d010_p75` | 577 | -7.6168% | 4.4330% | -0.5643% | 50.78% | -11.8713% | 0.2125 | 2066.00s | 0.4520 | `{'InitialSL': 276, 'BreakevenSL': 241, 'StructureSL': 45, 'NoNewHighExit': 10, 'NoNewLowExit': 4, 'EMA8Exit': 1}` | `{'base': 505, 'strong': 72}` | 2067 | 577 | 1220 | 270 | 0 | 0 |
| `arm_d010_p80` | 54 | -0.9446% | 0.1304% | -0.3006% | 42.59% | -1.5852% | 0.2000 | 2851.50s | 0.3162 | `{'InitialSL': 28, 'BreakevenSL': 14, 'StructureSL': 9, 'NoNewHighExit': 2, 'NoNewLowExit': 1}` | `{'base': 54}` | 81 | 54 | 13 | 14 | 0 | 0 |
| `arm_d020_p65` | 968 | -10.8555% | 11.3155% | 1.8524% | 49.59% | -18.5497% | 0.2294 | 1646.00s | 0.4358 | `{'InitialSL': 482, 'BreakevenSL': 399, 'StructureSL': 73, 'NoNewHighExit': 9, 'NoNewLowExit': 4, 'EMA8Exit': 1}` | `{'base': 683, 'strong': 285}` | 5099 | 968 | 3466 | 663 | 1 | 0 |

## Files

- Summary JSON: `research/eth_2025_prebreakout_armed_entry_replay_summary.json`
- `arm_d010_p70` ledger: `research/tmp_eth_2025_prebreakout_armed_entry_replay_arm_d010_p70_ledger.csv`
- `arm_d010_p75` ledger: `research/tmp_eth_2025_prebreakout_armed_entry_replay_arm_d010_p75_ledger.csv`
- `arm_d010_p80` ledger: `research/tmp_eth_2025_prebreakout_armed_entry_replay_arm_d010_p80_ledger.csv`
- `arm_d020_p65` ledger: `research/tmp_eth_2025_prebreakout_armed_entry_replay_arm_d020_p65_ledger.csv`
