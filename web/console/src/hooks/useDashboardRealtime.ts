import { useEffect, useRef, useState } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { fetchJSON } from '../utils/api';
import { writeStoredAuthSession } from '../utils/auth';
import { 
  LiveSession, Position, Order, Fill, PlatformAlert, 
  PlatformNotification, PlatformHealthSnapshot
} from '../types/domain';
import { useDashboardStream } from './useDashboardStream';
import { mergeLiveSessionSnapshot } from '../utils/liveSessionDetail';

const DEFAULT_REALTIME_POLL_MS = 5000;
const MIN_REALTIME_POLL_MS = 1000;
const DEFAULT_STREAM_SYNC_MS = 60000;
const MIN_STREAM_SYNC_MS = 60000;

function intervalFromEnv(raw: string | undefined, fallback: number, minimum: number) {
  const parsed = parseInt(raw || "", 10);
  return isNaN(parsed) ? fallback : Math.max(minimum, parsed);
}

type ConsoleNotification = { type: 'success' | 'error' | 'info'; message: string };

function liveSessionControlStatus(session: LiveSession) {
  const state = session.state ?? {};
  const desired = String(state.desiredStatus ?? "").trim().toUpperCase();
  const actual = String(state.actualStatus ?? "").trim().toUpperCase();
  const error = String(state.lastControlError ?? "").trim();
  const errorCode = String(state.lastControlErrorCode ?? "").trim().toUpperCase();
  return {
    desired,
    actual,
    error,
    errorCode,
    key: `${desired}|${actual}|${errorCode}|${error}`,
  };
}

function liveSessionControlErrorNotification(current: ReturnType<typeof liveSessionControlStatus>, sessionID: string) {
  const detail = current.error || sessionID;
  switch (current.errorCode) {
    case "ACTIVE_POSITIONS_OR_ORDERS":
      return `会话控制失败 (${current.errorCode})：${detail}。请先平仓/撤单，或确认风险后使用 force stop。`;
    case "RUNTIME_LEASE_NOT_ACQUIRED":
    case "CONTROL_OPERATION_IN_PROGRESS":
      return `会话控制失败 (${current.errorCode})：${detail}。当前 runner/control 操作占用中，稍后重试。`;
    case "CONFIG_ERROR":
      return `会话控制失败 (${current.errorCode})：${detail}。请检查 live session、账户和 runtime 配置。`;
    case "ADAPTER_ERROR":
      return `会话控制失败 (${current.errorCode})：${detail}。请检查交易所适配器连接和 runner 日志。`;
    default:
      return current.errorCode ? `会话控制失败 (${current.errorCode})：${detail}` : `会话控制失败：${detail}`;
  }
}

function notifyLiveSessionControlTransitions(
  previousByID: Map<string, string>,
  sessions: LiveSession[],
  setNotification: (notification: ConsoleNotification | null) => void
) {
  const seen = new Set<string>();
  for (const session of sessions) {
    seen.add(session.id);
    const current = liveSessionControlStatus(session);
    const previousKey = previousByID.get(session.id);
    previousByID.set(session.id, current.key);
    if (!previousKey || previousKey === current.key || !current.desired) {
      continue;
    }
    const [previousDesired, previousActual] = previousKey.split("|");
    const wasPending = previousDesired !== "" && previousDesired !== previousActual;
    if (current.actual === "ERROR") {
      setNotification({
        type: 'error',
        message: liveSessionControlErrorNotification(current, session.id),
      });
      continue;
    }
    if (wasPending && current.desired === current.actual) {
      setNotification({
        type: 'success',
        message: current.actual === "RUNNING" ? `会话已启动完成：${session.id}` : `会话已停止完成：${session.id}`,
      });
    }
  }
  for (const id of Array.from(previousByID.keys())) {
    if (!seen.has(id)) {
      previousByID.delete(id);
    }
  }
}

