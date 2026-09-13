// Показ сетки планов реестра сетей: черновик полей ввода и форматирование.
//
// Расчётов здесь нет. НДС, инвестиции и итоги считает backend
// (backend/services/network_service.go): во время ввода их возвращает
// POST /api/networks/:id/plan/preview, после сохранения — сам ответ на запись.

import type { NetworkPeriod, NetworkPeriodGroupInput, NetworkPlan, NetworkPlanScaleInput } from '../types/network';

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

// ─── Ступени контракта ──────────────────────────────────────────────────────
//
// Ступень — порог объёма и процент за него; ступень 1 — сама строка плана,
// её порог и процент живут в planRub / investmentsPct ячейки. Верхние ступени
// и SKU-исключения хранятся в scales. Правило считает сервер; здесь только то,
// что вводится, и разбор ввода.

export type CapMode = 'open' | 'pct' | 'closed';

export const CAP_MODES: Array<{ value: CapMode; label: string; short: string }> = [
  { value: 'open', label: 'Открытая', short: 'откр.' },
  { value: 'pct', label: 'Процентная', short: '%' },
  { value: 'closed', label: 'Закрытая', short: 'закрытая' },
];

export function capModeLabel(mode: string | null | undefined, pct?: number | string | null): string {
  switch (mode) {
    case 'pct': {
      const value = typeof pct === 'string' ? parseNumberInput(pct) : pct;
      return value == null ? 'крышка в %' : `крышка +${value.toLocaleString('ru-RU', { maximumFractionDigits: 2 })} %`;
    }
    case 'closed': return 'закрытая крышка';
    default: return 'открытая крышка';
  }
}

// SKU на ступени: пустые процент и крышка — «как у бренда».
export interface DraftSKU {
  sku: string;
  planRub: string;
  planUnits: string;
  investmentsPct: string;
  capMode: '' | CapMode;
  capPct: string;
}

// Ступень строки. Запись с номером 1 хранит только SKU: её порог и процент —
// это сама ячейка. Верхние ступени несут порог (у валового бренда — его план).
export interface DraftScale {
  scaleNo: number;
  planRub: string;
  planUnits: string;
  investmentsPct: string;
  skus: DraftSKU[];
  updatedAt: string;
}

export const MAX_SCALES = 3;

// Значение строки сетки во время редактирования.
// Факта и прогноза здесь нет: факт приходит загрузкой, прогноз ведётся помесячно
// во вкладке «Прогноз», и оба показываются готовыми суммами из ответа сервера.
export interface DraftCell {
  planRub: string;
  // Пара к planRub: вводится та метрика, что задана режимом бренда (entryUnit),
  // вторую считает сервер по ценам контракта и возвращает пересчётом.
  planUnits: string;
  entryUnit: 'rub' | 'units';
  investmentsPct: string;
  inGross: boolean;
  factRub: number | null;
  factInvestmentsRub: number | null;
  capMode: CapMode;
  capPct: string;
  scales: DraftScale[];
}

export const EMPTY_CELL: DraftCell = {
  planRub: '',
  planUnits: '',
  entryUnit: 'rub',
  investmentsPct: '',
  inGross: false,
  factRub: null,
  factInvestmentsRub: null,
  capMode: 'open',
  capPct: '',
  scales: [],
};

// Верхние ступени ячейки по номеру — то, что показывается под-строками.
export function upperScales(cell: DraftCell): DraftScale[] {
  return cell.scales.filter((scale) => scale.scaleNo >= 2).sort((a, b) => a.scaleNo - b.scaleNo);
}

// Порог ступени в рублях: у ступени 1 — сама ячейка.
export function scaleThreshold(cell: DraftCell, scaleNo: number): number | null {
  if (scaleNo === 1) return parseNumberInput(cell.planRub);
  const scale = cell.scales.find((s) => s.scaleNo === scaleNo);
  return scale ? parseNumberInput(scale.planRub) : null;
}

// SKU ступени; ступень 1 хранит их в записи с номером 1.
export function skusOf(cell: DraftCell, scaleNo: number): DraftSKU[] {
  return cell.scales.find((s) => s.scaleNo === scaleNo)?.skus ?? [];
}

// Ячейка с заменённым списком SKU ступени. Пустой список у ступени 1 убирает
// саму запись: без SKU ей хранить нечего.
export function withSkus(cell: DraftCell, scaleNo: number, skus: DraftSKU[]): DraftCell {
  const others = cell.scales.filter((s) => s.scaleNo !== scaleNo);
  const existing = cell.scales.find((s) => s.scaleNo === scaleNo);
  if (scaleNo === 1 && skus.length === 0) return { ...cell, scales: others };
  const scale: DraftScale = existing
    ? { ...existing, skus }
    : { scaleNo, planRub: '', planUnits: '', investmentsPct: '', skus, updatedAt: '' };
  return { ...cell, scales: [...others, scale].sort((a, b) => a.scaleNo - b.scaleNo) };
}

