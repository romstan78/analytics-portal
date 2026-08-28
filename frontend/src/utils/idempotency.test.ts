import { describe, expect, it, vi, afterEach } from 'vitest';
import { newIdempotencyKey } from './idempotency';

const UUID_V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('newIdempotencyKey', () => {
  it('выдаёт UUID v4', () => {
    expect(newIdempotencyKey()).toMatch(UUID_V4);
  });

  it('не повторяется', () => {
    expect(newIdempotencyKey()).not.toBe(newIdempotencyKey());
  });

  // По HTTP без TLS окно не считается защищённым контекстом, и randomUUID в нём
  // нет: сервер обязан получить ключ того же формата, иначе он его отбросит.
  it('обходится без crypto.randomUUID', () => {
    vi.stubGlobal('crypto', {});
    expect(newIdempotencyKey()).toMatch(UUID_V4);
  });

  it('обходится без crypto вовсе', () => {
    vi.stubGlobal('crypto', undefined);
    expect(newIdempotencyKey()).toMatch(UUID_V4);
  });
});
