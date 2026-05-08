# BTC/ETH Impulse Bar-Run 1s Replay (2025-01-01T00:00:00+00:00 to 2025-12-31T23:59:59+00:00)

Scope: research-only. Signals use closed 1h bars; execution uses continuous 1s OHLC bars built from local Binance trade ticks. Entry fills at the first 1s close after the impulse bar closes, with 2 bps/side slippage. Realistic accounting also includes maker entry 2 bps and market exit 4 bps.

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Median Hold | Median MFE R | Exit Reasons | Candidates | Entries | Ext Skip | Busy Skip |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| `BTCUSDT` | `break8_body75` | 130 | -1.5622% | 1.0293% | -0.0145% | 46.15% | -3.1132% | 4666.50s | 0.8175 | `{'InitialSL': 58, 'BreakevenSL': 43, 'NoNewHighExit': 10, 'TrailingSL': 9, 'NoNewLowExit': 5, 'MaxHoldExit': 4, 'EMA8Exit': 1}` | 140 | 130 | 0 | 9 |
| `BTCUSDT` | `break12_body65` | 151 | -2.0853% | 0.9149% | -0.2948% | 41.72% | -3.3249% | 4472.00s | 0.7163 | `{'InitialSL': 73, 'BreakevenSL': 42, 'NoNewHighExit': 13, 'TrailingSL': 10, 'NoNewLowExit': 6, 'MaxHoldExit': 6, 'EMA8Exit': 1}` | 164 | 151 | 0 | 11 |
| `BTCUSDT` | `break8_body65_pre2` | 77 | -2.8589% | -1.3515% | -1.9569% | 38.96% | -2.9541% | 4379.00s | 0.5052 | `{'InitialSL': 41, 'BreakevenSL': 20, 'NoNewHighExit': 6, 'TrailingSL': 5, 'MaxHoldExit': 3, 'NoNewLowExit': 2}` | 78 | 77 | 0 | 1 |
| `BTCUSDT` | `break8_body65_range1` | 148 | -1.7283% | 1.2227% | 0.0327% | 44.59% | -3.3995% | 3989.00s | 0.7234 | `{'InitialSL': 71, 'BreakevenSL': 45, 'TrailingSL': 10, 'NoNewLowExit': 8, 'NoNewHighExit': 8, 'MaxHoldExit': 6}` | 160 | 148 | 0 | 10 |
| `BTCUSDT` | `break8_body65_close85` | 137 | -2.0393% | 0.6805% | -0.4154% | 40.88% | -3.5982% | 4596.00s | 0.6351 | `{'InitialSL': 68, 'BreakevenSL': 39, 'NoNewHighExit': 11, 'TrailingSL': 7, 'MaxHoldExit': 6, 'NoNewLowExit': 5, 'EMA8Exit': 1}` | 149 | 137 | 0 | 10 |
| `ETHUSDT` | `break8_body75` | 109 | -1.1551% | 1.0241% | 0.1464% | 45.87% | -2.4882% | 5529.00s | 0.7790 | `{'InitialSL': 50, 'BreakevenSL': 26, 'TrailingSL': 16, 'NoNewLowExit': 6, 'NoNewHighExit': 6, 'MaxHoldExit': 4, 'EMA8Exit': 1}` | 112 | 109 | 0 | 3 |
| `ETHUSDT` | `break12_body65` | 114 | -0.1617% | 2.1401% | 1.2134% | 52.63% | -1.6553% | 6138.00s | 1.0441 | `{'InitialSL': 45, 'BreakevenSL': 31, 'TrailingSL': 19, 'NoNewHighExit': 10, 'NoNewLowExit': 6, 'MaxHoldExit': 3}` | 121 | 114 | 0 | 7 |
| `ETHUSDT` | `break8_body65_pre2` | 73 | -1.7772% | -0.3317% | -0.9128% | 52.05% | -2.0520% | 5681.00s | 1.0309 | `{'InitialSL': 29, 'BreakevenSL': 25, 'TrailingSL': 9, 'NoNewHighExit': 6, 'NoNewLowExit': 2, 'EMA8Exit': 1, 'MaxHoldExit': 1}` | 75 | 73 | 0 | 2 |
| `ETHUSDT` | `break8_body65_range1` | 132 | -0.0010% | 2.6744% | 1.5955% | 53.79% | -2.3689% | 5768.50s | 1.0763 | `{'InitialSL': 55, 'BreakevenSL': 39, 'TrailingSL': 19, 'NoNewHighExit': 9, 'NoNewLowExit': 5, 'MaxHoldExit': 5}` | 141 | 132 | 0 | 9 |
| `ETHUSDT` | `break8_body65_close85` | 103 | -0.2089% | 1.8682% | 1.0321% | 48.54% | -1.7714% | 6325.00s | 0.8620 | `{'InitialSL': 44, 'BreakevenSL': 26, 'TrailingSL': 15, 'NoNewHighExit': 8, 'NoNewLowExit': 6, 'MaxHoldExit': 4}` | 106 | 103 | 0 | 3 |

## Files

- Summary JSON: `research/btc_eth_2025_impulse_bar_run_quality_sweep_summary.json`
- `BTCUSDT break8_body75` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_quality_sweep_BTCUSDT_break8_body75_ledger.csv`
- `BTCUSDT break12_body65` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_quality_sweep_BTCUSDT_break12_body65_ledger.csv`
- `BTCUSDT break8_body65_pre2` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_quality_sweep_BTCUSDT_break8_body65_pre2_ledger.csv`
- `BTCUSDT break8_body65_range1` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_quality_sweep_BTCUSDT_break8_body65_range1_ledger.csv`
- `BTCUSDT break8_body65_close85` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_quality_sweep_BTCUSDT_break8_body65_close85_ledger.csv`
- `ETHUSDT break8_body75` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_quality_sweep_ETHUSDT_break8_body75_ledger.csv`
- `ETHUSDT break12_body65` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_quality_sweep_ETHUSDT_break12_body65_ledger.csv`
- `ETHUSDT break8_body65_pre2` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_quality_sweep_ETHUSDT_break8_body65_pre2_ledger.csv`
- `ETHUSDT break8_body65_range1` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_quality_sweep_ETHUSDT_break8_body65_range1_ledger.csv`
- `ETHUSDT break8_body65_close85` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_quality_sweep_ETHUSDT_break8_body65_close85_ledger.csv`
