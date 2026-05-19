# Lead Exact Exposure Windows

Research-only rebuild of the ETH pretouch lead exposure ledger.

## Summary

| Calendar sum | Worst month | Neg months | Trades | Missing entry | Missing exit | Max hold |
|---:|---:|---:|---:|---:|---:|---:|
| 22.971648% | 1.395821% | 0 | 62 | 0 | 0 | 7200.00s |

## Reference Parity

| Reference rows | Exact rows | Missing exact events | Extra exact events | Delay mismatches | Max weighted PnL diff | Max position diff | Max delay PnL diff |
|---:|---:|---:|---:|---:|---:|---:|---:|
| 62 | 62 | 0 | 0 | 0 | 0.000000000000000 | 0.000000000000000 | 0.000000000000000 |

## Read

- Windows use the selected `DelayResult` entry/exit timestamps from the production-aligned lead replay.
- PnL parity is checked against the existing compact adverse10 lead ledger before using this in portfolio sensitivity.
- The adverse fill proxy keeps the execution simulator's exit time and applies next-second adverse entry repricing as a first-order stress.

## Diagnostics

```json
{
  "symbol": "ETHUSDT",
  "scenario": "next_adverse_xslip10bps",
  "lead_replayed_events": 68,
  "lead_delay_errors": 0,
  "lead_exec_params": {
    "initial_stop_atr": 0.45,
    "stop_buffer_atr": 0.05,
    "stop_cap_atr": 0.8,
    "min_stop_bps": 12.0,
    "breakeven_at_r": 0.8,
    "cost_lock_bps": 10.0,
    "trail_start_r": 1.5,
    "trail_buffer_atr": 0.05,
    "max_hold_hours": 2.0,
    "slippage": 0.0002,
    "entry_fee": 0.0002,
    "exit_fee": 0.0004
  },
  "exact_rows_all": 68,
  "exact_rows_gate_on": 62,
  "missing_entry_times_gate_on": 0,
  "missing_exit_times_gate_on": 0
}
```
