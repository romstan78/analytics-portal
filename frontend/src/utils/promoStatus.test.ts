import { describe, expect, it } from 'vitest';
import { isPromoFactEditable, promoStatusColor, PROMO_STATUS } from './promoStatus';

describe('isPromoFactEditable', () => {
  it('открывает факт только после финализации', () => {
    expect(isPromoFactEditable(PROMO_STATUS.finalized)).toBe(true);
    expect(isPromoFactEditable(PROMO_STATUS.done)).toBe(true);
    expect(isPromoFactEditable(PROMO_STATUS.inApproval)).toBe(false);
    expect(isPromoFactEditable(PROMO_STATUS.rejected)).toBe(false);
    expect(isPromoFactEditable('')).toBe(false);
    expect(isPromoFactEditable(null)).toBe(false);
  });
});

describe('promoStatusColor', () => {
  it('раскрашивает четыре статуса и не зависит от регистра', () => {
    expect(promoStatusColor('Проведено')).toBe('success');
    expect(promoStatusColor('финализировано')).toBe('success');
    expect(promoStatusColor('В процессе согласования')).toBe('warning');
    expect(promoStatusColor('Отклонено')).toBe('error');
    expect(promoStatusColor('Планируется')).toBe('default');
    expect(promoStatusColor(null)).toBe('default');
  });
});
