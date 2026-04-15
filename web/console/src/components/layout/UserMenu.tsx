import React, { useRef } from 'react';
import { LogOut, ChevronDown } from 'lucide-react';
import { useUIStore } from '../../store/useUIStore';
import { useClickOutside } from '../../hooks/useClickOutside';
import { formatTime } from '../../utils/format';

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
    return <div className="text-zinc-500 text-xs">需要登录</div>;
  }

  return (
    <div className="relative" ref={userMenuRef}>
      <button
        type="button"
        className="flex items-center space-x-2 px-3 py-1.5 rounded-xl hover:bg-white/10 transition-colors text-zinc-200"
        onClick={() => setSettingsMenuOpen(!settingsMenuOpen)}
      >
        <div className="w-6 h-6 rounded-lg bg-emerald-500/20 flex items-center justify-center text-emerald-400 font-bold uppercase text-[10px]">
          {authSession.username.slice(0, 2)}
        </div>
        <span className="text-sm font-medium">{authSession.username}</span>
        <ChevronDown size={14} className={`text-zinc-500 transition-transform ${settingsMenuOpen ? 'rotate-180' : ''}`} />
      </button>

      {settingsMenuOpen && (
        <div className="absolute right-0 top-full mt-2 w-56 p-2 rounded-2xl border border-white/10 bg-zinc-950/80 backdrop-blur-2xl shadow-2xl z-50">
          <div className="px-3 py-2 border-b border-white/5 mb-2">
            <p className="text-[10px] text-zinc-500 uppercase tracking-wider mb-1">身份与会话</p>
            <p className="text-xs text-zinc-200 font-medium truncate">{authSession.username}</p>
            <p className="text-[10px] text-zinc-500 mt-1 italic">
              {authSession.expiresAt ? `有效期至 ${formatTime(authSession.expiresAt)}` : "已登录"}
            </p>
          </div>
          
          <div className="space-y-1">
            <button
              className="w-full text-left px-3 py-2 text-xs text-emerald-300 hover:bg-emerald-500/10 rounded-lg transition-colors"
              onClick={() => { setSidebarTab('monitor'); setSettingsMenuOpen(false); }}
            >
              打开监控台
            </button>
            <button
              className="w-full text-left px-3 py-2 text-xs text-zinc-300 hover:bg-white/5 rounded-lg transition-colors"
              onClick={() => { actions.openLiveAccountModal(); setSettingsMenuOpen(false); }}
            >
              新建账户
            </button>
            <button
              className="w-full text-left px-3 py-2 text-xs text-zinc-300 hover:bg-white/5 rounded-lg transition-colors"
              onClick={() => { 
                actions.openLiveBindingModal(quickLiveAccountId); 
                setSettingsMenuOpen(false); 
              }}
            >
              绑定账户
            </button>
            <button
              className="w-full text-left px-3 py-2 text-xs text-zinc-300 hover:bg-white/5 rounded-lg transition-colors"
              onClick={() => { setActiveSettingsModal("telegram"); setSettingsMenuOpen(false); }}
            >
              Telegram 通知
            </button>
          </div>

          <div className="mt-2 pt-2 border-t border-white/5">
            <button
              className="w-full flex items-center px-3 py-2 text-xs text-rose-400 hover:bg-rose-500/10 rounded-lg transition-colors"
              onClick={() => { actions.logout(); setSettingsMenuOpen(false); }}
            >
              <LogOut size={14} className="mr-2" />
              退出登录
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
