import { useState, useEffect, useMemo } from 'react';
import {
  Button, Stack, Box, Typography, TextField, Autocomplete, Grid, Paper, Alert, Snackbar,
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow, CircularProgress,
  ToggleButton, ToggleButtonGroup,
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

const FIELD_LABELS = {
  network_name: 'Сеть', sku: 'SKU', year: 'Год', month: 'Месяц', mechanics: 'Механика',
  gtn_opex: 'Тип инвестиций', contract_price: 'Цена контракта', baseline_units: 'Baseline (уп)',
  plan_promo_units: 'План промо (уп)', plan_investments_rub: 'Инвестиции (руб)',
  id_directum: 'ID Директум', ds_number: '№ ДС', discount_amount: 'Сумма скидки',
  conditions: 'Условия', ecom_segment: 'E-com сегмент', total_pharmacies: 'Аптек всего',
  promo_pharmacies: 'Аптек в промо',
};

const EMPTY_FORM = {
  id: null, network_name: '', kam: '', brand: '', sku: '',
  year: '2027', month: '', mechanics: '', gtn_opex: '', baseline_units: '',
  plan_promo_units: '', plan_investments_rub: '', contract_price: '',
  id_directum: '', ds_number: '', discount_amount: '',
  conditions: '', comments: '', ecom_segment: '',
  total_pharmacies: '', promo_pharmacies: '',
  actual_promo_sales_units: '', actual_investments: '', actual_promo_rub: '',
  actual_promo_uplift_units: '', actual_promo_uplift_rub: '',
  actual_external_ecom_units: '', actual_corrected_baseline: '',
  key_region: '', top20_segment: '',
  status: 'Планируется',
};

const fmt = (v) => {
  if (v == null || v === '') return '';
  return Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
};

const historyValue = (value, fractionDigits = 0) => value != null
  ? Number(value).toLocaleString('ru-RU', { minimumFractionDigits: fractionDigits, maximumFractionDigits: fractionDigits })
  : '—';

const cleanNumber = (v) => v.replace(/\s/g, '').replace(',', '.');

const safeNumber = (val) => {
  const n = parseInt(val);
  return isNaN(n) ? null : n;
};

const safeFloatNull = (val) => {
  const n = parseFloat(val);
  return isNaN(n) ? null : n;
};

const requiredLabel = (label) => `${label} *`;

const NumberField = ({ label, value, onChange, ...props }) => (
  <TextField
    label={label}
    type="text"
    size="small"
    fullWidth
    value={value ?? ''}
    onChange={(e) => {
      const nextValue = cleanNumber(e.target.value);
      if (/^-?\d*(\.\d*)?$/.test(nextValue)) onChange(nextValue);
    }}
    slotProps={{ htmlInput: { inputMode: 'decimal' } }}
    {...props}
  />
);

const HistoryPair = ({ plan, fact, fractionDigits = 0, suffix = '' }) => (
  <Box sx={{ display: 'grid', gap: 0.35, minWidth: 82 }}>
    <Box sx={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 0.5, px: 0.6, py: 0.35, borderRadius: 1, bgcolor: '#eef2ff' }}>
      <Typography variant="caption" color="text.secondary" sx={{ lineHeight: 1.1 }}>План</Typography>
      <Typography variant="caption" sx={{ fontWeight: 700, lineHeight: 1.1, whiteSpace: 'nowrap' }}>
        {historyValue(plan, fractionDigits)}{suffix}
      </Typography>
    </Box>
    <Box sx={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 0.5, px: 0.6, py: 0.35, borderRadius: 1, bgcolor: '#f0fdf4' }}>
      <Typography variant="caption" color="text.secondary" sx={{ lineHeight: 1.1 }}>Факт</Typography>
      <Typography variant="caption" sx={{ fontWeight: 700, lineHeight: 1.1, whiteSpace: 'nowrap' }}>
        {historyValue(fact, fractionDigits)}{suffix}
      </Typography>
    </Box>
  </Box>
);

