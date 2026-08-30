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
  kam: string[];
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

export interface SalesPivotPeriod {
  key: string;
  label: string;
  year: number;
  kind: string;
}

export interface SalesPivotNode {
  id: string;
  level: string;
  name: string;
  values: Record<string, number>;
  children: SalesPivotNode[];
}

export interface SalesPivotResponse {
  analysisYear: number;
  channel: string;
  segments: string[];
  unit: string;
  granularity: string;
  currencySource: string;
  periods: SalesPivotPeriod[];
  rows: SalesPivotNode[];
  totals: Record<string, number>;
  previousTotalKey: string;
  currentTotalKey: string;
  leafRows: number;
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
  brandDrivers: SalesDashboardDriver[];
  productDrivers: SalesDashboardDriver[];
  networkRanking: SalesDashboardRankDetail[];
  productRanking: SalesDashboardRankDetail[];
  focusTrends: SalesDashboardFocusPoint[];
  topNetworks: SalesDashboardRank[];
  topProducts: SalesDashboardRank[];
  segmentTotals: SalesDashboardRank[];
  segmentTrends: SalesDashboardSeriesPoint[];
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
  vat_included: boolean;
  vat_rate: number;
  month1_pct: number;
  month2_pct: number;
  month3_pct: number;
  has_annual_investment_cumulative: boolean;
  default_entry_level: string;
  default_entry_unit: string;
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
  entry_level: string;
  entry_unit: string;
  month1_pct: number;
  month2_pct: number;
  month3_pct: number;
  fact_rub: number | null;
  forecast_rub: number | null;
  investments_pct: number | null;
  pay_investments_from_fact: boolean;
  investments_rub: number | null;
  investments_rub_net: number | null;
  forecast_investments_rub: number | null;
  forecast_investments_rub_net: number | null;
  fact_investments_rub: number | null;
  fact_investments_rub_net: number | null;
  paid_investments_rub: number | null;
  forecast_investments_overridden: boolean;
  investment_scope: string;
  investment_period_start_quarter: number;
  investment_period_end_quarter: number;
  forecast_completion_pct: number | null;
  forecast_investments_earned: boolean;
  fact_completion_pct: number | null;
  fact_investments_earned: boolean;
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
  eac_rub: number;
  completion_pct: number | null;
  completed: boolean;
}

export interface NetworkPeriodGroup {
  id: number;
  network_id: number;
  year: number;
  start_quarter: number;
  end_quarter: number;
  brand_as: string | null;
  updated_by: string | null;
  updated_at: string;
}

export interface NetworkPeriodGroupTotals {
  start_quarter: number;
  end_quarter: number;
  brand_as: string | null;
  plan_rub: number;
  fact_rub: number;
  forecast_rub: number;
  investments_rub: number;
  investments_rub_net: number;
  forecast_investments_rub: number;
  forecast_investments_rub_net: number;
  fact_investments_rub: number;
  fact_investments_rub_net: number;
  eac_rub: number;
  completion_pct: number | null;
  completed: boolean;
}

export interface NetworkAnnualInvestmentRow {
  scope_type: string;
  brand_as: string | null;
  plan_rub: number;
  eac_rub: number;
  completion_pct: number | null;
  completed: boolean;
  eligible: boolean;
  accrued_investments_rub: number;
  accrued_investments_rub_net: number;
  fact_based_accrued_investments_rub: number;
  fact_based_accrued_investments_rub_net: number;
  paid_investments_rub: number;
  paid_investments_rub_net: number;
  q4_forecast_investments_rub: number;
  q4_forecast_investments_rub_net: number;
  supplement_rub: number;
  supplement_rub_net: number;
}

