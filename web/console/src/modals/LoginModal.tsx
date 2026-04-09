import React from 'react';
import { ActionButton } from '../components/ui/ActionButton';
import { AuthSession } from '../types/domain';

interface LoginModalProps {
  authSession: AuthSession | null;
  error: string | null;
  loginForm: any;
  loginAction: boolean;
  setLoginForm: (valOrUpdater: any | ((prev: any) => any)) => void;
  login: () => void;
}

export function LoginModal({
  authSession,
  error,
  loginForm,
  loginAction,
  setLoginForm,
  login
}: LoginModalProps) {
  if (authSession) return null;

  return (
    <div className="modal-overlay">
      <div className="modal-panel" onClick={(event) => event.stopPropagation()}>
        <div className="panel-header panel-header-tight">
          <div>
            <p className="panel-kicker">Authentication</p>
            <h3>登录平台 API</h3>
          </div>
        </div>
        <div className="backtest-form modal-form">
          {error ? <div className="modal-error">{error}</div> : null}
          <div className="form-grid">
            <label className="form-field">
              <span>Username</span>
              <input
                value={loginForm.username}
                onChange={(event) => setLoginForm((current: any) => ({ ...current, username: event.target.value }))}
                placeholder="admin"
              />
            </label>
            <label className="form-field">
              <span>Password</span>
              <input
                type="password"
                value={loginForm.password}
                onChange={(event) => setLoginForm((current: any) => ({ ...current, password: event.target.value }))}
                placeholder="change-this-password"
              />
            </label>
          </div>
          <div className="backtest-notes notes-compact">
            <div className="note-item">当前页面需要 Bearer token 才能加载账户、持仓和交易监控。</div>
          </div>
          <div className="backtest-actions inline-actions">
            <ActionButton
              label={loginAction ? "登录中..." : "登录"}
              disabled={loginAction || !loginForm.username.trim() || !loginForm.password}
              onClick={login}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
