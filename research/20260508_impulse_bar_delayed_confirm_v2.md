# Impulse Bar Delayed Confirm V2 1s Replay (2026-01-01T00:00:00+00:00 to 2026-04-30T23:59:59+00:00)

Scope: research-only. Extends impulse bar confirm sweep with VWAP slope, volume acceleration, intrabar close position, and post-confirm retrace gates.

成本：滑点 2bps/side，手续费 maker entry 2bps + market exit 4bps。

| Symbol | Variant | Trades | Realistic | Raw | 2bps Slip | Win Rate | Max DD | Diag |
|---|---|---:|---:|---:|---:|---:|---:|---|
| `BTCUSDT` | `c03_f10` | 36 | 1.0338% | 1.7647% | 1.4711% | 50.00% | -1.1216% | `{'candidate_signals': 53, 'entries': 36, 'entry_extension_skipped': 0, 'busy_skipped': 7, 'confirm_timeout_skipped': 2, 'early_reversal_skipped': 8, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 0, 'closepos_reject_skipped': 0, 'post_retrace_reject_skipped': 0, 'min_stop_skipped': 0}` |
| `BTCUSDT` | `c03_f10_vwap` | 36 | 1.0338% | 1.7647% | 1.4711% | 50.00% | -1.1216% | `{'candidate_signals': 53, 'entries': 36, 'entry_extension_skipped': 0, 'busy_skipped': 7, 'confirm_timeout_skipped': 2, 'early_reversal_skipped': 8, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 0, 'closepos_reject_skipped': 0, 'post_retrace_reject_skipped': 0, 'min_stop_skipped': 0}` |
| `BTCUSDT` | `c03_f10_vol` | 21 | 1.3578% | 1.7849% | 1.6134% | 47.62% | -0.7275% | `{'candidate_signals': 53, 'entries': 21, 'entry_extension_skipped': 0, 'busy_skipped': 5, 'confirm_timeout_skipped': 2, 'early_reversal_skipped': 9, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 16, 'closepos_reject_skipped': 0, 'post_retrace_reject_skipped': 0, 'min_stop_skipped': 0}` |
| `BTCUSDT` | `c03_f10_full` | 21 | 1.3578% | 1.7849% | 1.6134% | 47.62% | -0.7275% | `{'candidate_signals': 53, 'entries': 21, 'entry_extension_skipped': 0, 'busy_skipped': 5, 'confirm_timeout_skipped': 2, 'early_reversal_skipped': 9, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 16, 'closepos_reject_skipped': 0, 'post_retrace_reject_skipped': 0, 'min_stop_skipped': 0}` |
| `BTCUSDT` | `c05_f15` | 31 | 1.2036% | 1.8336% | 1.5806% | 54.84% | -1.1235% | `{'candidate_signals': 53, 'entries': 31, 'entry_extension_skipped': 0, 'busy_skipped': 7, 'confirm_timeout_skipped': 5, 'early_reversal_skipped': 10, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 0, 'closepos_reject_skipped': 0, 'post_retrace_reject_skipped': 0, 'min_stop_skipped': 0}` |
| `BTCUSDT` | `c05_f15_full` | 19 | 1.1530% | 1.5386% | 1.3837% | 47.37% | -0.5480% | `{'candidate_signals': 53, 'entries': 19, 'entry_extension_skipped': 0, 'busy_skipped': 5, 'confirm_timeout_skipped': 5, 'early_reversal_skipped': 11, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 13, 'closepos_reject_skipped': 0, 'post_retrace_reject_skipped': 0, 'min_stop_skipped': 0}` |
| `BTCUSDT` | `c03_f10_closepos` | 34 | 1.2774% | 1.9691% | 1.6913% | 52.94% | -0.9973% | `{'candidate_signals': 53, 'entries': 34, 'entry_extension_skipped': 0, 'busy_skipped': 7, 'confirm_timeout_skipped': 2, 'early_reversal_skipped': 8, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 0, 'closepos_reject_skipped': 2, 'post_retrace_reject_skipped': 0, 'min_stop_skipped': 0}` |
| `BTCUSDT` | `c03_f10_retrace` | 9 | -0.1101% | 0.0698% | -0.0022% | 55.56% | -0.5568% | `{'candidate_signals': 53, 'entries': 9, 'entry_extension_skipped': 0, 'busy_skipped': 3, 'confirm_timeout_skipped': 2, 'early_reversal_skipped': 10, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 0, 'closepos_reject_skipped': 0, 'post_retrace_reject_skipped': 29, 'min_stop_skipped': 0}` |
| `BTCUSDT` | `c03_f10_all` | 6 | -0.1353% | -0.0154% | -0.0633% | 50.00% | -0.4059% | `{'candidate_signals': 53, 'entries': 6, 'entry_extension_skipped': 0, 'busy_skipped': 2, 'confirm_timeout_skipped': 2, 'early_reversal_skipped': 10, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 18, 'closepos_reject_skipped': 2, 'post_retrace_reject_skipped': 13, 'min_stop_skipped': 0}` |
| `ETHUSDT` | `c03_f10` | 32 | 1.5249% | 2.1769% | 1.9153% | 50.00% | -0.9435% | `{'candidate_signals': 44, 'entries': 32, 'entry_extension_skipped': 0, 'busy_skipped': 3, 'confirm_timeout_skipped': 1, 'early_reversal_skipped': 8, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 0, 'closepos_reject_skipped': 0, 'post_retrace_reject_skipped': 0, 'min_stop_skipped': 0}` |
| `ETHUSDT` | `c03_f10_vwap` | 32 | 1.5249% | 2.1769% | 1.9153% | 50.00% | -0.9435% | `{'candidate_signals': 44, 'entries': 32, 'entry_extension_skipped': 0, 'busy_skipped': 3, 'confirm_timeout_skipped': 1, 'early_reversal_skipped': 8, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 0, 'closepos_reject_skipped': 0, 'post_retrace_reject_skipped': 0, 'min_stop_skipped': 0}` |
| `ETHUSDT` | `c03_f10_vol` | 19 | 0.6487% | 1.0321% | 0.8784% | 36.84% | -0.9593% | `{'candidate_signals': 44, 'entries': 19, 'entry_extension_skipped': 0, 'busy_skipped': 1, 'confirm_timeout_skipped': 1, 'early_reversal_skipped': 8, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 15, 'closepos_reject_skipped': 0, 'post_retrace_reject_skipped': 0, 'min_stop_skipped': 0}` |
| `ETHUSDT` | `c03_f10_full` | 19 | 0.6487% | 1.0321% | 0.8784% | 36.84% | -0.9593% | `{'candidate_signals': 44, 'entries': 19, 'entry_extension_skipped': 0, 'busy_skipped': 1, 'confirm_timeout_skipped': 1, 'early_reversal_skipped': 8, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 15, 'closepos_reject_skipped': 0, 'post_retrace_reject_skipped': 0, 'min_stop_skipped': 0}` |
| `ETHUSDT` | `c05_f15` | 30 | 0.7254% | 1.3321% | 1.0886% | 36.67% | -0.9098% | `{'candidate_signals': 44, 'entries': 30, 'entry_extension_skipped': 0, 'busy_skipped': 1, 'confirm_timeout_skipped': 5, 'early_reversal_skipped': 8, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 0, 'closepos_reject_skipped': 0, 'post_retrace_reject_skipped': 0, 'min_stop_skipped': 0}` |
| `ETHUSDT` | `c05_f15_full` | 18 | 0.4015% | 0.7642% | 0.6186% | 33.33% | -0.7177% | `{'candidate_signals': 44, 'entries': 18, 'entry_extension_skipped': 0, 'busy_skipped': 0, 'confirm_timeout_skipped': 5, 'early_reversal_skipped': 9, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 12, 'closepos_reject_skipped': 0, 'post_retrace_reject_skipped': 0, 'min_stop_skipped': 0}` |
| `ETHUSDT` | `c03_f10_closepos` | 30 | 1.4929% | 2.1040% | 1.8587% | 50.00% | -0.7356% | `{'candidate_signals': 44, 'entries': 30, 'entry_extension_skipped': 0, 'busy_skipped': 2, 'confirm_timeout_skipped': 1, 'early_reversal_skipped': 8, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 0, 'closepos_reject_skipped': 3, 'post_retrace_reject_skipped': 0, 'min_stop_skipped': 0}` |
| `ETHUSDT` | `c03_f10_retrace` | 7 | 0.5572% | 0.6983% | 0.6416% | 42.86% | -0.1628% | `{'candidate_signals': 44, 'entries': 7, 'entry_extension_skipped': 0, 'busy_skipped': 0, 'confirm_timeout_skipped': 1, 'early_reversal_skipped': 9, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 0, 'closepos_reject_skipped': 0, 'post_retrace_reject_skipped': 27, 'min_stop_skipped': 0}` |
| `ETHUSDT` | `c03_f10_all` | 2 | -0.0872% | -0.0472% | -0.0632% | 0.00% | -0.0872% | `{'candidate_signals': 44, 'entries': 2, 'entry_extension_skipped': 0, 'busy_skipped': 0, 'confirm_timeout_skipped': 1, 'early_reversal_skipped': 9, 'vwap_reject_skipped': 0, 'vol_accel_reject_skipped': 15, 'closepos_reject_skipped': 1, 'post_retrace_reject_skipped': 16, 'min_stop_skipped': 0}` |

