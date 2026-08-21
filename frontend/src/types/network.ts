// Типы реестра сетей. Соответствуют backend/models/network.go
// и gin.H в backend/handlers/network.go.

export type NetworkType = 'regular' | 'warehouse';
export type ContractType = 'regular' | 'gross';

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

// models.NetworkPeriod — настройки квартала: НДС (применяется к инвестициям) и тип контракта
export interface NetworkPeriod {
  id: number;
  network_id: number;
  year: number;
  quarter: number;
  vat_included: boolean;
  vat_rate: number;
  contract_type: ContractType;
  updated_at: string;
}

// models.NetworkPlan — строка плана; brand_as = null — общий объём валового контракта
export interface NetworkPlan {
  id: number;
  network_id: number;
  year: number;
  quarter: number;
  brand_as: string | null;
  plan_rub: number | null;
  plan_units: number | null;
  investments_pct: number | null;
  investments_rub: number | null;
  investments_rub_net: number | null;
  updated_by: string | null;
  updated_at: string;
}

// services.NetworkPlanTotals
export interface NetworkPlanTotals {
  quarter: number;
  plan_rub: number;
  gross_plan_rub: number | null;
  undistributed: number | null;
  investments_rub: number;
  investments_rub_net: number;
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
}

export interface NetworkPlanSaveResponse {
  message: string;
  year: number;
  periods: NetworkPeriod[];
  plans: NetworkPlan[];
  totals: NetworkPlanTotals[];
}

// Строка плана в запросе на сохранение
export interface NetworkPlanInput {
  quarter: number;
  brand_as: string | null;
  plan_rub: number | null;
  investments_pct: number | null;
  updated_at: string;
}

export interface NetworkPlanSaveRequest {
  year: number;
  periods: Array<{
    quarter: number;
    vat_included: boolean;
    vat_rate: number;
    contract_type: ContractType;
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
