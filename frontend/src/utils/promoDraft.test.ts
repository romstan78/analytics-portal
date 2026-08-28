import { describe, expect, it, beforeEach, vi } from 'vitest';
import { EMPTY_FORM, formFromRow } from '../hooks/usePromoForm';
import { PROMO_DRAFT_FIELDS, promoDraftKey, promoDraftValues } from './promoDraft';

function fakeStorage() {
  const data = new Map<string, string>();
  return {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => void data.set(key, value),
    removeItem: (key: string) => void data.delete(key),
    clear: () => data.clear(),
  };
}

describe('promoDraftValues', () => {
  // Расчётные поля приходят с сервера. Сохранённые в черновике, они
  // разошлись бы с формулой при восстановлении.
  it('не берёт в черновик расчётные поля', () => {
    const values = promoDraftValues({
      ...EMPTY_FORM,
      baseline_units: '100',
      plan_roi: '42.0',
      plan_promo_rub: '999',
      plan_promo_uplift_rub: '888',
      baseline_rub: '777',
      actual_roi: '13.0',
      actual_promo_rub: '666',
      actual_promo_uplift_units: '555',
    });

    expect(values.baseline_units).toBe('100');
    for (const field of [
      'plan_roi', 'plan_promo_rub', 'plan_promo_uplift_rub', 'baseline_rub',
      'actual_roi', 'actual_promo_rub', 'actual_promo_uplift_units',
    ]) {
      expect(Object.hasOwn(values, field)).toBe(false);
    }
  });

  // Согласование и история комментариев правятся отдельным процессом.
  it('не берёт в черновик согласование, комментарии и служебные поля', () => {
    for (const field of ['agreement1', 'agreement2', 'comments', 'id', 'updated_at', 'deleted_at']) {
      expect(PROMO_DRAFT_FIELDS as readonly string[]).not.toContain(field);
    }
  });

  it('берёт всё, что вводит пользователь', () => {
    const values = promoDraftValues(formFromRow({
      id: 994, sku: 'SKU-1', network_name: 'Аптека №1', year: 2026, month: 3,
      total_pharmacies: 120, conditions: 'условия',
    }));

    expect(values.sku).toBe('SKU-1');
    expect(values.network_name).toBe('Аптека №1');
    expect(values.year).toBe(2026);
    expect(values.total_pharmacies).toBe('120');
    expect(values.conditions).toBe('условия');
  });
});

describe('promoDraftKey', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', fakeStorage());
    localStorage.setItem('username', 'kam.ershov');
  });

  it('разводит новую карточку и существующее промо', () => {
    expect(promoDraftKey(994)).not.toBe(promoDraftKey(null));
  });
});
