// Типы ответов /api/data, /api/filters, /api/sales/*, /api/drilldown.
// Соответствуют backend/models/types.go и gin.H в backend/handlers/sales.go.

// models.Row
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

// models.DrilldownRow
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
  // Отсутствует при all=true (экспорт возвращает выборку целиком).
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

// ─── Дашборд ───────────────────────────────────────────────────────────────

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
  metricComparisons: {
    rub: SalesDashboardMetricComparison;
    eur: SalesDashboardMetricComparison;
    units: SalesDashboardMetricComparison;
  };
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
