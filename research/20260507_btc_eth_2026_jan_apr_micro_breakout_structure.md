# Micro Breakout Structure 1s Replay (2026-01-01T00:00:00+00:00 to 2026-04-30T23:59:59+00:00)

Scope: research-only. Signals use closed 1h breakout bars; execution uses continuous 1s OHLC bars. Higher timeframe trend filtering is variant-controlled. Entry sizing is based on recent 1s speed/efficiency, and structure exits trail behind completed 1h bar structure after the configured ATR profit threshold.

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Avg Share | Med Hold | Med MFE ATR | Exits | Quality | Cands | Entries | Weak Skip | Busy |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---:|---:|---:|---:|
| `BTCUSDT` | `dual_old_fixed` | 49 | -0.5718% | 0.4081% | 0.0146% | 59.18% | -1.1839% | 0.2000 | 1832.00s | 0.5788 | `{'TrailingSL': 24, 'InitialSL': 19, 'NoNewHighExit': 3, 'BreakevenSL': 3}` | `{'weak': 26, 'strong': 20, 'base': 3}` | 50 | 49 | 0 | 1 |
| `BTCUSDT` | `signal_old_micro` | 77 | -1.0242% | 0.3823% | -0.1832% | 57.14% | -1.8450% | 0.1831 | 2152.00s | 0.5699 | `{'TrailingSL': 38, 'InitialSL': 27, 'NoNewHighExit': 4, 'NoNewLowExit': 4, 'BreakevenSL': 3, 'EMA8Exit': 1}` | `{'weak': 40, 'strong': 27, 'base': 10}` | 79 | 77 | 0 | 2 |
| `BTCUSDT` | `signal_struct_micro` | 72 | -1.2833% | 0.0395% | -0.4922% | 48.61% | -2.2139% | 0.1847 | 4348.00s | 0.7066 | `{'InitialSL': 31, 'BreakevenSL': 21, 'StructureSL': 12, 'NoNewLowExit': 4, 'MaxHoldExit': 2, 'NoNewHighExit': 1, 'EMA8Exit': 1}` | `{'weak': 37, 'strong': 26, 'base': 9}` | 79 | 72 | 0 | 7 |
| `BTCUSDT` | `signal_hybrid_micro` | 77 | -1.1260% | 0.2790% | -0.2859% | 57.14% | -1.8732% | 0.1831 | 2152.00s | 0.5699 | `{'TrailingSL': 38, 'InitialSL': 27, 'NoNewLowExit': 4, 'StructureSL': 3, 'BreakevenSL': 3, 'NoNewHighExit': 1, 'EMA8Exit': 1}` | `{'weak': 40, 'strong': 27, 'base': 10}` | 79 | 77 | 0 | 2 |
| `BTCUSDT` | `none_struct_micro` | 72 | -1.2833% | 0.0395% | -0.4922% | 48.61% | -2.2139% | 0.1847 | 4348.00s | 0.7066 | `{'InitialSL': 31, 'BreakevenSL': 21, 'StructureSL': 12, 'NoNewLowExit': 4, 'MaxHoldExit': 2, 'NoNewHighExit': 1, 'EMA8Exit': 1}` | `{'weak': 37, 'strong': 26, 'base': 9}` | 79 | 72 | 0 | 7 |
| `BTCUSDT` | `signal_struct_skipweak` | 36 | -0.4962% | 0.4945% | 0.0966% | 55.56% | -1.1451% | 0.2750 | 2869.50s | 0.9237 | `{'InitialSL': 16, 'BreakevenSL': 11, 'StructureSL': 7, 'MaxHoldExit': 2}` | `{'strong': 27, 'base': 9}` | 79 | 36 | 39 | 4 |
| `ETHUSDT` | `dual_old_fixed` | 44 | -0.6697% | 0.2083% | -0.1438% | 63.64% | -0.9030% | 0.2000 | 2154.00s | 0.5684 | `{'TrailingSL': 27, 'InitialSL': 12, 'NoNewLowExit': 3, 'NoNewHighExit': 2}` | `{'weak': 24, 'strong': 11, 'base': 9}` | 44 | 44 | 0 | 0 |
| `ETHUSDT` | `signal_old_micro` | 59 | -0.0478% | 0.9868% | 0.5717% | 62.71% | -0.8114% | 0.1746 | 2160.00s | 0.5660 | `{'TrailingSL': 36, 'InitialSL': 16, 'NoNewLowExit': 5, 'NoNewHighExit': 2}` | `{'weak': 31, 'strong': 16, 'base': 12}` | 59 | 59 | 0 | 0 |
| `ETHUSDT` | `signal_struct_micro` | 56 | 1.0785% | 2.0751% | 1.6743% | 46.43% | -1.5808% | 0.1750 | 4348.00s | 0.6340 | `{'InitialSL': 22, 'BreakevenSL': 14, 'StructureSL': 8, 'NoNewHighExit': 5, 'NoNewLowExit': 4, 'MaxHoldExit': 3}` | `{'weak': 29, 'strong': 15, 'base': 12}` | 59 | 56 | 0 | 3 |
| `ETHUSDT` | `signal_hybrid_micro` | 59 | -0.0658% | 0.9687% | 0.5536% | 62.71% | -0.8114% | 0.1746 | 2160.00s | 0.5660 | `{'TrailingSL': 36, 'InitialSL': 16, 'NoNewLowExit': 4, 'NoNewHighExit': 2, 'StructureSL': 1}` | `{'weak': 31, 'strong': 16, 'base': 12}` | 59 | 59 | 0 | 0 |
| `ETHUSDT` | `none_struct_micro` | 56 | 1.0785% | 2.0751% | 1.6743% | 46.43% | -1.5808% | 0.1750 | 4348.00s | 0.6340 | `{'InitialSL': 22, 'BreakevenSL': 14, 'StructureSL': 8, 'NoNewHighExit': 5, 'NoNewLowExit': 4, 'MaxHoldExit': 3}` | `{'weak': 29, 'strong': 15, 'base': 12}` | 59 | 56 | 0 | 3 |
| `ETHUSDT` | `signal_struct_skipweak` | 27 | 1.8170% | 2.5233% | 2.2391% | 66.67% | -1.2964% | 0.2556 | 2462.00s | 0.9481 | `{'BreakevenSL': 11, 'InitialSL': 8, 'StructureSL': 4, 'MaxHoldExit': 2, 'NoNewHighExit': 1, 'NoNewLowExit': 1}` | `{'strong': 15, 'base': 12}` | 59 | 27 | 30 | 2 |

