import { useEffect, useState } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { fetchJSON } from '../utils/api';
import { writeStoredAuthSession } from '../utils/auth';
import { 
  LiveSession, Position, Order, Fill, PlatformAlert, 
  PlatformNotification, PlatformHealthSnapshot
} from '../types/domain';
import { useDashboardStream } from './useDashboardStream';

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

  useEffect(() => {
    let active = true;

    async function load() {
      if (!authSession?.token) {
        if (isFirstLoad) setLoading(false);
        return;
      }
      try {
        await loadRealtime();
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

    if (isStreamEnabled && isStreamConnected) {
      return () => { active = false; };
    }

    const rawInterval = parseInt(import.meta.env.VITE_DASHBOARD_REALTIME_POLL_MS || "5000", 10);
    const pollInterval = isNaN(rawInterval) ? 5000 : Math.max(1000, rawInterval);
    
    let fallbackTimeout: number | undefined;
    let fallbackInterval: number | undefined;

    if (isFirstLoad) {
      load();
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
