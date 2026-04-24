import { useEffect, useState } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { fetchJSON } from '../utils/api';
import { writeStoredAuthSession } from '../utils/auth';
import { 
  AccountSummary, AccountRecord, SignalRuntimeSession, 
  AccountEquitySnapshot, ChartAnnotation
} from '../types/domain';
import { resolveChartAnchor, buildTimeRange } from '../utils/derivation';

export function useDashboardState() {
  const setError = useUIStore(s => s.setError);
  const setAuthSession = useUIStore(s => s.setAuthSession);
  const authSession = useUIStore(s => s.authSession);
  const timeWindow = useUIStore(s => s.timeWindow);
  const chartOverrideRange = useUIStore(s => s.chartOverrideRange);

  const setSummaries = useTradingStore(s => s.setSummaries);
  const setAccounts = useTradingStore(s => s.setAccounts);
  const setSignalRuntimeSessions = useTradingStore(s => s.setSignalRuntimeSessions);
  const setSnapshots = useTradingStore(s => s.setSnapshots);
  const setAnnotations = useTradingStore(s => s.setAnnotations);
  const setSelectedSignalRuntimeId = useTradingStore(s => s.setSelectedSignalRuntimeId);
  const setSignalRuntimePlan = useTradingStore(s => s.setSignalRuntimePlan);

  async function loadState() {
    const [
      summaryData,
      accountData,
      signalRuntimeSessionData,
    ] = await Promise.all([
      fetchJSON<AccountSummary[]>("/api/v1/account-summaries"),
      fetchJSON<AccountRecord[]>("/api/v1/accounts"),
      fetchJSON<SignalRuntimeSession[]>("/api/v1/signal-runtime/sessions?view=summary"),
    ]);

    const normalizedSummaries = Array.isArray(summaryData) ? summaryData : [];
    const normalizedAccounts = Array.isArray(accountData) ? accountData : [];
    const normalizedSignalRuntimeSessions = Array.isArray(signalRuntimeSessionData) ? signalRuntimeSessionData : [];

    const liveSessions = useTradingStore.getState().liveSessions;
    const orders = useTradingStore.getState().orders;

    const anchorDate = resolveChartAnchor(liveSessions[0] ?? null, orders);
    const range = chartOverrideRange ?? buildTimeRange(anchorDate, timeWindow);
    const { from, to } = range;

    const [snapshotData, annotationData] = await Promise.all([
      normalizedSummaries[0]?.accountId
        ? fetchJSON<AccountEquitySnapshot[]>(
            `/api/v1/account-equity-snapshots?accountId=${encodeURIComponent(normalizedSummaries[0].accountId)}`
          )
        : Promise.resolve([]),
      fetchJSON<ChartAnnotation[]>(
        `/api/v1/chart/annotations?symbol=BTCUSDT&from=${from}&to=${to}&limit=300`
      ),
    ]);

    const normalizedSnapshots = Array.isArray(snapshotData) ? snapshotData : [];
    const normalizedAnnotations = Array.isArray(annotationData) ? annotationData : [];

    let runtimePlanData = null;
    const selectedRuntimeId = useTradingStore.getState().selectedSignalRuntimeId;
    const signalRuntimeFormSelector = useUIStore.getState().signalRuntimeForm;

    let planAccountId = "";
    let planStrategyId = "";

    if (selectedRuntimeId) {
      const session = normalizedSignalRuntimeSessions.find((s) => s.id === selectedRuntimeId);
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

    setSummaries(normalizedSummaries);
    setAccounts(normalizedAccounts);
    setSignalRuntimeSessions(normalizedSignalRuntimeSessions);
    setSnapshots(normalizedSnapshots);
    setAnnotations(normalizedAnnotations);
    setSignalRuntimePlan(runtimePlanData);

    setSelectedSignalRuntimeId((current: string | null) => {
      // 检查当前选中的 ID 是否在运行时会话或实盘会话中依然有效
      const stillValid = 
        (current && normalizedSignalRuntimeSessions.some((item) => item.id === current)) ||
        (current && liveSessions.some((item) => item.id === current));
      
      if (stillValid) {
        return current;
      }
      return normalizedSignalRuntimeSessions[0]?.id ?? liveSessions[0]?.id ?? null;
    });
  }

  useEffect(() => {
    let active = true;

    async function load() {
      if (!authSession?.token) return;
      try {
        await loadState();
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
        setError(err instanceof Error ? err.message : "Failed to load state data");
      }
    }

    load();
    const rawInterval = parseInt(import.meta.env.VITE_DASHBOARD_STATE_POLL_MS || "15000", 10);
    const pollInterval = isNaN(rawInterval) ? 15000 : Math.max(5000, rawInterval);
    const timer = window.setInterval(load, pollInterval);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [authSession?.token, timeWindow, chartOverrideRange]);

  return { loadState };
}
