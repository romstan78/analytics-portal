// Расчёты сетки планов реестра сетей.
// Повторяют backend/services/network_service.go, чтобы итоги в таблице
// пересчитывались во время ввода, до сохранения.

import type { NetworkPeriod, NetworkPlan } from '../types/network';

export const QUARTERS = [1, 2, 3, 4] as const;

export const round2 = (v: number): number => Math.round(v * 100) / 100;

// Ключ строки плана: квартал + бренд. Пустой бренд — общий объём валового контракта.
export const planKey = (quarter: number, brand: string | null): string => `${quarter}|${brand ?? ''}`;

// Сумма инвестиций с вычетом НДС. Сеть без НДС в этом квартале — сумма остаётся как есть.
// К объёмам НДС не применяется: план, факт и прогноз показываются так, как их внесли.
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

export function calcCell(cell: DraftCell | undefined, setting: QuarterSettings | undefined): CellAmounts {
  const vatIncluded = setting?.vat_included ?? false;
  const vatRate = setting?.vat_rate ?? 0;
  const plan = cell ? parseNumberInput(cell.planRub) : null;
  const forecast = cell ? parseNumberInput(cell.forecastRub) : null;
  const pct = cell ? parseNumberInput(cell.investmentsPct) : null;

  const invest = (volume: number | null): [number | null, number | null] => {
    if (volume == null || pct == null) return [null, null];
    const gross = round2((volume * pct) / 100);
    return [gross, netRub(gross, vatIncluded, vatRate)];
  };
  const [investPlan, investPlanNet] = invest(plan);
  const [investForecast, investForecastNet] = invest(forecast);

  // Факт инвестиций пришёл суммой — процентом его не пересчитываем,
  // но базу «без НДС» считаем по ставке того же квартала.
  const investFact = cell?.factInvestmentsRub ?? null;
  const investFactNet = investFact == null ? null : netRub(investFact, vatIncluded, vatRate);

  return {
    plan,
    fact: cell?.factRub ?? null,
    forecast,
    investPlan,
    investPlanNet,
    investForecast,
    investForecastNet,
    investFact,
    investFactNet,
  };
}

export interface QuarterTotals {
  quarter: number;
  planRub: number;
  grossBrandsPlan: number;
  separatePlanRub: number;
  grossPoolRub: number | null;
  undistributed: number | null;
  contractPlanRub: number;
  grossBrandsCount: number;
  factRub: number;
  grossPoolFactRub: number;
  forecastRub: number;
  grossPoolForecastRub: number | null;
  investmentsRub: number;
  investmentsRubNet: number;
  forecastInvestmentsRub: number;
  forecastInvestmentsRubNet: number;
  factInvestmentsRub: number;
  factInvestmentsRubNet: number;
}

