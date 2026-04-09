import React from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { ActionButton } from '../components/ui/ActionButton';
import { SimpleTable } from '../components/ui/SimpleTable';
import { StatusPill } from '../components/ui/StatusPill';
import { SignalBarChart } from '../components/charts/SignalBarChart';
import { formatTime, formatMaybeNumber, shrink } from '../utils/format';
import { strategyLabel, getNumber, getRecord, getList, boolLabel, deriveSignalBarCandles, buildTimelineNotes } from '../utils/derivation';

interface AccountSidePanelProps {
  bindAccountSignalSource: () => void;
  bindStrategySignalSource: () => void;
  updateRuntimePolicy: () => void;
  createSignalRuntimeSession: () => void;
  runSignalRuntimeAction: (id: string, action: "start" | "stop") => void;
}

export function AccountSidePanel({
  bindAccountSignalSource,
  bindStrategySignalSource,
  updateRuntimePolicy,
  createSignalRuntimeSession,
  runSignalRuntimeAction
}: AccountSidePanelProps) {
  const signalCatalog = useTradingStore(s => s.signalCatalog);
  const signalRuntimeSessions = useTradingStore(s => s.signalRuntimeSessions);
  const accountSignalForm = useUIStore(s => s.accountSignalForm);
  const setAccountSignalForm = useUIStore(s => s.setAccountSignalForm);
  const liveAccounts = useTradingStore(s => s.accounts);
  const signalBindingAction = useUIStore(s => s.signalBindingAction);
  const strategySignalForm = useUIStore(s => s.strategySignalForm);
  const setStrategySignalForm = useUIStore(s => s.setStrategySignalForm);
  const strategies = useTradingStore(s => s.strategies);
  const signalSourceTypes = useTradingStore(s => s.signalSourceTypes);
  const accountSignalBindings = useTradingStore(s => s.accountSignalBindings);
  const strategySignalBindings = useTradingStore(s => s.strategySignalBindings);
  const runtimePolicyForm = useUIStore(s => s.runtimePolicyForm);
  const setRuntimePolicyForm = useUIStore(s => s.setRuntimePolicyForm);
  const runtimePolicyAction = useUIStore(s => s.runtimePolicyAction);
  const runtimePolicy = useTradingStore(s => s.runtimePolicy);
  const signalRuntimeForm = useUIStore(s => s.signalRuntimeForm);
  const setSignalRuntimeForm = useUIStore(s => s.setSignalRuntimeForm);
  const signalRuntimeAction = useUIStore(s => s.signalRuntimeAction);
  const signalRuntimePlan = useTradingStore(s => s.signalRuntimePlan);
  const signalRuntimeAdapters = useTradingStore(s => s.signalRuntimeAdapters);
  const selectedSignalRuntimeId = useTradingStore(s => s.selectedSignalRuntimeId);
  const setSelectedSignalRuntimeId = useTradingStore(s => s.setSelectedSignalRuntimeId);

  // Derived states
  const strategyOptions = strategies.map((strategy) => ({
    value: strategy.id,
    label: strategyLabel(strategy),
  }));

  const selectedSignalRuntime =
    signalRuntimeSessions.find((item) => item.id === selectedSignalRuntimeId) ?? signalRuntimeSessions[0] ?? null;
  const selectedSignalRuntimeState = getRecord(selectedSignalRuntime?.state);
  const selectedSignalRuntimePlan = getRecord(selectedSignalRuntimeState.plan);
  const selectedSignalRuntimeLastSummary = getRecord(selectedSignalRuntimeState.lastEventSummary);
  const selectedSignalRuntimeSourceStates = getRecord(selectedSignalRuntimeState.sourceStates);
  const selectedSignalBarStates = getRecord(selectedSignalRuntimeState.signalBarStates);
  const selectedSignalRuntimeTimeline = getList(selectedSignalRuntimeState.timeline);
  const selectedSignalRuntimeSignalBars = deriveSignalBarCandles(selectedSignalRuntimeSourceStates);
  const selectedSignalRuntimeSubscriptions = Array.isArray(selectedSignalRuntimeState.subscriptions)
    ? (selectedSignalRuntimeState.subscriptions as Array<Record<string, unknown>>)
    : [];

  return (
    <div className="p-4 space-y-6">
      <section id="signals" className="panel panel-session">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">Signal Runtime</p>
            <h3>信号源绑定与市场数据运行时</h3>
          </div>
          <div className="range-box">
            <span>{signalCatalog?.sources?.length ?? 0} sources</span>
            <span>{signalRuntimeSessions.length} sessions</span>
          </div>
        </div>

        <div className="live-grid">
          <div className="backtest-form session-form">
            <h4>Bind Account Signal Source</h4>
            <div className="form-grid">
              <label className="form-field">
                <span>Account</span>
                <select value={accountSignalForm.accountId} onChange={(event) => setAccountSignalForm((current: any) => ({ ...current, accountId: event.target.value }))}>
                  {liveAccounts.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name} ({item.mode})
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>Source</span>
                <select value={accountSignalForm.sourceKey} onChange={(event) => setAccountSignalForm((current: any) => ({ ...current, sourceKey: event.target.value }))}>
                  {(signalCatalog?.sources ?? []).map((source) => (
                    <option key={source.key} value={source.key}>
                      {source.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>Role</span>
                <select value={accountSignalForm.role} onChange={(event) => setAccountSignalForm((current: any) => ({ ...current, role: event.target.value }))}>
                  <option value="signal">signal</option>
                  <option value="trigger">trigger</option>
                  <option value="feature">feature</option>
                </select>
              </label>
              <label className="form-field">
                <span>Timeframe</span>
                <select value={accountSignalForm.timeframe} onChange={(event) => setAccountSignalForm((current: any) => ({ ...current, timeframe: event.target.value }))}>
                  <option value="4h">4h</option>
                  <option value="1d">1d</option>
                </select>
              </label>
              <label className="form-field">
                <span>Symbol</span>
                <input value={accountSignalForm.symbol} onChange={(event) => setAccountSignalForm((current: any) => ({ ...current, symbol: event.target.value.toUpperCase() }))} />
              </label>
            </div>
            <div className="backtest-actions">
              <ActionButton label={signalBindingAction === "account" ? "Binding..." : "Bind Account Source"} disabled={signalBindingAction !== null || !accountSignalForm.accountId} onClick={bindAccountSignalSource} />
            </div>
          </div>

          <div className="backtest-form session-form">
            <h4>Bind Strategy Signal Source</h4>
            <div className="form-grid">
              <label className="form-field">
                <span>Strategy</span>
                <select value={strategySignalForm.strategyId} onChange={(event) => setStrategySignalForm((current: any) => ({ ...current, strategyId: event.target.value }))}>
                  {strategyOptions.map((strategy) => (
                    <option key={strategy.value} value={strategy.value}>
                      {strategy.label}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>Source</span>
                <select value={strategySignalForm.sourceKey} onChange={(event) => setStrategySignalForm((current: any) => ({ ...current, sourceKey: event.target.value }))}>
                  {(signalCatalog?.sources ?? []).map((source) => (
                    <option key={source.key} value={source.key}>
                      {source.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>Role</span>
                <select value={strategySignalForm.role} onChange={(event) => setStrategySignalForm((current: any) => ({ ...current, role: event.target.value }))}>
                  <option value="signal">signal</option>
                  <option value="trigger">trigger</option>
                  <option value="feature">feature</option>
                </select>
              </label>
              <label className="form-field">
                <span>Timeframe</span>
                <select value={strategySignalForm.timeframe} onChange={(event) => setStrategySignalForm((current: any) => ({ ...current, timeframe: event.target.value }))}>
                  <option value="4h">4h</option>
                  <option value="1d">1d</option>
                </select>
              </label>
              <label className="form-field">
                <span>Symbol</span>
                <input value={strategySignalForm.symbol} onChange={(event) => setStrategySignalForm((current: any) => ({ ...current, symbol: event.target.value.toUpperCase() }))} />
              </label>
            </div>
            <div className="backtest-actions">
              <ActionButton label={signalBindingAction === "strategy" ? "Binding..." : "Bind Strategy Source"} disabled={signalBindingAction !== null || !strategySignalForm.strategyId} onClick={bindStrategySignalSource} />
            </div>
          </div>
        </div>

        <div className="live-grid">
          <div className="backtest-list">
            <h4>Signal Source Catalog</h4>
            {signalCatalog?.sources?.length ? (
              <SimpleTable
                columns={["Source", "Exchange", "Type", "Roles", "Env", "Transport"]}
                rows={signalCatalog.sources.map((source) => [
                  source.name,
                  source.exchange,
                  source.streamType,
                  source.roles.join(", "),
                  source.environments.join(", "),
                  source.transport,
                ])}
                emptyMessage="No signal sources"
              />
            ) : (
              <div className="empty-state empty-state-compact">No signal source catalog</div>
            )}
            <div className="backtest-notes">
              {(signalCatalog?.notes ?? []).map((note) => (
                <div key={note} className="note-item">
                  {note}
                </div>
              ))}
              {(signalSourceTypes ?? []).map((item) => (
                <div key={item.streamType} className="note-item">
                  {item.streamType}: {item.description}
                </div>
              ))}
            </div>
          </div>

          <div className="backtest-list">
            <h4>Current Bindings</h4>
            <div className="backtest-breakdown">
              <h5>Account</h5>
              <SimpleTable
                columns={["Source", "Role", "Symbol", "Exchange", "Status"]}
                rows={accountSignalBindings.map((item) => [item.sourceName, item.role, item.symbol || "--", item.exchange, item.status])}
                emptyMessage="No account bindings"
              />
            </div>
            <div className="backtest-breakdown">
              <h5>Strategy</h5>
              <SimpleTable
                columns={["Source", "Role", "Symbol", "Exchange", "Status"]}
                rows={strategySignalBindings.map((item) => [item.sourceName, item.role, item.symbol || "--", item.exchange, item.status])}
                emptyMessage="No strategy bindings"
              />
            </div>
          </div>
        </div>

        <div className="live-grid">
          <div className="backtest-form session-form">
            <h4>Runtime Policy</h4>
            <div className="form-grid">
              <label className="form-field">
                <span>Trade Tick Freshness (s)</span>
                <input
                  value={runtimePolicyForm.tradeTickFreshnessSeconds}
                  onChange={(event) =>
                    setRuntimePolicyForm((current: any) => ({ ...current, tradeTickFreshnessSeconds: event.target.value }))
                  }
                />
              </label>
              <label className="form-field">
                <span>Order Book Freshness (s)</span>
                <input
                  value={runtimePolicyForm.orderBookFreshnessSeconds}
                  onChange={(event) =>
                    setRuntimePolicyForm((current: any) => ({ ...current, orderBookFreshnessSeconds: event.target.value }))
                  }
                />
              </label>
              <label className="form-field">
                <span>Signal Bar Freshness (s)</span>
                <input
                  value={runtimePolicyForm.signalBarFreshnessSeconds}
                  onChange={(event) =>
                    setRuntimePolicyForm((current: any) => ({ ...current, signalBarFreshnessSeconds: event.target.value }))
                  }
                />
              </label>
              <label className="form-field">
                <span>Runtime Quiet (s)</span>
                <input
                  value={runtimePolicyForm.runtimeQuietSeconds}
                  onChange={(event) =>
                    setRuntimePolicyForm((current: any) => ({ ...current, runtimeQuietSeconds: event.target.value }))
                  }
                />
              </label>
              <label className="form-field">
                <span>Paper Start Timeout (s)</span>
                <input
                  value={runtimePolicyForm.paperStartReadinessTimeoutSeconds}
                  onChange={(event) =>
                    setRuntimePolicyForm((current: any) => ({
                      ...current,
                      paperStartReadinessTimeoutSeconds: event.target.value,
                    }))
                  }
                />
              </label>
            </div>
            <div className="backtest-actions">
              <ActionButton
                label={runtimePolicyAction ? "Saving..." : "Save Runtime Policy"}
                disabled={runtimePolicyAction}
                onClick={updateRuntimePolicy}
              />
            </div>
            <div className="backtest-notes">
              <div className="note-item">
                active policy: tick {runtimePolicy?.tradeTickFreshnessSeconds ?? "--"}s · book {runtimePolicy?.orderBookFreshnessSeconds ?? "--"}s ·
                bar {runtimePolicy?.signalBarFreshnessSeconds ?? "--"}s
              </div>
              <div className="note-item">
                quiet {runtimePolicy?.runtimeQuietSeconds ?? "--"}s · paper preflight {runtimePolicy?.paperStartReadinessTimeoutSeconds ?? "--"}s
              </div>
            </div>
          </div>

          <div className="backtest-form session-form">
            <h4>Create Runtime Session</h4>
            <div className="form-grid">
              <label className="form-field">
                <span>Account</span>
                <select value={signalRuntimeForm.accountId} onChange={(event) => setSignalRuntimeForm((current: any) => ({ ...current, accountId: event.target.value }))}>
                  {liveAccounts.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name} ({item.mode})
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>Strategy</span>
                <select value={signalRuntimeForm.strategyId} onChange={(event) => setSignalRuntimeForm((current: any) => ({ ...current, strategyId: event.target.value }))}>
                  {strategyOptions.map((strategy) => (
                    <option key={strategy.value} value={strategy.value}>
                      {strategy.label}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <div className="backtest-actions">
              <ActionButton label={signalRuntimeAction === "create" ? "Creating..." : "Create Runtime Session"} disabled={signalRuntimeAction !== null || !signalRuntimeForm.accountId || !signalRuntimeForm.strategyId} onClick={createSignalRuntimeSession} />
            </div>
            <div className="detail-grid">
              <div className="detail-item">
                <span>Plan Ready</span>
                <strong>{boolLabel(signalRuntimePlan?.ready)}</strong>
              </div>
              <div className="detail-item">
                <span>Required</span>
                <strong>{String((signalRuntimePlan?.requiredBindings as unknown[] | undefined)?.length ?? 0)}</strong>
              </div>
              <div className="detail-item">
                <span>Matched</span>
                <strong>{String((signalRuntimePlan?.matchedBindings as unknown[] | undefined)?.length ?? 0)}</strong>
              </div>
              <div className="detail-item">
                <span>Missing</span>
                <strong>{String((signalRuntimePlan?.missingBindings as unknown[] | undefined)?.length ?? 0)}</strong>
              </div>
            </div>
            <div className="backtest-notes">
              <div className="note-item">runtime adapters: {signalRuntimeAdapters.map((item) => item.key).join(", ") || "--"}</div>
              {((signalRuntimePlan?.missingBindings as unknown[] | undefined) ?? []).slice(0, 4).map((item, index) => (
                <div key={index} className="note-item">
                  missing: {JSON.stringify(item)}
                </div>
              ))}
            </div>
          </div>

          <div className="backtest-list">
            <h4>Runtime Sessions</h4>
            {signalRuntimeSessions.length > 0 ? (
              <>
                <div className="table-wrap">
                  <table>
                    <thead>
                      <tr>
                        <th>Session</th>
                        <th>Status</th>
                        <th>Adapter</th>
                        <th>Subs</th>
                        <th>Heartbeat</th>
                        <th>Action</th>
                      </tr>
                    </thead>
                    <tbody>
                      {signalRuntimeSessions.map((session) => (
                        <tr
                          key={session.id}
                          className={session.id === selectedSignalRuntime?.id ? "table-row-active" : ""}
                          onClick={() => setSelectedSignalRuntimeId(session.id)}
                        >
                          <td>{shrink(session.id)}</td>
                          <td>{session.status}</td>
                          <td>{session.runtimeAdapter || "--"}</td>
                          <td>{String(session.subscriptionCount)}</td>
                          <td>{formatTime(String(session.state?.lastHeartbeatAt ?? ""))}</td>
                          <td>
                            <div className="inline-actions">
                              <ActionButton
                                label={signalRuntimeAction === `${session.id}:start` ? "Starting..." : "Start"}
                                disabled={signalRuntimeAction !== null || session.status === "RUNNING"}
                                onClick={() => runSignalRuntimeAction(session.id, "start")}
                              />
                              <ActionButton
                                label={signalRuntimeAction === `${session.id}:stop` ? "Stopping..." : "Stop"}
                                variant="ghost"
                                disabled={signalRuntimeAction !== null || session.status === "STOPPED"}
                                onClick={() => runSignalRuntimeAction(session.id, "stop")}
                              />
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                <div className="backtest-detail-card">
                  <div className="panel-header">
                    <div>
                      <p className="panel-kicker">Signal Session</p>
                      <h3>选中 Runtime Session 详情</h3>
                    </div>
                    <div className="range-box">
                      <span>{selectedSignalRuntime?.status ?? "NO SESSION"}</span>
                      <span>{selectedSignalRuntime?.runtimeAdapter ?? "--"}</span>
                    </div>
                  </div>
                  {selectedSignalRuntime ? (
                    <>
                      <div className="detail-grid">
                        <div className="detail-item">
                          <span>Session ID</span>
                          <strong>{shrink(selectedSignalRuntime.id)}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Account</span>
                          <strong>{shrink(selectedSignalRuntime.accountId)}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Strategy</span>
                          <strong>{shrink(selectedSignalRuntime.strategyId)}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Transport</span>
                          <strong>{selectedSignalRuntime.transport || "--"}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Health</span>
                          <strong>{String(selectedSignalRuntimeState.health ?? "--")}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Signal Events</span>
                          <strong>{String(Math.trunc(getNumber(selectedSignalRuntimeState.signalEventCount) ?? 0))}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Heartbeat</span>
                          <strong>{formatTime(String(selectedSignalRuntimeState.lastHeartbeatAt ?? ""))}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Last Event</span>
                          <strong>{formatTime(String(selectedSignalRuntimeState.lastEventAt ?? ""))}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Source States</span>
                          <strong>{String(Object.keys(selectedSignalRuntimeSourceStates).length)}</strong>
                        </div>
                        <div className="detail-item">
                          <span>Plan Ready</span>
                          <strong>{boolLabel(selectedSignalRuntimePlan.ready)}</strong>
                        </div>
                      </div>

                      <div className="backtest-breakdown">
                        <h4>Subscriptions</h4>
                        <SimpleTable
                          columns={["Source", "Role", "Symbol", "Channel", "Adapter"]}
                          rows={selectedSignalRuntimeSubscriptions.map((item) => [
                            String(item.sourceKey ?? "--"),
                            String(item.role ?? "--"),
                            String(item.symbol ?? "--"),
                            String(item.channel ?? "--"),
                            String(item.adapterKey ?? "--"),
                          ])}
                          emptyMessage="No subscriptions"
                        />
                      </div>

                      <div className="backtest-breakdown">
                        <h4>Signal Bars</h4>
                        {selectedSignalRuntimeSignalBars.length > 0 ? (
                          <div className="chart-shell">
                            <SignalBarChart candles={selectedSignalRuntimeSignalBars} />
                          </div>
                        ) : (
                          <div className="empty-state empty-state-compact">No 4h/1d signal bars cached yet</div>
                        )}
                      </div>

                      <div className="backtest-breakdown">
                        <h4>Signal States</h4>
                        <div className="backtest-notes">
                          {Object.entries(selectedSignalBarStates).length > 0 ? (
                            Object.entries(selectedSignalBarStates).map(([key, value]) => {
                              const state = getRecord(value);
                              const current = getRecord(state.current);
                              const prevBar1 = getRecord(state.prevBar1);
                              const prevBar2 = getRecord(state.prevBar2);
                              return (
                                <div key={key} className="note-item">
                                  {[
                                    key,
                                    `tf=${String(state.timeframe ?? "--")}`,
                                    `bars=${String(state.barCount ?? "--")}`,
                                    `ma20=${formatMaybeNumber(state.ma20)}`,
                                    `atr14=${formatMaybeNumber(state.atr14)}`,
                                    `t-1=${formatMaybeNumber(prevBar1.open)}/${formatMaybeNumber(prevBar1.high)}/${formatMaybeNumber(prevBar1.low)}/${formatMaybeNumber(prevBar1.close)}`,
                                    `t-2=${formatMaybeNumber(prevBar2.open)}/${formatMaybeNumber(prevBar2.high)}/${formatMaybeNumber(prevBar2.low)}/${formatMaybeNumber(prevBar2.close)}`,
                                    `current=${formatMaybeNumber(current.open)}/${formatMaybeNumber(current.high)}/${formatMaybeNumber(current.low)}/${formatMaybeNumber(current.close)}`,
                                  ].join(" · ")}
                                </div>
                              );
                            })
                          ) : (
                            <div className="empty-state empty-state-compact">No signal states yet</div>
                          )}
                        </div>
                      </div>

                      <div className="backtest-breakdown">
                        <h4>Runtime Timeline</h4>
                        <div className="backtest-notes">
                          {buildTimelineNotes(selectedSignalRuntimeTimeline).map((line: string) => (
                            <div key={line} className="note-item">
                              {line}
                            </div>
                          ))}
                        </div>
                      </div>

                      <div className="backtest-breakdown">
                        <h4>Last Event Summary</h4>
                        <div className="backtest-notes">
                          {Object.entries(selectedSignalRuntimeLastSummary).length > 0 ? (
                            Object.entries(selectedSignalRuntimeLastSummary).map(([key, value]) => (
                              <div key={key} className="note-item">
                                {key}: {typeof value === "object" ? JSON.stringify(value) : String(value)}
                              </div>
                            ))
                          ) : (
                            <div className="empty-state empty-state-compact">No event summary yet</div>
                          )}
                        </div>
                      </div>

                      <div className="backtest-breakdown">
                        <h4>Source States</h4>
                        <div className="backtest-notes">
                          {Object.entries(selectedSignalRuntimeSourceStates).length > 0 ? (
                            Object.entries(selectedSignalRuntimeSourceStates).slice(0, 8).map(([key, value]) => (
                              <div key={key} className="note-item">
                                {key}: {typeof value === "object" ? JSON.stringify(value) : String(value)}
                              </div>
                            ))
                          ) : (
                            <div className="empty-state empty-state-compact">No source states yet</div>
                          )}
                        </div>
                      </div>
                    </>
                  ) : (
                    <div className="empty-state empty-state-compact">No runtime session selected</div>
                  )}
                </div>
              </>
            ) : (
              <div className="empty-state empty-state-compact">No runtime sessions</div>
            )}
          </div>
        </div>
      </section>
    </div>
  );
}
