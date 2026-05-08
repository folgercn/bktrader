# Micro Breakout Structure 1s Replay (2026-01-01T00:00:00+00:00 to 2026-01-07T23:59:59+00:00)

Scope: research-only. Signals use closed 1h breakout bars; execution uses continuous 1s OHLC bars. Higher timeframe trend filtering is variant-controlled. Entry sizing is based on recent 1s speed/efficiency, and structure exits trail behind completed 1h bar structure after the configured ATR profit threshold.

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Cands | Entries | Weak Skip | Busy |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|
| `ETHUSDT` | `dual_old` | 0 | 0.0000% | 0.0000% | 0.0000% | 0.00% | 0.0000% | 0.0000 | 0.00s | 0.0000 | `{}` | `{}` | 0 | 0 | 0 | 0 |
| `ETHUSDT` | `signal_structure` | 2 | -0.2309% | -0.1709% | -0.1950% | 50.00% | -0.2369% | 0.3000 | 2339.50s | 0.6273 | `{'BreakevenSL': 1, 'InitialSL': 1}` | `{'strong': 2}` | 2 | 2 | 0 | 0 |
| `ETHUSDT` | `none_structure` | 2 | -0.2309% | -0.1709% | -0.1950% | 50.00% | -0.2369% | 0.3000 | 2339.50s | 0.6273 | `{'BreakevenSL': 1, 'InitialSL': 1}` | `{'strong': 2}` | 2 | 2 | 0 | 0 |
| `ETHUSDT` | `none_hybrid` | 2 | -0.1390% | -0.0790% | -0.1030% | 50.00% | -0.2369% | 0.3000 | 1834.00s | 0.6273 | `{'TrailingSL': 1, 'InitialSL': 1}` | `{'strong': 2}` | 2 | 2 | 0 | 0 |

## Files

- Summary JSON: `research/tmp_micro_breakout_structure_smoke_summary.json`
- `ETHUSDT dual_old` ledger: `research/tmp_micro_breakout_structure_smoke_ETHUSDT_dual_old_ledger.csv`
- `ETHUSDT signal_structure` ledger: `research/tmp_micro_breakout_structure_smoke_ETHUSDT_signal_structure_ledger.csv`
- `ETHUSDT none_structure` ledger: `research/tmp_micro_breakout_structure_smoke_ETHUSDT_none_structure_ledger.csv`
- `ETHUSDT none_hybrid` ledger: `research/tmp_micro_breakout_structure_smoke_ETHUSDT_none_hybrid_ledger.csv`
