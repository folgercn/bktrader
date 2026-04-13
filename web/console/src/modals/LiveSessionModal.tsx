import React from 'react';
import { ActionButton } from '../components/ui/ActionButton';
import { AccountRecord, StrategyRecord, LiveSession, LiveSessionForm, ActiveSettingsModal } from '../types/domain';
import { strategyLabel } from '../utils/derivation';

interface LiveSessionModalProps {
  activeSettingsModal: ActiveSettingsModal;
  setActiveSettingsModal: (modal: ActiveSettingsModal) => void;
  liveSessionError: string | null;
  liveSessionNotice: string | null;
  liveAccounts: AccountRecord[];
  liveSessionForm: LiveSessionForm;
  setLiveSessionForm: (valOrUpdater: LiveSessionForm | ((prev: LiveSessionForm) => LiveSessionForm)) => void;
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
            <p className="panel-kicker">实盘会话</p>
            <h3>配置实盘会话</h3>
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
              <span>实盘账户</span>
              <select
                value={liveSessionForm.accountId}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, accountId: event.target.value }))}
              >
                {liveAccounts.map((account) => (
                  <option key={account.id} value={account.id}>
                    {account.name} ({account.status})
                  </option>
                ))}
              </select>
            </label>
            <label className="form-field">
              <span>绑定策略</span>
              <select
                value={liveSessionForm.strategyId}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, strategyId: event.target.value }))}
              >
                {strategyOptions.map((strategy) => (
                  <option key={strategy.value} value={strategy.value}>
                    {strategy.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="form-field">
              <span>信号周期</span>
              <select
                value={liveSessionForm.signalTimeframe}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, signalTimeframe: event.target.value }))}
              >
                <option value="4h">4h</option>
                <option value="1d">1d</option>
              </select>
            </label>
            <label className="form-field">
              <span>执行数据源</span>
              <select
                value={liveSessionForm.executionDataSource}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, executionDataSource: event.target.value }))}
              >
                <option value="tick">tick</option>
                <option value="1min">1min</option>
              </select>
            </label>
            <label className="form-field">
              <span>交易对 (Symbol)</span>
              <input
                value={liveSessionForm.symbol}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, symbol: event.target.value.toUpperCase() }))}
              />
            </label>
            <label className="form-field">
              <span>默认下单量</span>
              <input
                value={liveSessionForm.defaultOrderQuantity}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, defaultOrderQuantity: event.target.value }))}
              />
            </label>
            <label className="form-field">
              <span>进场订单类型</span>
              <select
                value={liveSessionForm.executionEntryOrderType}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, executionEntryOrderType: event.target.value }))}
              >
                <option value="MARKET">MARKET</option>
                <option value="LIMIT">LIMIT</option>
              </select>
            </label>
            <label className="form-field">
              <span>进场最大价差 (bps)</span>
              <input
                value={liveSessionForm.executionEntryMaxSpreadBps}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, executionEntryMaxSpreadBps: event.target.value }))}
              />
            </label>
            <label className="form-field">
              <span>宽价差处理模式</span>
              <select
                value={liveSessionForm.executionEntryWideSpreadMode}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, executionEntryWideSpreadMode: event.target.value }))}
              >
                <option value="limit-maker">limit-maker</option>
                <option value="">wait</option>
              </select>
            </label>
            <label className="form-field">
              <span>进场超时备选</span>
              <select
                value={liveSessionForm.executionEntryTimeoutFallbackOrderType}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, executionEntryTimeoutFallbackOrderType: event.target.value }))}
              >
                <option value="MARKET">MARKET</option>
                <option value="LIMIT">LIMIT</option>
                <option value="">disabled</option>
              </select>
            </label>
            <label className="form-field">
              <span>止盈订单类型</span>
              <select
                value={liveSessionForm.executionPTExitOrderType}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, executionPTExitOrderType: event.target.value }))}
              >
                <option value="LIMIT">LIMIT</option>
                <option value="MARKET">MARKET</option>
              </select>
            </label>
            <label className="form-field">
              <span>止盈 TIF</span>
              <select
                value={liveSessionForm.executionPTExitTimeInForce}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, executionPTExitTimeInForce: event.target.value }))}
              >
                <option value="GTX">GTX</option>
                <option value="GTC">GTC</option>
                <option value="IOC">IOC</option>
              </select>
            </label>
            <label className="form-field checkbox-field">
              <span>止盈只做 Maker</span>
              <input
                type="checkbox"
                checked={liveSessionForm.executionPTExitPostOnly}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, executionPTExitPostOnly: event.target.checked }))}
              />
            </label>
            <label className="form-field">
              <span>止盈超时备选</span>
              <select
                value={liveSessionForm.executionPTExitTimeoutFallbackOrderType}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, executionPTExitTimeoutFallbackOrderType: event.target.value }))}
              >
                <option value="MARKET">MARKET</option>
                <option value="LIMIT">LIMIT</option>
                <option value="">disabled</option>
              </select>
            </label>
            <label className="form-field">
              <span>止损订单类型</span>
              <select
                value={liveSessionForm.executionSLExitOrderType}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, executionSLExitOrderType: event.target.value }))}
              >
                <option value="MARKET">MARKET</option>
                <option value="LIMIT">LIMIT</option>
              </select>
            </label>
            <label className="form-field">
              <span>止损最大价差 (bps)</span>
              <input
                value={liveSessionForm.executionSLExitMaxSpreadBps}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, executionSLExitMaxSpreadBps: event.target.value }))}
              />
            </label>
            <label className="form-field">
              <span>分发模式</span>
              <select
                value={liveSessionForm.dispatchMode}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, dispatchMode: event.target.value }))}
              >
                <option value="manual-review">manual-review</option>
                <option value="auto-dispatch">auto-dispatch</option>
              </select>
            </label>
            <label className="form-field">
              <span>分发冷却 (秒)</span>
              <input
                value={liveSessionForm.dispatchCooldownSeconds}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, dispatchCooldownSeconds: event.target.value }))}
              />
            </label>
          </div>
          <div className="backtest-actions inline-actions">
            <ActionButton
              label={liveSessionCreateAction ? (editingLiveSessionId ? "保存中..." : "创建中...") : editingLiveSessionId ? "保存实盘会话" : "创建实盘会话"}
              disabled={liveSessionCreateAction || liveSessionLaunchAction || !liveSessionForm.accountId || !liveSessionForm.strategyId}
              onClick={saveLiveSession}
            />
            <ActionButton
              label={liveSessionLaunchAction ? (editingLiveSessionId ? "保存中..." : "启动中...") : editingLiveSessionId ? "保存并启动" : "创建并启动"}
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
