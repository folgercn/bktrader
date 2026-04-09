import { create } from 'zustand';
import { AccountSummary, AccountRecord, Order, Fill, Position, AccountEquitySnapshot, StrategyRecord, BacktestRun, BacktestOptions, PaperSession, LiveSession, LiveAdapter, SignalSourceCatalog, SignalSourceType, SignalRuntimeAdapter, SignalRuntimeSession, RuntimePolicy, PlatformAlert, PlatformNotification, TelegramConfig, SignalBinding, ChartCandle, ChartAnnotation, MarkerDetail, ChartOverrideRange, SelectedSample, SourceFilter, EventFilter, TimeWindow, AuthSession } from '../types/domain';
import { readStoredAuthSession } from '../utils/auth';


export interface useUIStoreState {
  sidebarTab: "monitor" | "strategy" | "account";
  setSidebarTab: (val: "monitor" | "strategy" | "account") => void;
  dockTab: "orders" | "positions" | "fills" | "alerts";
  setDockTab: (val: "orders" | "positions" | "fills" | "alerts") => void;
  loading: boolean;
  setLoading: (valOrUpdater: boolean | ((prev: boolean) => boolean)) => void;
  error: string | null;
  setError: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  authSession: AuthSession | null;
  setAuthSession: (valOrUpdater: AuthSession | null | ((prev: AuthSession | null) => AuthSession | null)) => void;
  loginForm: any;
  setLoginForm: (valOrUpdater: any | ((prev: any) => any)) => void;
  loginAction: boolean;
  setLoginAction: (valOrUpdater: boolean | ((prev: boolean) => boolean)) => void;
  sessionAction: string | null;
  setSessionAction: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  paperCreateAction: boolean;
  setPaperCreateAction: (valOrUpdater: boolean | ((prev: boolean) => boolean)) => void;
  paperLaunchAction: boolean;
  setPaperLaunchAction: (valOrUpdater: boolean | ((prev: boolean) => boolean)) => void;
  liveCreateAction: boolean;
  setLiveCreateAction: (valOrUpdater: boolean | ((prev: boolean) => boolean)) => void;
  liveBindAction: boolean;
  setLiveBindAction: (valOrUpdater: boolean | ((prev: boolean) => boolean)) => void;
  liveSyncAction: string | null;
  setLiveSyncAction: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  liveAccountSyncAction: string | null;
  setLiveAccountSyncAction: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  liveFlowAction: string | null;
  setLiveFlowAction: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  liveOrderAction: boolean;
  setLiveOrderAction: (valOrUpdater: boolean | ((prev: boolean) => boolean)) => void;
  liveSessionAction: string | null;
  setLiveSessionAction: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  liveSessionCreateAction: boolean;
  setLiveSessionCreateAction: (valOrUpdater: boolean | ((prev: boolean) => boolean)) => void;
  liveSessionLaunchAction: boolean;
  setLiveSessionLaunchAction: (valOrUpdater: boolean | ((prev: boolean) => boolean)) => void;
  liveSessionDeleteAction: string | null;
  setLiveSessionDeleteAction: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  signalBindingAction: string | null;
  setSignalBindingAction: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  signalRuntimeAction: string | null;
  setSignalRuntimeAction: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  notificationAction: string | null;
  setNotificationAction: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  telegramAction: string | null;
  setTelegramAction: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  backtestAction: boolean;
  setBacktestAction: (valOrUpdater: boolean | ((prev: boolean) => boolean)) => void;
  runtimePolicyAction: boolean;
  setRuntimePolicyAction: (valOrUpdater: boolean | ((prev: boolean) => boolean)) => void;
  strategyCreateAction: boolean;
  setStrategyCreateAction: (valOrUpdater: boolean | ((prev: boolean) => boolean)) => void;
  strategySaveAction: boolean;
  setStrategySaveAction: (valOrUpdater: boolean | ((prev: boolean) => boolean)) => void;
  sourceFilter: SourceFilter;
  setSourceFilter: (valOrUpdater: SourceFilter | ((prev: SourceFilter) => SourceFilter)) => void;
  eventFilter: EventFilter;
  setEventFilter: (valOrUpdater: EventFilter | ((prev: EventFilter) => EventFilter)) => void;
  timeWindow: TimeWindow;
  setTimeWindow: (valOrUpdater: TimeWindow | ((prev: TimeWindow) => TimeWindow)) => void;
  focusNonce: number;
  setFocusNonce: (valOrUpdater: number | ((prev: number) => number)) => void;
  hoveredMarker: MarkerDetail | null;
  setHoveredMarker: (valOrUpdater: MarkerDetail | null | ((prev: MarkerDetail | null) => MarkerDetail | null)) => void;
  selectedBacktestId: string | null;
  setSelectedBacktestId: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  chartOverrideRange: ChartOverrideRange | null;
  setChartOverrideRange: (valOrUpdater: ChartOverrideRange | null | ((prev: ChartOverrideRange | null) => ChartOverrideRange | null)) => void;
  selectedSample: SelectedSample | null;
  setSelectedSample: (valOrUpdater: SelectedSample | null | ((prev: SelectedSample | null) => SelectedSample | null)) => void;
  backtestForm: any;
  setBacktestForm: (valOrUpdater: any | ((prev: any) => any)) => void;
  paperForm: any;
  setPaperForm: (valOrUpdater: any | ((prev: any) => any)) => void;
  liveAccountForm: any;
  setLiveAccountForm: (valOrUpdater: any | ((prev: any) => any)) => void;
  liveBindingForm: any;
  setLiveBindingForm: (valOrUpdater: any | ((prev: any) => any)) => void;
  liveOrderForm: any;
  setLiveOrderForm: (valOrUpdater: any | ((prev: any) => any)) => void;
  liveSessionForm: any;
  setLiveSessionForm: (valOrUpdater: any | ((prev: any) => any)) => void;
  accountSignalForm: any;
  setAccountSignalForm: (valOrUpdater: any | ((prev: any) => any)) => void;
  strategySignalForm: any;
  setStrategySignalForm: (valOrUpdater: any | ((prev: any) => any)) => void;
  strategyCreateForm: any;
  setStrategyCreateForm: (valOrUpdater: any | ((prev: any) => any)) => void;
  strategyEditorForm: any;
  setStrategyEditorForm: (valOrUpdater: any | ((prev: any) => any)) => void;
  signalRuntimeForm: any;
  setSignalRuntimeForm: (valOrUpdater: any | ((prev: any) => any)) => void;
  runtimePolicyForm: any;
  setRuntimePolicyForm: (valOrUpdater: any | ((prev: any) => any)) => void;
  telegramForm: any;
  setTelegramForm: (valOrUpdater: any | ((prev: any) => any)) => void;
  liveAccountError: string | null;
  setLiveAccountError: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  liveBindingError: string | null;
  setLiveBindingError: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  liveSessionError: string | null;
  setLiveSessionError: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  liveAccountNotice: string | null;
  setLiveAccountNotice: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  liveBindingNotice: string | null;
  setLiveBindingNotice: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  liveSessionNotice: string | null;
  setLiveSessionNotice: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  settingsMenuOpen: boolean;
  setSettingsMenuOpen: (valOrUpdater: boolean | ((prev: boolean) => boolean)) => void;
  activeSettingsModal: "telegram" | "live-account" | "live-binding" | "live-session" | null;
  setActiveSettingsModal: (valOrUpdater: "telegram" | "live-account" | "live-binding" | "live-session" | null | ((prev: "telegram" | "live-account" | "live-binding" | "live-session" | null) => "telegram" | "live-account" | "live-binding" | "live-session" | null)) => void;
}

