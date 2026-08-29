// Показ сетки планов реестра сетей: черновик полей ввода и форматирование.
//
// Расчётов здесь нет. НДС, инвестиции и итоги считает backend
// (backend/services/network_service.go): во время ввода их возвращает
// POST /api/networks/:id/plan/preview, после сохранения — сам ответ на запись.

import type { NetworkPeriod, NetworkPeriodGroupInput, NetworkPlan } from '../types/network';

export const QUARTERS = [1, 2, 3, 4] as const;

export const round2 = (v: number): number => Math.round(v * 100) / 100;

// Ключ строки плана: квартал + бренд. Пустой бренд — общий объём валового контракта.
export const planKey = (quarter: number, brand: string | null): string => `${quarter}|${brand ?? ''}`;

// Устойчивый ключ правила совместного зачёта. «*» — весь портфель сети.
export const periodGroupKey = (group: Pick<NetworkPeriodGroupInput, 'start_quarter' | 'end_quarter' | 'brand_as'>): string =>
  `${group.start_quarter}|${group.end_quarter}|${group.brand_as ?? '*'}`;

// Клиентская подсказка повторяет серверное правило пересечений: портфельная
// группа занимает диапазон для всех брендов, брендовые группы могут идти
// параллельно только для разных брендов.
export function periodGroupConflict(
  groups: NetworkPeriodGroupInput[],
  candidate: NetworkPeriodGroupInput,
): string | null {
  if (
    candidate.start_quarter < 1 || candidate.end_quarter > 4
    || candidate.start_quarter >= candidate.end_quarter
  ) {
    return 'Диапазон должен содержать минимум два смежных квартала.';
  }

  const conflict = groups.find((group) => {
    const overlaps = group.start_quarter <= candidate.end_quarter
      && candidate.start_quarter <= group.end_quarter;
    if (!overlaps) return false;
    return group.brand_as == null || candidate.brand_as == null || group.brand_as === candidate.brand_as;
  });
  if (!conflict) return null;
  const scope = conflict.brand_as ?? 'весь портфель';
  return `Пересекается с Q${conflict.start_quarter}–Q${conflict.end_quarter} · ${scope}.`;
}

// Разбор введённого числа: пробелы-разделители и запятая как в Excel.
// Пустая строка — значение снято, а не ноль.
export function parseNumberInput(raw: string): number | null {
  const cleaned = raw.replace(/\s/g, '').replace(',', '.').trim();
  if (cleaned === '') return null;
  const value = Number(cleaned);
  return Number.isFinite(value) ? value : null;
}

export function isMonthDistributionValid(values: [string, string, string]): boolean {
  const numbers = values.map(parseNumberInput);
  return numbers.every((value) => value != null && value >= 0 && value <= 100)
    && Math.abs(numbers.reduce<number>((sum, value) => sum + (value ?? 0), 0) - 100) < 0.001;
}

// Ставка НДС повторяет CK_NetworkPeriods_vat_rate: 100% и выше база не примет,
// поэтому проверка живёт рядом с полем ввода, а не только в подсказке об ошибке.
export function isVATRateValid(value: string): boolean {
  const parsed = parseNumberInput(value);
  return parsed != null && parsed >= 0 && parsed < 100;
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
// Факта и прогноза здесь нет: факт приходит загрузкой, прогноз ведётся помесячно
// во вкладке «Прогноз», и оба показываются готовыми суммами из ответа сервера.
export interface DraftCell {
  planRub: string;
  investmentsPct: string;
  inGross: boolean;
  factRub: number | null;
  factInvestmentsRub: number | null;
}

export const EMPTY_CELL: DraftCell = {
  planRub: '',
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
	paid: number | null;
	forecastEarned: boolean;
	factEarned: boolean;
	payFromFact: boolean;
	forecastCompletionPct: number | null;
	factCompletionPct: number | null;
	investmentPeriodStart: number;
	investmentPeriodEnd: number;
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
	paid: null,
	forecastEarned: false,
	factEarned: false,
	payFromFact: false,
	forecastCompletionPct: null,
	factCompletionPct: null,
	investmentPeriodStart: 0,
	investmentPeriodEnd: 0,
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
		paid: plan.paid_investments_rub,
		forecastEarned: plan.forecast_investments_earned,
		factEarned: plan.fact_investments_earned,
		payFromFact: plan.pay_investments_from_fact,
		forecastCompletionPct: plan.forecast_completion_pct,
		factCompletionPct: plan.fact_completion_pct,
		investmentPeriodStart: plan.investment_period_start_quarter,
		investmentPeriodEnd: plan.investment_period_end_quarter,
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
      investmentsPct: asInput(plan.investments_pct),
      inGross: plan.brand_as != null && plan.in_gross,
      factRub: plan.fact_rub,
      factInvestmentsRub: plan.fact_investments_rub,
    };
  });
  return draft;
}

// Перенос объёмов бренда в строку валового пула и обратно.
//
// Это не расчёт показателя, а правка введённых значений: бренд, выведенный из
// валового объёма, уносит из пула свой объём, переведённый в пул — приносит.
// Так переклассификация бренда не меняет ни обязательство по контракту, ни
// остаток к распределению: те же рубли просто считаются в другой части.
//
// Первый переведённый бренд создаёт пул своим объёмом. Иначе флаг in_gross
// сохранялся, но строки пула не возникало: итоги видели обычную сумму брендов
// и не могли распознать валовый контракт. При выводе бренда отсутствующий пул
// по-прежнему не создаём. Ниже нуля пул не опускаем — отрицательный объём
// бэкенд не примет, а ноль сразу показывает, что бренды разобрали больше, чем
// в пуле было.
export function shiftGrossPool(pool: DraftCell | undefined, brand: DraftCell, intoGross: boolean): DraftCell {
  const base = pool ?? EMPTY_CELL;
  const sign = intoGross ? 1 : -1;
  const shift = (poolValue: string, brandValue: string): string => {
    const delta = parseNumberInput(brandValue);
    if (delta == null) return poolValue;
    const parsed = parseNumberInput(poolValue);
    if (parsed == null && !intoGross) return poolValue;
    const current = parsed ?? 0;
    return formatNumberInput(String(Math.max(0, round2(current + sign * delta))));
  };
  return {
    ...base,
    planRub: shift(base.planRub, brand.planRub),
  };
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
