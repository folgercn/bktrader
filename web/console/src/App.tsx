import React, { useMemo, useRef } from 'react';
import { LogOut, ChevronDown } from 'lucide-react';
import { WorkbenchLayout } from './layouts/WorkbenchLayout';
import { useUIStore } from './store/useUIStore';
import { useTradingStore } from './store/useTradingStore';
import { useDashboard } from './hooks/useDashboard';
import { useTradingActions } from './hooks/useTradingActions';
import { fetchJSON } from './utils/api';

import { MetricCard } from './components/ui/MetricCard';
import { ActionButton } from './components/ui/ActionButton';
import { SimpleTable } from './components/ui/SimpleTable';
import { StatusPill } from './components/ui/StatusPill';
import { LoginModal } from './modals/LoginModal';
import { LiveAccountModal } from './modals/LiveAccountModal';
import { LiveBindingModal } from './modals/LiveBindingModal';
import { LiveSessionModal } from './modals/LiveSessionModal';
import { TelegramModal } from './modals/TelegramModal';
import { StrategySidePanel } from './pages/StrategySidePanel';
import { MonitorStage } from './pages/MonitorStage';
import { StrategyStage } from './pages/StrategyStage';
import { AccountStage } from './pages/AccountStage';
import { formatTime, formatMaybeNumber, shrink } from './utils/format';
import { 
  deriveHighlightedLiveSession, technicalStatusLabel 
} from './utils/derivation';

