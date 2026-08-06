/**
 * Чистые функции для расчёта плановых и фактических показателей промо.
 * Используются хуком usePromoCalculations и тестируются в calcUtils.test.ts.
 */

export interface PlanInput {
  plan_promo_units: number;
  contract_price: number;
  baseline_units: number;
  plan_investments_rub: number;
  gm: number;
}

export interface PlanOutput {
  plan_promo_rub: number;
  plan_promo_uplift_units: number;
  plan_promo_uplift_rub: number;
  plan_roi: number;
  baseline_rub: number;
}

export function calcPlan(input: PlanInput): PlanOutput {
  const { plan_promo_units: ppu, contract_price: cp, baseline_units: bu, plan_investments_rub: pir, gm } = input;
  const plan_rub = ppu * cp;
  const uplift_units = ppu - bu;
  const uplift_rub = uplift_units * cp;
  const roi = pir > 0 ? ((uplift_rub / pir) * gm * 100 - 100) : 0;
  const baseline_rub = bu * cp;
  return { plan_promo_rub: plan_rub, plan_promo_uplift_units: uplift_units, plan_promo_uplift_rub: uplift_rub, plan_roi: roi, baseline_rub };
}

export interface ActualInput {
  actual_promo_sales_units: number;
  contract_price: number;
  baseline_units: number;
  actual_investments: number;
  gm: number;
}

export interface ActualOutput {
  actual_promo_rub: number;
  actual_promo_uplift_units: number;
  actual_promo_uplift_rub: number;
  actual_roi: number;
}

export function calcActual(input: ActualInput): ActualOutput {
  const { actual_promo_sales_units: afu, contract_price: cp, baseline_units: bu, actual_investments: afi, gm } = input;
  const afr = afu * cp;
  const afupl = afu - bu;
  const afupr = afupl * cp;
  const aroi = afi > 0 ? ((afupr / afi) * gm * 100 - 100) : 0;
  return { actual_promo_rub: afr, actual_promo_uplift_units: afupl, actual_promo_uplift_rub: afupr, actual_roi: aroi };
}