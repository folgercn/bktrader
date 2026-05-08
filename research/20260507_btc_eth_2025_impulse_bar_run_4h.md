# BTC/ETH Impulse Bar-Run 1s Replay (2025-01-01T00:00:00+00:00 to 2025-12-31T23:59:59+00:00)

Scope: research-only. Signals use closed 4h bars with 1d EMA20 trend context; execution uses continuous 1s OHLC bars built from local Binance trade ticks. Entry fills at the first 1s close after the impulse bar closes, with 2 bps/side slippage. Realistic accounting also includes maker entry 2 bps and market exit 4 bps.

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Median Hold | Median MFE R | Exit Reasons | Candidates | Entries | Ext Skip | Busy Skip |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| `BTCUSDT` | `break8_body65` | 47 | -1.5830% | -0.6534% | -1.0262% | 51.06% | -1.8876% | 21154.00s | 0.6788 | `{'MaxHoldExit': 23, 'InitialSL': 15, 'BreakevenSL': 9}` | 48 | 47 | 0 | 1 |
| `BTCUSDT` | `break8_body65_range1` | 41 | -0.9500% | -0.1345% | -0.4614% | 51.22% | -1.5838% | 21154.00s | 0.7092 | `{'MaxHoldExit': 20, 'InitialSL': 14, 'BreakevenSL': 7}` | 42 | 41 | 0 | 1 |
| `BTCUSDT` | `break12_body65` | 34 | -0.8292% | -0.1525% | -0.4237% | 52.94% | -0.8510% | 21600.00s | 0.7156 | `{'MaxHoldExit': 18, 'InitialSL': 10, 'BreakevenSL': 6}` | 35 | 34 | 0 | 1 |
| `BTCUSDT` | `break8_body75` | 34 | -1.0072% | -0.3317% | -0.6024% | 55.88% | -1.0072% | 18920.00s | 0.7239 | `{'MaxHoldExit': 17, 'InitialSL': 9, 'BreakevenSL': 8}` | 34 | 34 | 0 | 0 |
| `ETHUSDT` | `break8_body65` | 39 | -2.0380% | -1.2701% | -1.5782% | 46.15% | -2.4855% | 21600.00s | 0.6372 | `{'MaxHoldExit': 24, 'InitialSL': 9, 'BreakevenSL': 6}` | 41 | 39 | 0 | 2 |
| `ETHUSDT` | `break8_body65_range1` | 37 | -1.5695% | -0.8378% | -1.1313% | 48.65% | -2.0192% | 21600.00s | 0.8374 | `{'MaxHoldExit': 23, 'InitialSL': 8, 'BreakevenSL': 6}` | 39 | 37 | 0 | 2 |
| `ETHUSDT` | `break12_body65` | 30 | -1.8179% | -1.2260% | -1.4636% | 46.67% | -2.8584% | 21600.00s | 0.7127 | `{'MaxHoldExit': 17, 'InitialSL': 8, 'BreakevenSL': 5}` | 32 | 30 | 0 | 2 |
| `ETHUSDT` | `break8_body75` | 29 | -1.1110% | -0.5353% | -0.7662% | 48.28% | -1.4333% | 21600.00s | 0.8374 | `{'MaxHoldExit': 18, 'BreakevenSL': 6, 'InitialSL': 5}` | 30 | 29 | 0 | 1 |

## Files

- Summary JSON: `research/btc_eth_2025_impulse_bar_run_4h_summary.json`
- `BTCUSDT break8_body65` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_4h_BTCUSDT_break8_body65_ledger.csv`
- `BTCUSDT break8_body65_range1` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_4h_BTCUSDT_break8_body65_range1_ledger.csv`
- `BTCUSDT break12_body65` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_4h_BTCUSDT_break12_body65_ledger.csv`
- `BTCUSDT break8_body75` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_4h_BTCUSDT_break8_body75_ledger.csv`
- `ETHUSDT break8_body65` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_4h_ETHUSDT_break8_body65_ledger.csv`
- `ETHUSDT break8_body65_range1` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_4h_ETHUSDT_break8_body65_range1_ledger.csv`
- `ETHUSDT break12_body65` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_4h_ETHUSDT_break12_body65_ledger.csv`
- `ETHUSDT break8_body75` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_4h_ETHUSDT_break8_body75_ledger.csv`
