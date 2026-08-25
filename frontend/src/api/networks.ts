// Клиент API реестра сетей.
// Общие хелперы (авторизация, разбор ответа, query string) берём из ./promo,
// чтобы не дублировать логику refresh-токена.

import { fetchWithAuth, parseJSONResponse, buildParams } from './promo';
import type {
  NetworkAuditResponse,
  NetworkBrandsResponse,
  NetworkCommentsResponse,
  NetworkForecastResponse,
  NetworkForecastClearResponse,
  NetworkForecastClearScope,
  NetworkForecastImportPreview,
  NetworkForecastImportResponse,
  NetworkForecastSaveRequest,
  NetworkForecastSaveResponse,
  NetworkEntryLevel,
  NetworkEntryUnit,
  NetworkListResponse,
  NetworkPlanPreviewResponse,
  NetworkPlanResponse,
  NetworkPlanSaveRequest,
  NetworkPlanSaveResponse,
  NetworkPricesResponse,
  NetworkPricesSaveRequest,
  NetworkPricesSaveResponse,
  NetworkSaveResponse,
} from '../types/network';

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080';

// ─── Реестр сетей ─────────────────────────────────────────────────────────
export const networkAPI = {
  // Список сетей реестра
  getNetworks: (params: { search?: string; kam?: string; include_inactive?: string } = {}): Promise<NetworkListResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks?${buildParams(params)}`)
      .then(r => parseJSONResponse<NetworkListResponse>(r, 'Ошибка загрузки списка сетей')),

  // Бренды для строк плана (планы ведутся по брендам, не по SKU)
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
    vat_included?: boolean;
    vat_rate?: number;
    month1_pct?: number;
    month2_pct?: number;
    month3_pct?: number;
    has_annual_investment_cumulative?: boolean;
    year?: number;
    periods?: Array<{
      quarter: number;
      vat_included: boolean;
      vat_rate: number;
    }>;
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

  // Пересчёт черновика до сохранения. НДС, инвестиции и итоги считает только
  // бэкенд — интерфейс показывает то, что вернул этот запрос.
  previewPlan: (id: number, data: NetworkPlanSaveRequest): Promise<NetworkPlanPreviewResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks/${id}/plan/preview`, { method: 'POST', body: JSON.stringify(data) })
      .then(r => parseJSONResponse<NetworkPlanPreviewResponse>(r, 'Ошибка пересчёта планов')),

  getForecast: (id: number, year: number, quarter: number): Promise<NetworkForecastResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks/${id}/forecast?year=${year}&quarter=${quarter}`)
      .then(r => parseJSONResponse<NetworkForecastResponse>(r, 'Ошибка загрузки прогноза')),

  saveForecast: (id: number, data: NetworkForecastSaveRequest): Promise<NetworkForecastSaveResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks/${id}/forecast`, { method: 'POST', body: JSON.stringify(data) })
      .then(r => parseJSONResponse<NetworkForecastSaveResponse>(r, 'Ошибка сохранения прогноза')),

  // Режим ведения бренда хранится на строке плана, но переключается и отсюда:
  // именно в прогнозе видно, что бренд удобнее вести иначе. Ответ — пересчитанный
  // квартал, потому что смена режима меняет, какие строки считаются введёнными.
  setEntryMode: (id: number, data: {
    year: number;
    quarter: number;
    brand_as: string;
    entry_level: NetworkEntryLevel;
    entry_unit: NetworkEntryUnit;
  }): Promise<NetworkForecastSaveResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks/${id}/entry-mode`, {
      method: 'POST', body: JSON.stringify(data),
    }).then(r => parseJSONResponse<NetworkForecastSaveResponse>(r, 'Ошибка смены режима ведения')),

  previewForecastImport: (id: number, year: number, quarter: number, file: File): Promise<NetworkForecastImportPreview> => {
    const form = new FormData();
    form.append('file', file);
    return fetchWithAuth(`${API_BASE}/api/networks/${id}/forecast/import/preview?year=${year}&quarter=${quarter}`, {
      method: 'POST', body: form,
    }, 60000).then(r => parseJSONResponse<NetworkForecastImportPreview>(r, 'Ошибка проверки Excel-файла'));
  },

  importForecast: (id: number, year: number, quarter: number, file: File): Promise<NetworkForecastImportResponse> => {
    const form = new FormData();
    form.append('file', file);
    return fetchWithAuth(`${API_BASE}/api/networks/${id}/forecast/import?year=${year}&quarter=${quarter}`, {
      method: 'POST', body: form,
    }, 60000).then(r => parseJSONResponse<NetworkForecastImportResponse>(r, 'Ошибка импорта прогноза'));
  },

  clearForecastMonth: (id: number, data: {
    year: number;
    month: number;
    scope: NetworkForecastClearScope;
  }): Promise<NetworkForecastClearResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks/${id}/forecast/clear`, {
      method: 'POST', body: JSON.stringify(data),
    }).then(r => parseJSONResponse<NetworkForecastClearResponse>(r, 'Ошибка очистки прогноза')),

  getPrices: (id: number, year: number): Promise<NetworkPricesResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks/${id}/prices?year=${year}`)
      .then(r => parseJSONResponse<NetworkPricesResponse>(r, 'Ошибка загрузки цен')),

  savePrices: (id: number, data: NetworkPricesSaveRequest): Promise<NetworkPricesSaveResponse> =>
    fetchWithAuth(`${API_BASE}/api/networks/${id}/prices`, { method: 'POST', body: JSON.stringify(data) })
      .then(r => parseJSONResponse<NetworkPricesSaveResponse>(r, 'Ошибка сохранения цен')),

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
