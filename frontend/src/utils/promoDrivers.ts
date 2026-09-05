// Правила блока «Крупнейшие отклонения» промо-дашборда: какое число берём под
// выбранную метрику и единицу, как его подписываем и каким цветом красим.
//
// Вынесено из компонента затем, чтобы полярность цвета и выбор единицы можно
// было проверить тестом: и то и другое — утверждения о смысле числа, а не о
// вёрстке, и ошибка в них читается как «перерасход — это хорошо».

import type { PromoDashboardMetrics } from '../types/promo';

export type DriverMetric = 'sales' | 'uplift' | 'investments' | 'roi';
export type DriverUnit = 'units' | 'rub';

export const DRIVER_METRIC_LABEL: Record<DriverMetric, string> = {
  sales: 'Продажи', uplift: 'Uplift', investments: 'Инвестиции', roi: 'ROI',
};

const POLARITY_GOOD = '#149174';
const POLARITY_BAD = '#d15d50';

// Отклонение факта от сопоставимого плана берём с сервера: обе единицы он
// считает на одном и том же срезе, и пересчитывать рубли из упаковок в браузере
// нечем. ROI остаётся разностью процентов — рублёвой доли у него нет.
export function driverVariance(
  metrics: PromoDashboardMetrics, metric: DriverMetric, unit: DriverUnit,
): number | null {
  if (metric === 'sales') return unit === 'rub' ? metrics.salesVarianceRub : metrics.salesVarianceUnits;
  if (metric === 'uplift') return unit === 'rub' ? metrics.upliftVarianceRub : metrics.upliftVarianceUnits;
  if (metric === 'investments') return metrics.investmentVarianceRub;
  if (metrics.actualRoi == null || metrics.comparablePlanRoi == null) return null;
  return metrics.actualRoi - metrics.comparablePlanRoi;
}

// Инвестиции бывают только в рублях, ROI — только в процентных пунктах:
// переключатель единиц к ним неприменим.
export const unitSwitchable = (metric: DriverMetric) => metric === 'sales' || metric === 'uplift';

// Погашенный переключатель показывает единицу самой метрики, а не последний
// выбор пользователя: подсвеченная «Уп.» рядом с рублёвыми инвестициями — ложь.
// У ROI не выбрано ничего: процентные пункты в переключателе не значатся.
export function shownUnit(metric: DriverMetric, unit: DriverUnit): DriverUnit | null {
  if (unitSwitchable(metric)) return unit;
  return metric === 'investments' ? 'rub' : null;
}

// Единицу подписываем всегда: рядом с переключателем «уп./₽» неподписанное
// число не читается, а «₽» рядом с упаковками врал бы — как в витрине реестра.
export function driverUnitLabel(metric: DriverMetric, unit: DriverUnit): string {
  if (metric === 'roi') return 'п.п.';
  return unitSwitchable(metric) && unit === 'units' ? 'уп.' : '₽';
}

// Цвет столбца означает «хорошо/плохо», а не «больше/меньше»: перерасход
// инвестиций — такая же неудача, как недобор продаж, хотя знак у него плюс.
const DRIVER_POLARITY: Record<DriverMetric, 1 | -1> = {
  sales: 1, uplift: 1, investments: -1, roi: 1,
};

export const driverColor = (value: number, metric: DriverMetric) =>
  value * DRIVER_POLARITY[metric] >= 0 ? POLARITY_GOOD : POLARITY_BAD;
