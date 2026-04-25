import { useEffect, useState, useRef } from 'react';
import { fetchJSON } from '../utils/api';
import { useTradingStore } from '../store/useTradingStore';
import { useUIStore } from '../store/useUIStore';

export function useDashboardStream(enabled: boolean) {
  const setError = useUIStore(s => s.setError);
  const setLoading = useUIStore(s => s.setLoading);
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
  const retryTimerRef = useRef<number | null>(null);

  useEffect(() => {
    if (!enabled || !authSession?.token) {
      if (retryTimerRef.current !== null) {
        window.clearTimeout(retryTimerRef.current);
        retryTimerRef.current = null;
      }
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
        setIsConnected(false);
      }
      return;
    }

    let active = true;

    async function connect() {
      if (!active) return;
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }

      setLoading(true);
      
      let streamToken = '';
      try {
        const res = await fetchJSON<{token: string}>('/api/v1/auth/stream-token', {
          method: 'POST'
        });
        streamToken = res.token;
      } catch (err) {
        if (!active) return;
        console.error("Failed to fetch stream token:", err);
        setError("Failed to authenticate stream");
        setLoading(false);
        setIsConnected(false);
        
        retryCount.current += 1;
        const delay = Math.min(10000, 1000 * Math.pow(2, retryCount.current));
        if (retryTimerRef.current !== null) {
          window.clearTimeout(retryTimerRef.current);
        }
        retryTimerRef.current = window.setTimeout(() => {
          retryTimerRef.current = null;
          if (active) connect();
        }, delay);
        return;
      }

      if (!active) return;

      const url = `${import.meta.env.VITE_API_BASE || ''}/api/v1/stream/dashboard?token=${encodeURIComponent(streamToken)}`;
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
        if (retryTimerRef.current !== null) {
          window.clearTimeout(retryTimerRef.current);
        }
        retryTimerRef.current = window.setTimeout(() => {
          retryTimerRef.current = null;
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
      if (retryTimerRef.current !== null) {
        window.clearTimeout(retryTimerRef.current);
        retryTimerRef.current = null;
      }
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
    };
  }, [enabled, authSession?.token]);

  return { isConnected };
}
