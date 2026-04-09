import React from 'react';
import { ActionButton } from '../components/ui/ActionButton';
import { AccountRecord, StrategyRecord, LiveSession } from '../types/domain';
import { strategyLabel } from '../utils/derivation';

interface LiveSessionModalProps {
  activeSettingsModal: string | null;
  setActiveSettingsModal: (modal: any) => void;
  liveSessionError: string | null;
  liveSessionNotice: string | null;
  liveAccounts: AccountRecord[];
  liveSessionForm: any;
  setLiveSessionForm: (valOrUpdater: any | ((prev: any) => any)) => void;
  strategies: StrategyRecord[];
  validLiveSessions: LiveSession[];
  editingLiveSessionId: string | null;
  strategyOptions: Array<{ value: string; label: string }>;
  liveSessionCreateAction: boolean;
  liveSessionLaunchAction: boolean;
  liveSessionAction: string | null;
  saveLiveSession: () => Promise<LiveSession | null>;
  createAndStartLiveSession: () => Promise<void>;
  setLiveSessionLaunchAction: (val: boolean) => void;
  setLiveSessionAction: (val: string | null) => void;
  setLiveSessionError: (val: string | null) => void;
  loadDashboard: () => Promise<void>;
  setError: (val: string | null) => void;
  fetchJSON: <T>(path: string, init?: RequestInit) => Promise<T>;
}

