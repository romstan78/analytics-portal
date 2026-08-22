// Клиент API реестра сетей.
// Общие хелперы (авторизация, разбор ответа, query string) берём из ./promo,
// чтобы не дублировать логику refresh-токена.

import { fetchWithAuth, parseJSONResponse, buildParams } from './promo';
import type {
  NetworkAuditResponse,
  NetworkBrandsResponse,
  NetworkCommentsResponse,
  NetworkListResponse,
  NetworkPlanResponse,
  NetworkPlanSaveRequest,
  NetworkPlanSaveResponse,
  NetworkSaveResponse,
} from '../types/network';

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080';

// ─── Реестр сетей ─────────────────────────────────────────────────────────
export const networkAPI = {
  // Список сетей реестра
  getNetworks: (params: { search?: string; kam?: string; include_inactive?: string } = {}): Promise<NetworkListResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks?${buildParams(params)}`)
      .then(r => parseJSONResponse<NetworkListResponse>(r, 'Ошибка загрузки списка сетей')),

  // Бренды для строк плана (планы ведутся по брендам, не по СКЮ)
  getBrands: (): Promise<NetworkBrandsResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks/brands`)
      .then(r => parseJSONResponse<NetworkBrandsResponse>(r, 'Ошибка загрузки брендов')),

  // Новая сеть; year открывает первый год сразу четырьмя кварталами
  create: (data: {
    name: string;
    kam?: string;
    network_type: string;
    vat_included?: boolean;
    vat_rate?: number;
    year?: number;
  }): Promise<NetworkSaveResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks`, { method: 'POST', body: JSON.stringify(data) })
      .then(r => parseJSONResponse<NetworkSaveResponse>(r, 'Ошибка создания сети')),

  // Карточка сети; updated_at — версия для контроля конкурентной правки
  update: (id: number, data: {
    name?: string;
    kam?: string;
    network_type?: string;
    is_active?: boolean;
    updated_at: string;
  }): Promise<NetworkSaveResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks/${id}`, { method: 'PATCH', body: JSON.stringify(data) })
      .then(r => parseJSONResponse<NetworkSaveResponse>(r, 'Ошибка сохранения сети')),

  // Планы, кварталы и итоги за год
  getPlan: (id: number, year: number): Promise<NetworkPlanResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks/${id}/plan?year=${year}`)
      .then(r => parseJSONResponse<NetworkPlanResponse>(r, 'Ошибка загрузки планов')),

  savePlan: (id: number, data: NetworkPlanSaveRequest): Promise<NetworkPlanSaveResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks/${id}/plan`, { method: 'POST', body: JSON.stringify(data) })
      .then(r => parseJSONResponse<NetworkPlanSaveResponse>(r, 'Ошибка сохранения планов')),

  // Комментарии: без года/квартала/бренда — ко всей сети
  getComments: (id: number): Promise<NetworkCommentsResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks/${id}/comments`)
      .then(r => parseJSONResponse<NetworkCommentsResponse>(r, 'Ошибка загрузки комментариев')),

  addComment: (id: number, data: {
    comment_text: string;
    year?: number | null;
    quarter?: number | null;
    brand_as?: string | null;
  }): Promise<NetworkCommentsResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks/${id}/comments`, { method: 'POST', body: JSON.stringify(data) })
      .then(r => parseJSONResponse<NetworkCommentsResponse>(r, 'Ошибка сохранения комментария')),

  // История изменений карточки и планов
  getAudit: (id: number): Promise<NetworkAuditResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks/${id}/audit`)
      .then(r => parseJSONResponse<NetworkAuditResponse>(r, 'Ошибка загрузки истории')),
};

