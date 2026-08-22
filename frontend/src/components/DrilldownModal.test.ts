import { describe, expect, it } from 'vitest';
import { prepareDrilldownChartData } from '../utils/drilldownChart';
import type { DrilldownRow } from '../types/sales';

describe('prepareChartData', () => {
  it('не складывает рубли и упаковки в одну серию сегмента или канала', () => {
    const base = { year: 2026, month: 2, metricType: 'Продажи', segment: 'OLAP.SS', channel: 'Ecom' };
    const rows: DrilldownRow[] = [
      { ...base, totalValue: 1000, un_rub: 'руб' },
      { ...base, totalValue: 12, un_rub: 'уп' },
    ];

    const [point] = prepareDrilldownChartData(rows);

    expect(point.рубли).toBe(1000);
    expect(point.упаковки).toBe(12);
    expect(Object.values(point.segments).sort((a, b) => a - b)).toEqual([12, 1000]);
    expect(Object.values(point.channels).sort((a, b) => a - b)).toEqual([12, 1000]);
  });
});
