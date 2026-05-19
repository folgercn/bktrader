# T3 Direct Entry Probability Progress

Research-only progress note for the T3 probability/quality overlay work on
`codex/research-eth-pretouch-breakout-20260518`.

## Contract

- Window: 2025-06 through 2026-04 unless noted.
- Base lifecycle: strict T2-disabled T3 60m exit, `strict_next_second_cross`.
- Baseline sizing: `dir2_zero_initial=true`, `zero_initial_mode=reentry_window`,
  `reentry_size_schedule=[0.20, 0.10]`, `max_trades_per_bar=2`.
- Direct entry diagnostics use `external_entry_mode=next_second_adverse`, which
  only enters after `bar_time > touch_time` and uses the next-second adverse
  price.
- No monthly gate was used.

## Key Results

| Candidate | Symbols | Entry | Size schedule | Calendar sum | Worst silo | Neg silos | Trades | By symbol |
|---|---|---|---|---:|---:|---:|---:|---|
| `short_speed_abs_ge_0p35` | ETH,BTC | reentry_window | `[0.20,0.10]` | 0.54% | -0.12% | 4 | 8 | ETH 0.51%, BTC 0.03% |
| `short_speed_abs_ge_0p35` | ETH,BTC | next_second_open | `[0.20,0.10]` | 2.33% | -0.25% | 13 | 77 | ETH 2.79%, BTC -0.46% |
| `short_speed_abs_ge_0p35` | ETH,BTC | next_second_adverse | `[0.20,0.10]` | 2.12% | -0.26% | 13 | 77 | ETH 2.61%, BTC -0.49% |
| `bayes_ge_m0p010 + short` | ETH,BTC | reentry_window | `[0.20,0.10]` | 0.66% | -0.06% | 4 | 20 | ETH 0.52%, BTC 0.14% |
| `bayes_ge_m0p010 + short` | ETH,BTC | next_second_adverse | `[0.20,0.10]` | 1.10% | -0.61% | 9 | 157 | ETH 2.54%, BTC -1.44% |
| `long_speed_abs_ge_0p35` | ETH,BTC | next_second_adverse | `[0.20,0.10]` | -0.16% | -1.47% | 13 | 219 | ETH 2.98%, BTC -3.14% |
| `all_speed_abs_ge_0p35` | ETH,BTC | next_second_adverse | `[0.20,0.10]` | 2.04% | -1.72% | 13 | 294 | ETH 5.66%, BTC -3.62% |
| `all_speed_abs_ge_0p35` | ETH only | next_second_adverse | `[0.40,0.20]` | 11.41% | -0.83% | 3 | 163 | ETH 11.41% |

## Lead Bridge

The current production-aligned ETH pretouch research lead is preserved as the
primary leg. The T3 direct-entry result is treated only as an ETH overlay leg,
converted from lifecycle percent returns into the lead ledger's fractional
`weighted_pnl` convention.

| Overlay size scale | T3 overlay | Lead adverse10 + overlay | Combined worst month | Combined neg months | Combined trades |
|---:|---:|---:|---:|---:|---:|
| 1.5x | 8.53% | 31.501648% | -0.35% | 1 | 225 |
| 2.0x | 11.41% | 34.381648% | -0.47% | 1 | 225 |
| 2.5x | 14.30% | 37.271648% | -0.59% | 1 | 225 |

Lead-only reference:

| Lead leg | Calendar sum | Worst month | Neg months | Trades |
|---|---:|---:|---:|---:|
| same-close | 30.217222% | 2.358319% | 0 | 62 |
| adverse10 | 22.971648% | 1.395821% | 0 | 62 |

Bridge artifact:
`research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_lead_bridge_sizing_matrix.csv`.

## Read

- The frozen pretouch RF probability is not a reliable monotonic T3 filter.
  Higher RF buckets were often worse; the useful explanatory features are side,
  absolute 300s speed, and symbol.
- The original strict reentry window is a bottleneck for these T3 events. Moving
  to next-second direct entry lifts `short_speed_abs_ge_0p35` from 0.54% to
  2.12% even under adverse pricing.
- BTC is the main blocker. Both long-speed and all-speed variants have positive
  ETH attribution but strongly negative BTC attribution.
