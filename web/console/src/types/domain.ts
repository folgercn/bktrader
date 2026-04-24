export type AccountSummary = {
  accountId: string;
  accountName: string;
  mode: string;
  exchange: string;
  status: string;
  startEquity: number;
  realizedPnl: number;
  unrealizedPnl: number;
  fees: number;
  netEquity: number;
  walletBalance: number;
  marginBalance: number;
  availableBalance: number;
  exposureNotional: number;
  openPositionCount: number;
  updatedAt: string;
};

export type AccountRecord = {
  id: string;
  name: string;
  mode: string;
  exchange: string;
  status: string;
  metadata?: Record<string, unknown>;
  bindings?: any;
  createdAt: string;
};

export type StrategyVersion = {
  id: string;
  strategyId: string;
  version: string;
  signalTimeframe: string;
  executionTimeframe: string;
  parameters?: Record<string, unknown>;
  createdAt: string;
};

export type StrategyRecord = {
  id: string;
  name: string;
  status: string;
  description: string;
  createdAt: string;
  currentVersion?: StrategyVersion;
};

export type AccountEquitySnapshot = {
  id: string;
  accountId: string;
  startEquity: number;
  realizedPnl: number;
  unrealizedPnl: number;
  fees: number;
  netEquity: number;
  exposureNotional: number;
  openPositionCount: number;
  createdAt: string;
};

export type Order = {
  id: string;
  accountId: string;
  strategyVersionId?: string;
  symbol: string;
  side: string;
  type: string;
  status: string;
  quantity: number;
  price: number;
  reduceOnly?: boolean;
  closePosition?: boolean;
  metadata?: Record<string, unknown>;
  bindings?: any;
  createdAt: string;
};

export type Fill = {
  id: string;
  orderId: string;
  strategyVersion?: string;
  symbol?: string;
  side?: string;
  price: number;
  quantity: number;
  fee: number;
  exchangeTradeId?: string;
  exchangeTradeTime?: string;
  createdAt: string;
};

export type Position = {
  id: string;
  accountId: string;
  symbol: string;
  side: string;
  quantity: number;
  entryPrice: number;
  markPrice: number;
  updatedAt: string;
};

export type PaperSession = {
  id: string;
  accountId: string;
  strategyId: string;
  status: string;
  startEquity: number;
  state?: Record<string, unknown>;
  createdAt: string;
};

export type LiveSession = {
  id: string;
  alias: string;
  accountId: string;
  strategyId: string;
  status: string;
  state?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
  createdAt: string;
};

