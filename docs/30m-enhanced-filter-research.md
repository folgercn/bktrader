# BTC/ETH 30m Enhanced Filter Notes

This document records the live BTCUSDT 30m enhanced template filters and the
Q1 2026 research runs used to choose the current low-volatility reentry gate.

## Live BTCUSDT 30m Enhanced Template

Template key: `binance-testnet-btc-30m-enhanced`

Strategy/version:

- `strategy-bk-btc-30m-enhanced`
- `strategyEngine=bk-live-intrabar-sma5-t3-sep`
- `signalTimeframe=30m`
- `executionDataSource=tick`

Sizing/risk baseline:

- `dir2_zero_initial=true`
- `zero_initial_mode=reentry_window`
- `reentry_size_schedule=[0.20, 0.10]`
- `max_trades_per_bar=2`
- `stop_mode=atr`
- `stop_loss_atr=0.3`
- `profit_protect_atr=1.0`
- `trailing_stop_atr=0.3`
- `delayed_trailing_activation_atr=0.5`
- `long_reentry_atr=0.1`
- `short_reentry_atr=0.0`

Signal filters:

- `use_sma5_intraday_structure=true`: intraday entries use the current 30m bar close vs SMA5 as the hard structure filter. Long entries need close above SMA5; short entries need close below SMA5.
- `breakout_shape=baseline_plus_t3`: the original T2 breakout remains enabled, and the T3 swing breakout is added.
- `t3_min_sma_atr_separation=0.25`: T3 swing breakout must be at least `0.25 * ATR14` away from SMA5. This filter applies to `t3_swing` only; it does not block `original_t2`.
- `reentry_min_stop_bps=6.0`: reentry entries require `stop_loss_atr * ATR14 / reentry_price * 10000 >= 6`.
- `reentry_atr_percentile_gte=25.0`: reentry entries require the current ATR14 percentile to be at least 25 over the rolling ATR sample.

Reentry-specific behavior:

- The low-volatility gate applies only to reentry entries: `Zero-Initial-Reentry`, `SL-Reentry`, and `PT-Reentry`.
- It does not filter stop-loss exits. SL remains a hard risk boundary.
- Live ATR percentile is derived from signal-bar history with the research-compatible rule: ATR14 rolling mean, then percentile rank of the latest ATR over a 240-bar window with at least 50 valid ATR values. Runtime signal-bar history keeps 260 bars so the 240-bar percentile can be computed after ATR warmup.

Execution and liquidity filters:

- Entry planning uses `book-aware-v1`.
- Entry max spread/slippage/source divergence are capped at 8 bps.
- Entry order book must be fresh within 500 ms and have top-book coverage at least 0.5.
- Wide entry spread mode is `limit-maker`, with 15s resting timeout and MARKET fallback.
- PT exit is post-only LIMIT/GTX with MARKET fallback.
- SL exit remains MARKET, with max spread/slippage caps of 8 bps.
- Live signal decision also gates entries on actionable price, order-book spread, and liquidity bias. For SL reentry, neutral book bias is not considered actionable.
- `sl_reentry_min_delay_seconds=60` blocks immediate SL reentry churn for the enhanced template.

## BTCUSDT Q1 2026 30m Results

Replay setup:

- Full Binance trade archive, continuous 1s execution bars
- Signal timeframe `30m`
- Replay mode `live_intrabar_sma5`
- `breakout_shape=baseline_plus_t3`
- `t3_min_sma_atr_separation=0.25`
- BTC live-like risk profile: `stop_loss_atr=0.3`, `trailing_stop_atr=0.3`, `delayed_trailing_activation=0.5`

Low-volatility reentry gate sweep:

| Variant | Return | Max DD | Trades | Win Rate | Sharpe | Delta vs Baseline |
|---|---:|---:|---:|---:|---:|---:|
| `baseline` | 127.45% | -3.03% | 3177 | 76.46% | 13.23 | 0.00 pp |
| `min_stop_bps_6` | 140.32% | -1.27% | 2967 | 80.49% | 13.91 | +12.87 pp |
| `min_stop_bps_8` | 151.38% | -0.66% | 2748 | 83.08% | 14.56 | +23.93 pp |
| `atr_pct_gte_25` | 137.25% | -1.27% | 2334 | 82.73% | 14.47 | +9.80 pp |
| `atr_pct_gte_35` | 127.15% | -1.13% | 2017 | 84.04% | 14.74 | -0.30 pp |
| `min_stop_bps_6 + atr_pct_gte_25` | 138.08% | -1.27% | 2318 | 83.09% | 14.54 | +10.63 pp |
| `min_stop_bps_6 + atr_pct_gte_35` | 127.15% | -1.13% | 2017 | 84.04% | 14.74 | -0.30 pp |

Read:

- `min_stop_bps_8` had the best raw Q1 return and drawdown in this sweep.
- `min_stop_bps_6 + atr_pct_gte_25` is more conservative and combines an absolute stop-distance check with a relative volatility-regime check. It is the parameter set currently wired into the live BTC 30m enhanced template.
- `atr_pct_gte_35` overfilters BTC Q1. Combining it with `min_stop_bps_6` produced the same headline metrics as `atr_pct_gte_35` alone.

Breakout confirmation sweep:

| Variant | Return | Max DD | Trades | Win Rate | Sharpe |
|---|---:|---:|---:|---:|---:|
| `baseline_single_observation` | 127.45% | -3.03% | 3177 | 76.46% | 13.23 |
| `margin_0p02atr` | 127.79% | -2.89% | 3111 | 76.73% | 13.36 |
| `confirm_2x_1s` | 126.80% | -2.85% | 3126 | 76.52% | 13.22 |
| `confirm_3x_1s` | 127.61% | -2.86% | 3079 | 76.78% | 13.30 |

