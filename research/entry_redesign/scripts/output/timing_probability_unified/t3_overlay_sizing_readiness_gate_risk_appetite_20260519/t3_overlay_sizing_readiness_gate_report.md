# T3 Overlay Sizing Readiness Gate

Research-only readiness gate for risk-on sizing.

## Verdict

- Status: `research_continue_collect_live_depth`
- Target lead scale: `1.50x`
- Target overlay scale: `2.00x`

## Evidence

| Check | Result |
|---|---:|
| Live samples | 6 |
| Live combined pass | 6/6 (100%) |
| Live min scaled top-depth coverage | 6223.736842 |
| Live worst 8bp slippage headroom | 2.795208bp |
| Live max adverse fill drift | 5.204792bp |
| Strict 15bp proxy calendar | 28.970948% |
| Strict 20bp proxy calendar | 22.420341% |
| Severe 15bp proxy calendar | 21.231073% |

## Thresholds

| Threshold | Value |
|---|---:|
| Lead adverse baseline | 22.971648% |
| Min live samples for live-candidate review | 30 |
| Min live combined pass ratio | 100% |
| Min worst slippage headroom | 2.00bp |
| Strict impact gate upper bound | 20.00bp |

## Reasons

- live telemetry passes current guard for target scale
- live sample size 6 is below promotion threshold 30
- strict 15bp proxy remains above lead adverse baseline
- strict 20bp remains a kill-stress failure
- severe 15bp fails by design, so thin-book scale should be blocked

## Read

- This is not a live sizing change and does not alter submitted quantity.
- `research_continue_collect_live_depth` means the candidate shape is still alive, but the sample size is too small for live promotion.
- If promoted later, the live-facing rule should remain conditional and fail closed to current sizing.
