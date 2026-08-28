// Куда вернуть пользователя после вынужденного входа.
//
// Истёкший refresh-токен уводит на /login прямо из fetchWithAuth, и вошедший
// заново оказывался на главной, потеряв открытый раздел. Путь переживает
// перезагрузку в sessionStorage: она привязана к вкладке, а перезагрузка здесь
// полная (window.location.replace), и состояние React до формы входа не
// доживает.

const STORAGE_KEY = 'auth:return_to';

export interface ReturnTarget {
  username: string | null;
  path: string;
}

/**
 * Путь, на который можно вернуться. Значение приходит из адресной строки,
 * поэтому наружу его выпускать нельзя: «//example.com» и «/\example.com»
 * браузер считает чужим origin. Возврат на саму форму входа и на главную
 * смысла не имеет — это и есть поведение по умолчанию.
 */
export function safeReturnPath(raw: unknown): string | null {
  if (typeof raw !== 'string' || raw === '') return null;
  if (!raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/\\')) return null;
  if (raw === '/' || raw === '/login' || raw.startsWith('/login?')) return null;
  return raw;
}

/**
 * Разбирает сохранённую отметку. Путь возвращается только тому же
 * пользователю: если в той же вкладке вошёл другой человек, он должен начать
 * с главной, а не с чужого раздела.
 */
export function parseReturnTarget(raw: string | null, username: string | null): string | null {
  if (!raw) return null;
  let target: ReturnTarget;
  try {
    target = JSON.parse(raw) as ReturnTarget;
  } catch {
    return null;
  }
  if (target?.username !== username) return null;
  return safeReturnPath(target?.path);
}

/**
 * Записывать ли отметку. Истёкшая сессия роняет не один запрос, а сразу все
 * запросы открытой страницы, и logout() стирает имя пользователя после первого
 * из них: последующие пришли бы уже с `username: null` и затёрли бы годную
 * отметку. Поэтому побеждает первая — только у неё имя ещё есть.
 */
export function shouldRemember(existing: string | null, path: string): boolean {
  return existing === null && safeReturnPath(path) !== null;
}

export function rememberReturnPath(path: string, username: string | null): void {
  try {
    if (!shouldRemember(sessionStorage.getItem(STORAGE_KEY), path)) return;
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify({ username, path } satisfies ReturnTarget));
  } catch {
    // Приватный режим и запрет на хранилище не должны ломать разлогин.
  }
}

/** Отдаёт сохранённый путь и сразу забывает его: возврат одноразовый. */
export function takeReturnPath(username: string | null): string | null {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    sessionStorage.removeItem(STORAGE_KEY);
    return parseReturnTarget(raw, username);
  } catch {
    return null;
  }
}

/** Осознанный выход отменяет возврат: следующий вход начинается с главной. */
export function forgetReturnPath(): void {
  try {
    sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    // см. rememberReturnPath
  }
}
