// СГЕНЕРИРОВАНО backend/cmd/tsgen — НЕ РЕДАКТИРОВАТЬ ВРУЧНУЮ.
//
// Источник — Go-структуры в backend/models и backend/repository.
// Пересобрать: make types (из корня проекта).
// CI проверяет, что файл совпадает с исходниками: make types-check.

export interface SalesRow {
  id: number;
  year: number;
  month: number;
  brandName: string;
  productName: string;
  networkName: string;
  metricType: string;
  metricValue: number;
  un_rub: string | null;
  segment: string | null;
  channel: string | null;
  updated_at: string | null;
}

export interface DrilldownRow {
  year: number;
  month: number;
  metricType: string;
  totalValue: number;
  un_rub: string | null;
  segment: string | null;
  channel: string | null;
}

export interface SalesFilterOptions {
  year: string[];
  brandName: string[];
  productName: string[];
  networkName: string[];
  un_rub: string[];
  segment: string[];
  channel: string[];
  segmentChannelMap: Record<string, string[]>;
  channelSegmentMap: Record<string, string[]>;
}

export interface SalesDataResponse {
  data: SalesRow[];
  totalRows?: number;
}

export interface SalesNetworkOptionsResponse {
  networkName: string[];
}

export interface DrilldownResponse {
  brandName: string;
  networkName: string;
  data: DrilldownRow[];
}

export interface SalesDashboardPoint {
  year: number;
  month: number;
  value: number;
}

export interface SalesDashboardRank {
  name: string;
  value: number;
}

export interface SalesDashboardSeriesPoint {
  name: string;
  year: number;
  month: number;
  value: number;
}

export interface SalesDashboardFocusPoint {
  type: string;
  name: string;
  year: number;
  month: number;
  value: number;
}

export interface SalesDashboardNetworkBreakdown {
  network: string;
  channel: string;
  segment: string;
  value: number;
}

export interface SalesDashboardMetricComparison {
  current: number;
  previous: number;
}

export interface SalesDashboardMetricComparisons {
  rub: SalesDashboardMetricComparison;
  eur: SalesDashboardMetricComparison;
  units: SalesDashboardMetricComparison;
}

export interface SalesDashboardDriver {
  name: string;
  current: number;
  previous: number;
  delta: number;
  deltaPercent: number | null;
}

export interface SalesDashboardRankDetail {
  name: string;
  value: number;
  previous: number;
  yoyPercent: number | null;
  share: number;
  rank: number;
  rankChange: number;
}

export interface SalesDashboardEcomShare {
  applicable: boolean;
  family: string;
  full: number;
  withoutEcom: number;
  ecom: number;
  share: number | null;
  previousFull: number;
  previousEcom: number;
  previousShare: number | null;
}

export interface SalesDashboardSummary {
  total: number;
  averagePerMonth: number;
  activeNetworks: number;
  activeProducts: number;
  periods: number;
  latestYear: number;
  latestMonth: number;
  latestValue: number | null;
  previousValue: number | null;
  yearAgoValue: number | null;
}

export interface SalesDashboardResponse {
  analysisYear: number;
  channel: string;
  channelSegments: string[];
  segment: string;
  segments: string[];
  unit: string;
  summary: SalesDashboardSummary;
  trend: SalesDashboardPoint[];
  previousYearTrend: SalesDashboardPoint[];
  metricComparisons: SalesDashboardMetricComparisons;
  currencySource: string;
  ecomShare: SalesDashboardEcomShare;
  networkDrivers: SalesDashboardDriver[];
  productDrivers: SalesDashboardDriver[];
  networkRanking: SalesDashboardRankDetail[];
  productRanking: SalesDashboardRankDetail[];
  focusTrends: SalesDashboardFocusPoint[];
  topNetworks: SalesDashboardRank[];
  topProducts: SalesDashboardRank[];
  segmentTotals: SalesDashboardRank[];
  networkTrends: SalesDashboardSeriesPoint[];
  channelTrends: SalesDashboardSeriesPoint[];
  networkBreakdown: SalesDashboardNetworkBreakdown[];
}

export interface Network {
  id: number;
  name: string;
  kam: string | null;
  network_type: string;
  is_active: boolean;
  created_at: string | null;
  updated_at: string;
}

export interface NetworkPeriod {
  id: number;
  network_id: number;
  year: number;
  quarter: number;
  vat_included: boolean;
  vat_rate: number;
  updated_at: string;
}

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

export interface AuditLogRow {
  id: number;
  entity_type: string;
  entity_id: number;
  user_name: string;
  action_type: string;
  changed_fields: string | null;
  created_at: string | null;
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

export interface NetworkPlanPreviewResponse {
  year: number;
  periods: NetworkPeriod[];
  plans: NetworkPlan[];
  totals: NetworkPlanTotals[];
  year_totals: NetworkPlanTotals;
}

export interface NetworkListResponse {
  data: Network[];
}

export interface NetworkSaveResponse {
  message: string;
  data: Network;
}

export interface NetworkCommentsResponse {
  message?: string;
  data: NetworkComment[];
}

export interface NetworkAuditResponse {
  data: AuditLogRow[];
}

export interface NetworkBrandsResponse {
  data: string[];
}

export interface NetworkPlanInput {
  quarter: number;
  brand_as: string | null;
  in_gross: boolean;
  plan_rub: number | null;
  forecast_rub: number | null;
  investments_pct: number | null;
  updated_at: string;
}

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

export interface CommentRow {
  id: number;
  promo_id: number;
  user_name: string;
  role: string;
  comment_text: string;
  created_at: string | null;
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

export interface NetworkGeo {
  kam: string;
  network_type: string;
  top20_segment: string;
  key_region: string;
}

export interface LastSKUData {
  contract_price: number;
  gm: number;
  total_pharmacies: number;
  key_region: string;
  top20_segment: string;
  olap_price: number;
}
