import { useState, useEffect, useMemo, useCallback } from 'react';
import {
  Button, Stack, Box, Typography, TextField, Autocomplete, Grid, Paper, Alert, Snackbar,
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow
} from '@mui/material';
import { Save as SaveIcon } from '@mui/icons-material';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import { promoAPI } from '../api/promo';

const MONTH_OPTIONS = [
  { label: 'Январь', value: 1 }, { label: 'Февраль', value: 2 }, { label: 'Март', value: 3 }, { label: 'Апрель', value: 4 },
  { label: 'Май', value: 5 }, { label: 'Июнь', value: 6 }, { label: 'Июль', value: 7 }, { label: 'Август', value: 8 },
  { label: 'Сентябрь', value: 9 }, { label: 'Октябрь', value: 10 }, { label: 'Ноябрь', value: 11 }, { label: 'Декабрь', value: 12 },
];

const ECOM_SEGMENT_OPTIONS = [
  'есть, не убирают из отчета',
  'есть, не убирают из отчета, засчитывается в промо',
  'есть, не убирают из отчетов, не засчитывается в промо',
  'есть, убирают из отчета',
  'нет внешнего е-ком',
  'нет данных',
];

const REQUIRED_FIELDS = [
  'network_name', 'sku', 'year', 'month', 'mechanics', 'gtn_opex', 'contract_price',
  'baseline_units', 'plan_promo_units', 'plan_investments_rub', 'id_directum', 'ds_number',
  'discount_amount', 'conditions', 'ecom_segment', 'total_pharmacies', 'promo_pharmacies'
];

const EMPTY_FORM = {
  id: null, network_name: '', kam: '', brand: '', sku: '',
  year: '', month: '', mechanics: '', gtn_opex: '', baseline_units: '',
  plan_promo_units: '', plan_investments_rub: '', contract_price: '',
  id_directum: '', ds_number: '', discount_amount: '',
  conditions: '', comments: '', ecom_segment: '',
  total_pharmacies: '', promo_pharmacies: '',
  actual_promo_sales_units: '', actual_investments: '', actual_promo_rub: '',
  actual_promo_uplift_units: '', actual_promo_uplift_rub: '',
  actual_external_ecom_units: '', actual_corrected_baseline: '',
  status: 'Планируется',
};

const fmt = (v) => {
  if (v == null || v === '') return '';
  return Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
};

const cleanNumber = (v) => v.replace(/\s/g, '').replace(',', '.');

const requiredLabel = (label) => `${label} *`;

const NumberField = ({ label, value, onChange, ...props }) => (
  <TextField
    label={label}
    type="text"
    size="small"
    fullWidth
    value={value != null && value !== '' ? Number(value).toLocaleString('ru-RU') : ''}
    onChange={(e) => onChange(cleanNumber(e.target.value))}
    slotProps={{ htmlInput: { inputMode: 'decimal' } }}
    {...props}
  />
);

