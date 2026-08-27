import { describe, it, expect } from 'vitest';
import {
  POLARITY_NEGATIVE,
  POLARITY_POSITIVE,
  amount,
  completionColor,
  completionOf,
  eacCompletionOf,
  gapColor,
  growthLabel,
  growthOf,
  metricEAC,
  metricFact,
  metricPlan,
  metricPrevFact,
  pctLabel,
  ratioPct,
  signedShort,
} from './networkDashboard';
import type { NetworkDashboardMetrics } from '../types/network';

// Суммы и проценты считает backend/services; здесь проверяется только выбор
// величины под единицу измерения и показ — то, что живёт на фронтенде.

const metrics = (patch: Partial<NetworkDashboardMetrics> = {}): NetworkDashboardMetrics => ({
  networkCount: 1,
  brandCount: 3,
  planRub: 1000,
  factRub: 800,
  planUnits: 500,
  factUnits: 450,
  eacUnits: 520,
  eacRub: 1100,
  completionPct: 80,
  eacCompletionPct: 110,
  gapRub: 100,
  planInvestmentsRub: 100,
  planInvestmentsRubNet: 83.33,
  factInvestmentsRub: 90,
  factInvestmentsRubNet: 75,
  eacInvestmentsRub: 120,
  eacInvestmentsRubNet: 100,
  investmentVarianceRub: 16.67,
  effectiveInvestmentsPct: 10.91,
  undistributedRub: null,
  closedCells: 21,
  closedCellsWithFact: 20,
  factCoveragePct: 95.24,
  openCellsWithoutForecast: 5,
  prevPlanRub: 900,
  prevFactRub: 850,
  prevFactUnits: 400,
  factYoyPct: -5.88,
  planYoyPct: 11.11,
  promoCount: 4,
  promoOnlineCount: 2,
  promoOfflineCount: 2,
  promoInvestmentsRub: 50,
  ...patch,
});

describe('выбор величины под единицу измерения', () => {
  it('рубли и упаковки берутся из разных полей', () => {
    const m = metrics();
    expect(metricPlan(m, 'rub')).toBe(1000);
    expect(metricPlan(m, 'units')).toBe(500);
    expect(metricFact(m, 'units')).toBe(450);
    expect(metricEAC(m, 'units')).toBe(520);
    expect(metricPrevFact(m, 'units')).toBe(400);
  });

  it('подпись меняется вместе с единицей: «₽» рядом с упаковками врал бы', () => {
    expect(amount(1500, 'rub')).toContain('₽');
    expect(amount(1500, 'units')).toContain('уп.');
    expect(amount(1500, 'units')).not.toContain('₽');
  });
});

describe('проценты', () => {
  it('в рублях берутся с сервера как есть', () => {
    const m = metrics({ completionPct: 80, eacCompletionPct: 110 });
    expect(completionOf(m, 'rub')).toBe(80);
    expect(eacCompletionOf(m, 'rub')).toBe(110);
  });

  it('в упаковках считаются по той же формуле из тех же сумм', () => {
    const m = metrics({ planUnits: 500, factUnits: 450, eacUnits: 520 });
    expect(completionOf(m, 'units')).toBe(90);
    expect(eacCompletionOf(m, 'units')).toBe(104);
  });

  it('нулевая база — не ноль процентов, а отсутствие ответа', () => {
    expect(ratioPct(10, 0)).toBeNull();
    expect(completionOf(metrics({ planUnits: 0 }), 'units')).toBeNull();
    expect(pctLabel(null)).toBe('—');
  });

  it('прирост в упаковках не берётся из рублёвого поля', () => {
    const m = metrics({ factYoyPct: -5.88, factUnits: 450, prevFactUnits: 400 });
    expect(growthOf(m, 'rub')).toBe(-5.88);
    expect(growthOf(m, 'units')).toBe(12.5);
  });

  it('без сопоставимого периода прироста нет', () => {
    const m = metrics({ prevFactUnits: null, factYoyPct: null });
    expect(growthOf(m, 'rub')).toBeNull();
    expect(growthOf(m, 'units')).toBeNull();
    expect(growthLabel(null)).toBe('—');
  });
});

describe('знак несёт смысл', () => {
  it('прирост всегда со знаком, минус — типографский', () => {
    expect(growthLabel(12.5)).toBe('+12,5 %');
    expect(growthLabel(-3)).toBe('−3 %');
    expect(growthLabel(0)).toBe('0 %');
  });

  it('отклонение показывается со знаком', () => {
    expect(signedShort(120)).toMatch(/^\+/);
    expect(signedShort(-120)).toMatch(/^−/);
  });

  it('цвет отклонения означает «хорошо/плохо», а не «больше/меньше»', () => {
    expect(gapColor(1)).toBe(POLARITY_POSITIVE);
    expect(gapColor(0)).toBe(POLARITY_POSITIVE);
    expect(gapColor(-1)).toBe(POLARITY_NEGATIVE);
  });
});

describe('цвет выполнения', () => {
  it('полярность вокруг ста процентов', () => {
    const done = completionColor(100);
    const near = completionColor(95);
    const missed = completionColor(80);
    expect(done).not.toEqual(near);
    expect(near).not.toEqual(missed);
  });

  it('нет данных — нейтральная ячейка, а не провал', () => {
    expect(completionColor(null)).not.toEqual(completionColor(80));
  });
});
