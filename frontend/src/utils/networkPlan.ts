// Показ сетки планов реестра сетей: черновик полей ввода и форматирование.
//
// Расчётов здесь нет. НДС, инвестиции и итоги считает backend
// (backend/services/network_service.go): во время ввода их возвращает
// POST /api/networks/:id/plan/preview, после сохранения — сам ответ на запись.

import type { NetworkPlan, NetworkPeriod } from '../types/network';

export const QUARTERS = [1, 2, 3, 4] as const;

export const round2 = (v: number): number => Math.round(v * 100) / 100;

// Ключ строки плана: квартал + бренд. Пустой бренд — общий объём валового контракта.
export const planKey = (quarter: number, brand: string | null): string => `${quarter}|${brand ?? ''}`;

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

// Короткая запись для плотной таблицы: планы сетей измеряются миллионами,
// копейки в них не читаются. Полное значение остаётся в подсказке ячейки.
export function formatRubShort(value: number | null | undefined): string {
  if (value == null) return '—';
  const abs = Math.abs(value);
  const compact = (divider: number, suffix: string) =>
    `${(value / divider).toLocaleString('ru-RU', { maximumFractionDigits: abs / divider >= 100 ? 0 : 1 })} ${suffix}`;
  if (abs >= 1e9) return compact(1e9, 'млрд');
  if (abs >= 1e6) return compact(1e6, 'млн');
  if (abs >= 1e4) return compact(1e3, 'тыс');
  return formatRub(value);
}

export function formatPct(value: number | null | undefined): string {
  if (value == null) return '—';
  return value.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

// Отклонение в процентах: null, если базы для сравнения нет.
export function deltaPct(value: number | null, base: number | null): number | null {
  if (value == null || base == null || base === 0) return null;
  return round2(((value - base) / base) * 100);
}

// Подпись отклонения со знаком: «+4,1 %» / «−12 %».
export function formatSignedPct(value: number | null): string {
  if (value == null) return '—';
  const sign = value > 0 ? '+' : value < 0 ? '−' : '';
  return `${sign}${Math.abs(value).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} %`;
}

// Ввод, приведённый к читаемому виду: «1200000» → «1 200 000». Нечисловой ввод
// не трогаем, чтобы не стирать то, что человек ещё дописывает.
export function formatNumberInput(raw: string): string {
  const value = parseNumberInput(raw);
  if (value == null) return raw.trim() === '' ? '' : raw;
  return value.toLocaleString('ru-RU', { maximumFractionDigits: 2, useGrouping: true });
}

// Русское склонение после числа: 1 бренд, 2 бренда, 5 брендов.
export function pluralRu(count: number, one: string, few: string, many: string): string {
  const mod100 = Math.abs(count) % 100;
  if (mod100 >= 11 && mod100 <= 14) return many;
  const mod10 = mod100 % 10;
  if (mod10 === 1) return one;
  if (mod10 >= 2 && mod10 <= 4) return few;
  return many;
}

// Значение строки сетки во время редактирования.
// Факт объёма и факт инвестиций приходят загрузкой и в интерфейсе не правятся.
export interface DraftCell {
  planRub: string;
  forecastRub: string;
  investmentsPct: string;
  inGross: boolean;
  factRub: number | null;
  factInvestmentsRub: number | null;
}

export const EMPTY_CELL: DraftCell = {
  planRub: '',
  forecastRub: '',
  investmentsPct: '',
  inGross: false,
  factRub: null,
  factInvestmentsRub: null,
};

export interface QuarterSettings {
  vat_included: boolean;
  vat_rate: number;
}

// Расчётные суммы одной ячейки: инвестиции считаются одним процентом
// и от планового объёма, и от прогноза.
export interface CellAmounts {
  plan: number | null;
  fact: number | null;
  forecast: number | null;
  investPlan: number | null;
  investPlanNet: number | null;
  investForecast: number | null;
  investForecastNet: number | null;
  investFact: number | null;
  investFactNet: number | null;
}

export const EMPTY_AMOUNTS: CellAmounts = {
  plan: null,
  fact: null,
  forecast: null,
  investPlan: null,
  investPlanNet: null,
  investForecast: null,
  investForecastNet: null,
  investFact: null,
  investFactNet: null,
};

// Расчётные суммы одной ячейки — так, как их вернул бэкенд.
export function amountsOfPlan(plan: NetworkPlan | undefined): CellAmounts {
  if (!plan) return EMPTY_AMOUNTS;
  return {
    plan: plan.plan_rub,
    fact: plan.fact_rub,
    forecast: plan.forecast_rub,
    investPlan: plan.investments_rub,
    investPlanNet: plan.investments_rub_net,
    investForecast: plan.forecast_investments_rub,
    investForecastNet: plan.forecast_investments_rub_net,
    investFact: plan.fact_investments_rub,
    investFactNet: plan.fact_investments_rub_net,
  };
}

// Расчётные суммы всех ячеек по ключу «квартал|бренд».
export function buildAmounts(plans: NetworkPlan[]): Record<string, CellAmounts> {
  const amounts: Record<string, CellAmounts> = {};
  plans.forEach((plan) => {
    amounts[planKey(plan.quarter, plan.brand_as)] = amountsOfPlan(plan);
  });
  return amounts;
}

// Черновик из загруженных строк плана: то, что показывается в полях ввода.
// Суммы сразу с разрядами — «13 500 000» читается, «13500000» приходится считать.
export function buildDraft(plans: NetworkPlan[]): Record<string, DraftCell> {
  const draft: Record<string, DraftCell> = {};
  const asInput = (value: number | null): string => (value == null ? '' : formatNumberInput(String(value)));
  plans.forEach((plan) => {
    draft[planKey(plan.quarter, plan.brand_as)] = {
      planRub: asInput(plan.plan_rub),
      forecastRub: asInput(plan.forecast_rub),
      investmentsPct: asInput(plan.investments_pct),
      inGross: plan.brand_as != null && plan.in_gross,
      factRub: plan.fact_rub,
      factInvestmentsRub: plan.fact_investments_rub,
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
      ? { vat_included: period.vat_included, vat_rate: period.vat_rate }
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
