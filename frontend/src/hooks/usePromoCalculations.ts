import { useCallback } from 'react';
import type { PromoFormValues } from './usePromoForm';
import { calcPlan, calcActual } from '../utils/calcUtils';

interface PlanFields {
  plan_promo_rub: string;
  plan_promo_uplift_units: string;
  plan_promo_uplift_rub: string;
  plan_roi: string;
  baseline_rub: string;
}

interface ActualFields {
  actual_promo_rub: string;
  actual_promo_uplift_units: string;
  actual_promo_uplift_rub: string;
  actual_roi: string;
}

export function usePromoCalculations(form: PromoFormValues) {
  const recalcPlan = useCallback((updates: Partial<PromoFormValues>): PlanFields => {
    const f = { ...form, ...updates };
    const r = calcPlan({
      plan_promo_units: parseFloat(f.plan_promo_units) || 0,
      contract_price: parseFloat(f.contract_price) || 0,
      baseline_units: parseFloat(f.baseline_units) || 0,
      plan_investments_rub: parseFloat(f.plan_investments_rub) || 0,
      gm: parseFloat(f.gm) || 1,
    });
    return {
      plan_promo_rub: r.plan_promo_rub.toFixed(2),
      plan_promo_uplift_units: r.plan_promo_uplift_units.toFixed(2),
      plan_promo_uplift_rub: r.plan_promo_uplift_rub.toFixed(2),
      plan_roi: r.plan_roi.toFixed(1),
      baseline_rub: r.baseline_rub.toFixed(2),
    };
  }, [form]);

  const recalcActual = useCallback((updates: Partial<PromoFormValues>): ActualFields => {
    const f = { ...form, ...updates };
    const r = calcActual({
      actual_promo_sales_units: parseFloat(f.actual_promo_sales_units) || 0,
      contract_price: parseFloat(f.contract_price) || 0,
      baseline_units: parseFloat(f.baseline_units) || 0,
      actual_investments: parseFloat(f.actual_investments) || 0,
      gm: parseFloat(f.gm) || 1,
    });
    return {
      actual_promo_rub: r.actual_promo_rub.toFixed(2),
      actual_promo_uplift_units: r.actual_promo_uplift_units.toFixed(2),
      actual_promo_uplift_rub: r.actual_promo_uplift_rub.toFixed(2),
      actual_roi: r.actual_roi.toFixed(1),
    };
  }, [form]);

  return { recalcPlan, recalcActual };
}
