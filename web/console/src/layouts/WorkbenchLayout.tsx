import React from 'react';
import { Activity, Briefcase, Settings, Bell, ListOrdered, Wallet, CreditCard } from 'lucide-react';

export interface WorkbenchLayoutProps {
  sidebarTab: 'monitor' | 'strategy' | 'account';
  onSidebarTabChange: (tab: 'monitor' | 'strategy' | 'account') => void;
  dockTab: 'orders' | 'positions' | 'fills' | 'alerts';
  onDockTabChange: (tab: 'orders' | 'positions' | 'fills' | 'alerts') => void;
  
  headerMetrics: React.ReactNode;
  headerConnection: React.ReactNode;
  
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
      </aside>

      {/* Main Workspace */}
      <div className="flex-1 flex flex-col min-w-0 relative z-10">
        {/* Header (Top) */}
        <header className="h-12 flex-shrink-0 flex items-center justify-between px-4 border-b border-white/5 bg-zinc-900/20 backdrop-blur-md">
          <div className="flex items-center space-x-6 text-sm">
            {headerMetrics}
          </div>
          <div className="flex items-center">
            {headerConnection}
          </div>
        </header>

        {/* Middle Area: Main Stage + Right Side Panel */}
        <div className="flex-1 flex min-h-0 relative">
          {/* Main Stage (Charts, etc.) */}
          <main className="flex-1 relative overflow-hidden bg-zinc-950/50">
            {mainStageContent}
          </main>
          
          {/* Side Panel (Slides in when strategy/account is selected, optional) */}
          {sidebarTab !== 'monitor' && sidePanelContent && (
            <section className="w-96 border-l border-white/5 bg-zinc-900/60 backdrop-blur-xl overflow-y-auto">
              {sidePanelContent}
            </section>
          )}
        </div>

        {/* Bottom Dock (Tabs + Content) */}
        {sidebarTab === 'monitor' && (
          <section className="h-64 flex-shrink-0 border-t border-white/5 bg-zinc-900/60 backdrop-blur-xl flex flex-col">
            <div className="h-10 flex items-center px-4 border-b border-white/5 space-x-6 text-xs text-zinc-500">
              <DockTab 
                icon={<ListOrdered size={14} />} 
                label="全部订单" 
                active={dockTab === 'orders'} 
                onClick={() => onDockTabChange('orders')} 
              />
              <DockTab 
                icon={<Wallet size={14} />} 
                label="持仓" 
                active={dockTab === 'positions'} 
                onClick={() => onDockTabChange('positions')} 
              />
              <DockTab 
                icon={<CreditCard size={14} />} 
                label="成交明细" 
                active={dockTab === 'fills'} 
                onClick={() => onDockTabChange('fills')} 
              />
              <DockTab 
                icon={<Bell size={14} />} 
                label="异常告警" 
                active={dockTab === 'alerts'} 
                onClick={() => onDockTabChange('alerts')} 
              />
            </div>
            <div className="flex-1 overflow-y-auto p-2">
              {dockContent}
            </div>
          </section>
        )}
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

function DockTab({ icon, label, active, onClick }: { icon: React.ReactNode, label: string, active: boolean, onClick: () => void }) {
  return (
    <button 
      className={`flex items-center space-x-1.5 h-full border-b-2 transition-colors ${
        active 
          ? 'border-emerald-400 text-zinc-200' 
          : 'border-transparent hover:text-zinc-300'
      }`}
      onClick={onClick}
    >
      {icon}
      <span>{label}</span>
    </button>
  );
}
