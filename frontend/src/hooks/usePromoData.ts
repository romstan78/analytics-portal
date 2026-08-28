import { keepPreviousData, useQuery } from '@tanstack/react-query';
import type { PromoDataResponse, PromoRow } from '../types/promo';
import { fetchWithAuth, parseJSONResponse, buildParams } from '../api/promo';
import { apiErrorMessage, queryFailure } from '../utils/apiError';

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080';

export interface PromoDataQuery {
  page: number;
  pageSize: number;
  search?: string;
  sortField?: string;
  sortDirection?: 'asc' | 'desc';
}

export interface PromoDataResult {
  rows: PromoRow[];
  total: number;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

/**
 * Страница таблицы промо.
 *
 * Выборка постраничная, а не целиком: раньше хук всегда запрашивал `all=true`,
 * и вся база уезжала в память вкладки без какого-либо потолка. Поиск и
 * сортировка тоже серверные — на клиенте они видели бы только текущую страницу.
 *
 * Запрос идёт через fetchWithAuth: он обновляет истёкший access-токен под
 * promise-lock и повторяет вызов. Ручной заголовок Authorization этого не умел,
 * и через 15 минут таблица падала с HTTP 401 там, где все остальные экраны
 * продолжали работать.
 */
export function usePromoData(
  filters: Record<string, unknown>,
  enabled = true,
  query: PromoDataQuery = { page: 0, pageSize: 100 },
): PromoDataResult {
  const { page, pageSize, search = '', sortField = '', sortDirection = 'desc' } = query;
  const queryKey = ['promoData', filters, page, pageSize, search, sortField, sortDirection] as const;

  const dataQuery = useQuery({
    queryKey,
    enabled,
    // Прошлая страница остаётся на экране, пока грузится следующая: иначе
    // таблица моргала бы пустотой на каждом перелистывании.
    placeholderData: keepPreviousData,
    queryFn: () => {
      const params = buildParams({
        ...filters,
        page,
        pageSize,
        ...(search ? { search } : {}),
        ...(sortField ? { sortField, sortDirection } : {}),
      });
      return fetchWithAuth(`${API_BASE}/api/promo/data?${params}`)
        .then(r => parseJSONResponse<PromoDataResponse>(r, 'Не удалось загрузить промо'));
    },
  });

  const failure = queryFailure(dataQuery);

  return {
    rows: dataQuery.data?.data ?? [],
    total: dataQuery.data?.totalRows ?? 0,
    loading: dataQuery.isLoading,
    error: failure != null ? apiErrorMessage(failure, 'Не удалось загрузить промо') : null,
    refetch: dataQuery.refetch,
  };
}
