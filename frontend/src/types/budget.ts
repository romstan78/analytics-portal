export type { BudgetResponse, BudgetBrand, BudgetCell, BudgetLine, BudgetVersion, BudgetSources, BudgetPromo, BudgetPromoResponse } from './api.generated';
export interface BudgetParams {
  year: number;
  composition?: string;
  expand?: string[];
  version: string;
  compare?: string;
  delta?: string;
  types?: string[];
  src: string;
  kind: string;
  vat: string;
  base?: string;
  to_sources: string;
  investment_sources: string;
}
export interface BudgetCellInput {
  brand: string;
  quarter: number;
  metric: string;
  value: number;
  updated_at: string;
}
