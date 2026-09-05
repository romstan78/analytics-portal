import { describe, expect, it } from 'vitest';
import { driverColor, driverUnitLabel, driverVariance, shownUnit, unitSwitchable } from './promoDrivers';
import type { PromoDashboardMetrics } from '../types/promo';

const metrics = {
  salesVarianceUnits: -70,
  salesVarianceRub: -700,
  investmentVarianceRub: 11,
  upliftVarianceUnits: -6,
  upliftVarianceRub: -60,
  actualRoi: 120,
  comparablePlanRoi: 150,
} as PromoDashboardMetrics;

describe('driverVariance', () => {
  it('переключатель единиц берёт готовое число, а не пересчитывает', () => {
    expect(driverVariance(metrics, 'sales', 'units')).toBe(-70);
    expect(driverVariance(metrics, 'sales', 'rub')).toBe(-700);
    expect(driverVariance(metrics, 'uplift', 'units')).toBe(-6);
    expect(driverVariance(metrics, 'uplift', 'rub')).toBe(-60);
  });

  it('инвестиции остаются в рублях при любом положении переключателя', () => {
    expect(driverVariance(metrics, 'investments', 'units')).toBe(11);
    expect(driverVariance(metrics, 'investments', 'rub')).toBe(11);
  });

  it('ROI — разность процентов, и без одной из сторон её нет', () => {
    expect(driverVariance(metrics, 'roi', 'units')).toBe(-30);
    expect(driverVariance({ ...metrics, actualRoi: null }, 'roi', 'units')).toBeNull();
  });

  it('срез без сопоставимого факта отклонения не даёт', () => {
    const empty = { ...metrics, salesVarianceUnits: null, upliftVarianceRub: null };
    expect(driverVariance(empty, 'sales', 'units')).toBeNull();
    expect(driverVariance(empty, 'uplift', 'rub')).toBeNull();
  });
});

describe('driverUnitLabel', () => {
  it('подпись следует за метрикой, а не только за переключателем', () => {
    expect(driverUnitLabel('sales', 'units')).toBe('уп.');
    expect(driverUnitLabel('sales', 'rub')).toBe('₽');
    expect(driverUnitLabel('uplift', 'units')).toBe('уп.');
    expect(driverUnitLabel('investments', 'units')).toBe('₽');
    expect(driverUnitLabel('roi', 'rub')).toBe('п.п.');
  });

  it('переключать единицы можно только там, где их две', () => {
    expect(unitSwitchable('sales')).toBe(true);
    expect(unitSwitchable('uplift')).toBe(true);
    expect(unitSwitchable('investments')).toBe(false);
    expect(unitSwitchable('roi')).toBe(false);
  });

  it('погашенный переключатель показывает единицу метрики, а не прошлый выбор', () => {
    expect(shownUnit('sales', 'units')).toBe('units');
    expect(shownUnit('investments', 'units')).toBe('rub');
    expect(shownUnit('roi', 'units')).toBeNull();
  });
});

describe('driverColor', () => {
  it('цвет означает «хорошо/плохо», а не знак: перерасход инвестиций — плохо', () => {
    expect(driverColor(11, 'investments')).toBe(driverColor(-70, 'sales'));
    expect(driverColor(-11, 'investments')).toBe(driverColor(70, 'sales'));
  });

  it('у продаж, uplift и ROI хорошо — это плюс', () => {
    const good = driverColor(70, 'sales');
    expect(driverColor(6, 'uplift')).toBe(good);
    expect(driverColor(30, 'roi')).toBe(good);
    expect(driverColor(-30, 'roi')).not.toBe(good);
  });

  it('ноль отклонения красным не считается', () => {
    expect(driverColor(0, 'investments')).toBe(driverColor(0, 'sales'));
  });
});
