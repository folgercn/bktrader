# BTC/ETH 2026 Jan-Apr Impulse Bar-Run 1s Replay

Scope: research-only. Signals use closed 1h bars; execution uses continuous 1s OHLC bars built from local Binance trade ticks. Entry fills at the first 1s close after the impulse bar closes, with 2 bps/side slippage. Realistic accounting also includes maker entry 2 bps and market exit 4 bps.

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Median Hold | Median MFE R | Exit Reasons | Candidates | Entries | Ext Skip | Busy Skip |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| `BTCUSDT` | `break8_body55` | 2 | 0.1697% | 0.2098% | 0.1937% | 50.00% | -0.1024% | 9652.00s | 4.2926 | `{'NoNewHighExit': 1, 'InitialSL': 1}` | 2 | 2 | 0 | 0 |

## Files

- Summary JSON: `research/tmp_impulse_bar_run_smoke_summary.json`
- `BTCUSDT break8_body55` ledger: `research/tmp_impulse_bar_run_smoke_BTCUSDT_break8_body55_ledger.csv`
