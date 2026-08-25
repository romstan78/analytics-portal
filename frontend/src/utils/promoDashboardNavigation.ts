export type PromoFilterSet = Record<string, unknown>;

export type DashboardReturnMode = 'original' | 'detail';

export function clonePromoFilters(filters: PromoFilterSet): PromoFilterSet {
  return Object.fromEntries(
    Object.entries(filters).map(([key, value]) => [
      key,
      Array.isArray(value) ? [...value] : value,
    ]),
  );
}

export function buildPromoDrilldownFilters(
  dashboardFilters: PromoFilterSet,
  drilldownFilters: PromoFilterSet,
): PromoFilterSet {
  return clonePromoFilters({ ...dashboardFilters, ...drilldownFilters });
}

export function selectPromoDashboardFilters(
  mode: DashboardReturnMode,
  originalDashboardFilters: PromoFilterSet,
  detailFilters: PromoFilterSet,
): PromoFilterSet {
  return clonePromoFilters(mode === 'original' ? originalDashboardFilters : detailFilters);
}