export default function App() {
  const { loadDashboard } = useDashboard();
  const actions = useTradingActions(loadDashboard);

  // UI State
  const sidebarTab = useUIStore(s => s.sidebarTab);
  const setSidebarTab = useUIStore(s => s.setSidebarTab);
  const dockTab = useUIStore(s => s.dockTab);
  const setDockTab = useUIStore(s => s.setDockTab);
  const error = useUIStore(s => s.error);
  const authSession = useUIStore(s => s.authSession);
  const settingsMenuOpen = useUIStore(s => s.settingsMenuOpen);
  const setSettingsMenuOpen = useUIStore(s => s.setSettingsMenuOpen);
  const activeSettingsModal = useUIStore(s => s.activeSettingsModal);
  const setActiveSettingsModal = useUIStore(s => s.setActiveSettingsModal);
  
  const loginForm = useUIStore(s => s.loginForm);
  const loginAction = useUIStore(s => s.loginAction);
  const liveAccountForm = useUIStore(s => s.liveAccountForm);
  const liveBindingForm = useUIStore(s => s.liveBindingForm);
  const liveSessionForm = useUIStore(s => s.liveSessionForm);
  const telegramForm = useUIStore(s => s.telegramForm);
  
  const liveAccountError = useUIStore(s => s.liveAccountError);
  const liveBindingError = useUIStore(s => s.liveBindingError);
  const liveSessionError = useUIStore(s => s.liveSessionError);
  const liveAccountNotice = useUIStore(s => s.liveAccountNotice);
  const liveBindingNotice = useUIStore(s => s.liveBindingNotice);
  const liveSessionNotice = useUIStore(s => s.liveSessionNotice);

  const liveCreateAction = useUIStore(s => s.liveCreateAction);
  const liveBindAction = useUIStore(s => s.liveBindAction);
  const liveSessionCreateAction = useUIStore(s => s.liveSessionCreateAction);
  const liveSessionLaunchAction = useUIStore(s => s.liveSessionLaunchAction);
  const liveSessionAction = useUIStore(s => s.liveSessionAction);
  const telegramAction = useUIStore(s => s.telegramAction);

  // Trading State
  const accounts = useTradingStore(s => s.accounts);
  const liveSessions = useTradingStore(s => s.liveSessions);
  const orders = useTradingStore(s => s.orders);
  const fills = useTradingStore(s => s.fills);
  const positions = useTradingStore(s => s.positions);
  const strategies = useTradingStore(s => s.strategies);
  const signalRuntimeSessions = useTradingStore(s => s.signalRuntimeSessions);
  const signalCatalog = useTradingStore(s => s.signalCatalog);
  const liveAdapters = useTradingStore(s => s.liveAdapters);
  const telegramConfig = useTradingStore(s => s.telegramConfig);
  const alerts = useTradingStore(s => s.alerts);
  const editingLiveSessionId = useTradingStore(s => s.editingLiveSessionId);

  const userMenuRef = useRef<HTMLDivElement>(null);

  // Derived State
  const highlightedLiveSession = useMemo(
    () => deriveHighlightedLiveSession(liveSessions, orders, fills, positions),
    [liveSessions, orders, fills, positions]
  );
  
  const monitorMode = highlightedLiveSession?.session ? "LIVE" : "--";
  const liveAccounts = accounts;
  const quickLiveAccountId = liveSessionForm.accountId || liveBindingForm.accountId || liveAccounts[0]?.id || "";
  const quickLiveAccount = useMemo(() => liveAccounts.find(a => a.id === quickLiveAccountId) || null, [liveAccounts, quickLiveAccountId]);

  const strategyIds = useMemo(() => new Set(strategies.map((item) => item.id)), [strategies]);
  const validLiveSessions = useMemo(
    () => liveSessions.filter((item) => strategyIds.has(item.strategyId)),
    [liveSessions, strategyIds]
  );

  const strategyOptions = useMemo(() => strategies.map(s => ({ value: s.id, label: s.name })), [strategies]);

  const mainStageContent = (
    <div className="h-full relative overflow-hidden">
      {sidebarTab === 'monitor' && <MonitorStage />}
      {sidebarTab === 'strategy' && <StrategyStage createStrategy={actions.createStrategy} saveStrategyParameters={actions.saveStrategyParameters} />}
      {sidebarTab === 'account' && (
        <AccountStage 
          logout={actions.logout}
          openLiveAccountModal={actions.openLiveAccountModal}
          openLiveBindingModal={() => actions.openLiveBindingModal(quickLiveAccountId)}
          openLiveSessionModal={(s) => actions.openLiveSessionModal(s ?? null, quickLiveAccountId, strategies)}
          launchLiveFlow={actions.launchLiveFlow}
          runLiveSessionAction={actions.runLiveSessionAction}
          dispatchLiveSessionIntent={actions.dispatchLiveSessionIntent}
          syncLiveSession={actions.syncLiveSession}
          deleteLiveSession={actions.deleteLiveSession}
          syncLiveAccount={actions.syncLiveAccount}
          syncLiveOrder={actions.syncLiveOrder}
          jumpToSignalRuntimeSession={actions.jumpToSignalRuntimeSession}
          runLiveNextAction={actions.runLiveNextAction}
          selectQuickLiveAccount={actions.selectQuickLiveAccount}
          bindAccountSignalSource={actions.bindAccountSignalSource}
          unbindAccountSignalSource={actions.unbindAccountSignalSource}
          bindStrategySignalSource={actions.bindStrategySignalSource}
          unbindStrategySignalSource={actions.unbindStrategySignalSource}
          updateRuntimePolicy={actions.updateRuntimePolicy}
          createSignalRuntimeSession={actions.createSignalRuntimeSession}
          deleteSignalRuntimeSession={(id) => actions.deleteSignalRuntimeSession(id, null)}
          runSignalRuntimeAction={actions.runSignalRuntimeAction}
        />
      )}
    </div>
  );

  const dockContent = (
    <div className="h-full relative overflow-hidden">
      {dockTab === 'orders' && (
        <SimpleTable
          columns={["ID", "策略版本", "Symbol", "Side", "Type", "数量", "价格", "状态", "创建时间", "操作"]}
          rows={orders.map((order) => [
            shrink(order.id),
            shrink(String(order.metadata?.strategyVersionId ?? order.metadata?.source ?? "--")),
            order.symbol,
            <StatusPill key={`${order.id}-side`} tone={order.side === "buy" ? "ready" : "neutral"}>{order.side}</StatusPill>,
            order.type,
            formatMaybeNumber(order.quantity),
            formatMaybeNumber(order.price),
            technicalStatusLabel(order.status),
            formatTime(order.createdAt),
            <div key={`${order.id}-actions`} className="inline-actions">
              <ActionButton label="Sync" variant="ghost" onClick={() => actions.syncLiveOrder(order.id)} />
            </div>,
          ])}
          emptyMessage="暂无订单"
        />
      )}
      {dockTab === 'positions' && (
        <SimpleTable
          columns={["ID", "账户", "Symbol", "Side", "仓位大小", "开仓价", "标记价", "更新时间"]}
          rows={positions.map((pos) => [
            shrink(pos.id),
            shrink(pos.accountId),
            pos.symbol,
            <StatusPill key={`${pos.id}-side`} tone={pos.side === "long" ? "ready" : "neutral"}>{pos.side}</StatusPill>,
            formatMaybeNumber(pos.quantity),
            formatMaybeNumber(pos.entryPrice),
            formatMaybeNumber(pos.markPrice),
            formatTime(pos.updatedAt),
          ])}
          emptyMessage="暂无持仓"
        />
      )}
      {dockTab === 'fills' && (
        <SimpleTable
          columns={["ID", "订单ID", "成交量", "成交价", "费用", "时间"]}
          rows={fills.map((fill) => [
            shrink(fill.id),
            shrink(fill.orderId),
            formatMaybeNumber(fill.quantity),
            formatMaybeNumber(fill.price),
            formatMaybeNumber(fill.fee),
            formatTime(fill.createdAt),
          ])}
          emptyMessage="暂无成交记录"
        />
      )}
      {dockTab === 'alerts' && (
        <SimpleTable
          columns={["时间", "级别", "模块", "消息"]}
          rows={alerts.map((alert) => [
            formatTime(alert.eventTime ?? ""),
            <StatusPill key={`${alert.id}-level`} tone={alert.level === "critical" ? "blocked" : alert.level === "warning" ? "watch" : "neutral"}>
              {alert.level}
            </StatusPill>,
            alert.title,
            alert.detail,
          ])}
          emptyMessage="暂无告警信息"
        />
      )}
    </div>
  );

  return (
    <>
      <WorkbenchLayout
        sidebarTab={sidebarTab}
        onSidebarTabChange={setSidebarTab}
        dockTab={dockTab}
        onDockTabChange={setDockTab}
        headerMetrics={
          <div className="flex space-x-2">
            <MetricCard label="账户" value={monitorMode} />
            <MetricCard label="策略" value={String(highlightedLiveSession?.session?.strategyId ?? "--")} />
            <MetricCard label="实盘会话" value={String(validLiveSessions.length)} />
            <MetricCard label="运行时会话" value={String(signalRuntimeSessions.length)} />
            <MetricCard label="可用信号源" value={String(signalCatalog?.sources?.length ?? 0)} />
            <MetricCard label="实盘状态" value={highlightedLiveSession?.health.status ?? "--"} tone={highlightedLiveSession?.health.status === "ready" ? "accent" : undefined} />
          </div>
        }
        headerConnection={
          <div 
            className={`flex items-center space-x-2 ${error ? 'cursor-pointer hover:bg-white/5 px-2 py-1 rounded transition-colors' : ''}`}
            onClick={() => { if (error) actions.setError(null); }}
            title={error || undefined}
          >
            <span className={!authSession?.token || error ? "w-2 h-2 rounded-full bg-rose-500" : "w-2 h-2 rounded-full bg-emerald-500"} />
            <span className="text-zinc-400 text-xs truncate max-w-[200px]">
              {!authSession?.token ? "需要登录" : error ? `连接异常: ${error}` : "运行正常"}
            </span>
          </div>
        }
        headerActions={
          authSession ? (
            <div className="relative" ref={userMenuRef}>
              <button
                type="button"
                className="flex items-center space-x-2 px-3 py-1.5 rounded-xl hover:bg-white/10 transition-colors text-zinc-200"
                onClick={() => setSettingsMenuOpen((current) => !current)}
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
          ) : (
            <div className="text-zinc-500 text-xs">需要登录</div>
          )
        }
        sidePanelContent={
          sidebarTab === 'strategy' ? (
            <StrategySidePanel createBacktestRun={actions.createBacktestRun} />
          ) : null
        }
        mainStageContent={mainStageContent}
        dockContent={dockContent}
      />

      {/* Global Modals */}
      <LoginModal 
        authSession={authSession} 
        error={error}
        loginForm={loginForm}
        loginAction={loginAction}
        setLoginForm={actions.setLoginForm}
        login={actions.login}
      />
      <LiveAccountModal 
        activeSettingsModal={activeSettingsModal} 
        setActiveSettingsModal={setActiveSettingsModal}
        quickLiveAccount={quickLiveAccount}
        liveAccounts={liveAccounts}
        quickLiveAccountId={quickLiveAccountId}
        selectQuickLiveAccount={actions.selectQuickLiveAccount}
        liveAccountError={liveAccountError}
        liveAccountNotice={liveAccountNotice}
        liveAccountForm={liveAccountForm}
        setLiveAccountForm={actions.setLiveAccountForm}
        liveCreateAction={liveCreateAction}
        createLiveAccount={actions.createLiveAccount}
        openLiveBindingModal={() => actions.openLiveBindingModal(quickLiveAccountId)}
      />
      <LiveBindingModal 
        activeSettingsModal={activeSettingsModal} 
        setActiveSettingsModal={setActiveSettingsModal}
        liveBindingError={liveBindingError}
        liveBindingNotice={liveBindingNotice}
        liveBindingForm={liveBindingForm}
        setLiveBindingForm={actions.setLiveBindingForm}
        liveAccounts={liveAccounts}
        liveAdapters={liveAdapters}
        quickLiveAccount={quickLiveAccount}
        liveBindAction={liveBindAction}
        bindLiveAccount={actions.bindLiveAccount}
      />
      <LiveSessionModal 
        activeSettingsModal={activeSettingsModal} 
        setActiveSettingsModal={setActiveSettingsModal}
        liveSessionError={liveSessionError}
        liveSessionNotice={liveSessionNotice}
        liveAccounts={liveAccounts}
        liveSessionForm={liveSessionForm}
        setLiveSessionForm={actions.setLiveSessionForm}
        strategies={strategies}
        validLiveSessions={validLiveSessions}
        editingLiveSessionId={editingLiveSessionId}
        strategyOptions={strategyOptions}
        liveSessionCreateAction={liveSessionCreateAction}
        liveSessionLaunchAction={liveSessionLaunchAction}
        liveSessionAction={liveSessionAction}
        saveLiveSession={actions.saveLiveSession}
        createAndStartLiveSession={actions.createAndStartLiveSession}
        setLiveSessionLaunchAction={actions.setLiveSessionLaunchAction}
        setLiveSessionAction={actions.setLiveSessionAction}
        setLiveSessionError={actions.setLiveSessionError}
        loadDashboard={loadDashboard}
        setError={actions.setError}
        fetchJSON={fetchJSON}
      />
      <TelegramModal 
        activeSettingsModal={activeSettingsModal} 
        setActiveSettingsModal={setActiveSettingsModal}
        telegramConfig={telegramConfig}
        telegramForm={telegramForm}
        setTelegramForm={actions.setTelegramForm}
        telegramAction={telegramAction}
        saveTelegramConfig={actions.saveTelegramConfig}
        sendTelegramTest={actions.sendTelegramTest}
      />
    </>
  );
}
