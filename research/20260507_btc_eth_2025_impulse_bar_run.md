# BTC/ETH Impulse Bar-Run 1s Replay (2025-01-01T00:00:00+00:00 to 2025-12-31T23:59:59+00:00)

Scope: research-only. Signals use closed 1h bars; execution uses continuous 1s OHLC bars built from local Binance trade ticks. Entry fills at the first 1s close after the impulse bar closes, with 2 bps/side slippage. Realistic accounting also includes maker entry 2 bps and market exit 4 bps.

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Median Hold | Median MFE R | Exit Reasons | Candidates | Entries | Ext Skip | Busy Skip |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| `BTCUSDT` | `break12_body55` | 170 | -2.9961% | 0.3568% | -0.9966% | 41.18% | -4.0133% | 4254.50s | 0.7106 | `{'InitialSL': 84, 'BreakevenSL': 48, 'NoNewHighExit': 14, 'TrailingSL': 11, 'NoNewLowExit': 6, 'MaxHoldExit': 6, 'EMA8Exit': 1}` | 187 | 170 | 0 | 15 |
| `BTCUSDT` | `break8_body65` | 174 | -2.5858% | 0.8620% | -0.5302% | 43.68% | -4.1159% | 4425.50s | 0.7318 | `{'InitialSL': 82, 'BreakevenSL': 51, 'NoNewHighExit': 14, 'TrailingSL': 11, 'MaxHoldExit': 8, 'NoNewLowExit': 7, 'EMA8Exit': 1}` | 194 | 174 | 0 | 18 |
| `ETHUSDT` | `break12_body55` | 131 | -1.4897% | 1.1252% | 0.0712% | 50.38% | -2.1818% | 5929.00s | 0.9544 | `{'InitialSL': 54, 'BreakevenSL': 34, 'TrailingSL': 21, 'NoNewHighExit': 11, 'NoNewLowExit': 8, 'MaxHoldExit': 3}` | 142 | 131 | 0 | 11 |
| `ETHUSDT` | `break8_body65` | 149 | -0.4446% | 2.5670% | 1.3514% | 53.02% | -2.2382% | 5854.00s | 1.0673 | `{'InitialSL': 60, 'BreakevenSL': 43, 'TrailingSL': 23, 'NoNewHighExit': 11, 'NoNewLowExit': 6, 'MaxHoldExit': 5, 'EMA8Exit': 1}` | 160 | 149 | 0 | 11 |

## Files

- Summary JSON: `research/btc_eth_2025_impulse_bar_run_summary.json`
- `BTCUSDT break12_body55` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_BTCUSDT_break12_body55_ledger.csv`
- `BTCUSDT break8_body65` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_BTCUSDT_break8_body65_ledger.csv`
- `ETHUSDT break12_body55` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_ETHUSDT_break12_body55_ledger.csv`
- `ETHUSDT break8_body65` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_ETHUSDT_break8_body65_ledger.csv`
