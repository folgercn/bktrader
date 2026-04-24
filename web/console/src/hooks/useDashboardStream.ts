import { useEffect, useState, useRef } from 'react';
import { useTradingStore } from '../store/useTradingStore';
import { useUIStore } from '../store/useUIStore';
import { writeStoredAuthSession } from '../utils/auth';
import { 
  LiveSession, Position, Order, Fill, PlatformAlert, 
  PlatformNotification, PlatformHealthSnapshot
} from '../types/domain';

export function useDashboardStream(enabled: boolean) {
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

  const [isConnected, setIsConnected] = useState(false);
  const retryCount = useRef(0);
  const eventSourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (!enabled || !authSession?.token) {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
        setIsConnected(false);
      }
      return;
    }

    let active = true;

    function connect() {
      if (!active) return;
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }

      setLoading(true);
      const token = authSession?.token || '';
      const url = `${import.meta.env.VITE_API_BASE || ''}/api/v1/stream/dashboard?token=${encodeURIComponent(token)}`;
      const es = new EventSource(url);
      eventSourceRef.current = es;

      es.onopen = () => {
        if (!active) return;
        setIsConnected(true);
        setError(null);
        setLoading(false);
        retryCount.current = 0;
      };

      es.onerror = (err) => {
        if (!active) return;
        console.error("SSE Error:", err);
        setIsConnected(false);
        es.close();

        // Increment retry and reconnect with backoff
        retryCount.current += 1;
        const delay = Math.min(10000, 1000 * Math.pow(2, retryCount.current)); // max 10s backoff
        setTimeout(() => {
          if (active) connect();
        }, delay);
      };

      // Handler for various domain events
      const handleEvent = (domain: string, setter: (data: any) => void) => (e: MessageEvent) => {
        if (!active) return;
        try {
          const parsed = JSON.parse(e.data);
          // For now we only support 'snapshot' which replaces the entire state.
          // Phase 5.3 will introduce 'upsert'/'delete' for incremental updates.
          if (parsed.action === 'snapshot') {
            const data = Array.isArray(parsed.payload) ? parsed.payload : (parsed.payload || null);
            setter(data);
          }
        } catch (err) {
          console.error(`Failed to parse SSE event for ${domain}`, err);
        }
      };

      es.addEventListener('live-sessions', handleEvent('live-sessions', setLiveSessions));
      es.addEventListener('positions', handleEvent('positions', setPositions));
      es.addEventListener('orders', handleEvent('orders', setOrders));
      es.addEventListener('fills', handleEvent('fills', setFills));
      es.addEventListener('alerts', handleEvent('alerts', setAlerts));
      es.addEventListener('notifications', handleEvent('notifications', setNotifications));
      es.addEventListener('monitor-health', handleEvent('monitor-health', setMonitorHealth));
    }

    connect();

    return () => {
      active = false;
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
    };
  }, [enabled, authSession?.token]);

  return { isConnected };
}
