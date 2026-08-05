import { useState, useEffect, useCallback, useRef } from 'react';
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
  const [meta, setMeta] = useState<FilterMeta>({
    kam: [], brand: [], sku: [], network_name: [], mechanics: [], channel: [], status: [],
    loading: true, error: null,
  });

  const [filters, setFilters] = useState<Record<string, unknown>>(() => {
    try {
      if (localStorage.getItem(persistFlagKey) === 'true') {
        const saved = sessionStorage.getItem(storageKey);
        if (saved) return JSON.parse(saved);
      }
    } catch (e) { /* ignore */ }
    return { ...initialFilters };
  });

  const [appliedFilters, setAppliedFilters] = useState<Record<string, unknown>>(filters);
  const [persistFilters, setPersistFilters] = useState(
    () => localStorage.getItem(persistFlagKey) === 'true',
  );
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Загрузка метаданных
  const fetchMeta = useCallback(async (currentFilters: Record<string, unknown>) => {
    setMeta(prev => ({ ...prev, loading: true }));
    try {
      const json = await promoAPI.getFilters(currentFilters) as Record<string, string[]>;
      setMeta({
        kam: json.kam || [], brand: json.brand || [], sku: json.sku || [],
        network_name: json.network_name || [], mechanics: json.mechanics || [],
        channel: json.channel || [], status: json.status || [],
        loading: false, error: null,
      });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setMeta(prev => ({ ...prev, loading: false, error: message }));
    }
  }, []);

  // Первичная загрузка
  useEffect(() => { fetchMeta(filters); }, []);

  // Обновление с debounce при изменении фильтров
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => fetchMeta(filters), 300);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [filters, fetchMeta]);

  const handleSearch = useCallback(() => {
    setAppliedFilters({ ...filters });
    // Если галочка включена — сохраняем фильтры в sessionStorage
    if (localStorage.getItem(persistFlagKey) === 'true') {
      sessionStorage.setItem(storageKey, JSON.stringify(filters));
    }
  }, [filters, persistFlagKey, storageKey]);

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
    fetchMeta,
  };
}