# Research Runtime Recovery and Passive-Close Code Review & Remediation Plan

## Background

The research strategy logic was originally validated in offline backtesting and paper-like flows. Those environments do not fully exercise runtime-specific failure modes such as service restart, recovery of persisted positions, exchange state drift, or passive close execution for positions that were opened in a previous process.

In production-like live execution, the main bugs are not in the entry logic itself, but in the runtime handover path:

- service restarts and runtime recovery
- takeover of persisted historical positions
- passive close of previously opened positions
- exchange synchronization and reconciliation drift
- missing execution metadata during recovery-triggered exits

A representative failure pattern is an exit path that can decide to close a recovered position, but cannot actually route the order because required runtime metadata is missing or inconsistent.

This plan defines how to review, judge, and remediate those paths.

---

## Review Objectives

The goal is not to re-validate signal generation formulas. The goal is to prove that runtime state remains correct and actionable across restart, recovery, and takeover scenarios.

The review covers three high-risk scenarios:

1. **Restart & Recovery**  
   After backend restart, the system must correctly rediscover and manage previously open positions.

2. **Passive Close of Historical Positions**  
   The system must be able to close positions loaded from DB or synced from the exchange even when the current process did not create the original entry.

3. **Exchange Synchronization & Reconciliation**  
   The system must prevent DB, in-memory state, session state, and Binance reality from drifting into contradictory truths.

---

## Core Review Principles

The review must follow the runtime fact chain, not just file-by-file code reading.

For every path, answer these four questions in order:

1. **Fact source**: what is the authoritative truth for the current position and order state?
2. **Internal state**: how is that truth represented in DB, session state, and runtime state?
3. **Behavior**: what trading action can the system take from that state?
4. **Recovery**: after restart, can the system reconstruct the same truth and the same allowed behavior?

Anything that relies on cached or inferred state without reconciling against an authoritative fact source must be treated as high risk.

---

## Review Workstreams

### A. Position Fact Source Review

#### Goal
Identify every place where the system decides whether a position exists, what side it is on, what quantity it has, and what entry price it uses.

#### Scope
- `internal/service/live*.go`
- position store/repository accessors
- order/fill derived position reconstruction
- session state readers such as `livePositionState`, `recoveredPosition`, `virtualPosition`
- exchange snapshot and sync functions

#### Review Questions
For each function that reads or reconstructs position state:

1. What is the primary source of truth?
   - DB position
   - exchange position snapshot
   - session state cache
   - order/fill inference

2. Is the source a fact source, a cache, or a derived approximation?

3. If multiple sources are mixed, is there an explicit priority order?

4. Can a stale cached value overwrite a newer factual value?

5. Can `0`, empty, or missing fields be written back as if they were real values?

#### Required Output
A source-of-truth matrix for:
- normal running state
- startup recovery
- takeover of historical positions
- exchange reconcile path

---

### B. Restart Recovery Review

#### Goal
Verify that restart recovery is deterministic, idempotent, and safe.

#### Scope
- startup recovery flow
- live session recovery
- runtime session recovery
- position recovery and plan recovery
- session status rehydration

#### Review Questions
1. Is there a single recovery entrypoint, or multiple competing recovery paths?
2. Is recovery idempotent if triggered twice?
3. Can restart recovery accidentally trigger a new entry or duplicate exit?
4. Are recovered session fields sufficient to rebuild the same runtime semantics as before restart?
5. Is there any implicit auto-advance of plan state before position truth is re-established?

#### Required Judgment Rules
Recovery is acceptable only if:
- running recovery multiple times yields the same state
- missing session cache does not erase a real position
- recovery does not silently fabricate tradeable context from incomplete metadata
- recovery cannot auto-dispatch until position truth is confirmed

---

### C. Historical Position Takeover Review

#### Goal
Review the path where the system inherits an already-open position and is only expected to manage or close it.

#### Key Scenarios
1. DB position exists and exchange position exists and they match
2. DB position exists but exchange position does not
3. DB position does not exist but exchange position exists
4. DB and exchange positions both exist but side, quantity, or entry price differ

#### Review Questions
1. Does the system distinguish takeover mode from normal fresh-entry mode?
2. Does takeover mode allow only a restricted action set?
3. If position truth cannot be proven, does the system enter a restricted error/stale/conflict state?
4. Can the system accidentally treat a recovered historical position as a fresh strategy cycle?

#### Required Action Matrix
Define explicit allowed behavior per recovery state, for example:
- `close-only-takeover`
- `recovery-monitoring`
- `unprotected-open-position`
- `recovery-conflict`
- `stale-sync`

For each state, define whether the system may:
- open a new position
- close an existing position
- place protection orders
- auto-dispatch
- require manual review

---

### D. Execution Path and Passive-Close Routing Review

#### Goal
Ensure that recovery-triggered exit orders go through a complete and safe execution path.

#### Scope
- `internal/service/execution_strategy.go`
- order routing and order creation path
- exchange-specific submit path
- metadata assembly and order context generation

#### Review Questions
1. When a close decision is triggered for a recovered position, does the system still provide all required execution metadata?
2. Can the passive-close path skip metadata injection steps that are present for fresh orders?
3. Can the path fail because `strategyVersionId`, runtime link, or execution context is missing?
4. Are exchange-facing fields such as `reduceOnly`, `closePosition`, `positionSide`, and direction always validated for close-only behavior?
5. In hedge mode, is close routing precise enough to avoid cross-side mistakes?

