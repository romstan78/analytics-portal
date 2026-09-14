import { describe, it, expect } from 'vitest';
import {
  EMPTY_AMOUNTS,
  EMPTY_CELL,
  amountsOfPlan,
  buildAmounts,
  buildDraft,
  canAddScale,
  deltaPct,
  formatRubShort,
  formatSignedPct,
  isVATRateValid,
  parseNumberInput,
  periodGroupConflict,
  periodGroupKey,
  planKey,
  scaleErrors,
  scaleThreshold,
  scalesInput,
  shiftGrossPool,
  skusOf,
  stepLabel,
  thresholdFromStep,
  upperScales,
  withScale,
  withSkus,
  withoutScale,
} from './networkPlan';
import type { DraftCell, DraftSKU } from './networkPlan';
import type { NetworkPlan } from '../types/network';
import type { NetworkPeriodGroupInput } from '../types/network';

// Расчёт НДС, инвестиций и итогов проверяется в backend/services:
// на фронтенде их больше нет, поэтому здесь только разбор ввода и показ.

describe('parseNumberInput', () => {
  it('понимает пробелы-разделители и запятую', () => {
    expect(parseNumberInput('1 200,50')).toBe(1200.5);
  });

  it('пустая строка — снятое значение, а не ноль', () => {
    expect(parseNumberInput('')).toBeNull();
  });

  it('нечисловой ввод не проходит', () => {
    expect(parseNumberInput('нет')).toBeNull();
  });
});

describe('formatRubShort', () => {
  it('сокращает миллионы и тысячи', () => {
    expect(formatRubShort(1_200_000)).toBe('1,2 млн');
    expect(formatRubShort(840_000)).toBe('840 тыс');
  });

  it('мелкие суммы показывает полностью', () => {
    // toLocaleString разделяет разряды неразрывным пробелом.
    expect(formatRubShort(9500)).toBe('9 500');
  });

  it('пустое значение — прочерк', () => {
    expect(formatRubShort(null)).toBe('—');
  });
});

describe('deltaPct / formatSignedPct', () => {
  it('считает отклонение от базы', () => {
    expect(deltaPct(1_150_000, 1_200_000)).toBe(-4.17);
  });

  it('без базы отклонения нет', () => {
    expect(deltaPct(100, null)).toBeNull();
    expect(deltaPct(100, 0)).toBeNull();
  });

  it('подписывает знак отклонения', () => {
    expect(formatSignedPct(4.2)).toBe('+4,2 %');
    expect(formatSignedPct(-4.2)).toBe('−4,2 %');
    expect(formatSignedPct(null)).toBe('—');
  });
});

describe('amountsOfPlan', () => {
  const plan = (patch: Partial<NetworkPlan>): NetworkPlan => ({
    id: 1, network_id: 1, year: 2026, quarter: 1, brand_as: 'Альфа', in_gross: true,
    plan_rub: null, plan_units: null, entry_level: 'brand', entry_unit: 'rub',
    month1_pct: 30, month2_pct: 30, month3_pct: 40,
    fact_rub: null, forecast_rub: null,
    fact_investments_rub: null, fact_investments_rub_net: null,
    investments_pct: null, investments_rub: null, investments_rub_net: null,
    forecast_investments_rub: null, forecast_investments_rub_net: null,
    pay_investments_from_fact: false,
    paid_investments_rub: null, forecast_investments_overridden: false,
    investment_scope: '', investment_period_start_quarter: 0, investment_period_end_quarter: 0,
    forecast_completion_pct: null, forecast_investments_earned: false,
    fact_completion_pct: null, fact_investments_earned: false,
    cap_mode: 'open', cap_pct: null, scales: [],
    forecast_scale: 0, fact_scale: 0, forecast_base_rub: null, fact_base_rub: null,
    updated_by: null, updated_at: '', ...patch,
  });

  it('переносит расчёт бэкенда в ячейку без пересчёта', () => {
    const amounts = amountsOfPlan(plan({
      plan_rub: 1_200_000,
      investments_rub: 120_000,
      investments_rub_net: 100_000,
    }));
    expect(amounts.plan).toBe(1_200_000);
    expect(amounts.investPlan).toBe(120_000);
    expect(amounts.investPlanNet).toBe(100_000);
  });

  it('строки без расчёта дают пустую ячейку', () => {
    expect(amountsOfPlan(undefined)).toEqual(EMPTY_AMOUNTS);
  });

  it('раскладывает строки по ключу «квартал|бренд»', () => {
    const amounts = buildAmounts([
      plan({ quarter: 2, brand_as: 'Бета', plan_rub: 500 }),
      plan({ quarter: 1, brand_as: null, plan_rub: 900 }),
    ]);
    expect(amounts[planKey(2, 'Бета')].plan).toBe(500);
    expect(amounts[planKey(1, null)].plan).toBe(900);
  });
});

