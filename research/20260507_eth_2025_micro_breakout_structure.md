# Micro Breakout Structure 1s Replay (2025-01-01T00:00:00+00:00 to 2025-12-31T23:59:59+00:00)

Scope: research-only. Signals use closed 1h breakout bars; execution uses continuous 1s OHLC bars. Higher timeframe trend filtering is variant-controlled. Entry sizing is based on recent 1s speed/efficiency, and structure exits trail behind completed 1h bar structure after the configured ATR profit threshold.

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Cands | Entries | Weak Skip | Busy |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|
| `ETHUSDT` | `dual_old_fixed` | 139 | -3.1270% | -0.3950% | -1.4973% | 60.43% | -3.5252% | 0.2000 | 1919.00s | 0.5585 | `{'TrailingSL': 81, 'InitialSL': 49, 'NoNewHighExit': 6, 'NoNewLowExit': 3}` | `{'weak': 79, 'strong': 43, 'base': 17}` | 141 | 139 | 0 | 2 |
| `ETHUSDT` | `signal_old_micro` | 259 | -4.0957% | 0.3097% | -1.4767% | 57.14% | -4.1764% | 0.1734 | 2228.00s | 0.5363 | `{'TrailingSL': 145, 'InitialSL': 90, 'NoNewHighExit': 16, 'NoNewLowExit': 8}` | `{'weak': 145, 'strong': 76, 'base': 38}` | 261 | 259 | 0 | 2 |
| `ETHUSDT` | `signal_struct_micro` | 245 | 5.3841% | 9.9672% | 8.1109% | 46.94% | -2.6635% | 0.1739 | 6347.00s | 0.6310 | `{'InitialSL': 108, 'BreakevenSL': 60, 'StructureSL': 42, 'NoNewHighExit': 17, 'NoNewLowExit': 11, 'MaxHoldExit': 7}` | `{'weak': 136, 'strong': 72, 'base': 37}` | 261 | 245 | 0 | 16 |
| `ETHUSDT` | `signal_struct_skipweak` | 112 | 7.5149% | 10.7631% | 9.4529% | 50.00% | -2.4915% | 0.2661 | 7272.00s | 0.6646 | `{'InitialSL': 41, 'BreakevenSL': 25, 'StructureSL': 23, 'NoNewHighExit': 13, 'NoNewLowExit': 5, 'MaxHoldExit': 5}` | `{'strong': 74, 'base': 38}` | 261 | 112 | 143 | 6 |

## Files

- Summary JSON: `research/btc_eth_2025_eth_micro_breakout_structure_summary.json`
- `ETHUSDT dual_old_fixed` ledger: `research/tmp_eth_2025_micro_breakout_structure_ETHUSDT_dual_old_fixed_ledger.csv`
- `ETHUSDT signal_old_micro` ledger: `research/tmp_eth_2025_micro_breakout_structure_ETHUSDT_signal_old_micro_ledger.csv`
- `ETHUSDT signal_struct_micro` ledger: `research/tmp_eth_2025_micro_breakout_structure_ETHUSDT_signal_struct_micro_ledger.csv`
- `ETHUSDT signal_struct_skipweak` ledger: `research/tmp_eth_2025_micro_breakout_structure_ETHUSDT_signal_struct_skipweak_ledger.csv`
