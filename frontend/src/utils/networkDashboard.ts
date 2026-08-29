// Общее для витрины реестра и разбора одной сети: палитра, единицы измерения
// и доступ к метрикам ответа.
//
// Вынесено в отдельный модуль ровно затем, чтобы у двух экранов не завелось
// двух реализаций одного процента — они однажды разойдутся в последнем знаке,
// и объяснить расхождение будет нечем.

import { formatPct, formatRub, formatRubShort } from './networkPlan';
import type { NetworkDashboardMetrics } from '../types/network';

// Палитра серий проверена валидатором на разделимость при дальтонизме:
// худшая соседняя пара ΔE 9,2 (протанопия) при пороге 8.
export const SERIES_PLAN = '#6366f1';
export const SERIES_FACT = '#149174';
export const SERIES_EAC = '#c57a24';
export const SERIES_PREV = '#8793a5';
export const NEUTRAL = '#8793a5';
export const GRID = '#e9edf2';
export const BORDER = '#dfe5ee';
export const INK_MUTED = '#64748b';

// Полярность отклонения: тёплый и холодный полюс с нейтралью в нуле.
export const POLARITY_POSITIVE = SERIES_FACT;
export const POLARITY_NEGATIVE = '#d15d50';

// Канал промо: онлайн и оффлайн различаются не только подписью, чтобы метку
// можно было прочитать боковым зрением.
export const CHANNEL_COLOR: Record<string, string> = {
  'онлайн': '#3b7ea1',
  'оффлайн': '#7a6ea8',
  'не указан': NEUTRAL,
};

export type Dimension = 'networks' | 'brands' | 'kams';
export type Unit = 'rub' | 'units';
export type Grain = 'quarter' | 'month';

export const MONTH_LABELS = [
  'Янв', 'Фев', 'Мар', 'Апр', 'Май', 'Июн',
  'Июл', 'Авг', 'Сен', 'Окт', 'Ноя', 'Дек',
];

export const DIMENSION_LABEL: Record<Dimension, string> = {
  networks: 'Сети',
  brands: 'Бренды',
  kams: 'КАМы',
};

// Заголовок первой колонки таблицы: русское единственное число не получается
// отсечением окончания, поэтому оно задано явно.
export const DIMENSION_COLUMN: Record<Dimension, string> = {
  networks: 'Сеть',
  brands: 'Бренд',
  kams: 'КАМ',
};

export const QUARTERS = [1, 2, 3, 4];

// Суммы вроде «3,7 млрд» рвутся по пробелу на две строки, и высота строк
// таблицы начинает скакать. Числовые ячейки не переносятся.
export const NUMERIC_CELL = { whiteSpace: 'nowrap' } as const;

export function pctLabel(value: number | null): string {
  return value == null ? '—' : `${formatPct(value)} %`;
}

// Прирост к прошлому году: знак несёт смысл, поэтому он всегда виден.
export function growthLabel(value: number | null): string {
  if (value == null) return '—';
  const sign = value > 0 ? '+' : value < 0 ? '−' : '';
  return `${sign}${Math.abs(value).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} %`;
}

export function signedShort(value: number): string {
  const sign = value > 0 ? '+' : value < 0 ? '−' : '';
  return `${sign}${formatRubShort(Math.abs(value))}`;
}

// То же со знаком, но с единицей: «−1,7 млрд ₽» и «−1,7 млн уп.» — разные
// утверждения, и рядом с переключателем единиц их нельзя не различать.
export function signedAmount(value: number, unit: Unit): string {
  return `${signedShort(value)} ${unit === 'rub' ? '₽' : 'уп.'}`;
}

// Единицы меняют не только числа, но и подписи: «₽» рядом с упаковками врал бы.
export function amount(value: number, unit: Unit): string {
  return unit === 'rub' ? `${formatRubShort(value)} ₽` : `${formatRubShort(value)} уп.`;
}

export function amountFull(value: number, unit: Unit): string {
  return unit === 'rub' ? `${formatRub(value)} ₽` : `${formatRub(value)} уп.`;
}

export function metricPlan(metrics: NetworkDashboardMetrics, unit: Unit): number {
  return unit === 'rub' ? metrics.planRub : metrics.planUnits;
}
export function metricFact(metrics: NetworkDashboardMetrics, unit: Unit): number {
  return unit === 'rub' ? metrics.factRub : metrics.factUnits;
}
export function metricEAC(metrics: NetworkDashboardMetrics, unit: Unit): number {
  return unit === 'rub' ? metrics.eacRub : metrics.eacUnits;
}
export function metricPrevFact(metrics: NetworkDashboardMetrics, unit: Unit): number | null {
  return unit === 'rub' ? metrics.prevFactRub : metrics.prevFactUnits;
}
export function metricGap(metrics: NetworkDashboardMetrics, unit: Unit): number {
  return unit === 'rub' ? metrics.gapRub : metrics.gapUnits;
}

// Проценты в рублях берём с сервера как есть — он их и считает. Для упаковок
// сервер процентов не отдаёт, поэтому они выводятся здесь из тех же сумм по той
// же формуле. Пересчитывать заодно и рублёвые нельзя: две реализации одного
// процента однажды разойдутся в последнем знаке.
export function ratioPct(value: number, base: number): number | null {
  if (base === 0) return null;
  return Math.round((value / base) * 10000) / 100;
}

export function completionOf(metrics: NetworkDashboardMetrics, unit: Unit): number | null {
  return unit === 'rub' ? metrics.completionPct : ratioPct(metrics.factUnits, metrics.planUnits);
}

export function eacCompletionOf(metrics: NetworkDashboardMetrics, unit: Unit): number | null {
  return unit === 'rub' ? metrics.eacCompletionPct : ratioPct(metrics.eacUnits, metrics.planUnits);
}

export function growthOf(metrics: NetworkDashboardMetrics, unit: Unit): number | null {
  if (unit === 'rub') return metrics.factYoyPct;
  const prev = metrics.prevFactUnits;
  if (prev == null || prev === 0) return null;
  return Math.round(((metrics.factUnits - prev) / prev) * 10000) / 100;
}

// Цвет ячейки выполнения — полярность вокруг 100%: план либо выполнен, либо нет.
export function completionColor(value: number | null): { bgcolor: string; color: string } {
  if (value == null) return { bgcolor: '#f3f5f8', color: '#9aa4b2' };
  if (value >= 100) return { bgcolor: '#d7f1e8', color: '#116b54' };
  if (value >= 90) return { bgcolor: '#fff0c7', color: '#8a5a12' };
  return { bgcolor: '#f8d8d4', color: '#9a3d34' };
}

// Отклонение в рублях: разрыв прогноза итога к обязательству. Знак означает
// «хорошо/плохо», а не «больше/меньше», поэтому цвет берётся от полярности.
export function gapColor(value: number): string {
  return value >= 0 ? POLARITY_POSITIVE : POLARITY_NEGATIVE;
}
