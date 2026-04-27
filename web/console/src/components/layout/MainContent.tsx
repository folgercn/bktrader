import React from 'react';
import { MonitorStage } from '../../pages/MonitorStage';
import { StrategyStage } from '../../pages/StrategyStage';
import { AccountStage } from '../../pages/AccountStage';
import { LogStage } from '../../pages/LogStage';
import { RecoveryStage } from '../../pages/RecoveryStage';
import { useUIStore } from '../../store/useUIStore';

interface MainContentProps {
  actions: any;
  dockContent: React.ReactNode;
  strategies: any[];
  quickLiveAccountId: string;
}

export function MainContent({ actions, dockContent, strategies, quickLiveAccountId }: MainContentProps) {
  const sidebarTab = useUIStore(s => s.sidebarTab);
  const setSidebarTab = useUIStore(s => s.setSidebarTab);
  const dockTab = useUIStore(s => s.dockTab);
  const setDockTab = useUIStore(s => s.setDockTab);

  return (
    <div className="h-full relative overflow-hidden">
      {sidebarTab === 'monitor' && (
        <MonitorStage
          syncLiveOrder={actions.syncLiveOrder}
          dockTab={dockTab}
          onDockTabChange={setDockTab}
          dockContent={dockContent}
        />
      )}
      {sidebarTab === 'strategy' && (
        <StrategyStage 
          createStrategy={actions.createStrategy} 
          saveStrategyParameters={actions.saveStrategyParameters} 
          createBacktestRun={actions.createBacktestRun}
        />
      )}
      {sidebarTab === 'account' && (
        <AccountStage 
          logout={actions.logout}
          openLiveAccountModal={actions.openLiveAccountModal}
          openLiveBindingModal={() => actions.openLiveBindingModal(quickLiveAccountId)}
          openLiveSessionModal={(s) => actions.openLiveSessionModal(s ?? null, quickLiveAccountId, strategies)}
          openMonitorStage={() => setSidebarTab('monitor')}
          launchLiveFlow={actions.launchLiveFlow}
          stopLiveFlow={actions.stopLiveFlow}
          runLiveSessionAction={actions.runLiveSessionAction}
          dispatchLiveSessionIntent={actions.dispatchLiveSessionIntent}
          syncLiveSession={actions.syncLiveSession}
          deleteLiveSession={actions.deleteLiveSession}
          syncLiveAccount={actions.syncLiveAccount}
          jumpToSignalRuntimeSession={actions.jumpToSignalRuntimeSession}
          runLiveNextAction={actions.runLiveNextAction}
          selectQuickLiveAccount={actions.selectQuickLiveAccount}
          updateRuntimePolicy={actions.updateRuntimePolicy}
          createSignalRuntimeSession={actions.createSignalRuntimeSession}
          deleteSignalRuntimeSession={(id) => actions.deleteSignalRuntimeSession(id, null)}
          runSignalRuntimeAction={actions.runSignalRuntimeAction}
          unbindLiveAccount={actions.unbindLiveAccount}
          executeLaunchTemplate={actions.executeLaunchTemplate}
        />
      )}
      {sidebarTab === 'log' && (
        <LogStage />
      )}
      {sidebarTab === 'recovery' && (
        <RecoveryStage />
      )}
    </div>
  );
}
