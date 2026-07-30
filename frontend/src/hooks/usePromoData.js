import { useState, useEffect, useCallback, useRef } from 'react';

export function usePromoData(filters, refreshKey) {
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const abortRef = useRef(null);

  const fetchData = useCallback(async () => {
    if (abortRef.current) abortRef.current.abort();

    const controller = new AbortController();
    abortRef.current = controller;

    setLoading(true);
    setError(null);

    try {
      const params = new URLSearchParams();
      Object.entries(filters).forEach(([key, value]) => {
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
  }, [JSON.stringify(filters)]);

  useEffect(() => {
    fetchData();
    return () => {
      if (abortRef.current) abortRef.current.abort();
    };
  }, [fetchData, refreshKey]);

  return { rows, loading, error, refetch: fetchData };
}