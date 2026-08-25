// Показ прогноза: подписи месяцев, режима ведения и величин.
//
// Расчётов здесь нет. Что считается введённым, а что выводится, решает backend
// (backend/services/network_forecast_service.go).

import type { NetworkEntryLevel, NetworkEntryUnit } from '../types/network';
import { formatRub, formatRubShort } from './networkPlan';

export const MONTHS = [
  'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
];

// Режим ведения бренда: на каком уровне вводят значения и в какой единице.
export interface EntryMode {
  level: NetworkEntryLevel;
  unit: NetworkEntryUnit;
}

export const MODE_OPTIONS: Array<EntryMode & { label: string; hint: string }> = [
  { level: 'brand', unit: 'rub', label: '₽ · бренд', hint: 'Ввод одной суммой на бренд' },
  { level: 'brand', unit: 'units', label: 'уп. · бренд', hint: 'Ввод упаковками на бренд, рубли по цене контракта' },
  { level: 'sku', unit: 'rub', label: '₽ · SKU', hint: 'Ввод по SKU в рублях, бренд — их сумма' },
  { level: 'sku', unit: 'units', label: 'уп. · SKU', hint: 'Ввод по SKU в упаковках, бренд — их сумма' },
];

export const modeLabel = (mode: EntryMode): string =>
  MODE_OPTIONS.find((option) => option.level === mode.level && option.unit === mode.unit)?.label
  ?? MODE_OPTIONS[0].label;

// Величина в той единице, в которой её показывают. Короткая запись — для свода,
// полная — для поля, где важна каждая цифра.
export const amountLabel = (value: number | null, unit: NetworkEntryUnit, short = true): string => {
  if (value == null) return '—';
  if (unit === 'units') return `${formatRub(value)} уп.`;
  return `${short ? formatRubShort(value) : formatRub(value)} ₽`;
};
