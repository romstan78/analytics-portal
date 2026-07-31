import { useState, useEffect } from 'react';
import {
  Button, Box, Typography, TextField, Grid, Paper, Dialog, DialogTitle,
  DialogContent, DialogActions, IconButton, MenuItem, Tooltip, Chip
} from '@mui/material';
import { Save as SaveIcon, Close as CloseIcon, Delete as DeleteIcon } from '@mui/icons-material';
import { promoAPI } from '../api/promo';

// ─── Месяцы ────────────────────────────────────────────────────────────────
const MONTH_OPTIONS = [
  { label: 'Январь', value: 1 }, { label: 'Февраль', value: 2 }, { label: 'Март', value: 3 }, { label: 'Апрель', value: 4 },
  { label: 'Май', value: 5 }, { label: 'Июнь', value: 6 }, { label: 'Июль', value: 7 }, { label: 'Август', value: 8 },
  { label: 'Сентябрь', value: 9 }, { label: 'Октябрь', value: 10 }, { label: 'Ноябрь', value: 11 }, { label: 'Декабрь', value: 12 },
];

// ─── Форматирование ────────────────────────────────────────────────────────
const fmtDisplay = (v) => {
  if (v == null || v === '') return '';
  return Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
};

// ─── Чип статуса согласования (для KAM — компактный, с Tooltip и скроллингом) ─
const AgreementChip = ({ label, value }) => {
  const text = value || '';
  if (!text || text === '0') return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.25 }}>
      <Typography variant="caption" color="text.secondary" sx={{ lineHeight: 1.2 }}>{label}</Typography>
      <Chip label="Ожидает" size="small" variant="outlined" sx={{ borderColor: '#94a3b8', color: '#64748b', fontWeight: 500, height: 28 }} />
    </Box>
  );

  const lower = text.toLowerCase();
  const isApproved = lower.startsWith('согласовано');
  const isRejected = lower.startsWith('отклонено');
  const color = isApproved ? '#16a34a' : isRejected ? '#dc2626' : '#6366f1';
  const bg = isApproved ? '#f0fdf4' : isRejected ? '#fef2f2' : '#eef2ff';
  const shortLabel = isApproved ? '✓ Согласовано' : isRejected ? '✗ Отклонено' : '💬 Комментарий';

  const chip = (
    <Chip
      label={shortLabel}
      size="small"
      variant="filled"
      sx={{
        bgcolor: bg, color, fontWeight: 600, height: 28, maxWidth: '100%',
        '& .MuiChip-label': { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
      }}
    />
  );

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.25, minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary" sx={{ lineHeight: 1.2 }}>{label}</Typography>
      <Tooltip title={text} arrow placement="top" slotProps={{ tooltip: { sx: { maxWidth: 320, whiteSpace: 'pre-wrap', wordBreak: 'break-word' } } }}>
        {chip}
      </Tooltip>
    </Box>
  );
};

// ─── Подсветка согласований ────────────────────────────────────────────────
const renderAgreementSx = (value) => {
  if (value == null || value === '' || value === '0') return {};
  const v = String(value).toLowerCase();
  if (v.startsWith('согласовано')) return { 
    '& .MuiOutlinedInput-input': { color: '#16a34a', fontWeight: 600 },
    '& .MuiOutlinedInput-notchedOutline': { borderColor: '#16a34a' },
    '& .MuiInputBase-root': { bgcolor: '#f0fdf4' },
  };
  if (v.startsWith('отклонено')) return { 
    '& .MuiOutlinedInput-input': { color: '#dc2626', fontWeight: 600 },
    '& .MuiOutlinedInput-notchedOutline': { borderColor: '#dc2626' },
    '& .MuiInputBase-root': { bgcolor: '#fef2f2' },
  };
  return { 
    '& .MuiOutlinedInput-input': { color: '#6366f1', fontStyle: 'italic' },
    '& .MuiInputBase-root': { bgcolor: '#eef2ff' },
  };
};