export const useUIStore = create<useUIStoreState>((set) => ({
  sidebarTab: "monitor",
  setSidebarTab: (val) => set({ sidebarTab: val }),
  dockTab: "orders",
  setDockTab: (val) => set({ dockTab: val }),
  loading: true,
  setLoading: (valOrUpdater) => set((state) => ({ loading: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.loading) : valOrUpdater })),
  error: null,
  setError: (valOrUpdater) => set((state) => ({ error: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.error) : valOrUpdater })),
  authSession: readStoredAuthSession(),
  setAuthSession: (valOrUpdater) => set((state) => ({ authSession: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.authSession) : valOrUpdater })),
  loginForm: { username: "admin", password: "" },
  setLoginForm: (valOrUpdater) => set((state) => ({ loginForm: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.loginForm) : valOrUpdater })),
  loginAction: false,
  setLoginAction: (valOrUpdater) => set((state) => ({ loginAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.loginAction) : valOrUpdater })),
  sessionAction: null,
  setSessionAction: (valOrUpdater) => set((state) => ({ sessionAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.sessionAction) : valOrUpdater })),
  paperCreateAction: false,
  setPaperCreateAction: (valOrUpdater) => set((state) => ({ paperCreateAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.paperCreateAction) : valOrUpdater })),
  paperLaunchAction: false,
  setPaperLaunchAction: (valOrUpdater) => set((state) => ({ paperLaunchAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.paperLaunchAction) : valOrUpdater })),
  liveCreateAction: false,
  setLiveCreateAction: (valOrUpdater) => set((state) => ({ liveCreateAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveCreateAction) : valOrUpdater })),
  liveBindAction: false,
  setLiveBindAction: (valOrUpdater) => set((state) => ({ liveBindAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveBindAction) : valOrUpdater })),
  liveSyncAction: null,
  setLiveSyncAction: (valOrUpdater) => set((state) => ({ liveSyncAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveSyncAction) : valOrUpdater })),
  liveAccountSyncAction: null,
  setLiveAccountSyncAction: (valOrUpdater) => set((state) => ({ liveAccountSyncAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveAccountSyncAction) : valOrUpdater })),
  liveFlowAction: null,
  setLiveFlowAction: (valOrUpdater) => set((state) => ({ liveFlowAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveFlowAction) : valOrUpdater })),
  liveOrderAction: false,
  setLiveOrderAction: (valOrUpdater) => set((state) => ({ liveOrderAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveOrderAction) : valOrUpdater })),
  liveSessionAction: null,
  setLiveSessionAction: (valOrUpdater) => set((state) => ({ liveSessionAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveSessionAction) : valOrUpdater })),
  liveSessionCreateAction: false,
  setLiveSessionCreateAction: (valOrUpdater) => set((state) => ({ liveSessionCreateAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveSessionCreateAction) : valOrUpdater })),
  liveSessionLaunchAction: false,
  setLiveSessionLaunchAction: (valOrUpdater) => set((state) => ({ liveSessionLaunchAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveSessionLaunchAction) : valOrUpdater })),
  liveSessionDeleteAction: null,
  setLiveSessionDeleteAction: (valOrUpdater) => set((state) => ({ liveSessionDeleteAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveSessionDeleteAction) : valOrUpdater })),
  signalBindingAction: null,
  setSignalBindingAction: (valOrUpdater) => set((state) => ({ signalBindingAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.signalBindingAction) : valOrUpdater })),
  signalRuntimeAction: null,
  setSignalRuntimeAction: (valOrUpdater) => set((state) => ({ signalRuntimeAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.signalRuntimeAction) : valOrUpdater })),
  notificationAction: null,
  setNotificationAction: (valOrUpdater) => set((state) => ({ notificationAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.notificationAction) : valOrUpdater })),
  telegramAction: null,
  setTelegramAction: (valOrUpdater) => set((state) => ({ telegramAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.telegramAction) : valOrUpdater })),
  backtestAction: false,
  setBacktestAction: (valOrUpdater) => set((state) => ({ backtestAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.backtestAction) : valOrUpdater })),
  runtimePolicyAction: false,
  setRuntimePolicyAction: (valOrUpdater) => set((state) => ({ runtimePolicyAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.runtimePolicyAction) : valOrUpdater })),
  strategyCreateAction: false,
  setStrategyCreateAction: (valOrUpdater) => set((state) => ({ strategyCreateAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.strategyCreateAction) : valOrUpdater })),
  strategySaveAction: false,
  setStrategySaveAction: (valOrUpdater) => set((state) => ({ strategySaveAction: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.strategySaveAction) : valOrUpdater })),
  sourceFilter: "all",
  setSourceFilter: (valOrUpdater) => set((state) => ({ sourceFilter: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.sourceFilter) : valOrUpdater })),
  eventFilter: "all",
  setEventFilter: (valOrUpdater) => set((state) => ({ eventFilter: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.eventFilter) : valOrUpdater })),
  timeWindow: "12h",
  setTimeWindow: (valOrUpdater) => set((state) => ({ timeWindow: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.timeWindow) : valOrUpdater })),
  focusNonce: 0,
  setFocusNonce: (valOrUpdater) => set((state) => ({ focusNonce: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.focusNonce) : valOrUpdater })),
  hoveredMarker: null,
  setHoveredMarker: (valOrUpdater) => set((state) => ({ hoveredMarker: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.hoveredMarker) : valOrUpdater })),
  selectedBacktestId: null,
  setSelectedBacktestId: (valOrUpdater) => set((state) => ({ selectedBacktestId: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.selectedBacktestId) : valOrUpdater })),
  chartOverrideRange: null,
  setChartOverrideRange: (valOrUpdater) => set((state) => ({ chartOverrideRange: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.chartOverrideRange) : valOrUpdater })),
  selectedSample: null,
  setSelectedSample: (valOrUpdater) => set((state) => ({ selectedSample: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.selectedSample) : valOrUpdater })),
  backtestForm: { strategyVersionId: "", signalTimeframe: "1d", executionDataSource: "1min", symbol: "BTCUSDT", from: "", to: "", },
  setBacktestForm: (valOrUpdater) => set((state) => ({ backtestForm: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.backtestForm) : valOrUpdater })),
  paperForm: { accountId: "", strategyId: "", startEquity: "100000", signalTimeframe: "1d", executionDataSource: "tick", symbol: "BTCUSDT", tradingFeeBps: "10", fundingRateBps: "0", fundingIntervalHours: "8", },
  setPaperForm: (valOrUpdater) => set((state) => ({ paperForm: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.paperForm) : valOrUpdater })),
  liveAccountForm: { name: "Binance Testnet", exchange: "binance-futures", },
  setLiveAccountForm: (valOrUpdater) => set((state) => ({ liveAccountForm: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveAccountForm) : valOrUpdater })),
  liveBindingForm: { accountId: "", adapterKey: "binance-futures", positionMode: "ONE_WAY", marginMode: "CROSSED", sandbox: true, apiKeyRef: "BINANCE_TESTNET_API_KEY", apiSecretRef: "BINANCE_TESTNET_API_SECRET", },
  setLiveBindingForm: (valOrUpdater) => set((state) => ({ liveBindingForm: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveBindingForm) : valOrUpdater })),
  liveOrderForm: { accountId: "", strategyVersionId: "", symbol: "BTCUSDT", side: "BUY", type: "LIMIT", quantity: "0.001", price: "", },
  setLiveOrderForm: (valOrUpdater) => set((state) => ({ liveOrderForm: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveOrderForm) : valOrUpdater })),
  liveSessionForm: { accountId: "", strategyId: "", signalTimeframe: "1d", executionDataSource: "tick", symbol: "BTCUSDT", defaultOrderQuantity: "0.001", executionEntryOrderType: "MARKET", executionEntryMaxSpreadBps: "8", executionEntryWideSpreadMode: "limit-maker", executionEntryTimeoutFallbackOrderType: "MARKET", executionPTExitOrderType: "LIMIT", executionPTExitTimeInForce: "GTX", executionPTExitPostOnly: true, executionPTExitTimeoutFallbackOrderType: "MARKET", executionSLExitOrderType: "MARKET", executionSLExitMaxSpreadBps: "999", dispatchMode: "manual-review", dispatchCooldownSeconds: "30", },
  setLiveSessionForm: (valOrUpdater) => set((state) => ({ liveSessionForm: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveSessionForm) : valOrUpdater })),
  accountSignalForm: { accountId: "", sourceKey: "", role: "trigger", symbol: "BTCUSDT", timeframe: "1d", },
  setAccountSignalForm: (valOrUpdater) => set((state) => ({ accountSignalForm: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.accountSignalForm) : valOrUpdater })),
  strategySignalForm: { strategyId: "", sourceKey: "", role: "trigger", symbol: "BTCUSDT", timeframe: "1d", },
  setStrategySignalForm: (valOrUpdater) => set((state) => ({ strategySignalForm: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.strategySignalForm) : valOrUpdater })),
  strategyCreateForm: { name: "", description: "", },
  setStrategyCreateForm: (valOrUpdater) => set((state) => ({ strategyCreateForm: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.strategyCreateForm) : valOrUpdater })),
  strategyEditorForm: { strategyId: "", strategyEngine: "bk-default", signalTimeframe: "1d", executionDataSource: "tick", parametersJson: "{}", },
  setStrategyEditorForm: (valOrUpdater) => set((state) => ({ strategyEditorForm: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.strategyEditorForm) : valOrUpdater })),
  signalRuntimeForm: { accountId: "", strategyId: "", },
  setSignalRuntimeForm: (valOrUpdater) => set((state) => ({ signalRuntimeForm: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.signalRuntimeForm) : valOrUpdater })),
  runtimePolicyForm: { tradeTickFreshnessSeconds: "15", orderBookFreshnessSeconds: "10", signalBarFreshnessSeconds: "30", runtimeQuietSeconds: "30", paperStartReadinessTimeoutSeconds: "5", },
  setRuntimePolicyForm: (valOrUpdater) => set((state) => ({ runtimePolicyForm: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.runtimePolicyForm) : valOrUpdater })),
  telegramForm: { enabled: false, botToken: "", chatId: "", sendLevels: "critical,warning", },
  setTelegramForm: (valOrUpdater) => set((state) => ({ telegramForm: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.telegramForm) : valOrUpdater })),
  liveAccountError: null,
  setLiveAccountError: (valOrUpdater) => set((state) => ({ liveAccountError: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveAccountError) : valOrUpdater })),
  liveBindingError: null,
  setLiveBindingError: (valOrUpdater) => set((state) => ({ liveBindingError: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveBindingError) : valOrUpdater })),
  liveSessionError: null,
  setLiveSessionError: (valOrUpdater) => set((state) => ({ liveSessionError: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveSessionError) : valOrUpdater })),
  liveAccountNotice: null,
  setLiveAccountNotice: (valOrUpdater) => set((state) => ({ liveAccountNotice: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveAccountNotice) : valOrUpdater })),
  liveBindingNotice: null,
  setLiveBindingNotice: (valOrUpdater) => set((state) => ({ liveBindingNotice: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveBindingNotice) : valOrUpdater })),
  liveSessionNotice: null,
  setLiveSessionNotice: (valOrUpdater) => set((state) => ({ liveSessionNotice: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.liveSessionNotice) : valOrUpdater })),
  settingsMenuOpen: false,
  setSettingsMenuOpen: (valOrUpdater) => set((state) => ({ settingsMenuOpen: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.settingsMenuOpen) : valOrUpdater })),
  activeSettingsModal: null,
  setActiveSettingsModal: (valOrUpdater) => set((state) => ({ activeSettingsModal: typeof valOrUpdater === 'function' ? (valOrUpdater as any)(state.activeSettingsModal) : valOrUpdater })),
}));
