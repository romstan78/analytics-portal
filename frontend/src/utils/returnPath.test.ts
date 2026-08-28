import { describe, it, expect } from 'vitest';
import { safeReturnPath, parseReturnTarget, shouldRemember } from './returnPath';

describe('safeReturnPath', () => {
  it('пропускает внутренний путь с параметрами', () => {
    expect(safeReturnPath('/network-registry?year=2026')).toBe('/network-registry?year=2026');
  });

  it('не выпускает наружу чужой origin', () => {
    // Браузер понимает «//example.com» и «/\example.com» как внешний адрес,
    // поэтому такой возврат стал бы открытым редиректом.
    expect(safeReturnPath('//example.com')).toBe(null);
    expect(safeReturnPath('/\\example.com')).toBe(null);
    expect(safeReturnPath('https://example.com')).toBe(null);
  });

  it('не возвращает на форму входа и на главную', () => {
    expect(safeReturnPath('/login')).toBe(null);
    expect(safeReturnPath('/login?next=/promo-analysis')).toBe(null);
    expect(safeReturnPath('/')).toBe(null);
  });

  it('отсекает пустое и нестроковое', () => {
    expect(safeReturnPath('')).toBe(null);
    expect(safeReturnPath(null)).toBe(null);
    expect(safeReturnPath(42)).toBe(null);
  });
});

describe('parseReturnTarget', () => {
  const stored = JSON.stringify({ username: 'kam.ershov', path: '/promo-analysis' });

  it('возвращает путь тому же пользователю', () => {
    expect(parseReturnTarget(stored, 'kam.ershov')).toBe('/promo-analysis');
  });

  it('не отдаёт путь другому пользователю той же вкладки', () => {
    expect(parseReturnTarget(stored, 'demo_admin')).toBe(null);
  });

  it('переживает пустое и испорченное хранилище', () => {
    expect(parseReturnTarget(null, 'kam.ershov')).toBe(null);
    expect(parseReturnTarget('не json', 'kam.ershov')).toBe(null);
  });

  it('проверяет сам путь, а не только пользователя', () => {
    const evil = JSON.stringify({ username: 'kam.ershov', path: '//example.com' });
    expect(parseReturnTarget(evil, 'kam.ershov')).toBe(null);
  });
});

describe('shouldRemember', () => {
  const stored = JSON.stringify({ username: 'kam.ershov', path: '/promo-analysis' });

  it('запоминает первую отметку', () => {
    expect(shouldRemember(null, '/network-registry')).toBe(true);
  });

  it('не затирает отметку следующими упавшими запросами', () => {
    // К этому моменту logout() уже стёр имя пользователя, и вторая отметка
    // легла бы с username: null — возврат после входа не состоялся бы.
    expect(shouldRemember(stored, '/network-registry')).toBe(false);
  });

  it('не запоминает путь, на который всё равно не вернёт', () => {
    expect(shouldRemember(null, '/login')).toBe(false);
    expect(shouldRemember(null, '//example.com')).toBe(false);
  });
});
