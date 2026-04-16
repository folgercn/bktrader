import { useEffect } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { fetchJSON, API_BASE } from '../utils/api';
import { writeStoredAuthSession } from '../utils/auth';
import { 
  AccountSummary, AccountRecord, Order, Fill, Position, PaperSession, LiveSession, 
  StrategyRecord, BacktestRun, BacktestOptions, LiveAdapter, SignalSourceCatalog, 
  SignalSourceType, SignalRuntimeAdapter, SignalRuntimeSession, RuntimePolicy, 
  PlatformAlert, PlatformNotification, TelegramConfig, AccountEquitySnapshot, PlatformHealthSnapshot,
  ChartCandle, ChartAnnotation, SignalBinding 
} from '../types/domain';
import { 
  resolveChartAnchor, buildTimeRange, strategyLabel, getRecord, getList, 
  deriveRuntimeMarketSnapshot, summarizeOrderPreflight 
} from '../utils/derivation';

export function useDashboard() {
  // UI State seters
  const setError = useUIStore(s => s.setError);
  const setLoading = useUIStore(s => s.setLoading);
  const setAuthSession = useUIStore(s => s.setAuthSession);
  const setTelegramForm = useUIStore(s => s.setTelegramForm);
  const setRuntimePolicyForm = useUIStore(s => s.setRuntimePolicyForm);
  const activeSettingsModal = useUIStore(s => s.activeSettingsModal);
  const authSession = useUIStore(s => s.authSession);
  const timeWindow = useUIStore(s => s.timeWindow);
  const chartOverrideRange = useUIStore(s => s.chartOverrideRange);
  const monitorResolution = useUIStore(s => s.monitorResolution);
  const liveOrderForm = useUIStore(s => s.liveOrderForm);
  const setSelectedBacktestId = useUIStore(s => s.setSelectedBacktestId);
  const setBacktestForm = useUIStore(s => s.setBacktestForm);

  // Trading State setters
  const setSummaries = useTradingStore(s => s.setSummaries);
  const setAccounts = useTradingStore(s => s.setAccounts);
  const setOrders = useTradingStore(s => s.setOrders);
  const setFills = useTradingStore(s => s.setFills);
  const setPositions = useTradingStore(s => s.setPositions);
  const setSnapshots = useTradingStore(s => s.setSnapshots);
  const setMonitorCandles = useTradingStore(s => s.setMonitorCandles);
  const setStrategies = useTradingStore(s => s.setStrategies);
  const setSelectedStrategyId = useTradingStore(s => s.setSelectedStrategyId);
  const setBacktests = useTradingStore(s => s.setBacktests);
  const setBacktestOptions = useTradingStore(s => s.setBacktestOptions);
  const setPaperSessions = useTradingStore(s => s.setPaperSessions);
  const setLiveSessions = useTradingStore(s => s.setLiveSessions);
  const setLiveAdapters = useTradingStore(s => s.setLiveAdapters);
  const setSignalCatalog = useTradingStore(s => s.setSignalCatalog);
  const setSignalSourceTypes = useTradingStore(s => s.setSignalSourceTypes);
  const setSignalRuntimeAdapters = useTradingStore(s => s.setSignalRuntimeAdapters);
  const setSignalRuntimeSessions = useTradingStore(s => s.setSignalRuntimeSessions);
  const setRuntimePolicy = useTradingStore(s => s.setRuntimePolicy);
  const setMonitorHealth = useTradingStore(s => s.setMonitorHealth);
  const setAlerts = useTradingStore(s => s.setAlerts);
  const setNotifications = useTradingStore(s => s.setNotifications);
  const setTelegramConfig = useTradingStore(s => s.setTelegramConfig);
  const setStrategySignalBindingMap = useTradingStore(s => s.setStrategySignalBindingMap);
  const setStrategySignalBindings = useTradingStore(s => s.setStrategySignalBindings);
  const setSignalRuntimePlan = useTradingStore(s => s.setSignalRuntimePlan);
  const setSelectedSignalRuntimeId = useTradingStore(s => s.setSelectedSignalRuntimeId);
  const setCandles = useTradingStore(s => s.setCandles);
  const setAnnotations = useTradingStore(s => s.setAnnotations);
  const setLaunchTemplates = useTradingStore(s => s.setLaunchTemplates);

  async function loadDashboard() {
    const [
      summaryData,
      accountData,
      ordersData,
      fillsData,
      positionsData,
      paperSessionData,
      liveSessionData,
      strategyData,
      backtestData,
      backtestOptionsData,
      liveAdapterData,
      signalCatalogData,
      signalSourceTypeData,
      signalRuntimeAdapterData,
      signalRuntimeSessionData,
      runtimePolicyData,
      monitorHealthData,
      alertData,
      notificationData,
      telegramConfigData,
      launchTemplateData,
    ] = await Promise.all([
      fetchJSON<AccountSummary[]>("/api/v1/account-summaries"),
      fetchJSON<AccountRecord[]>("/api/v1/accounts"),
      fetchJSON<Order[]>("/api/v1/orders"),
      fetchJSON<Fill[]>("/api/v1/fills"),
      fetchJSON<Position[]>("/api/v1/positions"),
      Promise.resolve([] as PaperSession[]),
      fetchJSON<LiveSession[]>("/api/v1/live/sessions"),
      fetchJSON<StrategyRecord[]>("/api/v1/strategies"),
      fetchJSON<BacktestRun[]>("/api/v1/backtests"),
      fetchJSON<BacktestOptions>("/api/v1/backtests/options"),
      fetchJSON<LiveAdapter[]>("/api/v1/live-adapters"),
      fetchJSON<SignalSourceCatalog>("/api/v1/signal-sources"),
      fetchJSON<SignalSourceType[]>("/api/v1/signal-source-types"),
      fetchJSON<SignalRuntimeAdapter[]>("/api/v1/signal-runtime/adapters"),
      fetchJSON<SignalRuntimeSession[]>("/api/v1/signal-runtime/sessions"),
      fetchJSON<RuntimePolicy>("/api/v1/runtime-policy"),
      fetchJSON<PlatformHealthSnapshot>("/api/v1/monitor/health"),
      fetchJSON<PlatformAlert[]>("/api/v1/alerts"),
      fetchJSON<PlatformNotification[]>("/api/v1/notifications?includeAcked=true"),
      fetchJSON<TelegramConfig>("/api/v1/telegram/config"),
      fetchJSON<any[]>("/api/v1/live/launch-templates").catch(() => []),
    ]);

    const strategyBindingEntries = await Promise.all(
      strategyData.map(async (strategy) => [
        strategy.id,
        await fetchJSON<SignalBinding[]>(`/api/v1/strategies/${strategy.id}/signal-bindings`),
      ] as const)
    );

    let runtimePlanData = null;
    const selectedRuntimeId = useTradingStore.getState().selectedSignalRuntimeId;
    const signalRuntimeFormSelector = useUIStore.getState().signalRuntimeForm;

    let planAccountId = "";
    let planStrategyId = "";

    if (selectedRuntimeId) {
      const session = signalRuntimeSessionData.find((s) => s.id === selectedRuntimeId);
      if (session) {
        planAccountId = session.accountId;
        planStrategyId = session.strategyId;
      }
    } else if (signalRuntimeFormSelector.accountId && signalRuntimeFormSelector.strategyId) {
      planAccountId = signalRuntimeFormSelector.accountId;
      planStrategyId = signalRuntimeFormSelector.strategyId;
    }

    if (planAccountId && planStrategyId) {
      try {
        runtimePlanData = await fetchJSON<Record<string, unknown>>(
          `/api/v1/signal-runtime/plan?accountId=${encodeURIComponent(planAccountId)}&strategyId=${encodeURIComponent(
            planStrategyId
          )}`
        );
      } catch (e) {
        console.warn("Failed to fetch runtime plan", e);
      }
    }

    const anchorDate = resolveChartAnchor(liveSessionData[0] ?? null, ordersData);
    const range = chartOverrideRange ?? buildTimeRange(anchorDate, timeWindow);
    const { from, to } = range;

    const monitorSessionForChart = liveSessionData[0] ?? null;
    let selectedResolution = monitorResolution;
    if (!selectedResolution) {
      const monitorSignalTimeframe = String(monitorSessionForChart?.state?.signalTimeframe ?? "1d");
      selectedResolution = monitorSignalTimeframe.toLowerCase() === "4h" ? "240" : "1D";
    }
    
    const monitorResolutionParam = selectedResolution;

    const [snapshotData, candleData, monitorCandleData, annotationData] = await Promise.all([
      summaryData[0]?.accountId
        ? fetchJSON<AccountEquitySnapshot[]>(
            `/api/v1/account-equity-snapshots?accountId=${encodeURIComponent(summaryData[0].accountId)}`
          )
        : Promise.resolve([]),
      fetchJSON<{ candles: ChartCandle[] }>(
        `/api/v1/chart/candles?symbol=BTCUSDT&resolution=1&from=${from}&to=${to}&limit=840`
      ),
      fetchJSON<{ candles: ChartCandle[] }>(
        `/api/v1/chart/candles?symbol=BTCUSDT&resolution=${encodeURIComponent(monitorResolutionParam)}&limit=400`
      ),
      fetchJSON<ChartAnnotation[]>(
        `/api/v1/chart/annotations?symbol=BTCUSDT&from=${from}&to=${to}&limit=300`
      ),
    ]);

    const normalizedSummaries = Array.isArray(summaryData) ? summaryData : [];
    const normalizedAccounts = Array.isArray(accountData) ? accountData : [];
    const normalizedOrders = Array.isArray(ordersData) ? ordersData : [];
    const normalizedFills = Array.isArray(fillsData) ? fillsData : [];
    const normalizedPositions = Array.isArray(positionsData) ? positionsData : [];
    const normalizedPaperSessions = Array.isArray(paperSessionData) ? paperSessionData : [];
    const normalizedLiveSessions = Array.isArray(liveSessionData) ? liveSessionData : [];
    const normalizedStrategies = Array.isArray(strategyData) ? strategyData : [];
    const normalizedBacktests = Array.isArray(backtestData) ? backtestData : [];
    const normalizedLiveAdapters = Array.isArray(liveAdapterData) ? liveAdapterData : [];
    const normalizedSignalSourceTypes = Array.isArray(signalSourceTypeData) ? signalSourceTypeData : [];
    const normalizedSignalRuntimeAdapters = Array.isArray(signalRuntimeAdapterData) ? signalRuntimeAdapterData : [];
    const normalizedSignalRuntimeSessions = Array.isArray(signalRuntimeSessionData) ? signalRuntimeSessionData : [];
    const normalizedAlerts = Array.isArray(alertData) ? alertData : [];
    const normalizedNotifications = Array.isArray(notificationData) ? notificationData : [];
    const normalizedSnapshots = Array.isArray(snapshotData) ? snapshotData : [];
    const normalizedAnnotations = Array.isArray(annotationData) ? annotationData : [];
    const normalizedCandles = Array.isArray(candleData?.candles) ? candleData.candles : [];
    const normalizedMonitorCandles = Array.isArray(monitorCandleData?.candles) ? monitorCandleData.candles : [];
    const normalizedSignalCatalog = signalCatalogData && typeof signalCatalogData === "object" ? signalCatalogData : { sources: [], notes: [] };
    const normalizedBacktestOptions =
      backtestOptionsData && typeof backtestOptionsData === "object" ? backtestOptionsData : ({} as BacktestOptions);
    const normalizedLaunchTemplates = Array.isArray(launchTemplateData) ? launchTemplateData : [];

    setSummaries(normalizedSummaries);
    setAccounts(normalizedAccounts);
    setOrders(normalizedOrders);
    setFills(normalizedFills);
    setPositions(normalizedPositions);
    setSnapshots(normalizedSnapshots);
    setMonitorCandles(normalizedMonitorCandles);
    setStrategies(normalizedStrategies);
    setSelectedStrategyId((current) => {
      if (current && normalizedStrategies.some((item) => item.id === current)) {
        return current;
      }
      return normalizedStrategies[0]?.id ?? null;
    });
    setBacktests(normalizedBacktests);
    setSelectedBacktestId((current: string | null) => {
      if (current && normalizedBacktests.some((item) => item.id === current)) {
        return current;
      }
      return normalizedBacktests.length > 0 ? normalizedBacktests[normalizedBacktests.length - 1].id : null;
    });
    setBacktestOptions(normalizedBacktestOptions);
    setPaperSessions(normalizedPaperSessions);
    setLiveSessions(normalizedLiveSessions);
    setLiveAdapters(normalizedLiveAdapters);
    setSignalCatalog(normalizedSignalCatalog as SignalSourceCatalog);
    setSignalSourceTypes(normalizedSignalSourceTypes);
    setSignalRuntimeAdapters(normalizedSignalRuntimeAdapters);
    setSignalRuntimeSessions(normalizedSignalRuntimeSessions);
    setRuntimePolicy(runtimePolicyData);
    setMonitorHealth(monitorHealthData);
    setAlerts(normalizedAlerts);
    setNotifications(normalizedNotifications);
    setTelegramConfig(telegramConfigData);
    setLaunchTemplates(normalizedLaunchTemplates);
    
    if (activeSettingsModal !== "telegram") {
      setTelegramForm({
        enabled: Boolean(telegramConfigData.enabled),
        botToken: "",
        chatId: String(telegramConfigData.chatId ?? ""),
        sendLevels: (telegramConfigData.sendLevels ?? []).join(",") || "critical,warning",
      });
    }
    setRuntimePolicyForm({
      tradeTickFreshnessSeconds: String(runtimePolicyData.tradeTickFreshnessSeconds ?? 15),
      orderBookFreshnessSeconds: String(runtimePolicyData.orderBookFreshnessSeconds ?? 10),
      signalBarFreshnessSeconds: String(runtimePolicyData.signalBarFreshnessSeconds ?? 30),
      runtimeQuietSeconds: String(runtimePolicyData.runtimeQuietSeconds ?? 30),
      strategyEvaluationQuietSeconds: String(runtimePolicyData.strategyEvaluationQuietSeconds ?? 0),
      liveAccountSyncFreshnessSeconds: String(runtimePolicyData.liveAccountSyncFreshnessSeconds ?? 0),
      paperStartReadinessTimeoutSeconds: String(runtimePolicyData.paperStartReadinessTimeoutSeconds ?? 5),
      dispatchMode: String(runtimePolicyData.dispatchMode ?? "manual-review"),
    });
    setStrategySignalBindingMap(Object.fromEntries(strategyBindingEntries));
    setStrategySignalBindings(strategyBindingEntries.flatMap((e) => e[1]));
    setSignalRuntimePlan(runtimePlanData);
    setSelectedSignalRuntimeId((current: string | null) => {
      if (current && normalizedSignalRuntimeSessions.some((item) => item.id === current)) {
        return current;
      }
      return normalizedSignalRuntimeSessions[0]?.id ?? null;
    });
    setCandles(normalizedCandles);
    setAnnotations(normalizedAnnotations);
    setBacktestForm((current: any) => ({
      strategyVersionId: current.strategyVersionId || normalizedStrategies[0]?.currentVersion?.id || "",
      signalTimeframe: current.signalTimeframe || normalizedBacktestOptions.defaultSignalTimeframe,
      executionDataSource: current.executionDataSource || normalizedBacktestOptions.defaultExecutionDataSource,
      symbol: current.symbol || "BTCUSDT",
      from: current.from || "",
      to: current.to || "",
    }));
  }

  useEffect(() => {
    let active = true;

    async function load() {
      if (!authSession?.token) {
        setLoading(false);
        return;
      }
      try {
        await loadDashboard();
        if (!active) {
          return;
        }
        setError(null);
      } catch (err) {
        if (!active) {
          return;
        }
        if (typeof err === "object" && err && "status" in err && (err as { status?: number }).status === 401) {
          writeStoredAuthSession(null);
          setAuthSession(null);
          setError("登录已失效，请重新登录");
          return;
        }
        setError(err instanceof Error ? err.message : "Failed to load monitoring data");
      } finally {
        if (active) {
          setLoading(false);
        }
      }
    }

    load();
    const timer = window.setInterval(load, 5000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [authSession?.token, timeWindow, chartOverrideRange, monitorResolution]);

  return { loadDashboard };
}
