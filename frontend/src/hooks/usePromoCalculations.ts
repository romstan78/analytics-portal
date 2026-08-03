import { useCallback } from 'react';
import type { PromoFormValues } from './usePromoForm';

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
    const ppu = parseFloat(f.plan_promo_units) || 0;
    const cp = parseFloat(f.contract_price) || 0;
    const bu = parseFloat(f.baseline_units) || 0;
    const pir = parseFloat(f.plan_investments_rub) || 0;
    const gm = parseFloat(f.gm) || 1;
    const plan_rub = ppu * cp;
    const uplift_units = ppu - bu;
    const uplift_rub = uplift_units * cp;
    const roi = pir > 0 ? ((uplift_rub / pir) * gm * 100 - 100) : 0;
    const baseline_rub = bu * cp;
    return {
      plan_promo_rub: plan_rub.toFixed(2),
      plan_promo_uplift_units: uplift_units.toFixed(2),
      plan_promo_uplift_rub: uplift_rub.toFixed(2),
      plan_roi: roi.toFixed(1),
      baseline_rub: baseline_rub.toFixed(2),
    };
  }, [form]);

  const recalcActual = useCallback((updates: Partial<PromoFormValues>): ActualFields => {
    const f = { ...form, ...updates };
    const afu = parseFloat(f.actual_promo_sales_units) || 0;
    const cp = parseFloat(f.contract_price) || 0;
    const bu = parseFloat(f.baseline_units) || 0;
    const afi = parseFloat(f.actual_investments) || 0;
    const gm = parseFloat(f.gm) || 1;
    const afr = afu * cp;
    const afupl = afu - bu;
    const afupr = afupl * cp;
    const aroi = afi > 0 ? ((afupr / afi) * gm * 100 - 100) : 0;
    return {
      actual_promo_rub: afr.toFixed(2),
      actual_promo_uplift_units: afupl.toFixed(2),
      actual_promo_uplift_rub: afupr.toFixed(2),
      actual_roi: aroi.toFixed(1),
    };
  }, [form]);

  return { recalcPlan, recalcActual };
}