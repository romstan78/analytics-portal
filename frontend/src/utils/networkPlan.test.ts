import { describe, it, expect } from 'vitest';
import { calcQuarterTotals, netRub, parseNumberInput } from './networkPlan';
import type { DraftCell, QuarterSettings } from './networkPlan';

describe('netRub', () => {
  it('вычитает НДС из инвестиций, если сеть работает с НДС', () => {
    expect(netRub(120000, true, 20)).toBe(100000);
  });

  it('оставляет сумму как есть, если сеть без НДС', () => {
    expect(netRub(120000, false, 20)).toBe(120000);
  });

  it('округляет до копеек', () => {
    expect(netRub(100, true, 20)).toBe(83.33);
  });
});

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

describe('calcQuarterTotals', () => {
  const settings = (contract: 'regular' | 'gross', vat: boolean): Record<number, QuarterSettings> => ({
    1: { vat_included: vat, vat_rate: 20, contract_type: contract },
    2: { vat_included: vat, vat_rate: 20, contract_type: contract },
    3: { vat_included: vat, vat_rate: 20, contract_type: contract },
    4: { vat_included: vat, vat_rate: 20, contract_type: contract },
  });

  it('суммирует планы и считает инвестиции до вычета и с вычетом НДС', () => {
    const draft: Record<string, DraftCell> = {
      '1|Альфа': { planRub: '360000', investmentsPct: '10' },
      '1|Бета': { planRub: '240000', investmentsPct: '5' },
    };
    const q1 = calcQuarterTotals(draft, ['Альфа', 'Бета'], settings('regular', true))[0];

    // План не зависит от НДС: 360000 + 240000.
    expect(q1.planRub).toBe(600000);
    // Инвестиции: 36000 + 12000 = 48000, с вычетом НДС 20% — 40000.
    expect(q1.investmentsRub).toBe(48000);
    expect(q1.investmentsRubNet).toBe(40000);
    expect(q1.grossPlanRub).toBeNull();
  });

  it('у сети без НДС обе базы инвестиций совпадают', () => {
    const draft: Record<string, DraftCell> = {
      '1|Альфа': { planRub: '360000', investmentsPct: '10' },
    };
    const q1 = calcQuarterTotals(draft, ['Альфа'], settings('regular', false))[0];

    expect(q1.investmentsRub).toBe(36000);
    expect(q1.investmentsRubNet).toBe(36000);
  });

  it('показывает остаток к распределению у валового контракта', () => {
    const draft: Record<string, DraftCell> = {
      '1|': { planRub: '600000', investmentsPct: '' },
      '1|Альфа': { planRub: '360000', investmentsPct: '' },
      '1|Бета': { planRub: '180000', investmentsPct: '' },
    };
    const q1 = calcQuarterTotals(draft, ['Альфа', 'Бета'], settings('gross', true))[0];

    expect(q1.grossPlanRub).toBe(600000);
    expect(q1.planRub).toBe(540000);
    expect(q1.undistributed).toBe(60000);
  });

  it('общий объём не попадает в сумму по брендам', () => {
    const draft: Record<string, DraftCell> = {
      '2|': { planRub: '500000', investmentsPct: '' },
      '2|Альфа': { planRub: '500000', investmentsPct: '' },
    };
    const q2 = calcQuarterTotals(draft, ['Альфа'], settings('gross', false))[1];

    expect(q2.planRub).toBe(500000);
    expect(q2.undistributed).toBe(0);
  });
});
