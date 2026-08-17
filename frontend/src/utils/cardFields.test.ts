import { describe, expect, it } from 'vitest';
import { DEFAULT_VISIBLE_FIELDS, normalizeVisibleFields } from './cardFields';

describe('normalizeVisibleFields', () => {
  it('returns defaults for invalid stored settings', () => {
    expect(normalizeVisibleFields({ field: 'sku' })).toEqual(DEFAULT_VISIBLE_FIELDS);
  });

  it('drops unknown and duplicate field identifiers', () => {
    expect(normalizeVisibleFields(['sku', 'unknown', 'sku', 'plan_roi'])).toEqual(['sku', 'plan_roi']);
  });

  it('allows the user to hide every optional field', () => {
    expect(normalizeVisibleFields([])).toEqual([]);
  });
});
