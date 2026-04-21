import { create } from 'zustand';
import { AccountSummary, AccountRecord, Order, Fill, Position, AccountEquitySnapshot, StrategyRecord, BacktestRun, BacktestOptions, PaperSession, LiveSession, LiveAdapter, SignalSourceCatalog, SignalSourceType, SignalRuntimeAdapter, SignalRuntimeSession, RuntimePolicy, PlatformAlert, PlatformNotification, TelegramConfig, SignalBinding, ChartCandle, ChartAnnotation, MarkerDetail, ChartOverrideRange, SelectedSample, SourceFilter, EventFilter, TimeWindow, AuthSession, PlatformHealthSnapshot } from '../types/domain';
import { readStoredAuthSession } from '../utils/auth';
import { resolveUpdater } from './helpers';


export interface useTradingStoreState {
  summaries: AccountSummary[];
  setSummaries: (valOrUpdater: AccountSummary[] | ((prev: AccountSummary[]) => AccountSummary[])) => void;
  accounts: AccountRecord[];
  setAccounts: (valOrUpdater: AccountRecord[] | ((prev: AccountRecord[]) => AccountRecord[])) => void;
  orders: Order[];
  setOrders: (valOrUpdater: Order[] | ((prev: Order[]) => Order[])) => void;
  fills: Fill[];
  setFills: (valOrUpdater: Fill[] | ((prev: Fill[]) => Fill[])) => void;
  positions: Position[];
  setPositions: (valOrUpdater: Position[] | ((prev: Position[]) => Position[])) => void;
  snapshots: AccountEquitySnapshot[];
  setSnapshots: (valOrUpdater: AccountEquitySnapshot[] | ((prev: AccountEquitySnapshot[]) => AccountEquitySnapshot[])) => void;
  strategies: StrategyRecord[];
  setStrategies: (valOrUpdater: StrategyRecord[] | ((prev: StrategyRecord[]) => StrategyRecord[])) => void;
  backtests: BacktestRun[];
  setBacktests: (valOrUpdater: BacktestRun[] | ((prev: BacktestRun[]) => BacktestRun[])) => void;
  backtestOptions: BacktestOptions | null;
  setBacktestOptions: (valOrUpdater: BacktestOptions | null | ((prev: BacktestOptions | null) => BacktestOptions | null)) => void;
  paperSessions: PaperSession[];
  setPaperSessions: (valOrUpdater: PaperSession[] | ((prev: PaperSession[]) => PaperSession[])) => void;
  liveSessions: LiveSession[];
  setLiveSessions: (valOrUpdater: LiveSession[] | ((prev: LiveSession[]) => LiveSession[])) => void;
  liveAdapters: LiveAdapter[];
  setLiveAdapters: (valOrUpdater: LiveAdapter[] | ((prev: LiveAdapter[]) => LiveAdapter[])) => void;
  signalCatalog: SignalSourceCatalog | null;
  setSignalCatalog: (valOrUpdater: SignalSourceCatalog | null | ((prev: SignalSourceCatalog | null) => SignalSourceCatalog | null)) => void;
  signalSourceTypes: SignalSourceType[];
  setSignalSourceTypes: (valOrUpdater: SignalSourceType[] | ((prev: SignalSourceType[]) => SignalSourceType[])) => void;
  signalRuntimeAdapters: SignalRuntimeAdapter[];
  setSignalRuntimeAdapters: (valOrUpdater: SignalRuntimeAdapter[] | ((prev: SignalRuntimeAdapter[]) => SignalRuntimeAdapter[])) => void;
  signalRuntimeSessions: SignalRuntimeSession[];
  setSignalRuntimeSessions: (valOrUpdater: SignalRuntimeSession[] | ((prev: SignalRuntimeSession[]) => SignalRuntimeSession[])) => void;
  runtimePolicy: RuntimePolicy | null;
  setRuntimePolicy: (valOrUpdater: RuntimePolicy | null | ((prev: RuntimePolicy | null) => RuntimePolicy | null)) => void;
  monitorHealth: PlatformHealthSnapshot | null;
  setMonitorHealth: (valOrUpdater: PlatformHealthSnapshot | null | ((prev: PlatformHealthSnapshot | null) => PlatformHealthSnapshot | null)) => void;
  alerts: PlatformAlert[];
  setAlerts: (valOrUpdater: PlatformAlert[] | ((prev: PlatformAlert[]) => PlatformAlert[])) => void;
  notifications: PlatformNotification[];
  setNotifications: (valOrUpdater: PlatformNotification[] | ((prev: PlatformNotification[]) => PlatformNotification[])) => void;
  telegramConfig: TelegramConfig | null;
  setTelegramConfig: (valOrUpdater: TelegramConfig | null | ((prev: TelegramConfig | null) => TelegramConfig | null)) => void;
  strategySignalBindings: SignalBinding[];
  setStrategySignalBindings: (valOrUpdater: SignalBinding[] | ((prev: SignalBinding[]) => SignalBinding[])) => void;
  strategySignalBindingMap: Record<string, SignalBinding[]>;
  setStrategySignalBindingMap: (valOrUpdater: Record<string, SignalBinding[]> | ((prev: Record<string, SignalBinding[]>) => Record<string, SignalBinding[]>)) => void;
  signalRuntimePlan: Record<string, unknown> | null;
  setSignalRuntimePlan: (valOrUpdater: Record<string, unknown> | null | ((prev: Record<string, unknown> | null) => Record<string, unknown> | null)) => void;
  selectedSignalRuntimeId: string | null;
  setSelectedSignalRuntimeId: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  selectedStrategyId: string | null;
  setSelectedStrategyId: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  candles: ChartCandle[];
  setCandles: (valOrUpdater: ChartCandle[] | ((prev: ChartCandle[]) => ChartCandle[])) => void;
  monitorCandles: ChartCandle[];
  setMonitorCandles: (valOrUpdater: ChartCandle[] | ((prev: ChartCandle[]) => ChartCandle[])) => void;
  annotations: ChartAnnotation[];
  setAnnotations: (valOrUpdater: ChartAnnotation[] | ((prev: ChartAnnotation[]) => ChartAnnotation[])) => void;
  editingLiveSessionId: string | null;
  setEditingLiveSessionId: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  launchTemplates: any[];
  setLaunchTemplates: (valOrUpdater: any[] | ((prev: any[]) => any[])) => void;
}

