# Breakout Structure Risk Appetite Sweep 2026-05-19

Scope: research-only. This pass answers whether the harness was suppressing
return by keeping sizing too conservative.

## Harness Change

`t3_overlay_orderbook_impact_sensitivity.py` now sweeps both:

- `lead_scale`
- `overlay_scale`

The old default remains unchanged because `overlay_scale=1.0` unless explicitly
provided. The new aggressive matrix is:

- output:
  `research/entry_redesign/scripts/output/timing_probability_unified/t3_overlay_aggressive_risk_appetite_sweep_20260519/`
- lead scales: `1.0, 1.25, 1.5, 1.75, 2.0`
- overlay scales: `1.0, 1.25, 1.5, 2.0`
- capacity: `1.6, 2.0, 2.5, 3.0, 4.0`
- overlay extra round-trip slippage: `10, 15, 20, 25bp`
- impact profiles: `moderate_top1p6_active0p5`,
  `strict_top1p2_active1p0`, `severe_top1p0_active2p0`

## Anchor

Current conservative references:

| Row | Calendar | DD | Read |
|---|---:|---:|---|
| `lead_adverse10_exact` | 22.971648% | n/a | lead only conservative replay |
| `lead 1.0 / overlay 1.0 / capacity 1.6 / 15bp` | 24.548369% | -2.105913% | current 2.0x overlay portfolio haircut row |
| `conditional 1.25x lead / overlay 1.0 / strict 15bp` | 25.554185% | -2.121955% | previous conditional scale row |

## Aggressive Results

Best rows by practical stress band:

| Stress | Lead | Overlay | Capacity | Calendar | DD | Worst Month | Neg Months | Read |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| moderate 15bp | 2.0x | 2.0x | 4.0 | 38.856778% | -4.211826% | -3.151416% | 2 | high-return but likely too optimistic |
| strict 10bp | 1.5x | 2.0x | 2.5 | 35.521555% | -3.582801% | -2.414146% | 2 | viable aggressive research row if execution stays near 10bp |
| strict 15bp | 1.5x | 2.0x | 2.5 | 28.970948% | -4.179741% | -3.151416% | 2 | best stricter stress row, below 30% but above current conservative rows |
| strict 20bp | 1.5x | 1.0x | 2.5 | 24.118923% | -2.436468% | -1.944343% | 3 | overlay scale should be cut when 20bp stress is plausible |
| severe 15bp | 1.25x | 2.0x | 1.6 | 22.222649% | -4.163699% | -3.151416% | 2 | fails versus lead adverse10; severe book must block scaling |

## Decision

Yes, the earlier harness was risk-preference limited for research purposes:
it did not jointly sweep larger lead and overlay sizing in the same impact
matrix. Once that axis is opened, there is a real return/risk frontier.

The best aggressive candidate is not maximum leverage. Under strict impact,
`lead_scale=1.5`, `overlay_scale=2.0`, `capacity>=2.5` is the cleanest risk-on
row:

- 10bp overlay stress: 35.521555%, DD -3.582801%
- 15bp overlay stress: 28.970948%, DD -4.179741%

Pushing lead to `1.75x` or `2.0x` does not reliably improve the strict profile;
impact cost starts eating the extra notional. In severe profile, even 15bp
stress fails versus `lead_adverse10_exact`, so any real live candidate still
needs a depth/impact gate.

## Conditional Risk-On Sweep

After the unconditional matrix, `t3_overlay_conditional_lead_scale.py` was
extended from lead-only conditional scaling to per-leg conditional scaling:

- `target_lead_scale`
- `target_overlay_scale`
- separate lead / overlay impact gates
- multi-capacity sweeps

Outputs:

- `t3_overlay_conditional_risk_appetite_1p5x2p0_20260519/`
- `t3_overlay_conditional_risk_appetite_2p0x2p0_20260519/`

Key conditional rows:

| Policy | Stress | Capacity | Gate | Calendar | DD | Worst Month | Neg Months | Scaled Lead | Scaled Overlay | Read |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `lead 1.5x / overlay 2.0x` | moderate 15bp | 2.5 | 10bp | 33.871545% | -4.179741% | -3.151416% | 2 | 62/62 | 163/163 | optimistic execution band |
| `lead 1.5x / overlay 2.0x` | strict 10bp | 4.0 | 20bp | 35.521555% | -3.582801% | -2.414146% | 2 | 62/62 | 163/163 | viable only if realized overlay friction is near 10bp |
| `lead 1.5x / overlay 2.0x` | strict 15bp | 4.0 | 20bp | 28.970948% | -4.179741% | -3.151416% | 2 | 62/62 | 163/163 | best aggressive research risk-on row |
| `lead 1.5x / overlay 2.0x` | strict 15bp | 2.5 | 10bp | 25.607085% | -4.179741% | -3.151416% | 2 | 35/62 | 163/163 | lower-impact partial lead scale, still above prior conditional |
| `lead 2.0x / overlay 2.0x` | strict 15bp | 2.5 | 10bp | 24.473835% | -4.288626% | -3.151416% | 2 | 35/62 | 163/163 | worse than 1.5x; extra lead target is not efficient |
| `lead 1.5x / overlay 2.0x` | severe 15bp | 1.6 | 4bp | 21.231073% | -4.227741% | -3.151416% | 2 | 35/62 | 163/163 | kill-stress failure versus lead-only anchor |

