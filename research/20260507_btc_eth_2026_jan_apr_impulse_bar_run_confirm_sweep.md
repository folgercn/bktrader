# BTC/ETH Impulse Bar-Run 1s Replay (2026-01-01T00:00:00+00:00 to 2026-04-30T23:59:59+00:00)

Scope: research-only. Signals use closed 1h bars with 4h EMA20 trend context; execution uses continuous 1s OHLC bars built from local Binance trade ticks. Entry fills at the first 1s close after the impulse bar closes, with 2 bps/side slippage. Realistic accounting also includes maker entry 2 bps and market exit 4 bps.

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Median Hold | Median MFE R | Exit Reasons | Candidates | Entries | Ext Skip | Confirm Miss | Early Rev | Busy Skip |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|---:|---:|
| `BTCUSDT` | `base_range1` | 45 | 0.3704% | 1.2789% | 0.9138% | 48.89% | -1.4738% | 3297.00s | 0.9077 | `{'InitialSL': 22, 'BreakevenSL': 13, 'TrailingSL': 7, 'NoNewHighExit': 2, 'MaxHoldExit': 1}` | 50 | 45 | 0 | 0 | 0 | 5 |
| `BTCUSDT` | `confirm03_fail10` | 36 | 1.1245% | 1.8559% | 1.5621% | 50.00% | -1.1216% | 5851.50s | 1.0151 | `{'InitialSL': 16, 'BreakevenSL': 8, 'TrailingSL': 8, 'NoNewHighExit': 2, 'NoNewLowExit': 1, 'MaxHoldExit': 1}` | 50 | 36 | 0 | 0 | 8 | 6 |
| `BTCUSDT` | `confirm05_fail15` | 32 | 1.5375% | 2.1900% | 1.9280% | 56.25% | -1.1235% | 7694.50s | 1.2220 | `{'InitialSL': 13, 'BreakevenSL': 8, 'TrailingSL': 8, 'NoNewHighExit': 1, 'NoNewLowExit': 1, 'MaxHoldExit': 1}` | 50 | 32 | 0 | 0 | 12 | 6 |
| `BTCUSDT` | `confirm08_fail20` | 32 | 0.6648% | 1.3117% | 1.0520% | 56.25% | -0.9336% | 4017.50s | 1.2301 | `{'InitialSL': 14, 'BreakevenSL': 9, 'TrailingSL': 8, 'NoNewHighExit': 1}` | 50 | 32 | 1 | 0 | 12 | 5 |
| `BTCUSDT` | `confirm05_nofail` | 40 | 0.6932% | 1.5029% | 1.1776% | 47.50% | -1.3523% | 4486.50s | 0.8241 | `{'InitialSL': 19, 'BreakevenSL': 9, 'TrailingSL': 8, 'NoNewHighExit': 2, 'NoNewLowExit': 1, 'MaxHoldExit': 1}` | 50 | 40 | 1 | 3 | 0 | 6 |
| `ETHUSDT` | `base_range1` | 41 | 0.8090% | 1.6396% | 1.3061% | 46.34% | -1.3011% | 5618.00s | 0.8830 | `{'InitialSL': 17, 'BreakevenSL': 9, 'NoNewHighExit': 5, 'TrailingSL': 5, 'NoNewLowExit': 3, 'MaxHoldExit': 2}` | 44 | 41 | 0 | 0 | 0 | 3 |
| `ETHUSDT` | `confirm03_fail10` | 32 | 1.5249% | 2.1769% | 1.9153% | 50.00% | -0.9435% | 7193.50s | 0.9584 | `{'InitialSL': 11, 'BreakevenSL': 8, 'NoNewHighExit': 5, 'TrailingSL': 4, 'NoNewLowExit': 3, 'MaxHoldExit': 1}` | 44 | 32 | 0 | 0 | 9 | 3 |
| `ETHUSDT` | `confirm05_fail15` | 32 | 0.3199% | 0.9647% | 0.7058% | 37.50% | -0.9900% | 7193.50s | 0.6517 | `{'InitialSL': 14, 'NoNewHighExit': 5, 'BreakevenSL': 5, 'NoNewLowExit': 4, 'TrailingSL': 3, 'MaxHoldExit': 1}` | 44 | 32 | 0 | 0 | 10 | 2 |
| `ETHUSDT` | `confirm08_fail20` | 27 | 0.8902% | 1.4371% | 1.2175% | 40.74% | -0.8231% | 5842.00s | 0.6871 | `{'InitialSL': 12, 'BreakevenSL': 5, 'NoNewLowExit': 4, 'NoNewHighExit': 3, 'TrailingSL': 2, 'MaxHoldExit': 1}` | 44 | 27 | 0 | 1 | 14 | 2 |
| `ETHUSDT` | `confirm05_nofail` | 37 | 0.1153% | 0.8597% | 0.5607% | 40.54% | -1.1998% | 6471.00s | 0.6699 | `{'InitialSL': 16, 'BreakevenSL': 8, 'NoNewHighExit': 5, 'NoNewLowExit': 4, 'TrailingSL': 3, 'MaxHoldExit': 1}` | 44 | 37 | 0 | 5 | 0 | 2 |

## Files

- Summary JSON: `research/btc_eth_2026_jan_apr_impulse_bar_run_confirm_sweep_summary.json`
- `BTCUSDT base_range1` ledger: `research/tmp_btc_eth_2026_jan_apr_impulse_bar_run_confirm_sweep_BTCUSDT_base_range1_ledger.csv`
- `BTCUSDT confirm03_fail10` ledger: `research/tmp_btc_eth_2026_jan_apr_impulse_bar_run_confirm_sweep_BTCUSDT_confirm03_fail10_ledger.csv`
- `BTCUSDT confirm05_fail15` ledger: `research/tmp_btc_eth_2026_jan_apr_impulse_bar_run_confirm_sweep_BTCUSDT_confirm05_fail15_ledger.csv`
- `BTCUSDT confirm08_fail20` ledger: `research/tmp_btc_eth_2026_jan_apr_impulse_bar_run_confirm_sweep_BTCUSDT_confirm08_fail20_ledger.csv`
- `BTCUSDT confirm05_nofail` ledger: `research/tmp_btc_eth_2026_jan_apr_impulse_bar_run_confirm_sweep_BTCUSDT_confirm05_nofail_ledger.csv`
- `ETHUSDT base_range1` ledger: `research/tmp_btc_eth_2026_jan_apr_impulse_bar_run_confirm_sweep_ETHUSDT_base_range1_ledger.csv`
- `ETHUSDT confirm03_fail10` ledger: `research/tmp_btc_eth_2026_jan_apr_impulse_bar_run_confirm_sweep_ETHUSDT_confirm03_fail10_ledger.csv`
- `ETHUSDT confirm05_fail15` ledger: `research/tmp_btc_eth_2026_jan_apr_impulse_bar_run_confirm_sweep_ETHUSDT_confirm05_fail15_ledger.csv`
- `ETHUSDT confirm08_fail20` ledger: `research/tmp_btc_eth_2026_jan_apr_impulse_bar_run_confirm_sweep_ETHUSDT_confirm08_fail20_ledger.csv`
- `ETHUSDT confirm05_nofail` ledger: `research/tmp_btc_eth_2026_jan_apr_impulse_bar_run_confirm_sweep_ETHUSDT_confirm05_nofail_ledger.csv`
