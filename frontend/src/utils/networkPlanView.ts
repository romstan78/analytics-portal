// Правила показа сетки планов: как окрашивать отклонения и какие величины
// доступны в годовом разрезе. Вынесено из компонентов, чтобы файлы компонентов
// экспортировали только компоненты (требование react-refresh).

export type HintTone = 'neutral' | 'good' | 'warn' | 'bad';

export const TONE_COLOR: Record<HintTone, string> = {
  neutral: 'text.secondary',
  good: 'success.main',
  warn: 'warning.main',
  bad: 'error.main',
};

// Тон выполнения плана: недобор ниже 90 % — предупреждение, ниже 75 % — проблема.
export function completionTone(pct: number | null): HintTone {
  if (pct == null) return 'neutral';
  if (pct >= 100) return 'good';
  if (pct >= 90) return 'neutral';
  if (pct >= 75) return 'warn';
  return 'bad';
}

// Тон отклонения прогноза от плана: и перебор, и недобор одинаково важны.
export function deviationTone(pct: number | null): HintTone {
  if (pct == null) return 'neutral';
  const abs = Math.abs(pct);
  if (abs <= 2) return 'good';
  if (abs <= 10) return 'warn';
  return 'bad';
}

// Величина, показываемая в годовом разрезе по четырём кварталам сразу.
export type YearMetric =
  | 'plan' | 'fact' | 'forecast' | 'pct' | 'investPlan' | 'investForecast' | 'investFact' | 'payable';

export const YEAR_METRICS: Array<{ value: YearMetric; label: string }> = [
  { value: 'plan', label: 'План' },
  { value: 'fact', label: 'Факт' },
  { value: 'forecast', label: 'Прогноз' },
  { value: 'pct', label: 'Инв., %' },
  { value: 'investPlan', label: 'Инв. план' },
  { value: 'investForecast', label: 'Инв. прогноз' },
  { value: 'investFact', label: 'Инв. факт' },
	{ value: 'payable', label: 'К выплате' },
];