- The first result in the requested 10-20% range is ETH-only all-speed adverse
  with 2x T3 sizing: 11.41% calendar sum, worst month -0.83%, 3 negative months.
  This is not promotion-ready; it needs drawdown/exposure/final-mark audit and
  slippage stress before being treated as a live candidate.
- As a lead enhancement, the 2x T3 overlay lifts lead adverse10 from 22.97% to
  34.38% in additive fixed-calendar accounting. This is the strongest current
  interpretation: the T3 work is useful as an overlay to the lead, not as a
  standalone replacement.

## Exposure Audit

2x ETH overlay audit artifact:
`research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_lead_exposure_audit/`.

| Check | Result |
|---|---:|
| T3 overlay calendar sum | 11.410000% |
| T3 paired lifecycle fee-net PnL | 11.402632% |
| T3 paired lifecycle gross PnL | 24.503851% |
| T3 round-trip fee drag | 13.101219% |
| T3-only max drawdown | -1.286147% |
| T3 max loss streak | 8 |
| FinalMarkToMarket contribution | 0.000000% / 0 trades |
| Exact lead weighted-PnL parity diff | 0.000000% |
| Exact lead avg / p50 / p90 hold | 1487.58s / 924.50s / 3748.80s |
| Exact lead max hold | 7199.00s |
| Exact lead/overlay overlap pairs | 0 |
| Combined fee-net realized PnL | 34.374280% |
| Combined sequential max drawdown | -1.210504% |

Read: the overlay still survives a fee-net exposure audit and aligns with the
11.41% calendar bridge after fee adjustment. The 24.50% gross paired-trade PnL
is not an investable return; commission accounts for 13.10pp of drag. The lead
window weakness has been retired by rebuilding the selected `DelayResult`
entry/exit ledger; parity versus the compact adverse10 lead ledger is exact
across 62/62 trades.

## Portfolio / Slippage Sensitivity

Exact-lead portfolio sensitivity artifact:
`research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_lead_portfolio_sensitivity_exact_lead/`.

Exact lead window artifact:
`research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_lead_exact_exposure/`.

The allocator now reads exact selected lead `DelayResult` entry/exit windows and
the actual overlay lifecycle windows. Capacity `1.6` matches the lead's max
single-trade desired notional share and is already unconstrained with exact
windows: observed peak active notional is `1.600` instead of the previous
approximate-window `2.128`.

| Policy | Capacity | Extra overlay RT slip | Calendar sum | Worst month | Neg months | Max DD | Lead PnL | Overlay PnL | Allocation |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `scale_to_available` | 1.0 | 10bp | 20.579629% | -1.207073% | 2 | -1.807443% | 15.727604% | 4.852025% | 0.888179 |
| `scale_to_available` | 1.6 | 0bp | 34.374280% | -0.469803% | 1 | -1.210504% | 22.971648% | 11.402632% | 1.000000 |
| `scale_to_available` | 1.6 | 10bp | 27.823672% | -1.207073% | 2 | -1.807443% | 22.971648% | 4.852025% | 1.000000 |
| `scale_to_available` | 1.6 | 15bp | 24.548369% | -1.575708% | 2 | -2.105913% | 22.971648% | 1.576721% | 1.000000 |
| `scale_to_available` | 1.6 | 20bp | 21.273065% | -1.944343% | 3 | -2.404383% | 22.971648% | -1.698583% | 1.000000 |
| `skip_if_insufficient` | 1.6 | 10bp | 27.823672% | -1.207073% | 2 | -1.807443% | 22.971648% | 4.852025% | 1.000000 |
| `scale_to_available` | 2.5 | 10bp | 27.823672% | -1.207073% | 2 | -1.807443% | 22.971648% | 4.852025% | 1.000000 |
| `scale_to_available` | 2.5 | 20bp | 21.273065% | -1.944343% | 3 | -2.404383% | 22.971648% | -1.698583% | 1.000000 |

Read: exact lead windows remove the artificial capacity haircut at `1.6`, but
they do not change the promotion boundary. The enhancement survives `10-15bp`
additional overlay round-trip slippage; at `20bp`, the overlay leg turns
negative and the combined result no longer beats the production-aligned lead
adverse10 baseline.

## Position Scale Sensitivity

Position scale artifact:
`research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_position_scale_sensitivity_report.md`.

