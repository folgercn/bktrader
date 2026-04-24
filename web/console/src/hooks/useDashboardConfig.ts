import { useEffect } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { fetchJSON } from '../utils/api';
import { writeStoredAuthSession } from '../utils/auth';
import { 
  StrategyRecord, BacktestRun, BacktestOptions, LiveAdapter, 
  SignalSourceCatalog, SignalSourceType, SignalRuntimeAdapter, 
  RuntimePolicy, TelegramConfig, SignalBinding 
} from '../types/domain';

export function useDashboardConfig() {
  const setError = useUIStore(s => s.setError);
  const setAuthSession = useUIStore(s => s.setAuthSession);
  const setTelegramForm = useUIStore(s => s.setTelegramForm);
  const setRuntimePolicyForm = useUIStore(s => s.setRuntimePolicyForm);
  const activeSettingsModal = useUIStore(s => s.activeSettingsModal);
  const authSession = useUIStore(s => s.authSession);
  const setBacktestForm = useUIStore(s => s.setBacktestForm);

  const setStrategies = useTradingStore(s => s.setStrategies);
  const setSelectedStrategyId = useTradingStore(s => s.setSelectedStrategyId);
  const setBacktests = useTradingStore(s => s.setBacktests);
  const setSelectedBacktestId = useTradingStore(s => s.setSelectedBacktestId);
  const setBacktestOptions = useTradingStore(s => s.setBacktestOptions);
  const setLiveAdapters = useTradingStore(s => s.setLiveAdapters);
  const setSignalCatalog = useTradingStore(s => s.setSignalCatalog);
  const setSignalSourceTypes = useTradingStore(s => s.setSignalSourceTypes);
  const setSignalRuntimeAdapters = useTradingStore(s => s.setSignalRuntimeAdapters);
  const setRuntimePolicy = useTradingStore(s => s.setRuntimePolicy);
  const setTelegramConfig = useTradingStore(s => s.setTelegramConfig);
  const setLaunchTemplates = useTradingStore(s => s.setLaunchTemplates);
  const setStrategySignalBindingMap = useTradingStore(s => s.setStrategySignalBindingMap);
  const setStrategySignalBindings = useTradingStore(s => s.setStrategySignalBindings);
  async function loadConfig() {
    const [
      strategyData,
      backtestData,
      backtestOptionsData,
      liveAdapterData,
      signalCatalogData,
      signalSourceTypeData,
      signalRuntimeAdapterData,
      runtimePolicyData,
      telegramConfigData,
      launchTemplateData,
    ] = await Promise.all([
      fetchJSON<StrategyRecord[]>("/api/v1/strategies"),
      fetchJSON<BacktestRun[]>("/api/v1/backtests"),
      fetchJSON<BacktestOptions>("/api/v1/backtests/options"),
      fetchJSON<LiveAdapter[]>("/api/v1/live-adapters"),
      fetchJSON<SignalSourceCatalog>("/api/v1/signal-sources"),
      fetchJSON<SignalSourceType[]>("/api/v1/signal-source-types"),
      fetchJSON<SignalRuntimeAdapter[]>("/api/v1/signal-runtime/adapters"),
      fetchJSON<RuntimePolicy>("/api/v1/runtime-policy"),
      fetchJSON<TelegramConfig>("/api/v1/telegram/config"),
      fetchJSON<any[]>("/api/v1/live/launch-templates").catch(() => []),
    ]);

    const strategyBindingEntries = await Promise.all(
      strategyData.map(async (strategy) => [
        strategy.id,
        await fetchJSON<SignalBinding[]>(`/api/v1/strategies/${strategy.id}/signal-bindings`),
      ] as const)
    );

    const normalizedStrategies = Array.isArray(strategyData) ? strategyData : [];
    const normalizedBacktests = Array.isArray(backtestData) ? backtestData : [];
    const normalizedLiveAdapters = Array.isArray(liveAdapterData) ? liveAdapterData : [];
    const normalizedSignalSourceTypes = Array.isArray(signalSourceTypeData) ? signalSourceTypeData : [];
    const normalizedSignalRuntimeAdapters = Array.isArray(signalRuntimeAdapterData) ? signalRuntimeAdapterData : [];
    const normalizedSignalCatalog = signalCatalogData && typeof signalCatalogData === "object" ? signalCatalogData : { sources: [], notes: [] };
    const normalizedBacktestOptions = backtestOptionsData && typeof backtestOptionsData === "object" ? backtestOptionsData : ({} as BacktestOptions);
    const normalizedLaunchTemplates = Array.isArray(launchTemplateData) ? launchTemplateData : [];

    setStrategies(normalizedStrategies);
    setSelectedStrategyId((current) => {
      if (current && normalizedStrategies.some((item) => item.id === current)) return current;
      return normalizedStrategies[0]?.id ?? null;
    });
    setBacktests(normalizedBacktests);
    setSelectedBacktestId((current: string | null) => {
      if (current && normalizedBacktests.some((item) => item.id === current)) return current;
      return normalizedBacktests.length > 0 ? normalizedBacktests[normalizedBacktests.length - 1].id : null;
    });
    setBacktestOptions(normalizedBacktestOptions);
    setLiveAdapters(normalizedLiveAdapters);
    setSignalCatalog(normalizedSignalCatalog as SignalSourceCatalog);
    setSignalSourceTypes(normalizedSignalSourceTypes);
    setSignalRuntimeAdapters(normalizedSignalRuntimeAdapters);
    setRuntimePolicy(runtimePolicyData);
    setTelegramConfig(telegramConfigData);
    setLaunchTemplates(normalizedLaunchTemplates);
    
    if (activeSettingsModal !== "telegram") {
      setTelegramForm({
        enabled: Boolean(telegramConfigData.enabled),
        botToken: "",
        chatId: String(telegramConfigData.chatId ?? ""),
        sendLevels: (telegramConfigData.sendLevels ?? []).join(",") || "critical,warning",
        tradeEventsEnabled: Boolean(telegramConfigData.tradeEventsEnabled),
        positionReportEnabled: Boolean(telegramConfigData.positionReportEnabled),
        positionReportIntervalMinutes: String(telegramConfigData.positionReportIntervalMinutes ?? 30),
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
      if (!authSession?.token) return;
      try {
        await loadConfig();
        if (!active) return;
        setError(null);
      } catch (err) {
        if (!active) return;
        if (typeof err === "object" && err && "status" in err && (err as { status?: number }).status === 401) {
          writeStoredAuthSession(null);
          setAuthSession(null);
          setError("登录已失效，请重新登录");
          return;
        }
        setError(err instanceof Error ? err.message : "Failed to load config data");
      }
    }

    load();
    const rawInterval = parseInt(import.meta.env.VITE_DASHBOARD_CONFIG_POLL_MS || "60000", 10);
    const pollInterval = isNaN(rawInterval) ? 60000 : Math.max(10000, rawInterval);
    const timer = window.setInterval(load, pollInterval);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [authSession?.token]);

  return { loadConfig };
}
