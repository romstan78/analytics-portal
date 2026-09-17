// Подписи осей и цвета печатной страницы отчёта.
//
// Числа в карточках и таблицах приходят с сервера уже отформатированными
// (reports/theme.go); здесь только то, что нужно самим графикам: деления
// осей в шкале страницы и цвет оценки. Это не расчёт — шкалу выбрал сервер.

import type { ReportPrintScale, ReportTone } from '../types/reports';
import { POLARITY_NEGATIVE, POLARITY_POSITIVE, SERIES_PLAN } from './networkDashboard';

// Цвета оценок для SVG и таблиц: hex, а не ключи темы, потому что графики
// Recharts ключей темы не понимают.
export const REPORT_TONE_COLOR: Record<ReportTone, string> = {
  neutral: '#0f172a',
  good: POLARITY_POSITIVE,
  warn: '#b45309',
  bad: POLARITY_NEGATIVE,
};

// Акцент плитки — по оценке отклонения; без оценки — цвет плана темы.
export const REPORT_TONE_ACCENT: Record<ReportTone, string> = {
  neutral: SERIES_PLAN,
  good: POLARITY_POSITIVE,
  warn: '#c57a24',
  bad: POLARITY_NEGATIVE,
};

export function reportTone(value: string): ReportTone {
  return value === 'good' || value === 'warn' || value === 'bad' ? value : 'neutral';
}

// Деление оси в шкале страницы: «7,5» при шкале млрд, «1 250» при шкале тыс.
// Знаков после запятой — как задала шкала; у нуля и круглых делений хвост
// «,00» не пишется, ось от этого только чище.
export function formatScaled(value: number, scale: ReportPrintScale): string {
  const div = scale.div > 0 ? scale.div : 1;
  return (value / div).toLocaleString('ru-RU', {
    minimumFractionDigits: 0,
    maximumFractionDigits: Math.max(0, scale.digits),
  });
}

// То же со знаком: подписи ступеней водопада.
export function formatScaledSigned(value: number, scale: ReportPrintScale): string {
  const text = formatScaled(Math.abs(value), scale);
  if (value > 0) return `+${text}`;
  if (value < 0) return `−${text}`;
  return text;
}