## Files

- Summary JSON: `research/impulse_bar_delayed_confirm_v2_summary.json`
- `BTCUSDT c03_f10` ledger: `research/tmp_impulse_delayed_confirm_v2_BTCUSDT_c03_f10_ledger.csv`
- `BTCUSDT c03_f10_vwap` ledger: `research/tmp_impulse_delayed_confirm_v2_BTCUSDT_c03_f10_vwap_ledger.csv`
- `BTCUSDT c03_f10_vol` ledger: `research/tmp_impulse_delayed_confirm_v2_BTCUSDT_c03_f10_vol_ledger.csv`
- `BTCUSDT c03_f10_full` ledger: `research/tmp_impulse_delayed_confirm_v2_BTCUSDT_c03_f10_full_ledger.csv`
- `BTCUSDT c05_f15` ledger: `research/tmp_impulse_delayed_confirm_v2_BTCUSDT_c05_f15_ledger.csv`
- `BTCUSDT c05_f15_full` ledger: `research/tmp_impulse_delayed_confirm_v2_BTCUSDT_c05_f15_full_ledger.csv`
- `BTCUSDT c03_f10_closepos` ledger: `research/tmp_impulse_delayed_confirm_v2_BTCUSDT_c03_f10_closepos_ledger.csv`
- `BTCUSDT c03_f10_retrace` ledger: `research/tmp_impulse_delayed_confirm_v2_BTCUSDT_c03_f10_retrace_ledger.csv`
- `BTCUSDT c03_f10_all` ledger: `research/tmp_impulse_delayed_confirm_v2_BTCUSDT_c03_f10_all_ledger.csv`
- `ETHUSDT c03_f10` ledger: `research/tmp_impulse_delayed_confirm_v2_ETHUSDT_c03_f10_ledger.csv`
- `ETHUSDT c03_f10_vwap` ledger: `research/tmp_impulse_delayed_confirm_v2_ETHUSDT_c03_f10_vwap_ledger.csv`
- `ETHUSDT c03_f10_vol` ledger: `research/tmp_impulse_delayed_confirm_v2_ETHUSDT_c03_f10_vol_ledger.csv`
- `ETHUSDT c03_f10_full` ledger: `research/tmp_impulse_delayed_confirm_v2_ETHUSDT_c03_f10_full_ledger.csv`
- `ETHUSDT c05_f15` ledger: `research/tmp_impulse_delayed_confirm_v2_ETHUSDT_c05_f15_ledger.csv`
- `ETHUSDT c05_f15_full` ledger: `research/tmp_impulse_delayed_confirm_v2_ETHUSDT_c05_f15_full_ledger.csv`
- `ETHUSDT c03_f10_closepos` ledger: `research/tmp_impulse_delayed_confirm_v2_ETHUSDT_c03_f10_closepos_ledger.csv`
- `ETHUSDT c03_f10_retrace` ledger: `research/tmp_impulse_delayed_confirm_v2_ETHUSDT_c03_f10_retrace_ledger.csv`
- `ETHUSDT c03_f10_all` ledger: `research/tmp_impulse_delayed_confirm_v2_ETHUSDT_c03_f10_all_ledger.csv`