export const useTradingStore = create<useTradingStoreState>((set) => ({
  summaries: [],
  setSummaries: (valOrUpdater) => set((state) => ({ summaries: resolveUpdater(valOrUpdater, state.summaries) })),
  accounts: [],
  setAccounts: (valOrUpdater) => set((state) => ({ accounts: resolveUpdater(valOrUpdater, state.accounts) })),
  orders: [],
  setOrders: (valOrUpdater) => set((state) => ({ orders: resolveUpdater(valOrUpdater, state.orders) })),
  fills: [],
  setFills: (valOrUpdater) => set((state) => ({ fills: resolveUpdater(valOrUpdater, state.fills) })),
  positions: [],
  setPositions: (valOrUpdater) => set((state) => ({ positions: resolveUpdater(valOrUpdater, state.positions) })),
  snapshots: [],
  setSnapshots: (valOrUpdater) => set((state) => ({ snapshots: resolveUpdater(valOrUpdater, state.snapshots) })),
  strategies: [],
  setStrategies: (valOrUpdater) => set((state) => ({ strategies: resolveUpdater(valOrUpdater, state.strategies) })),
  backtests: [],
  setBacktests: (valOrUpdater) => set((state) => ({ backtests: resolveUpdater(valOrUpdater, state.backtests) })),
  backtestOptions: null,
  setBacktestOptions: (valOrUpdater) => set((state) => ({ backtestOptions: resolveUpdater(valOrUpdater, state.backtestOptions) })),
  paperSessions: [],
  setPaperSessions: (valOrUpdater) => set((state) => ({ paperSessions: resolveUpdater(valOrUpdater, state.paperSessions) })),
  liveSessions: [],
  setLiveSessions: (valOrUpdater) => set((state) => ({ liveSessions: resolveUpdater(valOrUpdater, state.liveSessions) })),
  liveAdapters: [],
  setLiveAdapters: (valOrUpdater) => set((state) => ({ liveAdapters: resolveUpdater(valOrUpdater, state.liveAdapters) })),
  signalCatalog: null,
  setSignalCatalog: (valOrUpdater) => set((state) => ({ signalCatalog: resolveUpdater(valOrUpdater, state.signalCatalog) })),
  signalSourceTypes: [],
  setSignalSourceTypes: (valOrUpdater) => set((state) => ({ signalSourceTypes: resolveUpdater(valOrUpdater, state.signalSourceTypes) })),
  signalRuntimeAdapters: [],
  setSignalRuntimeAdapters: (valOrUpdater) => set((state) => ({ signalRuntimeAdapters: resolveUpdater(valOrUpdater, state.signalRuntimeAdapters) })),
  signalRuntimeSessions: [],
  setSignalRuntimeSessions: (valOrUpdater) => set((state) => ({ signalRuntimeSessions: resolveUpdater(valOrUpdater, state.signalRuntimeSessions) })),
  runtimePolicy: null,
  setRuntimePolicy: (valOrUpdater) => set((state) => ({ runtimePolicy: resolveUpdater(valOrUpdater, state.runtimePolicy) })),
  monitorHealth: null,
  setMonitorHealth: (valOrUpdater) => set((state) => ({ monitorHealth: resolveUpdater(valOrUpdater, state.monitorHealth) })),
  alerts: [],
  setAlerts: (valOrUpdater) => set((state) => ({ alerts: resolveUpdater(valOrUpdater, state.alerts) })),
  notifications: [],
  setNotifications: (valOrUpdater) => set((state) => ({ notifications: resolveUpdater(valOrUpdater, state.notifications) })),
  telegramConfig: null,
  setTelegramConfig: (valOrUpdater) => set((state) => ({ telegramConfig: resolveUpdater(valOrUpdater, state.telegramConfig) })),
  strategySignalBindings: [],
  setStrategySignalBindings: (valOrUpdater) => set((state) => ({ strategySignalBindings: resolveUpdater(valOrUpdater, state.strategySignalBindings) })),
  strategySignalBindingMap: {},
  setStrategySignalBindingMap: (valOrUpdater) => set((state) => ({ strategySignalBindingMap: resolveUpdater(valOrUpdater, state.strategySignalBindingMap) })),
  signalRuntimePlan: null,
  setSignalRuntimePlan: (valOrUpdater) => set((state) => ({ signalRuntimePlan: resolveUpdater(valOrUpdater, state.signalRuntimePlan) })),
  selectedSignalRuntimeId: localStorage.getItem('bk_selected_signal_runtime_id'),
  setSelectedSignalRuntimeId: (valOrUpdater) => set((state) => {
    const next = resolveUpdater(valOrUpdater, state.selectedSignalRuntimeId);
    if (next) {
      localStorage.setItem('bk_selected_signal_runtime_id', next);
    } else {
      localStorage.removeItem('bk_selected_signal_runtime_id');
    }
    return { selectedSignalRuntimeId: next };
  }),
  selectedStrategyId: null,
  setSelectedStrategyId: (valOrUpdater) => set((state) => ({ selectedStrategyId: resolveUpdater(valOrUpdater, state.selectedStrategyId) })),
  candles: [],
  setCandles: (valOrUpdater) => set((state) => ({ candles: resolveUpdater(valOrUpdater, state.candles) })),
  monitorCandles: [],
  setMonitorCandles: (valOrUpdater) => set((state) => ({ monitorCandles: resolveUpdater(valOrUpdater, state.monitorCandles) })),
  annotations: [],
  setAnnotations: (valOrUpdater) => set((state) => ({ annotations: resolveUpdater(valOrUpdater, state.annotations) })),
  editingLiveSessionId: null,
  setEditingLiveSessionId: (valOrUpdater) => set((state) => ({ editingLiveSessionId: resolveUpdater(valOrUpdater, state.editingLiveSessionId) })),
  launchTemplates: [],
  setLaunchTemplates: (valOrUpdater) => set((state) => ({ launchTemplates: resolveUpdater(valOrUpdater, state.launchTemplates) })),
}));
