import { describe, it, expect } from 'vitest';
import {
  EMPTY_AMOUNTS,
  amountsOfPlan,
  buildAmounts,
  deltaPct,
  formatRubShort,
  formatSignedPct,
  parseNumberInput,
  planKey,
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
    plan_rub: null, plan_units: null, fact_rub: null, forecast_rub: null,
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
