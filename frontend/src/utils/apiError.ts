// Текст ошибки от API. React Query отдаёт причину как unknown, а бросают её
// два разных места: parseJSONResponse — объектом { status, message } с текстом
// поля error, хуки — обычным Error. Отказ по области видимости объясняет себя
// сам («учётная запись не привязана к КАМу»), и подменять его общим «не удалось
// загрузить» значит возвращать тот самый молчаливый пустой экран.
export interface ApiErrorLike {
  status?: number;
  message?: string;
}

export function apiErrorMessage(error: unknown, fallback: string): string {
  if (typeof error === 'object' && error !== null) {
    const { message } = error as ApiErrorLike;
    if (typeof message === 'string' && message.trim() !== '') {
      return message;
    }
  }
  return fallback;
}

/**
 * Причина неудачи запроса React Query — с учётом состояния `paused`.
 *
 * После первой неудачи запрос уходит в повтор, и пока повтор не состоялся,
 * `status` остаётся `pending`, а `isError` — false: ошибка лежит только в
 * `failureReason`. Смотреть на один `error` значит показывать пустой экран
 * ровно там, где сервер объяснил отказ.
 */
export function queryFailure(query: {
  error?: unknown;
  failureReason?: unknown;
  failureCount?: number;
}): unknown {
  if (query.error != null) {
    return query.error;
  }
  return (query.failureCount ?? 0) > 0 ? query.failureReason ?? null : null;
}
