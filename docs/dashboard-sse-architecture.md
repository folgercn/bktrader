# Dashboard SSE Architecture

## 1. Overview

Dashboard realtime data currently supports two transport modes:

1. **HTTP polling** — baseline and fallback mode.
2. **SSE streaming** — preferred realtime mode when `VITE_DASHBOARD_STREAM_ENABLED=true`.

The intent of the SSE path is to reduce high-frequency Dashboard API polling pressure while keeping the UI resilient. When SSE is unavailable, the frontend must fall back to HTTP polling automatically.

This document describes the current design introduced around PR #187, and the longer-term evolution path from polling-based SSE to event-driven realtime delivery tracked by #195.

## 2. High-level Data Flow

```text
React Dashboard hooks
  |
  |-- useDashboardStream()
  |     |
  |     |-- POST /api/v1/auth/stream-token
  |     |-- GET  /api/v1/stream/dashboard?token=<short-lived-stream-token>
  |
  |-- useDashboardRealtime()
        |
        |-- HTTP polling fallback when SSE is disabled or unavailable

Backend HTTP layer
  |
  |-- stream endpoint validates dashboard_stream token
  |-- subscribes client to DashboardBroker

DashboardBroker
  |
  |-- periodically fetches lightweight snapshots
  |-- hashes payloads to detect changes
  |-- publishes snapshot events to subscribers

Platform service
  |
  |-- ListLiveSessionsSummary()
  |-- ListOrdersWithLimit(50, 0)
  |-- ListFillsWithLimit(50, 0)
  |-- ListPositions()
  |-- ListAlerts()
  |-- ListNotifications(true)
  |-- HealthSnapshot()
```

## 3. Frontend Runtime Behavior

### 3.1 SSE enabled

When `VITE_DASHBOARD_STREAM_ENABLED=true`:

1. The frontend requests a short-lived stream token through `/api/v1/auth/stream-token` using the normal authenticated API session.
2. The frontend opens an EventSource connection to `/api/v1/stream/dashboard?token=<stream-token>`.
3. Once the SSE connection is established, realtime HTTP polling is stopped.
4. If the SSE connection fails, the frontend reconnects with backoff and HTTP polling can be used as fallback.

### 3.2 SSE disabled

When `VITE_DASHBOARD_STREAM_ENABLED=false`:

- Dashboard realtime data continues to use HTTP polling.
- The SSE token endpoint and stream endpoint are not used by the frontend.

### 3.3 Snapshot semantics

Current SSE events are **snapshot-only**:

- Each event replaces the full frontend state slice for that domain.
- This keeps the frontend reducer behavior simple and idempotent.
- Incremental `upsert` / `delete` events are intentionally not part of the current protocol.

## 4. Backend Stream Endpoint

Endpoint:

```http
GET /api/v1/stream/dashboard?token=<stream-token>
```

The endpoint:

1. Validates request method.
2. Verifies streaming support via `http.Flusher`.
3. Validates the query token as a stream token.
4. Verifies `scope == dashboard_stream`.
5. Checks that `DashboardBroker` is initialized.
6. Sets SSE headers.
7. Subscribes the client and pushes initial snapshots.
8. Sends keepalive comments periodically.

SSE headers are only set after method/auth/broker checks pass, so error responses are returned as normal HTTP errors rather than malformed event-stream responses.

## 5. Stream Token Design

The SSE connection uses a short-lived stream token instead of the main API token.

### 5.1 API token

- Sent through `Authorization: Bearer <token>`.
- Used for normal API requests.
- Used to request a stream token.

### 5.2 Stream token

- Issued by `POST /api/v1/auth/stream-token`.
- Has a very short TTL, currently around 1 minute.
- Carries `scope=dashboard_stream`.
- Carries a `jti` claim for future revoke/audit extension.
- Is accepted only by the stream endpoint.

### 5.3 Rationale

Native EventSource cannot set custom request headers. Passing the main API token in the query string would expose a high-privilege token to URLs and logs.

The short-lived stream token limits this risk:

- It has a narrow scope.
- It expires quickly.
- It is only useful for the SSE stream endpoint.

Stronger JTI uniqueness with UUID has been implemented in PR #198 (Issue #193).

## 6. DashboardBroker Design

`DashboardBroker` is responsible for polling selected backend data, detecting changes, and publishing SSE events to subscribers.

### 6.1 Subscriber model

- Each SSE client subscribes with a buffered channel.
- Events are broadcast to all active subscribers.
- If a subscriber channel is full, the event is dropped for that subscriber.
- This prevents one slow browser tab from blocking the broker.

### 6.2 No-subscriber behavior

When there are no subscribers, `checkAndPublish` returns immediately.

