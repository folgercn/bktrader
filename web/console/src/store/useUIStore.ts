import { create } from 'zustand';
import { toast } from 'sonner';
import { 
  AccountSummary, AccountRecord, Order, Fill, Position, AccountEquitySnapshot, StrategyRecord, BacktestRun, BacktestOptions, PaperSession, LiveSession, LiveAdapter, SignalSourceCatalog, SignalSourceType, SignalRuntimeAdapter, SignalRuntimeSession, RuntimePolicy, PlatformAlert, PlatformNotification, TelegramConfig, SignalBinding, ChartCandle, ChartAnnotation, MarkerDetail, ChartOverrideRange, SelectedSample, SourceFilter, EventFilter, TimeWindow, AuthSession,
  LoginForm, BacktestForm, PaperForm, LiveAccountForm, LiveBindingForm, LiveOrderForm, LiveSessionForm, StrategySignalForm, StrategyCreateForm, StrategyEditorForm, SignalRuntimeForm, RuntimePolicyForm, TelegramForm,
  TimelineConfig
} from '../types/domain';

import { readStoredAuthSession } from '../utils/auth';
import { resolveUpdater } from './helpers';

type SidebarTab = "monitor" | "strategy" | "account" | "log" | "recovery";
type DockTab = "pairs" | "orders" | "positions" | "fills" | "alerts";

export type SystemLogEntry = {
  id: string;
  level: "error" | "info";
  message: string;
  createdAt: string;
};

export type ConfirmDialogConfig = {
  isOpen: boolean;
  title: string;
  description: string;
  onConfirm: () => Promise<void> | void;
};

const CONSOLE_NAV_STORAGE_KEY = "bktrader-console-nav";
const SYSTEM_LOGS_STORAGE_KEY = "bktrader-system-logs";
const DEFAULT_SIDEBAR_TAB: SidebarTab = "monitor";
const DEFAULT_DOCK_TAB: DockTab = "pairs";
const TIMELINE_CONFIG_STORAGE_KEY = "bktrader-timeline-config";


function readStoredSystemLogs(): SystemLogEntry[] {
  if (typeof window === "undefined") {
    return [];
  }
  try {
    const raw = window.localStorage.getItem(SYSTEM_LOGS_STORAGE_KEY);
    if (!raw) return [];
    return JSON.parse(raw);
  } catch {
    return [];
  }
}

function writeStoredSystemLogs(logs: SystemLogEntry[]) {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(SYSTEM_LOGS_STORAGE_KEY, JSON.stringify(logs));
}

function readStoredConsoleNav(): { sidebarTab: SidebarTab; dockTab: DockTab } {
  if (typeof window === "undefined") {
    return { sidebarTab: DEFAULT_SIDEBAR_TAB, dockTab: DEFAULT_DOCK_TAB };
  }

  try {
    const raw = window.localStorage.getItem(CONSOLE_NAV_STORAGE_KEY);
    if (!raw) {
      return { sidebarTab: DEFAULT_SIDEBAR_TAB, dockTab: DEFAULT_DOCK_TAB };
    }

    const parsed = JSON.parse(raw) as Partial<{ sidebarTab: SidebarTab; dockTab: DockTab }>;
    const sidebarTab = parsed.sidebarTab === "strategy" || parsed.sidebarTab === "account" || parsed.sidebarTab === "monitor" || parsed.sidebarTab === "log" || parsed.sidebarTab === "recovery"
      ? parsed.sidebarTab
      : DEFAULT_SIDEBAR_TAB;
    const dockTab = parsed.dockTab === "positions" || parsed.dockTab === "fills" || parsed.dockTab === "alerts" || parsed.dockTab === "orders" || parsed.dockTab === "pairs"
      ? parsed.dockTab
      : DEFAULT_DOCK_TAB;

    return { sidebarTab, dockTab };
  } catch {
    return { sidebarTab: DEFAULT_SIDEBAR_TAB, dockTab: DEFAULT_DOCK_TAB };
  }
}

function writeStoredConsoleNav(partial: Partial<{ sidebarTab: SidebarTab; dockTab: DockTab }>) {
  if (typeof window === "undefined") {
    return;
  }

  const current = readStoredConsoleNav();
  window.localStorage.setItem(CONSOLE_NAV_STORAGE_KEY, JSON.stringify({
    ...current,
    ...partial,
  }));
}

const DEFAULT_TIMELINE_CONFIG: TimelineConfig = {

  deduplicationEnabled: true,
  quietSeconds: 60,
  maxRepeats: 1,
};

function readStoredTimelineConfig(): TimelineConfig {
  if (typeof window === "undefined") {
    return DEFAULT_TIMELINE_CONFIG;
  }
  try {
    const raw = window.localStorage.getItem(TIMELINE_CONFIG_STORAGE_KEY);
    if (!raw) return DEFAULT_TIMELINE_CONFIG;
    return { ...DEFAULT_TIMELINE_CONFIG, ...JSON.parse(raw) };
  } catch {
    return DEFAULT_TIMELINE_CONFIG;
  }
}

