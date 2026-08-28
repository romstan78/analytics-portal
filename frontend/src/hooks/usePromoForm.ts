import { useState, useCallback } from 'react';
import { promoAPI } from '../api/promo';
import type { PromoRow } from '../types/promo';

// Строка промо из таблицы или из GET /api/promo/:id. Часть полей может
// отсутствовать, поэтому обязателен только id.
export type PromoRowLike = Partial<PromoRow> & { id: number };

export interface PromoFormValues {
  id: number | null;
  network_name: string;
  kam: string;
  brand: string;
  sku: string;
  year: number;
  month: number;
  mechanics: string;
  gtn_opex: string;
  baseline_units: string;
  baseline_rub: string;
  plan_promo_units: string;
  plan_promo_rub: string;
  plan_promo_uplift_units: string;
  plan_promo_uplift_rub: string;
  plan_investments_rub: string;
  contract_price: string;
  plan_roi: string;
  gm: string;
  discount_amount: string;
  actual_promo_sales_units: string;
  actual_investments: string;
  actual_promo_rub: string;
  actual_promo_uplift_units: string;
  actual_promo_uplift_rub: string;
  actual_roi: string;
  actual_external_ecom_units: string;
  actual_corrected_baseline: string;
  agreement1: string;
  agreement2: string;
  conditions: string;
  comments: string;
  id_directum: string;
  ds_number: string;
  total_pharmacies: string;
  promo_pharmacies: string;
  status: string;
  ecom_segment?: string;
  updated_at: string | null;
  deleted_at: string | null;
}

// ─── Пустая форма ──────────────────────────────────────────────────────────
export const EMPTY_FORM: PromoFormValues = {
  id: null, network_name: '', kam: '', brand: '', sku: '',
  year: new Date().getFullYear(), month: new Date().getMonth() + 1,
  mechanics: '', gtn_opex: '', baseline_units: '', baseline_rub: '',
  plan_promo_units: '', plan_promo_rub: '', plan_promo_uplift_units: '',
  plan_promo_uplift_rub: '', plan_investments_rub: '', contract_price: '',
  plan_roi: '', gm: '', discount_amount: '',
  actual_promo_sales_units: '', actual_investments: '', actual_promo_rub: '',
  actual_promo_uplift_units: '', actual_promo_uplift_rub: '', actual_roi: '',
  actual_external_ecom_units: '', actual_corrected_baseline: '',
  agreement1: '', agreement2: '',
  conditions: '', comments: '',
  id_directum: '', ds_number: '',
  total_pharmacies: '', promo_pharmacies: '',
  status: '',
  updated_at: null, // ← для optimistic locking
  deleted_at: null,
};

// ─── Строка таблицы → значения формы ────────────────────────────────────────
// Отдельно от хука: те же значения нужны как точка отсчёта для черновика —
// с ними сравнивается введённое, чтобы понять, есть ли что восстанавливать.
export function formFromRow(row: PromoRowLike): PromoFormValues {
  return {
    ...EMPTY_FORM,
    id: row.id,
    network_name: (row.network_name as string) ?? '',
    kam: (row.kam as string) ?? '',
    brand: ((row.brand_as || row.brand) as string) ?? '',
    sku: (row.sku as string) ?? '',
    year: row.year as number,
    month: row.month as number,
    mechanics: (row.mechanics as string) ?? '',
    gtn_opex: (row.gtn_opex as string) ?? '',
    baseline_units: String(row.baseline_units ?? ''),
    baseline_rub: String(row.baseline_rub ?? ''),
    plan_promo_units: String(row.plan_promo_units ?? ''),
    plan_promo_rub: String(row.plan_promo_rub ?? ''),
    plan_promo_uplift_units: String(row.plan_promo_uplift_units ?? ''),
    plan_promo_uplift_rub: String(row.plan_promo_uplift_rub ?? ''),
    plan_investments_rub: String(row.plan_investments_rub ?? ''),
    contract_price: String(row.contract_price ?? ''),
    discount_amount: String(row.discount_amount ?? ''),
    plan_roi: String(row.plan_roi ?? ''),
    gm: String(row.gm ?? ''),
    total_pharmacies: String(row.total_pharmacies ?? ''),
    promo_pharmacies: String(row.promo_pharmacies ?? ''),
    actual_promo_sales_units: String(row.actual_promo_sales_units ?? ''),
    actual_investments: String(row.actual_investments ?? ''),
    actual_promo_rub: String(row.actual_promo_rub ?? ''),
    actual_promo_uplift_units: String(row.actual_promo_uplift_units ?? ''),
    actual_promo_uplift_rub: String(row.actual_promo_uplift_rub ?? ''),
    actual_roi: String(row.actual_roi ?? ''),
    actual_external_ecom_units: String(row.actual_external_ecom_units ?? ''),
    actual_corrected_baseline: String(row.actual_corrected_baseline ?? ''),
    agreement1: (row.agreement1 as string) ?? '',
    agreement2: (row.agreement2 as string) ?? '',
    conditions: (row.conditions as string) ?? '',
    comments: (row.comments as string) ?? '',
    id_directum: (row.id_directum as string) ?? '',
    ds_number: (row.ds_number as string) ?? '',
    status: (row.status as string) ?? '',
    updated_at: (row.updated_at as string) ?? null,
    deleted_at: (row.deleted_at as string) ?? null,
  };
}

