import { describe, it, expect } from 'vitest';
import {
  EMPTY_CELL,
  calcCell,
  calcQuarterTotals,
  deltaPct,
  formatRubShort,
  formatSignedPct,
  netRub,
  parseNumberInput,
  sumYearTotals,
} from './networkPlan';
import type { DraftCell, QuarterSettings } from './networkPlan';

const cell = (patch: Partial<DraftCell>): DraftCell => ({ ...EMPTY_CELL, ...patch });

const settings = (vat: boolean): Record<number, QuarterSettings> => ({
  1: { vat_included: vat, vat_rate: 20 },
  2: { vat_included: vat, vat_rate: 20 },
  3: { vat_included: vat, vat_rate: 20 },
  4: { vat_included: vat, vat_rate: 20 },
});

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

describe('calcCell', () => {
  it('считает инвестиции одним процентом от плана и от прогноза', () => {
    const amounts = calcCell(
      cell({ planRub: '1200000', forecastRub: '900000', investmentsPct: '10' }),
      { vat_included: true, vat_rate: 20 },
    );

    expect(amounts.investPlan).toBe(120000);
    expect(amounts.investPlanNet).toBe(100000);
    expect(amounts.investForecast).toBe(90000);
    expect(amounts.investForecastNet).toBe(75000);
  });

  it('без процента инвестиции не считаются', () => {
    const amounts = calcCell(cell({ planRub: '1200000' }), { vat_included: true, vat_rate: 20 });
    expect(amounts.investPlan).toBeNull();
    expect(amounts.investForecast).toBeNull();
  });

  it('факт инвестиций берётся суммой, а не процентом', () => {
    const amounts = calcCell(
      // По проценту вышло бы 120000, но загрузка принесла 105600.
      cell({ planRub: '1200000', investmentsPct: '10', factInvestmentsRub: 105600 }),
      { vat_included: true, vat_rate: 20 },
    );

    expect(amounts.investPlan).toBe(120000);
    expect(amounts.investFact).toBe(105600);
    expect(amounts.investFactNet).toBe(88000);
  });
});