Read:

- Multi-tick confirmation is not the primary recommendation for BTC 30m. It reduces some trades, but the return improvement is weak or negative.
- The low-volatility reentry gate better targets the observed issue: entries with very thin ATR-derived stop distance.

## ETHUSDT 30m Recommendation

The ETH low-volatility gate was evaluated on the ETH research baseline:

- `stop_loss_atr=0.05`
- `trailing_stop_atr=0.3`
- `delayed_trailing_activation=0.5`
- `breakout_shape=baseline_plus_t3`
- `t3_min_sma_atr_separation=0.25`

ETH stop-distance distribution under `stop_loss_atr=0.05`:

| Count | Min | P05 | P10 | P25 | P50 | P75 | P90 | P95 | Max |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 4307 | 0.6144 | 1.2044 | 1.5776 | 2.3642 | 3.4132 | 4.6035 | 5.9251 | 7.2042 | 15.9955 |

ETH low-volatility reentry gate sweep:

| Variant | Return | Max DD | Trades | Win Rate | Sharpe | Delta vs Baseline |
|---|---:|---:|---:|---:|---:|---:|
| `baseline` | 436.11% | -2.04% | 3226 | 80.60% | 13.49 | 0.00 pp |
| `min_stop_bps_2` | 477.67% | -0.38% | 2622 | 86.84% | 15.18 | +41.56 pp |
| `min_stop_bps_4` | 288.53% | -0.21% | 1185 | 91.39% | 17.71 | -147.58 pp |
| `min_stop_bps_6` | 92.49% | -0.17% | 307 | 92.83% | 18.79 | -343.62 pp |
| `min_stop_bps_8` | 41.72% | -0.17% | 133 | 90.23% | 18.49 | -394.39 pp |
| `atr_pct_gte_25` | 386.72% | -0.34% | 2297 | 85.46% | 14.66 | -49.39 pp |
| `min_stop_bps_4 + atr_pct_gte_25` | 264.45% | -0.21% | 1101 | 91.37% | 17.55 | -171.66 pp |

Recommended ETH 30m filter parameters for the current ETH research profile:

- `reentry_min_stop_bps=2.0`
- `reentry_atr_percentile_gte=0` or unset

Read:

- ETH should not inherit BTC's 6 or 8 bps threshold under `stop_loss_atr=0.05`; both overfilter.
- `min_stop_bps_2` is the strongest ETH Q1 candidate in this profile.
- ATR percentile is useful as a defensive risk-off overlay but reduced return in Q1.

ETH stop-loss ATR sweep:

| stop_loss_atr | Return | Max DD | Trades | Win Rate | Sharpe |
|---:|---:|---:|---:|---:|---:|
| 0.05 | 436.11% | -2.04% | 3226 | 80.60% | 13.49 |
| 0.10 | 434.50% | -2.12% | 3259 | 80.95% | 13.40 |
| 0.20 | 437.39% | -2.01% | 3229 | 82.41% | 13.40 |
| 0.30 | 451.20% | -1.86% | 3190 | 84.23% | 13.61 |
| 0.40 | 441.23% | -1.63% | 3136 | 85.36% | 13.72 |

Read:

- `stop_loss_atr=0.3` is reasonable for ETH 30m in Q1 and had the best return in this sweep.
- If ETH 30m moves from `stop_loss_atr=0.05` to `0.3`, the bps gate must be recalibrated. The current `reentry_min_stop_bps=2.0` recommendation is tied to the `0.05` stop profile.

## Follow-up: Unified Entry Quality Report

The current BTC live template wires the low-volatility checks into the reentry entry path only. A follow-up PR should consolidate entry eligibility checks into one report schema instead of adding more isolated conditionals over time.

Suggested gate result shape:

```go
type EntryQualityGateResult struct {
	Name       string         `json:"name"`
	Applied    bool           `json:"applied"`
	Ready      bool           `json:"ready"`
	Reason     string         `json:"reason,omitempty"`
	Metrics    map[string]any `json:"metrics,omitempty"`
	Thresholds map[string]any `json:"thresholds,omitempty"`
}

type EntryQualityReport struct {
	Ready       bool                     `json:"ready"`
	Reason      string                   `json:"reason,omitempty"`
	GateResults []EntryQualityGateResult `json:"gates"`
}
```

Candidate gates to migrate into this report:

- signal bar readiness: SMA5, breakout shape, T3 separation.
- reentry trigger readiness: planned price versus trigger price.
- low-volatility reentry quality: stop-distance bps and ATR percentile.
- execution quality: spread, source divergence, book bias, book freshness, top-book coverage.
- delay controls: SL reentry cooldown.

The report should explicitly carry scope such as `entry-only`, `reentry-only`, or `exit-never` so protective SL exits cannot be blocked by entry-quality filters.

## Source Reports

- `research/20260429_btc_q1_30min_low_vol_entry_filters.md`
- `research/20260429_btc_q1_30min_low_vol_entry_filters_combo35.md`
- `research/20260429_btc_q1_30min_breakout_confirmation_filters.md`
- `research/20260429_eth_q1_30min_low_vol_entry_filters.md`
- `research/20260429_eth_q1_30min_stop_loss_atr_sweep.md`
- `research/20260427_eth_q1_30min_t3_sma5_sep_0p25_marginal.md`
