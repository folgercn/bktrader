import React, { useRef } from "react";
import { ChevronDown, LogOut } from "lucide-react";

import { useUIStore } from "../../store/useUIStore";
import { useClickOutside } from "../../hooks/useClickOutside";
import { formatTime } from "../../utils/format";
import { Button } from "../ui/button";
import { cn } from "../../lib/utils";

interface UserMenuProps {
  actions: any;
  setSidebarTab: (tab: 'monitor' | 'strategy' | 'account') => void;
  setActiveSettingsModal: (modal: "telegram" | "live-account" | "live-binding" | "live-session" | null) => void;
  quickLiveAccountId: string;
}

export function UserMenu({ actions, setSidebarTab, setActiveSettingsModal, quickLiveAccountId }: UserMenuProps) {
  const authSession = useUIStore(s => s.authSession);
  const settingsMenuOpen = useUIStore(s => s.settingsMenuOpen);
  const setSettingsMenuOpen = useUIStore(s => s.setSettingsMenuOpen);
  const userMenuRef = useRef<HTMLDivElement>(null);

  useClickOutside(userMenuRef, () => {
    if (settingsMenuOpen) setSettingsMenuOpen(false);
  });

  if (!authSession) {
    return <div className="text-xs text-[var(--bk-text-secondary)]">需要登录</div>;
  }

  const menuItemClassName =
    "w-full justify-start rounded-xl px-3 py-2 text-xs font-medium";

  return (
    <div className="relative" ref={userMenuRef}>
      <Button
        type="button"
        variant="bento-outline"
        className="h-9 items-center gap-2 rounded-xl px-3 text-[var(--bk-text-primary)] shadow-sm"
        onClick={() => setSettingsMenuOpen(!settingsMenuOpen)}
      >
        <div className="flex h-6 w-6 items-center justify-center rounded-lg bg-[var(--bk-status-success-soft)] text-[10px] font-bold uppercase text-[var(--bk-status-success)]">
          {authSession.username.slice(0, 2)}
        </div>
        <span className="text-sm font-medium">{authSession.username}</span>
        <ChevronDown
          size={14}
          className={cn(
            "text-[var(--bk-text-secondary)] transition-transform",
            settingsMenuOpen && "rotate-180"
          )}
        />
      </Button>

      {settingsMenuOpen && (
        <div className="absolute right-0 top-full z-50 mt-2 w-64 rounded-[24px] border border-[var(--bk-border-strong)] bg-[var(--bk-surface-strong)] p-2.5 shadow-[var(--bk-shadow-card)]">
          <div className="mb-2 rounded-2xl border border-[var(--bk-border-soft)] bg-[var(--bk-surface-overlay)] px-3 py-3">
            <p className="mb-1 text-[10px] uppercase tracking-wider text-[var(--bk-text-secondary)]">身份与会话</p>
            <p className="truncate text-xs font-medium text-[var(--bk-text-primary)]">{authSession.username}</p>
            <p className="mt-1 text-[10px] italic text-[var(--bk-text-muted)]">
              {authSession.expiresAt ? `有效期至 ${formatTime(authSession.expiresAt)}` : "已登录"}
            </p>
          </div>
          
          <div className="space-y-1">
            <Button
              variant="bento-ghost"
              className={cn(menuItemClassName, "text-[var(--bk-status-success)]")}
              onClick={() => { setSidebarTab('monitor'); setSettingsMenuOpen(false); }}
            >
              打开监控台
            </Button>
            <Button
              variant="bento-ghost"
              className={menuItemClassName}
              onClick={() => { actions.openLiveAccountModal(); setSettingsMenuOpen(false); }}
            >
              新建账户
            </Button>
            <Button
              variant="bento-ghost"
              className={menuItemClassName}
              onClick={() => { 
                actions.openLiveBindingModal(quickLiveAccountId); 
                setSettingsMenuOpen(false); 
              }}
            >
              绑定账户
            </Button>
            <Button
              variant="bento-ghost"
              className={menuItemClassName}
              onClick={() => { setActiveSettingsModal("telegram"); setSettingsMenuOpen(false); }}
            >
              Telegram 通知
            </Button>
          </div>

          <div className="mt-2 border-t border-[var(--bk-border-soft)] pt-2">
            <Button
              variant="bento-ghost"
              className={cn(menuItemClassName, "text-[var(--bk-status-danger)]")}
              onClick={() => { actions.logout(); setSettingsMenuOpen(false); }}
            >
              <LogOut size={14} className="mr-2" />
              退出登录
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