describe('shiftGrossPool', () => {
  const cell = (patch: Partial<typeof EMPTY_CELL>) => ({ ...EMPTY_CELL, ...patch });

  it('вывод бренда уменьшает пул на его объём', () => {
    const pool = shiftGrossPool(cell({ planRub: '10 000 000' }), cell({ planRub: '1 500 000' }), false);
    expect(parseNumberInput(pool.planRub)).toBe(8_500_000);
  });

  it('перевод бренда в пул увеличивает пул на его объём', () => {
    const pool = shiftGrossPool(cell({ planRub: '8 500 000' }), cell({ planRub: '1 500 000' }), true);
    expect(parseNumberInput(pool.planRub)).toBe(10_000_000);
  });

  // Прогноз пул больше не двигает: он ведётся помесячно и приходит сводом,
  // а не вводится в этой сетке.
  it('двигает план и не выдумывает прогноз', () => {
    const pool = shiftGrossPool(cell({ planRub: '1 000' }), cell({ planRub: '100' }), false);
    expect(parseNumberInput(pool.planRub)).toBe(900);
  });

  it('первый переведённый бренд создаёт валовый пул своим объёмом', () => {
    const pool = shiftGrossPool(undefined, cell({ planRub: '500' }), true);
    expect(parseNumberInput(pool.planRub)).toBe(500);
  });

  it('вывод бренда не создаёт отсутствующий валовый пул', () => {
    const pool = shiftGrossPool(undefined, cell({ planRub: '500' }), false);
    expect(pool.planRub).toBe('');
  });

  it('пустой объём бренда пул не меняет', () => {
    const pool = shiftGrossPool(cell({ planRub: '1 000' }), EMPTY_CELL, false);
    expect(pool.planRub).toBe('1 000');
  });

  it('ниже нуля пул не опускается', () => {
    const pool = shiftGrossPool(cell({ planRub: '1 000' }), cell({ planRub: '1 500' }), false);
    expect(parseNumberInput(pool.planRub)).toBe(0);
  });

  it('признак валового объёма самой строки пула не трогает', () => {
    const pool = shiftGrossPool(cell({ planRub: '1 000', inGross: false }), cell({ planRub: '100' }), true);
    expect(pool.inGross).toBe(false);
  });
});

describe('period groups', () => {
  const group = (patch: Partial<NetworkPeriodGroupInput>): NetworkPeriodGroupInput => ({
    start_quarter: 1,
    end_quarter: 2,
    brand_as: null,
    updated_at: '',
    ...patch,
  });

  it('строит отдельные ключи для портфеля и бренда', () => {
    expect(periodGroupKey(group({}))).toBe('1|2|*');
    expect(periodGroupKey(group({ brand_as: 'Альфа' }))).toBe('1|2|Альфа');
  });

  it('разрешает пересекающиеся периоды разным брендам', () => {
    const current = [group({ start_quarter: 1, end_quarter: 3, brand_as: 'Альфа' })];
    expect(periodGroupConflict(current, group({ start_quarter: 2, end_quarter: 4, brand_as: 'Бета' }))).toBeNull();
  });

  it('не разрешает пересечение портфеля с брендом', () => {
    const current = [group({ start_quarter: 1, end_quarter: 2 })];
    expect(periodGroupConflict(current, group({ start_quarter: 2, end_quarter: 4, brand_as: 'Альфа' }))).toContain('Пересекается');
  });

  it('не разрешает одному бренду участвовать в двух пересекающихся группах', () => {
    const current = [group({ start_quarter: 1, end_quarter: 3, brand_as: 'Альфа' })];
    expect(periodGroupConflict(current, group({ start_quarter: 3, end_quarter: 4, brand_as: 'Альфа' }))).toContain('Пересекается');
  });
});

