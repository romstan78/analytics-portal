import { describe, it, expect } from 'vitest';
import { calcPlan, calcActual, type PlanInput, type ActualInput } from './calcUtils';

describe('calcPlan', () => {
  it('рассчитывает plan_promo_rub = units * price', () => {
    const result = calcPlan({ plan_promo_units: 100, contract_price: 200, baseline_units: 0, plan_investments_rub: 0, gm: 1 });
    expect(result.plan_promo_rub).toBe(20000);
  });

  it('рассчитывает uplift_units = plan - baseline', () => {
    const result = calcPlan({ plan_promo_units: 150, contract_price: 100, baseline_units: 100, plan_investments_rub: 0, gm: 1 });
    expect(result.plan_promo_uplift_units).toBe(50);
  });

  it('рассчитывает uplift_rub = uplift_units * price', () => {
    const result = calcPlan({ plan_promo_units: 150, contract_price: 100, baseline_units: 100, plan_investments_rub: 0, gm: 1 });
    expect(result.plan_promo_uplift_rub).toBe(5000);
  });

  it('рассчитывает ROI = (uplift_rub / investments) * gm * 100 - 100', () => {
    // uplift_rub = (150-100)*200 = 10000; ROI = (10000/50000)*0.5*100 - 100 = 10 - 100 = -90
    const result = calcPlan({ plan_promo_units: 150, contract_price: 200, baseline_units: 100, plan_investments_rub: 50000, gm: 0.5 });
    expect(result.plan_roi).toBe(-90);
  });

  it('положительный ROI при окупаемости', () => {
    // uplift_rub = (300-100)*200 = 40000; ROI = (40000/20000)*0.5*100 - 100 = 100 - 100 = 0
    const result = calcPlan({ plan_promo_units: 300, contract_price: 200, baseline_units: 100, plan_investments_rub: 20000, gm: 0.5 });
    expect(result.plan_roi).toBe(0);
  });

  it('ROI = 0 при нулевых инвестициях', () => {
    const result = calcPlan({ plan_promo_units: 200, contract_price: 100, baseline_units: 100, plan_investments_rub: 0, gm: 0.56 });
    expect(result.plan_roi).toBe(0);
  });

  it('baseline_rub = baseline_units * price', () => {
    const result = calcPlan({ plan_promo_units: 0, contract_price: 300, baseline_units: 100, plan_investments_rub: 0, gm: 1 });
    expect(result.baseline_rub).toBe(30000);
  });

  it('отрицательный uplift при плане меньше baseline', () => {
    const result = calcPlan({ plan_promo_units: 80, contract_price: 200, baseline_units: 100, plan_investments_rub: 5000, gm: 1 });
    expect(result.plan_promo_uplift_units).toBe(-20);
    expect(result.plan_promo_uplift_rub).toBe(-4000);
  });

  it('все нули дают нулевой результат', () => {
    const result = calcPlan({ plan_promo_units: 0, contract_price: 0, baseline_units: 0, plan_investments_rub: 0, gm: 1 });
    expect(result).toEqual({
      plan_promo_rub: 0, plan_promo_uplift_units: 0, plan_promo_uplift_rub: 0, plan_roi: 0, baseline_rub: 0,
    });
  });

  it('GM=1 (значение по умолчанию)', () => {
    // uplift = 200-100=100; uplift_rub=100*150=15000; roi=(15000/30000)*1*100 - 100 = -50
    const result = calcPlan({ plan_promo_units: 200, contract_price: 150, baseline_units: 100, plan_investments_rub: 30000, gm: 1 });
    expect(result.plan_roi).toBe(-50);
  });
});

describe('calcActual', () => {
  it('рассчитывает actual_promo_rub = units * price', () => {
    const result = calcActual({ actual_promo_sales_units: 120, contract_price: 200, baseline_units: 100, actual_investments: 0, gm: 1 });
    expect(result.actual_promo_rub).toBe(24000);
  });

  it('рассчитывает actual_uplift_units = actual - baseline', () => {
    const result = calcActual({ actual_promo_sales_units: 120, contract_price: 200, baseline_units: 100, actual_investments: 0, gm: 1 });
    expect(result.actual_promo_uplift_units).toBe(20);
  });

  it('рассчитывает actual_uplift_rub = uplift_units * price', () => {
    const result = calcActual({ actual_promo_sales_units: 120, contract_price: 200, baseline_units: 100, actual_investments: 0, gm: 1 });
    expect(result.actual_promo_uplift_rub).toBe(4000);
  });

  it('actual ROI при положительных инвестициях', () => {
    // uplift_rub = (120-100)*200 = 4000; ROI = (4000/10000)*0.56*100 - 100 = -77.6
    const result = calcActual({ actual_promo_sales_units: 120, contract_price: 200, baseline_units: 100, actual_investments: 10000, gm: 0.56 });
    expect(result.actual_roi).toBeCloseTo(-77.6, 1);
  });

  it('ROI = 0 при нулевых инвестициях', () => {
    const result = calcActual({ actual_promo_sales_units: 200, contract_price: 100, baseline_units: 100, actual_investments: 0, gm: 0.56 });
    expect(result.actual_roi).toBe(0);
  });

  it('отрицательный uplift при факте меньше baseline', () => {
    const result = calcActual({ actual_promo_sales_units: 80, contract_price: 200, baseline_units: 100, actual_investments: 0, gm: 1 });
    expect(result.actual_promo_uplift_units).toBe(-20);
    expect(result.actual_promo_uplift_rub).toBe(-4000);
  });
});