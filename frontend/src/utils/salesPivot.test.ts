import { describe, expect, it } from 'vitest';
import type { SalesPivotNode } from '../types/sales';
import {
  defaultSalesPivotExpansion,
  filterSalesPivotTree,
  flattenSalesPivotTree,
  salesPivotComparison,
} from './salesPivot';

const tree: SalesPivotNode[] = [{
  id: 'c1', level: 'channel', name: 'PURE', values: {}, children: [{
    id: 's1', level: 'segment', name: 'Аптеки', values: {}, children: [{
      id: 'n1', level: 'network', name: 'Сеть Север', values: {}, children: [
        { id: 'p1', level: 'sku', name: 'SKU Альфа', values: {}, children: [] },
      ],
    }],
  }],
}];

describe('sales pivot tree', () => {
  it('shows networks by default but keeps SKU collapsed', () => {
    const expanded = defaultSalesPivotExpansion(tree);
    expect(flattenSalesPivotTree(tree, expanded).map(row => row.node.id)).toEqual(['c1', 's1', 'n1']);
  });

  it('keeps ancestors when search matches a SKU', () => {
    const filtered = filterSalesPivotTree(tree, 'альфа');
    expect(flattenSalesPivotTree(filtered, new Set(), true).map(row => row.node.id)).toEqual(['c1', 's1', 'n1', 'p1']);
  });

  it('calculates delta and yoy from year totals', () => {
    expect(salesPivotComparison({ previous: 100, current: 125 }, 'previous', 'current')).toEqual({
      previous: 100, current: 125, delta: 25, yoy: 25,
    });
    expect(salesPivotComparison({ current: 10 }, 'previous', 'current').yoy).toBeNull();
  });
});
