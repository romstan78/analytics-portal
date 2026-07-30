import { useState, useCallback } from 'react';
import { promoAPI } from '../api/promo';

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
  status: '',
};

export function usePromoForm(onSave) {
  const [form, setForm] = useState({ ...EMPTY_FORM });
  const [editMode, setEditMode] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const handleRowClick = useCallback((row) => {
    setForm({
      id: row.id,
      network_name: row.network_name ?? '',
      id_directum: row.id_directum ?? '',        // ← добавить
      ds_number: row.ds_number ?? '',            // ← добавить
      kam: row.kam ?? '',
      brand: row.brand_as ?? row.brand ?? '',
      sku: row.sku ?? '',
      year: row.year,
      month: row.month,
      mechanics: row.mechanics ?? '',
      gtn_opex: row.gtn_opex ?? '',
      // Числовые поля — ?? сохраняет 0
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
      actual_promo_sales_units: row.actual_promo_sales_units ?? '',
      actual_investments: row.actual_investments ?? '',
      actual_promo_rub: row.actual_promo_rub ?? '',
      actual_promo_uplift_units: row.actual_promo_uplift_units ?? '',
      actual_promo_uplift_rub: row.actual_promo_uplift_rub ?? '',
      actual_roi: row.actual_roi ?? '',
      actual_external_ecom_units: row.actual_external_ecom_units ?? '',
      actual_corrected_baseline: row.actual_corrected_baseline ?? '',
      total_pharmacies: row.total_pharmacies ?? '',
      promo_pharmacies: row.promo_pharmacies ?? '',
      // Текстовые — ?? сохраняет ''
      agreement1: row.agreement1 ?? '',
      agreement2: row.agreement2 ?? '',
      conditions: row.conditions ?? '',
      comments: row.comments ?? '',
      status: row.status ?? '',
    });
    setEditMode(true);
  }, []);

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
      };

      const result = await promoAPI.save(payload);

      if (result.data) {
        setForm(prev => ({ ...prev, ...result.data, id: result.id }));
      }

      if (onSave) onSave();
      return { success: true, message: '✅ Сохранено' };
    } catch (err) {
      return { success: false, message: '❌ Ошибка: ' + err.message };
    } finally {
      setSaving(false);
    }
  }, [form, onSave]);

  const handleDelete = useCallback(async () => {
    if (!form.id) return { success: false, message: 'Нет ID' };
    setDeleting(true);
    try {
      await promoAPI.delete(form.id);
      return { success: true, message: '🗑️ Удалено' };
    } catch (err) {
      return { success: false, message: '❌ Ошибка удаления' };
    } finally {
      setDeleting(false);
    }
  }, [form.id]);

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