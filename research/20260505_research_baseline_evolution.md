# 2026-05-05 Research Baseline Evolution: Zero-Initial And Planned Reentry Fill

Scope: research-only history note. This note uses only `research` documents and research code as semantic sources. The removed replay module is not used as evidence.

## Finding

The optimistic planned-price reentry fill did not originate exclusively from `zero_initial_mode=reentry_window`.

The older reclaim reentry helper already used `re_p` as both the trigger level and the returned fill price:

- long reclaim trigger: `high >= re_p`, returned fill price: `re_p`
- short reclaim trigger: `low <= re_p`, returned fill price: `re_p`

What `reentry_window` changed was the exposure lifecycle:

- before window mode, `dir2_zero_initial=true` still created a persistent zero-notional `Initial` synthetic position after breakout, then managed that synthetic position through `SL/PT` and downstream reentry logic
- after window mode, breakout no longer created the persistent synthetic position; it opened a current-plus-next signal-bar reentry window, and the first real exposure became `Zero-Initial-Reentry`

Because same-bar reclaim stayed allowed, the existing planned-price fill became much more important: the strategy could observe a breakout, immediately see that the same bar also satisfies reclaim, and then book the first real order at the planned `re_p` rather than at the observed post-breakout price.

## Timeline

### 2026-04-16: Pre-window reclaim baseline

Document: `research/20260416_breakout_reentry_experiments.md`

The baseline was already defined around wick reentry anchors:

- long reentry at `prev_low_1 + 0.1ATR`
- short reentry at `prev_high_1`

The experiment also explicitly contrasted reclaim versus true pullback triggers:

- reclaim long: `bar.high >= re_p`
- reclaim short: `bar.low <= re_p`
- pullback long: `bar.low <= re_p`
- pullback short: `bar.high >= re_p`

The code path behind this uses `_reentry_triggered()`, which returns `re_p` for non-confirm reclaim and pullback modes.

### 2026-04-19: `zero_initial_mode=reentry_window` introduced

Documents:

- `research/20260419_zero_initial_reentry_window.md`
- `research/20260419_zero_initial_reentry_window_enhanced.md`

The documented problem statement says the prior canonical zero-initial behavior still created a persistent zero-notional position after `Initial` breakout, then managed that synthetic position with `SL/PT` exits.

The new research-only semantic changed that to:

- `Initial` breakout does not create a persistent position
- breakout opens a reentry window for the current signal bar and next signal bar
- no reentry inside the window resets state and waits for a fresh breakout

The important caveat was already recorded: the reentry trigger remained the existing reclaim rule, and same-bar reentry could still happen if `reP` was already satisfied.

### 2026-04-20: Window mode became the intraday research baseline

Documents:

- `research/20260420_eth_q1_reentry_window_second_bar_replay.md`
- `research/20260420_eth_q1_reentry_window_second_bar_replay_20_10_max2.md`

The baseline moved to:

- `dir2_zero_initial=true`
- `zero_initial_mode='reentry_window'`
- `reentry_trigger_mode='reclaim'`

The later 2026-04-20 note corrected sizing semantics:

- first real order in a signal bar: `20%`
- second real order in the same signal bar: `10%`
- `max_trades_per_bar=2`

This became the long-lived research baseline later copied into project memory.

### 2026-04-27 onward: T3 and gates optimized on top

Documents:

- `research/20260427_eth_q1_breakout_t3_shape_compare.md`
- `research/20260427_eth_q1_30min_t3_sma5_sep_0p25_marginal.md`
- `research/20260429_btc_q1_30min_low_vol_entry_filters.md`

These experiments retained the same zero-initial reentry-window baseline and added breakout shape, T3, and entry-quality gates on top of it.

## Practical Interpretation

The user's memory is directionally correct: before window mode, the research baseline had a first breakout that created a zero-notional synthetic `Initial` state, then later `SL/PT/reentry` management could produce actual exposure. This is close to "first breakout, then later reentry opens", although the implementation is better described as persistent synthetic-position management than a clean second-breakout-only rule.

The more precise diagnosis is:

1. planned-price fill was already in the old reclaim helper
2. `reentry_window` removed the persistent synthetic position and made `Zero-Initial-Reentry` the first real exposure
3. same-bar reclaim remained allowed
4. therefore the optimistic fill became central to the headline baseline performance

## Implication For New Baseline Work

The old planned-fill result should be treated as historical reference only. A new research baseline should keep the structural breakout, SL, trailing stop, profit protection, and reentry lifecycle as needed, but reentry fill should use an observed event-price proxy instead of backfilling to the planned `re_p`.

Removed optimization gates should be reintroduced only after this new observed-fill baseline is established.
