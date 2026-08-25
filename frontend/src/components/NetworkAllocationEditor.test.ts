import { describe, expect, it } from 'vitest';
import { isMonthDistributionValid } from '../utils/networkPlan';

describe('isMonthDistributionValid', () => {
  it('принимает профиль с суммой 100%', () => {
    expect(isMonthDistributionValid(['30', '30', '40'])).toBe(true);
    expect(isMonthDistributionValid(['20,5', '29,5', '50'])).toBe(true);
  });

  it('отклоняет неполный профиль и значения вне диапазона', () => {
    expect(isMonthDistributionValid(['30', '', '40'])).toBe(false);
    expect(isMonthDistributionValid(['-10', '50', '60'])).toBe(false);
    expect(isMonthDistributionValid(['30', '30', '30'])).toBe(false);
  });
});
