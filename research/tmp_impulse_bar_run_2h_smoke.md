# BTC/ETH Impulse Bar-Run 1s Replay (2025-01-01T00:00:00+00:00 to 2025-01-07T23:59:59+00:00)

Scope: research-only. Signals use closed 2h bars with 8h EMA20 trend context; execution uses continuous 1s OHLC bars built from local Binance trade ticks. Entry fills at the first 1s close after the impulse bar closes, with 2 bps/side slippage. Realistic accounting also includes maker entry 2 bps and market exit 4 bps.

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip No Fee | Win Rate | Max DD | Median Hold | Median MFE R | Exit Reasons | Candidates | Entries | Ext Skip | Busy Skip |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|
| `BTCUSDT` | `break8_body65` | 0 | 0.0000% | 0.0000% | 0.0000% | 0.00% | 0.0000% | 0.00s | 0.0000 | `{}` | 0 | 0 | 0 | 0 |

## Files

- Summary JSON: `research/tmp_impulse_bar_run_2h_smoke_summary.json`
- `BTCUSDT break8_body65` ledger: `research/tmp_impulse_bar_run_2h_smoke_BTCUSDT_break8_body65_ledger.csv`