function writeStoredTimelineConfig(config: TimelineConfig) {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(TIMELINE_CONFIG_STORAGE_KEY, JSON.stringify(config));
}

const initialConsoleNav = readStoredConsoleNav();
const initialTimelineConfig = readStoredTimelineConfig();


export interface useUIStoreState {
  sidebarTab: SidebarTab;
  setSidebarTab: (val: SidebarTab) => void;
  dockTab: DockTab;
  setDockTab: (val: DockTab) => void;
  loading: boolean;
  setLoading: (valOrUpdater: boolean | ((prev: boolean) => boolean)) => void;
  error: string | null;
  setError: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  systemLogs: SystemLogEntry[];
  clearSystemLogs: () => void;
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
  launchingTemplate: string | null;
  setLaunchingTemplate: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  settingsMenuOpen: boolean;
  setSettingsMenuOpen: (valOrUpdater: boolean | ((prev: boolean) => boolean)) => void;
  activeSettingsModal: "telegram" | "live-account" | "live-binding" | "live-session" | null;
  setActiveSettingsModal: (valOrUpdater: "telegram" | "live-account" | "live-binding" | "live-session" | null | ((prev: "telegram" | "live-account" | "live-binding" | "live-session" | null) => "telegram" | "live-account" | "live-binding" | "live-session" | null)) => void;
  monitorResolution: string | null;
  setMonitorResolution: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  notification: { type: 'success' | 'error' | 'info'; message: string } | null;
  setNotification: (valOrUpdater: { type: 'success' | 'error' | 'info'; message: string } | null | ((prev: { type: 'success' | 'error' | 'info'; message: string } | null) => { type: 'success' | 'error' | 'info'; message: string } | null)) => void;
  positionCloseAction: string | null;
  setPositionCloseAction: (valOrUpdater: string | null | ((prev: string | null) => string | null)) => void;
  confirmDialogConfig: ConfirmDialogConfig;
  setConfirmDialogConfig: (valOrUpdater: ConfirmDialogConfig | ((prev: ConfirmDialogConfig) => ConfirmDialogConfig)) => void;
  openConfirmDialog: (title: string, description: string, onConfirm: () => Promise<void> | void) => void;
  closeConfirmDialog: () => void;
  timelineConfig: TimelineConfig;
  setTimelineConfig: (valOrUpdater: TimelineConfig | ((prev: TimelineConfig) => TimelineConfig)) => void;
}