## Files

- Summary JSON: `research/btc_eth_2026_jan_apr_micro_breakout_structure_summary.json`
- `BTCUSDT dual_old_fixed` ledger: `research/tmp_btc_eth_2026_jan_apr_micro_breakout_structure_BTCUSDT_dual_old_fixed_ledger.csv`
- `BTCUSDT signal_old_micro` ledger: `research/tmp_btc_eth_2026_jan_apr_micro_breakout_structure_BTCUSDT_signal_old_micro_ledger.csv`
- `BTCUSDT signal_struct_micro` ledger: `research/tmp_btc_eth_2026_jan_apr_micro_breakout_structure_BTCUSDT_signal_struct_micro_ledger.csv`
- `BTCUSDT signal_hybrid_micro` ledger: `research/tmp_btc_eth_2026_jan_apr_micro_breakout_structure_BTCUSDT_signal_hybrid_micro_ledger.csv`
- `BTCUSDT none_struct_micro` ledger: `research/tmp_btc_eth_2026_jan_apr_micro_breakout_structure_BTCUSDT_none_struct_micro_ledger.csv`
- `BTCUSDT signal_struct_skipweak` ledger: `research/tmp_btc_eth_2026_jan_apr_micro_breakout_structure_BTCUSDT_signal_struct_skipweak_ledger.csv`
- `ETHUSDT dual_old_fixed` ledger: `research/tmp_btc_eth_2026_jan_apr_micro_breakout_structure_ETHUSDT_dual_old_fixed_ledger.csv`
- `ETHUSDT signal_old_micro` ledger: `research/tmp_btc_eth_2026_jan_apr_micro_breakout_structure_ETHUSDT_signal_old_micro_ledger.csv`
- `ETHUSDT signal_struct_micro` ledger: `research/tmp_btc_eth_2026_jan_apr_micro_breakout_structure_ETHUSDT_signal_struct_micro_ledger.csv`
- `ETHUSDT signal_hybrid_micro` ledger: `research/tmp_btc_eth_2026_jan_apr_micro_breakout_structure_ETHUSDT_signal_hybrid_micro_ledger.csv`
- `ETHUSDT none_struct_micro` ledger: `research/tmp_btc_eth_2026_jan_apr_micro_breakout_structure_ETHUSDT_none_struct_micro_ledger.csv`
- `ETHUSDT signal_struct_skipweak` ledger: `research/tmp_btc_eth_2026_jan_apr_micro_breakout_structure_ETHUSDT_signal_struct_skipweak_ledger.csv`