// Ячейка с изменённой верхней ступенью.
export function withScale(cell: DraftCell, scaleNo: number, patch: Partial<DraftScale>): DraftCell {
  const existing = cell.scales.find((s) => s.scaleNo === scaleNo);
  const scale: DraftScale = {
    ...(existing ?? { scaleNo, planRub: '', planUnits: '', investmentsPct: '', skus: [], updatedAt: '' }),
    ...patch,
  };
  return {
    ...cell,
    scales: [...cell.scales.filter((s) => s.scaleNo !== scaleNo), scale].sort((a, b) => a.scaleNo - b.scaleNo),
  };
}

// Ячейка без ступени и всех ступеней выше неё: дыр в лестнице не бывает.
export function withoutScale(cell: DraftCell, scaleNo: number): DraftCell {
  return { ...cell, scales: cell.scales.filter((s) => s.scaleNo < scaleNo) };
}

// Число ступеней, которые ячейка может завести: по настройке квартала.
export function canAddScale(cell: DraftCell, quarterScales: number): boolean {
  const top = upperScales(cell).at(-1)?.scaleNo ?? 1;
  return top < Math.min(MAX_SCALES, quarterScales);
}

// Шаг ступени: порог следующей от порога предыдущей. Ввод «+10 000 000» или
// «+10 %» — помощник ввода, хранится порог в рублях.
export function thresholdFromStep(previous: number | null, step: string): number | null {
  if (previous == null) return null;
  const raw = step.replace(/\s/g, '').replace(',', '.').trim();
  if (raw === '') return null;
  const isPct = raw.endsWith('%');
  const value = Number(raw.replace(/[+%]/g, ''));
  if (!Number.isFinite(value)) return null;
  return round2(isPct ? previous * (1 + value / 100) : previous + value);
}

// Подпись шага между порогами: «+10 млн · +10 %».
export function stepLabel(previous: number | null, next: number | null): string {
  if (previous == null || next == null || previous <= 0) return '';
  const delta = round2(next - previous);
  const pct = round2((delta / previous) * 100);
  return `${delta >= 0 ? '+' : '−'}${formatRubShort(Math.abs(delta))} · ${formatSignedPct(pct)}`;
}

// Проверка лестницы владельца порога: пороги обязаны расти. У валового бренда
// верхние ступени — его планы, им расти не обязательно, но пустыми быть нельзя.
export function scaleErrors(cell: DraftCell, owner: boolean): Record<number, string> {
  const errors: Record<number, string> = {};
  let previous = parseNumberInput(cell.planRub);
  upperScales(cell).forEach((scale) => {
    const value = parseNumberInput(scale.planRub);
    const units = parseNumberInput(scale.planUnits);
    if (value == null && units == null) {
      errors[scale.scaleNo] = 'Порог не задан';
    } else if (owner && value != null && previous != null && value <= previous) {
      errors[scale.scaleNo] = `Порог должен быть выше ступени ${scale.scaleNo - 1}`;
    }
    previous = value ?? previous;
  });
  return errors;
}

export interface QuarterSettings {
  vat_included: boolean;
  vat_rate: number;
}

// Расчёт SKU на ступени — «как если бы ступень была достигнутой».
export interface SKUAmounts {
  sku: string;
  plan: number | null;
  planUnits: number | null;
  forecast: number | null;
  forecastBase: number | null;
  forecastInvest: number | null;
  fact: number | null;
  factBase: number | null;
  factInvest: number | null;
}

// Расчёт ступени: плановые инвестиции и гипотеза «если достигнута» по
// прогнозу и факту. Какая ступень достигнута на самом деле — в CellAmounts.
export interface ScaleAmounts {
  scaleNo: number;
  plan: number | null;
  planUnits: number | null;
  investmentsPct: number | null;
  // Процент, по которому ступень считается: свой или унаследованный снизу.
  effectiveInvestmentsPct: number | null;
  investPlan: number | null;
  investPlanNet: number | null;
  forecast: number | null;
  forecastBase: number | null;
  investForecast: number | null;
  investForecastNet: number | null;
  forecastReached: boolean;
  fact: number | null;
  factBase: number | null;
  investFact: number | null;
  investFactNet: number | null;
  factReached: boolean;
  skus: SKUAmounts[];
}

// Расчётные суммы одной ячейки: инвестиции считаются одним процентом
// и от планового объёма, и от прогноза.
export interface CellAmounts {
  plan: number | null;
  planUnits: number | null;
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
  // Ступени: достигнутая по прогнозу и по факту (0 — ни одна), база после
  // крышки и расчёт каждой ступени.
  forecastScale: number;
  factScale: number;
  forecastBase: number | null;
  factBase: number | null;
  scales: ScaleAmounts[];
}

export const EMPTY_AMOUNTS: CellAmounts = {
  plan: null,
  planUnits: null,
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
  forecastScale: 0,
  factScale: 0,
  forecastBase: null,
  factBase: null,
  scales: [],
};

