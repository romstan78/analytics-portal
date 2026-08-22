// Типы ответов /api/data, /api/filters, /api/sales/*, /api/drilldown.
//
// Описаны в Go (backend/models/sales.go, backend/models/types.go) и собираются
// генератором в ./api.generated.ts. Здесь только реэкспорт, чтобы импорты
// компонентов не зависели от того, откуда взялся тип.

export type {
  SalesRow,
  DrilldownRow,
  SalesFilterOptions,
  SalesDataResponse,
  SalesNetworkOptionsResponse,
  DrilldownResponse,
  SalesPivotPeriod,
  SalesPivotNode,
  SalesPivotResponse,
  SalesDashboardPoint,
  SalesDashboardRank,
  SalesDashboardSeriesPoint,
  SalesDashboardFocusPoint,
  SalesDashboardNetworkBreakdown,
  SalesDashboardMetricComparison,
  SalesDashboardMetricComparisons,
  SalesDashboardDriver,
  SalesDashboardRankDetail,
  SalesDashboardEcomShare,
  SalesDashboardSummary,
  SalesDashboardResponse,
} from './api.generated';
