import type { ApprovalRow } from '../types/promo';

export interface CardField {
  // Ограничение по ApprovalRow ловит опечатки в идентификаторах полей карточки.
  id: keyof ApprovalRow;
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
    ],
  },
  {
    group: 'Плановые показатели',
    fields: [
      { id: 'baseline_units', label: 'Baseline (уп)' },
      { id: 'plan_promo_units', label: 'План (уп)' },
      { id: 'plan_investments_rub', label: 'Инвестиции (руб)', isMoney: true },
      { id: 'plan_roi', label: 'ROI План (%)', isRoi: true },
    ],
  },
  {
    group: 'Фактические показатели',
    fields: [
      { id: 'actual_promo_sales_units', label: 'Факт продаж (уп)' },
      { id: 'actual_roi', label: 'ROI Факт (%)', isRoi: true },
    ],
  },
];

export const ALL_FIELDS_FLAT: CardField[] = FIELD_GROUPS.flatMap(g => g.fields);

export const DEFAULT_VISIBLE_FIELDS: string[] = [
  'network_name', 'sku', 'mechanics', 'brand_as',
  'baseline_units', 'plan_promo_units', 'plan_roi', 'plan_investments_rub',
  'actual_promo_sales_units', 'actual_roi',
];

const VALID_FIELD_IDS: Set<string> = new Set(ALL_FIELDS_FLAT.map(field => field.id));

export function normalizeVisibleFields(value: unknown): string[] {
  if (!Array.isArray(value)) return [...DEFAULT_VISIBLE_FIELDS];

  return Array.from(new Set(
    value.filter((field): field is string => typeof field === 'string' && VALID_FIELD_IDS.has(field)),
  ));
}

export const FIELDS_MAP: Record<string, CardField> = {};
ALL_FIELDS_FLAT.forEach(f => { FIELDS_MAP[f.id] = f; });
