import { useState, useEffect, useCallback, useRef } from 'react';

export function usePromoData(filters, refreshTrigger) {
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const abortRef = useRef(null);

  // Сравниваем фильтры через JSON — только реальные изменения триггерят запрос
  const filtersRef = useRef(filters);
  const [fetchTrigger, setFetchTrigger] = useState(0);

  useEffect(() => {
    const prev = JSON.stringify(filtersRef.current);
    const next = JSON.stringify(filters);
    if (prev !== next) {
      filtersRef.current = filters;
      setFetchTrigger(t => t + 1);
    }
  }, [filters]);

  const fetchData = useCallback(async () => {
    if (abortRef.current) abortRef.current.abort();

    const controller = new AbortController();
    abortRef.current = controller;

    setLoading(true);
    setError(null);

    try {
      const params = new URLSearchParams();
      const currentFilters = filtersRef.current;
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
          signal: controller.signal,
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('token')}`,
          },
        }
      );

      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const json = await response.json();
      setRows(json.data || []);
    } catch (err) {
      if (err.name !== 'AbortError') {
        setError(err.message);
      }
    } finally {
      setLoading(false);
    }
  }, []); // ← стабильная ссылка, не пересоздаётся

  useEffect(() => {
    fetchData();
    return () => {
      if (abortRef.current) abortRef.current.abort();
    };
  }, [fetchTrigger, refreshTrigger, fetchData]);

  return { rows, setRows, loading, error, refetch: fetchData };
}