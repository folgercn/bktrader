import { create } from 'zustand';
import { AccountSummary, AccountRecord, Order, Fill, Position, AccountEquitySnapshot, StrategyRecord, BacktestRun, BacktestOptions, PaperSession, LiveSession, LiveAdapter, SignalSourceCatalog, SignalSourceType, SignalRuntimeAdapter, SignalRuntimeSession, RuntimePolicy, PlatformAlert, PlatformNotification, TelegramConfig, SignalBinding, ChartCandle, ChartAnnotation, MarkerDetail, ChartOverrideRange, SelectedSample, SourceFilter, EventFilter, TimeWindow, AuthSession } from '../types/domain';
import { readStoredAuthSession } from '../utils/auth';


export interface useTradingStoreState {
  summaries: any[];
  setSummaries: (valOrUpdater: any[] | ((prev: any[]) => any[])) => void;
  accounts: any[];
  setAccounts: (valOrUpdater: any[] | ((prev: any[]) => any[])) => void;
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
  alerts: PlatformAlert[];
  setAlerts: (valOrUpdater: PlatformAlert[] | ((prev: PlatformAlert[]) => PlatformAlert[])) => void;
  notifications: PlatformNotification[];
  setNotifications: (valOrUpdater: PlatformNotification[] | ((prev: PlatformNotification[]) => PlatformNotification[])) => void;
  telegramConfig: TelegramConfig | null;
  setTelegramConfig: (valOrUpdater: TelegramConfig | null | ((prev: TelegramConfig | null) => TelegramConfig | null)) => void;
  accountSignalBindings: SignalBinding[];
  setAccountSignalBindings: (valOrUpdater: SignalBinding[] | ((prev: SignalBinding[]) => SignalBinding[])) => void;
  strategySignalBindings: SignalBinding[];
  setStrategySignalBindings: (valOrUpdater: SignalBinding[] | ((prev: SignalBinding[]) => SignalBinding[])) => void;
  accountSignalBindingMap: Record<string, SignalBinding[]>;
  setAccountSignalBindingMap: (valOrUpdater: Record<string, SignalBinding[]> | ((prev: Record<string, SignalBinding[]>) => Record<string, SignalBinding[]>)) => void;
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
}

export const useTradingStore = create<useTradingStoreState>((set) => ({
  summaries: [],
  setSummaries: (valOrUpdater) => set((state) => ({ summaries: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.summaries) : valOrUpdater })),
  accounts: [],
  setAccounts: (valOrUpdater) => set((state) => ({ accounts: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.accounts) : valOrUpdater })),
  orders: [],
  setOrders: (valOrUpdater) => set((state) => ({ orders: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.orders) : valOrUpdater })),
  fills: [],
  setFills: (valOrUpdater) => set((state) => ({ fills: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.fills) : valOrUpdater })),
  positions: [],
  setPositions: (valOrUpdater) => set((state) => ({ positions: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.positions) : valOrUpdater })),
  snapshots: [],
  setSnapshots: (valOrUpdater) => set((state) => ({ snapshots: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.snapshots) : valOrUpdater })),
  strategies: [],
  setStrategies: (valOrUpdater) => set((state) => ({ strategies: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.strategies) : valOrUpdater })),
  backtests: [],
  setBacktests: (valOrUpdater) => set((state) => ({ backtests: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.backtests) : valOrUpdater })),
  backtestOptions: null,
  setBacktestOptions: (valOrUpdater) => set((state) => ({ backtestOptions: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.backtestOptions) : valOrUpdater })),
  paperSessions: [],
  setPaperSessions: (valOrUpdater) => set((state) => ({ paperSessions: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.paperSessions) : valOrUpdater })),
  liveSessions: [],
  setLiveSessions: (valOrUpdater) => set((state) => ({ liveSessions: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveSessions) : valOrUpdater })),
  liveAdapters: [],
  setLiveAdapters: (valOrUpdater) => set((state) => ({ liveAdapters: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveAdapters) : valOrUpdater })),
  signalCatalog: null,
  setSignalCatalog: (valOrUpdater) => set((state) => ({ signalCatalog: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.signalCatalog) : valOrUpdater })),
  signalSourceTypes: [],
  setSignalSourceTypes: (valOrUpdater) => set((state) => ({ signalSourceTypes: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.signalSourceTypes) : valOrUpdater })),
  signalRuntimeAdapters: [],
  setSignalRuntimeAdapters: (valOrUpdater) => set((state) => ({ signalRuntimeAdapters: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.signalRuntimeAdapters) : valOrUpdater })),
  signalRuntimeSessions: [],
  setSignalRuntimeSessions: (valOrUpdater) => set((state) => ({ signalRuntimeSessions: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.signalRuntimeSessions) : valOrUpdater })),
  runtimePolicy: null,
  setRuntimePolicy: (valOrUpdater) => set((state) => ({ runtimePolicy: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.runtimePolicy) : valOrUpdater })),
  alerts: [],
  setAlerts: (valOrUpdater) => set((state) => ({ alerts: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.alerts) : valOrUpdater })),
  notifications: [],
  setNotifications: (valOrUpdater) => set((state) => ({ notifications: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.notifications) : valOrUpdater })),
  telegramConfig: null,
  setTelegramConfig: (valOrUpdater) => set((state) => ({ telegramConfig: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.telegramConfig) : valOrUpdater })),
  accountSignalBindings: [],
  setAccountSignalBindings: (valOrUpdater) => set((state) => ({ accountSignalBindings: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.accountSignalBindings) : valOrUpdater })),
  strategySignalBindings: [],
  setStrategySignalBindings: (valOrUpdater) => set((state) => ({ strategySignalBindings: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.strategySignalBindings) : valOrUpdater })),
  accountSignalBindingMap: {},
  setAccountSignalBindingMap: (valOrUpdater) => set((state) => ({ accountSignalBindingMap: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.accountSignalBindingMap) : valOrUpdater })),
  strategySignalBindingMap: {},
  setStrategySignalBindingMap: (valOrUpdater) => set((state) => ({ strategySignalBindingMap: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.strategySignalBindingMap) : valOrUpdater })),
  signalRuntimePlan: null,
  setSignalRuntimePlan: (valOrUpdater) => set((state) => ({ signalRuntimePlan: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.signalRuntimePlan) : valOrUpdater })),
  selectedSignalRuntimeId: null,
  setSelectedSignalRuntimeId: (valOrUpdater) => set((state) => ({ selectedSignalRuntimeId: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.selectedSignalRuntimeId) : valOrUpdater })),
  selectedStrategyId: null,
  setSelectedStrategyId: (valOrUpdater) => set((state) => ({ selectedStrategyId: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.selectedStrategyId) : valOrUpdater })),
  candles: [],
  setCandles: (valOrUpdater) => set((state) => ({ candles: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.candles) : valOrUpdater })),
  monitorCandles: [],
  setMonitorCandles: (valOrUpdater) => set((state) => ({ monitorCandles: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.monitorCandles) : valOrUpdater })),
  annotations: [],
  setAnnotations: (valOrUpdater) => set((state) => ({ annotations: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.annotations) : valOrUpdater })),
  editingLiveSessionId: null,
  setEditingLiveSessionId: (valOrUpdater) => set((state) => ({ editingLiveSessionId: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.editingLiveSessionId) : valOrUpdater })),
}));
