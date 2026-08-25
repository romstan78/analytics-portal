import { useState, useEffect, useCallback, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { promoAPI } from '../api/promo';

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

export function usePromoFilters(
  initialFilters: Record<string, unknown>,
  storageKey: string,
  persistFlagKey: string,
) {
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

  const { data, isFetching, error, refetch } = useQuery({
    queryKey: ['promoFilters', debouncedFilters] as const,
    queryFn: () => promoAPI.getFilters(debouncedFilters),
  });

  const meta: FilterMeta = useMemo(() => ({
    kam: data?.kam || [],
    brand: data?.brand || [],
    sku: data?.sku || [],
    network_name: data?.network_name || [],
    mechanics: data?.mechanics || [],
    channel: data?.channel || [],
    status: data?.status || [],
    loading: isFetching,
    error: error ? (error instanceof Error ? error.message : String(error)) : null,
  }), [data, isFetching, error]);

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