interface SaveResult {
  success: boolean;
  message: string;
  // Форма после ответа сервера. Сохранённые значения он нормализует и
  // пересчитывает, поэтому точкой отсчёта — в том числе для черновика —
  // годится только она, а не то, что ушло в запрос.
  form?: PromoFormValues;
}

interface UsePromoFormCallbacks {
  onEditSuccess?: (id: number, data: Record<string, unknown>) => void;
  onDeleteSuccess?: (id: number) => void;
  onCreateSuccess?: () => void;
}

// ─── Хук ────────────────────────────────────────────────────────────────────
export function usePromoForm({ onEditSuccess, onDeleteSuccess, onCreateSuccess }: UsePromoFormCallbacks) {
  const [form, setForm] = useState<PromoFormValues>({ ...EMPTY_FORM });
  const [editMode, setEditMode] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);

  // ─── Загрузка строки в форму (клик по таблице) ──────────────────────────
  const handleRowClick = useCallback((row: PromoRowLike) => {
    setForm(formFromRow(row));
    setEditMode(true);
  }, []);

  // ─── Сохранение (INSERT или UPDATE). commentOverride — комментарий КАМ из диалога (обходит batching) ──
  const handleSave = useCallback(async (commentOverride?: string | null): Promise<SaveResult> => {
    if (!form.sku || !form.network_name) {
      return { success: false, message: '⚠️ Заполните Сеть и SKU' };
    }
    setSaving(true);
    try {
      const commentsValue = commentOverride !== undefined ? commentOverride : (form.comments ?? null);
      const payload: Record<string, unknown> = {
        id: form.id || undefined,
        network_name: form.network_name, kam: form.kam, brand: form.brand, brand_as: form.brand,
        sku: form.sku, year: parseInt(String(form.year)), month: parseInt(String(form.month)),
        mechanics: form.mechanics, gtn_opex: form.gtn_opex,
        baseline_units: parseFloat(form.baseline_units) || null,
        plan_promo_units: parseFloat(form.plan_promo_units) || null,
        plan_investments_rub: parseFloat(form.plan_investments_rub) || null,
        contract_price: parseFloat(form.contract_price) || null,
        gm: parseFloat(form.gm) || null,
        id_directum: form.id_directum ?? null,
        ds_number: form.ds_number ?? null,
        discount_amount: parseFloat(form.discount_amount) || null,
        conditions: form.conditions ?? null,
        comments: commentsValue,
        ecom_segment: form.ecom_segment,
        total_pharmacies: form.total_pharmacies !== '' ? parseInt(form.total_pharmacies) : null,
        promo_pharmacies: form.promo_pharmacies !== '' ? parseInt(form.promo_pharmacies) : null,
        actual_promo_sales_units: parseFloat(form.actual_promo_sales_units) || null,
        actual_investments: parseFloat(form.actual_investments) || null,
        actual_promo_rub: parseFloat(form.actual_promo_rub) || null,
        actual_promo_uplift_units: parseFloat(form.actual_promo_uplift_units) || null,
        actual_promo_uplift_rub: parseFloat(form.actual_promo_uplift_rub) || null,
        actual_external_ecom_units: form.actual_external_ecom_units !== '' ? parseFloat(form.actual_external_ecom_units) : null,
        actual_corrected_baseline: form.actual_corrected_baseline !== '' ? parseFloat(form.actual_corrected_baseline) : null,
        agreement1: form.agreement1 ?? null,
        agreement2: form.agreement2 ?? null,
        status: form.status,
        updated_at: form.updated_at,
      };

      const result = await promoAPI.save(payload) as { data?: Record<string, unknown>; id: number; message: string };

      // Форма во время сохранения не меняется (кнопка заблокирована), поэтому
      // ответ накладывается на неё же — и возвращается вызывающему.
      const savedForm = result.data
        ? { ...form, ...result.data, id: result.id } as PromoFormValues
        : form;
      if (result.data) {
        setForm(savedForm);
      }

      if (form.id && onEditSuccess && result.data) {
        onEditSuccess(form.id, result.data);
      } else if (!form.id && onCreateSuccess) {
        onCreateSuccess();
      }

      return { success: true, message: '✅ Сохранено', form: savedForm };
    } catch (err: unknown) {
      const apiErr = err as { status?: number; message?: string };
      if (apiErr.status === 409) {
        return { success: false, message: '⚠️ Запись изменена другим пользователем. Обновите страницу.' };
      }
      return { success: false, message: '❌ ' + (apiErr.message || 'Ошибка сохранения') };
    } finally {
      setSaving(false);
    }
  }, [form, onEditSuccess, onCreateSuccess]);

  // ─── Удаление (soft-delete) ─────────────────────────────────────────────
  const handleDelete = useCallback(async (): Promise<SaveResult> => {
    if (!form.id) return { success: false, message: 'Нет ID' };
    setDeleting(true);
    try {
      await promoAPI.delete(form.id);
      if (onDeleteSuccess) onDeleteSuccess(form.id);
      return { success: true, message: '🗑️ Удалено' };
    } catch (err: unknown) {
      const apiErr = err as { message?: string };
      return { success: false, message: '❌ ' + (apiErr.message || 'Ошибка удаления') };
    } finally {
      setDeleting(false);
    }
  }, [form.id, onDeleteSuccess]);

  // ─── Сброс формы ────────────────────────────────────────────────────────
  const resetForm = useCallback(() => {
    setForm({ ...EMPTY_FORM });
    setEditMode(false);
  }, []);

  return {
    form, setForm, editMode, saving, deleting,
    handleRowClick, handleSave, handleDelete, resetForm,
  };
}