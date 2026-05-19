# T3 Overlay Position Scale Sensitivity

Research-only sizing check for the ETH pretouch lead enhancement. This report
compares the current 2.0x T3 overlay with a replayed 2.5x T3 overlay, then adds
a linear what-if for scaling the lead leg. The lead-scale rows are diagnostic
only: they reuse the exact lead exposure ledger and scale notional/PnL
linearly, without a larger-order book-cost replay.

## Replayed T3 Overlay Scale

Exact lead windows are used for portfolio accounting.

| Overlay scale | Schedule | T3 fee-net PnL | T3 gross PnL | Fees | T3 DD | Combined 0bp | Combined 10bp | Combined 15bp | Combined 20bp |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 2.0x | `[0.40,0.20]` | 11.402632% | 24.503851% | 13.101219% | -1.286147% | 34.374280% | 27.823672% | 24.548369% | 21.273065% |
| 2.5x | `[0.50,0.25]` | 14.290931% | 30.699754% | 16.408823% | -1.604931% | 37.262579% | 29.058168% | 24.955963% | 20.853758% |

Read: 2.5x overlay adds 2.89pp before extra slippage and 1.23pp at 10bp, but
only 0.41pp at 15bp. At 20bp it is worse than the 2.0x overlay and remains below
the lead adverse10 baseline. The 2.5x overlay is an aggressive research row, not
a clear live-candidate default.

## Lead Scale What-If

Rows below use the current 2.0x overlay unless noted. Capacity is raised enough
to avoid allocator clipping, so the table is testing leverage appetite rather
than fixed-cap allocation.

| Overlay | Lead scale | Capacity | Peak active | 0bp | 10bp | 15bp | 20bp | Max DD at 10bp |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 2.0x | 1.00x | 1.6 | 1.6 | 34.374280% | 27.823672% | 24.548369% | 21.273065% | -1.807443% |
| 2.0x | 1.25x | 2.0 | 2.0 | 40.117192% | 33.566584% | 30.291281% | 27.015977% | -1.823486% |
| 2.0x | 1.50x | 2.5 | 2.4 | 45.860104% | 39.309496% | 36.034192% | 32.758889% | -1.839528% |
| 2.5x | 1.00x | 1.6 | 1.6 | 37.262579% | 29.058168% | 24.955963% | 20.853758% | -2.240630% |
| 2.5x | 1.25x | 2.0 | 2.0 | 43.005491% | 34.801080% | 30.698875% | 26.596670% | -2.256673% |

Read: if capacity/leverage can safely rise, lead scaling is the cleaner
candidate than pushing overlay size. A 1.25x lead-scale what-if with the current
2.0x overlay keeps 20bp stress above the unscaled lead adverse10 baseline and
does not materially worsen sequential DD in this event-ledger model. This still
needs a stricter order-book impact model before any live-template sizing change.

## Decision

- Keep 2.0x overlay as the conservative research-lead enhancement row.
- Keep 2.5x overlay as an aggressive research row, not the default candidate.
- Next best path is to test a 1.25x lead-scale / capacity-2.0 scenario with a
  stricter order-book cost model. Do not promote this from the linear what-if
  alone.