export function useDashboardRealtime() {
  const setError = useUIStore(s => s.setError);
  const setLoading = useUIStore(s => s.setLoading);
  const setAuthSession = useUIStore(s => s.setAuthSession);
  const setNotification = useUIStore(s => s.setNotification);
  const authSession = useUIStore(s => s.authSession);

  const setLiveSessions = useTradingStore(s => s.setLiveSessions);
  const setPositions = useTradingStore(s => s.setPositions);
  const setOrders = useTradingStore(s => s.setOrders);
  const setFills = useTradingStore(s => s.setFills);
  const setAlerts = useTradingStore(s => s.setAlerts);
  const setNotifications = useTradingStore(s => s.setNotifications);
  const setMonitorHealth = useTradingStore(s => s.setMonitorHealth);

  const [isFirstLoad, setIsFirstLoad] = useState(true);
  const realtimeLoadInFlightRef = useRef<Promise<void> | null>(null);
  const liveSessionControlRef = useRef<Map<string, string>>(new Map());

  const isStreamEnabled = import.meta.env.VITE_DASHBOARD_STREAM_ENABLED === 'true';
  const { isConnected: isStreamConnected } = useDashboardStream(isStreamEnabled);

  async function loadRealtime() {
    const [
      liveSessionData,
      positionsData,
      ordersData,
      fillsData,
      alertData,
      notificationData,
      monitorHealthData,
    ] = await Promise.all([
      fetchJSON<LiveSession[]>("/api/v1/live/sessions?view=summary"),
      fetchJSON<Position[]>("/api/v1/positions"),
      fetchJSON<Order[]>("/api/v1/orders?limit=50"),
      fetchJSON<Fill[]>("/api/v1/fills?limit=50"),
      fetchJSON<PlatformAlert[]>("/api/v1/alerts"),
      fetchJSON<PlatformNotification[]>("/api/v1/notifications?includeAcked=true"),
      fetchJSON<PlatformHealthSnapshot>("/api/v1/monitor/health"),
    ]);

    const normalizedLiveSessions = Array.isArray(liveSessionData) ? liveSessionData : [];
    const normalizedPositions = Array.isArray(positionsData) ? positionsData : [];
    const normalizedOrders = Array.isArray(ordersData) ? ordersData : [];
    const normalizedFills = Array.isArray(fillsData) ? fillsData : [];
    const normalizedAlerts = Array.isArray(alertData) ? alertData : [];
    const normalizedNotifications = Array.isArray(notificationData) ? notificationData : [];

    const mergedLiveSessions = mergeLiveSessionSnapshot(useTradingStore.getState().liveSessions, normalizedLiveSessions);
    notifyLiveSessionControlTransitions(liveSessionControlRef.current, mergedLiveSessions, setNotification);
    setLiveSessions(mergedLiveSessions);
    setPositions(normalizedPositions);
    setOrders(normalizedOrders);
    setFills(normalizedFills);
    setAlerts(normalizedAlerts);
    setNotifications(normalizedNotifications);
    setMonitorHealth(monitorHealthData);
  }

  async function loadRealtimeOnce() {
    if (realtimeLoadInFlightRef.current) {
      return realtimeLoadInFlightRef.current;
    }
    const promise = loadRealtime().finally(() => {
      if (realtimeLoadInFlightRef.current === promise) {
        realtimeLoadInFlightRef.current = null;
      }
    });
    realtimeLoadInFlightRef.current = promise;
    return promise;
  }

  useEffect(() => {
    let active = true;

    async function load() {
      if (!authSession?.token) {
        if (isFirstLoad) setLoading(false);
        return;
      }
      try {
        await loadRealtimeOnce();
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
        setError(err instanceof Error ? err.message : "Failed to load realtime data");
      } finally {
        if (active && isFirstLoad) {
          setLoading(false);
          setIsFirstLoad(false);
        }
      }
    }

    const pollInterval = intervalFromEnv(
      import.meta.env.VITE_DASHBOARD_REALTIME_POLL_MS,
      DEFAULT_REALTIME_POLL_MS,
      MIN_REALTIME_POLL_MS
    );
    const streamSyncInterval = intervalFromEnv(
      import.meta.env.VITE_DASHBOARD_REALTIME_SYNC_MS,
      DEFAULT_STREAM_SYNC_MS,
      MIN_STREAM_SYNC_MS
    );
    let fallbackTimeout: number | undefined;
    let fallbackInterval: number | undefined;
    let streamSyncTimer: number | undefined;

    load();

    if (isStreamEnabled && isStreamConnected) {
      streamSyncTimer = window.setInterval(load, streamSyncInterval);
      return () => {
        active = false;
        if (streamSyncTimer) window.clearInterval(streamSyncTimer);
      };
    }

    if (isFirstLoad) {
      fallbackInterval = window.setInterval(load, pollInterval);
    } else {
      fallbackTimeout = window.setTimeout(() => {
        if (!active) return;
        load();
        fallbackInterval = window.setInterval(load, pollInterval);
      }, pollInterval);
    }

    return () => {
      active = false;
      if (fallbackTimeout) window.clearTimeout(fallbackTimeout);
      if (fallbackInterval) window.clearInterval(fallbackInterval);
    };
  }, [authSession?.token, isStreamEnabled, isStreamConnected]);

  return { loadRealtime };
}
