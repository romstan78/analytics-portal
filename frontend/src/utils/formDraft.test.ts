import { describe, expect, it, beforeEach, vi } from 'vitest';
import {
  DRAFT_TTL_MS,
  draftDiffers,
  draftStorageKey,
  encodeDraft,
  parseDraft,
  readDraft,
  removeDraft,
  writeDraft,
} from './formDraft';

function fakeStorage() {
  const data = new Map<string, string>();
  return {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => void data.set(key, value),
    removeItem: (key: string) => void data.delete(key),
    clear: () => data.clear(),
  };
}

// Приватный режим и запрет на хранилище: любое обращение бросает исключение.
function brokenStorage() {
  const fail = () => { throw new DOMException('storage disabled', 'SecurityError'); };
  return { getItem: fail, setItem: fail, removeItem: fail, clear: fail };
}

const NOW = Date.UTC(2026, 7, 29, 12, 0, 0);

describe('draftStorageKey', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', fakeStorage());
    localStorage.setItem('username', 'kam.ershov');
  });

  // Черновик новой карточки не должен лечь поверх черновика существующей.
  it('разводит записи по идентификатору', () => {
    expect(draftStorageKey('promo_card_draft_v1', 994)).toBe('promo_card_draft_v1:994:kam.ershov');
    expect(draftStorageKey('promo_card_draft_v1', null)).toBe('promo_card_draft_v1:new:kam.ershov');
  });

  it('разводит записи по пользователю', () => {
    const first = draftStorageKey('promo_card_draft_v1', 994);
    localStorage.setItem('username', 'demo_admin');
    expect(draftStorageKey('promo_card_draft_v1', 994)).not.toBe(first);
  });
});

describe('parseDraft', () => {
  it('возвращает значения и время сохранения', () => {
    const raw = encodeDraft({ sku: 'SKU-1' }, 'kam.ershov', NOW - 1000);
    expect(parseDraft(raw, 'kam.ershov', NOW)).toEqual({
      savedAt: NOW - 1000,
      values: { sku: 'SKU-1' },
    });
  });

  // В той же вкладке мог войти другой человек: ключ уже другой, но проверка
  // дублируется — чужие значения не должны всплыть ни при каких обстоятельствах.
  it('не отдаёт черновик другого пользователя', () => {
    const raw = encodeDraft({ sku: 'SKU-1' }, 'kam.ershov', NOW);
    expect(parseDraft(raw, 'demo_admin', NOW)).toBe(null);
  });

  it('молча выбрасывает просроченный черновик', () => {
    const raw = encodeDraft({ sku: 'SKU-1' }, 'kam.ershov', NOW - DRAFT_TTL_MS - 1);
    expect(parseDraft(raw, 'kam.ershov', NOW)).toBe(null);
  });

  it('держит черновик до конца срока', () => {
    const raw = encodeDraft({ sku: 'SKU-1' }, 'kam.ershov', NOW - DRAFT_TTL_MS + 1000);
    expect(parseDraft(raw, 'kam.ershov', NOW)).not.toBe(null);
  });

  it('переживает пустое и испорченное хранилище', () => {
    expect(parseDraft(null, 'kam.ershov', NOW)).toBe(null);
    expect(parseDraft('не json', 'kam.ershov', NOW)).toBe(null);
    expect(parseDraft('"строка"', 'kam.ershov', NOW)).toBe(null);
    expect(parseDraft(JSON.stringify({ username: 'kam.ershov' }), 'kam.ershov', NOW)).toBe(null);
  });
});

describe('draftDiffers', () => {
  it('не видит различий там, где их нет для пользователя', () => {
    // Поля ввода отдают строки, а с сервера число приходит числом; пустое поле —
    // то '', то null. Предлагать восстановление из-за этого нельзя.
    expect(draftDiffers({ year: '2026', sku: null }, { year: 2026, sku: '' })).toBe(false);
  });

  it('замечает введённое пользователем', () => {
    expect(draftDiffers({ baseline_units: '120' }, { baseline_units: '100' })).toBe(true);
  });

  it('сравнивает вложенные значения сетки', () => {
    const grid = { brands: ['Бренд А'], cells: { '1|Бренд А': { planRub: '100' } } };
    expect(draftDiffers(grid, { brands: ['Бренд А'], cells: { '1|Бренд А': { planRub: '100' } } })).toBe(false);
    expect(draftDiffers(grid, { brands: ['Бренд А'], cells: { '1|Бренд А': { planRub: '200' } } })).toBe(true);
    expect(draftDiffers(grid, { brands: [], cells: { '1|Бренд А': { planRub: '100' } } })).toBe(true);
  });
});

describe('чтение и запись', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', fakeStorage());
  });

  it('возвращает записанное', () => {
    writeDraft('draft:1', { sku: 'SKU-1' }, 'kam.ershov', NOW);
    expect(readDraft('draft:1', 'kam.ershov', NOW)).toEqual({ savedAt: NOW, values: { sku: 'SKU-1' } });
  });

  it('убирает негодный черновик из хранилища', () => {
    writeDraft('draft:1', { sku: 'SKU-1' }, 'kam.ershov', NOW - DRAFT_TTL_MS - 1);
    expect(readDraft('draft:1', 'kam.ershov', NOW)).toBe(null);
    expect(localStorage.getItem('draft:1')).toBe(null);
  });

  it('без ключа ничего не делает', () => {
    writeDraft(null, { sku: 'SKU-1' }, 'kam.ershov', NOW);
    expect(readDraft(null, 'kam.ershov', NOW)).toBe(null);
  });

  // Отсутствие хранилища не должно ломать форму: без черновика она работает.
  it('переживает недоступное хранилище', () => {
    vi.stubGlobal('localStorage', brokenStorage());
    expect(() => writeDraft('draft:1', { sku: 'SKU-1' }, 'kam.ershov', NOW)).not.toThrow();
    expect(() => removeDraft('draft:1')).not.toThrow();
    expect(readDraft('draft:1', 'kam.ershov', NOW)).toBe(null);
  });
});
