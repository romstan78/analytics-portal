// Расчёты сетки планов реестра сетей.
// Повторяют backend/services/network_service.go, чтобы итоги в таблице
// пересчитывались во время ввода, до сохранения.

import type { ContractType, NetworkPeriod, NetworkPlan } from '../types/network';

export const QUARTERS = [1, 2, 3, 4] as const;

export const round2 = (v: number): number => Math.round(v * 100) / 100;

// Ключ строки плана: квартал + бренд. Пустой бренд — общий объём валового контракта.
export const planKey = (quarter: number, brand: string | null): string => `${quarter}|${brand ?? ''}`;

// Сумма инвестиций с вычетом НДС. Сеть без НДС в этом квартале — сумма остаётся как есть.
// К планам НДС не применяется: план хранится и показывается так, как его ввели.
export function netRub(gross: number, vatIncluded: boolean, vatRate: number): number {
  if (!vatIncluded || vatRate <= 0) return round2(gross);
  return round2(gross / (1 + vatRate / 100));
}

// Разбор введённого числа: пробелы-разделители и запятая как в Excel.
// Пустая строка — значение снято, а не ноль.
export function parseNumberInput(raw: string): number | null {
  const cleaned = raw.replace(/\s/g, '').replace(',', '.').trim();
  if (cleaned === '') return null;
  const value = Number(cleaned);
  return Number.isFinite(value) ? value : null;
}

export function formatRub(value: number | null | undefined, digits = 0): string {
  if (value == null) return '—';
  return value.toLocaleString('ru-RU', { minimumFractionDigits: digits, maximumFractionDigits: digits });
}

export function formatPct(value: number | null | undefined): string {
  if (value == null) return '—';
  return value.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

// Значение строки сетки во время редактирования.
export interface DraftCell {
  planRub: string;
  investmentsPct: string;
}

export interface QuarterSettings {
  vat_included: boolean;
  vat_rate: number;
  contract_type: ContractType;
}

export interface QuarterTotals {
  quarter: number;
  planRub: number;
  investmentsRub: number;
  investmentsRubNet: number;
  grossPlanRub: number | null;
  undistributed: number | null;
}

// Итоги квартала по черновику: сумма планов по брендам, инвестиции до вычета
// и с вычетом НДС, остаток к распределению у валового контракта.
export function calcQuarterTotals(
  draft: Record<string, DraftCell>,
  brands: string[],
  settings: Record<number, QuarterSettings>,
): QuarterTotals[] {
  return QUARTERS.map((quarter) => {
    const setting = settings[quarter];
    const vatIncluded = setting?.vat_included ?? false;
    const vatRate = setting?.vat_rate ?? 0;

    let planRub = 0;
    let investmentsRub = 0;
    let investmentsRubNet = 0;

    brands.forEach((brand) => {
      const cell = draft[planKey(quarter, brand)];
      const value = cell ? parseNumberInput(cell.planRub) : null;
      if (value == null) return;
      planRub = round2(planRub + value);
      const pct = cell ? parseNumberInput(cell.investmentsPct) : null;
      if (pct != null) {
        const investments = round2((value * pct) / 100);
        investmentsRub = round2(investmentsRub + investments);
        investmentsRubNet = round2(investmentsRubNet + netRub(investments, vatIncluded, vatRate));
      }
    });

    let grossPlanRub: number | null = null;
    let undistributed: number | null = null;
    if (setting?.contract_type === 'gross') {
      const grossCell = draft[planKey(quarter, null)];
      grossPlanRub = grossCell ? parseNumberInput(grossCell.planRub) : null;
      if (grossPlanRub != null) undistributed = round2(grossPlanRub - planRub);
    }

    return { quarter, planRub, investmentsRub, investmentsRubNet, grossPlanRub, undistributed };
  });
}

// Черновик из загруженных строк плана: то, что показывается в полях ввода.
export function buildDraft(plans: NetworkPlan[]): Record<string, DraftCell> {
  const draft: Record<string, DraftCell> = {};
  plans.forEach((plan) => {
    draft[planKey(plan.quarter, plan.brand_as)] = {
      planRub: plan.plan_rub != null ? String(plan.plan_rub) : '',
      investmentsPct: plan.investments_pct != null ? String(plan.investments_pct) : '',
    };
  });
  return draft;
}

// Настройки кварталов: у года, который ещё не открывали, берутся значения по умолчанию.
export function buildSettings(periods: NetworkPeriod[], fallback: QuarterSettings): Record<number, QuarterSettings> {
  const settings: Record<number, QuarterSettings> = {};
  QUARTERS.forEach((quarter) => {
    const period = periods.find((p) => p.quarter === quarter);
    settings[quarter] = period
      ? { vat_included: period.vat_included, vat_rate: period.vat_rate, contract_type: period.contract_type }
      : { ...fallback };
  });
  return settings;
}

// Бренды, у которых есть строки плана, в алфавитном порядке.
export function brandsFromPlans(plans: NetworkPlan[]): string[] {
  const unique = new Set<string>();
  plans.forEach((plan) => {
    if (plan.brand_as) unique.add(plan.brand_as);
  });
  return Array.from(unique).sort((a, b) => a.localeCompare(b, 'ru'));
}
