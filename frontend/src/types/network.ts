// Типы реестра сетей.
//
// Ответы описаны в Go (backend/models/network.go) и собираются генератором
// в ./api.generated.ts. Здесь остаётся только то, чего в Go нет: сужения
// значений для интерфейса и тело запроса на сохранение.

import type { AuditLogRow, NetworkPlanInput } from './api.generated';

export type {
  Network,
  NetworkPeriod,
  NetworkPlan,
  NetworkPlanTotals,
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
} from './api.generated';

// В Go тип сети — строка с проверкой на стороне сервера; интерфейсу нужен выбор
// из двух значений, поэтому сужение живёт здесь.
export type NetworkType = 'regular' | 'warehouse';

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
}
