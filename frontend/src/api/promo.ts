import { refreshToken, logout } from './auth';
import type {
  ApprovalFiltersResponse,
  ApprovalsResponse,
  BatchApproveResponse,
  FilterOptions,
  LastContractPriceResponse,
  LastNetworkDataResponse,
  LastSKUDataResponse,
  MessageResponse,
  NetworkGeoResponse,
  PromoCommentsResponse,
  PromoDataResponse,
  PromoHistoryResponse,
  PromoRow,
  PromoSaveResponse,
  SKUInfoResponse,
  StringListResponse,
} from '../types/promo';
import type {
  DrilldownResponse,
  SalesDashboardResponse,
  SalesDataResponse,
  SalesFilterOptions,
  SalesNetworkOptionsResponse,
} from '../types/sales';

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080';

interface ApiError {
  status: number;
  message: string;
}

// ─── Promise Lock для refreshToken (предотвращает шторм 401) ──────────────
let isRefreshing = false;
let refreshPromise: Promise<boolean> | null = null;

async function refreshTokenOnce(): Promise<boolean> {
  if (!isRefreshing) {
    isRefreshing = true;
    refreshPromise = refreshToken().finally(() => {
      isRefreshing = false;
      refreshPromise = null;
    });
  }
  return refreshPromise!;
}

// ─── Утилита: fetch с авторизацией и таймаутом ──────────────────────────────
export async function fetchWithAuth(url: string, options: RequestInit = {}, timeout = 15000): Promise<Response> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeout);

  const doFetch = (): Promise<Response> => {
    const token = localStorage.getItem('token');
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string> || {}),
    };
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    return fetch(url, { ...options, headers, signal: controller.signal });
  };

  let res = await doFetch();

  // При 401 пробуем обновить токен и повторить (с Promise Lock)
  if (res.status === 401) {
    const refreshed = await refreshTokenOnce();
    if (refreshed) {
      res = await doFetch();
    } else {
      // Рефреш не удался — полный logout и редирект
      logout();
      window.location.replace('/login');
      // Заглушка с валидным JSON, чтобы не ломать вызывающий код при разборе
      return new Response('{}', {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      });
    }
  }

  clearTimeout(timer);
  return res;
}

export async function parseJSONResponse<T>(response: Response, fallbackMessage: string): Promise<T> {
  const json = await response.json().catch(() => ({})) as Record<string, unknown>;
  if (!response.ok) {
    throw {
      status: response.status,
      message: typeof json.error === 'string' ? json.error : fallbackMessage,
    } as ApiError;
  }
  return json as T;
}

// Разбор ответов, у которых backend не отдаёт поле error с текстом.
async function readJSON<T>(response: Response): Promise<T> {
  return await response.json() as T;
}

