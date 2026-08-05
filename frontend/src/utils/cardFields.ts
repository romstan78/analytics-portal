export interface CardField {
  id: string;
  label: string;
  isMoney?: boolean;
  isRoi?: boolean;
  isPercent?: boolean;
}

export interface FieldGroup {
  group: string;
  fields: CardField[];
}

export const FIELD_GROUPS: FieldGroup[] = [
  {
    group: 'Основные',
    fields: [
      { id: 'network_name', label: 'Сеть' },
      { id: 'sku', label: 'SKU' },
      { id: 'mechanics', label: 'Механика' },
      { id: 'brand_as', label: 'Бренд' },
      { id: 'kam', label: 'KAM' },
    ],
  },
  {
    group: 'Плановые показатели',
    fields: [
      { id: 'baseline_units', label: 'Baseline (уп)' },
      { id: 'plan_promo_units', label: 'План (уп)' },
      { id: 'plan_investments_rub', label: 'Инвестиции (руб)', isMoney: true },
      { id: 'plan_roi', label: 'ROI План (%)', isRoi: true },
      { id: 'plan_uplift_percent', label: 'Uplift План (%)', isPercent: true },
      { id: 'plan_total_spb_sales_rub', label: 'Продажи SPB план (руб)', isMoney: true },
    ],
  },
  {
    group: 'Фактические показатели',
    fields: [
      { id: 'actual_promo_sales_units', label: 'Факт продаж (уп)' },
      { id: 'actual_roi', label: 'ROI Факт (%)', isRoi: true },
      { id: 'actual_uplift_percent', label: 'Uplift Факт (%)', isPercent: true },
      { id: 'actual_total_spb_sales_rub', label: 'Продажи SPB факт (руб)', isMoney: true },
      { id: 'actual_investments_rub', label: 'Инвестиции факт (руб)', isMoney: true },
    ],
  },
  {
    group: 'Исторические данные',
    fields: [
      { id: 'historical_count', label: 'Кол-во ист. промо' },
      { id: 'avg_historical_roi', label: 'Средний ист. ROI (%)', isRoi: true },
      { id: 'avg_historical_uplift', label: 'Средний ист. Uplift (%)', isPercent: true },
    ],
  },
];

export const ALL_FIELDS_FLAT: CardField[] = FIELD_GROUPS.flatMap(g => g.fields);

export const DEFAULT_VISIBLE_FIELDS: string[] = [
  'network_name', 'sku', 'mechanics', 'brand_as', 'kam',
  'baseline_units', 'plan_promo_units', 'plan_roi', 'plan_investments_rub',
  'actual_promo_sales_units', 'actual_roi',
];

export const FIELDS_MAP: Record<string, CardField> = {};
ALL_FIELDS_FLAT.forEach(f => { FIELDS_MAP[f.id] = f; });