import { describe, expect, it } from 'vitest';
import type { NetworkOpexCell } from '../types/network';
import { draftDiffers } from './formDraft';
import {
  buildOpexDraft,
  hasOpexChanges,
  isOpexAmountValid,
  opexCellKey,
  opexCellsByKey,
  opexChangedRows,
  opexDraftSum,
  opexMonthsLabel,
  type OpexDraft,
} from './networkOpex';

// Раскладку по месяцам и базу «без НДС» считает backend; здесь проверяется то,
// что живёт на фронтенде: черновик, отбор изменённых ячеек и сложение введённого.

const BRANDS = ['Альфа', 'Бета'];
const ARTICLES = ['display', 'reports'];

const cell = (
  brand: string,
  article: string,
  quarter: number,
  amount: number,
  updatedAt = '2026-09-10 12:00:00.000',
): NetworkOpexCell => ({
  quarter,
  brand_as: brand,
  article,
  amount_rub: amount,
  amount_rub_net: Math.round((amount / 1.2) * 100) / 100,
  months: [
    { month: (quarter - 1) * 3 + 1, amount_rub: 33.33, amount_rub_net: 27.78 },
    { month: (quarter - 1) * 3 + 2, amount_rub: 33.33, amount_rub_net: 27.77 },
    { month: (quarter - 1) * 3 + 3, amount_rub: 33.35, amount_rub_net: 27.79 },
  ],
  updated_at: updatedAt,
});

describe('isOpexAmountValid', () => {
  it('принимает пустое значение: ячейка без бюджета — это не ошибка', () => {
    expect(isOpexAmountValid('')).toBe(true);
  });

  it('принимает отрицательную сумму: корректировка бюджета вводится со знаком', () => {
    expect(isOpexAmountValid('-120 000,50')).toBe(true);
  });

  it('отказывает значению мельче копейки', () => {
    expect(isOpexAmountValid('100,005')).toBe(false);
  });

  it('отказывает нечисловому вводу', () => {
    expect(isOpexAmountValid('сто')).toBe(false);
  });
});

describe('buildOpexDraft', () => {
  it('показывает сумму с разрядами', () => {
    const draft = buildOpexDraft([cell('Альфа', 'display', 1, 1200000)]);
    const value = draft[opexCellKey('Альфа', 'display', 1)];
    // Разделитель разрядов в ru-RU — неразрывный пробел, а не обычный.
    expect(value.replace(/[\u00a0\u202f]/g, ' ')).toBe('1 200 000');
  });

  it('не хранит версию ячейки: она принадлежит данным, а не человеку', () => {
    const draft = buildOpexDraft([cell('Альфа', 'display', 1, 100)]);
    expect(Object.values(draft)).toEqual(['100']);
  });
});

describe('opexChangedRows', () => {
  const saved = opexCellsByKey([cell('Альфа', 'display', 1, 100.01)]);

  it('не отправляет ячейки, которых не касались', () => {
    const draft = buildOpexDraft([cell('Альфа', 'display', 1, 100.01)]);
    expect(opexChangedRows(draft, saved, BRANDS, ARTICLES)).toEqual([]);
    expect(hasOpexChanges(draft, saved, BRANDS, ARTICLES)).toBe(false);
  });

  it('отправляет изменённую ячейку вместе с её версией', () => {
    const draft: OpexDraft = {
      ...buildOpexDraft([cell('Альфа', 'display', 1, 100.01)]),
      [opexCellKey('Альфа', 'display', 1)]: '250,55',
    };
    const rows = opexChangedRows(draft, saved, BRANDS, ARTICLES);
    expect(rows).toEqual([{
      quarter: 1,
      brand_as: 'Альфа',
      article: 'display',
      amount_rub: 250.55,
      updated_at: '2026-09-10 12:00:00.000',
    }]);
  });

  it('очищенная ячейка уходит с null: это снятие бюджета, а не ноль', () => {
    const draft: OpexDraft = {
      [opexCellKey('Альфа', 'display', 1)]: '',
    };
    const rows = opexChangedRows(draft, saved, BRANDS, ARTICLES);
    expect(rows).toHaveLength(1);
    expect(rows[0].amount_rub).toBeNull();
  });

  it('нулевая сумма отличается от снятой: её отправляют нулём', () => {
    const draft: OpexDraft = {
      [opexCellKey('Альфа', 'display', 1)]: '0',
    };
    expect(opexChangedRows(draft, saved, BRANDS, ARTICLES)[0].amount_rub).toBe(0);
  });

  it('новая ячейка уходит с пустой версией', () => {
    const draft: OpexDraft = {
      ...buildOpexDraft([cell('Альфа', 'display', 1, 100.01)]),
      [opexCellKey('Бета', 'reports', 3)]: '500',
    };
    const rows = opexChangedRows(draft, saved, BRANDS, ARTICLES);
    expect(rows).toEqual([{
      quarter: 3, brand_as: 'Бета', article: 'reports', amount_rub: 500, updated_at: '',
    }]);
  });
});

