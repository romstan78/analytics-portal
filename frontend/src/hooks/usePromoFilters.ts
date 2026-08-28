import { useState, useEffect, useCallback, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { promoAPI } from '../api/promo';
import { userScopedKey } from '../utils/storage';
import { apiErrorMessage, queryFailure } from '../utils/apiError';

export interface FilterMeta {
  kam: string[];
  brand: string[];
  sku: string[];
  network_name: string[];
  mechanics: string[];
  channel: string[];
  status: string[];
  loading: boolean;
  error: string | null;
}

// Ключи скоупятся по пользователю здесь, а не в вызывающих страницах: сохранить
// чужой фильтр не должен ни один из них, и забыть про это в новой странице
// нельзя. Пересчёт при рендере достаточен — смена пользователя перемонтирует
// дерево, и хук стартует уже с ключами вошедшего.
export function usePromoFilters(
  initialFilters: Record<string, unknown>,
  baseStorageKey: string,
  basePersistFlagKey: string,
) {
  const storageKey = userScopedKey(baseStorageKey);
  const persistFlagKey = userScopedKey(basePersistFlagKey);

  const [filters, setFilters] = useState<Record<string, unknown>>(() => {
    try {
      if (localStorage.getItem(persistFlagKey) === 'true') {
        const saved = sessionStorage.getItem(storageKey);
        if (saved) return JSON.parse(saved);
      }
    } catch { /* повреждённый сохранённый фильтр — берём значения по умолчанию */ }
    return { ...initialFilters };
  });

  const [appliedFilters, setAppliedFilters] = useState<Record<string, unknown>>(filters);
  const [persistFilters, setPersistFilters] = useState(
    () => localStorage.getItem(persistFlagKey) === 'true',
  );
  // Справочники перезапрашиваются с задержкой после изменения фильтров.
  // Первое значение равно текущим фильтрам, поэтому стартовая загрузка идёт сразу.
  const [debouncedFilters, setDebouncedFilters] = useState<Record<string, unknown>>(filters);
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedFilters(filters), 300);
    return () => clearTimeout(timer);
  }, [filters]);

  const filtersQuery = useQuery({
    queryKey: ['promoFilters', debouncedFilters] as const,
    queryFn: () => promoAPI.getFilters(debouncedFilters),
  });
  const { data, isFetching, refetch } = filtersQuery;
  const failure = queryFailure(filtersQuery);

  const meta: FilterMeta = useMemo(() => ({
    kam: data?.kam || [],
    brand: data?.brand || [],
    sku: data?.sku || [],
    network_name: data?.network_name || [],
    mechanics: data?.mechanics || [],
    channel: data?.channel || [],
    status: data?.status || [],
    loading: isFetching,
    error: failure != null ? apiErrorMessage(failure, 'Ошибка загрузки справочников') : null,
  }), [data, isFetching, failure]);

  // Ручное обновление справочников (кнопка «Повторить» при ошибке).
  const fetchMeta = useCallback(() => { void refetch(); }, [refetch]);

  const applyFilters = useCallback((nextFilters: Record<string, unknown>) => {
    const next = { ...nextFilters };
    setFilters(next);
    setAppliedFilters(next);
    // Если галочка включена — сохраняем фильтры в sessionStorage
    if (localStorage.getItem(persistFlagKey) === 'true') {
      sessionStorage.setItem(storageKey, JSON.stringify(next));
    }
  }, [persistFlagKey, storageKey]);

  const handleSearch = useCallback(() => {
    applyFilters(filters);
  }, [applyFilters, filters]);

  const handleReset = useCallback(() => {
    const empty = { ...initialFilters };
    setFilters(empty);
    setAppliedFilters(empty);
    sessionStorage.removeItem(storageKey);
  }, [initialFilters, storageKey]);

  const handlePersistChange = useCallback((checked: boolean) => {
    setPersistFilters(checked);
    localStorage.setItem(persistFlagKey, String(checked));
    if (checked) {
      // Сразу сохраняем текущие фильтры при включении
      sessionStorage.setItem(storageKey, JSON.stringify(filters));
    } else {
      sessionStorage.removeItem(storageKey);
    }
  }, [persistFlagKey, storageKey, filters]);

  return {
    meta, filters, setFilters, appliedFilters,
    persistFilters, handleSearch, handleReset, handlePersistChange,
    fetchMeta, applyFilters,
  };
}