Replayed overlay scale:

| Overlay scale | Schedule | T3 fee-net PnL | T3 DD | Combined 0bp | Combined 10bp | Combined 15bp | Combined 20bp |
|---:|---|---:|---:|---:|---:|---:|---:|
| 2.0x | `[0.40,0.20]` | 11.402632% | -1.286147% | 34.374280% | 27.823672% | 24.548369% | 21.273065% |
| 2.5x | `[0.50,0.25]` | 14.290931% | -1.604931% | 37.262579% | 29.058168% | 24.955963% | 20.853758% |

Read: 2.5x overlay can buy more headline return, but the marginal quality is
thin. It adds 2.89pp before extra slippage and 1.23pp at 10bp, only 0.41pp at
15bp, and turns worse than 2.0x at 20bp. Keep 2.5x as an aggressive research
row, not the default live-candidate row.

Lead scale what-if:

| Overlay | Lead scale | Capacity | Peak active | 10bp combined | 15bp combined | 20bp combined | Max DD at 10bp |
|---|---:|---:|---:|---:|---:|---:|---:|
| 2.0x | 1.00x | 1.6 | 1.6 | 27.823672% | 24.548369% | 21.273065% | -1.807443% |
| 2.0x | 1.25x | 2.0 | 2.0 | 33.566584% | 30.291281% | 27.015977% | -1.823486% |
| 2.0x | 1.50x | 2.5 | 2.4 | 39.309496% | 36.034192% | 32.758889% | -1.839528% |

The lead-scale rows are linear diagnostics only; they do not replay larger
orders against order-book depth. They suggest the cleaner sizing lever is a
modest lead-scale increase, especially 1.25x with capacity `2.0`, but this
requires stricter order-book impact validation before any live-template sizing
change.

## Order-Book Impact Proxy

Order-book impact decision artifact:
`research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_orderbook_impact_decision_report.md`.

Historical research artifacts do not contain full depth snapshots, so this is a
transparent proxy rather than a real fill replay. It adds round-trip impact bps
from single-trade notional concentration and active-notional pressure.

| Overlay | Profile | Lead scale | Capacity | Overlay slip | Calendar | DD | Impact cost | Max impact bps | Read |
|---|---|---:|---:|---:|---:|---:|---:|---:|---|
| 2.0x | `moderate_top1p6_active0p5` | 1.25x | 2.0 | 15bp | 28.817057% | -2.121955% | 1.474224% | 3.200000 | pass |
| 2.0x | `strict_top1p2_active1p0` | 1.25x | 2.0 | 15bp | 25.554185% | -2.121955% | 4.737096% | 9.600000 | pass, thinner cushion |
| 2.0x | `strict_top1p2_active1p0` | 1.25x | 2.0 | 20bp | 22.278881% | -2.420425% | 4.737096% | 9.600000 | fail/kill-stress |
| 2.0x | `severe_top1p0_active2p0` | 1.25x | 2.0 | 15bp | 20.291321% | -2.121955% | 9.999960% | 20.000000 | fail |
| 2.5x | `strict_top1p2_active1p0` | 1.25x | 2.0 | 15bp | 25.961779% | -2.629283% | 4.737096% | 9.600000 | only +0.41pp vs 2.0x, worse DD |

Read: unconditional sizing expansion is not justified. The next candidate is a
conditional sizing lift: keep the current 2.0x overlay, allow `1.25x` lead scale
only when a live-like depth gate supports capacity `2.0`, and keep 20bp as the
kill-stress. The 2.5x overlay remains an aggressive research row, not a default
promotion candidate.

Conditional lead-scale artifact:
`research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_conditional_lead_scale_size2p0/`.

| Profile | Gate | Overlay slip | Calendar | Scaled lead trades | Blocked lead trades | Read |
|---|---:|---:|---:|---:|---:|---|
| `moderate_top1p6_active0p5` | 6bp | 15bp | 28.817057% | 62 | 0 | pass |
| `strict_top1p2_active1p0` | 8bp | 15bp | 23.687991% | 38 | 24 | thin cushion |
| `strict_top1p2_active1p0` | 10bp | 15bp | 25.554185% | 62 | 0 | pass if near-10bp RT impact is acceptable |
| `strict_top1p2_active1p0` | 10bp | 20bp | 22.278881% | 62 | 0 | fail/kill-stress |
| `severe_top1p0_active2p0` | 10bp | 15bp | 20.482920% | 35 | 27 | fail |

