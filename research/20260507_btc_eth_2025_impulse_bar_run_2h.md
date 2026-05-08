# BTC/ETH Impulse Bar-Run 1s Replay (2025-01-01T00:00:00+00:00 to 2025-12-31T23:59:59+00:00)

Scope: research-only. Signals use closed 2h bars with 8h EMA20 trend context; execution uses continuous 1s OHLC bars built from local Binance trade ticks. Entry fills at the first 1s close after the impulse bar closes, with 2 bps/side slippage. Realistic accounting also includes maker entry 2 bps and market exit 4 bps.

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Median Hold | Median MFE R | Exit Reasons | Candidates | Entries | Ext Skip | Busy Skip |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| `BTCUSDT` | `break8_body65` | 102 | -3.8115% | -1.8293% | -2.6265% | 43.14% | -3.8564% | 8348.00s | 0.6954 | `{'InitialSL': 45, 'BreakevenSL': 25, 'MaxHoldExit': 23, 'NoNewHighExit': 4, 'NoNewLowExit': 3, 'EMA8Exit': 2}` | 108 | 102 | 0 | 6 |
| `BTCUSDT` | `break8_body65_range1` | 85 | -2.5916% | -0.9216% | -1.5927% | 48.24% | -2.8417% | 7734.00s | 0.8409 | `{'InitialSL': 39, 'BreakevenSL': 24, 'MaxHoldExit': 21, 'NoNewLowExit': 1}` | 88 | 85 | 0 | 3 |
| `BTCUSDT` | `break12_body65` | 75 | -3.2475% | -1.7853% | -2.3725% | 44.00% | -3.4781% | 7829.00s | 0.6965 | `{'InitialSL': 35, 'BreakevenSL': 21, 'MaxHoldExit': 17, 'NoNewHighExit': 2}` | 80 | 75 | 0 | 5 |
| `BTCUSDT` | `break8_body75` | 67 | -1.3965% | -0.0669% | -0.6005% | 49.25% | -1.5902% | 8174.00s | 0.8060 | `{'InitialSL': 29, 'BreakevenSL': 17, 'MaxHoldExit': 17, 'NoNewLowExit': 2, 'NoNewHighExit': 1, 'EMA8Exit': 1}` | 71 | 67 | 0 | 4 |
| `ETHUSDT` | `break8_body65` | 93 | -3.0424% | -1.2203% | -1.9540% | 49.46% | -3.3745% | 11390.00s | 0.7640 | `{'InitialSL': 35, 'BreakevenSL': 22, 'MaxHoldExit': 15, 'NoNewHighExit': 7, 'NoNewLowExit': 7, 'TrailingSL': 7}` | 98 | 93 | 0 | 5 |
| `ETHUSDT` | `break8_body65_range1` | 89 | -3.3968% | -1.6602% | -2.3592% | 48.31% | -3.8699% | 11322.00s | 0.7640 | `{'InitialSL': 34, 'BreakevenSL': 22, 'MaxHoldExit': 15, 'NoNewHighExit': 7, 'TrailingSL': 6, 'NoNewLowExit': 5}` | 93 | 89 | 0 | 4 |
| `ETHUSDT` | `break12_body65` | 72 | -1.4993% | -0.0689% | -0.6444% | 50.00% | -2.3487% | 12960.00s | 0.7646 | `{'InitialSL': 28, 'BreakevenSL': 15, 'MaxHoldExit': 15, 'NoNewHighExit': 5, 'TrailingSL': 5, 'NoNewLowExit': 4}` | 75 | 72 | 0 | 3 |
| `ETHUSDT` | `break8_body75` | 63 | -0.0941% | 1.1735% | 0.6640% | 55.56% | -1.9879% | 11915.00s | 1.0041 | `{'InitialSL': 20, 'BreakevenSL': 17, 'MaxHoldExit': 12, 'NoNewHighExit': 6, 'TrailingSL': 6, 'NoNewLowExit': 2}` | 64 | 63 | 0 | 1 |

## Files

- Summary JSON: `research/btc_eth_2025_impulse_bar_run_2h_summary.json`
- `BTCUSDT break8_body65` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_2h_BTCUSDT_break8_body65_ledger.csv`
- `BTCUSDT break8_body65_range1` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_2h_BTCUSDT_break8_body65_range1_ledger.csv`
- `BTCUSDT break12_body65` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_2h_BTCUSDT_break12_body65_ledger.csv`
- `BTCUSDT break8_body75` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_2h_BTCUSDT_break8_body75_ledger.csv`
- `ETHUSDT break8_body65` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_2h_ETHUSDT_break8_body65_ledger.csv`
- `ETHUSDT break8_body65_range1` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_2h_ETHUSDT_break8_body65_range1_ledger.csv`
- `ETHUSDT break12_body65` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_2h_ETHUSDT_break12_body65_ledger.csv`
- `ETHUSDT break8_body75` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_2h_ETHUSDT_break8_body75_ledger.csv`
