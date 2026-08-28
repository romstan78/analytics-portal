// Типы промо: строки — из Go, конверты ответов и формы — здесь.

// Строки промо описаны в Go (backend/models/types.go) и собираются генератором
// в ./api.generated.ts. Ниже — только конверты ответов и формы,
// которых в Go нет.
import type { ApprovalRow, CommentRow, HistoryRow, PromoRow } from './api.generated';

export type {
  PromoRow,
  CommentRow,
  HistoryRow,
  ApprovalRow,
  NetworkGeo,
  LastSKUData,
  PromoDashboardMetrics,
  PromoDashboardTrendPoint,
  PromoDashboardBreakdown,
  PromoDashboardCalendarPoint,
  PromoDashboardResponse,
  PromoApprovalAccessResponse,
} from './api.generated';

export interface FilterOptions {
  kam: string[];
  brand: string[];
  sku: string[];
  network_name: string[];
  mechanics: string[];
  status: string[];
  channel: string[];
}

export interface PromoFormData {
  network_name?: string;
  kam?: string;
  brand?: string;
  brand_as?: string;
  sku?: string;
  year?: number;
  month?: number;
  mechanics?: string;
  gtn_opex?: string;
  id_directum?: string;
  ds_number?: string;
  discount_amount?: number;
  conditions?: string;
  comments?: string;
  ecom_segment?: string;
  total_pharmacies?: number;
  promo_pharmacies?: number;
  status?: string;
  baseline_units?: number;
  plan_promo_units?: number;
  plan_investments_rub?: number;
  contract_price?: number;
  gm?: number;
  key_region?: string;
  top20_segment?: string;
  actual_promo_sales_units?: number;
  actual_promo_rub?: number;
  actual_investments?: number;
  actual_promo_uplift_units?: number;
  actual_promo_uplift_rub?: number;
  actual_external_ecom_units?: number;
  actual_corrected_baseline?: number;
}

// ─── Ответы API промо ──────────────────────────────────────────────────────
// Соответствуют gin.H в backend/handlers/promo.go.

export interface PromoDataResponse {
  data: PromoRow[];
  // Размер всей выборки. Приходит только в постраничном режиме: при all=true
  // строки и так пришли целиком.
  totalRows?: number;
}

export interface PromoHistoryResponse {
  data: HistoryRow[];
}

export interface PromoCommentsResponse {
  data: CommentRow[];
}

export interface StringListResponse {
  data: string[];
}

export interface PromoSaveResponse {
  message: string;
  id: number;
  data: Record<string, unknown>;
}

export interface MessageResponse {
  message: string;
}

export interface BatchApproveResponse {
  message: string;
  affected: number;
}

export interface SKUInfoResponse {
  brand: string | null;
  brand_as: string | null;
}

export interface LastContractPriceResponse {
  price: number | null;
}

// Пустой объект, если данных по SKU нет.
export interface LastSKUDataResponse {
  contract_price?: number;
  gm?: number;
  total_pharmacies?: number;
  key_region?: string;
  top20_segment?: string;
  olap_price?: number;
}

// Пустой объект, если сеть не передана; total_pharmacies = 0, если данных нет.
export interface LastNetworkDataResponse {
  total_pharmacies?: number;
}

export interface NetworkGeoResponse {
  kam: string | null;
  network_type: string | null;
  top20_segment: string | null;
  key_region: string | null;
}

export interface ApprovalsResponse {
  data: ApprovalRow[];
  total: number;
  // Ступень, на которой сервер выполнил запрос. У делегированного КАМа она
  // следует из закрепления и может отличаться от отправленной.
  approval_role?: string;
}

export interface ApprovalFiltersResponse {
  networks: string[];
  brands: string[];
  mechanics: string[];
  kams: string[];
}
