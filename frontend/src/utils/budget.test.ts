import { describe, expect, it } from 'vitest';
import { budgetFormat, budgetParse } from './budget';
describe('budget input and formatting', () => {
  it('parses Russian decimal input in millions and rejects empty or non-finite values', () => {
    expect(budgetParse('1 234,5')).toBe(1234500000);
    expect(budgetParse('-0,01')).toBe(-10000);
    expect(budgetParse('')).toBeNull();
    expect(budgetParse('Infinity')).toBeNull();
  });
  it('keeps missing percentages different from zero', () => {
    expect(budgetFormat(null, true)).toBe('—');
    expect(budgetFormat(0, true)).toBe('0,0 %');
    expect(budgetFormat(1200000)).toBe('1,2');
  });
});
