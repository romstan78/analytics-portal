const API_BASE = 'http://localhost:8080';

function getToken() {
  return localStorage.getItem('token');
}

function fetchWithAuth(url, options = {}, timeout = 15000) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeout);

  const token = getToken();
  const headers = {
    'Content-Type': 'application/json',
    ...(options.headers || {}),
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  return fetch(url, {
    ...options,
    headers,
    signal: controller.signal,
  }).finally(() => clearTimeout(timer));
}

function buildParams(filters) {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (Array.isArray(value)) {
      value.forEach(v => { if (v !== '' && v != null) params.append(key, String(v)); });
    } else if (value !== '' && value != null) {
      params.set(key, String(value));
    }
  });
  return params.toString();
}

export const promoAPI = {
  getFilters: (filters = {}) =>
    fetchWithAuth(`${API_BASE}/api/promo/filters?${buildParams(filters)}`).then(r => r.json()),

  getData: (filters = {}) =>
    fetchWithAuth(`${API_BASE}/api/promo/data?all=true&${buildParams(filters)}`).then(r => r.json()),

  getHistory: (params = {}) =>
    fetchWithAuth(`${API_BASE}/api/promo/history?${new URLSearchParams(params)}`).then(r => r.json()),

  save: (data) =>
    fetchWithAuth(`${API_BASE}/api/promo/save`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }).then(async r => {
      const json = await r.json();
      if (!r.ok) throw { status: r.status, message: json.error || 'Ошибка сохранения' };
      return json;
    }),

    delete: (id) =>
      fetchWithAuth(`${API_BASE}/api/promo/${id}`, { method: 'DELETE' }).then(async r => {
        if (!r.ok) {
          const data = await r.json().catch(() => ({}));
          throw new Error(data.error || `HTTP ${r.status}`);
        }
        return r.json();
      }),

  getSKUByBrand: (brand) =>
    fetchWithAuth(`${API_BASE}/api/promo/sku-by-brand?brand=${encodeURIComponent(brand)}`).then(r => r.json()),

  getSKUInfo: (sku) =>
    fetchWithAuth(`${API_BASE}/api/promo/sku-info?sku=${encodeURIComponent(sku)}`).then(r => r.json()),

  getLastSKUData: (sku) =>
    fetchWithAuth(`${API_BASE}/api/promo/last-sku-data?sku=${encodeURIComponent(sku)}`).then(r => r.json()),

  getKAMByNetwork: (network) =>
    fetchWithAuth(`${API_BASE}/api/promo/kam-by-network?network=${encodeURIComponent(network)}`).then(r => r.json()),

  getLastNetworkData: (network) =>
    fetchWithAuth(`${API_BASE}/api/promo/last-network-data?network=${encodeURIComponent(network)}`).then(r => r.json()),

  getNetworkGeo: (network) =>
    fetchWithAuth(`${API_BASE}/api/promo/network-geo?network=${encodeURIComponent(network)}`).then(r => r.json()),

  getInvestmentTypes: () =>
    fetchWithAuth(`${API_BASE}/api/promo/investment-types`).then(r => r.json()),

  getLastContractPrice: (sku) =>
    fetchWithAuth(`${API_BASE}/api/promo/last-contract-price?sku=${encodeURIComponent(sku)}`).then(r => r.json()),
};

export const salesAPI = {
  getFilters: () =>
    fetchWithAuth(`${API_BASE}/api/filters`).then(r => r.json()),

  getData: (filters = {}) =>
    fetchWithAuth(`${API_BASE}/api/data?${buildParams(filters)}`).then(r => r.json()),

  getDrilldown: (params = {}) =>
    fetchWithAuth(`${API_BASE}/api/drilldown?${new URLSearchParams(params)}`).then(r => r.json()),
};