describe('isVATRateValid', () => {
  it('принимает ставки, которые примет CK_NetworkPeriods_vat_rate', () => {
    expect(isVATRateValid('20')).toBe(true);
    expect(isVATRateValid('0')).toBe(true);
    expect(isVATRateValid('99,99')).toBe(true);
    expect(isVATRateValid(' 5.5 ')).toBe(true);
  });

  it('отклоняет пустое поле, отрицательные и 100% и выше', () => {
    expect(isVATRateValid('')).toBe(false);
    expect(isVATRateValid('-1')).toBe(false);
    expect(isVATRateValid('100')).toBe(false);
    expect(isVATRateValid('двадцать')).toBe(false);
  });
});

describe('ступени контракта', () => {
  const cell = (): DraftCell => ({
    ...EMPTY_CELL,
    planRub: '100 000 000',
    investmentsPct: '5',
    scales: [
      { scaleNo: 2, planRub: '110 000 000', planUnits: '', investmentsPct: '6,5', skus: [], updatedAt: '' },
      { scaleNo: 3, planRub: '120 000 000', planUnits: '', investmentsPct: '8', skus: [], updatedAt: '' },
    ],
  });

  it('шаг задаётся в рублях или процентах и даёт порог', () => {
    expect(thresholdFromStep(100, '+10')).toBe(110);
    expect(thresholdFromStep(100, '10 %')).toBe(110);
    expect(thresholdFromStep(100, '+12,5%')).toBe(112.5);
    expect(thresholdFromStep(null, '+10')).toBeNull();
    expect(thresholdFromStep(100, 'нет')).toBeNull();
  });

  it('подписывает шаг между порогами суммой и процентом', () => {
    expect(stepLabel(100_000_000, 110_000_000)).toBe('+10 млн · +10 %');
    expect(stepLabel(null, 110)).toBe('');
  });

  it('пороги владельца обязаны расти', () => {
    expect(scaleErrors(cell(), true)).toEqual({});
    const broken = withScale(cell(), 3, { planRub: '105 000 000' });
    expect(scaleErrors(broken, true)[3]).toMatch(/выше ступени 2/);
    // У валового бренда верхние ступени — планы, им расти не обязательно.
    expect(scaleErrors(broken, false)).toEqual({});
    const empty = withScale(cell(), 3, { planRub: '' });
    expect(scaleErrors(empty, true)[3]).toBe('Порог не задан');
  });

  it('удаление ступени уносит и ступени выше', () => {
    expect(upperScales(withoutScale(cell(), 2))).toEqual([]);
    expect(upperScales(withoutScale(cell(), 3)).map((s) => s.scaleNo)).toEqual([2]);
  });

  it('число ступеней ограничено настройкой квартала', () => {
    expect(canAddScale(cell(), 3)).toBe(false);
    expect(canAddScale(withoutScale(cell(), 3), 3)).toBe(true);
    expect(canAddScale(withoutScale(cell(), 2), 1)).toBe(false);
  });

  it('SKU ступени 1 живут в записи с номером 1 и уходят вместе с последним SKU', () => {
    const sku: DraftSKU = { sku: 'A', planRub: '10', planUnits: '', investmentsPct: '7', capMode: 'open', capPct: '' };
    const withOne = withSkus(cell(), 1, [sku]);
    expect(skusOf(withOne, 1)).toEqual([sku]);
    expect(scaleThreshold(withOne, 1)).toBe(100_000_000);
    expect(skusOf(withSkus(withOne, 1, []), 1)).toEqual([]);
    expect(withSkus(withOne, 1, []).scales.some((s) => s.scaleNo === 1)).toBe(false);
  });

  it('тело запроса несёт верхние ступени и SKU, у ступени 1 — только SKU', () => {
    const sku: DraftSKU = { sku: 'A', planRub: '10', planUnits: '', investmentsPct: '', capMode: 'pct', capPct: '5' };
    const input = scalesInput(withSkus(cell(), 1, [sku]));
    expect(input.map((s) => s.scale_no)).toEqual([1, 2, 3]);
    expect(input[0].plan_rub).toBeNull();
    expect(input[0].skus[0]).toEqual({ sku: 'A', plan_rub: 10, plan_units: null, investments_pct: null, cap_mode: 'pct', cap_pct: 5 });
    expect(input[1]).toMatchObject({ scale_no: 2, plan_rub: 110_000_000, investments_pct: 6.5 });
    expect(scalesInput(EMPTY_CELL)).toEqual([]);
  });

  it('черновик собирается из ответа: ступень 1 только ради SKU, крышка и пара', () => {
    const plan: NetworkPlan = {
      id: 1, network_id: 1, year: 2026, quarter: 1, brand_as: 'Альфа', in_gross: false,
      plan_rub: 100, plan_units: 40, entry_level: 'brand', entry_unit: 'units',
      month1_pct: 30, month2_pct: 30, month3_pct: 40,
      fact_rub: null, forecast_rub: null,
      fact_investments_rub: null, fact_investments_rub_net: null,
      investments_pct: 5, investments_rub: null, investments_rub_net: null,
      forecast_investments_rub: null, forecast_investments_rub_net: null,
      pay_investments_from_fact: false,
      paid_investments_rub: null, forecast_investments_overridden: false,
      investment_scope: '', investment_period_start_quarter: 0, investment_period_end_quarter: 0,
      forecast_completion_pct: null, forecast_investments_earned: false,
      fact_completion_pct: null, fact_investments_earned: false,
      cap_mode: 'pct', cap_pct: 5,
      forecast_scale: 0, fact_scale: 0, forecast_base_rub: null, fact_base_rub: null,
      updated_by: null, updated_at: '',
      scales: [
        { id: 1, scale_no: 1, plan_rub: 100, plan_units: 40, investments_pct: 5, effective_investments_pct: 5, skus: [], updated_by: null, updated_at: 'v1',
          plan_investments_rub: null, plan_investments_rub_net: null, forecast_rub: null, forecast_base_rub: null,
          forecast_investments_rub: null, forecast_investments_rub_net: null, forecast_reached: false,
          fact_rub: null, fact_base_rub: null, fact_investments_rub: null, fact_investments_rub_net: null, fact_reached: false },
        { id: 2, scale_no: 2, plan_rub: 110, plan_units: 44, investments_pct: 6.5, effective_investments_pct: 6.5, updated_by: null, updated_at: 'v2',
          skus: [{ id: 5, sku: 'A', plan_rub: 60, plan_units: 24, investments_pct: null, cap_mode: '', cap_pct: null,
            plan_investments_rub: null, plan_investments_rub_net: null, forecast_rub: null, forecast_base_rub: null,
            forecast_investments_rub: null, forecast_investments_rub_net: null, fact_rub: null, fact_base_rub: null,
            fact_investments_rub: null, fact_investments_rub_net: null, updated_at: '' }],
          plan_investments_rub: null, plan_investments_rub_net: null, forecast_rub: null, forecast_base_rub: null,
          forecast_investments_rub: null, forecast_investments_rub_net: null, forecast_reached: false,
          fact_rub: null, fact_base_rub: null, fact_investments_rub: null, fact_investments_rub_net: null, fact_reached: false },
      ],
    };
    const draft = buildDraft([plan])[planKey(1, 'Альфа')];
    expect(draft.entryUnit).toBe('units');
    expect(draft.planUnits).toBe('40');
    expect(draft.capMode).toBe('pct');
    expect(draft.capPct).toBe('5');
    expect(draft.scales.map((s) => s.scaleNo)).toEqual([2]);
    expect(draft.scales[0].updatedAt).toBe('v2');
    expect(draft.scales[0].skus[0]).toMatchObject({ sku: 'A', planRub: '60', planUnits: '24', investmentsPct: '', capMode: '' });
  });
});
