# ETH Pre-Breakout Entry 1s Replay (2025-01-01T00:00:00+00:00 to 2025-12-31T23:59:59+00:00)

Scope: research-only. Entry uses 2025-trained empirical state hit probabilities before the 1h breakout level is touched. Execution and exits use continuous 1s bars and structure trailing.

| Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Candidate Min | Entries | Busy | Already Broken |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|
| `pre_d005_p70` | 573 | -4.7642% | 9.5556% | 3.5874% | 46.60% | -13.4072% | 0.2445 | 1898.00s | 0.3685 | `{'InitialSL': 304, 'BreakevenSL': 215, 'StructureSL': 43, 'NoNewHighExit': 9, 'NoNewLowExit': 1, 'EMA8Exit': 1}` | `{'base': 318, 'strong': 255}` | 1093 | 573 | 506 | 13 |
| `pre_d010_p70` | 1114 | -16.9782% | 7.3563% | -3.1349% | 48.47% | -21.5742% | 0.2307 | 1681.00s | 0.4180 | `{'InitialSL': 565, 'BreakevenSL': 449, 'StructureSL': 83, 'NoNewHighExit': 10, 'EMA8Exit': 4, 'NoNewLowExit': 3}` | `{'base': 772, 'strong': 342}` | 4307 | 1114 | 3178 | 14 |
| `pre_d010_p75` | 704 | -10.4565% | 3.9632% | -2.0638% | 48.44% | -15.3916% | 0.2121 | 2050.00s | 0.4205 | `{'InitialSL': 356, 'BreakevenSL': 279, 'StructureSL': 52, 'NoNewHighExit': 13, 'NoNewLowExit': 2, 'EMA8Exit': 2}` | `{'base': 619, 'strong': 85}` | 2067 | 704 | 1356 | 7 |
| `pre_d010_p80` | 64 | -0.8333% | 0.4441% | -0.0687% | 54.69% | -1.8137% | 0.2000 | 4076.00s | 0.4444 | `{'InitialSL': 27, 'BreakevenSL': 23, 'StructureSL': 8, 'NoNewHighExit': 5, 'NoNewLowExit': 1}` | `{'base': 64}` | 81 | 64 | 17 | 0 |
| `pre_d020_p65` | 1150 | -16.5113% | 8.1666% | -2.4790% | 48.43% | -20.9250% | 0.2251 | 1741.50s | 0.4164 | `{'InitialSL': 582, 'BreakevenSL': 459, 'StructureSL': 89, 'NoNewHighExit': 13, 'EMA8Exit': 4, 'NoNewLowExit': 3}` | `{'base': 861, 'strong': 289}` | 5099 | 1150 | 3935 | 13 |
| `pre_d020_p70` | 1114 | -16.9782% | 7.3563% | -3.1349% | 48.47% | -21.5742% | 0.2307 | 1681.00s | 0.4180 | `{'InitialSL': 565, 'BreakevenSL': 449, 'StructureSL': 83, 'NoNewHighExit': 10, 'EMA8Exit': 4, 'NoNewLowExit': 3}` | `{'base': 772, 'strong': 342}` | 4307 | 1114 | 3178 | 14 |

## Files

- Summary JSON: `research/eth_2025_prebreakout_entry_replay_summary.json`
- `pre_d005_p70` ledger: `research/tmp_eth_2025_prebreakout_entry_replay_pre_d005_p70_ledger.csv`
- `pre_d010_p70` ledger: `research/tmp_eth_2025_prebreakout_entry_replay_pre_d010_p70_ledger.csv`
- `pre_d010_p75` ledger: `research/tmp_eth_2025_prebreakout_entry_replay_pre_d010_p75_ledger.csv`
- `pre_d010_p80` ledger: `research/tmp_eth_2025_prebreakout_entry_replay_pre_d010_p80_ledger.csv`
- `pre_d020_p65` ledger: `research/tmp_eth_2025_prebreakout_entry_replay_pre_d020_p65_ledger.csv`
- `pre_d020_p70` ledger: `research/tmp_eth_2025_prebreakout_entry_replay_pre_d020_p70_ledger.csv`