// Итоги квартала по черновику. Валовый объём — свойство бренда: остаток
// к распределению считается только от брендов, отнесённых к пулу.
export function calcQuarterTotals(
  draft: Record<string, DraftCell>,
  brands: string[],
  settings: Record<number, QuarterSettings>,
): QuarterTotals[] {
  return QUARTERS.map((quarter) => {
    const setting = settings[quarter];
    const t: QuarterTotals = {
      quarter,
      planRub: 0,
      grossBrandsPlan: 0,
      separatePlanRub: 0,
      grossPoolRub: null,
      undistributed: null,
      contractPlanRub: 0,
      grossBrandsCount: 0,
      factRub: 0,
      grossPoolFactRub: 0,
      forecastRub: 0,
      grossPoolForecastRub: null,
      investmentsRub: 0,
      investmentsRubNet: 0,
      forecastInvestmentsRub: 0,
      forecastInvestmentsRubNet: 0,
      factInvestmentsRub: 0,
      factInvestmentsRubNet: 0,
    };

    const pool = calcCell(draft[planKey(quarter, null)], setting);
    t.grossPoolRub = pool.plan;
    t.grossPoolForecastRub = pool.forecast;

    brands.forEach((brand) => {
      const cell = draft[planKey(quarter, brand)];
      if (!cell) return;
      const amounts = calcCell(cell, setting);
      if (cell.inGross) t.grossBrandsCount += 1;

      if (amounts.plan != null) {
        t.planRub = round2(t.planRub + amounts.plan);
        if (cell.inGross) t.grossBrandsPlan = round2(t.grossBrandsPlan + amounts.plan);
        else t.separatePlanRub = round2(t.separatePlanRub + amounts.plan);
      }
      if (amounts.fact != null) {
        t.factRub = round2(t.factRub + amounts.fact);
        if (cell.inGross) t.grossPoolFactRub = round2(t.grossPoolFactRub + amounts.fact);
      }
      if (amounts.forecast != null) t.forecastRub = round2(t.forecastRub + amounts.forecast);
      if (amounts.investPlan != null) {
        t.investmentsRub = round2(t.investmentsRub + amounts.investPlan);
        t.investmentsRubNet = round2(t.investmentsRubNet + (amounts.investPlanNet ?? 0));
      }
      if (amounts.investForecast != null) {
        t.forecastInvestmentsRub = round2(t.forecastInvestmentsRub + amounts.investForecast);
        t.forecastInvestmentsRubNet = round2(t.forecastInvestmentsRubNet + (amounts.investForecastNet ?? 0));
      }
      if (amounts.investFact != null) {
        t.factInvestmentsRub = round2(t.factInvestmentsRub + amounts.investFact);
        t.factInvestmentsRubNet = round2(t.factInvestmentsRubNet + (amounts.investFactNet ?? 0));
      }
    });

    // Остаток есть только там, где пул заведён: без него распределять нечего.
    if (t.grossPoolRub != null) t.undistributed = round2(t.grossPoolRub - t.grossBrandsPlan);
    // Обязательство по контракту: пул целиком, даже если бренды разобрали его
    // не полностью, плюс бренды вне пула как есть.
    t.contractPlanRub = round2((t.grossPoolRub ?? t.grossBrandsPlan) + t.separatePlanRub);
    return t;
  });
}

// Сумма итогов за год: складывает те же поля по всем кварталам.
// Поля пула суммируются только там, где пул заведён.
export function sumYearTotals(totals: QuarterTotals[]): QuarterTotals {
  const year: QuarterTotals = {
    quarter: 0,
    planRub: 0,
    grossBrandsPlan: 0,
    separatePlanRub: 0,
    grossPoolRub: null,
    undistributed: null,
    contractPlanRub: 0,
    grossBrandsCount: 0,
    factRub: 0,
    grossPoolFactRub: 0,
    forecastRub: 0,
    grossPoolForecastRub: null,
    investmentsRub: 0,
    investmentsRubNet: 0,
    forecastInvestmentsRub: 0,
    forecastInvestmentsRubNet: 0,
    factInvestmentsRub: 0,
    factInvestmentsRubNet: 0,
  };

  totals.forEach((t) => {
    year.planRub = round2(year.planRub + t.planRub);
    year.grossBrandsPlan = round2(year.grossBrandsPlan + t.grossBrandsPlan);
    year.separatePlanRub = round2(year.separatePlanRub + t.separatePlanRub);
    year.contractPlanRub = round2(year.contractPlanRub + t.contractPlanRub);
    year.grossBrandsCount = Math.max(year.grossBrandsCount, t.grossBrandsCount);
    year.factRub = round2(year.factRub + t.factRub);
    year.grossPoolFactRub = round2(year.grossPoolFactRub + t.grossPoolFactRub);
    year.forecastRub = round2(year.forecastRub + t.forecastRub);
    year.investmentsRub = round2(year.investmentsRub + t.investmentsRub);
    year.investmentsRubNet = round2(year.investmentsRubNet + t.investmentsRubNet);
    year.forecastInvestmentsRub = round2(year.forecastInvestmentsRub + t.forecastInvestmentsRub);
    year.forecastInvestmentsRubNet = round2(year.forecastInvestmentsRubNet + t.forecastInvestmentsRubNet);
    year.factInvestmentsRub = round2(year.factInvestmentsRub + t.factInvestmentsRub);
    year.factInvestmentsRubNet = round2(year.factInvestmentsRubNet + t.factInvestmentsRubNet);
    if (t.grossPoolRub != null) year.grossPoolRub = round2((year.grossPoolRub ?? 0) + t.grossPoolRub);
    if (t.grossPoolForecastRub != null) {
      year.grossPoolForecastRub = round2((year.grossPoolForecastRub ?? 0) + t.grossPoolForecastRub);
    }
    if (t.undistributed != null) year.undistributed = round2((year.undistributed ?? 0) + t.undistributed);
  });
  return year;
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