// ─── Утилита: построение query string ──────────────────────────────────────
export function buildParams(filters: Record<string, unknown>): string {
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

// ─── Типы для параметров API ───────────────────────────────────────────────
export interface ApprovalParams {
  approval_role?: 'agreement1' | 'agreement2';
  kam?: string;
  approval_status?: string;
  year?: string;
  month?: string;
  network_name?: string;
  brand?: string;
  mechanics?: string;
  has_comments?: boolean;
}

export interface ApprovalFiltersParams {
  approval_role?: 'agreement1' | 'agreement2';
  approval_status?: string;
  kam?: string;
  network_name?: string;
  brand?: string;
  mechanics?: string;
  year?: string;
  month?: string;
}

export interface ApprovalItemVersion {
  id: number;
  updated_at: string;
}

// ─── API: Промо ────────────────────────────────────────────────────────────
export const promoAPI = {
  // Справочники фильтров
  getFilters: (filters: Record<string, unknown> = {}): Promise<FilterOptions> =>
    fetchWithAuth(`${API_BASE}/api/promo/filters?${buildParams(filters)}`).then(readJSON<FilterOptions>),

  // Данные промо
  getData: (filters: Record<string, unknown> = {}): Promise<PromoDataResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/data?all=true&${buildParams(filters)}`).then(readJSON<PromoDataResponse>),

  getById: (id: number): Promise<PromoRow> =>
    fetchWithAuth(`${API_BASE}/api/promo/${id}`).then(async r => {
      const data = await r.json() as PromoRow & { error?: string };
      if (!r.ok) throw new Error(data.error || `HTTP ${r.status}`);
      return data;
    }),

  // История промо по SKU/сети/механике
  getHistory: (params: Record<string, string> = {}): Promise<PromoHistoryResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/history?${new URLSearchParams(params)}`).then(readJSON<PromoHistoryResponse>),

  // Сохранение (INSERT / UPDATE)
  save: (data: unknown): Promise<PromoSaveResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/save`, {
      method: 'POST',
      body: JSON.stringify(data),
    }).then(async r => {
      const json = await r.json() as PromoSaveResponse & { error?: string };
      if (!r.ok) throw { status: r.status, message: json.error || 'Ошибка сохранения' } as ApiError;
      return json;
    }),

  // Удаление (soft-delete)
  delete: (id: number): Promise<MessageResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/${id}`, { method: 'DELETE' }).then(async r => {
      if (!r.ok) {
        const data = await r.json().catch(() => ({})) as { error?: string };
        throw new Error(data.error || `HTTP ${r.status}`);
      }
      return await r.json() as MessageResponse;
    }),

  // Восстановление soft-deleted записи
  restore: (id: number): Promise<MessageResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/${id}/restore`, { method: 'PATCH' }).then(async r => {
      const json = await r.json() as MessageResponse & { error?: string };
      if (!r.ok) throw { status: r.status, message: json.error || 'Ошибка восстановления' } as ApiError;
      return json;
    }),

  // Комментарии к промо
  getComments: (promoId: number): Promise<PromoCommentsResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/comments/${promoId}`).then(readJSON<PromoCommentsResponse>),

  // SKU по бренду
  getSKUByBrand: (brand: string): Promise<StringListResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/sku-by-brand?brand=${encodeURIComponent(brand)}`).then(readJSON<StringListResponse>),

  // Информация о SKU (бренд)
  getSKUInfo: (sku: string): Promise<SKUInfoResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/sku-info?sku=${encodeURIComponent(sku)}`).then(readJSON<SKUInfoResponse>),

  // Последние данные по SKU
  getLastSKUData: (sku: string): Promise<LastSKUDataResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/last-sku-data?sku=${encodeURIComponent(sku)}`).then(readJSON<LastSKUDataResponse>),

  // KAM по сети
  getKAMByNetwork: (network: string): Promise<StringListResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/kam-by-network?network=${encodeURIComponent(network)}`).then(readJSON<StringListResponse>),

  // Гео-маппинг сети (KAM, регион, сегмент ТОП-20)
  getNetworkGeo: (network: string): Promise<NetworkGeoResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/network-geo?network=${encodeURIComponent(network)}`).then(readJSON<NetworkGeoResponse>),

  // Последние данные по сети (аптеки)
  getLastNetworkData: (network: string): Promise<LastNetworkDataResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/last-network-data?network=${encodeURIComponent(network)}`).then(readJSON<LastNetworkDataResponse>),

  // Типы инвестиций
  getInvestmentTypes: (): Promise<StringListResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/investment-types`).then(readJSON<StringListResponse>),

  // Последняя цена контракта по SKU
  getLastContractPrice: (sku: string): Promise<LastContractPriceResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/last-contract-price?sku=${encodeURIComponent(sku)}`).then(readJSON<LastContractPriceResponse>),

  // ─── Согласование ──────────────────────────────────────────────────────

  // Список KAM'ов с промо на согласовании
  getApprovalKAMs: (approvalRole?: string): Promise<StringListResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/approval-kams?approval_role=${encodeURIComponent(approvalRole || '')}`).then(readJSON<StringListResponse>),

  // Сети для выбранного KAM (в согласовании)
  getApprovalNetworks: (kam: string, approvalRole?: string): Promise<StringListResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/approval-networks?kam=${encodeURIComponent(kam)}&approval_role=${encodeURIComponent(approvalRole || '')}`).then(readJSON<StringListResponse>),

  // Бренды для KAM + сети (в согласовании)
  getApprovalBrands: (kam: string, network = '', approvalRole?: string): Promise<StringListResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/approval-brands?kam=${encodeURIComponent(kam)}&network_name=${encodeURIComponent(network)}&approval_role=${encodeURIComponent(approvalRole || '')}`).then(readJSON<StringListResponse>),

  // Список промо на согласование
  getApprovals: (params: ApprovalParams & { page?: number; pageSize?: number } = {}): Promise<ApprovalsResponse> => {
    const qs = new URLSearchParams();
    if (params.approval_role) qs.set('approval_role', params.approval_role);
    if (params.kam) qs.set('kam', params.kam);
    if (params.approval_status) qs.set('approval_status', params.approval_status);
    else qs.set('approval_status', 'pending');
    if (params.year) qs.set('year', params.year);
    if (params.month) qs.set('month', params.month);
    if (params.network_name) qs.set('network_name', params.network_name);
    if (params.brand) qs.set('brand', params.brand);
    if (params.mechanics) qs.set('mechanics', params.mechanics);
    if (params.has_comments) qs.set('has_comments', '1');
    if (params.page !== undefined) qs.set('page', String(params.page));
    if (params.pageSize !== undefined) qs.set('pageSize', String(params.pageSize));
    return fetchWithAuth(`${API_BASE}/api/promo/approvals?${qs}`)
      .then(r => parseJSONResponse<ApprovalsResponse>(r, 'Ошибка загрузки промо на согласование'));
  },

  // Справочники сетей/брендов/механик для страницы согласования
  getApprovalFilters: (params: ApprovalFiltersParams = {}): Promise<ApprovalFiltersResponse> => {
    const qs = new URLSearchParams();
    if (params.approval_role) qs.set('approval_role', params.approval_role);
    qs.set('approval_status', params.approval_status || 'pending');
    if (params.kam) qs.set('kam', params.kam);
    if (params.network_name) qs.set('network_name', params.network_name);
    if (params.brand) qs.set('brand', params.brand);
    if (params.mechanics) qs.set('mechanics', params.mechanics);
    if (params.year) qs.set('year', params.year);
    if (params.month) qs.set('month', params.month);
    return fetchWithAuth(`${API_BASE}/api/promo/approval-filters?${qs}`)
      .then(r => parseJSONResponse<ApprovalFiltersResponse>(r, 'Ошибка загрузки фильтров согласования'));
  },

  // Действие согласования: comment / согласовано / отклонено
  approve: (id: number, updatedAt: string, status: string, comment = '', approvalRole?: string): Promise<MessageResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/approve`, {
      method: 'POST',
      body: JSON.stringify({ id, updated_at: updatedAt, status, comment, approval_role: approvalRole }),
    }).then(async r => {
      const json = await r.json() as MessageResponse & { error?: string };
      if (!r.ok) throw { status: r.status, message: json.error || 'Ошибка' } as ApiError;
      return json;
    }),

  // Массовое согласование
  batchApprove: (items: ApprovalItemVersion[], status: string, comment = '', approvalRole?: string): Promise<BatchApproveResponse> =>
    fetchWithAuth(`${API_BASE}/api/promo/approve/batch`, {
      method: 'POST',
      body: JSON.stringify({ items, status, comment, approval_role: approvalRole }),
    }).then(async r => {
      const json = await r.json() as BatchApproveResponse & { error?: string };
      if (!r.ok) throw { status: r.status, message: json.error || 'Ошибка' } as ApiError;
      return json;
    }),
};