// Расчётные суммы одной ячейки — так, как их вернул бэкенд.
export function amountsOfPlan(plan: NetworkPlan | undefined): CellAmounts {
  if (!plan) return EMPTY_AMOUNTS;
  return {
    plan: plan.plan_rub,
    planUnits: plan.plan_units,
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
    forecastScale: plan.forecast_scale,
    factScale: plan.fact_scale,
    forecastBase: plan.forecast_base_rub,
    factBase: plan.fact_base_rub,
    scales: (plan.scales ?? []).map((scale) => ({
      scaleNo: scale.scale_no,
      plan: scale.plan_rub,
      planUnits: scale.plan_units,
      investmentsPct: scale.investments_pct,
      effectiveInvestmentsPct: scale.effective_investments_pct,
      investPlan: scale.plan_investments_rub,
      investPlanNet: scale.plan_investments_rub_net,
      forecast: scale.forecast_rub,
      forecastBase: scale.forecast_base_rub,
      investForecast: scale.forecast_investments_rub,
      investForecastNet: scale.forecast_investments_rub_net,
      forecastReached: scale.forecast_reached,
      fact: scale.fact_rub,
      factBase: scale.fact_base_rub,
      investFact: scale.fact_investments_rub,
      investFactNet: scale.fact_investments_rub_net,
      factReached: scale.fact_reached,
      skus: (scale.skus ?? []).map((sku) => ({
        sku: sku.sku,
        plan: sku.plan_rub,
        planUnits: sku.plan_units,
        forecast: sku.forecast_rub,
        forecastBase: sku.forecast_base_rub,
        forecastInvest: sku.forecast_investments_rub,
        fact: sku.fact_rub,
        factBase: sku.fact_base_rub,
        factInvest: sku.fact_investments_rub,
      })),
    })),
  };
}

// Расчёт ступени ячейки по номеру.
export function scaleAmountsOf(amounts: CellAmounts, scaleNo: number): ScaleAmounts | undefined {
  return amounts.scales.find((scale) => scale.scaleNo === scaleNo);
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
  const asInput = (value: number | null | undefined): string => (value == null ? '' : formatNumberInput(String(value)));
  const asCap = (mode: string): CapMode => (mode === 'pct' || mode === 'closed' ? mode : 'open');
  const asSkuCap = (mode: string): '' | CapMode => (mode === 'pct' || mode === 'closed' || mode === 'open' ? mode : '');
  plans.forEach((plan) => {
    // Ступень 1 в ответе зеркалит строку: в черновике она нужна только ради SKU.
    const scales: DraftScale[] = (plan.scales ?? [])
      .filter((scale) => scale.scale_no >= 2 || (scale.skus?.length ?? 0) > 0)
      .map((scale) => ({
        scaleNo: scale.scale_no,
        planRub: scale.scale_no === 1 ? '' : asInput(scale.plan_rub),
        planUnits: scale.scale_no === 1 ? '' : asInput(scale.plan_units),
        investmentsPct: scale.scale_no === 1 ? '' : asInput(scale.investments_pct),
        updatedAt: scale.updated_at,
        skus: (scale.skus ?? []).map((sku) => ({
          sku: sku.sku,
          planRub: asInput(sku.plan_rub),
          planUnits: asInput(sku.plan_units),
          investmentsPct: asInput(sku.investments_pct),
          capMode: asSkuCap(sku.cap_mode),
          capPct: asInput(sku.cap_pct),
        })),
      }))
      .sort((a, b) => a.scaleNo - b.scaleNo);
    draft[planKey(plan.quarter, plan.brand_as)] = {
      planRub: asInput(plan.plan_rub),
      planUnits: asInput(plan.plan_units),
      entryUnit: plan.entry_unit === 'units' ? 'units' : 'rub',
      investmentsPct: asInput(plan.investments_pct),
      inGross: plan.brand_as != null && plan.in_gross,
      factRub: plan.fact_rub,
      factInvestmentsRub: plan.fact_investments_rub,
      capMode: asCap(plan.cap_mode),
      capPct: asInput(plan.cap_pct),
      scales,
    };
  });
  return draft;
}

// Ступени ячейки в теле запроса. Массив уходит всегда, даже пустой: черновик —
// полное состояние строки, и пустой массив означает «ступеней нет».
export function scalesInput(cell: DraftCell): NetworkPlanScaleInput[] {
  return cell.scales
    .filter((scale) => scale.scaleNo >= 2 || scale.skus.length > 0)
    .map((scale) => ({
      scale_no: scale.scaleNo,
      plan_rub: scale.scaleNo === 1 ? null : parseNumberInput(scale.planRub),
      plan_units: scale.scaleNo === 1 ? null : parseNumberInput(scale.planUnits),
      investments_pct: scale.scaleNo === 1 ? null : parseNumberInput(scale.investmentsPct),
      updated_at: scale.updatedAt,
      skus: scale.skus.map((sku) => ({
        sku: sku.sku,
        plan_rub: parseNumberInput(sku.planRub),
        plan_units: parseNumberInput(sku.planUnits),
        investments_pct: parseNumberInput(sku.investmentsPct),
        cap_mode: sku.capMode,
        cap_pct: sku.capMode === 'pct' ? parseNumberInput(sku.capPct) : null,
      })),
    }));
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
