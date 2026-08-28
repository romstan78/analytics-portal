// Черновик карточки промо: что именно сохранять.
//
// Только введённое руками. Расчётные поля (ROI, uplift, суммы) считает сервер —
// POST /api/promo/calculate и пересчёт при сохранении, — и восстановленные из
// черновика они разошлись бы с формулой: черновик недельной давности помнит
// старую цену контракта, а формула уже другая. После восстановления их
// пересчитывает сервер, поэтому терять здесь нечего.
//
// Поля согласования и история комментариев тоже не наши: они приходят с
// сервера и правятся отдельным процессом.

import type { PromoFormValues } from '../hooks/usePromoForm';
import { draftStorageKey } from './formDraft';

const DRAFT_BASE_KEY = 'promo_card_draft_v1';

export const PROMO_DRAFT_FIELDS = [
  'network_name', 'kam', 'brand', 'sku', 'year', 'month',
  'mechanics', 'gtn_opex', 'status', 'ecom_segment',
  'id_directum', 'ds_number', 'conditions',
  'total_pharmacies', 'promo_pharmacies',
  'baseline_units', 'plan_promo_units', 'plan_investments_rub',
  'contract_price', 'discount_amount', 'gm',
  'actual_promo_sales_units', 'actual_investments',
  'actual_external_ecom_units', 'actual_corrected_baseline',
] as const satisfies readonly (keyof PromoFormValues)[];

export type PromoDraftValues = Pick<PromoFormValues, (typeof PROMO_DRAFT_FIELDS)[number]>;

export function promoDraftValues(form: PromoFormValues): PromoDraftValues {
  const values = {} as Record<string, unknown>;
  for (const field of PROMO_DRAFT_FIELDS) {
    values[field] = form[field];
  }
  return values as PromoDraftValues;
}

/** Ключ черновика карточки. id === null — черновик новой карточки. */
export function promoDraftKey(id: number | null): string {
  return draftStorageKey(DRAFT_BASE_KEY, id);
}
