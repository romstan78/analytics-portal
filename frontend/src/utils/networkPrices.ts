import type {
  NetworkContractPrice,
  NetworkContractPriceDeleteInput,
  NetworkContractPriceInput,
} from '../types/network';
import { formatNumberInput, parseNumberInput } from './networkPlan';

export type PriceQuarterValues<T> = [T, T, T, T];

export interface QuarterlyPriceDraft {
  key: string;
  brand: string;
  sku: string;
  prices: PriceQuarterValues<string>;
  periodIds: PriceQuarterValues<number>;
  updatedAts: PriceQuarterValues<string>;
  confirmed: boolean;
  olapPrice: number | null;
  olapYear: number | null;
  olapMonth: number | null;
  manualNew: boolean;
  canDelete: boolean;
  deleteRows: NetworkContractPriceDeleteInput[];
}

const quarterBounds = (year: number, quarter: number): { from: string; to: string } => {
  const bounds = [
    [`${year}-01-01`, `${year}-03-31`],
    [`${year}-04-01`, `${year}-06-30`],
    [`${year}-07-01`, `${year}-09-30`],
    [`${year}-10-01`, `${year}-12-31`],
  ] as const;
  const [from, to] = bounds[quarter - 1];
  return { from, to };
};

const overlapsQuarter = (row: NetworkContractPrice, year: number, quarter: number): boolean => {
  const { from, to } = quarterBounds(year, quarter);
  return row.valid_from <= to && row.valid_to >= from;
};

const skuKey = (sku: string): string => sku.trim().toLocaleLowerCase('ru-RU');

const emptyStrings = (): PriceQuarterValues<string> => ['', '', '', ''];
const emptyNumbers = (): PriceQuarterValues<number> => [0, 0, 0, 0];

// API продолжает хранить непересекающиеся периоды, а интерфейс показывает
// один SKU строкой. Если исторически внутри квартала было несколько периодов,
// в ячейке показывается самый поздний из них; при сохранении год нормализуется
// до четырёх квартальных периодов.
export const buildQuarterlyPriceDrafts = (
  rows: NetworkContractPrice[],
  year: number,
): QuarterlyPriceDraft[] => {
  const grouped = new Map<string, NetworkContractPrice[]>();
  for (const row of rows) {
    const key = skuKey(row.sku);
    grouped.set(key, [...(grouped.get(key) ?? []), row]);
  }

  const result: QuarterlyPriceDraft[] = [];
  for (const [key, skuRows] of grouped) {
    const sorted = [...skuRows].sort((a, b) => a.valid_from.localeCompare(b.valid_from));
    const selected = [1, 2, 3, 4].map((quarter) => {
      const candidates = sorted.filter((row) => overlapsQuarter(row, year, quarter));
      return candidates.at(-1) ?? null;
    });
    const representative = selected.find((row) => row != null) ?? sorted[0];
    const olap = sorted.find((row) => row.olap_price != null) ?? representative;
    const deleteRows = [...new Map(
      sorted
        .filter((row) => row.id > 0)
        .map((row) => [row.id, { id: row.id, updated_at: row.updated_at }]),
    ).values()];

    result.push({
      key: `sku-${key}`,
      brand: representative.brand_as,
      sku: representative.sku,
      prices: selected.map((row) => row == null ? '' : formatNumberInput(String(row.contract_price))) as PriceQuarterValues<string>,
      periodIds: selected.map((row) => row?.id ?? 0) as PriceQuarterValues<number>,
      updatedAts: selected.map((row) => row?.updated_at ?? '') as PriceQuarterValues<string>,
      confirmed: selected.every((row) => row != null && row.is_confirmed),
      olapPrice: olap.olap_price,
      olapYear: olap.olap_year,
      olapMonth: olap.olap_month,
      manualNew: false,
      canDelete: deleteRows.length > 0
        && olap.olap_price == null
        && sorted.every((row) => row.source_type === 'manual'),
      deleteRows,
    });
  }

  return result.sort((a, b) => a.brand.localeCompare(b.brand, 'ru') || a.sku.localeCompare(b.sku, 'ru'));
};

export const createEmptyQuarterlyPriceDraft = (key: string): QuarterlyPriceDraft => ({
  key,
  brand: '',
  sku: '',
  prices: emptyStrings(),
  periodIds: emptyNumbers(),
  updatedAts: emptyStrings(),
  confirmed: true,
  olapPrice: null,
  olapYear: null,
  olapMonth: null,
  manualNew: true,
  canDelete: true,
  deleteRows: [],
});

// Один физический период может заполнить сразу четыре ячейки (годовая цена),
// но его ID можно обновить только один раз. Поэтому первая четверть сохраняет
// ID и optimistic-lock версию, остальные становятся новыми периодами.
export const quarterlyPriceInputs = (
  drafts: QuarterlyPriceDraft[],
  year: number,
): NetworkContractPriceInput[] => {
  const rows: NetworkContractPriceInput[] = [];
  for (const draft of drafts) {
    const usedIDs = new Set<number>();
    for (let index = 0; index < 4; index += 1) {
      const sourceID = draft.periodIds[index];
      const reuseID = sourceID > 0 && !usedIDs.has(sourceID) ? sourceID : 0;
      if (reuseID > 0) usedIDs.add(reuseID);
      const { from, to } = quarterBounds(year, index + 1);
      rows.push({
        id: reuseID,
        brand_as: draft.brand.trim(),
        sku: draft.sku.trim(),
        contract_price: parseNumberInput(draft.prices[index]) ?? 0,
        valid_from: from,
        valid_to: to,
        is_confirmed: draft.confirmed,
        updated_at: reuseID > 0 ? draft.updatedAts[index] : '',
      });
    }
  }
  return rows;
};