describe('opexDraftSum', () => {
  const draft: OpexDraft = {
    [opexCellKey('Альфа', 'display', 1)]: '100,01',
    [opexCellKey('Альфа', 'reports', 1)]: '50',
    [opexCellKey('Бета', 'display', 2)]: '-30,5',
  };

  it('складывает квартал по всем брендам и статьям', () => {
    expect(opexDraftSum(draft, BRANDS, ARTICLES, { quarter: 1 })).toBe(150.01);
  });

  it('считает итог бренда за год', () => {
    expect(opexDraftSum(draft, BRANDS, ARTICLES, { brand: 'Альфа' })).toBe(150.01);
  });

  it('учитывает знак в общем итоге', () => {
    expect(opexDraftSum(draft, BRANDS, ARTICLES)).toBe(119.51);
  });

  it('не считает бренды и статьи вне сетки', () => {
    expect(opexDraftSum(draft, ['Бета'], ['display'])).toBe(-30.5);
  });
});

describe('opexMonthsLabel', () => {
  it('перечисляет месяцы квартала как их сохранил сервер', () => {
    expect(opexMonthsLabel(cell('Альфа', 'display', 1, 100.01))).toBe('33,33 / 33,33 / 33,35');
  });

  it('пустая подпись, если ячейки ещё нет', () => {
    expect(opexMonthsLabel(undefined)).toBe('');
  });
});

// Черновик уезжает в localStorage как есть и сравнивается с серверными данными
// тем же draftDiffers, что и в остальных формах. Проверяется именно круг:
// значения форматированы неразрывными пробелами, и после JSON они обязаны
// сойтись — иначе вкладка предлагала бы восстановление после каждой загрузки.
describe('черновик для localStorage', () => {
  const cells = [cell('Альфа', 'display', 1, 1200000), cell('Бета', 'reports', 2, -500.25)];

  it('совпавший с сервером черновик восстанавливать не предлагают', () => {
    const stored = JSON.parse(JSON.stringify(buildOpexDraft(cells))) as OpexDraft;
    expect(draftDiffers(stored, buildOpexDraft(cells))).toBe(false);
  });

  it('изменённый черновик отличается от серверного', () => {
    const stored = { ...buildOpexDraft(cells), [opexCellKey('Альфа', 'display', 1)]: '999' };
    expect(draftDiffers(stored, buildOpexDraft(cells))).toBe(true);
  });

  it('восстановленный черновик даёт те же изменённые строки', () => {
    const saved = opexCellsByKey(cells);
    const stored = { ...buildOpexDraft(cells), [opexCellKey('Бета', 'reports', 2)]: '-600' };
    const rows = opexChangedRows(stored, saved, BRANDS, ARTICLES);
    expect(rows).toEqual([{
      quarter: 2,
      brand_as: 'Бета',
      article: 'reports',
      amount_rub: -600,
      // Версия берётся из свежих данных, а не из черновика.
      updated_at: '2026-09-10 12:00:00.000',
    }]);
  });
});
