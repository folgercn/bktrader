import React from "react";

import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { AuthSession, LoginForm } from "../types/domain";
import {
  ModalActions,
  ModalField,
  ModalFormGrid,
  ModalMetaItem,
  ModalMetaStrip,
  ModalNotice,
  SettingsModalFrame,
} from "./modal-frame";

interface LoginModalProps {
  authSession: AuthSession | null;
  error: string | null;
  loginForm: LoginForm;
  loginAction: boolean;
  setLoginForm: (valOrUpdater: LoginForm | ((prev: LoginForm) => LoginForm)) => void;
  login: () => void;
}

export function LoginModal({
  authSession,
  error,
  loginForm,
  loginAction,
  setLoginForm,
  login,
}: LoginModalProps) {
  return (
    <SettingsModalFrame
      open={!authSession}
      onOpenChange={() => {}}
      kicker="Authentication"
      title="登录平台 API"
      description="当前控制台需要 Bearer token 才能加载账户、持仓和交易监控。"
      className="max-w-[min(560px,calc(100vw-2rem))]"
      showClose={false}
    >
      {error ? <ModalNotice tone="error">{error}</ModalNotice> : null}

      <ModalMetaStrip>
        <ModalMetaItem label="Mode" value="Protected Session" />
        <ModalMetaItem label="Access" value="Bearer Token Required" />
      </ModalMetaStrip>

      <ModalFormGrid>
        <ModalField label="Username">
          <Input
            value={loginForm.username}
            onChange={(event) => setLoginForm((current) => ({ ...current, username: event.target.value }))}
            placeholder="admin"
            className="h-10 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-3"
          />
        </ModalField>
        <ModalField label="Password">
          <Input
            type="password"
            value={loginForm.password}
            onChange={(event) => setLoginForm((current) => ({ ...current, password: event.target.value }))}
            placeholder="change-this-password"
            className="h-10 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-3"
          />
        </ModalField>
      </ModalFormGrid>

      <ModalActions>
        <Button
          variant="bento"
          className="h-10 rounded-xl px-5 font-black"
          disabled={loginAction || !loginForm.username.trim() || !loginForm.password}
          onClick={login}
        >
          {loginAction ? "登录中..." : "登录"}
        </Button>
      </ModalActions>
    </SettingsModalFrame>
  );
}
