// Ключ идемпотентности для создания записи.
//
// Ответ на «Сохранить» может не дойти из-за сети, и повторное нажатие
// создавало вторую такую же запись. Ключ выдаётся на открытие формы и
// повторяется при каждой попытке сохранить её: сервер по нему возвращает
// прежний результат вместо второй вставки.

/**
 * Случайный UUID v4. crypto.randomUUID есть не везде — по HTTP без TLS
 * (кроме localhost) окно не считается защищённым контекстом, и метода в нём
 * нет. Запасной вариант собирает тот же формат из crypto.getRandomValues, а
 * при отсутствии и его — из Math.random: ключ нужен только чтобы отличить
 * повтор от нового сохранения, и предсказуемость на это не влияет.
 */
export function newIdempotencyKey(): string {
  const cryptoApi = globalThis.crypto as Crypto | undefined;
  if (typeof cryptoApi?.randomUUID === 'function') {
    return cryptoApi.randomUUID();
  }

  const bytes = new Uint8Array(16);
  if (typeof cryptoApi?.getRandomValues === 'function') {
    cryptoApi.getRandomValues(bytes);
  } else {
    for (let i = 0; i < bytes.length; i += 1) bytes[i] = Math.floor(Math.random() * 256);
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40; // версия 4
  bytes[8] = (bytes[8] & 0x3f) | 0x80; // вариант RFC 4122

  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
