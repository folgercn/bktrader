import React from 'react';
import { Activity, Briefcase, Settings, FileText } from 'lucide-react';

export interface WorkbenchLayoutProps {
  sidebarTab: 'monitor' | 'strategy' | 'account' | 'log';
  onSidebarTabChange: (tab: 'monitor' | 'strategy' | 'account' | 'log') => void;
  dockTab: 'pairs' | 'orders' | 'positions' | 'fills' | 'alerts';
  onDockTabChange: (tab: 'pairs' | 'orders' | 'positions' | 'fills' | 'alerts') => void;
  
  headerMetrics: React.ReactNode;
  headerConnection: React.ReactNode;
  headerActions?: React.ReactNode;
  
  mainStageContent: React.ReactNode;
  dockContent: React.ReactNode;
  sidePanelContent?: React.ReactNode; // For strategy or account views when selected
}

export function WorkbenchLayout({
  sidebarTab,
  onSidebarTabChange,
  dockTab,
  onDockTabChange,
  headerMetrics,
  headerConnection,
  headerActions,
  mainStageContent,
  dockContent,
  sidePanelContent
}: WorkbenchLayoutProps) {
  return (
    <div className="flex w-screen h-screen overflow-hidden bg-zinc-950 text-zinc-300 font-sans selection:bg-zinc-800">
      {/* Sidebar (Left) */}
      <aside className="w-16 flex-shrink-0 flex flex-col items-center py-6 border-r border-white/5 bg-zinc-900/40 relative z-20">
        <SidebarItem 
          icon={<Activity size={22} />} 
          label="监控台" 
          active={sidebarTab === 'monitor'} 
          onClick={() => onSidebarTabChange('monitor')} 
        />
        <SidebarItem 
          icon={<Briefcase size={22} />} 
          label="策略面板" 
          active={sidebarTab === 'strategy'} 
          onClick={() => onSidebarTabChange('strategy')} 
        />
        <SidebarItem 
          icon={<Settings size={22} />} 
          label="账户配置" 
          active={sidebarTab === 'account'} 
          onClick={() => onSidebarTabChange('account')} 
        />
        <SidebarItem 
          icon={<FileText size={22} />} 
          label="日志查看台" 
          active={sidebarTab === 'log'} 
          onClick={() => onSidebarTabChange('log')} 
        />
      </aside>

      {/* Main Workspace */}
      <div className="flex-1 flex flex-col min-w-0 relative z-10">
        {/* Header (Top) */}
        <header className="h-12 flex-shrink-0 flex items-center justify-between px-4 border-b border-white/5 bg-zinc-900/40 backdrop-blur-md relative z-30">
          <div className="flex items-center space-x-6 text-sm">
            {headerMetrics}
          </div>
          <div className="flex items-center space-x-4">
            {headerConnection}
            {headerActions && (
              <div className="flex items-center pl-4 border-l border-white/10 relative">
                {headerActions}
              </div>
            )}
          </div>
        </header>

        {/* Middle Area: Main Stage + Right Side Panel */}
        <div className="flex flex-row w-full min-h-0 relative flex-1">
          {/* Main Stage (Charts, etc.) */}
          <main className="flex-1 min-h-0 relative overflow-hidden bg-zinc-950/50">
            {mainStageContent}
          </main>
          
          {/* Side Panel (Slides in when strategy/account is selected, optional) */}
          {sidebarTab !== 'monitor' && sidePanelContent && (
            <section className="w-96 border-l border-white/5 bg-zinc-900/60 backdrop-blur-xl overflow-y-auto">
              {sidePanelContent}
            </section>
          )}
        </div>

      </div>
    </div>
  );
}

function SidebarItem({ icon, label, active, onClick }: { icon: React.ReactNode, label: string, active: boolean, onClick: () => void }) {
  return (
    <button 
      className={`group relative flex items-center justify-center w-12 h-12 rounded-2xl mb-2 transition-all duration-200 ${
        active 
          ? 'bg-emerald-500/10 text-emerald-400 shadow-[inset_0_1px_1px_rgba(255,255,255,0.1)]' 
          : 'text-zinc-500 hover:text-zinc-300 hover:bg-white/5'
      }`}
      onClick={onClick}
    >
      {icon}
      {/* Tooltip */}
      <div className="absolute left-14 px-3 py-1.5 bg-zinc-800 text-zinc-200 text-xs rounded-lg shadow-xl opacity-0 pointer-events-none group-hover:opacity-100 transition-opacity whitespace-nowrap z-50 border border-white/10">
        {label}
      </div>
    </button>
  );
}
