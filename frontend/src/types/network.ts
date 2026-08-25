// Типы реестра сетей.
//
// Ответы описаны в Go (backend/models/network.go) и собираются генератором
// в ./api.generated.ts. Здесь остаётся только то, чего в Go нет: сужения
// значений для интерфейса и тело запроса на сохранение.

import type {
  AuditLogRow,
  NetworkContractPriceInput,
  NetworkForecastInput,
  NetworkPeriodGroupInput,
  NetworkPlanInput,
} from './api.generated';

export type {
  Network,
  NetworkPeriod,
  NetworkPlan,
  NetworkPlanTotals,
  NetworkAnnualInvestmentRow,
  NetworkAnnualInvestmentCumulative,
  NetworkPeriodGroup,
  NetworkPeriodGroupTotals,
  NetworkPeriodGroupInput,
  NetworkComment,
  NetworkPlanInput,
  NetworkPlanResponse,
  NetworkPlanSaveResponse,
  NetworkPlanPreviewResponse,
  NetworkListResponse,
  NetworkSaveResponse,
  NetworkCommentsResponse,
  NetworkAuditResponse,
  NetworkBrandsResponse,
  NetworkMonthlyFact,
  NetworkForecastLine,
  NetworkPromoIndicator,
  NetworkForecastMonth,
  NetworkForecastBrandTotals,
  NetworkForecastTotals,
  NetworkForecastResponse,
  NetworkForecastSaveResponse,
  NetworkContractPrice,
  NetworkPriceSKUOption,
  NetworkPricesResponse,
  NetworkPricesSaveResponse,
  NetworkForecastInput,
  NetworkContractPriceInput,
} from './api.generated';

// В Go тип сети — строка с проверкой на стороне сервера; интерфейсу нужен выбор
// из двух значений, поэтому сужение живёт здесь.
export type NetworkType = 'regular' | 'warehouse';

// Откуда взялась сумма инвестиций в строке прогноза. В Go это строка с
// проверкой на стороне сервера; форме нужен закрытый список.
export type NetworkInvestmentsSource = 'fact' | 'pct' | 'override' | 'none';

// Режим ведения бренда: на каком уровне вводят значения и в какой единице.
// Один и тот же для вкладок «Планы» и «Прогноз».
export type NetworkEntryLevel = 'brand' | 'sku';
export type NetworkEntryUnit = 'rub' | 'units';

// Запись аудита для entity_type network / network_plan.
export type NetworkAuditRow = AuditLogRow;

// Разбор changed_fields для истории планов: в Go это строка с JSON,
// структуру которой знает только клиент.
export interface NetworkPlanChange {
  quarter: number;
  brand?: string;
  field: string;
  old: number | string | boolean | null;
  new: number | string | boolean | null;
}

// Тело POST /api/networks/:id/plan и .../plan/preview.
export interface NetworkPlanSaveRequest {
  year: number;
  periods: Array<{
    quarter: number;
    vat_included: boolean;
    vat_rate: number;
  }>;
  plans: NetworkPlanInput[];
  period_groups: NetworkPeriodGroupInput[];
}

export interface NetworkForecastSaveRequest {
  year: number;
  quarter: number;
  lines: NetworkForecastInput[];
}

export interface NetworkForecastImportIssue {
  row: number;
  message: string;
}

export interface NetworkForecastImportPreview {
  file_name: string;
  rows: number;
  valid_rows: number;
  added_rows: number;
  updated_rows: number;
  unchanged_rows: number;
  affected_brands: number;
  errors: NetworkForecastImportIssue[];
  warnings: NetworkForecastImportIssue[];
}

export interface NetworkForecastImportResponse {
  message: string;
  imported_rows: number;
  data: import('./api.generated').NetworkForecastResponse;
}

export type NetworkForecastClearScope = 'rub' | 'units' | 'all';

export interface NetworkForecastClearResponse {
  message: string;
  cleared_rows: number;
  data: import('./api.generated').NetworkForecastResponse;
}

export interface NetworkPricesSaveRequest {
  year: number;
  rows: NetworkContractPriceInput[];
  deleted_rows: NetworkContractPriceDeleteInput[];
}

export interface NetworkContractPriceDeleteInput {
  id: number;
  updated_at: string;
}
