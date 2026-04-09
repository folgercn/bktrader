import React from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { ActionButton } from '../components/ui/ActionButton';
import { SimpleTable } from '../components/ui/SimpleTable';
import { SampleCard } from '../components/ui/SampleCard';
import { API_BASE } from '../utils/api';
import { formatTime, formatPercent, formatSigned, formatMaybeNumber } from '../utils/format';
import { strategyLabel, getNumber, buildSampleKey, buildSampleRange } from '../utils/derivation';
import { ReplayReasonStats, ExecutionTrade, ReplaySample } from '../types/domain';

interface StrategySidePanelProps {
  createBacktestRun: () => Promise<void>;
}

export function StrategySidePanel({ createBacktestRun }: StrategySidePanelProps) {
  const backtestForm = useUIStore(s => s.backtestForm);
  const setBacktestForm = useUIStore(s => s.setBacktestForm);
  const backtestAction = useUIStore(s => s.backtestAction);
  const backtestOptions = useTradingStore(s => s.backtestOptions);
  const strategies = useTradingStore(s => s.strategies);
  const backtests = useTradingStore(s => s.backtests);
  const selectedBacktestId = useUIStore(s => s.selectedBacktestId);
  const setSelectedBacktestId = useUIStore(s => s.setSelectedBacktestId);
  const chartOverrideRange = useUIStore(s => s.chartOverrideRange);
  const setChartOverrideRange = useUIStore(s => s.setChartOverrideRange);
  const setFocusNonce = useUIStore(s => s.setFocusNonce);
  const selectedSample = useUIStore(s => s.selectedSample);
  const setSelectedSample = useUIStore(s => s.setSelectedSample);
  const setSourceFilter = useUIStore(s => s.setSourceFilter);
  const setEventFilter = useUIStore(s => s.setEventFilter);

  // Derived states
  const selectedExecutionAvailability = backtestOptions?.availability?.[backtestForm.executionDataSource] ?? "unknown";
  const selectedExecutionDatasets = backtestOptions?.datasets?.[backtestForm.executionDataSource] ?? [];
  const selectedExecutionSymbols = backtestOptions?.supportedSymbols?.[backtestForm.executionDataSource] ?? [];
  const selectedExecutionSchema = backtestOptions?.schema?.[backtestForm.executionDataSource];
  const selectedSymbolAvailable =
    selectedExecutionSymbols.length === 0 || selectedExecutionSymbols.includes(backtestForm.symbol.trim().toUpperCase());
  const backtestItems = backtests.slice().reverse().slice(0, 8);
  const selectedBacktest =
    backtests.find((item) => item.id === selectedBacktestId) ??
    (backtests.length > 0 ? backtests[backtests.length - 1] : null);
  const latestBacktestSummary = (selectedBacktest?.resultSummary ?? {}) as Record<string, unknown>;
  const latestExecutionSource = String(latestBacktestSummary.executionDataSource ?? selectedBacktest?.parameters?.executionDataSource ?? "");
  const previewCountLabel = latestExecutionSource === "tick" ? "Preview Ticks" : "Preview Bars";
  const processedCountLabel = latestExecutionSource === "tick" ? "Processed Ticks" : "Processed Bars";
  const processedCountValue =
    latestExecutionSource === "tick"
      ? String(latestBacktestSummary.processedTicks ?? "--")
      : String(latestBacktestSummary.processedBars ?? "--");
  const latestReplayByReason = (latestBacktestSummary.replayLedgerByReason ?? {}) as ReplayReasonStats;
  const latestExecutionTrades = Array.isArray(latestBacktestSummary.executionTrades)
    ? (latestBacktestSummary.executionTrades as ExecutionTrade[])
    : [];
  const latestReplaySkippedSamples = Array.isArray(latestBacktestSummary.replayLedgerSkippedSamples)
    ? (latestBacktestSummary.replayLedgerSkippedSamples as ReplaySample[])
    : [];
  const latestReplayCompletedSamples = Array.isArray(latestBacktestSummary.replayLedgerCompletedSamples)
    ? (latestBacktestSummary.replayLedgerCompletedSamples as ReplaySample[])
    : [];

  return (
    <div className="p-4 space-y-6">
      <section id="backtests" className="panel panel-backtests">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">Backtests</p>
            <h3>回测配置与运行记录</h3>
          </div>
          <div className="range-box">
            <span>{backtests.length} runs</span>
            <span>{strategies.length} strategies</span>
          </div>
        </div>
        <div className="backtest-layout">
          <div className="backtest-form">
            <div className="form-grid">
              <label className="form-field">
                <span>Strategy Version</span>
                <select
                  value={backtestForm.strategyVersionId}
                  onChange={(event) => setBacktestForm((current) => ({ ...current, strategyVersionId: event.target.value }))}
                >
                  {strategies.map((strategy) => (
                    <option key={strategy.id} value={strategy.currentVersion?.id ?? ""}>
                      {strategyLabel(strategy)}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>Signal Timeframe</span>
                <select
                  value={backtestForm.signalTimeframe}
                  onChange={(event) => setBacktestForm((current) => ({ ...current, signalTimeframe: event.target.value }))}
                >
                  {(backtestOptions?.signalTimeframes ?? ["4h", "1d"]).map((item) => (
                    <option key={item} value={item}>
                      {item}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>Execution Source</span>
                <select
                  value={backtestForm.executionDataSource}
                  onChange={(event) => setBacktestForm((current) => ({ ...current, executionDataSource: event.target.value }))}
                >
                  {(backtestOptions?.executionDataSources ?? ["tick", "1min"]).map((item) => (
                    <option key={item} value={item}>
                      {item}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>Symbol</span>
                <input
                  value={backtestForm.symbol}
                  onChange={(event) => setBacktestForm((current) => ({ ...current, symbol: event.target.value.toUpperCase() }))}
                  placeholder="BTCUSDT"
                />
              </label>
              <label className="form-field">
                <span>From (RFC3339)</span>
                <input
                  value={backtestForm.from}
                  onChange={(event) => setBacktestForm((current) => ({ ...current, from: event.target.value }))}
                  placeholder="2020-01-01T00:00:00Z"
                />
              </label>
              <label className="form-field">
                <span>To (RFC3339)</span>
                <input
                  value={backtestForm.to}
                  onChange={(event) => setBacktestForm((current) => ({ ...current, to: event.target.value }))}
                  placeholder="2020-01-31T23:59:59Z"
                />
              </label>
            </div>
            <div className="backtest-actions">
              <ActionButton
                label={backtestAction ? "Submitting..." : "Create Backtest"}
                disabled={
                  backtestAction ||
                  backtestForm.strategyVersionId.trim() === "" ||
                  backtestForm.symbol.trim() === "" ||
                  selectedExecutionAvailability === "missing" ||
                  !selectedSymbolAvailable
                }
                onClick={createBacktestRun}
              />
            </div>
            {backtestOptions ? (
              <div className="backtest-notes">
                <div className="note-item">
                  tick: {String(backtestOptions.availability?.tick ?? "unknown")} · dir: {String(backtestOptions.dataDirectories?.tick ?? "--")}
                </div>
                <div className="note-item">
                  1min: {String(backtestOptions.availability?.["1min"] ?? "unknown")} · dir: {String(backtestOptions.dataDirectories?.["1min"] ?? "--")}
                </div>
                <div className="note-item">
                  selected source: {backtestForm.executionDataSource} · {selectedExecutionDatasets.length} dataset file(s)
                </div>
                <div className="note-item">
                  symbols: {selectedExecutionSymbols.length > 0 ? selectedExecutionSymbols.join(", ") : "--"}
                </div>
                <div className="note-item">
                  required columns: {selectedExecutionSchema?.requiredColumns?.join(", ") ?? "--"}
                </div>
                <div className="note-item">
                  file examples: {selectedExecutionSchema?.filenameExamples?.join(", ") ?? "--"}
                </div>
                {!selectedSymbolAvailable ? (
                  <div className="note-item">
                    symbol {backtestForm.symbol.trim().toUpperCase()} is not available for {backtestForm.executionDataSource}
                  </div>
                ) : null}
                {selectedExecutionDatasets.slice(0, 3).map((dataset) => (
                  <div key={dataset.path} className="note-item">
                    {dataset.name} · {dataset.symbol}
                    {dataset.format ? ` · ${dataset.format}` : ""}
                    {dataset.fileCount ? ` · files ${dataset.fileCount}` : ""}
                  </div>
                ))}
                {(backtestOptions.notes ?? []).map((note) => (
                  <div key={note} className="note-item">
                    {note}
                  </div>
                ))}
              </div>
            ) : null}
          </div>
          <div className="backtest-list">
            {backtestItems.length > 0 ? (
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Time</th>
                      <th>Mode</th>
                      <th>Symbol</th>
                      <th>Status</th>
                      <th>Return</th>
                      <th>DD</th>
                    </tr>
                  </thead>
                  <tbody>
                    {backtestItems.map((item) => (
                      <tr
                        key={item.id}
                        className={item.id === selectedBacktest?.id ? "table-row-active" : ""}
                        onClick={() => setSelectedBacktestId(item.id)}
                      >
                        <td>{formatTime(item.createdAt)}</td>
                        <td>{String(item.parameters?.backtestMode ?? "--")}</td>
                        <td>{String(item.parameters?.symbol ?? "--")}</td>
                        <td>{item.status}</td>
                        <td>{formatPercent(item.resultSummary?.return)}</td>
                        <td>{formatPercent(item.resultSummary?.maxDrawdown)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="empty-state">No backtests yet</div>
            )}
            <div className="backtest-detail-card">
              <div className="panel-header">
                <div>
                  <p className="panel-kicker">Strategy Replay</p>
                  <h3>选中回测详情</h3>
                </div>
                <div className="range-box range-box-wrap">
                  <span>{selectedBacktest?.status ?? "NO RUN"}</span>
                  <span>{String(selectedBacktest?.parameters?.backtestMode ?? "--")}</span>
                  <button
                    type="button"
                    className="filter-chip"
                    disabled={!selectedBacktest?.parameters?.from || !selectedBacktest?.parameters?.to}
                    onClick={() => {
                      const from = Date.parse(String(selectedBacktest?.parameters?.from ?? ""));
                      const to = Date.parse(String(selectedBacktest?.parameters?.to ?? ""));
                      if (!Number.isFinite(from) || !Number.isFinite(to)) {
                        return;
                      }
                      setChartOverrideRange({
                        from: Math.floor(from / 1000),
                        to: Math.floor(to / 1000),
                        label: "Backtest Window",
                      });
                      setFocusNonce((value) => value + 1);
                    }}
                  >
                    Use Backtest Window
                  </button>
                  <button
                    type="button"
                    className="filter-chip"
                    disabled={!chartOverrideRange}
                    onClick={() => setChartOverrideRange(null)}
                  >
                    Back To Live Window
                  </button>
                  <a
                    className={`filter-chip ${selectedBacktest ? "" : "filter-chip-disabled"}`}
                    href={selectedBacktest ? `${API_BASE}/api/v1/backtests/${selectedBacktest.id}/execution-trades.csv` : undefined}
                  >
                    Export Trades CSV
                  </a>
                </div>
              </div>
              {selectedBacktest ? (
                <>
                  <div className="detail-grid">
                    <div className="detail-item">
                      <span>Execution Source</span>
                      <strong>{latestExecutionSource || "--"}</strong>
                    </div>
                    <div className="detail-item">
                      <span>Matched Files</span>
                      <strong>{String(latestBacktestSummary.matchedArchiveFiles ?? "--")}</strong>
                    </div>
                    <div className="detail-item">
                      <span>{previewCountLabel}</span>
                      <strong>{String(latestBacktestSummary.streamPreviewTicks ?? "--")}</strong>
                    </div>
                    <div className="detail-item">
                      <span>{processedCountLabel}</span>
                      <strong>{processedCountValue}</strong>
                    </div>
                    <div className="detail-item">
                      <span>Trade Count</span>
                      <strong>{String(latestBacktestSummary.executionTradeCount ?? "--")}</strong>
                    </div>
                    <div className="detail-item">
                      <span>Closed Trades</span>
                      <strong>{String(latestBacktestSummary.executionClosedCount ?? "--")}</strong>
                    </div>
                    <div className="detail-item">
                      <span>Win Rate</span>
                      <strong>{formatPercent(latestBacktestSummary.executionWinRate)}</strong>
                    </div>
                    <div className="detail-item">
                      <span>Total PnL</span>
                      <strong>{formatSigned(getNumber(latestBacktestSummary.executionRealizedPnL))}</strong>
                    </div>
                  </div>

                  <div className="backtest-breakdown">
                    <h4>Execution Trades</h4>
                    {latestExecutionTrades.length > 0 ? (
                      <SimpleTable
                        columns={["Status", "Source", "Side", "Qty", "Entry", "Exit", "Exit Type", "PnL"]}
                        rows={latestExecutionTrades.map((trade) => [
                          String(trade.status ?? "--"),
                          String(trade.source ?? "--"),
                          String(trade.side ?? "--"),
                          formatMaybeNumber(trade.quantity),
                          `${formatMaybeNumber(trade.entryPrice)} @ ${formatTime(String(trade.entryTime ?? ""))}`,
                          `${formatMaybeNumber(trade.exitPrice)} @ ${formatTime(String(trade.exitTime ?? ""))}`,
                          String(trade.exitType ?? "--"),
                          formatSigned(getNumber(trade.realizedPnL)),
                        ])}
                        emptyMessage="No execution trades"
                      />
                    ) : (
                      <div className="empty-state empty-state-compact">No execution trades yet</div>
                    )}
                  </div>

                  {Boolean(latestBacktestSummary.replayLedgerTrades) ? (
                    <>
                      <div className="backtest-breakdown">
                        <h4>Optional Ledger Audit</h4>
                        {Object.keys(latestReplayByReason).length > 0 ? (
                          <SimpleTable
                            columns={["Reason", "Trades", "Completed", "Skipped", "Entry", "Exit"]}
                            rows={Object.entries(latestReplayByReason).map(([reason, stats]) => [
                              reason,
                              String(stats.trades ?? 0),
                              String(stats.completed ?? 0),
                              String(stats.skipped ?? 0),
                              String(stats.skippedEntry ?? 0),
                              String(stats.skippedExit ?? 0),
                            ])}
                            emptyMessage="No grouped replay stats"
                          />
                        ) : (
                          <div className="empty-state empty-state-compact">No optional ledger audit data</div>
                        )}
                      </div>

                      <div className="backtest-samples-grid">
                        <div className="backtest-sample-panel">
                          <h4>Completed Samples</h4>
                          {latestReplayCompletedSamples.length > 0 ? (
                            latestReplayCompletedSamples.map((sample, index) => (
                              <SampleCard
                                key={`completed-${index}`}
                                sample={sample}
                                selected={selectedSample?.key === buildSampleKey("completed", index, sample)}
                                onSelect={() => {
                                  const range = buildSampleRange(sample);
                                  if (!range) {
                                    return;
                                  }
                                  setSelectedSample({ key: buildSampleKey("completed", index, sample), sample });
                                  setChartOverrideRange(range);
                                  setSourceFilter("backtest");
                                  setEventFilter("all");
                                  setFocusNonce((value) => value + 1);
                                }}
                              />
                            ))
                          ) : (
                            <div className="empty-state empty-state-compact">No completed samples</div>
                          )}
                        </div>
                        <div className="backtest-sample-panel">
                          <h4>Skipped Samples</h4>
                          {latestReplaySkippedSamples.length > 0 ? (
                            latestReplaySkippedSamples.map((sample, index) => (
                              <SampleCard
                                key={`skipped-${index}`}
                                sample={sample}
                                selected={selectedSample?.key === buildSampleKey("skipped", index, sample)}
                                onSelect={() => {
                                  const range = buildSampleRange(sample);
                                  if (!range) {
                                    return;
                                  }
                                  setSelectedSample({ key: buildSampleKey("skipped", index, sample), sample });
                                  setChartOverrideRange(range);
                                  setSourceFilter("backtest");
                                  setEventFilter("all");
                                  setFocusNonce((value) => value + 1);
                                }}
                              />
                            ))
                          ) : (
                            <div className="empty-state empty-state-compact">No skipped samples</div>
                          )}
                        </div>
                      </div>
                    </>
                  ) : null}
                </>
              ) : (
                <div className="empty-state empty-state-compact">No backtest detail yet</div>
              )}
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}
