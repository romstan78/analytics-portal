import type { SalesPivotNode } from '../types/sales';

export interface FlatSalesPivotRow {
  node: SalesPivotNode;
  depth: number;
}

export function filterSalesPivotTree(nodes: SalesPivotNode[], search: string): SalesPivotNode[] {
  const query = search.trim().toLocaleLowerCase('ru-RU');
  if (!query) return nodes;
  const filter = (items: SalesPivotNode[]): SalesPivotNode[] => items.flatMap(node => {
    const children = filter(node.children || []);
    if (node.name.toLocaleLowerCase('ru-RU').includes(query)) return [node];
    return children.length ? [{ ...node, children }] : [];
  });
  return filter(nodes);
}

export function flattenSalesPivotTree(nodes: SalesPivotNode[], expanded: Set<string>, forceExpanded = false): FlatSalesPivotRow[] {
  const result: FlatSalesPivotRow[] = [];
  const visit = (items: SalesPivotNode[], depth: number) => {
    items.forEach(node => {
      result.push({ node, depth });
      if ((forceExpanded || expanded.has(node.id)) && node.children?.length) visit(node.children, depth + 1);
    });
  };
  visit(nodes, 0);
  return result;
}

export function defaultSalesPivotExpansion(nodes: SalesPivotNode[]): Set<string> {
  const result = new Set<string>();
  const visit = (items: SalesPivotNode[]) => items.forEach(node => {
    if (node.level === 'channel' || node.level === 'segment') result.add(node.id);
    visit(node.children || []);
  });
  visit(nodes);
  return result;
}

export function allSalesPivotExpansion(nodes: SalesPivotNode[]): Set<string> {
  const result = new Set<string>();
  const visit = (items: SalesPivotNode[]) => items.forEach(node => {
    if (node.children?.length) result.add(node.id);
    visit(node.children || []);
  });
  visit(nodes);
  return result;
}

export function salesPivotComparison(values: Record<string, number>, previousKey: string, currentKey: string) {
  const previous = values[previousKey] || 0;
  const current = values[currentKey] || 0;
  const delta = current - previous;
  return { previous, current, delta, yoy: previous === 0 ? null : delta / previous * 100 };
}
