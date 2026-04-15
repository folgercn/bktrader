import React, { useState, useRef, useMemo } from 'react';
import { useUIStore } from '../../store/useUIStore';
import { useTradingStore } from '../../store/useTradingStore';
import { useClickOutside } from '../../hooks/useClickOutside';
import { formatTime } from '../../utils/format';

interface SystemStatusMenuProps {
  setError: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
}

export function SystemStatusMenu({ setError }: SystemStatusMenuProps) {
  const [systemLogOpen, setSystemLogOpen] = useState(false);
  const systemLogRef = useRef<HTMLDivElement>(null);
  
  const authSession = useUIStore(s => s.authSession);
  const error = useUIStore(s => s.error);
  const systemLogs = useUIStore(s => s.systemLogs);
  const clearSystemLogs = useUIStore(s => s.clearSystemLogs);
  const alerts = useTradingStore(s => s.alerts);

  const recentAlerts = useMemo(
    () => [...alerts].sort((left, right) => Date.parse(right.eventTime ?? "") - Date.parse(left.eventTime ?? "")).slice(0, 6),
    [alerts]
  );

  useClickOutside(systemLogRef, () => {
    if (systemLogOpen) setSystemLogOpen(false);
  });

  return (
    <div className="relative" ref={systemLogRef}>
      <button
        type="button"
        className="flex items-center space-x-2 px-2 py-1 rounded transition-colors hover:bg-white/5"
        onClick={() => setSystemLogOpen((current) => !current)}
        title={error || "打开最近日志"}
      >
        <span className={!authSession?.token || error ? "w-2 h-2 rounded-full bg-rose-500" : "w-2 h-2 rounded-full bg-emerald-500"} />
        <span className="text-zinc-400 text-xs truncate max-w-[220px]">
          {!authSession?.token ? "需要登录" : error ? `连接异常` : "运行正常"}
        </span>
      </button>

      {systemLogOpen && (
        <div className="absolute right-0 top-full mt-2 w-[420px] max-w-[80vw] p-3 rounded-2xl border border-white/10 bg-zinc-950/90 backdrop-blur-2xl shadow-2xl z-50">
          <div className="flex items-start justify-between gap-3 mb-3">
            <div>
              <p className="text-[10px] text-zinc-500 uppercase tracking-wider mb-1">System Logs</p>
              <p className="text-sm text-zinc-100 font-medium">最近状态与告警</p>
            </div>
            <div className="flex items-center gap-2">
              {error ? (
                <button
                  type="button"
                  className="px-2 py-1 text-[11px] rounded-lg text-rose-300 hover:bg-rose-500/10 transition-colors"
                  onClick={() => setError(null)}
                >
                  清除当前错误
                </button>
              ) : null}
              <button
                type="button"
                className="px-2 py-1 text-[11px] rounded-lg text-zinc-400 hover:bg-white/5 transition-colors"
                onClick={clearSystemLogs}
              >
                清空记录
              </button>
            </div>
          </div>

          <div className="space-y-2 max-h-[360px] overflow-y-auto pr-1">
            {error ? (
              <div className="rounded-xl border border-rose-500/20 bg-rose-500/10 px-3 py-2">
                <div className="text-[11px] text-rose-300 font-semibold">当前错误</div>
                <div className="text-xs text-zinc-200 mt-1">{error}</div>
              </div>
            ) : (
              <div className="rounded-xl border border-emerald-500/20 bg-emerald-500/10 px-3 py-2">
                <div className="text-[11px] text-emerald-300 font-semibold">当前状态</div>
                <div className="text-xs text-zinc-200 mt-1">运行正常</div>
              </div>
            )}

            {systemLogs.length > 0 ? (
              <div className="rounded-xl border border-white/5 bg-white/5 p-2">
                <div className="text-[11px] text-zinc-400 font-semibold px-1 pb-2">最近状态记录</div>
                <div className="space-y-2">
                  {systemLogs.map((item) => (
                    <div key={item.id} className="px-2 py-2 rounded-lg bg-black/10 border border-white/5">
                      <div className="flex items-center justify-between gap-3">
                        <span className={`text-[11px] font-semibold ${item.level === 'error' ? 'text-rose-300' : 'text-emerald-300'}`}>
                          {item.level === 'error' ? '异常' : '恢复'}
                        </span>
                        <span className="text-[11px] text-zinc-500">{formatTime(item.createdAt)}</span>
                      </div>
                      <div className="text-xs text-zinc-200 mt-1">{item.message}</div>
                    </div>
                  ))}
                </div>
              </div>
            ) : null}

            <div className="rounded-xl border border-white/5 bg-white/5 p-2">
              <div className="text-[11px] text-zinc-400 font-semibold px-1 pb-2">最近告警</div>
              {recentAlerts.length > 0 ? (
                <div className="space-y-2">
                  {recentAlerts.map((alert) => (
                    <div key={alert.id} className="px-2 py-2 rounded-lg bg-black/10 border border-white/5">
                      <div className="flex items-center justify-between gap-3">
                        <span className={`text-[11px] font-semibold ${alert.level === 'critical' ? 'text-rose-300' : alert.level === 'warning' ? 'text-amber-300' : 'text-zinc-300'}`}>
                          {alert.level === 'critical' ? '严重' : alert.level === 'warning' ? '警告' : '信息'}
                        </span>
                        <span className="text-[11px] text-zinc-500">{formatTime(alert.eventTime ?? "")}</span>
                      </div>
                      <div className="text-xs text-zinc-100 mt-1">{alert.title}</div>
                      <div className="text-xs text-zinc-400 mt-1">{alert.detail}</div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="px-2 py-3 text-xs text-zinc-500">最近没有告警</div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
