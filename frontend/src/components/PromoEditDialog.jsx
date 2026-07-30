import { useState, useEffect } from 'react';
import {
  Button, Box, Typography, TextField, Grid, Paper, Dialog, DialogTitle,
  DialogContent, DialogActions, IconButton, MenuItem
} from '@mui/material';
import { Save as SaveIcon, Close as CloseIcon, Delete as DeleteIcon } from '@mui/icons-material';
import { promoAPI } from '../api/promo';

const MONTH_OPTIONS = [
  { label: 'Январь', value: 1 }, { label: 'Февраль', value: 2 }, { label: 'Март', value: 3 }, { label: 'Апрель', value: 4 },
  { label: 'Май', value: 5 }, { label: 'Июнь', value: 6 }, { label: 'Июль', value: 7 }, { label: 'Август', value: 8 },
  { label: 'Сентябрь', value: 9 }, { label: 'Октябрь', value: 10 }, { label: 'Ноябрь', value: 11 }, { label: 'Декабрь', value: 12 },
];

const fmtDisplay = (v) => {
  if (v == null || v === '') return '';
  return Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
};

export default function PromoEditDialog({
  open, onClose, form, setForm, recalcPlan, recalcActual,
  onSave, onDelete, saving, deleting,
  meta, allSkuOptions, allNetworkOptions, investmentTypes
}) {
  const [editingFields, setEditingFields] = useState({});

  const fetchSKUInfoForEdit = async (sku) => {
    try { const data = await promoAPI.getSKUInfo(sku); if (data.brand) setForm(prev => ({ ...prev, brand: data.brand })); } catch (e) {}
  };

  useEffect(() => { setEditingFields({}); }, [open]);

  if (!form) return null;

  const updateField = (field) => (e) => setForm(prev => ({ ...prev, [field]: e.target.value }));

  const planTriggers = ['baseline_units', 'plan_promo_units', 'contract_price', 'plan_investments_rub'];
  const actualTriggers = ['actual_promo_sales_units', 'actual_investments', 'actual_promo_uplift_units'];
  const textFields = ['network_name', 'kam', 'brand', 'sku', 'mechanics', 'gtn_opex', 'conditions', 'comments', 'ecom_segment', 'status', 'agreement1', 'agreement2', 'id_directum', 'ds_number'];

  const handleFieldChange = (field) => (e) => {
    const rawValue = e.target.value;
    const cleanValue = rawValue.replace(/\s/g, '').replace(',', '.');

    if (planTriggers.includes(field)) {
      setForm(prev => {
        const calc = recalcPlan({ [field]: cleanValue });
        return { ...prev, [field]: cleanValue, ...calc };
      });
    } else if (actualTriggers.includes(field)) {
      setForm(prev => {
        const calc = recalcActual({ [field]: cleanValue });
        return { ...prev, [field]: cleanValue, ...calc };
      });
    } else {
      setForm(prev => ({ ...prev, [field]: textFields.includes(field) ? rawValue : cleanValue }));
    }
  };

  const handleFocus = (field) => () => {
    setEditingFields(prev => ({ ...prev, [field]: true }));
  };

  const handleBlur = (field) => () => {
    setEditingFields(prev => ({ ...prev, [field]: false }));
  };

  const getDisplayValue = (field, editable) => {
    if (!editable) {
      return form[field] != null ? fmtDisplay(form[field]) : '';
    }
    if (editingFields[field]) {
      return form[field] != null ? String(form[field]) : '';
  }
    return form[field] != null ? fmtDisplay(form[field]) : '';
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="lg" fullWidth PaperProps={{ sx: { height: '90vh' } }}>
      <DialogTitle sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Typography component="span" sx={{ fontSize: '1.25rem', fontWeight: 600 }}>
          Редактирование: {form.network_name || 'Промо'}
        </Typography>
        <IconButton onClick={onClose}><CloseIcon /></IconButton>
      </DialogTitle>
      <DialogContent sx={{ overflow: 'auto' }}>
        <Grid container spacing={2} sx={{ mt: 0.5 }}>
          {/* Основные данные */}
          <Grid item xs={12} sx={{ width: '100%' }}>
          <Paper variant="outlined" sx={{ p: 2, bgcolor: '#fafafa', width: '100%' }}>
              <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1.5 }}>📋 Основные данные</Typography>
              
              <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr 1fr', gap: 2 }}>
                {/* Строка 1 */}
                <TextField label="ID Директум" size="small" fullWidth value={form.id_directum || ''} onChange={updateField('id_directum')} />
                <TextField label="№ ДС" size="small" fullWidth value={form.ds_number || ''} onChange={updateField('ds_number')} />
                <TextField select size="small" fullWidth label="Месяц" value={form.month || ''} onChange={updateField('month')}>
                  {MONTH_OPTIONS.map(m => <MenuItem key={m.value} value={m.value}>{m.label}</MenuItem>)}
                </TextField>
                <TextField label="Год" type="number" size="small" fullWidth value={form.year || ''} onChange={updateField('year')} slotProps={{ htmlInput: { min: 2020, max: 2030 } }} />

                {/* Строка 2 */}
                <TextField select size="small" fullWidth label="SKU" value={form.sku || ''}
                  onChange={(e) => { const v = e.target.value; setForm(prev => ({ ...prev, sku: v })); if (v) fetchSKUInfoForEdit(v); }}>
                  {allSkuOptions.map(s => <MenuItem key={s} value={s}>{s}</MenuItem>)}
                </TextField>
                <TextField select size="small" fullWidth label="Механика" value={form.mechanics || ''} onChange={updateField('mechanics')}>
                  {meta.mechanics?.map(m => <MenuItem key={m} value={m}>{m}</MenuItem>)}
                </TextField>
                <TextField select size="small" fullWidth label="Тип инвест." value={form.gtn_opex || ''} onChange={updateField('gtn_opex')}>
                  {investmentTypes.map(t => <MenuItem key={t} value={t}>{t}</MenuItem>)}
                </TextField>
                <TextField select size="small" fullWidth label="Статус" value={form.status || ''} onChange={updateField('status')}>
                  {(() => { const opts = [...(meta.status || [])]; if (form.status && !opts.includes(form.status)) opts.push(form.status); return opts.map(s => <MenuItem key={s} value={s}>{s}</MenuItem>); })()}
                </TextField>

                {/* Строка 3 */}
                <TextField label="Аптек ТОТАЛ" type="number" size="small" fullWidth value={form.total_pharmacies || ''} onChange={updateField('total_pharmacies')} slotProps={{ htmlInput: { min: 0 } }} />
                <TextField label="Аптек в промо" type="number" size="small" fullWidth value={form.promo_pharmacies || ''} onChange={updateField('promo_pharmacies')} slotProps={{ htmlInput: { min: 0 } }} />
                <TextField label="Согласование 1" size="small" fullWidth value={form.agreement1 || ''} onChange={updateField('agreement1')} />
                <TextField label="Согласование 2" size="small" fullWidth value={form.agreement2 || ''} onChange={updateField('agreement2')} />
              </Box>

              <TextField 
                label="Условия" size="small" fullWidth multiline minRows={2} maxRows={4}
                value={form.conditions || ''} onChange={updateField('conditions')}
                sx={{ mt: 2 }}
              />
              <TextField 
                label="Комментарии" size="small" fullWidth multiline minRows={2} maxRows={4}
                value={form.comments || ''} onChange={updateField('comments')}
                sx={{ mt: 2 }}
              />
            </Paper>
          </Grid>

          {/* Плановые */}
          <Grid item xs={12} md={6}>
            <Paper variant="outlined" sx={{ p: 2, bgcolor: '#f0f4ff', height: '100%' }}>
              <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1.5, color: '#1a237e' }}>📊 Плановые показатели</Typography>
              <Grid container spacing={2}>
                {[
                  { label: 'Baseline (уп)', field: 'baseline_units', editable: true },
                  { label: 'Baseline (руб)', field: 'baseline_rub', editable: true },
                  { label: 'План промо (уп)', field: 'plan_promo_units', editable: true },
                  { label: 'План промо (руб)', field: 'plan_promo_rub', editable: true },
                  { label: 'Сумма скидки', field: 'discount_amount', editable: true },
                  { label: 'План инвестиций (руб)', field: 'plan_investments_rub', editable: true },
                  { label: 'Цена контракта', field: 'contract_price', editable: true },
                  { label: 'Uplift (уп)', field: 'plan_promo_uplift_units', editable: true },
                  { label: 'Uplift (руб)', field: 'plan_promo_uplift_rub', editable: true },
                  { label: 'ROI план %', field: 'plan_roi', editable: false },
                ].map(({ label, field, editable }) => (
                  <Grid item xs={6} key={field}>
                    <TextField
                      label={label}
                      type="text"
                      size="small"
                      fullWidth
                      value={getDisplayValue(field, editable)}
                      onChange={editable ? handleFieldChange(field) : undefined}
                      onFocus={editable ? handleFocus(field) : undefined}
                      onBlur={editable ? handleBlur(field) : undefined}
                      slotProps={{
                        input: editable ? {} : { readOnly: true },
                        htmlInput: { inputMode: 'text' },
                      }}
                    />
                  </Grid>
                ))}
              </Grid>
            </Paper>
          </Grid>

          {/* Фактические */}
          <Grid item xs={12} md={6}>
            <Paper variant="outlined" sx={{ p: 2, bgcolor: '#f0fff0', height: '100%' }}>
              <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1.5, color: '#1b5e20' }}>✅ Фактические показатели</Typography>
              <Grid container spacing={2}>
                {[
                  { label: 'Факт продажи (уп)', field: 'actual_promo_sales_units', editable: true },
                  { label: 'Факт промо (руб)', field: 'actual_promo_rub', editable: true },
                  { label: 'Факт инвестиции', field: 'actual_investments', editable: true },
                  { label: 'Факт Uplift (уп)', field: 'actual_promo_uplift_units', editable: true },
                  { label: 'Факт Uplift (руб)', field: 'actual_promo_uplift_rub', editable: true },
                  { label: 'Факт ROI %', field: 'actual_roi', editable: false },
                  { label: 'Внешний e-com (уп)', field: 'actual_external_ecom_units', editable: true },
                  { label: 'Скорр. Baseline', field: 'actual_corrected_baseline', editable: true },
                ].map(({ label, field, editable }) => (
                  <Grid item xs={6} key={field}>
                    <TextField
                      label={label}
                      type="text"
                      size="small"
                      fullWidth
                      value={getDisplayValue(field, editable)}
                      onChange={editable ? handleFieldChange(field) : undefined}
                      onFocus={editable ? handleFocus(field) : undefined}
                      onBlur={editable ? handleBlur(field) : undefined}
                      slotProps={{
                        input: editable ? {} : { readOnly: true },
                        htmlInput: { inputMode: 'text' },
                      }}
                    />
                  </Grid>
                ))}
              </Grid>
            </Paper>
          </Grid>
        </Grid>
      </DialogContent>
      <DialogActions sx={{ justifyContent: 'space-between', px: 3, pb: 2 }}>
        <Button color="error" startIcon={<DeleteIcon />} onClick={onDelete}>Удалить</Button>
        <Box>
          <Button onClick={onClose} sx={{ mr: 1 }}>Отмена</Button>
          <Button variant="contained" startIcon={<SaveIcon />} onClick={onSave} disabled={saving}>
            {saving ? 'Сохранение...' : 'Сохранить'}
          </Button>
        </Box>
      </DialogActions>
    </Dialog>
  );
}