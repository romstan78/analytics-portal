// Типы, соответствующие backend/models/types.go

export interface PromoRow {
  id: number;
  network_name: string | null;
  kam: string | null;
  id_directum: string | null;
  ds_number: string | null;
  year: number;
  month: number | null;
  quarter: number | null;
  sku: string | null;
  brand: string | null;
  brand_as: string | null;
  mechanics: string | null;
  discount_amount: number | null;
  gtn_opex: string | null;
  conditions: string | null;
  comments: string | null;
  baseline_units: number | null;
  baseline_rub: number | null;
  plan_promo_units: number | null;
  plan_promo_rub: number | null;
  plan_investments_rub: number | null;
  plan_promo_uplift_units: number | null;
  plan_promo_uplift_rub: number | null;
  plan_promo_uplift_pct_units: number | null;
  plan_promo_uplift_pct_rub: number | null;
  plan_investments_pct: number | null;
  plan_roi: number | null;
  contract_price: number | null;
  gm: number | null;
  total_pharmacies: number | null;
  promo_pharmacies: number | null;
  actual_promo_sales_units: number | null;
  actual_investments: number | null;
  status: string | null;
  actual_promo_rub: number | null;
  actual_promo_uplift_units: number | null;
  actual_promo_uplift_rub: number | null;
  actual_external_ecom_units: number | null;
  actual_corrected_baseline: number | null;
  actual_roi: number | null;
  plan_vs_fact_rub: number | null;
  plan_vs_fact_investments: number | null;
  channel: string | null;
  agreement1: string | null;
  agreement2: string | null;
  date: string | null;
  created_at: string | null;
  updated_at: string | null;
  deleted_at: string | null;
}

export interface CommentRow {
  id: number;
  promo_id: number;
  user_name: string;
  role: string;
  comment_text: string;
  created_at: string | null;
}

// models.HistoryRow — история промо по SKU/сети/механике
export interface HistoryRow {
  id: number;
  network_name: string | null;
  year: number;
  month: number;
  mechanics: string | null;
  sku: string | null;
  baseline_units: number | null;
  plan_promo_units: number | null;
  actual_promo_sales_units: number | null;
  plan_investments_rub: number | null;
  actual_investments: number | null;
  plan_promo_uplift_units: number | null;
  actual_promo_uplift_units: number | null;
  plan_roi: number | null;
  actual_roi: number | null;
}

export interface ApprovalRow {
  id: number;
  network_name: string | null;
  brand_as: string | null;
  sku: string | null;
  mechanics: string | null;
  year: number;
  month: number | null;
  baseline_units: number | null;
  plan_promo_units: number | null;
  actual_promo_sales_units: number | null;
  plan_investments_rub: number | null;
  plan_roi: number | null;
  actual_roi: number | null;
  conditions: string | null;
  comments: string | null;
  agreement1: string | null;
  agreement1_status: string | null;
  agreement1_comment: string | null;
  agreement2: string | null;
  agreement2_status: string | null;
  agreement2_comment: string | null;
  status: string | null;
  historical_count: number;
  avg_historical_roi: number | null;
  updated_at: string;
}

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
}

export interface ApprovalFiltersResponse {
  networks: string[];
  brands: string[];
  mechanics: string[];
  kams: string[];
}