export interface NetworkAnnualInvestmentCumulative {
  portfolio_plan_rub: number;
  portfolio_eac_rub: number;
  portfolio_completion_pct: number | null;
  portfolio_completed: boolean;
  rows: NetworkAnnualInvestmentRow[];
  total_supplement_rub: number;
  total_supplement_rub_net: number;
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
  period_groups: NetworkPeriodGroup[];
  period_group_totals: NetworkPeriodGroupTotals[];
  annual_investment_cumulative?: NetworkAnnualInvestmentCumulative;
}

export interface NetworkPlanSaveResponse {
  message: string;
  year: number;
  periods: NetworkPeriod[];
  plans: NetworkPlan[];
  totals: NetworkPlanTotals[];
  year_totals: NetworkPlanTotals;
  period_groups: NetworkPeriodGroup[];
  period_group_totals: NetworkPeriodGroupTotals[];
  annual_investment_cumulative?: NetworkAnnualInvestmentCumulative;
}

export interface NetworkPlanPreviewResponse {
  year: number;
  periods: NetworkPeriod[];
  plans: NetworkPlan[];
  totals: NetworkPlanTotals[];
  year_totals: NetworkPlanTotals;
  period_groups: NetworkPeriodGroup[];
  period_group_totals: NetworkPeriodGroupTotals[];
  annual_investment_cumulative?: NetworkAnnualInvestmentCumulative;
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

export interface NetworkKAMsResponse {
  data: string[];
}

export interface NetworkMonthlyFact {
  id: number;
  network_id: number;
  year: number;
  month: number;
  brand_as: string;
  sku: string | null;
  fact_rub: number | null;
  fact_units: number | null;
  fact_investments_rub: number | null;
  is_final: boolean;
  source_name: string | null;
  updated_at: string;
}

export interface NetworkForecastLine {
  id: number;
  network_id: number;
  year: number;
  month: number;
  brand_as: string;
  sku: string | null;
  forecast_rub: number | null;
  forecast_units: number | null;
  forecast_investments_rub: number | null;
  system_forecast_rub: number | null;
  system_forecast_units: number | null;
  confidence: string | null;
  adjustment_reason: string | null;
  updated_by: string | null;
  updated_at: string;
}

export interface NetworkPromoIndicator {
  year: number;
  month: number;
  brand_as: string;
  promo_count: number;
  approved_count: number;
  draft_count: number;
  plan_promo_units: number;
  plan_promo_rub: number;
  plan_investments_rub: number;
  plan_uplift_rub: number;
  plan_uplift_units: number;
}

export interface NetworkForecastMonth {
  year: number;
  quarter: number;
  month: number;
  brand_as: string;
  sku: string | null;
  contract_price: number | null;
  plan_rub: number | null;
  plan_investments_rub: number | null;
  investments_pct: number | null;
  investments_source: string;
  entry_level: string;
  entry_unit: string;
  is_derived: boolean;
  fact_rub: number | null;
  fact_units: number | null;
  fact_investments_rub: number | null;
  forecast_rub: number | null;
  forecast_units: number | null;
  forecast_investments_rub: number | null;
  system_forecast_rub: number | null;
  system_forecast_units: number | null;
  eac_rub: number | null;
  eac_units: number | null;
  eac_investments_rub: number | null;
  confidence: string | null;
  adjustment_reason: string | null;
  promo_count: number;
  approved_promo_count: number;
  draft_promo_count: number;
  promo_plan_units: number;
  promo_plan_rub: number;
  promo_investments_rub: number;
  promo_uplift_rub: number;
  is_closed: boolean;
  is_current: boolean;
  updated_at: string;
}

export interface NetworkForecastBrandTotals {
  brand_as: string;
  plan_rub: number;
  fact_rub: number;
  fact_units: number;
  eac_rub: number;
  eac_units: number;
  completion_pct: number | null;
  gap_rub: number;
  plan_investments_rub: number;
  fact_investments_rub: number;
  eac_investments_rub: number;
  investment_variance_rub: number;
  promo_count: number;
}

export interface NetworkForecastTotals {
  plan_rub: number;
  fact_rub: number;
  fact_units: number;
  eac_rub: number;
  eac_units: number;
  completion_pct: number | null;
  gap_rub: number;
  plan_investments_rub: number;
  fact_investments_rub: number;
  eac_investments_rub: number;
  investment_variance_rub: number;
  promo_count: number;
}

export interface NetworkForecastResponse {
  network: Network;
  year: number;
  quarter: number;
  months: NetworkForecastMonth[];
  brands: NetworkForecastBrandTotals[];
  totals: NetworkForecastTotals;
}

export interface NetworkForecastSaveResponse {
  message: string;
  data: NetworkForecastResponse;
}

export interface NetworkContractPrice {
  id: number;
  network_id: number;
  brand_as: string;
  sku: string;
  contract_price: number;
  valid_from: string;
  valid_to: string;
  source_type: string;
  source_year: number | null;
  source_month: number | null;
  is_confirmed: boolean;
  olap_price: number | null;
  olap_year: number | null;
  olap_month: number | null;
  updated_by: string | null;
  updated_at: string;
}

export interface NetworkPriceSKUOption {
  brand_as: string;
  sku: string;
  price: number;
  source_year: number;
  source_month: number;
}

export interface NetworkPricesResponse {
  network: Network;
  year: number;
  data: NetworkContractPrice[];
  sku_options: NetworkPriceSKUOption[];
}

export interface NetworkPricesSaveResponse {
  message: string;
  data: NetworkPricesResponse;
}

export interface NetworkDashboardMetrics {
  networkCount: number;
  brandCount: number;
  planRub: number;
  factRub: number;
  planUnits: number;
  factUnits: number;
  eacUnits: number;
  eacRub: number;
  completionPct: number | null;
  eacCompletionPct: number | null;
  gapRub: number;
  gapUnits: number;
  planInvestmentsRub: number;
  planInvestmentsRubNet: number;
  factInvestmentsRub: number;
  factInvestmentsRubNet: number;
  eacInvestmentsRub: number;
  eacInvestmentsRubNet: number;
  investmentVarianceRub: number;
  effectiveInvestmentsPct: number | null;
  undistributedRub: number | null;
  closedCells: number;
  closedCellsWithFact: number;
  factCoveragePct: number | null;
  openCellsWithoutForecast: number;
  prevPlanRub: number | null;
  prevFactRub: number | null;
  prevFactUnits: number | null;
  factYoyPct: number | null;
  planYoyPct: number | null;
  promoCount: number;
  promoOnlineCount: number;
  promoOfflineCount: number;
  promoInvestmentsRub: number;
}

export interface NetworkDashboardPromoTag {
  code: string;
  mechanics: string;
  channel: string;
  count: number;
  planRub: number;
}

export interface NetworkDashboardPeriodPoint {
  year: number;
  quarter: number;
  metrics: NetworkDashboardMetrics;
}

export interface NetworkDashboardMonthPoint {
  year: number;
  month: number;
  quarter: number;
  planRub: number;
  planUnits: number;
  factRub: number;
  factUnits: number;
  eacRub: number;
  eacUnits: number;
  prevFactRub: number | null;
  prevFactUnits: number | null;
  promoCount: number;
  promoOnlineCount: number;
  promoOfflineCount: number;
  closed: boolean;
  cellsWithoutForecast: number;
}

export interface NetworkDashboardBreakdown {
  name: string;
  networkId: number | null;
  kam: string | null;
  metrics: NetworkDashboardMetrics;
  inGross: boolean | null;
}

export interface NetworkDashboardCell {
  networkId: number;
  name: string;
  quarter: number;
  metrics: NetworkDashboardMetrics;
  promoTags: NetworkDashboardPromoTag[];
}

export interface NetworkDashboardBrandQuarter {
  brand: string;
  quarter: number;
  metrics: NetworkDashboardMetrics;
  promoTags: NetworkDashboardPromoTag[];
}

export interface NetworkDashboardBrandMonth {
  brand: string;
  month: number;
  promoCount: number;
  promoOnlineCount: number;
  promoOfflineCount: number;
  promoInvestmentsRub: number;
  promoTags: NetworkDashboardPromoTag[];
}

export interface NetworkDashboardSKU {
  brand: string;
  sku: string;
  factRub: number;
  factUnits: number;
  eacRub: number;
  eacUnits: number;
  factInvestmentsRub: number;
  prevFactRub: number | null;
  prevFactUnits: number | null;
  factYoyPct: number | null;
  shareOfBrandPct: number | null;
}

export interface NetworkDashboardResponse {
  year: number;
  selectedQuarters: number[];
  availableYears: number[];
  summary: NetworkDashboardMetrics;
  quarters: NetworkDashboardPeriodPoint[];
  months: NetworkDashboardMonthPoint[];
  networks: NetworkDashboardBreakdown[];
  brands: NetworkDashboardBreakdown[];
  kams: NetworkDashboardBreakdown[];
  networkQuarters: NetworkDashboardCell[];
  brandQuarters: NetworkDashboardBrandQuarter[];
  brandMonths: NetworkDashboardBrandMonth[];
  skus: NetworkDashboardSKU[];
  annualInvestmentCumulative?: NetworkAnnualInvestmentCumulative;
}

export interface NetworkPlanInput {
  quarter: number;
  brand_as: string | null;
  in_gross: boolean;
  plan_rub: number | null;
  investments_pct: number | null;
  entry_level: string;
  entry_unit: string;
  updated_at: string;
}

export interface NetworkPeriodGroupInput {
  start_quarter: number;
  end_quarter: number;
  brand_as: string | null;
  updated_at: string;
}

export interface NetworkForecastInput {
  month: number;
  brand_as: string;
  sku: string | null;
  forecast_rub: number | null;
  forecast_units: number | null;
  forecast_investments_rub: number | null;
  adjustment_reason: string | null;
  updated_at: string;
}

export interface NetworkContractPriceInput {
  id: number;
  brand_as: string;
  sku: string;
  contract_price: number;
  valid_from: string;
  valid_to: string;
  is_confirmed: boolean;
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

export interface PromoDashboardMetrics {
  promoCount: number;
  factReadyCount: number;
  factCoveragePct: number | null;
  planUnits: number;
  comparablePlanUnits: number;
  actualUnits: number | null;
  planInvestmentsRub: number;
  comparablePlanInvestmentsRub: number;
  actualInvestmentsRub: number | null;
  effectiveInvestmentsRub: number;
  factInvestmentsCount: number;
  planUpliftUnits: number;
  comparablePlanUpliftUnits: number;
  actualUpliftUnits: number | null;
  planRoi: number | null;
  comparablePlanRoi: number | null;
  actualRoi: number | null;
  salesCompletionPct: number | null;
  investmentCompletionPct: number | null;
  salesVarianceUnits: number | null;
  investmentVarianceRub: number | null;
}

export interface PromoDashboardTrendPoint {
  year: number;
  month: number;
  metrics: PromoDashboardMetrics;
}

export interface PromoDashboardBreakdown {
  name: string;
  metrics: PromoDashboardMetrics;
}

export interface PromoDashboardCalendarPoint {
  name: string;
  year: number;
  month: number;
  metrics: PromoDashboardMetrics;
}

export interface PromoApprovalAccessResponse {
  allowed: boolean;
  approval_role: string;
  scoped: boolean;
}

export interface PromoDashboardResponse {
  availableYears: number[];
  summary: PromoDashboardMetrics;
  trend: PromoDashboardTrendPoint[];
  networks: PromoDashboardBreakdown[];
  brands: PromoDashboardBreakdown[];
  skus: PromoDashboardBreakdown[];
  mechanics: PromoDashboardBreakdown[];
  networkCalendar: PromoDashboardCalendarPoint[];
  brandCalendar: PromoDashboardCalendarPoint[];
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
