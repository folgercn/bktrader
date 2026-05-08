# BTC/ETH 2026 Jan-Apr Impulse Bar-Run 1s Replay

Scope: research-only. Signals use closed 1h bars; execution uses continuous 1s OHLC bars built from local Binance trade ticks. Entry fills at the first 1s close after the impulse bar closes, with 2 bps/side slippage. Realistic accounting also includes maker entry 2 bps and market exit 4 bps.

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Median Hold | Median MFE R | Exit Reasons | Candidates | Entries | Ext Skip | Busy Skip |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| `BTCUSDT` | `break8_body55` | 59 | -0.0693% | 1.1178% | 0.6407% | 49.15% | -1.5002% | 4299.00s | 0.9077 | `{'InitialSL': 29, 'BreakevenSL': 17, 'TrailingSL': 8, 'NoNewHighExit': 2, 'NoNewLowExit': 2, 'MaxHoldExit': 1}` | 66 | 59 | 0 | 7 |
| `BTCUSDT` | `break12_body55` | 43 | -1.0440% | -0.1887% | -0.5319% | 44.19% | -1.4995% | 3297.00s | 0.7807 | `{'InitialSL': 23, 'BreakevenSL': 11, 'TrailingSL': 6, 'NoNewLowExit': 2, 'NoNewHighExit': 1}` | 49 | 43 | 0 | 6 |
| `BTCUSDT` | `break8_body65` | 47 | 0.2130% | 1.1607% | 0.7798% | 48.94% | -1.4738% | 3297.00s | 0.9077 | `{'InitialSL': 23, 'BreakevenSL': 13, 'TrailingSL': 8, 'NoNewHighExit': 2, 'MaxHoldExit': 1}` | 53 | 47 | 0 | 6 |
| `ETHUSDT` | `break8_body55` | 49 | 0.9645% | 1.9592% | 1.5598% | 48.98% | -1.2932% | 5618.00s | 0.9131 | `{'InitialSL': 19, 'BreakevenSL': 13, 'NoNewHighExit': 6, 'TrailingSL': 5, 'MaxHoldExit': 3, 'NoNewLowExit': 3}` | 53 | 49 | 0 | 4 |
| `ETHUSDT` | `break12_body55` | 37 | 1.3860% | 2.1397% | 1.8370% | 48.65% | -1.0914% | 6471.00s | 0.8830 | `{'InitialSL': 13, 'BreakevenSL': 9, 'NoNewHighExit': 5, 'NoNewLowExit': 4, 'TrailingSL': 3, 'MaxHoldExit': 3}` | 38 | 37 | 0 | 1 |
| `ETHUSDT` | `break8_body65` | 41 | 0.8090% | 1.6396% | 1.3061% | 46.34% | -1.3011% | 5618.00s | 0.8830 | `{'InitialSL': 17, 'BreakevenSL': 9, 'NoNewHighExit': 5, 'TrailingSL': 5, 'NoNewLowExit': 3, 'MaxHoldExit': 2}` | 44 | 41 | 0 | 3 |

## Files

- Summary JSON: `research/btc_eth_2026_jan_apr_impulse_bar_run_summary.json`
- `BTCUSDT break8_body55` ledger: `research/tmp_btc_eth_2026_jan_apr_impulse_bar_run_BTCUSDT_break8_body55_ledger.csv`
- `BTCUSDT break12_body55` ledger: `research/tmp_btc_eth_2026_jan_apr_impulse_bar_run_BTCUSDT_break12_body55_ledger.csv`
- `BTCUSDT break8_body65` ledger: `research/tmp_btc_eth_2026_jan_apr_impulse_bar_run_BTCUSDT_break8_body65_ledger.csv`
- `ETHUSDT break8_body55` ledger: `research/tmp_btc_eth_2026_jan_apr_impulse_bar_run_ETHUSDT_break8_body55_ledger.csv`
- `ETHUSDT break12_body55` ledger: `research/tmp_btc_eth_2026_jan_apr_impulse_bar_run_ETHUSDT_break12_body55_ledger.csv`
- `ETHUSDT break8_body65` ledger: `research/tmp_btc_eth_2026_jan_apr_impulse_bar_run_ETHUSDT_break8_body65_ledger.csv`
