import type { DrilldownRow } from '../types/sales';

export interface DrilldownChartPoint {
  period: string;
  упаковки: number;
  рубли: number;
  segments: Record<string, number>;
  channels: Record<string, number>;
}

export function prepareDrilldownChartData(data: DrilldownRow[]): DrilldownChartPoint[] {
  const grouped: Record<string, DrilldownChartPoint> = {};
  data.forEach((row) => {
    const key = `${row.year}-${String(row.month).padStart(2, '0')}`;
    if (!grouped[key]) grouped[key] = { period: key, упаковки: 0, рубли: 0, segments: {}, channels: {} };
    if (row.un_rub === 'уп') grouped[key].упаковки += row.totalValue;
    else if (row.un_rub === 'руб') grouped[key].рубли += row.totalValue;
    const unitSuffix = row.un_rub === 'уп' ? 'шт.' : row.un_rub === 'руб' ? '₽' : (row.un_rub || 'без ед.');
    const segmentKey = `${encodeSeriesName(row.segment || 'Без сегмента')}__${unitSuffix}`;
    if (!grouped[key].segments[segmentKey]) grouped[key].segments[segmentKey] = 0;
    grouped[key].segments[segmentKey] += row.totalValue;
    const channelKey = `${encodeSeriesName(row.channel || 'Без канала')}__${unitSuffix}`;
    if (!grouped[key].channels[channelKey]) grouped[key].channels[channelKey] = 0;
    grouped[key].channels[channelKey] += row.totalValue;
  });
  return Object.values(grouped).sort((a, b) => a.period.localeCompare(b.period));
}

export function drilldownSeriesLabel(key: string): string {
  const [name, unit] = key.split('__');
  return `${decodeURIComponent(name)} · ${unit}`;
}

function encodeSeriesName(value: string): string {
  return encodeURIComponent(value).replace(/\./g, '%2E');
}
