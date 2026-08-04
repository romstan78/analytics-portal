import { refreshToken } from './auth';

const API_BASE = 'http://localhost:8080';

// ─── Утилита: fetch с авторизацией и таймаутом ──────────────────────────────
async function fetchWithAuth(url, options = {}, timeout = 15000) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeout);

  const doFetch = () => {
    const token = localStorage.getItem('token');
    const headers = {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    };
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    return fetch(url, { ...options, headers, signal: controller.signal });
  };

  let res = await doFetch();

  // При 401 пробуем обновить токен и повторить
  if (res.status === 401) {
    const refreshed = await refreshToken();
    if (refreshed) {
      res = await doFetch();
    }
  }

  clearTimeout(timer);
  return res;
}

// ─── Утилита: построение query string ──────────────────────────────────────
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

// ─── API: Промо ────────────────────────────────────────────────────────────
export const promoAPI = {
  // Справочники фильтров
  getFilters: (filters = {}) =>
    fetchWithAuth(`${API_BASE}/api/promo/filters?${buildParams(filters)}`).then(r => r.json()),

  // Данные промо
  getData: (filters = {}) =>
    fetchWithAuth(`${API_BASE}/api/promo/data?all=true&${buildParams(filters)}`).then(r => r.json()),

  // История промо по SKU/сети/механике
  getHistory: (params = {}) =>
    fetchWithAuth(`${API_BASE}/api/promo/history?${new URLSearchParams(params)}`).then(r => r.json()),

  // Сохранение (INSERT / UPDATE)
  save: (data) =>
    fetchWithAuth(`${API_BASE}/api/promo/save`, {
      method: 'POST',
      body: JSON.stringify(data),
    }).then(async r => {
      const json = await r.json();
      if (!r.ok) throw { status: r.status, message: json.error || 'Ошибка сохранения' };
      return json;
    }),

  // Удаление (soft-delete)
  delete: (id) =>
    fetchWithAuth(`${API_BASE}/api/promo/${id}`, { method: 'DELETE' }).then(async r => {
      if (!r.ok) {
        const data = await r.json().catch(() => ({}));
        throw new Error(data.error || `HTTP ${r.status}`);
      }
      return r.json();
    }),

  // SKU по бренду
  getSKUByBrand: (brand) =>
    fetchWithAuth(`${API_BASE}/api/promo/sku-by-brand?brand=${encodeURIComponent(brand)}`).then(r => r.json()),

  // Информация о SKU (бренд)
  getSKUInfo: (sku) =>
    fetchWithAuth(`${API_BASE}/api/promo/sku-info?sku=${encodeURIComponent(sku)}`).then(r => r.json()),

  // Последние данные по SKU
  getLastSKUData: (sku) =>
    fetchWithAuth(`${API_BASE}/api/promo/last-sku-data?sku=${encodeURIComponent(sku)}`).then(r => r.json()),

  // KAM по сети
  getKAMByNetwork: (network) =>
    fetchWithAuth(`${API_BASE}/api/promo/kam-by-network?network=${encodeURIComponent(network)}`).then(r => r.json()),

  // Гео-маппинг сети (KAM, регион, сегмент ТОП-20)
  getNetworkGeo: (network) =>
    fetchWithAuth(`${API_BASE}/api/promo/network-geo?network=${encodeURIComponent(network)}`).then(r => r.json()),

  // Последние данные по сети (аптеки)
  getLastNetworkData: (network) =>
    fetchWithAuth(`${API_BASE}/api/promo/last-network-data?network=${encodeURIComponent(network)}`).then(r => r.json()),

  // Типы инвестиций
  getInvestmentTypes: () =>
    fetchWithAuth(`${API_BASE}/api/promo/investment-types`).then(r => r.json()),

  // Последняя цена контракта по SKU
  getLastContractPrice: (sku) =>
    fetchWithAuth(`${API_BASE}/api/promo/last-contract-price?sku=${encodeURIComponent(sku)}`).then(r => r.json()),

  // ─── Согласование ──────────────────────────────────────────────────────

  // Список KAM'ов с промо на согласовании
  getApprovalKAMs: () =>
    fetchWithAuth(`${API_BASE}/api/promo/approval-kams`).then(r => r.json()),

  // Сети для выбранного KAM (в согласовании)
  getApprovalNetworks: (kam) =>
    fetchWithAuth(`${API_BASE}/api/promo/approval-networks?kam=${encodeURIComponent(kam)}`).then(r => r.json()),

  // Бренды для KAM + сети (в согласовании)
  getApprovalBrands: (kam, network = '') =>
    fetchWithAuth(`${API_BASE}/api/promo/approval-brands?kam=${encodeURIComponent(kam)}&network_name=${encodeURIComponent(network)}`).then(r => r.json()),

  // Список промо на согласование
  getApprovals: (params = {}) => {
    const qs = new URLSearchParams();
    if (params.kam) qs.set('kam', params.kam);
    if (params.approval_status) qs.set('approval_status', params.approval_status);
    else qs.set('approval_status', 'pending');
    if (params.year) qs.set('year', params.year);
    if (params.month) qs.set('month', params.month);
    if (params.network_name) qs.set('network_name', params.network_name);
    if (params.brand) qs.set('brand', params.brand);
    if (params.mechanics) qs.set('mechanics', params.mechanics);
    return fetchWithAuth(`${API_BASE}/api/promo/approvals?${qs}`).then(r => r.json());
  },

  // Справочники сетей/брендов/механик для страницы согласования
  getApprovalFilters: (params = {}) => {
    const qs = new URLSearchParams();
    qs.set('approval_status', params.approval_status || 'pending');
    if (params.kam) qs.set('kam', params.kam);
    if (params.network_name) qs.set('network_name', params.network_name);
    if (params.brand) qs.set('brand', params.brand);
    if (params.mechanics) qs.set('mechanics', params.mechanics);
    if (params.year) qs.set('year', params.year);
    if (params.month) qs.set('month', params.month);
    return fetchWithAuth(`${API_BASE}/api/promo/approval-filters?${qs}`).then(r => r.json());
  },

  // Действие согласования: comment / согласовано / отклонено
  approve: (id, status, comment = '') =>
    fetchWithAuth(`${API_BASE}/api/promo/approve`, {
      method: 'POST',
      body: JSON.stringify({ id, status, comment }),
    }).then(async r => {
      const json = await r.json();
      if (!r.ok) throw { status: r.status, message: json.error || 'Ошибка' };
      return json;
    }),

  // Массовое согласование
  batchApprove: (ids, status, comment = '') =>
    fetchWithAuth(`${API_BASE}/api/promo/approve/batch`, {
      method: 'POST',
      body: JSON.stringify({ ids, status, comment }),
    }).then(async r => {
      const json = await r.json();
      if (!r.ok) throw { status: r.status, message: json.error || 'Ошибка' };
      return json;
    }),
};

// ─── API: Интернет-продажи ─────────────────────────────────────────────────
export const salesAPI = {
  getFilters: () =>
    fetchWithAuth(`${API_BASE}/api/filters`).then(r => r.json()),

  getData: (filters = {}) =>
    fetchWithAuth(`${API_BASE}/api/data?${buildParams(filters)}`).then(r => r.json()),

  getDrilldown: (params = {}) =>
    fetchWithAuth(`${API_BASE}/api/drilldown?${new URLSearchParams(params)}`).then(r => r.json()),
};