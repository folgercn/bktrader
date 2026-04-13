import React from 'react';
import { useTradingStore } from '../store/useTradingStore';

export function AccountSidePanel() {
  const signalCatalog = useTradingStore(s => s.signalCatalog);
  const signalRuntimeSessions = useTradingStore(s => s.signalRuntimeSessions);
  const liveSessions = useTradingStore(s => s.liveSessions);

  return (
    <div className="p-4 space-y-6">
      <section className="panel panel-compact">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">Status Summary</p>
            <h3>运行概览</h3>
          </div>
        </div>
        <div className="backtest-notes">
          <div className="note-item">
            活动实盘会话: <strong>{liveSessions.length}</strong>
          </div>
          <div className="note-item">
            运行时会话: <strong>{signalRuntimeSessions.length}</strong>
          </div>
          <div className="note-item">
            可用信号源: <strong>{signalCatalog?.sources?.length ?? 0}</strong>
          </div>
        </div>
        <div className="p-2">
          <p className="text-[10px] text-zinc-500 leading-relaxed">
            信号源绑定、运行时策略和会话管理的详细操作已移至主界面底部，以提供更宽阔的操作空间。
          </p>
        </div>
      </section>
    </div>
  );
}
