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

const DEFAULT_REALTIME_POLL_MS = 5000;
const MIN_REALTIME_POLL_MS = 1000;
const DEFAULT_STREAM_SYNC_MS = 60000;
const MIN_STREAM_SYNC_MS = 60000;

function intervalFromEnv(raw: string | undefined, fallback: number, minimum: number) {
  const parsed = parseInt(raw || "", 10);
  return isNaN(parsed) ? fallback : Math.max(minimum, parsed);
}

export function useDashboardRealtime() {
  const setError = useUIStore(s => s.setError);
  const setLoading = useUIStore(s => s.setLoading);
  const setAuthSession = useUIStore(s => s.setAuthSession);
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

    setLiveSessions(normalizedLiveSessions);
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