export function LiveSessionModal({
  activeSettingsModal,
  setActiveSettingsModal,
  liveSessionError,
  liveSessionNotice,
  liveAccounts,
  liveSessionForm,
  setLiveSessionForm,
  strategies,
  validLiveSessions,
  editingLiveSessionId,
  strategyOptions,
  liveSessionCreateAction,
  liveSessionLaunchAction,
  liveSessionAction,
  saveLiveSession,
  createAndStartLiveSession,
  setLiveSessionLaunchAction,
  setLiveSessionAction,
  setLiveSessionError,
  loadDashboard,
  setError,
  fetchJSON
}: LiveSessionModalProps) {
  if (activeSettingsModal !== "live-session") return null;

  return (
    <div className="modal-overlay" onClick={() => setActiveSettingsModal(null)}>
      <div className="modal-panel modal-panel-wide" onClick={(event) => event.stopPropagation()}>
        <div className="panel-header panel-header-tight">
          <div>
            <p className="panel-kicker">Live Session</p>
            <h3>创建 Live Session</h3>
          </div>
          <button type="button" className="hero-menu-button" onClick={() => setActiveSettingsModal(null)}>
            关闭
          </button>
        </div>
        <div className="backtest-form modal-form">
          {liveSessionError ? <div className="modal-error">{liveSessionError}</div> : null}
          {liveSessionNotice ? <div className="modal-success">{liveSessionNotice}</div> : null}
          <div className="backtest-notes notes-compact">
            <div className="note-item">当前账户：{liveAccounts.find((account) => account.id === liveSessionForm.accountId)?.name ?? liveSessionForm.accountId ?? "--"}</div>
            <div className="note-item">当前策略：{strategyLabel(strategies.find((strategy) => strategy.id === liveSessionForm.strategyId))}</div>
            <div className="note-item">有效会话：{validLiveSessions.filter((session) => session.accountId === liveSessionForm.accountId).length}</div>
            {editingLiveSessionId ? <div className="note-item">编辑会话：{editingLiveSessionId}</div> : null}
          </div>
          <div className="form-grid">
            <label className="form-field">
              <span>Live Account</span>
              <select
                value={liveSessionForm.accountId}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, accountId: event.target.value }))}
              >
                {liveAccounts.map((account) => (
                  <option key={account.id} value={account.id}>
                    {account.name} ({account.status})
                  </option>
                ))}
              </select>
            </label>
            <label className="form-field">
              <span>Strategy</span>
              <select
                value={liveSessionForm.strategyId}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, strategyId: event.target.value }))}
              >
                {strategyOptions.map((strategy) => (
                  <option key={strategy.value} value={strategy.value}>
                    {strategy.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="form-field">
              <span>Signal TF</span>
              <select
                value={liveSessionForm.signalTimeframe}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, signalTimeframe: event.target.value }))}
              >
                <option value="4h">4h</option>
                <option value="1d">1d</option>
              </select>
            </label>
            <label className="form-field">
              <span>Execution Source</span>
              <select
                value={liveSessionForm.executionDataSource}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, executionDataSource: event.target.value }))}
              >
                <option value="tick">tick</option>
                <option value="1min">1min</option>
              </select>
            </label>
            <label className="form-field">
              <span>Symbol</span>
              <input
                value={liveSessionForm.symbol}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, symbol: event.target.value.toUpperCase() }))}
              />
            </label>
            <label className="form-field">
              <span>Default Qty</span>
              <input
                value={liveSessionForm.defaultOrderQuantity}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, defaultOrderQuantity: event.target.value }))}
              />
            </label>
            <label className="form-field">
              <span>Entry Order</span>
              <select
                value={liveSessionForm.executionEntryOrderType}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, executionEntryOrderType: event.target.value }))}
              >
                <option value="MARKET">MARKET</option>
                <option value="LIMIT">LIMIT</option>
              </select>
            </label>
            <label className="form-field">
              <span>Entry Max Spread</span>
              <input
                value={liveSessionForm.executionEntryMaxSpreadBps}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, executionEntryMaxSpreadBps: event.target.value }))}
              />
            </label>
            <label className="form-field">
              <span>Wide Spread Mode</span>
              <select
                value={liveSessionForm.executionEntryWideSpreadMode}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, executionEntryWideSpreadMode: event.target.value }))}
              >
                <option value="limit-maker">limit-maker</option>
                <option value="">wait</option>
              </select>
            </label>
            <label className="form-field">
              <span>Entry Fallback</span>
              <select
                value={liveSessionForm.executionEntryTimeoutFallbackOrderType}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, executionEntryTimeoutFallbackOrderType: event.target.value }))}
              >
                <option value="MARKET">MARKET</option>
                <option value="LIMIT">LIMIT</option>
                <option value="">disabled</option>
              </select>
            </label>
            <label className="form-field">
              <span>PT Exit Order</span>
              <select
                value={liveSessionForm.executionPTExitOrderType}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, executionPTExitOrderType: event.target.value }))}
              >
                <option value="LIMIT">LIMIT</option>
                <option value="MARKET">MARKET</option>
              </select>
            </label>
            <label className="form-field">
              <span>PT Exit TIF</span>
              <select
                value={liveSessionForm.executionPTExitTimeInForce}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, executionPTExitTimeInForce: event.target.value }))}
              >
                <option value="GTX">GTX</option>
                <option value="GTC">GTC</option>
                <option value="IOC">IOC</option>
              </select>
            </label>
            <label className="form-field checkbox-field">
              <span>PT Exit Post Only</span>
              <input
                type="checkbox"
                checked={liveSessionForm.executionPTExitPostOnly}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, executionPTExitPostOnly: event.target.checked }))}
              />
            </label>
            <label className="form-field">
              <span>PT Exit Fallback</span>
              <select
                value={liveSessionForm.executionPTExitTimeoutFallbackOrderType}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, executionPTExitTimeoutFallbackOrderType: event.target.value }))}
              >
                <option value="MARKET">MARKET</option>
                <option value="LIMIT">LIMIT</option>
                <option value="">disabled</option>
              </select>
            </label>
            <label className="form-field">
              <span>SL Exit Order</span>
              <select
                value={liveSessionForm.executionSLExitOrderType}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, executionSLExitOrderType: event.target.value }))}
              >
                <option value="MARKET">MARKET</option>
                <option value="LIMIT">LIMIT</option>
              </select>
            </label>
            <label className="form-field">
              <span>SL Exit Max Spread</span>
              <input
                value={liveSessionForm.executionSLExitMaxSpreadBps}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, executionSLExitMaxSpreadBps: event.target.value }))}
              />
            </label>
            <label className="form-field">
              <span>Dispatch Mode</span>
              <select
                value={liveSessionForm.dispatchMode}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, dispatchMode: event.target.value }))}
              >
                <option value="manual-review">manual-review</option>
                <option value="auto-dispatch">auto-dispatch</option>
              </select>
            </label>
            <label className="form-field">
              <span>Dispatch Cooldown (s)</span>
              <input
                value={liveSessionForm.dispatchCooldownSeconds}
                onChange={(event) => setLiveSessionForm((current: any) => ({ ...current, dispatchCooldownSeconds: event.target.value }))}
              />
            </label>
          </div>
          <div className="backtest-actions inline-actions">
            <ActionButton
              label={liveSessionCreateAction ? (editingLiveSessionId ? "Saving..." : "Creating...") : editingLiveSessionId ? "Save Live Session" : "Create Live Session"}
              disabled={liveSessionCreateAction || liveSessionLaunchAction || !liveSessionForm.accountId || !liveSessionForm.strategyId}
              onClick={saveLiveSession}
            />
            <ActionButton
              label={liveSessionLaunchAction ? (editingLiveSessionId ? "Saving..." : "Launching...") : editingLiveSessionId ? "Save & Start" : "Create & Start"}
              disabled={
                liveSessionCreateAction ||
                liveSessionLaunchAction ||
                liveSessionAction !== null ||
                !liveSessionForm.accountId ||
                !liveSessionForm.strategyId
              }
              onClick={async () => {
                if (!editingLiveSessionId) {
                  await createAndStartLiveSession();
                  return;
                }
                setLiveSessionLaunchAction(true);
                try {
                  const updated = await saveLiveSession();
                  if (!updated?.id) {
                    return;
                  }
                  setLiveSessionAction(`${updated.id}:start`);
                  await fetchJSON(`/api/v1/live/sessions/${updated.id}/start`, { method: "POST" });
                  await loadDashboard();
                  setError(null);
                } catch (err) {
                  setLiveSessionError(err instanceof Error ? err.message : "Failed to save and start live session");
                } finally {
                  setLiveSessionAction(null);
                  setLiveSessionLaunchAction(false);
                }
              }}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
