import React from 'react';
import { ActionButton } from '../components/ui/ActionButton';
import { AccountRecord, LiveAdapter, LiveBindingForm, ActiveSettingsModal } from '../types/domain';

interface LiveBindingModalProps {
  activeSettingsModal: ActiveSettingsModal;
  setActiveSettingsModal: (modal: ActiveSettingsModal) => void;
  liveBindingError: string | null;
  liveBindingNotice: string | null;
  liveBindingForm: LiveBindingForm;
  setLiveBindingForm: (valOrUpdater: LiveBindingForm | ((prev: LiveBindingForm) => LiveBindingForm)) => void;
  liveAccounts: AccountRecord[];
  liveAdapters: LiveAdapter[];
  quickLiveAccount: AccountRecord | null;
  liveBindAction: boolean;
  bindLiveAccount: () => void;
}

export function LiveBindingModal({
  activeSettingsModal,
  setActiveSettingsModal,
  liveBindingError,
  liveBindingNotice,
  liveBindingForm,
  setLiveBindingForm,
  liveAccounts,
  liveAdapters,
  quickLiveAccount,
  liveBindAction,
  bindLiveAccount
}: LiveBindingModalProps) {
  if (activeSettingsModal !== "live-binding") return null;

  return (
    <div className="modal-overlay" onClick={() => setActiveSettingsModal(null)}>
      <div className="modal-panel" onClick={(event) => event.stopPropagation()}>
        <div className="panel-header panel-header-tight">
          <div>
            <p className="panel-kicker">Live Binding</p>
            <h3>绑定 Live / Testnet 适配器</h3>
          </div>
          <button type="button" className="hero-menu-button" onClick={() => setActiveSettingsModal(null)}>
            关闭
          </button>
        </div>
        <div className="backtest-form modal-form">
          {liveBindingError ? <div className="modal-error">{liveBindingError}</div> : null}
          {liveBindingNotice ? <div className="modal-success">{liveBindingNotice}</div> : null}
          <div className="form-grid">
            <label className="form-field">
              <span>Live Account</span>
              <select
                value={liveBindingForm.accountId}
                onChange={(event) => setLiveBindingForm((current) => ({ ...current, accountId: event.target.value }))}
              >
                {liveAccounts.map((account) => (
                  <option key={account.id} value={account.id}>
                    {account.name} ({account.status})
                  </option>
                ))}
              </select>
            </label>
            <label className="form-field">
              <span>Adapter</span>
              <select
                value={liveBindingForm.adapterKey}
                onChange={(event) => setLiveBindingForm((current) => ({ ...current, adapterKey: event.target.value }))}
              >
                {liveAdapters.map((adapter) => (
                  <option key={adapter.key} value={adapter.key}>
                    {adapter.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="form-field">
              <span>Position Mode</span>
              <select
                value={liveBindingForm.positionMode}
                onChange={(event) => setLiveBindingForm((current) => ({ ...current, positionMode: event.target.value }))}
              >
                <option value="ONE_WAY">ONE_WAY</option>
                <option value="HEDGE">HEDGE</option>
              </select>
            </label>
            <label className="form-field">
              <span>Margin Mode</span>
              <select
                value={liveBindingForm.marginMode}
                onChange={(event) => setLiveBindingForm((current) => ({ ...current, marginMode: event.target.value }))}
              >
                <option value="CROSSED">CROSSED</option>
                <option value="ISOLATED">ISOLATED</option>
              </select>
            </label>
            <label className="form-field">
              <span>API Key Env</span>
              <input value={liveBindingForm.apiKeyRef} onChange={(event) => setLiveBindingForm((current) => ({ ...current, apiKeyRef: event.target.value }))} />
            </label>
            <label className="form-field">
              <span>API Secret Env</span>
              <input value={liveBindingForm.apiSecretRef} onChange={(event) => setLiveBindingForm((current) => ({ ...current, apiSecretRef: event.target.value }))} />
            </label>
            <label className="form-field form-field-checkbox">
              <span>Sandbox</span>
              <input
                type="checkbox"
                checked={liveBindingForm.sandbox}
                onChange={(event) => setLiveBindingForm((current) => ({ ...current, sandbox: event.target.checked }))}
              />
            </label>
          </div>
          <div className="backtest-notes notes-compact">
            <div className="note-item">sandbox=true 时默认从 `.env` 读取 `BINANCE_TESTNET_API_KEY` / `BINANCE_TESTNET_API_SECRET`。</div>
            <div className="note-item">当前账户绑定状态：{String(quickLiveAccount?.bindings?.live?.adapterKey ?? "--")} · sandbox {String(quickLiveAccount?.bindings?.live?.sandbox ?? "--")}</div>
          </div>
          <div className="backtest-actions inline-actions">
            <ActionButton label={liveBindAction ? "Binding..." : "Bind Live Adapter"} disabled={liveBindAction || !liveBindingForm.accountId} onClick={bindLiveAccount} />
          </div>
        </div>
      </div>
    </div>
  );
}
