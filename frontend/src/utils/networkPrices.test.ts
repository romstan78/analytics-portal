import { describe, expect, it } from 'vitest';
import type { NetworkContractPrice, NetworkPriceSKUOption } from '../types/network';
import {
  buildQuarterlyPriceDrafts,
  createQuarterlyPriceDraftFromOption,
  filterAvailableSKUOptions,
  quarterlyPriceInputs,
} from './networkPrices';

const priceRow = (patch: Partial<NetworkContractPrice> = {}): NetworkContractPrice => ({
  id: 11,
  network_id: 3,
  brand_as: 'Бренд А',
  sku: 'Длинное наименование SKU 100 мг №30',
  contract_price: 125.5,
  valid_from: '2026-01-01',
  valid_to: '2026-12-31',
  source_type: 'olap_seed',
  source_year: 2026,
  source_month: 7,
  is_confirmed: false,
  olap_price: 125.5,
  olap_year: 2026,
  olap_month: 7,
  updated_by: null,
  updated_at: '2026-08-24 10:00:00.000',
  ...patch,
});

describe('quarterly network prices', () => {
  it('shows an annual OLAP default in all four quarter cells', () => {
    const [draft] = buildQuarterlyPriceDrafts([priceRow()], 2026);

    expect(draft.sku).toBe('Длинное наименование SKU 100 мг №30');
    expect(draft.prices).toEqual(['125,5', '125,5', '125,5', '125,5']);
    expect(draft.periodIds).toEqual([11, 11, 11, 11]);
    expect(draft.confirmed).toBe(false);
    expect(draft.canDelete).toBe(false);
  });

  it('serializes the matrix into four non-overlapping quarter periods', () => {
    const [draft] = buildQuarterlyPriceDrafts([priceRow()], 2026);
    draft.prices = ['100', '110', '120', '130'];
    draft.confirmed = true;

    const rows = quarterlyPriceInputs([draft], 2026);

    expect(rows.map((row) => row.id)).toEqual([11, 0, 0, 0]);
    expect(rows.map((row) => [row.valid_from, row.valid_to])).toEqual([
      ['2026-01-01', '2026-03-31'],
      ['2026-04-01', '2026-06-30'],
      ['2026-07-01', '2026-09-30'],
      ['2026-10-01', '2026-12-31'],
    ]);
    expect(rows.map((row) => row.contract_price)).toEqual([100, 110, 120, 130]);
    expect(rows.every((row) => row.is_confirmed)).toBe(true);
  });

  it('keeps existing ids when prices are already stored by quarter', () => {
    const rows = [
      priceRow({ id: 1, valid_from: '2026-01-01', valid_to: '2026-03-31', contract_price: 101, is_confirmed: true }),
      priceRow({ id: 2, valid_from: '2026-04-01', valid_to: '2026-06-30', contract_price: 102, is_confirmed: true }),
      priceRow({ id: 3, valid_from: '2026-07-01', valid_to: '2026-09-30', contract_price: 103, is_confirmed: true }),
      priceRow({ id: 4, valid_from: '2026-10-01', valid_to: '2026-12-31', contract_price: 104, is_confirmed: true }),
    ];

    const [draft] = buildQuarterlyPriceDrafts(rows, 2026);
    const inputs = quarterlyPriceInputs([draft], 2026);

    expect(draft.prices).toEqual(['101', '102', '103', '104']);
    expect(inputs.map((row) => row.id)).toEqual([1, 2, 3, 4]);
  });

  it('allows deletion only for persisted manually created SKU rows', () => {
    const [manual] = buildQuarterlyPriceDrafts([
      priceRow({
        source_type: 'manual', source_year: null, source_month: null,
        olap_price: null, olap_year: null, olap_month: null,
      }),
    ], 2026);
    const [olap] = buildQuarterlyPriceDrafts([priceRow()], 2026);

    expect(manual.canDelete).toBe(true);
    expect(manual.deleteRows).toEqual([{ id: 11, updated_at: '2026-08-24 10:00:00.000' }]);
    expect(olap.canDelete).toBe(false);
  });

  it('fills brand and all quarter prices from a selected OLAP SKU', () => {
    const option: NetworkPriceSKUOption = {
      brand_as: 'Бренд Б',
      sku: 'SKU из списка',
      price: 77.25,
      source_year: 2026,
      source_month: 8,
    };

    const draft = createQuarterlyPriceDraftFromOption(option, 'new-1');

    expect(draft.brand).toBe('Бренд Б');
    expect(draft.sku).toBe('SKU из списка');
    expect(draft.prices).toEqual(['77,25', '77,25', '77,25', '77,25']);
    expect(draft.olapPrice).toBe(77.25);
    expect(draft.isNew).toBe(true);
    expect(draft.canDelete).toBe(true);
  });

  it('excludes SKU already shown in the price table from add options', () => {
    const options: NetworkPriceSKUOption[] = [
      { brand_as: 'A', sku: 'SKU 1', price: 10, source_year: 2026, source_month: 8 },
      { brand_as: 'B', sku: 'SKU 2', price: 20, source_year: 2026, source_month: 8 },
    ];
    const drafts = [createQuarterlyPriceDraftFromOption(options[0], 'new-1')];

    expect(filterAvailableSKUOptions(options, drafts)).toEqual([options[1]]);
  });
});
