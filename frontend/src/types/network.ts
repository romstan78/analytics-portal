// Типы реестра сетей. Соответствуют backend/models/network.go
// и gin.H в backend/handlers/network.go.

export type NetworkType = 'regular' | 'warehouse';

// models.Network
export interface Network {
  id: number;
  name: string;
  kam: string | null;
  network_type: NetworkType;
  is_active: boolean;
  created_at: string | null;
  updated_at: string;
}

// models.NetworkPeriod — настройки квартала: НДС применяется только к инвестициям.
// Тип контракта здесь не хранится: валовый объём — свойство бренда (NetworkPlan.in_gross).
export interface NetworkPeriod {
  id: number;
  network_id: number;
  year: number;
  quarter: number;
  vat_included: boolean;
  vat_rate: number;
  updated_at: string;
}

// models.NetworkPlan — строка плана; brand_as = null — общий объём валового контракта (пул),
// в который входят бренды с in_gross. Остальные бренды планируются отдельно.
export interface NetworkPlan {
  id: number;
  network_id: number;
  year: number;
  quarter: number;
  brand_as: string | null;
  in_gross: boolean;
  plan_rub: number | null;
  plan_units: number | null;
  fact_rub: number | null;
  forecast_rub: number | null;
  fact_investments_rub: number | null;
  fact_investments_rub_net: number | null;
  investments_pct: number | null;
  investments_rub: number | null;
  investments_rub_net: number | null;
  forecast_investments_rub: number | null;
  forecast_investments_rub_net: number | null;
  updated_by: string | null;
  updated_at: string;
}

// services.NetworkPlanTotals
export interface NetworkPlanTotals {
  quarter: number;
  plan_rub: number;
  gross_brands_plan: number;
  separate_plan_rub: number;
  gross_pool_rub: number | null;
  undistributed: number | null;
  contract_plan_rub: number;
  gross_brands_count: number;
  fact_rub: number;
  forecast_rub: number;
  gross_pool_fact_rub: number;
  gross_pool_forecast_rub: number | null;
  investments_rub: number;
  investments_rub_net: number;
  forecast_investments_rub: number;
  forecast_investments_rub_net: number;
  fact_investments_rub: number;
  fact_investments_rub_net: number;
}

// models.NetworkComment
export interface NetworkComment {
  id: number;
  network_id: number;
  year: number | null;
  quarter: number | null;
  brand_as: string | null;
  user_name: string;
  role: string;
  comment_text: string;
  created_at: string | null;
}

// models.AuditLogRow для entity_type network / network_plan
export interface NetworkAuditRow {
  id: number;
  entity_type: string;
  entity_id: number;
  user_name: string;
  action_type: string;
  changed_fields: string | null;
  created_at: string | null;
}

// Разбор changed_fields для истории планов
export interface NetworkPlanChange {
  quarter: number;
  brand?: string;
  field: string;
  old: number | string | boolean | null;
  new: number | string | boolean | null;
}

export interface NetworkPlanResponse {
  network: Network;
  year: number;
  periods: NetworkPeriod[];
  plans: NetworkPlan[];
  totals: NetworkPlanTotals[];
  year_totals: NetworkPlanTotals;
}

export interface NetworkPlanSaveResponse {
  message: string;
  year: number;
  periods: NetworkPeriod[];
  plans: NetworkPlan[];
  totals: NetworkPlanTotals[];
  year_totals: NetworkPlanTotals;
}

// Пересчёт несохранённого черновика: то же, что вернётся после сохранения.
export interface NetworkPlanPreviewResponse {
  year: number;
  periods: NetworkPeriod[];
  plans: NetworkPlan[];
  totals: NetworkPlanTotals[];
  year_totals: NetworkPlanTotals;
}

// Строка плана в запросе на сохранение.
// Факта здесь нет: он приходит загрузкой отгрузок и в интерфейсе не правится.
export interface NetworkPlanInput {
  quarter: number;
  brand_as: string | null;
  in_gross: boolean;
  plan_rub: number | null;
  forecast_rub: number | null;
  investments_pct: number | null;
  updated_at: string;
}

export interface NetworkPlanSaveRequest {
  year: number;
  periods: Array<{
    quarter: number;
    vat_included: boolean;
    vat_rate: number;
  }>;
  plans: NetworkPlanInput[];
}

export interface NetworkListResponse {
  data: Network[];
}

export interface NetworkCommentsResponse {
  data: NetworkComment[];
  message?: string;
}

export interface NetworkAuditResponse {
  data: NetworkAuditRow[];
}

export interface NetworkBrandsResponse {
  data: string[];
}

export interface NetworkSaveResponse {
  message: string;
  data: Network;
}
