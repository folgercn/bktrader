# T3 Overlay Live Depth Calibration

Research-only calibration from live execution telemetry. Raw production JSON is not stored here;
only sanitized execution-quality metrics are written.

## Source

- Source: `bktrader-ctl order list --limit 200 --json @ 2026-05-19T08:35Z`
- Entry samples: `6`

## Observed Entry Quality

| Metric | Value |
|---|---:|
| Max spread bps | 7.320544 |
| Max book age ms | 413.137042 |
| Max source divergence bps | 0.320167 |
| Min top-depth coverage | 9335.605263 |
| Max adverse fill drift bps | 5.204792 |
| P90 adverse fill drift bps | 2.791609 |

## Quantity Scale Matrix

| Scale | Pre-submit pass | Fill pass | Combined pass | Min scaled coverage | P10 scaled coverage | Max adverse drift | Worst 8bp headroom |
|---:|---:|---:|---:|---:|---:|---:|---:|
| 1.00x | 6/6 (100%) | 6/6 (100%) | 6/6 (100%) | 9335.605263 | 14929.198801 | 5.204792 | 2.795208 |
| 1.25x | 6/6 (100%) | 6/6 (100%) | 6/6 (100%) | 7468.48421 | 11943.359041 | 5.204792 | 2.795208 |
| 1.50x | 6/6 (100%) | 6/6 (100%) | 6/6 (100%) | 6223.736842 | 9952.799201 | 5.204792 | 2.795208 |
| 2.00x | 6/6 (100%) | 6/6 (100%) | 6/6 (100%) | 4667.802631 | 7464.5994 | 5.204792 | 2.795208 |
| 2.50x | 6/6 (100%) | 6/6 (100%) | 6/6 (100%) | 3734.242105 | 5971.67952 | 5.204792 | 2.795208 |

## Read

- Current live telemetry supports using top-depth coverage, book freshness, source divergence, and observed fill drift as the calibration surface.
- Passing this matrix is necessary but not sufficient for promotion: the sample is small and comes from testnet execution, so it should not override the research impact proxy by itself.
- A lead-scale lift should remain conditional; thin-book or high-drift cases must fall back to current sizing.
