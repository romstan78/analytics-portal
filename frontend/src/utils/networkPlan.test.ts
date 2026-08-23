import { describe, it, expect } from 'vitest';
import {
  EMPTY_AMOUNTS,
  EMPTY_CELL,
  amountsOfPlan,
  buildAmounts,
  deltaPct,
  formatRubShort,
  formatSignedPct,
  parseNumberInput,
  planKey,
  shiftGrossPool,
} from './networkPlan';
import type { NetworkPlan } from '../types/network';

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
    plan_rub: null, plan_units: null, month1_pct: 30, month2_pct: 30, month3_pct: 40,
    fact_rub: null, forecast_rub: null,
    fact_investments_rub: null, fact_investments_rub_net: null,
    investments_pct: null, investments_rub: null, investments_rub_net: null,
    forecast_investments_rub: null, forecast_investments_rub_net: null,
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

  it('двигает и прогноз, и план', () => {
    const pool = shiftGrossPool(
      cell({ planRub: '1 000', forecastRub: '900' }),
      cell({ planRub: '100', forecastRub: '90' }),
      false,
    );
    expect(parseNumberInput(pool.planRub)).toBe(900);
    expect(parseNumberInput(pool.forecastRub)).toBe(810);
  });

  it('пустой пул не заполняет: валовый объём в квартале не ведут', () => {
    const pool = shiftGrossPool(undefined, cell({ planRub: '500' }), true);
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