// ─── Компонент ─────────────────────────────────────────────────────────────
export default function PromoEditDialog({
  open, onClose, form, setForm, recalcPlan, recalcActual,
  onSave, onDelete, saving, deleting,
  meta, allSkuOptions, allNetworkOptions, investmentTypes,
  role,
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
      setForm(prev => { const calc = recalcPlan({ [field]: cleanValue }); return { ...prev, [field]: cleanValue, ...calc }; });
    } else if (actualTriggers.includes(field)) {
      setForm(prev => { const calc = recalcActual({ [field]: cleanValue }); return { ...prev, [field]: cleanValue, ...calc }; });
    } else {
      setForm(prev => ({ ...prev, [field]: textFields.includes(field) ? rawValue : cleanValue }));
    }
  };

  const handleFocus = (field) => () => setEditingFields(prev => ({ ...prev, [field]: true }));
  const handleBlur = (field) => () => setEditingFields(prev => ({ ...prev, [field]: false }));

  const getDisplayValue = (field, editable) => {
    if (!editable) return form[field] != null ? fmtDisplay(form[field]) : '';
    if (editingFields[field]) return form[field] != null ? String(form[field]) : '';
    return form[field] != null ? fmtDisplay(form[field]) : '';
  };

  const isKAM = role === 'agreement1' || role === 'agreement2';

  return (
    <Dialog 
      open={open} 
      onClose={onClose} 
      maxWidth="lg" 
      fullWidth 
      // Растягиваем окно почти на всю высоту экрана
      PaperProps={{ sx: { height: '96vh', maxHeight: '96vh', bgcolor: '#f5f7fa' } }}
    >
      
      {/* Чуть уменьшили отступы (py: 1.5) в шапке */}
      <DialogTitle sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', bgcolor: '#ffffff', py: 1.5, px: 3 }}>
        <Typography component="span" sx={{ fontSize: '1.25rem', fontWeight: 600 }}>
          Редактирование: {form.network_name || 'Промо'}
        </Typography>
        <IconButton onClick={onClose} size="small"><CloseIcon /></IconButton>
      </DialogTitle>
  
      {/* Уменьшили внутренний отступ формы (p: 2 вместо 3) */}
      <DialogContent dividers sx={{ p: 2, overflow: 'auto' }}>
        
        {/* Уменьшили расстояние между тремя блоками (gap: 1.5 вместо 3) */}
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
  
          {(() => {
            const gridStyles = { 
              display: 'grid', 
              gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', 
              gap: 1.5, // Уменьшили расстояние между полями (было 2.5)
            };
  
            const paperStyles = {
              p: 2, // Уменьшили отступы внутри белых блоков (было 3)
              borderRadius: 2, 
              boxShadow: '0 2px 8px rgba(0,0,0,0.05)',
            };
  
            // Стиль для заголовков блоков (mb: 1.5 вместо 2.5)
            const titleStyles = { fontWeight: 600, mb: 1.5 };
  
            return (
              <>
                {/* ─── Блок 1: Основные данные ──────────────────── */}
                <Paper sx={{ ...paperStyles, bgcolor: '#ffffff' }}>
                  <Typography variant="subtitle1" sx={{ ...titleStyles }}>📋 Основные данные</Typography>
                  
                  <Box sx={gridStyles}>
                    <TextField label="ID Директум" size="small" fullWidth value={form.id_directum || ''} onChange={updateField('id_directum')} />
                    <TextField label="№ ДС" size="small" fullWidth value={form.ds_number || ''} onChange={updateField('ds_number')} />
                    <TextField select size="small" fullWidth label="Месяц" value={form.month || ''} onChange={updateField('month')}>
                      {MONTH_OPTIONS.map(m => <MenuItem key={m.value} value={m.value}>{m.label}</MenuItem>)}
                    </TextField>
                    <TextField label="Год" type="number" size="small" fullWidth value={form.year || ''} onChange={updateField('year')} slotProps={{ htmlInput: { min: 2020, max: 2030 } }} />
  
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
  
                    <TextField label="Аптек ТОТАЛ" type="number" size="small" fullWidth value={form.total_pharmacies || ''} onChange={updateField('total_pharmacies')} slotProps={{ htmlInput: { min: 0 } }} />
                    <TextField label="Аптек в промо" type="number" size="small" fullWidth value={form.promo_pharmacies || ''} onChange={updateField('promo_pharmacies')} slotProps={{ htmlInput: { min: 0 } }} />
                    {isKAM ? (
                      <AgreementChip label="Согласование 1" value={form.agreement1} />
                    ) : (
                      <TextField label="Согласование 1" size="small" fullWidth value={form.agreement1 || ''}
                        onChange={updateField('agreement1')} sx={renderAgreementSx(form.agreement1)} />
                    )}
                    {isKAM ? (
                      <AgreementChip label="Согласование 2" value={form.agreement2} />
                    ) : (
                      <TextField label="Согласование 2" size="small" fullWidth value={form.agreement2 || ''}
                        onChange={updateField('agreement2')} sx={renderAgreementSx(form.agreement2)} />
                    )}
                  </Box>
  
                  {/* Поля Условия и Комментарии: minRows={1} экономит место, но позволяет расширяться */}
                  <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, mt: 1.5 }}>
                    <TextField label="Условия" size="small" fullWidth multiline minRows={1} maxRows={3}
                      value={form.conditions || ''} onChange={updateField('conditions')} />
                    <TextField label="Комментарии" size="small" fullWidth multiline minRows={1} maxRows={3}
                      value={form.comments || ''} onChange={updateField('comments')} />
                  </Box>
                </Paper>
  
                {/* ─── Блок 2: Плановые показатели ──────────────────── */}
                <Paper sx={{ ...paperStyles, bgcolor: '#f8faff', border: '1px solid #e0e7ff' }}>
                  <Typography variant="subtitle1" sx={{ ...titleStyles, color: '#1a237e' }}>📊 Плановые показатели</Typography>
                  <Box sx={gridStyles}>
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
                      <TextField key={field} label={label} type="text" size="small" fullWidth
                        value={getDisplayValue(field, editable)}
                        onChange={editable ? handleFieldChange(field) : undefined}
                        onFocus={editable ? handleFocus(field) : undefined}
                        onBlur={editable ? handleBlur(field) : undefined}
                        slotProps={{ input: editable ? {} : { readOnly: true }, htmlInput: { inputMode: 'text' } }} 
                        sx={{ bgcolor: editable ? '#ffffff' : '#f0f0f0' }} 
                      />
                    ))}
                  </Box>
                </Paper>
  
                {/* ─── Блок 3: Фактические показатели ──────────────────── */}
                <Paper sx={{ ...paperStyles, bgcolor: '#f2fbf4', border: '1px solid #d4ebd9' }}>
                  <Typography variant="subtitle1" sx={{ ...titleStyles, color: '#1b5e20' }}>✅ Фактические показатели</Typography>
                  <Box sx={gridStyles}>
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
                      <TextField key={field} label={label} type="text" size="small" fullWidth
                        value={getDisplayValue(field, editable)}
                        onChange={editable ? handleFieldChange(field) : undefined}
                        onFocus={editable ? handleFocus(field) : undefined}
                        onBlur={editable ? handleBlur(field) : undefined}
                        slotProps={{ input: editable ? {} : { readOnly: true }, htmlInput: { inputMode: 'text' } }} 
                        sx={{ bgcolor: editable ? '#ffffff' : '#e9ecea' }} 
                      />
                    ))}
                  </Box>
                </Paper>
              </>
            );
          })()}
  
        </Box>
      </DialogContent>
  
      {/* Уменьшили отступы в подвале (py: 1.5) */}
      <DialogActions sx={{ justifyContent: 'space-between', px: 3, py: 1.5, bgcolor: '#ffffff' }}>
        <Button color="error" startIcon={<DeleteIcon />} onClick={onDelete}>Удалить</Button>
        <Box>
          <Button onClick={onClose} sx={{ mr: 2 }}>Отмена</Button>
          <Button variant="contained" startIcon={<SaveIcon />} onClick={onSave} disabled={saving}>
            {saving ? 'Сохранение...' : 'Сохранить'}
          </Button>
        </Box>
      </DialogActions>
    </Dialog>
  );
}