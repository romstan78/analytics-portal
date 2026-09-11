// Вкладка «Инвестиции OPEX» реестра сетей: черновик ввода и сборка запроса.
//
// Расчётов здесь нет. Раскладку квартальной суммы по месяцам и базу «без НДС»
// считает backend (backend/services/network_opex_service.go) — и при записи, и
// при чтении. Итоговые строки складывают ровно то, что человек набрал в полях:
// это не вторая копия формулы, а тот же ввод, показанный суммой.

import type { NetworkOpexCell, NetworkOpexInput } from '../types/network';
import { formatNumberInput, parseNumberInput, round2 } from './networkPlan';

export const OPEX_QUARTERS = [1, 2, 3, 4] as const;

// Ключ ячейки ввода: бренд, статья, квартал.
export const opexCellKey = (brand: string, article: string, quarter: number): string =>
  `${brand}|${article}|${quarter}`;

// Черновик — только введённые строки, без версий ячеек.
//
// Версия (updated_at) в черновик не попадает намеренно: она принадлежит не
// человеку, а загруженным данным, и берётся из них в момент сохранения. Иначе
// восстановленный через сутки черновик нёс бы устаревшую версию и получал 409
// даже там, где ячейку никто не трогал. Это же делает черновик пригодным для
// localStorage как есть — в нём нет ничего, кроме набранного.
export type OpexDraft = Record<string, string>;

// Черновик из пришедших ячеек. Суммы сразу с разрядами: «1 200 000» читается,
// «1200000» приходится считать.
export function buildOpexDraft(cells: NetworkOpexCell[]): OpexDraft {
  const draft: OpexDraft = {};
  cells.forEach((cell) => {
    draft[opexCellKey(cell.brand_as, cell.article, cell.quarter)] = formatNumberInput(String(cell.amount_rub));
  });
  return draft;
}

// Ячейки по ключу: подсказка показывает месяцы и базу «без НДС» из сохранённого,
// а сохранение берёт оттуда версию ячейки.
export function opexCellsByKey(cells: NetworkOpexCell[]): Record<string, NetworkOpexCell> {
  const byKey: Record<string, NetworkOpexCell> = {};
  cells.forEach((cell) => {
    byKey[opexCellKey(cell.brand_as, cell.article, cell.quarter)] = cell;
  });
  return byKey;
}

// Значение ячейки как число: пустая строка — значение снято, а не ноль.
// Согласованный нулевой бюджет — тоже решение, и отличать его обязательно.
export function opexCellAmount(value: string | undefined): number | null {
  if (value == null) return null;
  return parseNumberInput(value);
}

// Ввод допустим, если это число до сотых. Минус разрешён намеренно: возврат и
// корректировка бюджета вводятся со знаком.
export function isOpexAmountValid(value: string): boolean {
  const trimmed = value.trim();
  if (trimmed === '') return true;
  const parsed = parseNumberInput(trimmed);
  if (parsed == null) return false;
  return Math.abs(parsed * 100 - Math.round(parsed * 100)) < 1e-9;
}

// Ячейки, расходящиеся с сохранённым, — только они и уходят в запрос.
// Отправлять всю сетку значило бы натыкаться на конфликты в строках, которых
// пользователь не касался.
export function opexChangedRows(
  draft: OpexDraft,
  saved: Record<string, NetworkOpexCell>,
  brands: string[],
  articles: string[],
): NetworkOpexInput[] {
  const rows: NetworkOpexInput[] = [];
  brands.forEach((brand) => {
    articles.forEach((article) => {
      OPEX_QUARTERS.forEach((quarter) => {
        const key = opexCellKey(brand, article, quarter);
        const amount = opexCellAmount(draft[key]);
        const savedCell = saved[key];
        const savedAmount = savedCell ? savedCell.amount_rub : null;
        if (amount === savedAmount) return;
        rows.push({
          quarter,
          brand_as: brand,
          article,
          amount_rub: amount,
          updated_at: savedCell?.updated_at ?? '',
        });
      });
    });
  });
  return rows;
}

export function hasOpexChanges(
  draft: OpexDraft,
  saved: Record<string, NetworkOpexCell>,
  brands: string[],
  articles: string[],
): boolean {
  return opexChangedRows(draft, saved, brands, articles).length > 0;
}

// Сумма введённого по области: квартал, бренд и статья необязательны.
export function opexDraftSum(
  draft: OpexDraft,
  brands: string[],
  articles: string[],
  scope: { quarter?: number; brand?: string; article?: string } = {},
): number {
  let total = 0;
  brands.forEach((brand) => {
    if (scope.brand != null && brand !== scope.brand) return;
    articles.forEach((article) => {
      if (scope.article != null && article !== scope.article) return;
      OPEX_QUARTERS.forEach((quarter) => {
        if (scope.quarter != null && quarter !== scope.quarter) return;
        total = round2(total + (opexCellAmount(draft[opexCellKey(brand, article, quarter)]) ?? 0));
      });
    });
  });
  return total;
}

// Подпись месяцев ячейки для подсказки: «33,33 / 33,33 / 33,35».
export function opexMonthsLabel(cell: NetworkOpexCell | undefined): string {
  if (!cell || cell.months.length === 0) return '';
  return cell.months
    .map((month) => month.amount_rub.toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 }))
    .join(' / ');
}
