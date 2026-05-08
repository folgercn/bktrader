# BTC/ETH Impulse Bar-Run 1s Replay (2025-01-01T00:00:00+00:00 to 2025-12-31T23:59:59+00:00)

Scope: research-only. Signals use closed 1h bars with 4h EMA20 trend context; execution uses continuous 1s OHLC bars built from local Binance trade ticks. Entry fills at the first 1s close after the impulse bar closes, with 2 bps/side slippage. Realistic accounting also includes maker entry 2 bps and market exit 4 bps.

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Median Hold | Median MFE R | Exit Reasons | Candidates | Entries | Ext Skip | Confirm Miss | Early Rev | Busy Skip |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|---:|---:|
| `ETHUSDT` | `base_range1` | 132 | -0.0010% | 2.6744% | 1.5955% | 53.79% | -2.3689% | 5768.50s | 1.0763 | `{'InitialSL': 55, 'BreakevenSL': 39, 'TrailingSL': 19, 'NoNewHighExit': 9, 'NoNewLowExit': 5, 'MaxHoldExit': 5}` | 141 | 132 | 0 | 0 | 0 | 9 |
| `ETHUSDT` | `confirm03_fail10` | 105 | 0.7230% | 2.8609% | 2.0000% | 53.33% | -2.7174% | 5657.00s | 1.0478 | `{'InitialSL': 44, 'BreakevenSL': 32, 'TrailingSL': 14, 'NoNewHighExit': 7, 'MaxHoldExit': 5, 'NoNewLowExit': 3}` | 141 | 105 | 0 | 0 | 28 | 8 |
| `ETHUSDT` | `confirm05_fail15` | 96 | 1.6832% | 3.6540% | 2.8611% | 55.21% | -2.1080% | 5839.00s | 1.0762 | `{'InitialSL': 38, 'BreakevenSL': 30, 'TrailingSL': 13, 'NoNewHighExit': 7, 'MaxHoldExit': 5, 'NoNewLowExit': 3}` | 141 | 96 | 0 | 0 | 37 | 8 |
| `ETHUSDT` | `confirm08_fail20` | 87 | 1.4409% | 3.2205% | 2.5052% | 52.87% | -1.5906% | 6212.00s | 1.0550 | `{'InitialSL': 36, 'BreakevenSL': 22, 'TrailingSL': 13, 'NoNewHighExit': 7, 'NoNewLowExit': 5, 'MaxHoldExit': 4}` | 141 | 87 | 1 | 0 | 45 | 8 |
| `ETHUSDT` | `confirm05_fail15_w1800` | 96 | 1.6832% | 3.6540% | 2.8611% | 55.21% | -2.1080% | 5839.00s | 1.0762 | `{'InitialSL': 38, 'BreakevenSL': 30, 'TrailingSL': 13, 'NoNewHighExit': 7, 'MaxHoldExit': 5, 'NoNewLowExit': 3}` | 141 | 96 | 0 | 0 | 37 | 8 |
| `ETHUSDT` | `confirm05_nofail` | 117 | 0.6473% | 3.0299% | 2.0701% | 52.99% | -2.2515% | 5705.00s | 1.0322 | `{'InitialSL': 49, 'BreakevenSL': 34, 'TrailingSL': 17, 'NoNewHighExit': 8, 'MaxHoldExit': 5, 'NoNewLowExit': 4}` | 141 | 117 | 0 | 16 | 0 | 8 |

## Files

- Summary JSON: `research/btc_eth_2025_impulse_bar_run_confirm_sweep_summary.json`
- `ETHUSDT base_range1` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_confirm_sweep_ETHUSDT_base_range1_ledger.csv`
- `ETHUSDT confirm03_fail10` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_confirm_sweep_ETHUSDT_confirm03_fail10_ledger.csv`
- `ETHUSDT confirm05_fail15` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_confirm_sweep_ETHUSDT_confirm05_fail15_ledger.csv`
- `ETHUSDT confirm08_fail20` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_confirm_sweep_ETHUSDT_confirm08_fail20_ledger.csv`
- `ETHUSDT confirm05_fail15_w1800` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_confirm_sweep_ETHUSDT_confirm05_fail15_w1800_ledger.csv`
- `ETHUSDT confirm05_nofail` ledger: `research/tmp_btc_eth_2025_impulse_bar_run_confirm_sweep_ETHUSDT_confirm05_nofail_ledger.csv`
