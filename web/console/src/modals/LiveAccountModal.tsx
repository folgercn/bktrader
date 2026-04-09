import React from 'react';
import { ActionButton } from '../components/ui/ActionButton';
import { AccountRecord, LiveAccountForm, ActiveSettingsModal } from '../types/domain';

interface LiveAccountModalProps {
  activeSettingsModal: ActiveSettingsModal;
  setActiveSettingsModal: (modal: ActiveSettingsModal) => void;
  quickLiveAccount: AccountRecord | null;
  liveAccounts: AccountRecord[];
  quickLiveAccountId: string;
  selectQuickLiveAccount: (id: string) => void;
  liveAccountError: string | null;
  liveAccountNotice: string | null;
  liveAccountForm: LiveAccountForm;
  setLiveAccountForm: (valOrUpdater: LiveAccountForm | ((prev: LiveAccountForm) => LiveAccountForm)) => void;
  liveCreateAction: boolean;
  createLiveAccount: () => void;
  openLiveBindingModal: () => void;
}

export function LiveAccountModal({
  activeSettingsModal,
  setActiveSettingsModal,
  quickLiveAccount,
  liveAccounts,
  quickLiveAccountId,
  selectQuickLiveAccount,
  liveAccountError,
  liveAccountNotice,
  liveAccountForm,
  setLiveAccountForm,
  liveCreateAction,
  createLiveAccount,
  openLiveBindingModal
}: LiveAccountModalProps) {
  if (activeSettingsModal !== "live-account") return null;

  return (
    <div className="modal-overlay" onClick={() => setActiveSettingsModal(null)}>
      <div className="modal-panel" onClick={(event) => event.stopPropagation()}>
        <div className="panel-header panel-header-tight">
          <div>
            <p className="panel-kicker">Live Account</p>
            <h3>新建实盘 / Testnet 账户</h3>
          </div>
          <button type="button" className="hero-menu-button" onClick={() => setActiveSettingsModal(null)}>
            关闭
          </button>
        </div>
        <div className="backtest-form modal-form">
          <div className="backtest-notes notes-compact">
            <div className="note-item">当前选中账户：{quickLiveAccount?.name ?? "--"} · {quickLiveAccount?.status ?? "--"} · {quickLiveAccount?.exchange ?? "--"}</div>
            <div className="note-item">已有账户：{liveAccounts.length > 0 ? liveAccounts.map((item) => item.name).join(" / ") : "暂无账户"}</div>
          </div>
          {liveAccounts.length > 0 ? (
            <div className="form-grid live-account-picker">
              <label className="form-field form-field-wide">
                <span>切换到已有账户</span>
                <select
                  value={quickLiveAccountId}
                  onChange={(event) => selectQuickLiveAccount(event.target.value)}
                >
                  {liveAccounts.map((account) => (
                    <option key={account.id} value={account.id}>
                      {account.name} ({account.status})
                    </option>
                  ))}
                </select>
              </label>
            </div>
          ) : null}
          {liveAccountError ? <div className="modal-error">{liveAccountError}</div> : null}
          {liveAccountNotice ? <div className="modal-success">{liveAccountNotice}</div> : null}
          <div className="form-grid">
            <label className="form-field">
              <span>Name</span>
              <input value={liveAccountForm.name} onChange={(event) => setLiveAccountForm((current) => ({ ...current, name: event.target.value }))} />
            </label>
            <label className="form-field">
              <span>Exchange</span>
              <input value={liveAccountForm.exchange} onChange={(event) => setLiveAccountForm((current) => ({ ...current, exchange: event.target.value }))} />
            </label>
          </div>
          <div className="backtest-notes notes-compact">
            <div className="note-item">默认会自动补一个不冲突的 testnet 名称，避免和已有账户重名。</div>
          </div>
          <div className="backtest-actions inline-actions">
            <ActionButton
              label={liveCreateAction ? "Creating..." : "Create Live Account"}
              disabled={liveCreateAction || !liveAccountForm.name.trim() || !liveAccountForm.exchange.trim()}
              onClick={createLiveAccount}
            />
            <ActionButton
              label="使用当前选中账户去绑定"
              variant="ghost"
              disabled={!quickLiveAccountId}
              onClick={() => {
                if (quickLiveAccountId) {
                  selectQuickLiveAccount(quickLiveAccountId);
                }
                openLiveBindingModal();
              }}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
