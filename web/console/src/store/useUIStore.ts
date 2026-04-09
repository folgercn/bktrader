import { create } from 'zustand';
import { 
  AccountSummary, AccountRecord, Order, Fill, Position, AccountEquitySnapshot, StrategyRecord, BacktestRun, BacktestOptions, PaperSession, LiveSession, LiveAdapter, SignalSourceCatalog, SignalSourceType, SignalRuntimeAdapter, SignalRuntimeSession, RuntimePolicy, PlatformAlert, PlatformNotification, TelegramConfig, SignalBinding, ChartCandle, ChartAnnotation, MarkerDetail, ChartOverrideRange, SelectedSample, SourceFilter, EventFilter, TimeWindow, AuthSession,
  LoginForm, BacktestForm, PaperForm, LiveAccountForm, LiveBindingForm, LiveOrderForm, LiveSessionForm, AccountSignalForm, StrategySignalForm, StrategyCreateForm, StrategyEditorForm, SignalRuntimeForm, RuntimePolicyForm, TelegramForm
} from '../types/domain';
import { readStoredAuthSession } from '../utils/auth';
import { resolveUpdater } from './helpers';


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
  loginForm: LoginForm;
  setLoginForm: (valOrUpdater: LoginForm | ((prev: LoginForm) => LoginForm)) => void;
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
  backtestForm: BacktestForm;
  setBacktestForm: (valOrUpdater: BacktestForm | ((prev: BacktestForm) => BacktestForm)) => void;
  paperForm: PaperForm;
  setPaperForm: (valOrUpdater: PaperForm | ((prev: PaperForm) => PaperForm)) => void;
  liveAccountForm: LiveAccountForm;
  setLiveAccountForm: (valOrUpdater: LiveAccountForm | ((prev: LiveAccountForm) => LiveAccountForm)) => void;
  liveBindingForm: LiveBindingForm;
  setLiveBindingForm: (valOrUpdater: LiveBindingForm | ((prev: LiveBindingForm) => LiveBindingForm)) => void;
  liveOrderForm: LiveOrderForm;
  setLiveOrderForm: (valOrUpdater: LiveOrderForm | ((prev: LiveOrderForm) => LiveOrderForm)) => void;
  liveSessionForm: LiveSessionForm;
  setLiveSessionForm: (valOrUpdater: LiveSessionForm | ((prev: LiveSessionForm) => LiveSessionForm)) => void;
  accountSignalForm: AccountSignalForm;
  setAccountSignalForm: (valOrUpdater: AccountSignalForm | ((prev: AccountSignalForm) => AccountSignalForm)) => void;
  strategySignalForm: StrategySignalForm;
  setStrategySignalForm: (valOrUpdater: StrategySignalForm | ((prev: StrategySignalForm) => StrategySignalForm)) => void;
  strategyCreateForm: StrategyCreateForm;
  setStrategyCreateForm: (valOrUpdater: StrategyCreateForm | ((prev: StrategyCreateForm) => StrategyCreateForm)) => void;
  strategyEditorForm: StrategyEditorForm;
  setStrategyEditorForm: (valOrUpdater: StrategyEditorForm | ((prev: StrategyEditorForm) => StrategyEditorForm)) => void;
  signalRuntimeForm: SignalRuntimeForm;
  setSignalRuntimeForm: (valOrUpdater: SignalRuntimeForm | ((prev: SignalRuntimeForm) => SignalRuntimeForm)) => void;
  runtimePolicyForm: RuntimePolicyForm;
  setRuntimePolicyForm: (valOrUpdater: RuntimePolicyForm | ((prev: RuntimePolicyForm) => RuntimePolicyForm)) => void;
  telegramForm: TelegramForm;
  setTelegramForm: (valOrUpdater: TelegramForm | ((prev: TelegramForm) => TelegramForm)) => void;
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
  setLoading: (valOrUpdater) => set((state) => ({ loading: resolveUpdater(valOrUpdater, state.loading) })),
  error: null,
  setError: (valOrUpdater) => set((state) => ({ error: resolveUpdater(valOrUpdater, state.error) })),
  authSession: readStoredAuthSession(),
  setAuthSession: (valOrUpdater) => set((state) => ({ authSession: resolveUpdater(valOrUpdater, state.authSession) })),
  loginForm: { username: "admin", password: "" },
  setLoginForm: (valOrUpdater) => set((state) => ({ loginForm: resolveUpdater(valOrUpdater, state.loginForm) })),
  loginAction: false,
  setLoginAction: (valOrUpdater) => set((state) => ({ loginAction: resolveUpdater(valOrUpdater, state.loginAction) })),
  sessionAction: null,
  setSessionAction: (valOrUpdater) => set((state) => ({ sessionAction: resolveUpdater(valOrUpdater, state.sessionAction) })),
  paperCreateAction: false,
  setPaperCreateAction: (valOrUpdater) => set((state) => ({ paperCreateAction: resolveUpdater(valOrUpdater, state.paperCreateAction) })),
  paperLaunchAction: false,
  setPaperLaunchAction: (valOrUpdater) => set((state) => ({ paperLaunchAction: resolveUpdater(valOrUpdater, state.paperLaunchAction) })),
  liveCreateAction: false,
  setLiveCreateAction: (valOrUpdater) => set((state) => ({ liveCreateAction: resolveUpdater(valOrUpdater, state.liveCreateAction) })),
  liveBindAction: false,
  setLiveBindAction: (valOrUpdater) => set((state) => ({ liveBindAction: resolveUpdater(valOrUpdater, state.liveBindAction) })),
  liveSyncAction: null,
  setLiveSyncAction: (valOrUpdater) => set((state) => ({ liveSyncAction: resolveUpdater(valOrUpdater, state.liveSyncAction) })),
  liveAccountSyncAction: null,
  setLiveAccountSyncAction: (valOrUpdater) => set((state) => ({ liveAccountSyncAction: resolveUpdater(valOrUpdater, state.liveAccountSyncAction) })),
  liveFlowAction: null,
  setLiveFlowAction: (valOrUpdater) => set((state) => ({ liveFlowAction: resolveUpdater(valOrUpdater, state.liveFlowAction) })),
  liveOrderAction: false,
  setLiveOrderAction: (valOrUpdater) => set((state) => ({ liveOrderAction: resolveUpdater(valOrUpdater, state.liveOrderAction) })),
  liveSessionAction: null,
  setLiveSessionAction: (valOrUpdater) => set((state) => ({ liveSessionAction: resolveUpdater(valOrUpdater, state.liveSessionAction) })),
  liveSessionCreateAction: false,
  setLiveSessionCreateAction: (valOrUpdater) => set((state) => ({ liveSessionCreateAction: resolveUpdater(valOrUpdater, state.liveSessionCreateAction) })),
  liveSessionLaunchAction: false,
  setLiveSessionLaunchAction: (valOrUpdater) => set((state) => ({ liveSessionLaunchAction: resolveUpdater(valOrUpdater, state.liveSessionLaunchAction) })),
  liveSessionDeleteAction: null,
  setLiveSessionDeleteAction: (valOrUpdater) => set((state) => ({ liveSessionDeleteAction: resolveUpdater(valOrUpdater, state.liveSessionDeleteAction) })),
  signalBindingAction: null,
  setSignalBindingAction: (valOrUpdater) => set((state) => ({ signalBindingAction: resolveUpdater(valOrUpdater, state.signalBindingAction) })),
  signalRuntimeAction: null,
  setSignalRuntimeAction: (valOrUpdater) => set((state) => ({ signalRuntimeAction: resolveUpdater(valOrUpdater, state.signalRuntimeAction) })),
  notificationAction: null,
  setNotificationAction: (valOrUpdater) => set((state) => ({ notificationAction: resolveUpdater(valOrUpdater, state.notificationAction) })),
  telegramAction: null,
  setTelegramAction: (valOrUpdater) => set((state) => ({ telegramAction: resolveUpdater(valOrUpdater, state.telegramAction) })),
  backtestAction: false,
  setBacktestAction: (valOrUpdater) => set((state) => ({ backtestAction: resolveUpdater(valOrUpdater, state.backtestAction) })),
  runtimePolicyAction: false,
  setRuntimePolicyAction: (valOrUpdater) => set((state) => ({ runtimePolicyAction: resolveUpdater(valOrUpdater, state.runtimePolicyAction) })),
  strategyCreateAction: false,
  setStrategyCreateAction: (valOrUpdater) => set((state) => ({ strategyCreateAction: resolveUpdater(valOrUpdater, state.strategyCreateAction) })),
  strategySaveAction: false,
  setStrategySaveAction: (valOrUpdater) => set((state) => ({ strategySaveAction: resolveUpdater(valOrUpdater, state.strategySaveAction) })),
  sourceFilter: "all",
  setSourceFilter: (valOrUpdater) => set((state) => ({ sourceFilter: resolveUpdater(valOrUpdater, state.sourceFilter) })),
  eventFilter: "all",
  setEventFilter: (valOrUpdater) => set((state) => ({ eventFilter: resolveUpdater(valOrUpdater, state.eventFilter) })),
  timeWindow: "12h",
  setTimeWindow: (valOrUpdater) => set((state) => ({ timeWindow: resolveUpdater(valOrUpdater, state.timeWindow) })),
  focusNonce: 0,
  setFocusNonce: (valOrUpdater) => set((state) => ({ focusNonce: resolveUpdater(valOrUpdater, state.focusNonce) })),
  hoveredMarker: null,
  setHoveredMarker: (valOrUpdater) => set((state) => ({ hoveredMarker: resolveUpdater(valOrUpdater, state.hoveredMarker) })),
  selectedBacktestId: null,
  setSelectedBacktestId: (valOrUpdater) => set((state) => ({ selectedBacktestId: resolveUpdater(valOrUpdater, state.selectedBacktestId) })),
  chartOverrideRange: null,
  setChartOverrideRange: (valOrUpdater) => set((state) => ({ chartOverrideRange: resolveUpdater(valOrUpdater, state.chartOverrideRange) })),
  selectedSample: null,
  setSelectedSample: (valOrUpdater) => set((state) => ({ selectedSample: resolveUpdater(valOrUpdater, state.selectedSample) })),
  backtestForm: { strategyVersionId: "", signalTimeframe: "1d", executionDataSource: "1min", symbol: "BTCUSDT", from: "", to: "", },
  setBacktestForm: (valOrUpdater) => set((state) => ({ backtestForm: resolveUpdater(valOrUpdater, state.backtestForm) })),
  paperForm: { accountId: "", strategyId: "", startEquity: "100000", signalTimeframe: "1d", executionDataSource: "tick", symbol: "BTCUSDT", tradingFeeBps: "10", fundingRateBps: "0", fundingIntervalHours: "8", },
  setPaperForm: (valOrUpdater) => set((state) => ({ paperForm: resolveUpdater(valOrUpdater, state.paperForm) })),
  liveAccountForm: { name: "Binance Testnet", exchange: "binance-futures", },
  setLiveAccountForm: (valOrUpdater) => set((state) => ({ liveAccountForm: resolveUpdater(valOrUpdater, state.liveAccountForm) })),
  liveBindingForm: { accountId: "", adapterKey: "binance-futures", positionMode: "ONE_WAY", marginMode: "CROSSED", sandbox: true, apiKeyRef: "BINANCE_TESTNET_API_KEY", apiSecretRef: "BINANCE_TESTNET_API_SECRET", },
  setLiveBindingForm: (valOrUpdater) => set((state) => ({ liveBindingForm: resolveUpdater(valOrUpdater, state.liveBindingForm) })),
  liveOrderForm: { accountId: "", strategyVersionId: "", symbol: "BTCUSDT", side: "BUY", type: "LIMIT", quantity: "0.001", price: "", },
  setLiveOrderForm: (valOrUpdater) => set((state) => ({ liveOrderForm: resolveUpdater(valOrUpdater, state.liveOrderForm) })),
  liveSessionForm: { accountId: "", strategyId: "", signalTimeframe: "1d", executionDataSource: "tick", symbol: "BTCUSDT", defaultOrderQuantity: "0.001", executionEntryOrderType: "MARKET", executionEntryMaxSpreadBps: "8", executionEntryWideSpreadMode: "limit-maker", executionEntryTimeoutFallbackOrderType: "MARKET", executionPTExitOrderType: "LIMIT", executionPTExitTimeInForce: "GTX", executionPTExitPostOnly: true, executionPTExitTimeoutFallbackOrderType: "MARKET", executionSLExitOrderType: "MARKET", executionSLExitMaxSpreadBps: "999", dispatchMode: "manual-review", dispatchCooldownSeconds: "30", },
  setLiveSessionForm: (valOrUpdater) => set((state) => ({ liveSessionForm: resolveUpdater(valOrUpdater, state.liveSessionForm) })),
  accountSignalForm: { accountId: "", sourceKey: "", role: "trigger", symbol: "BTCUSDT", timeframe: "1d", },
  setAccountSignalForm: (valOrUpdater) => set((state) => ({ accountSignalForm: resolveUpdater(valOrUpdater, state.accountSignalForm) })),
  strategySignalForm: { strategyId: "", sourceKey: "", role: "trigger", symbol: "BTCUSDT", timeframe: "1d", },
  setStrategySignalForm: (valOrUpdater) => set((state) => ({ strategySignalForm: resolveUpdater(valOrUpdater, state.strategySignalForm) })),
  strategyCreateForm: { name: "", description: "", },
  setStrategyCreateForm: (valOrUpdater) => set((state) => ({ strategyCreateForm: resolveUpdater(valOrUpdater, state.strategyCreateForm) })),
  strategyEditorForm: { strategyId: "", strategyEngine: "bk-default", signalTimeframe: "1d", executionDataSource: "tick", parametersJson: "{}", },
  setStrategyEditorForm: (valOrUpdater) => set((state) => ({ strategyEditorForm: resolveUpdater(valOrUpdater, state.strategyEditorForm) })),
  signalRuntimeForm: { accountId: "", strategyId: "", },
  setSignalRuntimeForm: (valOrUpdater) => set((state) => ({ signalRuntimeForm: resolveUpdater(valOrUpdater, state.signalRuntimeForm) })),
  runtimePolicyForm: { tradeTickFreshnessSeconds: "15", orderBookFreshnessSeconds: "10", signalBarFreshnessSeconds: "30", runtimeQuietSeconds: "30", paperStartReadinessTimeoutSeconds: "5", },
  setRuntimePolicyForm: (valOrUpdater) => set((state) => ({ runtimePolicyForm: resolveUpdater(valOrUpdater, state.runtimePolicyForm) })),
  telegramForm: { enabled: false, botToken: "", chatId: "", sendLevels: "critical,warning", },
  setTelegramForm: (valOrUpdater) => set((state) => ({ telegramForm: resolveUpdater(valOrUpdater, state.telegramForm) })),
  liveAccountError: null,
  setLiveAccountError: (valOrUpdater) => set((state) => ({ liveAccountError: resolveUpdater(valOrUpdater, state.liveAccountError) })),
  liveBindingError: null,
  setLiveBindingError: (valOrUpdater) => set((state) => ({ liveBindingError: resolveUpdater(valOrUpdater, state.liveBindingError) })),
  liveSessionError: null,
  setLiveSessionError: (valOrUpdater) => set((state) => ({ liveSessionError: resolveUpdater(valOrUpdater, state.liveSessionError) })),
  liveAccountNotice: null,
  setLiveAccountNotice: (valOrUpdater) => set((state) => ({ liveAccountNotice: resolveUpdater(valOrUpdater, state.liveAccountNotice) })),
  liveBindingNotice: null,
  setLiveBindingNotice: (valOrUpdater) => set((state) => ({ liveBindingNotice: resolveUpdater(valOrUpdater, state.liveBindingNotice) })),
  liveSessionNotice: null,
  setLiveSessionNotice: (valOrUpdater) => set((state) => ({ liveSessionNotice: resolveUpdater(valOrUpdater, state.liveSessionNotice) })),
  settingsMenuOpen: false,
  setSettingsMenuOpen: (valOrUpdater) => set((state) => ({ settingsMenuOpen: resolveUpdater(valOrUpdater, state.settingsMenuOpen) })),
  activeSettingsModal: null,
  setActiveSettingsModal: (valOrUpdater) => set((state) => ({ activeSettingsModal: resolveUpdater(valOrUpdater, state.activeSettingsModal) })),
}));