export default function PromoForm({ onSave, onOpenPromo }) {
  const [form, setForm] = useState({ ...EMPTY_FORM });
  const [allSkuOptions, setAllSkuOptions] = useState([]);
  const [allNetworkOptions, setAllNetworkOptions] = useState([]);
  const [mechanicsOptions, setMechanicsOptions] = useState([]);
  const [investmentTypes, setInvestmentTypes] = useState([]);
  const [history, setHistory] = useState([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyMetric, setHistoryMetric] = useState('units');
  const [saving, setSaving] = useState(false);
  const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'success' });
  const [lastSKUData, setLastSKUData] = useState({});
  const [skuDataLoading, setSkuDataLoading] = useState(false);
  const [manualOverrides, setManualOverrides] = useState({ contract_price: false, total_pharmacies: false });

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

  // Данные SKU и сети подтягиваем после короткой паузы: это исключает запросы на каждый символ
  // и не даёт опоздавшему ответу перезаписать уже выбранное значение.
  useEffect(() => {
    const sku = form.sku.trim();
    if (!sku) {
      setLastSKUData({});
      setSkuDataLoading(false);
      return undefined;
    }

    let active = true;
    setSkuDataLoading(true);
    const timer = window.setTimeout(async () => {
      const [skuInfo, lastData] = await Promise.all([
        promoAPI.getSKUInfo(sku).catch(() => ({})),
        promoAPI.getLastSKUData(sku).catch(() => ({})),
      ]);
      if (!active) return;

      setLastSKUData(lastData || {});
      setSkuDataLoading(false);
      setForm(prev => {
        if (prev.sku !== sku) return prev;
        const updates = {};
        if (skuInfo.brand) updates.brand = skuInfo.brand;
        if (!manualOverrides.contract_price && lastData.contract_price != null) updates.contract_price = String(lastData.contract_price);
        return Object.keys(updates).length > 0 ? { ...prev, ...updates } : prev;
      });
    }, 300);

    return () => {
      active = false;
      window.clearTimeout(timer);
    };
  }, [form.sku, manualOverrides.contract_price]);

  useEffect(() => {
    const network = form.network_name.trim();
    if (!network) return undefined;

    let active = true;
    const timer = window.setTimeout(async () => {
      const [geo, lastNetworkData] = await Promise.all([
        promoAPI.getNetworkGeo(network).catch(() => ({})),
        promoAPI.getLastNetworkData(network).catch(() => ({})),
      ]);
      if (!active) return;

      setForm(prev => {
        if (prev.network_name !== network) return prev;
        const updates = {};
        if (geo.kam) updates.kam = geo.kam;
        if (geo.key_region) updates.key_region = geo.key_region;
        if (geo.top20_segment) updates.top20_segment = geo.top20_segment;
        if (!manualOverrides.total_pharmacies && lastNetworkData.total_pharmacies != null) {
          updates.total_pharmacies = String(lastNetworkData.total_pharmacies);
        }
        return Object.keys(updates).length > 0 ? { ...prev, ...updates } : prev;
      });
    }, 300);

    return () => {
      active = false;
      window.clearTimeout(timer);
    };
  }, [form.network_name, manualOverrides.total_pharmacies]);

  // История
  const historySelectionComplete = Boolean(form.network_name && form.sku && form.mechanics);
  useEffect(() => {
    if (!historySelectionComplete) {
      setHistory([]);
      setHistoryLoading(false);
      return undefined;
    }

    let active = true;
    setHistory([]);
    setHistoryLoading(true);
    promoAPI.getHistory({
      network_name: form.network_name,
      sku: form.sku,
      mechanics: form.mechanics,
    }).then(data => {
      if (active) setHistory(data.data || []);
    }).catch(() => {
      if (active) setHistory([]);
    }).finally(() => {
      if (active) setHistoryLoading(false);
    });

    return () => { active = false; };
  }, [form.network_name, form.sku, form.mechanics, historySelectionComplete]);

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
  const missingFieldLabels = missingFields.map(field => FIELD_LABELS[field] || field);

  const handleSave = async () => {
    if (missingFields.length > 0) {
      setSnackbar({ open: true, message: `⚠️ Заполните: ${missingFieldLabels.slice(0, 5).join(', ')}`, severity: 'warning' });
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
        conditions: form.conditions, comments: form.comments ?? null,
        ecom_segment: form.ecom_segment,
        total_pharmacies: safeNumber(form.total_pharmacies),
        promo_pharmacies: safeNumber(form.promo_pharmacies),
        key_region: form.key_region || null,
        top20_segment: form.top20_segment || null,
        actual_promo_sales_units: parseFloat(form.actual_promo_sales_units) || null,
        actual_investments: parseFloat(form.actual_investments) || null,
        actual_promo_rub: parseFloat(form.actual_promo_rub) || null,
        actual_promo_uplift_units: parseFloat(form.actual_promo_uplift_units) || null,
        actual_promo_uplift_rub: parseFloat(form.actual_promo_uplift_rub) || null,
        actual_external_ecom_units: safeFloatNull(form.actual_external_ecom_units),
        actual_corrected_baseline: safeFloatNull(form.actual_corrected_baseline),
        status: form.status, date: calculated.promo_date,
      };

      await promoAPI.save(payload);
      handleReset();
      setSnackbar({ open: true, message: '✅ Промо создано. Форма очищена для следующей записи.', severity: 'success' });
      if (onSave) onSave();
    } catch (err) {
      setSnackbar({ open: true, message: '❌ Ошибка: ' + err.message, severity: 'error' });
    } finally { setSaving(false); }
  };

  const handleReset = () => {
    setForm({ ...EMPTY_FORM });
    setHistory([]);
    setLastSKUData({});
    setSkuDataLoading(false);
    setManualOverrides({ contract_price: false, total_pharmacies: false });
  };

  const updateForm = (field) => (value) => setForm(prev => ({ ...prev, [field]: value }));

  const updateManualNumber = (field) => (value) => {
    setManualOverrides(prev => ({ ...prev, [field]: true }));
    setForm(prev => ({ ...prev, [field]: value }));
  };

  const chartData = useMemo(() => {
    return history.map(row => ({
      period: `${row.year}-${String(row.month).padStart(2, '0')}`,
      plan: row.plan_promo_units || 0,
      fact: row.actual_promo_sales_units || 0,
      planInvestments: row.plan_investments_rub || 0,
      factInvestments: row.actual_investments || 0,
      planRoi: row.plan_roi || 0,
      factRoi: row.actual_roi || 0,
    })).reverse();
  }, [history]);

  const chartConfig = {
    units: { planKey: 'plan', factKey: 'fact', planLabel: 'План (уп)', factLabel: 'Факт (уп)' },
    investments: { planKey: 'planInvestments', factKey: 'factInvestments', planLabel: 'Инвестиции план (руб)', factLabel: 'Инвестиции факт (руб)' },
    roi: { planKey: 'planRoi', factKey: 'factRoi', planLabel: 'ROI план (%)', factLabel: 'ROI факт (%)' },
  }[historyMetric];

  return (
    <Box sx={{ flex: 1, minHeight: 0, overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
      <Grid container spacing={2} sx={{ flex: 1, minHeight: 0, overflow: 'hidden', alignItems: 'stretch' }}>
        <Grid size={{ xs: 12, md: 6 }} sx={{ display: 'flex', minHeight: 0 }}>
          <Paper sx={{ p: 2, flex: 1, minHeight: 0, overflowY: 'auto' }}>
            <Grid container spacing={1.5}>
              <Grid size={6}>
                <Stack spacing={1.5}>
                  <Typography variant="overline" color="text.secondary" sx={{ fontWeight: 700 }}>Идентификация</Typography>
                  <Autocomplete size="small" freeSolo options={allNetworkOptions} value={form.network_name || ''}
                    onChange={(_, v) => {
                      setManualOverrides(prev => ({ ...prev, total_pharmacies: false }));
                      setForm(prev => ({ ...prev, network_name: v || '', kam: '' }));
                    }}
                    onInputChange={(_, v) => {
                      setManualOverrides(prev => ({ ...prev, total_pharmacies: false }));
                      setForm(prev => ({ ...prev, network_name: v }));
                    }}
                    renderInput={(p) => <TextField {...p} label={requiredLabel('Сеть')} size="small" />} />
                  <Autocomplete size="small" freeSolo options={allSkuOptions} value={form.sku || ''}
                    onChange={(_, v) => {
                      setManualOverrides(prev => ({ ...prev, contract_price: false }));
                      setForm(prev => ({ ...prev, sku: v || '' }));
                    }}
                    onInputChange={(_, v) => {
                      setManualOverrides(prev => ({ ...prev, contract_price: false }));
                      setForm(prev => ({ ...prev, sku: v }));
                    }}
                    renderInput={(p) => <TextField {...p} label={requiredLabel('SKU')} size="small" />} />
                  <Typography variant="overline" color="text.secondary" sx={{ fontWeight: 700, mt: 0.5 }}>Параметры промо</Typography>
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

                  <TextField label={requiredLabel('Условия')} size="small" fullWidth multiline rows={3} value={form.conditions}
                    onChange={(e) => setForm(prev => ({ ...prev, conditions: e.target.value }))} />

                  <Typography variant="overline" color="text.secondary" sx={{ fontWeight: 700, mt: 0.5 }}>Коммерческие параметры</Typography>
                  <NumberField label={requiredLabel('Сумма скидки')} value={form.discount_amount} onChange={updateForm('discount_amount')} />
                  <Autocomplete size="small" freeSolo options={investmentTypes} value={form.gtn_opex || ''}
                    onChange={(_, v) => setForm(prev => ({ ...prev, gtn_opex: v || '' }))}
                    onInputChange={(_, v) => setForm(prev => ({ ...prev, gtn_opex: v }))}
                    renderInput={(p) => <TextField {...p} label={requiredLabel('Тип инвест.')} size="small" />} />
                  <Autocomplete size="small" options={ECOM_SEGMENT_OPTIONS} value={form.ecom_segment || ''}
                    onChange={(_, v) => setForm(prev => ({ ...prev, ecom_segment: v || '' }))}
                    renderInput={(p) => <TextField {...p} label={requiredLabel('E-com сегмент')} size="small" />} />
                  <NumberField label={requiredLabel('Аптек всего')} value={form.total_pharmacies} onChange={updateManualNumber('total_pharmacies')} />
                  <NumberField label={requiredLabel('Аптек в промо')} value={form.promo_pharmacies} onChange={updateForm('promo_pharmacies')} />

                  <Typography variant="overline" color="text.secondary" sx={{ fontWeight: 700, mt: 0.5 }}>Документы</Typography>
                  <TextField label={requiredLabel('ID Директум')} size="small" fullWidth value={form.id_directum}
                    onChange={(e) => setForm(prev => ({ ...prev, id_directum: e.target.value }))} />
                  <TextField label={requiredLabel('№ ДС')} size="small" fullWidth value={form.ds_number}
                    onChange={(e) => setForm(prev => ({ ...prev, ds_number: e.target.value }))} />
                </Stack>
              </Grid>
              <Grid size={6}>
                <Stack spacing={1.5}>
                  <TextField label="Комментарии" size="small" fullWidth multiline rows={3} value={form.comments}
                    onChange={(e) => setForm(prev => ({ ...prev, comments: e.target.value }))} />
                  <Paper variant="outlined" sx={{ p: 1.5, bgcolor: '#f8f9fa' }}>
                    <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1 }}>📊 Baseline и План</Typography>
                    <Stack spacing={1.5}>
                      <NumberField label={requiredLabel('Цена контракта')} value={form.contract_price} onChange={updateManualNumber('contract_price')} />
                      <NumberField label={requiredLabel('Baseline (уп)')} value={form.baseline_units} onChange={updateForm('baseline_units')} />
                      <NumberField label={requiredLabel('План промо (уп)')} value={form.plan_promo_units} onChange={updateForm('plan_promo_units')} />
                      <TextField label="План (руб)" size="small" fullWidth value={fmt(calculated.plan_promo_rub)} slotProps={{ input: { readOnly: true } }} />
                      <NumberField label={requiredLabel('Инвестиции (руб)')} value={form.plan_investments_rub} onChange={updateForm('plan_investments_rub')} />
                      <TextField label="Uplift (уп)" size="small" fullWidth value={fmt(calculated.plan_promo_uplift_units)} slotProps={{ input: { readOnly: true } }} />
                      <TextField label="Uplift (руб)" size="small" fullWidth value={fmt(calculated.plan_promo_uplift_rub)} slotProps={{ input: { readOnly: true } }} />
                      <TextField label="ROI план %" size="small" fullWidth value={calculated.plan_roi.toFixed(1)} slotProps={{ input: { readOnly: true } }} />
                      {form.sku && !skuDataLoading && !lastSKUData.gm && (
                        <Alert severity="warning" sx={{ py: 0.25 }}>
                          GM для SKU не найден. ROI рассчитан предварительно с GM = 1.
                        </Alert>
                      )}
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
        <Grid size={{ xs: 12, md: 6 }} sx={{ display: 'flex', minHeight: 0 }}>
          <Box sx={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0, gap: 1 }}>
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
                      <TableCell>Продажи, уп.</TableCell>
                      <TableCell>Инвестиции, руб.</TableCell>
                      <TableCell>Uplift, уп.</TableCell>
                      <TableCell>ROI, %</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {history.map((row) => (
                      <TableRow key={row.id} hover tabIndex={0}
                        onClick={() => onOpenPromo?.(row.id)}
                        onKeyDown={(event) => {
                          if (event.key === 'Enter' || event.key === ' ') {
                            event.preventDefault();
                            onOpenPromo?.(row.id);
                          }
                        }}
                        sx={{ cursor: onOpenPromo ? 'pointer' : 'default' }}
                        aria-label={`Открыть промо за ${row.year}/${String(row.month).padStart(2, '0')}`}>
                        <TableCell>{row.year}/{String(row.month).padStart(2, '0')}</TableCell>
                        <TableCell align="right">{historyValue(row.baseline_units)}</TableCell>
                        <TableCell sx={{ px: 0.75, py: 0.75 }}><HistoryPair plan={row.plan_promo_units} fact={row.actual_promo_sales_units} /></TableCell>
                        <TableCell sx={{ px: 0.75, py: 0.75 }}><HistoryPair plan={row.plan_investments_rub} fact={row.actual_investments} fractionDigits={2} /></TableCell>
                        <TableCell sx={{ px: 0.75, py: 0.75 }}><HistoryPair plan={row.plan_promo_uplift_units} fact={row.actual_promo_uplift_units} /></TableCell>
                        <TableCell sx={{ px: 0.75, py: 0.75 }}><HistoryPair plan={row.plan_roi} fact={row.actual_roi} fractionDigits={1} suffix="%" /></TableCell>
                      </TableRow>
                    ))}
                    {historyLoading && (
                      <TableRow><TableCell colSpan={6} align="center"><CircularProgress size={20} /></TableCell></TableRow>
                    )}
                    {!historyLoading && history.length === 0 && (
                      <TableRow><TableCell colSpan={6} align="center">
                        {historySelectionComplete ? 'По выбранной связке истории нет.' : 'Выберите сеть, SKU и механику.'}
                      </TableCell></TableRow>
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </Paper>
            <Paper sx={{ p: 2, height: 286, flexShrink: 0 }}>
              <Stack direction="row" sx={{ alignItems: 'center', justifyContent: 'space-between', mb: 1 }}>
                <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>📈 Динамика</Typography>
                <ToggleButtonGroup size="small" exclusive value={historyMetric} onChange={(_, value) => value && setHistoryMetric(value)}>
                  <ToggleButton value="units">Уп.</ToggleButton>
                  <ToggleButton value="investments">Руб.</ToggleButton>
                  <ToggleButton value="roi">ROI</ToggleButton>
                </ToggleButtonGroup>
              </Stack>
              {history.length > 0 ? (
                <Box sx={{ height: 220 }}>
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={chartData}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="period" tick={{ fontSize: 10 }} />
                      <YAxis tick={{ fontSize: 10 }} />
                      <Tooltip formatter={(v) => Number(v).toLocaleString('ru-RU')} />
                      <Legend wrapperStyle={{ fontSize: 11 }} />
                      <Bar dataKey={chartConfig.planKey} name={chartConfig.planLabel} fill="#8884d8" radius={[3, 3, 0, 0]} />
                      <Bar dataKey={chartConfig.factKey} name={chartConfig.factLabel} fill="#82ca9d" radius={[3, 3, 0, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                </Box>
              ) : (
                <Box sx={{ height: 220, display: 'flex', alignItems: 'center', justifyContent: 'center', bgcolor: '#fafafa', borderRadius: 1 }}>
                  <Typography color="text.disabled">{historyLoading ? 'Загружаем историю…' : 'Нет данных'}</Typography>
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