This avoids unnecessary:

- store reads
- JSON marshaling
- hashing
- object allocation

### 6.3 Change detection

For each domain payload:

1. Fetch latest snapshot.
2. JSON marshal the payload.
3. Calculate SHA-256 hash.
4. Compare with the previous hash.
5. Publish only when the hash changes.

Initial snapshots also update `lastHashes`, so the first polling tick after a new connection does not immediately resend unchanged data.

### 6.4 Sequence numbers

Every published event has a monotonically increasing `seq`.

Initial snapshots also increment `seq`, so multiple initial events do not share the same sequence number.

The current frontend does not rely on `seq` for ordering or recovery, but keeping it monotonic preserves room for future reconnect and replay semantics.

## 7. Payload Strategy

Current SSE payloads are deliberately conservative.

| Event type | Payload source | Notes |
|---|---|---|
| `live-sessions` | `ListLiveSessionsSummary()` | Strips heavy `sourceStates` and `signalBarStates`. |
| `positions` | `ListPositions()` | Full position list. |
| `orders` | `ListOrdersWithLimit(50, 0)` | Recent 50 only. |
| `fills` | `ListFillsWithLimit(50, 0)` | Recent 50 only. |
| `alerts` | `ListAlerts()` | Full alert list for now. |
| `notifications` | `ListNotifications(true)` | Includes acknowledged notifications. |
| `monitor-health` | `HealthSnapshot()` | Health snapshot. |

`live-sessions` must use the same summary boundary as the HTTP Dashboard summary view. This avoids reintroducing the large response problem solved by the summary endpoints.

## 8. HTTP Polling Fallback

HTTP polling remains a required fallback path.

Fallback is used when:

- stream mode is disabled;
- stream token request fails;
- EventSource cannot connect;
- the stream disconnects for a sustained period.

Short disconnects are handled with a debounce before polling resumes.

## 9. Configuration

Frontend:

```env
VITE_DASHBOARD_STREAM_ENABLED=false
VITE_DASHBOARD_REALTIME_POLL_MS=5000
VITE_DASHBOARD_STATE_POLL_MS=15000
VITE_DASHBOARD_CONFIG_POLL_MS=60000
```

Backend broker polling intervals are configurable per domain:

```env
DASHBOARD_LIVE_SESSIONS_POLL_MS=2000
DASHBOARD_POSITIONS_POLL_MS=2000
DASHBOARD_ORDERS_POLL_MS=2000
DASHBOARD_FILLS_POLL_MS=2000
DASHBOARD_ALERTS_POLL_MS=2000
DASHBOARD_NOTIFICATIONS_POLL_MS=2000
DASHBOARD_MONITOR_HEALTH_POLL_MS=2000
```

Backend config enforces minimum interval protection so accidental very-low values do not create excessive load.

## 10. Failure Modes

### 10.1 Stream token request fails

Expected behavior:

- The frontend marks SSE as disconnected.
- Reconnect is attempted with backoff.
- HTTP polling fallback should continue to keep Dashboard usable.

### 10.2 SSE connection drops

Expected behavior:

- EventSource closes.
- Frontend reconnects with backoff.
- HTTP polling fallback resumes if SSE remains unavailable.

### 10.3 Broker fetch fails for one domain

Expected behavior:

- That domain event is skipped for the current tick.
- Other domains continue independently.
- The stream stays open.

### 10.4 Slow subscriber

Expected behavior:

- Events may be dropped for that subscriber when its buffer is full.
- Broker must not block all clients because one tab is slow.

## 11. Current Limitations

1. SSE events are snapshot-only.
2. No replay or resume from `Last-Event-ID` yet.
3. Subscriber drops are silent.
4. Error aggregation between SSE and polling is still basic.

## 12. Follow-up Roadmap

Tracked by issue #195:

- #195: Event-driven DashboardBroker design and phased implementation.

Possible future work:

- Add stream metrics: active subscribers, dropped events, publish latency.
- Add `Last-Event-ID` support.
- Add incremental `upsert` / `delete` actions.
- Move broker from polling-diff to event-driven publishing where possible.
- Add per-domain error events for frontend visibility.

## 13. Transport Evolution: HTTP Polling -> SSE -> WebSocket -> Event-driven

The current SSE design is a good middle step. It reduces browser-driven polling pressure without requiring a full duplex protocol. Future evolution should be staged instead of jumping directly to WebSocket.

### 13.1 Stage 0: HTTP polling

Status: existing fallback.

Characteristics:

- Client periodically pulls data.
- Simple and reliable.
- Expensive when many tabs or users are open.
- Latency is bounded by polling interval.

Keep this path permanently as a fallback.

### 13.2 Stage 1: SSE snapshot stream

