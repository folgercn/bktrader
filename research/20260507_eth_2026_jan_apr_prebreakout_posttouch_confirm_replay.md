# ETH Pre-Breakout Watch + Post-Touch Confirm 1s Replay (2026-01-01T00:00:00+00:00 to 2026-04-30T23:59:59+00:00)

Scope: research-only. The pre-breakout probability table only arms one setup per 1h level. Entry waits for the breakout level to be touched, then requires a 1s close beyond the confirmation distance. Execution and exits use continuous 1s bars and structure trailing.

| Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Armed | Entries | Busy | Pre Fail | Pre Timeout | Post Fail | Confirm Timeout | Dedupe |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `pt_d010_p70_c002` | 149 | -0.7969% | 2.7390% | 1.3083% | 44.97% | -4.1897% | 0.2349 | 1145.00s | 0.3315 | `{'InitialSL': 82, 'BreakevenSL': 50, 'StructureSL': 16, 'NoNewLowExit': 1}` | `{'base': 97, 'strong': 52}` | 360 | 149 | 48 | 103 | 0 | 54 | 0 | 536 |
| `pt_d010_p70_c005` | 130 | -2.4762% | 0.5560% | -0.6689% | 46.15% | -3.7525% | 0.2354 | 1162.00s | 0.3682 | `{'InitialSL': 70, 'BreakevenSL': 47, 'StructureSL': 12, 'NoNewLowExit': 1}` | `{'base': 84, 'strong': 46}` | 360 | 130 | 35 | 109 | 0 | 79 | 0 | 536 |
| `pt_d010_p75_c005` | 70 | 0.7162% | 2.2790% | 1.6511% | 54.29% | -1.5151% | 0.2200 | 1476.00s | 0.4671 | `{'InitialSL': 32, 'BreakevenSL': 31, 'StructureSL': 6, 'NoNewLowExit': 1}` | `{'base': 56, 'strong': 14}` | 207 | 70 | 8 | 79 | 0 | 45 | 1 | 234 |
| `pt_d020_p65_c005` | 148 | -3.0682% | 0.2461% | -1.0939% | 45.27% | -4.2064% | 0.2270 | 1084.50s | 0.3682 | `{'InitialSL': 81, 'BreakevenSL': 53, 'StructureSL': 12, 'NoNewLowExit': 2}` | `{'base': 108, 'strong': 40}` | 380 | 148 | 41 | 114 | 0 | 69 | 1 | 717 |

## Files

- Summary JSON: `research/eth_2026_jan_apr_prebreakout_posttouch_confirm_replay_summary.json`
- `pt_d010_p70_c002` ledger: `research/tmp_eth_2026_jan_apr_prebreakout_posttouch_confirm_replay_pt_d010_p70_c002_ledger.csv`
- `pt_d010_p70_c005` ledger: `research/tmp_eth_2026_jan_apr_prebreakout_posttouch_confirm_replay_pt_d010_p70_c005_ledger.csv`
- `pt_d010_p75_c005` ledger: `research/tmp_eth_2026_jan_apr_prebreakout_posttouch_confirm_replay_pt_d010_p75_c005_ledger.csv`
- `pt_d020_p65_c005` ledger: `research/tmp_eth_2026_jan_apr_prebreakout_posttouch_confirm_replay_pt_d020_p65_c005_ledger.csv`
