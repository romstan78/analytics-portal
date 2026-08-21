import { useQuery } from '@tanstack/react-query';
import type { PromoDataResponse, PromoRow } from '../types/promo';

export interface PromoDataResult {
  rows: PromoRow[];
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

/**
 * Хук для получения данных промо с использованием React Query.
 * Использует стабильный queryKey — инвалидация через invalidateQueries.
 *
 * Возвращает интерфейс: { rows, loading, error, refetch }
 */
export function usePromoData(
  filters: Record<string, unknown>,
): PromoDataResult {
  // Стабильный queryKey — без refreshTrigger. Инвалидация через invalidateQueries.
  const queryKey = ['promoData', filters] as const;

  const { data: rows = [], isLoading, error, refetch } = useQuery({
    queryKey,
    queryFn: async ({ signal }) => {
      // filters берём из замыкания: queryKey уже содержит их, поэтому запрос
      // пересоздаётся при любом изменении фильтров.
      const currentFilters = filters;
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
        `${import.meta.env.VITE_API_BASE || 'http://localhost:8080'}/api/promo/data?all=true${qs ? '&' + qs : ''}`,
        {
          signal,
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('token')}`,
          },
        },
      );

      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const json = await response.json() as PromoDataResponse;
      return json.data || [];
    },
  });

  return {
    rows,
    loading: isLoading,
    error: error ? (error as Error).message || String(error) : null,
    refetch,
  };
}
