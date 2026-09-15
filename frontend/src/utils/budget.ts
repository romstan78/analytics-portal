const number = new Intl.NumberFormat('ru-RU', { minimumFractionDigits: 1, maximumFractionDigits: 1 });
export function budgetFormat(value: number | null | undefined, percent = false): string {
  if (value == null || !Number.isFinite(value))
    return '—';
  return `${number.format(percent ? value : value / 1000000)}${percent ? ' %' : ''}`;
}
export function budgetParse(value: string): number | null {
  const text = value.replace(/\s/g, '').replace(',', '.');
  if (!text)
    return null;
  const n = Number(text);
  return Number.isFinite(n) ? n * 1000000 : null;
}
export const budgetMarks: Record<string, string> = { paid: '✓', partial: '◐', forecast: '○', budget: 'П' };
export const budgetMarkTitles: Record<string, string> = { paid: 'Факт оплаты', partial: 'Разные источники', forecast: 'Прогноз', budget: 'План' };
