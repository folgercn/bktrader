import { useEffect, useState } from 'react';
import { LiveTradePair } from '../types/domain';
import { fetchJSON } from '../utils/api';

export function useLiveTradePairs(sessionId: string | null, limit = 8) {
  const [pairs, setPairs] = useState<LiveTradePair[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!sessionId) {
      setPairs([]);
      setLoading(false);
      setError(null);
      return;
    }
    let active = true;
    setLoading(true);
    setError(null);
    fetchJSON<LiveTradePair[]>(
      `/api/v1/live/sessions/${encodeURIComponent(sessionId)}/trade-pairs?limit=${limit}`
    )
      .then((items) => {
        if (!active) {
          return;
        }
        setPairs(Array.isArray(items) ? items : []);
      })
      .catch((err) => {
        if (!active) {
          return;
        }
        console.warn('Failed to load live trade pairs', err);
        setPairs([]);
        setError(err instanceof Error ? err.message : '加载失败');
      })
      .finally(() => {
        if (active) {
          setLoading(false);
        }
      });

    return () => {
      active = false;
    };
  }, [limit, sessionId]);

  return { pairs, loading, error };
}
