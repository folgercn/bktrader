# T3 Overlay Order-Book Impact Decision

Research-only sizing stress for the ETH pretouch lead enhancement.

Historical research artifacts do not contain full order-book depth snapshots, so
this is an impact proxy, not a real fill replay. It ranks sizing candidates
before spending engineering time on a real depth-backed simulator.

## Profiles

| Profile | Meaning |
|---|---|
| `moderate_top1p6_active0p5` | No extra concentration cost up to 1.6x notional; small active-notional pressure. |
| `strict_top1p2_active1p0` | Starts charging impact above 1.2x notional; active-notional pressure is material. |
| `severe_top1p0_active2p0` | Thin-book stress; starts charging above 1.0x and doubles active pressure. |

## Current 2.0x Overlay

Sensible capacity rows: current lead scale uses capacity `1.6`; 1.25x lead scale
uses capacity `2.0`.

| Profile | Lead scale | Overlay slip | Calendar | DD | Impact cost | Max impact bps | Read |
|---|---:|---:|---:|---:|---:|---:|---|
| `moderate_top1p6_active0p5` | 1.00x | 15bp | 24.548369% | -2.105913% | 0.000000% | 0.000000 | Existing conservative row survives. |
| `strict_top1p2_active1p0` | 1.00x | 15bp | 22.728992% | -2.105913% | 1.819377% | 4.800000 | Slightly below lead adverse10; no sizing lift means strict impact eats the overlay edge. |
| `moderate_top1p6_active0p5` | 1.25x | 15bp | 28.817057% | -2.121955% | 1.474224% | 3.200000 | Strong candidate if depth is near current peak-notional comfort. |
| `strict_top1p2_active1p0` | 1.25x | 15bp | 25.554185% | -2.121955% | 4.737096% | 9.600000 | Still above lead adverse10, but the cushion is now modest. |
| `strict_top1p2_active1p0` | 1.25x | 20bp | 22.278881% | -2.420425% | 4.737096% | 9.600000 | Fails the kill-stress boundary. |
| `severe_top1p0_active2p0` | 1.25x | 15bp | 20.291321% | -2.121955% | 9.999960% | 20.000000 | Fails. Thin-book conditions should not allow sizing lift. |

## 2.5x Overlay Check

The aggressive `[0.50,0.25]` overlay scale does not solve the impact problem.

| Profile | Lead scale | Overlay slip | Calendar | DD | Read |
|---|---:|---:|---:|---:|---|
| `strict_top1p2_active1p0` | 1.25x | 15bp | 25.961779% | -2.629283% | Only +0.407594pp over 2.0x overlay, with meaningfully worse DD. |
| `strict_top1p2_active1p0` | 1.25x | 20bp | 21.859574% | -3.001894% | Fails kill-stress and is worse than 2.0x overlay. |
| `severe_top1p0_active2p0` | 1.25x | 15bp | 20.698915% | -2.629283% | Fails. |

## Decision

- Do not promote 2.5x overlay as the default row. Its extra return is too thin
  after impact, and the 20bp kill-stress is worse than the 2.0x overlay.
- The only sizing-lift row worth the next stage is: current 2.0x overlay,
  lead scale `1.25x`, capacity `2.0`, and a live-like depth gate that blocks the
  lift under thin-book conditions.
- Promotion criterion for the next stage: pass strict impact at 15bp while
  keeping 20bp as fail/kill-stress; fail severe profile by design unless real
  depth proves the book is not severe.

## Conditional Lead Scale

Artifact:
`research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_conditional_lead_scale_size2p0/`.

The conditional policy keeps the current lead sizing by default and applies
`1.25x` only when the proxy impact estimate is below a gate.

| Profile | Gate | Overlay slip | Calendar | Lead scaled | Lead blocked | Impact cost | Max impact bps | Read |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| `moderate_top1p6_active0p5` | 6bp | 15bp | 28.817057% | 62 | 0 | 1.474224% | 3.200000 | Passes; same as unconditional under moderate book. |
| `strict_top1p2_active1p0` | 8bp | 15bp | 23.687991% | 38 | 24 | 2.100515% | 7.920000 | Barely above lead adverse10; too threshold-sensitive. |
| `strict_top1p2_active1p0` | 10bp | 15bp | 25.554185% | 62 | 0 | 4.737096% | 9.600000 | Passes strict 15bp, but relies on accepting near-10bp RT impact. |
| `strict_top1p2_active1p0` | 10bp | 20bp | 22.278881% | 62 | 0 | 4.737096% | 9.600000 | Fails kill-stress. |
| `severe_top1p0_active2p0` | 10bp | 15bp | 20.482920% | 35 | 27 | 4.716134% | 12.000000 | Fails; severe book must block sizing lift and likely block the setup. |

Read: conditional scaling is the right shape, but the exact gate cannot be
chosen from this proxy alone. A real depth replay must calibrate whether
8-10bp round-trip impact corresponds to acceptable live execution quality.

## Live Depth Telemetry Calibration

Artifact:
`research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_live_depth_calibration_20260519/`.

The first production-aligned live telemetry pass uses `bktrader-ctl order list`
and stores only sanitized derived metrics. It found 6 ETH pretouch entry samples.

| Max spread | Max book age | Max source divergence | Min top-depth coverage | Max adverse fill drift |
|---:|---:|---:|---:|---:|
| 7.320544bp | 413.137042ms | 0.320167bp | 9335.605263 | 5.204792bp |

Under the current live pre-submit guard, `1.25x` quantity scale passes 6/6
samples with min scaled top-depth coverage `7468.484210`. Larger scales also
pass this small testnet sample, but that should be read as "not currently
severe-book" evidence, not as permission to increase default sizing. The worst
actual fill already leaves only `2.795208bp` of the 8bp slippage guard.

## Readiness Gate

Artifact:
`research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_sizing_readiness_gate_20260519/`.

Status: `research_continue_collect_live_depth`.

The gate passes live guard and proxy separation for `1.25x`, but blocks live
promotion because the live entry sample count is `6`, below the `30` sample
human-review threshold. This keeps the sizing decision bounded: continue
collecting depth telemetry, do not alter template sizing yet.

The telemetry accumulation refresh is now implemented. It merges the latest
`bktrader-ctl order list` entry samples with prior sanitized
`t3_overlay_live_depth_entry_samples.csv`, dedupes by `order_id`, and then reruns
the readiness gate. The 2026-05-19 dry run produced `current_entry_count=6`,
`history_entry_count=6`, `deduped_entry_count=6`, confirming repeat refreshes do
not double-count the same fills.

## Artifacts

- 2.0x overlay proxy matrix:
  `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_orderbook_impact_sensitivity_size2p0/`
- 2.5x overlay proxy matrix:
  `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_orderbook_impact_sensitivity_size2p5/`
- Conditional lead-scale matrix:
  `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_conditional_lead_scale_size2p0/`
- Live depth telemetry calibration:
  `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_live_depth_calibration_20260519/`
- Sizing readiness gate:
  `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_sizing_readiness_gate_20260519/`
- Accumulated live-depth calibration dry run:
  `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_live_depth_calibration_accumulated_20260519/`
- Accumulated readiness gate dry run:
  `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_sizing_readiness_gate_accumulated_20260519/`
