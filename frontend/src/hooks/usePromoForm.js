import { useState, useCallback } from 'react';
import { promoAPI } from '../api/promo';

// ─── Пустая форма ──────────────────────────────────────────────────────────
const EMPTY_FORM = {
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
};

// ─── Хук ────────────────────────────────────────────────────────────────────
// Колбэки:
//   onEditSuccess(id, data) — после успешного редактирования
//   onDeleteSuccess(id)     — после успешного удаления
//   onCreateSuccess()       — после создания нового промо
export function usePromoForm({ onEditSuccess, onDeleteSuccess, onCreateSuccess }) {
  const [form, setForm] = useState({ ...EMPTY_FORM });
  const [editMode, setEditMode] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);

  // ─── Загрузка строки в форму (клик по таблице) ──────────────────────────
  const handleRowClick = useCallback((row) => {
    setForm({
      id: row.id,
      network_name: row.network_name ?? '',
      kam: row.kam ?? '',
      brand: row.brand_as ?? row.brand ?? '',
      sku: row.sku ?? '',
      year: row.year,
      month: row.month,
      mechanics: row.mechanics ?? '',
      gtn_opex: row.gtn_opex ?? '',
      baseline_units: row.baseline_units ?? '',
      baseline_rub: row.baseline_rub ?? '',
      plan_promo_units: row.plan_promo_units ?? '',
      plan_promo_rub: row.plan_promo_rub ?? '',
      plan_promo_uplift_units: row.plan_promo_uplift_units ?? '',
      plan_promo_uplift_rub: row.plan_promo_uplift_rub ?? '',
      plan_investments_rub: row.plan_investments_rub ?? '',
      contract_price: row.contract_price ?? '',
      discount_amount: row.discount_amount ?? '',
      plan_roi: row.plan_roi ?? '',
      gm: row.gm ?? '',
      total_pharmacies: row.total_pharmacies ?? '',
      promo_pharmacies: row.promo_pharmacies ?? '',
      actual_promo_sales_units: row.actual_promo_sales_units ?? '',
      actual_investments: row.actual_investments ?? '',
      actual_promo_rub: row.actual_promo_rub ?? '',
      actual_promo_uplift_units: row.actual_promo_uplift_units ?? '',
      actual_promo_uplift_rub: row.actual_promo_uplift_rub ?? '',
      actual_roi: row.actual_roi ?? '',
      actual_external_ecom_units: row.actual_external_ecom_units ?? '',
      actual_corrected_baseline: row.actual_corrected_baseline ?? '',
      agreement1: row.agreement1 ?? '',
      agreement2: row.agreement2 ?? '',
      conditions: row.conditions ?? '',
      comments: row.comments ?? '',
      id_directum: row.id_directum ?? '',
      ds_number: row.ds_number ?? '',
      status: row.status ?? '',
      updated_at: row.updated_at ?? null, // ← критично для optimistic locking
    });
    setEditMode(true);
  }, []);

  // ─── Сохранение (INSERT или UPDATE) ─────────────────────────────────────
  const handleSave = useCallback(async () => {
    if (!form.sku || !form.network_name) {
      return { success: false, message: '⚠️ Заполните Сеть и SKU' };
    }
    setSaving(true);
    try {
      const payload = {
        id: form.id || undefined,
        network_name: form.network_name, kam: form.kam, brand: form.brand, brand_as: form.brand,
        sku: form.sku, year: parseInt(form.year), month: parseInt(form.month),
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
        comments: form.comments ?? null,
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
        updated_at: form.updated_at, // ← для optimistic locking
      };

      const result = await promoAPI.save(payload);

      if (result.data) {
        setForm(prev => ({ ...prev, ...result.data, id: result.id }));
      }

      if (form.id && onEditSuccess && result.data) {
        onEditSuccess(form.id, result.data);
      } else if (!form.id && onCreateSuccess) {
        onCreateSuccess();
      }

      return { success: true, message: '✅ Сохранено' };
    } catch (err) {
      // 409 — конфликт версий (optimistic locking)
      if (err.status === 409) {
        return { success: false, message: '⚠️ Запись изменена другим пользователем. Обновите страницу.' };
      }
      return { success: false, message: '❌ ' + (err.message || 'Ошибка сохранения') };
    } finally {
      setSaving(false);
    }
  }, [form, onEditSuccess, onCreateSuccess]);

  // ─── Удаление (soft-delete) ─────────────────────────────────────────────
  const handleDelete = useCallback(async () => {
    if (!form.id) return { success: false, message: 'Нет ID' };
    setDeleting(true);
    try {
      await promoAPI.delete(form.id);
      if (onDeleteSuccess) onDeleteSuccess(form.id);
      return { success: true, message: '🗑️ Удалено' };
    } catch (err) {
      return { success: false, message: '❌ ' + (err.message || 'Ошибка удаления') };
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
    handleRowClick, handleSave, handleDelete, resetForm
  };
}

export { EMPTY_FORM };