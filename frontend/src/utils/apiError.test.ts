import { describe, it, expect } from 'vitest';
import { apiErrorMessage, queryFailure } from './apiError';

describe('apiErrorMessage', () => {
  it('берёт текст сервера из ошибки parseJSONResponse', () => {
    const error = { status: 403, message: 'Учётная запись не привязана к КАМу' };
    expect(apiErrorMessage(error, 'Не удалось загрузить')).toBe('Учётная запись не привязана к КАМу');
  });

  it('берёт текст из Error', () => {
    expect(apiErrorMessage(new Error('HTTP 500'), 'Не удалось загрузить')).toBe('HTTP 500');
  });

  it('подставляет запасной текст, когда сервер ничего не объяснил', () => {
    expect(apiErrorMessage(new Error(''), 'Не удалось загрузить')).toBe('Не удалось загрузить');
    expect(apiErrorMessage({ status: 500 }, 'Не удалось загрузить')).toBe('Не удалось загрузить');
    expect(apiErrorMessage(null, 'Не удалось загрузить')).toBe('Не удалось загрузить');
  });
});

describe('queryFailure', () => {
  const denied = { status: 403, message: 'Учётная запись не привязана к КАМу' };

  it('отдаёт ошибку завершившегося запроса', () => {
    expect(queryFailure({ error: denied })).toBe(denied);
  });

  it('отдаёт причину запроса, ждущего повтора (paused)', () => {
    // status остаётся pending, isError — false, и без failureReason отказ
    // выглядел бы как пустая выдача.
    expect(queryFailure({ error: null, failureReason: denied, failureCount: 1 })).toBe(denied);
  });

  it('молчит, пока запрос ещё не падал', () => {
    expect(queryFailure({ error: null, failureReason: null, failureCount: 0 })).toBe(null);
    expect(queryFailure({})).toBe(null);
  });
});
