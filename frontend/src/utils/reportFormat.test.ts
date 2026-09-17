import { describe, expect, it } from 'vitest';
import { formatScaled, formatScaledSigned, reportTone } from './reportFormat';

const billions = { div: 1e9, label: 'млрд ₽', digits: 1 };
const thousands = { div: 1e3, label: 'тыс. уп.', digits: 0 };
const raw = { div: 1, label: '₽', digits: 2 };

// toLocaleString разделяет разряды узким неразрывным пробелом.
const plain = (value: string) => value.replace(/[\u00a0\u202f]/g, ' ');

describe('formatScaled', () => {
  it('делит на шкалу и ограничивает знаки шкалой', () => {
    expect(formatScaled(7_512_000_000, billions)).toBe('7,5');
    expect(plain(formatScaled(1_250_400, thousands))).toBe('1 250');
  });

  it('не дописывает нули круглым делениям', () => {
    expect(formatScaled(0, billions)).toBe('0');
    expect(formatScaled(5e9, billions)).toBe('5');
    expect(plain(formatScaled(1234.5, raw))).toBe('1 234,5');
  });

  it('переживает нулевой делитель', () => {
    expect(formatScaled(12, { div: 0, label: '', digits: 0 })).toBe('12');
  });
});

describe('formatScaledSigned', () => {
  it('ставит плюс и типографский минус', () => {
    expect(formatScaledSigned(1.5e9, billions)).toBe('+1,5');
    expect(formatScaledSigned(-1.5e9, billions)).toBe('−1,5');
    expect(formatScaledSigned(0, billions)).toBe('0');
  });
});

describe('reportTone', () => {
  it('сужает строку сервера к четырём тонам', () => {
    expect(reportTone('good')).toBe('good');
    expect(reportTone('bad')).toBe('bad');
    expect(reportTone('')).toBe('neutral');
    expect(reportTone('purple')).toBe('neutral');
  });
});
