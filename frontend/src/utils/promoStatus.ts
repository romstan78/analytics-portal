// Статусы промо выводит сервер (services/promo_status.go); здесь — те же
// имена для подписей и проверка, открыт ли ввод факта. Правило вывода не
// дублируем: фронт только отражает статус, который пришёл.
export const PROMO_STATUS = {
  inApproval: 'В процессе согласования',
  finalized: 'Финализировано',
  done: 'Проведено',
  rejected: 'Отклонено',
} as const;

export type PromoStatus = (typeof PROMO_STATUS)[keyof typeof PROMO_STATUS];

export const PROMO_FACT_LOCKED_MESSAGE =
  'Факт можно вносить только после финализации промо (оба согласования получены)';

// Факт открыт после финализации; у проведённого промо его уточняют.
export const isPromoFactEditable = (status: string | null | undefined): boolean =>
  status === PROMO_STATUS.finalized || status === PROMO_STATUS.done;

export type PromoStatusColor = 'success' | 'warning' | 'error' | 'default';

// Цвет чипа статуса. Регистр не важен: строки до нормализации могли
// приходить в нижнем регистре, и такие ещё встречаются в выгрузках.
export function promoStatusColor(status: string | null | undefined): PromoStatusColor {
  switch ((status ?? '').trim().toLowerCase()) {
    case 'проведено':
    case 'финализировано':
      return 'success';
    case 'в процессе согласования':
      return 'warning';
    case 'отклонено':
      return 'error';
    default:
      return 'default';
  }
}
