import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useCallback } from 'react';

/**
 * Хук для получения данных промо с использованием React Query.
 * Заменяет ручной AbortController, JSON.stringify сравнение фильтров,
 * и state-машину loading/error.
 *
 * Возвращает совместимый интерфейс: { rows, setRows, loading, error, refetch }
 * чтобы не ломать PromoAnalysis.jsx.
 */
export function usePromoData(filters, refreshTrigger) {
  const queryClient = useQueryClient();
  const filtersRef = useRef(filters);
  filtersRef.current = filters;

  // Стабильный queryKey на основе фильтров и refreshTrigger
  const queryKey = ['promoData', filters, refreshTrigger];

  const fetchPromoData = useCallback(async ({ signal }) => {
    const currentFilters = filtersRef.current;
    const params = new URLSearchParams();
    Object.entries(currentFilters).forEach(([key, value]) => {
      if (Array.isArray(value)) {
        value.forEach(v => { if (v !== '' && v != null) params.append(key, String(v)); });
      } else if (value !== '' && value != null) {
        params.set(key, String(value));
      }
    });

    const qs = params.toString();
    const response = await fetch(
      `http://localhost:8080/api/promo/data?all=true${qs ? '&' + qs : ''}`,
      {
        signal,
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      }
    );

    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const json = await response.json();
    return json.data || [];
  }, []);

  const { data: rows = [], isLoading, error, refetch } = useQuery({
    queryKey,
    queryFn: fetchPromoData,
  });

  // setRows — для обратной совместимости: обновляет кеш React Query
  const setRows = useCallback((newRowsOrUpdater) => {
    queryClient.setQueryData(queryKey, (old) => {
      if (typeof newRowsOrUpdater === 'function') {
        return newRowsOrUpdater(old || []);
      }
      return newRowsOrUpdater;
    });
  }, [queryClient, queryKey]);

  return {
    rows,
    setRows,
    loading: isLoading,
    error: error?.message || null,
    refetch,
  };
}