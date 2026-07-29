import { useState, useEffect, useCallback, useRef } from 'react';
import { promoAPI } from '../api/promo';

export function usePromoFilters(initialFilters, storageKey, persistFlagKey) {
  const [meta, setMeta] = useState({
    kam: [], brand: [], sku: [], network_name: [], mechanics: [], channel: [], status: [],
    loading: true, error: null
  });
  
  const [filters, setFilters] = useState(() => {
    try {
      if (localStorage.getItem(persistFlagKey) === 'true') {
        const saved = sessionStorage.getItem(storageKey);
        if (saved) return JSON.parse(saved);
      }
    } catch (e) {}
    return { ...initialFilters };
  });

  const [appliedFilters, setAppliedFilters] = useState(filters);
  const [persistFilters, setPersistFilters] = useState(
    () => localStorage.getItem(persistFlagKey) === 'true'
  );
  const debounceRef = useRef(null);

  // Загрузка метаданных
  const fetchMeta = useCallback(async (currentFilters) => {
    setMeta(prev => ({ ...prev, loading: true }));
    try {
      const json = await promoAPI.getFilters(currentFilters);
      setMeta({
        kam: json.kam || [], brand: json.brand || [], sku: json.sku || [],
        network_name: json.network_name || [], mechanics: json.mechanics || [],
        channel: json.channel || [], status: json.status || [],
        loading: false, error: null
      });
    } catch (err) {
      setMeta(prev => ({ ...prev, loading: false, error: err.message }));
    }
  }, []);

  // Первичная загрузка
  useEffect(() => { fetchMeta(filters); }, []);

  // Обновление с debounce при изменении фильтров
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => fetchMeta(filters), 300);
    return () => clearTimeout(debounceRef.current);
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

  const handlePersistChange = useCallback((checked) => {
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
    fetchMeta
  };
}