#### Mandatory Remediation Item
Recovered historical positions must not rely on an incomplete execution shortcut. Recovery-triggered close orders must pass through the same metadata completeness checks as normal execution.

---

### E. Exchange Sync and Hard Reconciliation Review

#### Goal
Prevent divergence between DB state, in-memory state, session state, and Binance reality.

#### Scope
- exchange REST sync
- websocket runtime updates
- reconcile functions
- position and open-order snapshot ingestion
- reconciliation docs and requirements

#### Review Questions
1. Is there a combined WS + REST consistency model, or is the system over-trusting one side?
2. Before takeover becomes actionable, does the system force a hard REST reconciliation?
3. After WS reconnect or restart, can the system prove that local state still matches the exchange?
4. If Binance shows no position but DB still has one, does the system stop instead of issuing a blind exit?
5. Can stale local data be re-promoted into authoritative state during fallback?

#### Mandatory Remediation Item
Before a recovered historical position becomes eligible for monitoring, protection, or exit execution, the system must perform a synchronous Binance REST position check. If local and remote facts do not match, the system must enter `stale`, `conflict`, or `error` state and block automated actions.

---

### F. Plan Index and Runtime State Machine Review

#### Goal
Ensure that plan recovery and runtime progression never move ahead of proven position truth.

#### Review Questions
1. Can `planIndexRecoveredFromPosition` or similar logic advance plan state before the recovered position is validated?
2. Does the runtime state machine explicitly distinguish normal running state from recovery state?
3. Is there any “seamless transition” assumption that hides unresolved inconsistency?
4. If runtime/session linkage cannot be re-established, does the system downgrade into a restricted close-only mode rather than fabricating a normal linked state?

#### Required Rule
Plan progression must never auto-advance solely because a position-like shape was reconstructed. Position truth must be validated first.

---

## Required Additions to the Existing Plan

The following items are mandatory additions and must be explicitly included in implementation and review:

### 1. Recovery Context Completion Check
When taking over a historical position, the system must validate whether `StrategyVersionId`, execution context, and usable runtime/session linkage can be reconstructed. If not, it must not continue into a normal execution path.

### 2. Hard Reconcile Before Takeover Becomes Actionable
Recovered positions must not become tradable based only on DB state. A hard Binance REST position check is required before allowing automatic monitoring or close execution.

### 3. Full Metadata Assembly for Recovery-Triggered Close Orders
Recovery-triggered exits must not bypass metadata injection. They must carry the same minimum context guarantees as standard order execution.

### 4. Reduce-Only Safety Guard
All historical-position close orders must pass a reduce-only safety check before exchange submission. This is a final guardrail, not a substitute for correct state recovery.

### 5. WS + REST Dual-Channel Consistency Model
WS provides timeliness; REST provides authoritative catch-up. Restart, WS reconnect, and historical takeover must each force explicit REST verification.

### 6. No Automatic Plan Progression from Unproven Recovery
Recovered runtime state may enter monitoring, but cannot auto-progress into normal strategy action unless position truth is verified.

### 7. Explicit Action Matrix for Takeover States
Recovery-specific states must define allowed actions for open, close, protection, auto-dispatch, and manual review.

### 8. Targeted Runtime Verification Scenarios
The test plan must explicitly cover DB-backed historical takeover, exchange-only takeover, mismatch scenarios, and duplicate-exit prevention.

---

## Remediation Plan

### Phase 1: Deep Code Review
Deliverables:
- source-of-truth matrix
- restart recovery sequence diagram
- takeover action matrix
- list of P0/P1/P2 defects
- minimum regression test inventory

### Phase 2: Runtime Safety Fixes
Priority order:
1. Recovery metadata completion and close-only fallback mode
2. Hard reconciliation gate before takeover activation
3. Execution-path metadata completeness for passive-close orders
4. Reduce-only safety guard at exchange submit boundary
5. Plan/runtime state machine restrictions during recovery
6. WS reconnect and restart consistency checks

### Phase 3: Verification and Smoke Testing
Use mock/testnet or equivalent safe environment.

Required scenarios:
1. DB position exists, no active process, startup takeover succeeds
2. DB position exists, exchange matches, close signal executes without missing-context errors
3. DB position exists, exchange missing, system blocks automated close and marks stale/conflict
4. Exchange position exists, DB missing, system enters takeover mode without treating it as a fresh entry cycle
5. Existing open exit order is recognized so recovery does not create a duplicate exit order
6. Partial fill followed by restart preserves quantity truth and close behavior

---

## Severity Classification

### P0
May cause:
- duplicate order submission
- wrong-side closing
- unintended reverse opening
- failure to close a real position
- closing based on false local state

### P1
May cause:
- recovery state drift
- missing risk state
- blocked passive-close path
- incorrect plan index progression
- watchdog/protection not rebuilding correctly

### P2
May cause:
- weak observability
- incomplete logging
- missing or shallow regression tests

---

## Final Review Standard

The review is complete only when the team can prove the following:

1. A recovered historical position is never treated as tradable solely because it exists in DB.
2. A recovery-triggered close order never reaches the exchange without complete execution metadata.
3. Restart recovery is idempotent and cannot silently drift plan/runtime state.
4. Exchange truth can override stale local assumptions in a controlled way.
5. Recovery-specific runtime states have an explicit action matrix.
6. The regression suite contains startup takeover, mismatch, and duplicate-exit scenarios.

If those conditions are not met, the runtime recovery and passive-close flow must still be considered unsafe for live use.
