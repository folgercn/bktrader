/**
 * 注意：这里是全局路由与容器编排层 (Root Orchestrator)。
 * 请勿在此实现具体的 UI 片段或复杂的业务功能逻辑。
 * 具体的 UI 功能应拆分到 src/components/layout/ 或 src/pages/ 中实现。
 **/

import React, { useMemo } from 'react';
import { WorkbenchLayout } from './layouts/WorkbenchLayout';
import { useUIStore } from './store/useUIStore';
import { useTradingStore } from './store/useTradingStore';
import { useDashboard } from './hooks/useDashboard';
import { useTradingActions } from './hooks/useTradingActions';
import { fetchJSON } from './utils/api';
import { deriveHighlightedLiveSession } from './utils/derivation';

// Layout Components
import { HeaderMetrics } from './components/layout/HeaderMetrics';
import { SystemStatusMenu } from './components/layout/SystemStatusMenu';
import { UserMenu } from './components/layout/UserMenu';
import { DockContent } from './components/layout/DockContent';
import { MainContent } from './components/layout/MainContent';

// Modals
import { LoginModal } from './modals/LoginModal';
import { LiveAccountModal } from './modals/LiveAccountModal';
import { LiveBindingModal } from './modals/LiveBindingModal';
import { LiveSessionModal } from './modals/LiveSessionModal';
import { TelegramModal } from './modals/TelegramModal';
import { ConfirmModal } from './modals/ConfirmModal';

import { Toaster } from "./components/ui/sonner";

import { TooltipProvider } from "./components/ui/tooltip";

export default function App() {
  const { loadDashboard } = useDashboard();
  const actions = useTradingActions(loadDashboard);

  // UI State from Store
  const sidebarTab = useUIStore(s => s.sidebarTab);
  const setSidebarTab = useUIStore(s => s.setSidebarTab);
  const dockTab = useUIStore(s => s.dockTab);
  const setDockTab = useUIStore(s => s.setDockTab);
  const error = useUIStore(s => s.error);
  const authSession = useUIStore(s => s.authSession);
  const activeSettingsModal = useUIStore(s => s.activeSettingsModal);
  const setActiveSettingsModal = useUIStore(s => s.setActiveSettingsModal);
  
  // Form States & Actions from Store
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

  // Trading State from Store
  const accounts = useTradingStore(s => s.accounts);
  const liveSessions = useTradingStore(s => s.liveSessions);
  const strategies = useTradingStore(s => s.strategies);
  const liveAdapters = useTradingStore(s => s.liveAdapters);
  const telegramConfig = useTradingStore(s => s.telegramConfig);
  const runtimePolicy = useTradingStore(s => s.runtimePolicy);
  const editingLiveSessionId = useTradingStore(s => s.editingLiveSessionId);
  const orders = useTradingStore(s => s.orders);
  const fills = useTradingStore(s => s.fills);
  const positions = useTradingStore(s => s.positions);

  // Quick Account Resolution
  const liveAccounts = accounts;
  const quickLiveAccountId = liveSessionForm.accountId || liveBindingForm.accountId || liveAccounts[0]?.id || "";
  const quickLiveAccount = useMemo(() => liveAccounts.find(a => a.id === quickLiveAccountId) || null, [liveAccounts, quickLiveAccountId]);
  const strategyIds = useMemo(() => new Set(strategies.map(s => s.id)), [strategies]);
  const validLiveSessions = useMemo(
    () => liveSessions.filter(s => strategyIds.has(s.strategyId)),
    [liveSessions, strategyIds]
  );
  const strategyOptions = useMemo(() => strategies.map(s => ({ value: s.id, label: s.name })), [strategies]);

  const selectedSignalRuntimeId = useTradingStore(s => s.selectedSignalRuntimeId);
  const monitorSessionId = useMemo(() => {
    if (selectedSignalRuntimeId) {
      const sessionWithRuntime = liveSessions.find(s => 
        s.id === selectedSignalRuntimeId || 
        String(s.state?.signalRuntimeSessionId) === selectedSignalRuntimeId
      );
      if (sessionWithRuntime) {
        const highlighted = deriveHighlightedLiveSession([sessionWithRuntime], orders, fills, positions);
        return highlighted?.session.id || null;
      }
    }
    const highlighted = deriveHighlightedLiveSession(liveSessions, orders, fills, positions);
    return highlighted?.session.id || null;
  }, [liveSessions, orders, fills, positions, selectedSignalRuntimeId]);

  // Compose dynamic content
  const dockContent = <DockContent dockTab={dockTab} actions={actions} sessionId={monitorSessionId} />;
  const mainStageContent = <MainContent actions={actions} dockContent={dockContent} strategies={strategies} quickLiveAccountId={quickLiveAccountId} />;

  return (
    <TooltipProvider>
      <WorkbenchLayout
        sidebarTab={sidebarTab}
        onSidebarTabChange={setSidebarTab}
        dockTab={dockTab}
        onDockTabChange={setDockTab}
        headerMetrics={<HeaderMetrics />}
        headerConnection={<SystemStatusMenu setError={actions.setError} />}
        headerActions={
          <UserMenu 
            actions={actions} 
            setSidebarTab={setSidebarTab} 
            setActiveSettingsModal={setActiveSettingsModal} 
            quickLiveAccountId={quickLiveAccountId} 
          />
        }
        sidePanelContent={null}
        mainStageContent={mainStageContent}
        dockContent={dockContent}
      />

      {/* Global Modals */}
      <ConfirmModal />
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
        runtimePolicy={runtimePolicy}
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
      <Toaster richColors closeButton position="top-right" />
    </TooltipProvider>
  );
}
