import { describe, expect, it } from 'vitest';
import {
  buildPromoDrilldownFilters,
  selectPromoDashboardFilters,
} from './promoDashboardNavigation';

describe('promo dashboard navigation filters', () => {
  const dashboardFilters = {
    yearFrom: 2026,
    yearTo: 2026,
    brand: ['Brand A'],
    network_name: [],
  };

  const detailFilters = {
    yearFrom: 2026,
    yearTo: 2026,
    brand: ['Brand A'],
    network_name: ['Network 1'],
  };

  it('adds drilldown values without changing the dashboard snapshot', () => {
    const next = buildPromoDrilldownFilters(dashboardFilters, { network_name: ['Network 1'] });

    expect(next).toEqual(detailFilters);
    expect(dashboardFilters.network_name).toEqual([]);
    expect(next.brand).not.toBe(dashboardFilters.brand);
  });

  it('can restore the original dashboard filters', () => {
    const next = selectPromoDashboardFilters('original', dashboardFilters, detailFilters);

    expect(next).toEqual(dashboardFilters);
    expect(next.brand).not.toBe(dashboardFilters.brand);
  });

  it('can keep the applied detail filters', () => {
    const next = selectPromoDashboardFilters('detail', dashboardFilters, detailFilters);

    expect(next).toEqual(detailFilters);
    expect(next.network_name).not.toBe(detailFilters.network_name);
  });
});