export const useUIStore = create<useUIStoreState>((set) => ({
  sidebarTab: initialConsoleNav.sidebarTab,
  setSidebarTab: (val) => {
    writeStoredConsoleNav({ sidebarTab: val });
    set({ sidebarTab: val });
  },
  dockTab: initialConsoleNav.dockTab,
  setDockTab: (val) => {
    writeStoredConsoleNav({ dockTab: val });
    set({ dockTab: val });
  },
  loading: true,
  setLoading: (valOrUpdater) => set((state) => ({ loading: resolveUpdater(valOrUpdater, state.loading) })),
  error: null,
  setError: (valOrUpdater) => set((state) => {
    const nextError = resolveUpdater(valOrUpdater, state.error);
    if (nextError === state.error) {
      return { error: nextError };
    }

    const nextLogs = [...state.systemLogs];
    if (nextError) {
      nextLogs.unshift({
        id: `error:${Date.now()}:${Math.random().toString(36).slice(2, 8)}`,
        level: "error",
        message: nextError,
        createdAt: new Date().toISOString(),
      });
    } else if (state.error) {
      nextLogs.unshift({
        id: `info:${Date.now()}:${Math.random().toString(36).slice(2, 8)}`,
        level: "info",
        message: "连接已恢复正常",
        createdAt: new Date().toISOString(),
      });
    }

    const finalLogs = nextLogs.slice(0, 40);
    writeStoredSystemLogs(finalLogs);

    return {
      error: nextError,
      systemLogs: finalLogs,
    };
  }),
  systemLogs: readStoredSystemLogs(),
  clearSystemLogs: () => {
    if (typeof window !== "undefined") {
      window.localStorage.removeItem(SYSTEM_LOGS_STORAGE_KEY);
    }
    set({ systemLogs: [] });
  },
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
  liveSessionForm: { alias: "", accountId: "", strategyId: "", signalTimeframe: "1d", executionDataSource: "tick", symbol: "BTCUSDT", positionSizingMode: "fixed_quantity", defaultOrderQuantity: "0.001", reentrySizeScheduleFirst: "0.20", reentrySizeScheduleSecond: "0.10", executionEntryOrderType: "MARKET", executionEntryMaxSpreadBps: "8", executionEntryWideSpreadMode: "limit-maker", executionEntryTimeoutFallbackOrderType: "MARKET", executionPTExitOrderType: "LIMIT", executionPTExitTimeInForce: "GTX", executionPTExitPostOnly: true, executionPTExitTimeoutFallbackOrderType: "MARKET", executionSLExitOrderType: "MARKET", executionSLExitMaxSpreadBps: "999", dispatchMode: "manual-review", dispatchCooldownSeconds: "30", freshnessOverrideSignalBarFreshnessSeconds: "", freshnessOverrideTradeTickFreshnessSeconds: "", freshnessOverrideOrderBookFreshnessSeconds: "", freshnessOverrideRuntimeQuietSeconds: "", },
  setLiveSessionForm: (valOrUpdater) => set((state) => ({ liveSessionForm: resolveUpdater(valOrUpdater, state.liveSessionForm) })),
  strategySignalForm: { strategyId: "", sourceKey: "", role: "trigger", symbol: "BTCUSDT", timeframe: "1d", },
  setStrategySignalForm: (valOrUpdater) => set((state) => ({ strategySignalForm: resolveUpdater(valOrUpdater, state.strategySignalForm) })),
  strategyCreateForm: { name: "", description: "", },
  setStrategyCreateForm: (valOrUpdater) => set((state) => ({ strategyCreateForm: resolveUpdater(valOrUpdater, state.strategyCreateForm) })),
  strategyEditorForm: { strategyId: "", strategyEngine: "bk-default", signalTimeframe: "1d", executionDataSource: "tick", parametersJson: "{}", },
  setStrategyEditorForm: (valOrUpdater) => set((state) => ({ strategyEditorForm: resolveUpdater(valOrUpdater, state.strategyEditorForm) })),
  signalRuntimeForm: { accountId: "", strategyId: "", },
  setSignalRuntimeForm: (valOrUpdater) => set((state) => ({ signalRuntimeForm: resolveUpdater(valOrUpdater, state.signalRuntimeForm) })),
  runtimePolicyForm: {
    tradeTickFreshnessSeconds: "15",
    orderBookFreshnessSeconds: "10",
    signalBarFreshnessSeconds: "30",
    runtimeQuietSeconds: "30",
    strategyEvaluationQuietSeconds: "0",
    liveAccountSyncFreshnessSeconds: "0",
    paperStartReadinessTimeoutSeconds: "5",
    dispatchMode: "manual-review",
  },
  setRuntimePolicyForm: (valOrUpdater) => set((state) => ({ runtimePolicyForm: resolveUpdater(valOrUpdater, state.runtimePolicyForm) })),
  telegramForm: { enabled: false, botToken: "", chatId: "", sendLevels: "critical,warning", tradeEventsEnabled: true, positionReportEnabled: true, positionReportIntervalMinutes: "30", },
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
  launchingTemplate: null,
  setLaunchingTemplate: (valOrUpdater) => set((state) => ({ launchingTemplate: resolveUpdater(valOrUpdater, state.launchingTemplate) })),
  settingsMenuOpen: false,
  setSettingsMenuOpen: (valOrUpdater) => set((state) => ({ settingsMenuOpen: resolveUpdater(valOrUpdater, state.settingsMenuOpen) })),
  activeSettingsModal: null,
  setActiveSettingsModal: (valOrUpdater) => set((state) => ({ activeSettingsModal: resolveUpdater(valOrUpdater, state.activeSettingsModal) })),
  monitorResolution: null,
  setMonitorResolution: (valOrUpdater) => set((state) => ({ monitorResolution: resolveUpdater(valOrUpdater, state.monitorResolution) })),
  notification: null,
  setNotification: (valOrUpdater) => set((state) => {
    const next = resolveUpdater(valOrUpdater, state.notification);
    if (next) {
      if (next.type === 'success') toast.success(next.message);
      else if (next.type === 'error') toast.error(next.message);
      else toast.info(next.message);
    }
    return { notification: next };
  }),
  positionCloseAction: null,
  setPositionCloseAction: (valOrUpdater) => set((state) => ({ positionCloseAction: resolveUpdater(valOrUpdater, state.positionCloseAction) })),
  confirmDialogConfig: { isOpen: false, title: '', description: '', onConfirm: () => {} },
  setConfirmDialogConfig: (valOrUpdater) => set((state) => ({ confirmDialogConfig: resolveUpdater(valOrUpdater, state.confirmDialogConfig) })),
  openConfirmDialog: (title, description, onConfirm) => set(() => ({ confirmDialogConfig: { isOpen: true, title, description, onConfirm } })),
  closeConfirmDialog: () => set((state) => ({ confirmDialogConfig: { ...state.confirmDialogConfig, isOpen: false } })),
  timelineConfig: initialTimelineConfig,
  setTimelineConfig: (valOrUpdater) => set((state) => {
    const next = resolveUpdater(valOrUpdater, state.timelineConfig);
    writeStoredTimelineConfig(next);
    return { timelineConfig: next };
  }),
}));