export default function PromoForm({ onSave }) {
  const [form, setForm] = useState({ ...EMPTY_FORM });
  const [allSkuOptions, setAllSkuOptions] = useState([]);
  const [allNetworkOptions, setAllNetworkOptions] = useState([]);
  const [mechanicsOptions, setMechanicsOptions] = useState([]);
  const [investmentTypes, setInvestmentTypes] = useState([]);
  const [history, setHistory] = useState([]);
  const [saving, setSaving] = useState(false);
  const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'success' });
  const [lastSKUData, setLastSKUData] = useState({});
  const [kamOptions, setKamOptions] = useState([]);

  // Загрузка справочников
  useEffect(() => {
    promoAPI.getFilters().then(data => {
      setAllSkuOptions(data.sku || []);
      setAllNetworkOptions(data.network_name || []);
      setMechanicsOptions(data.mechanics || []);
    }).catch(() => {});
    
    promoAPI.getInvestmentTypes().then(data => {
      setInvestmentTypes(data.data || []);
    }).catch(() => setInvestmentTypes(['GTN', 'GTN в ОС', 'OPEX', 'OPEX Marketing']));
  }, []);

  // При выборе SKU
  useEffect(() => {
    if (form.sku) {
      promoAPI.getSKUInfo(form.sku).then(data => {
        if (data.brand) setForm(prev => ({ ...prev, brand: data.brand }));
      }).catch(() => {});
      
      promoAPI.getLastSKUData(form.sku).then(data => setLastSKUData(data)).catch(() => {});
    }
  }, [form.sku]);

  // При выборе сети
  useEffect(() => {
    if (form.network_name) {
      promoAPI.getKAMByNetwork(form.network_name).then(data => {
        setKamOptions(data.data || []);
        if (data.data?.length === 1) setForm(prev => ({ ...prev, kam: data.data[0] }));
      }).catch(() => setKamOptions([]));
      
      promoAPI.getLastNetworkData(form.network_name).then(data => {
        if (data.total_pharmacies) setForm(prev => ({ ...prev, total_pharmacies: data.total_pharmacies }));
      }).catch(() => {});
    }
  }, [form.network_name]);

  // Автозаполнение из lastSKUData
  useEffect(() => {
    if (lastSKUData.contract_price) setForm(prev => ({ ...prev, contract_price: lastSKUData.contract_price }));
  }, [lastSKUData]);

  // История
  const fetchHistory = useCallback(async () => {
    if (!form.network_name || !form.sku || !form.mechanics) return;
    try {
      const data = await promoAPI.getHistory({
        network_name: form.network_name,
        sku: form.sku,
        mechanics: form.mechanics,
      });
      setHistory(data.data || []);
    } catch (e) { setHistory([]); }
  }, [form.network_name, form.sku, form.mechanics]);

  useEffect(() => { fetchHistory(); }, [form.network_name, form.sku, form.mechanics, fetchHistory]);

  // Расчёты
  const calculated = useMemo(() => {
    const ppu = parseFloat(form.plan_promo_units) || 0;
    const cp = parseFloat(form.contract_price) || 0;
    const bu = parseFloat(form.baseline_units) || 0;
    const pir = parseFloat(form.plan_investments_rub) || 0;
    const gm = parseFloat(lastSKUData.gm) || 1;
    const month = parseInt(form.month) || 1;

    const plan_promo_rub = ppu * cp;
    const plan_promo_uplift_units = ppu - bu;
    const plan_promo_uplift_rub = plan_promo_uplift_units * cp;
    const plan_roi = pir > 0 ? ((plan_promo_uplift_rub / pir) * gm * 100 - 100) : 0;
    const promo_date = form.year && form.month ? `${form.year}-${String(month).padStart(2, '0')}-01` : '';

    return { plan_promo_rub, plan_promo_uplift_units, plan_promo_uplift_rub, plan_roi, promo_date };
  }, [form.plan_promo_units, form.contract_price, form.baseline_units, form.plan_investments_rub, form.year, form.month, lastSKUData.gm]);

  const missingFields = REQUIRED_FIELDS.filter(f => !form[f] || form[f] === '');

  const handleSave = async () => {
    if (missingFields.length > 0) {
      setSnackbar({ open: true, message: `⚠️ Заполните: ${missingFields.slice(0, 5).join(', ')}`, severity: 'warning' });
      return;
    }
    setSaving(true);
    try {
      const payload = {
        network_name: form.network_name, kam: form.kam, brand: form.brand, brand_as: form.brand,
        sku: form.sku, year: parseInt(form.year), month: parseInt(form.month),
        mechanics: form.mechanics, gtn_opex: form.gtn_opex,
        baseline_units: parseFloat(form.baseline_units),
        plan_promo_units: parseFloat(form.plan_promo_units),
        plan_promo_rub: calculated.plan_promo_rub,
        plan_promo_uplift_units: calculated.plan_promo_uplift_units,
        plan_promo_uplift_rub: calculated.plan_promo_uplift_rub,
        plan_investments_rub: parseFloat(form.plan_investments_rub) || null,
        plan_roi: calculated.plan_roi,
        contract_price: parseFloat(form.contract_price),
        id_directum: form.id_directum, ds_number: form.ds_number,
        discount_amount: parseFloat(form.discount_amount) || null,
        conditions: form.conditions, comments: form.comments || null,
        ecom_segment: form.ecom_segment,
        total_pharmacies: parseInt(form.total_pharmacies),
        promo_pharmacies: parseInt(form.promo_pharmacies),
        status: form.status, date: calculated.promo_date,
      };

      await promoAPI.save(payload);
      setSnackbar({ open: true, message: '✅ Сохранено', severity: 'success' });
      if (onSave) onSave();
    } catch (err) {
      setSnackbar({ open: true, message: '❌ Ошибка: ' + err.message, severity: 'error' });
    } finally { setSaving(false); }
  };

  const handleReset = () => {
    setForm({ ...EMPTY_FORM });
    setHistory([]);
    setLastSKUData({});
  };

  const updateForm = (field) => (value) => setForm(prev => ({ ...prev, [field]: value }));

  const chartData = useMemo(() => {
    return history.map(row => ({
      period: `${row.year}-${String(row.month).padStart(2, '0')}`,
      plan: row.plan_promo_units || 0,
      fact: row.actual_promo_sales_units || 0,
    })).reverse();
  }, [history]);

  return (
    <Box sx={{ flex: 1, overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
      <Grid container spacing={2} sx={{ flex: 1, overflow: 'hidden' }}>
        <Grid item xs={12} md={6} sx={{ height: '100%', overflow: 'auto' }}>
          <Paper sx={{ p: 2, height: '100%' }}>
            <Typography variant="h6" sx={{ mb: 1 }}>Новое промо</Typography>
            <Grid container spacing={1.5}>
              <Grid item xs={6}>
                <Stack spacing={1.5}>
                  <Autocomplete size="small" freeSolo options={allNetworkOptions} value={form.network_name || ''}
                    onChange={(_, v) => setForm(prev => ({ ...prev, network_name: v || '', kam: '' }))}
                    onInputChange={(_, v) => setForm(prev => ({ ...prev, network_name: v }))}
                    renderInput={(p) => <TextField {...p} label={requiredLabel('Сеть')} size="small" />} />
                  <Autocomplete size="small" freeSolo options={allSkuOptions} value={form.sku || ''}
                    onChange={(_, v) => setForm(prev => ({ ...prev, sku: v || '' }))}
                    onInputChange={(_, v) => setForm(prev => ({ ...prev, sku: v }))}
                    renderInput={(p) => <TextField {...p} label={requiredLabel('SKU')} size="small" />} />
                  <Autocomplete size="small" options={MONTH_OPTIONS} getOptionLabel={o => o.label}
                    value={MONTH_OPTIONS.find(m => m.value === parseInt(form.month)) || null}
                    onChange={(_, v) => setForm(prev => ({ ...prev, month: v?.value || '' }))}
                    renderInput={(p) => <TextField {...p} label={requiredLabel('Месяц')} size="small" />} />
                  <TextField label={requiredLabel('Год')} type="number" size="small" fullWidth value={form.year}
                    onChange={(e) => setForm(prev => ({ ...prev, year: e.target.value }))}
                    slotProps={{ htmlInput: { min: 2020, max: 2030 } }} />
                  <Autocomplete size="small" freeSolo options={mechanicsOptions} value={form.mechanics || ''}
                    onChange={(_, v) => setForm(prev => ({ ...prev, mechanics: v || '' }))}
                    onInputChange={(_, v) => setForm(prev => ({ ...prev, mechanics: v }))}
                    renderInput={(p) => <TextField {...p} label={requiredLabel('Механика')} size="small" />} />
                  <NumberField label={requiredLabel('Сумма скидки')} value={form.discount_amount} onChange={updateForm('discount_amount')} />
                  <Autocomplete size="small" freeSolo options={investmentTypes} value={form.gtn_opex || ''}
                    onChange={(_, v) => setForm(prev => ({ ...prev, gtn_opex: v || '' }))}
                    onInputChange={(_, v) => setForm(prev => ({ ...prev, gtn_opex: v }))}
                    renderInput={(p) => <TextField {...p} label={requiredLabel('Тип инвест.')} size="small" />} />
                  <Autocomplete size="small" options={ECOM_SEGMENT_OPTIONS} value={form.ecom_segment || ''}
                    onChange={(_, v) => setForm(prev => ({ ...prev, ecom_segment: v || '' }))}
                    renderInput={(p) => <TextField {...p} label={requiredLabel('E-com сегмент')} size="small" />} />
                  <NumberField label={requiredLabel('Аптек ТОТАЛ')} value={form.total_pharmacies} onChange={updateForm('total_pharmacies')} />
                  <NumberField label={requiredLabel('Аптек в промо')} value={form.promo_pharmacies} onChange={updateForm('promo_pharmacies')} />
                  <TextField label={requiredLabel('ID Директум')} size="small" fullWidth value={form.id_directum}
                    onChange={(e) => setForm(prev => ({ ...prev, id_directum: e.target.value }))} />
                  <TextField label={requiredLabel('№ ДС')} size="small" fullWidth value={form.ds_number}
                    onChange={(e) => setForm(prev => ({ ...prev, ds_number: e.target.value }))} />
                  <NumberField label={requiredLabel('Цена контракта')} value={form.contract_price} onChange={updateForm('contract_price')} />
                </Stack>
              </Grid>
              <Grid item xs={6}>
                <Stack spacing={1.5}>
                  <TextField label={requiredLabel('Условия')} size="small" fullWidth multiline rows={4} value={form.conditions}
                    onChange={(e) => setForm(prev => ({ ...prev, conditions: e.target.value }))} />
                  <TextField label="Комментарии" size="small" fullWidth multiline rows={3} value={form.comments}
                    onChange={(e) => setForm(prev => ({ ...prev, comments: e.target.value }))} />
                  <Paper variant="outlined" sx={{ p: 1.5, bgcolor: '#f8f9fa' }}>
                    <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1 }}>📊 Baseline и План</Typography>
                    <Stack spacing={1.5}>
                      <NumberField label={requiredLabel('Baseline (уп)')} value={form.baseline_units} onChange={updateForm('baseline_units')} />
                      <NumberField label={requiredLabel('План промо (уп)')} value={form.plan_promo_units} onChange={updateForm('plan_promo_units')} />
                      <TextField label="План (руб)" size="small" fullWidth value={fmt(calculated.plan_promo_rub)} slotProps={{ input: { readOnly: true } }} />
                      <NumberField label={requiredLabel('Инвестиции (руб)')} value={form.plan_investments_rub} onChange={updateForm('plan_investments_rub')} />
                      <TextField label="Uplift (уп)" size="small" fullWidth value={fmt(calculated.plan_promo_uplift_units)} slotProps={{ input: { readOnly: true } }} />
                      <TextField label="Uplift (руб)" size="small" fullWidth value={fmt(calculated.plan_promo_uplift_rub)} slotProps={{ input: { readOnly: true } }} />
                      <TextField label="ROI план %" size="small" fullWidth value={calculated.plan_roi.toFixed(1)} slotProps={{ input: { readOnly: true } }} />
                    </Stack>
                  </Paper>
                  <Stack direction="row" spacing={1}>
                    <Button variant="contained" startIcon={<SaveIcon />} onClick={handleSave} disabled={saving} fullWidth size="small">
                      {saving ? 'Сохранение...' : 'Сохранить'}
                    </Button>
                    <Button variant="outlined" onClick={handleReset} size="small" sx={{ minWidth: 90 }}>Сброс</Button>
                  </Stack>
                </Stack>
              </Grid>
            </Grid>
          </Paper>
        </Grid>
        <Grid item xs={12} md={6} sx={{ height: '100%' }}>
          <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%', gap: 1 }}>
            <Paper sx={{ p: 2, flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 1 }}>
                📋 История: {form.network_name || 'сеть'} / {form.sku || 'SKU'} / {form.mechanics || 'механика'}
              </Typography>
              <TableContainer sx={{ flex: 1 }}>
                <Table size="small" stickyHeader>
                  <TableHead>
                    <TableRow>
                      <TableCell>Период</TableCell>
                      <TableCell align="right">Baseline</TableCell>
                      <TableCell align="right">План (уп)</TableCell>
                      <TableCell align="right">Факт (уп)</TableCell>
                      <TableCell align="right">Uplift план</TableCell>
                      <TableCell align="right">Uplift факт</TableCell>
                      <TableCell align="right">ROI план %</TableCell>
                      <TableCell align="right">ROI факт %</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {history.map((row) => (
                      <TableRow key={row.id} hover>
                        <TableCell>{row.year}/{String(row.month).padStart(2, '0')}</TableCell>
                        <TableCell align="right">{row.baseline_units != null ? Number(row.baseline_units).toLocaleString('ru-RU') : '-'}</TableCell>
                        <TableCell align="right">{row.plan_promo_units != null ? Number(row.plan_promo_units).toLocaleString('ru-RU') : '-'}</TableCell>
                        <TableCell align="right">{row.actual_promo_sales_units != null ? Number(row.actual_promo_sales_units).toLocaleString('ru-RU') : '-'}</TableCell>
                        <TableCell align="right">{row.plan_promo_uplift_units != null ? Number(row.plan_promo_uplift_units).toLocaleString('ru-RU') : '-'}</TableCell>
                        <TableCell align="right">{row.actual_promo_uplift_units != null ? Number(row.actual_promo_uplift_units).toLocaleString('ru-RU') : '-'}</TableCell>
                        <TableCell align="right">{row.plan_roi != null ? Number(row.plan_roi).toFixed(1) : '-'}</TableCell>
                        <TableCell align="right">{row.actual_roi != null ? Number(row.actual_roi).toFixed(1) : '-'}</TableCell>
                      </TableRow>
                    ))}
                    {history.length === 0 && (
                      <TableRow><TableCell colSpan={8} align="center">Выберите сеть, SKU и механику</TableCell></TableRow>
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </Paper>
            <Paper sx={{ p: 2, height: 250, flexShrink: 0 }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 1 }}>📈 План / Факт</Typography>
              {history.length > 0 ? (
                <Box sx={{ height: 190 }}>
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={chartData}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="period" tick={{ fontSize: 10 }} />
                      <YAxis tick={{ fontSize: 10 }} />
                      <Tooltip formatter={(v) => Number(v).toLocaleString('ru-RU')} />
                      <Legend wrapperStyle={{ fontSize: 11 }} />
                      <Bar dataKey="plan" name="План (уп)" fill="#8884d8" radius={[3, 3, 0, 0]} />
                      <Bar dataKey="fact" name="Факт (уп)" fill="#82ca9d" radius={[3, 3, 0, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                </Box>
              ) : (
                <Box sx={{ height: 190, display: 'flex', alignItems: 'center', justifyContent: 'center', bgcolor: '#fafafa', borderRadius: 1 }}>
                  <Typography color="text.disabled">Нет данных</Typography>
                </Box>
              )}
            </Paper>
          </Box>
        </Grid>
      </Grid>
      <Snackbar open={snackbar.open} autoHideDuration={4000} onClose={() => setSnackbar(s => ({ ...s, open: false }))}>
        <Alert severity={snackbar.severity} onClose={() => setSnackbar(s => ({ ...s, open: false }))}>{snackbar.message}</Alert>
      </Snackbar>
    </Box>
  );
}