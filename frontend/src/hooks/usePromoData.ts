import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useCallback } from 'react';
import type { PromoRow } from '../types/promo';

// ─── Типы ────────────────────────────────────────────────────────────────────

export interface PromoFilters {
  yearFrom?: string;
  yearTo?: string;
  months?: string[];
  kam?: string[];
  brand?: string[];
  sku?: string[];
  network_name?: string[];
  mechanics?: string[];
  status?: string[];
  channel?: string[];
  [key: string]: string | string[] | undefined;
}

export interface UsePromoDataReturn {
  rows: PromoRow[];
  setRows: (newRowsOrUpdater: PromoRow[] | ((old: PromoRow[]) => PromoRow[])) => void;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

// ─── Хук ─────────────────────────────────────────────────────────────────────

/**
 * Хук для получения данных промо с использованием React Query.
 */
export function usePromoData(
  filters: PromoFilters,
  refreshTrigger: number
): UsePromoDataReturn {
  const queryClient = useQueryClient();
  const filtersRef = useRef<PromoFilters>(filters);
  filtersRef.current = filters;

  const queryKey = ['promoData', filters, refreshTrigger] as const;

  const fetchPromoData = useCallback(async (): Promise<PromoRow[]> => {
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
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      }
    );

    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const json = await response.json();
    return json.data || [];
  }, []);

  const { data: rows = [], isLoading, error, refetch } = useQuery<PromoRow[]>({
    queryKey: queryKey as unknown as readonly unknown[],
    queryFn: fetchPromoData,
  });

  // Обновление кэша React Query (для обратной совместимости)
  const setRows = useCallback(
    (newRowsOrUpdater: PromoRow[] | ((old: PromoRow[]) => PromoRow[])) => {
      queryClient.setQueryData(queryKey as unknown as readonly unknown[], (old: unknown) => {
        const currentRows = (old as PromoRow[]) || [];
        if (typeof newRowsOrUpdater === 'function') {
          return newRowsOrUpdater(currentRows);
        }
        return newRowsOrUpdater;
      });
    },
    [queryClient, queryKey]
  );

  return {
    rows,
    setRows,
    loading: isLoading,
    error: error?.message || null,
    refetch,
  };
}