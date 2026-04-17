import React, { useMemo, useRef, useState } from "react";

import { useUIStore } from "../../store/useUIStore";
import { useTradingStore } from "../../store/useTradingStore";
import { useClickOutside } from "../../hooks/useClickOutside";
import { formatTime } from "../../utils/format";
import { Button } from "../ui/button";
import { Badge } from "../ui/badge";

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
      <Button
        type="button"
        variant="bento-outline"
        className="h-8 gap-2 rounded-xl px-3 text-xs font-medium"
        onClick={() => setSystemLogOpen((current) => !current)}
        title={error || "打开最近日志"}
      >
        <span
          className={
            !authSession?.token || error
              ? "h-2 w-2 rounded-full bg-[var(--bk-status-danger)]"
              : "h-2 w-2 rounded-full bg-[var(--bk-status-success)]"
          }
        />
        <span className="max-w-[220px] truncate text-xs text-[var(--bk-text-secondary)]">
          {!authSession?.token ? "需要登录" : error ? `连接异常` : "运行正常"}
        </span>
      </Button>

      {systemLogOpen && (
        <div className="absolute right-0 top-full z-50 mt-2 w-[420px] max-w-[80vw] rounded-[28px] border border-[var(--bk-border-strong)] bg-[var(--bk-surface-strong)] p-4 shadow-[var(--bk-shadow-card)]">
          <div className="mb-3 flex items-start justify-between gap-3">
            <div>
              <p className="mb-1 text-[10px] uppercase tracking-wider text-[var(--bk-text-secondary)]">System Logs</p>
              <p className="text-sm font-medium text-[var(--bk-text-primary)]">最近状态与告警</p>
            </div>
            <div className="flex items-center gap-2">
              {error ? (
                <Button type="button" variant="bento-ghost" className="h-7 rounded-lg px-2 text-[11px] text-[var(--bk-status-danger)]" onClick={() => setError(null)}>
                  清除当前错误
                </Button>
              ) : null}
              <Button type="button" variant="bento-ghost" className="h-7 rounded-lg px-2 text-[11px]" onClick={clearSystemLogs}>
                清空记录
              </Button>
            </div>
          </div>

          <div className="max-h-[360px] space-y-3 overflow-y-auto pr-1">
            {error ? (
              <div className="rounded-2xl border border-[var(--bk-status-danger)]/20 bg-[color:color-mix(in_srgb,var(--bk-status-danger)_10%,transparent)] px-3 py-2">
                <div className="text-[11px] font-semibold text-[var(--bk-status-danger)]">当前错误</div>
                <div className="mt-1 text-xs text-[var(--bk-text-primary)]">{error}</div>
              </div>
            ) : (
              <div className="rounded-2xl border border-[var(--bk-status-success)]/20 bg-[color:color-mix(in_srgb,var(--bk-status-success)_10%,transparent)] px-3 py-2">
                <div className="text-[11px] font-semibold text-[var(--bk-status-success)]">当前状态</div>
                <div className="mt-1 text-xs text-[var(--bk-text-primary)]">运行正常</div>
              </div>
            )}

            {systemLogs.length > 0 ? (
              <div className="rounded-2xl border border-[var(--bk-border-soft)] bg-[var(--bk-surface-overlay)] p-2.5">
                <div className="px-1 pb-2 text-[11px] font-semibold text-[var(--bk-text-secondary)]">最近状态记录</div>
                <div className="space-y-2">
                  {systemLogs.map((item) => (
                    <div key={item.id} className="rounded-xl border border-[var(--bk-border-soft)] bg-[var(--bk-surface)] px-3 py-2">
                      <div className="flex items-center justify-between gap-3">
                        <span
                          className={`text-[11px] font-semibold ${item.level === "error" ? "text-[var(--bk-status-danger)]" : "text-[var(--bk-status-success)]"}`}
                        >
                          {item.level === 'error' ? '异常' : '恢复'}
                        </span>
                        <span className="text-[11px] text-[var(--bk-text-muted)]">{formatTime(item.createdAt)}</span>
                      </div>
                      <div className="mt-1 text-xs text-[var(--bk-text-primary)]">{item.message}</div>
                    </div>
                  ))}
                </div>
              </div>
            ) : null}

            <div className="rounded-2xl border border-[var(--bk-border-soft)] bg-[var(--bk-surface-overlay)] p-2.5">
              <div className="px-1 pb-2 text-[11px] font-semibold text-[var(--bk-text-secondary)]">最近告警</div>
              {recentAlerts.length > 0 ? (
                <div className="space-y-2">
                  {recentAlerts.map((alert) => (
                    <div key={alert.id} className="rounded-xl border border-[var(--bk-border-soft)] bg-[var(--bk-surface)] px-3 py-2">
                      <div className="flex items-center justify-between gap-3">
                        <Badge
                          className={
                            alert.level === "critical"
                              ? "border-[var(--bk-status-danger)]/20 bg-[color:color-mix(in_srgb,var(--bk-status-danger)_10%,transparent)] text-[var(--bk-status-danger)]"
                              : alert.level === "warning"
                                ? "border-[var(--bk-status-warning)]/25 bg-[color:color-mix(in_srgb,var(--bk-status-warning)_10%,transparent)] text-[var(--bk-status-warning)]"
                                : "border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] text-[var(--bk-text-secondary)]"
                          }
                        >
                          {alert.level === "critical" ? "严重" : alert.level === "warning" ? "警告" : "信息"}
                        </Badge>
                        <span className="text-[11px] text-[var(--bk-text-muted)]">{formatTime(alert.eventTime ?? "")}</span>
                      </div>
                      <div className="mt-1 text-xs font-medium text-[var(--bk-text-primary)]">{alert.title}</div>
                      <div className="mt-1 text-xs text-[var(--bk-text-muted)]">{alert.detail}</div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="px-2 py-3 text-xs text-[var(--bk-text-secondary)]">最近没有告警</div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