Status: current design from PR #187.

Characteristics:

- Server pushes snapshot events over one-way stream.
- Frontend applies idempotent state replacement.
- HTTP polling remains fallback.
- Broker still internally polls and hashes payloads.

This stage is intentionally conservative. It improves user-perceived latency and reduces repeated browser requests while avoiding complicated client-side reconcile logic.

### 13.3 Stage 2: SSE incremental events

Target: after snapshot stream stabilizes.

Possible actions:

- `snapshot`: replace full slice.
- `upsert`: insert or update one record.
- `delete`: remove one record.
- `patch`: update selected fields.
- `error`: report per-domain data fetch problem.

Recommended approach:

1. Keep snapshot as the authoritative recovery mechanism.
2. Introduce incremental events per domain, starting with low-risk data such as notifications or alerts.
3. Add frontend reducer tests before enabling incremental mode for trading-sensitive panels.
4. Periodically send snapshot events to repair drift.

Do not remove snapshot support when incremental events are introduced.

### 13.4 Stage 3: SSE replay and resume

Target: improve reconnect correctness.

Possible additions:

- Respect `Last-Event-ID` from EventSource.
- Keep a bounded ring buffer of recent events per domain.
- Resume from last acknowledged event when possible.
- Fall back to initial snapshots if the requested event is too old.

This requires `seq` and event IDs to be stable and monotonic. PR #187 already keeps `seq` monotonic, which is a useful foundation.

### 13.5 Stage 4: Event-driven broker

Target: remove internal polling-diff where the backend already knows something changed.

Possible data sources:

- order created / status changed
- fill persisted
- position updated
- live session state changed
- alert emitted
- notification created / acked
- health state changed

Design direction:

```text
Domain write path
  -> domain event
  -> DashboardBroker.Publish(...)
  -> SSE subscribers
```

Polling can remain only for data sources that do not have reliable write-path events.

This is the main path to reduce backend CPU/GC cost further, because it avoids periodic marshal/hash for unchanged data.

### 13.6 Stage 5: WebSocket, only if bidirectional control is required

WebSocket is not automatically better than SSE for Dashboard telemetry.

Prefer staying on SSE while the Dashboard only needs server-to-client updates.

Consider WebSocket only if the product requires:

- browser-to-server interactive subscriptions;
- per-panel dynamic topic selection without reopening streams;
- client acknowledgements;
- low-latency command/control flows;
- multiplexing many logical streams over one connection;
- collaborative or multi-operator realtime interactions.

If WebSocket is introduced, trading command execution must remain carefully separated from telemetry. Do not mix trading actions and passive Dashboard updates in the same unreviewed protocol path.

## 14. Operational Observability Roadmap

To operate the SSE path safely, add broker and stream metrics before relying on it as the only realtime path.

Recommended metrics:

- active SSE subscribers
- subscriber connect/disconnect count
- stream token issue count
- stream token auth failure count
- per-domain publish count
- per-domain dropped event count
- per-domain marshal/hash duration
- per-domain payload size
- reconnect count on frontend
- fallback polling activation count

Recommended logs:

- stream connection accepted
- stream auth rejected with reason class, without logging token
- subscriber removed due to client disconnect
- dropped event due to full channel
- broker fetch failure by domain

Avoid logging stream tokens, API tokens, or full SSE URLs containing query tokens.

## 15. Safety Boundaries

Dashboard SSE is a telemetry channel. It must not become an execution path.

Rules:

- Do not send trading commands through Dashboard SSE.
- Do not add write actions to the SSE endpoint.
- Keep stream tokens scoped to `dashboard_stream`.
- Keep normal API tokens out of query strings.
- Keep HTTP polling fallback until SSE has enough production soak time.
- Keep summary payloads lightweight by default.

## 16. Decision Matrix: When to Change Transport

| Need | Recommended transport |
|---|---|
| Simple dashboard updates | SSE |
| One-way server push | SSE |
| Must support legacy fallback | SSE + HTTP polling |
| Client sends frequent realtime commands | WebSocket |
| Need ack/replay only | SSE with `Last-Event-ID` first |
| Need topic subscription changes at high frequency | WebSocket may be justified |
| Need trading actions | Normal reviewed API path, not SSE |

## 17. Design Principles

- Keep trade execution paths untouched.
- Keep SSE payloads lightweight by default.
- Prefer snapshot idempotency until incremental semantics are well-tested.
- Keep HTTP polling as a reliable fallback.
- Avoid exposing high-privilege API tokens in URLs.
- Keep summary field boundaries centralized and consistent.
- Move from polling-diff to event-driven publishing incrementally.
- Use WebSocket only when bidirectional realtime requirements clearly justify it.