Read: conditional scaling is the right strategy shape, but its threshold is
too sensitive to choose from proxy data alone. Real historical depth replay is
now the blocker for any live sizing change.

## Live Depth Telemetry Calibration

Live depth calibration artifact:
`research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_live_depth_calibration_20260519/`.

The calibration consumes `bktrader-ctl order list --limit 200 --json` and writes
only sanitized derived metrics. It filters to `ETHUSDT`,
`strategy-version-bk-eth-pretouch-timing-v010`, `signalKind=entry`, and
`reduceOnly=false`.

| Entry samples | Max spread | Max book age | Max source divergence | Min top-depth coverage | Max adverse fill drift | P90 adverse fill drift |
|---:|---:|---:|---:|---:|---:|---:|
| 6 | 7.320544bp | 413.137042ms | 0.320167bp | 9335.605263 | 5.204792bp | 2.791609bp |

Quantity-scale matrix against the current live pre-submit guard:

| Scale | Combined pass | Min scaled top-depth coverage | Worst 8bp slippage headroom |
|---:|---:|---:|---:|
| 1.00x | 6/6 | 9335.605263 | 2.795208bp |
| 1.25x | 6/6 | 7468.484210 | 2.795208bp |
| 1.50x | 6/6 | 6223.736842 | 2.795208bp |
| 2.00x | 6/6 | 4667.802631 | 2.795208bp |
| 2.50x | 6/6 | 3734.242105 | 2.795208bp |

Read: current live testnet telemetry is not severe-book evidence; it supports
continuing the `1.25x` conditional lead-scale candidate. It does not justify
unconditional scale-up. The sample is only 6 entries, top-book coverage on
testnet is unusually large, and the worst actual entry drift already consumed
`5.204792bp` of the 8bp guard.

## Sizing Readiness Gate

Readiness artifact:
`research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_sizing_readiness_gate_20260519/`.

The readiness gate combines the live depth calibration with the conditional
lead-scale proxy. Current status is:

`research_continue_collect_live_depth`

| Check | Result |
|---|---:|
| Target lead scale | 1.25x |
| Live samples | 6 |
| Live combined pass | 6/6 |
| Sample gate | fail, threshold 30 |
| Strict 15bp proxy | 25.554185% |
| Strict 20bp proxy | 22.278881%, kill-stress fail |
| Severe 15bp proxy | 20.482920%, thin-book fail |

Read: the candidate shape is still alive, but it is not live-ready. The next
promotion threshold is no longer ambiguous: collect at least 30 ETH pretouch
entry samples while preserving 100% combined live guard pass, worst slippage
headroom `>=2bp`, strict 15bp above `lead_adverse10_exact`, and strict 20bp /
severe 15bp below `lead_adverse10_exact`.

## Telemetry Accumulation Flow

Accumulated calibration artifact:
`research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_live_depth_calibration_accumulated_20260519/`.

Accumulated readiness artifact:
`research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_sizing_readiness_gate_accumulated_20260519/`.

The live-depth calibration runner now accepts a prior sanitized history CSV via
`--history-csv`, merges current `bktrader-ctl order list` samples, dedupes by
`order_id`, and writes a refreshed sanitized sample file plus matrix. It does
not persist raw ctl JSON.

2026-05-19 dry run:

| Current extracted | Prior history | Deduped samples | Status |
|---:|---:|---:|---|
| 6 | 6 | 6 | `research_continue_collect_live_depth` |

Read: the accumulation path is ready. The next useful production-data step is
not another manual one-off query; it is rerunning this refresh after new ETH
pretouch entries arrive and tracking whether deduped samples approach the 30
sample gate without losing slippage headroom.

## Next Checks

1. Keep BTC excluded until a separate BTC-specific quality layer can clear the
   negative attribution without monthly gates.
2. Calibrate the conditional sizing gate with real depth data or production
   decision telemetry; proxy-only thresholds are not enough for live sizing.
3. Keep this as research-only until real depth data, order-book impact replay,
   and live/replay event-time parity checks are done.
