import { useCallback, useEffect, useRef } from 'react';
import type { PromoFormValues } from './usePromoForm';
import { promoAPI } from '../api/promo';

/**
 * Пересчёт плановых и фактических показателей карточки промо.
 *
 * Считает сервер, а не браузер: формулы (ROI, uplift, доли) жили в двух местах
 * сразу — в services/promo_service.go и построчно здесь, — и синхронизировать
 * две копии было некому. Тот же приём уже применён к реестру сетей, где расчёт
 * живёт только в services, а TypeScript отвечает за форматирование.
 *
 * На сохранённые данные это не влияло и раньше: SavePromo всё равно
 * пересчитывает всё сам, а браузерная копия была только предпросмотром.
 */

// Поля, которые заполняет расчёт. Остальное в ответе форму не касается.
const PLAN_FIELDS = [
  'plan_promo_rub', 'plan_promo_uplift_units', 'plan_promo_uplift_rub', 'baseline_rub',
] as const;
const ACTUAL_FIELDS = [
  'actual_promo_rub', 'actual_promo_uplift_units', 'actual_promo_uplift_rub',
] as const;

const RECALC_DELAY_MS = 300;

const num = (value: unknown): number => {
  const parsed = parseFloat(String(value ?? ''));
  return Number.isFinite(parsed) ? parsed : 0;
};

export function usePromoCalculations(
  form: PromoFormValues,
  setForm: (update: (prev: PromoFormValues) => PromoFormValues) => void,
) {
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Номер запроса: ответы приходят по сети и могут разминуться с порядком
  // ввода, а устаревший ответ вернул бы в поля прошлые цифры.
  const requestId = useRef(0);

  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);

  const scheduleRecalc = useCallback((updates: Partial<PromoFormValues>) => {
    const draft = { ...form, ...updates };
    if (timer.current) clearTimeout(timer.current);

    timer.current = setTimeout(() => {
      const currentRequest = ++requestId.current;
      void promoAPI.calculate({
        year: num(draft.year),
        month: num(draft.month),
        sku: draft.sku ?? '',
        network_name: draft.network_name ?? '',
        mechanics: draft.mechanics ?? '',
        gm: num(draft.gm),
        contract_price: num(draft.contract_price),
        baseline_units: num(draft.baseline_units),
        plan_promo_units: num(draft.plan_promo_units),
        plan_investments_rub: num(draft.plan_investments_rub),
        actual_promo_sales_units: num(draft.actual_promo_sales_units),
        actual_investments: num(draft.actual_investments),
        // Факт в рублях и uplift не отправляются: сервер выводит их из факта
        // в упаковках. Отправить прежние значения — значит удержать их на
        // экране после того, как факт в упаковках стёрли.
        //
        // Скорректированный baseline, наоборот, нужен: он задаёт базу
        // фактического uplift, и без него сервер считал бы его от планового.
        actual_corrected_baseline: num(draft.actual_corrected_baseline),
        actual_external_ecom_units: num(draft.actual_external_ecom_units),
        promo_pharmacies: num(draft.promo_pharmacies),
        total_pharmacies: num(draft.total_pharmacies),
      })
        .then((calc) => {
          if (currentRequest !== requestId.current) return;
          setForm((prev) => {
            const next = { ...prev };
            for (const field of PLAN_FIELDS) {
              next[field] = (calc[field] ?? 0).toFixed(2);
            }
            for (const field of ACTUAL_FIELDS) {
              next[field] = (calc[field] ?? 0).toFixed(2);
            }
            next.plan_roi = (calc.plan_roi ?? 0).toFixed(1);
            next.actual_roi = (calc.actual_roi ?? 0).toFixed(1);
            return next;
          });
        })
        .catch(() => {
          // Расчёт не состоялся — поля остаются прежними. Значения всё равно
          // пересчитает сервер при сохранении, поэтому расходиться им не с чем.
        });
    }, RECALC_DELAY_MS);
  }, [form, setForm]);

  return { scheduleRecalc };
}
