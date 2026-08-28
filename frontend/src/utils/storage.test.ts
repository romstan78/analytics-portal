import { describe, expect, it, beforeEach, vi } from 'vitest';
import { userScopedKey } from './storage';

function fakeStorage() {
  const data = new Map<string, string>();
  return {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => void data.set(key, value),
    removeItem: (key: string) => void data.delete(key),
    clear: () => data.clear(),
  };
}

describe('userScopedKey', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', fakeStorage());
  });

  // Ровно та ситуация, ради которой ключ и скоупится: в одной вкладке сменился
  // пользователь, и состояние страницы прежнего не должно достаться новому.
  it('разводит состояние разных пользователей', () => {
    localStorage.setItem('username', 'demo_admin');
    const admin = userScopedKey('promo_filters_v20');

    localStorage.setItem('username', 'kam.orlov.dmitriy');
    const kam = userScopedKey('promo_filters_v20');

    expect(admin).not.toBe(kam);
    expect(kam).toBe('promo_filters_v20:kam.orlov.dmitriy');
  });

  it('возвращается к тому же ключу, когда пользователь вернулся', () => {
    localStorage.setItem('username', 'demo_admin');
    const first = userScopedKey('internet_sales_filters_v9');
    localStorage.setItem('username', 'kam.orlov.dmitriy');
    localStorage.setItem('username', 'demo_admin');

    expect(userScopedKey('internet_sales_filters_v9')).toBe(first);
  });

  // До входа пользователя ещё нет: ключ обязан остаться валидным, а не стать
  // «...:null», иначе сохранённое до логина потерялось бы молча.
  it('без пользователя использует общий суффикс', () => {
    expect(userScopedKey('promo_filters_v20')).toBe('promo_filters_v20:local');
  });
});
