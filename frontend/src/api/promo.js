const API_BASE = 'http://localhost:8080';

// Утилита для fetch с AbortController
function fetchWithAbort(url, options = {}, timeout = 15000) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeout);
  
  return fetch(url, {
    ...options,
    signal: controller.signal,
  }).finally(() => clearTimeout(timer));
}

// Построение query string из объекта фильтров
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
  // Справочники фильтров
  getFilters: (filters = {}) =>
    fetchWithAbort(`${API_BASE}/api/promo/filters?${buildParams(filters)}`).then(r => r.json()),

  // Данные промо
  getData: (filters = {}) =>
    fetchWithAbort(`${API_BASE}/api/promo/data?all=true&${buildParams(filters)}`).then(r => r.json()),

  // История
  getHistory: (params = {}) =>
    fetchWithAbort(`${API_BASE}/api/promo/history?${new URLSearchParams(params)}`).then(r => r.json()),

  // Сохранение
  save: (data) =>
    fetchWithAbort(`${API_BASE}/api/promo/save`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }).then(r => r.json()),

  // Удаление
  delete: (id) =>
    fetchWithAbort(`${API_BASE}/api/promo/${id}`, { method: 'DELETE' }).then(r => r.json()),

  // SKU по бренду
  getSKUByBrand: (brand) =>
    fetchWithAbort(`${API_BASE}/api/promo/sku-by-brand?brand=${encodeURIComponent(brand)}`).then(r => r.json()),

  // Информация о SKU
  getSKUInfo: (sku) =>
    fetchWithAbort(`${API_BASE}/api/promo/sku-info?sku=${encodeURIComponent(sku)}`).then(r => r.json()),

  // Последние данные по SKU
  getLastSKUData: (sku) =>
    fetchWithAbort(`${API_BASE}/api/promo/last-sku-data?sku=${encodeURIComponent(sku)}`).then(r => r.json()),

  // KAM по сети
  getKAMByNetwork: (network) =>
    fetchWithAbort(`${API_BASE}/api/promo/kam-by-network?network=${encodeURIComponent(network)}`).then(r => r.json()),

  // Последние данные по сети
  getLastNetworkData: (network) =>
    fetchWithAbort(`${API_BASE}/api/promo/last-network-data?network=${encodeURIComponent(network)}`).then(r => r.json()),

  // Типы инвестиций
  getInvestmentTypes: () =>
    fetchWithAbort(`${API_BASE}/api/promo/investment-types`).then(r => r.json()),

  // Цена контракта
  getLastContractPrice: (sku) =>
    fetchWithAbort(`${API_BASE}/api/promo/last-contract-price?sku=${encodeURIComponent(sku)}`).then(r => r.json()),

  getNetworkGeo: (network) =>
    fetchWithAbort(`${API_BASE}/api/promo/network-geo?network=${encodeURIComponent(network)}`).then(r => r.json()),
};

// Для интернет-продаж
export const salesAPI = {
  getFilters: () =>
    fetchWithAbort(`${API_BASE}/api/filters`).then(r => r.json()),

  getData: (filters = {}) =>
    fetchWithAbort(`${API_BASE}/api/data?${buildParams(filters)}`).then(r => r.json()),

  getDrilldown: (params = {}) =>
    fetchWithAbort(`${API_BASE}/api/drilldown?${new URLSearchParams(params)}`).then(r => r.json()),
};