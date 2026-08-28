// Черновик формы: заполненное не должно исчезать вместе со вкладкой.
//
// Истёкший refresh-токен уводит на /login прямо из fetchWithAuth, и введённое
// в карточку живёт только в памяти вкладки — заполнив два десятка полей, КАМ
// начинал заново. Возврат на прерванный раздел уже сделан (returnPath.ts),
// здесь — сами значения.
//
// Хранилище — localStorage, а не sessionStorage: sessionStorage не переживает
// закрытую вкладку, а черновик обязан дождаться следующего входа. Каждое
// обращение в try/catch: в приватном режиме и при запрете на хранилище оно
// бросает исключение, и форма из-за этого ломаться не должна.

import { userScopedKey } from './storage';

// Срок жизни черновика. Через неделю введённое устарело настолько, что
// подставлять его опаснее, чем потерять: цены, планы и статусы уже другие.
export const DRAFT_TTL_MS = 7 * 24 * 60 * 60 * 1000;

// Что лежит в хранилище. Имя пользователя дублирует привязку ключа: ключ
// строится на открытии формы, а прочитан может быть уже после смены
// пользователя в той же вкладке — чужой черновик показывать нельзя.
interface DraftEnvelope<T> {
  username: string | null;
  savedAt: number;
  values: T;
}

export interface SavedDraft<T> {
  savedAt: number;
  values: T;
}

/**
 * Ключ черновика: база, идентификатор записи и пользователь. Без
 * идентификатора черновик новой карточки и черновик промо №994 писались бы
 * друг поверх друга.
 */
export function draftStorageKey(base: string, recordId: number | string | null): string {
  return userScopedKey(`${base}:${recordId ?? 'new'}`);
}

export function encodeDraft<T>(values: T, username: string | null, savedAt: number): string {
  return JSON.stringify({ username, savedAt, values } satisfies DraftEnvelope<T>);
}

/**
 * Разбирает сохранённое. Возвращает null для чужого, просроченного и
 * испорченного черновика — во всех этих случаях предлагать нечего.
 */
export function parseDraft<T>(
  raw: string | null,
  username: string | null,
  now: number,
  ttlMs: number = DRAFT_TTL_MS,
): SavedDraft<T> | null {
  if (!raw) return null;
  let envelope: DraftEnvelope<T>;
  try {
    envelope = JSON.parse(raw) as DraftEnvelope<T>;
  } catch {
    return null;
  }
  if (envelope === null || typeof envelope !== 'object') return null;
  if (envelope.username !== username) return null;
  if (typeof envelope.savedAt !== 'number' || !Number.isFinite(envelope.savedAt)) return null;
  if (now - envelope.savedAt > ttlMs) return null;
  if (envelope.values === null || typeof envelope.values !== 'object') return null;
  return { savedAt: envelope.savedAt, values: envelope.values };
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

// Пустое поле приходит то пустой строкой, то null, а число — то строкой, то
// числом (JSON типы сохраняет, поля ввода отдают строки). Для пользователя это
// одно и то же значение, и разницей между ними предлагать восстановление
// нельзя.
const normalize = (value: unknown): string => (value == null ? '' : String(value));

function sameValues(a: unknown, b: unknown): boolean {
  if (a === b) return true;
  if (Array.isArray(a) || Array.isArray(b)) {
    if (!Array.isArray(a) || !Array.isArray(b) || a.length !== b.length) return false;
    return a.every((item, index) => sameValues(item, b[index]));
  }
  if (isRecord(a) && isRecord(b)) {
    const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
    for (const key of keys) {
      if (!sameValues(a[key], b[key])) return false;
    }
    return true;
  }
  return normalize(a) === normalize(b);
}

/** Отличается ли черновик от того, что пришло с сервера. */
export function draftDiffers(draft: unknown, baseline: unknown): boolean {
  return !sameValues(draft, baseline);
}

/** Когда черновик сохранён — пользователь должен понимать, откуда значения. */
export function draftSavedAtLabel(savedAt: number): string {
  if (!Number.isFinite(savedAt)) return '';
  return new Date(savedAt).toLocaleString('ru-RU', { dateStyle: 'short', timeStyle: 'short' });
}

/**
 * Читает черновик. Негодный (просроченный, чужой, испорченный) сразу убирает:
 * предложен он уже не будет, а место занимает.
 */
export function readDraft<T>(
  key: string | null,
  username: string | null,
  now: number = Date.now(),
): SavedDraft<T> | null {
  if (!key) return null;
  try {
    const raw = localStorage.getItem(key);
    const draft = parseDraft<T>(raw, username, now);
    if (!draft && raw !== null) localStorage.removeItem(key);
    return draft;
  } catch {
    // Хранилище недоступно — форма просто работает без черновика.
    return null;
  }
}

export function writeDraft<T>(
  key: string | null,
  values: T,
  username: string | null,
  savedAt: number = Date.now(),
): void {
  if (!key) return;
  try {
    localStorage.setItem(key, encodeDraft(values, username, savedAt));
  } catch {
    // см. readDraft: приватный режим и переполненное хранилище тоже бросают.
  }
}

export function removeDraft(key: string | null): void {
  if (!key) return;
  try {
    localStorage.removeItem(key);
  } catch {
    // см. readDraft
  }
}