// ─── API: Интернет-продажи ─────────────────────────────────────────────────
export const salesAPI = {
  getFilters: (): Promise<SalesFilterOptions> =>
    fetchWithAuth(`${API_BASE}/api/filters`).then(readJSON<SalesFilterOptions>),

  getData: (filters: Record<string, unknown> = {}): Promise<SalesDataResponse> =>
    fetchWithAuth(`${API_BASE}/api/data?${buildParams(filters)}`).then(readJSON<SalesDataResponse>),

  getDashboard: (filters: Record<string, unknown> = {}): Promise<SalesDashboardResponse> =>
    fetchWithAuth(`${API_BASE}/api/sales/dashboard?${buildParams(filters)}`).then(r => parseJSONResponse<SalesDashboardResponse>(r, 'Ошибка загрузки дашборда')),

  getNetworkOptions: (filters: Record<string, unknown> = {}): Promise<SalesNetworkOptionsResponse> =>
    fetchWithAuth(`${API_BASE}/api/sales/network-options?${buildParams(filters)}`).then(r => parseJSONResponse<SalesNetworkOptionsResponse>(r, 'Ошибка загрузки списка сетей')),

  // buildParams разворачивает массивы в повторяющиеся параметры (months, segment, channel).
  getDrilldown: (params: Record<string, unknown> = {}): Promise<DrilldownResponse> =>
    fetchWithAuth(`${API_BASE}/api/drilldown?${buildParams(params)}`).then(readJSON<DrilldownResponse>),
};