export type ChartCandle = {
  symbol: string;
  resolution: string;
  time: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

export type ChartAnnotation = {
  id: string;
  source: string;
  type: string;
  symbol: string;
  time: string;
  price: number;
  label: string;
  metadata?: Record<string, unknown>;
  bindings?: any;
};

export type LaunchTemplateStep = {
  label: string;
  pathTemplate: string;
  method: string;
  payloadRef: string;
};

export type LaunchTemplate = {
  key: string;
  name: string;
  description: string;
  strategyId: string;
  symbol: string;
  signalTimeframe: string;
  steps: LaunchTemplateStep[];
  [key: string]: any; // 支持 payloadRef 引用的动态属性
};

export type MarkerLegendItem = {
  label: string;
  color: string;
};

export type BacktestRun = {
  id: string;
  strategyVersionId: string;
  status: string;
  parameters?: Record<string, unknown>;
  resultSummary?: Record<string, unknown>;
  createdAt: string;
};

export type BacktestOptions = {
  signalTimeframes: string[];
  executionDataSources: string[];
  defaultSignalTimeframe: string;
  defaultExecutionDataSource: string;
  dataDirectories?: Record<string, string>;
  availability?: Record<string, string>;
  datasets?: Record<string, Array<{ name: string; path: string; symbol?: string; format?: string; fileCount?: number; timeColumn?: string }>>;
  supportedSymbols?: Record<string, string[]>;
  schema?: Record<string, { requiredColumns?: string[]; optionalColumns?: string[]; filenameExamples?: string[] }>;
  notes: string[];
};

export type LiveAdapter = {
  key: string;
  name: string;
  environments?: string[];
  positionModes?: string[];
  marginModes?: string[];
  feeSource?: string;
  fundingSource?: string;
};

export type SignalSourceDefinition = {
  key: string;
  name: string;
  exchange: string;
  streamType: string;
  transport: string;
  status: string;
  roles: string[];
  environments: string[];
  symbolScope: string;
  description: string;
  metadata?: Record<string, unknown>;
  bindings?: any;
};

export type SignalSourceCatalog = {
  sources: SignalSourceDefinition[];
  notes: string[];
  byEnvironment?: Record<string, SignalSourceDefinition[]>;
};

export type SignalSourceType = {
  streamType: string;
  primaryRole: string;
  description: string;
  typicalInputs?: string[];
};

export type SignalBinding = {
  id: string;
  accountId?: string;
  strategyId?: string;
  sourceKey: string;
  sourceName: string;
  exchange: string;
  role: string;
  streamType: string;
  symbol: string;
  timeframe?: string;
  status: string;
  options?: Record<string, unknown>;
  createdAt: string;
};

export type PlatformHealthAlertCounts = {
  total: number;
  critical: number;
  warning: number;
  info: number;
};

export type PlatformHealthAccountSnapshot = {
  id: string;
  name: string;
  exchange: string;
  status: string;
  lastLiveSyncAt?: string;
  syncAgeSeconds: number;
  syncStale: boolean;
  runtimeSessionCount: number;
  runningLiveSessionCount: number;
  accountSync?: Record<string, unknown>;
};

export type PlatformHealthRuntimeSessionSnapshot = {
  id: string;
  accountId: string;
  strategyId: string;
  strategyName?: string;
  status: string;
  transport: string;
  health: string;
  lastEventAt?: string;
  lastHeartbeatAt?: string;
  quiet: boolean;
  tradeTick?: Record<string, unknown>;
  orderBook?: Record<string, unknown>;
};

export type PlatformHealthStrategySessionSnapshot = {
  id: string;
  mode: string;
  accountId: string;
  strategyId: string;
  strategyName?: string;
  status: string;
  runtimeSessionId?: string;
  lastSignalRuntimeEventAt?: string;
  lastStrategyEvaluationAt?: string;
  lastStrategyEvaluationStatus?: string;
  lastSyncedOrderStatus?: string;
  evaluationQuiet: boolean;
  strategyIngress?: Record<string, unknown>;
  execution?: Record<string, unknown>;
  sourceGate?: Record<string, unknown>;
};

export type PlatformHealthSnapshot = {
  generatedAt: string;
  status: string;
  alertCounts: PlatformHealthAlertCounts;
  runtimePolicy: RuntimePolicy;
  liveAccounts: PlatformHealthAccountSnapshot[];
  runtimeSessions: PlatformHealthRuntimeSessionSnapshot[];
  liveSessions: PlatformHealthStrategySessionSnapshot[];
  paperSessions: PlatformHealthStrategySessionSnapshot[];
};

export type SignalRuntimeAdapter = {
  key: string;
  name: string;
  transport?: string;
  environments?: string[];
  streamTypes?: string[];
};

export type SignalRuntimeSession = {
  id: string;
  accountId: string;
  strategyId: string;
  status: string;
  runtimeAdapter: string;
  transport: string;
  subscriptionCount: number;
  state?: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
};

export type ReplayReasonStats = Record<string, Record<string, number>>;

export type ReplaySample = Record<string, unknown>;

export type ExecutionTrade = Record<string, unknown>;

export type SourceFilter = "all" | "paper" | "backtest" | "live";

export type EventFilter = "all" | "initial" | "reentry" | "pt" | "sl";

export type TimeWindow = "6h" | "12h" | "1d" | "3d";

export type MarkerDetail = {
  id: string;
  source: string;
  type: string;
  label: string;
  time: string;
  price: number;
  reason?: string;
  paperSession?: string;
  liveSession?: string;
};

export type ChartOverrideRange = {
  from: number;
  to: number;
  label: string;
};

export type SelectedSample = {
  key: string;
  sample: ReplaySample;
};

export type SelectableSample = SelectedSample & {
  group: "completed" | "skipped";
};

export type RuntimeMarketSnapshot = {
  tradePrice?: number;
  bestBid?: number;
  bestAsk?: number;
  spreadBps?: number;
};

export type RuntimeSourceSummary = {
  tradeTickCount: number;
  orderBookCount: number;
  staleCount: number;
  latestEventAt?: string;
};

export type RuntimeReadiness = {
  ready: boolean;
  status: "ready" | "warning" | "blocked";
  reason: string;
};

export type SignalBarCandle = {
  time: string;
  open: number;
  high: number;
  low: number;
  close: number;
  timeframe: string;
  isClosed: boolean;
};

export type AlertItem = {
  level: "critical" | "warning" | "info";
  title: string;
  detail: string;
};

export type PlatformAlert = {
  id: string;
  scope: "paper" | "live" | "runtime" | string;
  level: "critical" | "warning" | "info" | string;
  title: string;
  detail: string;
  accountId?: string;
  accountName?: string;
  strategyId?: string;
  strategyName?: string;
  paperSessionId?: string;
  runtimeSessionId?: string;
  anchor?: string;
  eventTime?: string;
  metadata?: Record<string, unknown>;
  bindings?: any;
};

export type PlatformNotification = {
  id: string;
  status: "active" | "acked" | string;
  ackedAt?: string;
  alert: PlatformAlert;
  metadata?: Record<string, unknown> & {
    telegramStatus?: string;
    telegramSentAt?: string;
    telegramAttemptedAt?: string;
    telegramLastError?: string;
  };
  updatedAt: string;
};

export type TelegramConfig = {
  enabled: boolean;
  chatId: string;
  sendLevels: string[];
  tradeEventsEnabled: boolean;
  positionReportEnabled: boolean;
  positionReportIntervalMinutes: number;
  hasBotToken: boolean;
  maskedBotToken: string;
  updatedAt?: string;
};

export type RuntimePolicy = {
  tradeTickFreshnessSeconds: number;
  orderBookFreshnessSeconds: number;
  signalBarFreshnessSeconds: number;
  runtimeQuietSeconds: number;
  strategyEvaluationQuietSeconds: number;
  liveAccountSyncFreshnessSeconds: number;
  paperStartReadinessTimeoutSeconds: number;
  dispatchMode?: string;
};

export type LivePreflightSummary = {
  status: "ready" | "watch" | "blocked";
  reason: string;
  detail: string;
};

export type LiveNextAction = {
  key: string;
  label: string;
  detail: string;
};

export type LiveDispatchPreview = {
  status: "ready" | "watch" | "blocked";
  reason: string;
  detail: string;
  payload: Record<string, unknown>;
};

export type LiveSessionExecutionSummary = {
  orderCount: number;
  fillCount: number;
  latestOrder: Order | null;
  latestFill: Fill | null;
  position: Position | null;
};

export type LiveTradePair = {
  id: string;
  liveSessionId: string;
  accountId: string;
  strategyId: string;
  symbol: string;
  status: string;
  side: string;
  entryOrderIds: string[];
  exitOrderIds: string[];
  entryAt: string;
  exitAt?: string;
  entryAvgPrice: number;
  exitAvgPrice: number;
  entryQuantity: number;
  exitQuantity: number;
  openQuantity: number;
  entryReason?: string;
  exitReason?: string;
  exitClassifier?: string;
  exitVerdict: string;
  realizedPnl: number;
  unrealizedPnl: number;
  fees: number;
  netPnl: number;
  entryFillCount: number;
  exitFillCount: number;
  notes?: string[];
};

export type LiveSessionHealth = {
  status: "ready" | "active" | "waiting-sync" | "error" | "idle" | "neutral";
  detail: string;
};

export type HighlightedLiveSession = {
  session: LiveSession;
  health: LiveSessionHealth;
  execution: LiveSessionExecutionSummary;
} | null;

export type LiveSessionFlowStep = {
  key: string;
  label: string;
  status: "ready" | "watch" | "blocked" | "neutral";
  detail: string;
};

export type SessionMarker = {
  time: string;
  position: "aboveBar" | "belowBar";
  color: string;
  shape: "arrowUp" | "arrowDown" | "circle" | "square";
  text: string;
};

export type SignalMonitorOverlay = {
  startTime: string;
  endTime: string;
  price: number;
  color: string;
  lineStyle: "solid" | "dashed" | "dotted";
};

export type AuthSession = {
  token: string;
  username: string;
  expiresAt?: string;
};

export type ActiveSettingsModal = "telegram" | "live-account" | "live-binding" | "live-session" | null;

// ─── Form Types ──────────────────────────────────────────

export interface LoginForm {
  username: string;
  password: string;
}

export interface BacktestForm {
  strategyVersionId: string;
  signalTimeframe: string;
  executionDataSource: string;
  symbol: string;
  from: string;
  to: string;
}

export interface PaperForm {
  accountId: string;
  strategyId: string;
  startEquity: string;
  signalTimeframe: string;
  executionDataSource: string;
  symbol: string;
  tradingFeeBps: string;
  fundingRateBps: string;
  fundingIntervalHours: string;
}

export interface LiveAccountForm {
  name: string;
  exchange: string;
}

export interface LiveBindingForm {
  accountId: string;
  adapterKey: string;
  positionMode: string;
  marginMode: string;
  sandbox: boolean;
  apiKeyRef: string;
  apiSecretRef: string;
}

export interface LiveOrderForm {
  accountId: string;
  strategyVersionId: string;
  symbol: string;
  side: string;
  type: string;
  quantity: string;
  price: string;
}

export interface LiveSessionForm {
  alias: string;
  accountId: string;
  strategyId: string;
  signalTimeframe: string;
  executionDataSource: string;
  symbol: string;
  positionSizingMode: string;
  defaultOrderQuantity: string;
  reentrySizeScheduleFirst: string;
  reentrySizeScheduleSecond: string;
  executionEntryOrderType: string;
  executionEntryMaxSpreadBps: string;
  executionEntryWideSpreadMode: string;
  executionEntryTimeoutFallbackOrderType: string;
  executionPTExitOrderType: string;
  executionPTExitTimeInForce: string;
  executionPTExitPostOnly: boolean;
  executionPTExitTimeoutFallbackOrderType: string;
  executionSLExitOrderType: string;
  executionSLExitMaxSpreadBps: string;
  dispatchMode: string;
  dispatchCooldownSeconds: string;
  freshnessOverrideSignalBarFreshnessSeconds?: string;
  freshnessOverrideTradeTickFreshnessSeconds?: string;
  freshnessOverrideOrderBookFreshnessSeconds?: string;
  freshnessOverrideRuntimeQuietSeconds?: string;
}

export interface AccountSignalForm {
  accountId: string;
  sourceKey: string;
  role: string;
  symbol: string;
  timeframe: string;
}

export interface StrategySignalForm {
  strategyId: string;
  sourceKey: string;
  role: string;
  symbol: string;
  timeframe: string;
}

export interface StrategyCreateForm {
  name: string;
  description: string;
}

export interface StrategyEditorForm {
  strategyId: string;
  strategyEngine: string;
  signalTimeframe: string;
  executionDataSource: string;
  parametersJson: string;
}

export interface SignalRuntimeForm {
  accountId: string;
  strategyId: string;
}

export interface RuntimePolicyForm {
  tradeTickFreshnessSeconds: string;
  orderBookFreshnessSeconds: string;
  signalBarFreshnessSeconds: string;
  runtimeQuietSeconds: string;
  strategyEvaluationQuietSeconds: string;
  liveAccountSyncFreshnessSeconds: string;
  paperStartReadinessTimeoutSeconds: string;
  dispatchMode: string;
}

export interface TelegramForm {
  enabled: boolean;
  botToken: string;
  chatId: string;
  sendLevels: string;
  tradeEventsEnabled: boolean;
  positionReportEnabled: boolean;
  positionReportIntervalMinutes: string;
}

export interface LiveLaunchResult {
  liveSessionId: string;
  runtimeSessionId: string | null;
  templateApplied?: boolean;
  templateBindingCount?: number;
  runtimePlanRefreshed?: boolean;
  stoppedLiveSessions?: number;
}

export type TimelineConfig = {
  deduplicationEnabled: boolean;
  quietSeconds: number;
  maxRepeats: number;
};