describe('calcQuarterTotals', () => {
  it('суммирует планы и считает инвестиции до вычета и с вычетом НДС', () => {
    const draft: Record<string, DraftCell> = {
      '1|Альфа': cell({ planRub: '360000', investmentsPct: '10' }),
      '1|Бета': cell({ planRub: '240000', investmentsPct: '5' }),
    };
    const q1 = calcQuarterTotals(draft, ['Альфа', 'Бета'], settings(true))[0];

    // План не зависит от НДС: 360000 + 240000.
    expect(q1.planRub).toBe(600000);
    // Инвестиции: 36000 + 12000 = 48000, с вычетом НДС 20% — 40000.
    expect(q1.investmentsRub).toBe(48000);
    expect(q1.investmentsRubNet).toBe(40000);
    expect(q1.grossPoolRub).toBeNull();
  });

  it('у сети без НДС обе базы инвестиций совпадают', () => {
    const draft: Record<string, DraftCell> = {
      '1|Альфа': cell({ planRub: '360000', investmentsPct: '10' }),
    };
    const q1 = calcQuarterTotals(draft, ['Альфа'], settings(false))[0];

    expect(q1.investmentsRub).toBe(36000);
    expect(q1.investmentsRubNet).toBe(36000);
  });

  it('остаток к распределению считает только по брендам валового объёма', () => {
    const draft: Record<string, DraftCell> = {
      '1|': cell({ planRub: '600000' }),
      '1|Альфа': cell({ planRub: '360000', inGross: true }),
      '1|Бета': cell({ planRub: '180000', inGross: true }),
      // Бренд вне пула: в остаток не входит и обязательство увеличивает.
      '1|Гамма': cell({ planRub: '250000' }),
    };
    const q1 = calcQuarterTotals(draft, ['Альфа', 'Бета', 'Гамма'], settings(true))[0];

    expect(q1.grossPoolRub).toBe(600000);
    expect(q1.grossBrandsPlan).toBe(540000);
    expect(q1.separatePlanRub).toBe(250000);
    expect(q1.grossBrandsCount).toBe(2);
    expect(q1.undistributed).toBe(60000);
    expect(q1.planRub).toBe(790000);
    expect(q1.contractPlanRub).toBe(850000);
  });

  it('без заведённого пула остатка нет, а обязательство равно сумме брендов', () => {
    const draft: Record<string, DraftCell> = {
      '2|Альфа': cell({ planRub: '300000', inGross: true }),
      '2|Бета': cell({ planRub: '200000', inGross: true }),
    };
    const q2 = calcQuarterTotals(draft, ['Альфа', 'Бета'], settings(false))[1];

    expect(q2.undistributed).toBeNull();
    expect(q2.contractPlanRub).toBe(500000);
  });

  it('общий объём не попадает в сумму по брендам', () => {
    const draft: Record<string, DraftCell> = {
      '2|': cell({ planRub: '500000' }),
      '2|Альфа': cell({ planRub: '500000', inGross: true }),
    };
    const q2 = calcQuarterTotals(draft, ['Альфа'], settings(false))[1];

    expect(q2.planRub).toBe(500000);
    expect(q2.undistributed).toBe(0);
  });

  it('собирает факт и прогноз, включая факт по брендам пула', () => {
    const draft: Record<string, DraftCell> = {
      '4|': cell({ planRub: '500000', forecastRub: '480000' }),
      '4|Альфа': cell({ planRub: '300000', forecastRub: '290000', investmentsPct: '10', inGross: true, factRub: 210000 }),
      '4|Бета': cell({ planRub: '100000', forecastRub: '120000', investmentsPct: '5', factRub: 90000 }),
    };
    const q4 = calcQuarterTotals(draft, ['Альфа', 'Бета'], settings(true))[3];

    expect(q4.factRub).toBe(300000);
    expect(q4.grossPoolFactRub).toBe(210000);
    expect(q4.forecastRub).toBe(410000);
    expect(q4.grossPoolForecastRub).toBe(480000);
    // 29000 + 6000 = 35000 до вычета НДС, / 1.2 = 29166.67.
    expect(q4.forecastInvestmentsRub).toBe(35000);
    expect(q4.forecastInvestmentsRubNet).toBe(29166.67);
  });

  it('складывает факт инвестиций и считает его базу без НДС', () => {
    const draft: Record<string, DraftCell> = {
      '1|Альфа': cell({ planRub: '300000', investmentsPct: '10', factInvestmentsRub: 26400 }),
      '1|Бета': cell({ planRub: '100000', investmentsPct: '5', factInvestmentsRub: 6000 }),
    };
    const q1 = calcQuarterTotals(draft, ['Альфа', 'Бета'], settings(true))[0];

    expect(q1.investmentsRub).toBe(35000);
    expect(q1.factInvestmentsRub).toBe(32400);
    // 26400 / 1.2 + 6000 / 1.2 = 22000 + 5000.
    expect(q1.factInvestmentsRubNet).toBe(27000);
  });
});

describe('sumYearTotals', () => {
  it('складывает кварталы в год', () => {
    const draft: Record<string, DraftCell> = {
      '1|Альфа': cell({ planRub: '100000', investmentsPct: '10', factRub: 80000 }),
      '2|Альфа': cell({ planRub: '200000', investmentsPct: '10', factRub: 150000 }),
    };
    const year = sumYearTotals(calcQuarterTotals(draft, ['Альфа'], settings(false)));

    expect(year.planRub).toBe(300000);
    expect(year.factRub).toBe(230000);
    expect(year.investmentsRub).toBe(30000);
  });

  it('пул суммируется только по кварталам, где он заведён', () => {
    const draft: Record<string, DraftCell> = {
      '1|': cell({ planRub: '600000' }),
      '1|Альфа': cell({ planRub: '500000', inGross: true }),
      '3|Альфа': cell({ planRub: '400000', inGross: true }),
    };
    const year = sumYearTotals(calcQuarterTotals(draft, ['Альфа'], settings(false)));

    expect(year.grossPoolRub).toBe(600000);
    expect(year.undistributed).toBe(100000);
    // Q1 идёт пулом (600000), Q3 — суммой брендов (400000).
    expect(year.contractPlanRub).toBe(1000000);
  });
});
