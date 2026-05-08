# 2026-05-05 No-`re_p` Observed Entry Baseline Plan

Scope: research-only plan. This note does not change live execution code.

## Decision

Drop `re_p` from the primary entry model.

`re_p` should no longer be used as:

- a fill price
- the primary live baseline gate
- an actionability anchor that entries must stay near

The reason is empirical and semantic:

- planned fills at `re_p` are unrealistic after breakout has already moved price
- requiring observed market price to remain near or satisfy `re_p` makes BTCUSDT 2026 Q1 30m collapse into a small-trade-count regime with poor returns
- reverting to immediate breakout market entry is also not acceptable, because that was the earliest strategy shape and had low Initial-entry quality

## Guardrail

Do not rebuild the new baseline as "breakout happens, immediately open market".

Historical research already moved away from that behavior:

- early `dir2_zero_initial=false` opened real `Initial` exposure directly at breakout
- `dir2_zero_initial=true` made `Initial` zero-notional because direct breakout exposure was not the desired edge
- the 2026-04-07 research note records the canonical semantic at that point: `Initial` records state only, while real exposure mainly comes from `Reentry`

The new baseline should preserve that lesson.

## Proposed Entry Semantics

Use breakout as a structure lock, not as an immediate market order.

Entry should be observed-market-price based and should require a post-breakout event that can happen in live without backfilling to a stale planned price.

### Candidate A: Virtual SL Then Second Breakout

This is closest to the user's recalled pre-window logic.

Flow:

1. First breakout arms a virtual zero-notional `Initial` state at the observed breakout price.
2. The virtual position can be invalidated by the existing ATR/structural SL logic.
3. After virtual SL, wait `sl_reentry_cooldown_seconds`.
4. First real exposure opens only if price makes a fresh observed reclaim/breakout after the cooldown.
5. Fill at observed market proxy, not `re_p`.

Long trigger after cooldown:

- observed `1s close >= initial_breakout_level` or a stricter fresh high above the post-SL local high
- fill at next observed proxy, preferably `next_1s_open`

Short trigger after cooldown:

- observed `1s close <= initial_breakout_level` or a stricter fresh low below the post-SL local low
- fill at next observed proxy, preferably `next_1s_open`

Primary cooldown sweep:

- `30s`
- `60s`
- `120s`

Recommended first candidate:

- `60s` cooldown
- trigger on reclaim of the original breakout level
- fill at `next_1s_open`

### Candidate B: Breakout Retest Then Reclaim

Flow:

1. Breakout arms a short-lived window.
2. Require a retest of the breakout level instead of a reclaim of `prev_low_1` / `prev_high_1`.
3. Enter only after price reclaims the breakout level again.
4. Fill at observed market proxy.

Long:

- breakout level = `prev_high_2`
- retest condition: observed low touches `breakout_level + retest_buffer`
- reclaim condition: observed close back above `breakout_level`
- fill: `next_1s_open`

Short:

- breakout level = `prev_low_2`
- retest condition: observed high touches `breakout_level - retest_buffer`
- reclaim condition: observed close back below `breakout_level`
- fill: `next_1s_open`

Retest buffer sweep:

- `0`
- `0.02 ATR`
- `0.05 ATR`

### Candidate C: Delayed Breakout Acceptance

This is the most direct continuation-style candidate, so it is a secondary test rather than the recommended baseline.

Flow:

1. Breakout arms a window.
2. Wait a fixed delay.
3. Enter only if price is still on the correct side of the breakout level and not overextended.
4. Fill at observed market proxy.

Delay sweep:

- `30s`
- `60s`
- `120s`

Overextension guard:

- max distance from breakout level: `10bps`, `20bps`, or `0.10 ATR`

This candidate checks whether a delayed acceptance filter can salvage some continuation edge without returning to immediate breakout chasing.

## Reference Rows

Every report should include these references:

1. Legacy planned fill:
   - `re_p` trigger/fill historical reference only.
2. Observed close + `re_p` gate/actionability:
   - demonstrates why `re_p` should not remain the primary gate.
3. Direct Initial breakout market entry:
   - historical negative-control row, showing why the new baseline must not become immediate breakout chasing.

## Metrics Required

Besides the usual return, MaxDD, trades, win rate, and Sharpe, report:

- entry mix by reason
- Initial/Zero-Initial/SL-Reentry win rate separately
- median seconds from first breakout lock to first real entry
- entry distance from breakout level in bps and ATR
- post-SL cooldown distribution
- rejected locks by reason
- T2/T3 attribution
- trade count and PnL by long/short side

## Recommendation

Start with Candidate A.

It keeps the important historical insight:

- first breakout is proof/state
- real money should wait for a later observed event
- no fill should be backfilled to `re_p`

If Candidate A is too sparse, test Candidate B before Candidate C. Candidate C is closer to continuation chasing and should not become the default unless the data clearly beats the alternatives.
