import { useState, useEffect, useCallback } from 'react';
import { Fill } from '../types/domain';
import { fetchJSON } from '../utils/api';
import { useUIStore } from '../store/useUIStore';

export function useFillsPageQuery(pageSize: number, active: boolean) {
  const [fills, setFills] = useState<Fill[]>([]);
  const [totalCount, setTotalCount] = useState<number>(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const authSession = useUIStore(s => s.authSession);

  const fetchPage = useCallback(async (page: number) => {
    if (!authSession?.token || !active) return;
    setLoading(true);
    try {
      const offset = (page - 1) * pageSize;
      const [fillsData, countData] = await Promise.all([
        fetchJSON<Fill[]>(`/api/v1/fills?limit=${pageSize}&offset=${offset}`),
        fetchJSON<{count: number}>("/api/v1/fills/count"),
      ]);
      setFills(fillsData || []);
      setTotalCount(countData?.count || 0);
    } catch (err) {
      console.error("Failed to fetch fills page", err);
    } finally {
      setLoading(false);
    }
  }, [authSession?.token, active, pageSize]);

  useEffect(() => {
    if (active) {
      fetchPage(currentPage);
    }
  }, [active, currentPage, fetchPage]);

  const refetch = useCallback(() => {
    fetchPage(currentPage);
  }, [fetchPage, currentPage]);

  return { fills, totalCount, currentPage, setCurrentPage, loading, refetch };
}
