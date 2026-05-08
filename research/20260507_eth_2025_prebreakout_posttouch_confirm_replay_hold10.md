# ETH Pre-Breakout Watch + Post-Touch Confirm 1s Replay (2025-01-01T00:00:00+00:00 to 2025-12-31T23:59:59+00:00)

Scope: research-only. The pre-breakout probability table only arms one setup per 1h level. Entry waits for the breakout level to be touched, then requires a 1s close beyond the confirmation distance. Execution and exits use continuous 1s bars and structure trailing.

| Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Armed | Entries | Busy | Pre Fail | Pre Timeout | Post Fail | Confirm Timeout | Dedupe |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `pt_d010_p75_c005_h10` | 375 | -3.0352% | 4.9779% | 1.6963% | 47.47% | -8.6680% | 0.2117 | 1821.00s | 0.3817 | `{'InitialSL': 195, 'BreakevenSL': 141, 'MaxHoldExit': 19, 'StructureSL': 12, 'NoNewHighExit': 5, 'NoNewLowExit': 3}` | `{'base': 331, 'strong': 44}` | 901 | 375 | 136 | 202 | 0 | 183 | 5 | 1166 |

## Files

- Summary JSON: `research/eth_2025_prebreakout_posttouch_confirm_replay_hold10_summary.json`
- `pt_d010_p75_c005_h10` ledger: `research/tmp_eth_2025_prebreakout_posttouch_confirm_replay_hold10_pt_d010_p75_c005_h10_ledger.csv`
