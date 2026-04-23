import { useState, useEffect, useCallback } from 'react';
import { Order } from '../types/domain';
import { fetchJSON } from '../utils/api';
import { useUIStore } from '../store/useUIStore';

export function useOrdersPageQuery(pageSize: number, active: boolean) {
  const [orders, setOrders] = useState<Order[]>([]);
  const [totalCount, setTotalCount] = useState<number>(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const authSession = useUIStore(s => s.authSession);

  const fetchPage = useCallback(async (page: number) => {
    if (!authSession?.token || !active) return;
    setLoading(true);
    try {
      const offset = (page - 1) * pageSize;
      const [ordersData, countData] = await Promise.all([
        fetchJSON<Order[]>(`/api/v1/orders?limit=${pageSize}&offset=${offset}`),
        fetchJSON<{count: number}>("/api/v1/orders/count"),
      ]);
      setOrders(ordersData || []);
      setTotalCount(countData?.count || 0);
    } catch (err) {
      console.error("Failed to fetch orders page", err);
    } finally {
      setLoading(false);
    }
  }, [authSession?.token, active, pageSize]);

  useEffect(() => {
    let interval: number;
    if (active) {
      fetchPage(currentPage);
      interval = window.setInterval(() => {
        fetchPage(currentPage);
      }, 5000);
    }
    return () => {
      if (interval) window.clearInterval(interval);
    };
  }, [active, currentPage, fetchPage]);

  return { orders, totalCount, currentPage, setCurrentPage, loading };
}