Read:

- The harness was limiting risk appetite: once overlay scale and capacity are
  exposed, the same event set can move from the mid-20s to roughly 29-35%.
- The cost of that move is visible: DD roughly doubles from the conservative
  `-2.105913%` area to about `-4.18%`.
- `lead 2.0x` is not the next lever. Under strict 15bp, conditional `2.0x/2.0x`
  falls to `24.473835%`, while `1.5x/2.0x` keeps `28.970948%`.
- Overlay scaling is carrying much of the incremental return, so overlay
  friction calibration is now the key uncertainty.
- Severe profile still fails below `lead_adverse10_exact`; any promotion path
  must block risk-on sizing when live depth/impact resembles the severe band.

## Next Slice

Continue with risk calibration instead of another breakout-structure tweak:

1. Keep `lead_scale=1.5`, `overlay_scale=2.0` as the aggressive research row.
2. Keep `strict 15bp` as the primary validation stress and `severe 15bp` as the
   kill stress.
3. Add real-depth or production telemetry calibration before changing any live
   sizing defaults.
4. If live/testnet depth telemetry stays clean, promote a conditional ladder
   that selects between:
   - base: `lead 1.0 / overlay 1.0`
   - risk-on: `lead 1.5 / overlay 2.0`
   - block: no scale-up under severe/thin-book conditions.

## Testnet Shadow Alignment

Go/testnet shadow is aligned to the current research row with sandbox-only
1.5x lead quantity submission and guarded T3 overlay 2.0x entry proposals:

- template/session parameters now carry `pretouchShadowMode=testnet_shadow_collect`,
  `pretouchShadowLeadScale=1.5`, `pretouchShadowOverlayScale=2.0`,
  `pretouchShadowOverlayBaseShare=0.40`, `pretouchShadowOverlaySpeedThreshold=0.35`,
  `pretouchShadowSubmitRiskOnQuantity=true`, `pretouchShadowSubmitOverlayOrder=true`, and
  `pretouchShadowCandidateID=lead_1p5_overlay_2p0_strict15_20260519`;
- `testnet_shadow_collect` defaults to risk-on lead sizing so existing shadow
  sessions can collect real 1.5x fills after deploy; setting
  `pretouchShadowSubmitRiskOnQuantity=false` opts out;
- T3 overlay is a separate testnet-shadow event source: original_t2 must miss,
  strict `t3_swing` must touch, `abs(speed_300s_atr) >= 0.35`, and sandbox/rest
  depth/spread guards must pass before an `entry-t3-overlay` proposal is emitted;
  setting `pretouchShadowSubmitOverlayOrder=false` opts out;
- `pretouchShadowSizing` metadata records production quantity, submitted
  before/after shadow, shadow `1.5x` lead quantity, top-depth coverage after
  scale, spread guard, risk-on block reason, and the research reference metrics;
- `pretouchShadowOverlaySizing` metadata records overlay base share, overlay
  scale, submitted overlay quantity, top-depth coverage, spread guard, sandbox/rest
  state, and overlay block reason;
- live submitted entry quantity uses the `1.5x` lead quantity only when live
  semantics, account binding `sandbox=true`, `executionMode=rest`, and the
  shadow pre-submit guard all pass; otherwise it remains the production
  `suggestedQuantity`;
- overlay submitted entry quantity defaults to `pretouchBaseOrderQuantity * 0.40 * 2.0`
  (`0.080` ETH in the current template) and is never generated for mainnet/non-sandbox
  sessions.
- this live-shadow bridge covers only the T3 initial entry proposal; the research
  overlay's later reentry schedule and `min_hold_sl_60m` exit override remain
  research-only until separately implemented and reviewed.

The refreshed readiness output is
`t3_overlay_sizing_readiness_gate_risk_appetite_20260519/`:

- status `research_continue_collect_live_depth`;
- `1.5x` live telemetry `6/6` combined pass;
- min scaled top-depth coverage `6223.736842`;
- worst 8bp slippage headroom `2.795208bp`;
- strict 15bp proxy `28.970948%`;
- strict 20bp proxy `22.420341%`;
- severe 15bp proxy `21.231073%`;
- sample gate still blocked by `6 < 30`.
