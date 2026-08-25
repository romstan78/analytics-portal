import { type ChangeEvent, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Drawer,
  FormControl,
  FormControlLabel,
  IconButton,
  Paper,
  Radio,
  RadioGroup,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@mui/material';
import {
  Close as CloseIcon,
  DeleteSweepOutlined as ClearIcon,
  Inventory2Outlined as SKUIcon,
  Save as SaveIcon,
  UploadFileOutlined as ImportIcon,
} from '@mui/icons-material';
import { networkAPI } from '../api/networks';
import type {
  NetworkForecastInput,
  NetworkForecastClearScope,
  NetworkForecastImportPreview,
  NetworkForecastMonth,
  NetworkForecastResponse,
  NetworkForecastSaveRequest,
} from '../types/network';
import {
  formatNumberInput,
  formatRub,
  formatRubShort,
  parseNumberInput,
} from '../utils/networkPlan';

const QUARTERS = [1, 2, 3, 4] as const;
const MONTHS = [
  'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
];

type Measure = 'volume' | 'investments';
type Unit = 'rub' | 'units';

interface ForecastDraft {
  rub: string;
  units: string;
  investments: string;
  reason: string;
}

interface Props {
  networkId: number;
  year: number;
  canEdit: boolean;
}

const forecastKey = (month: number, brand: string, sku: string | null): string =>
  `${month}|${brand}|${sku ?? ''}`;

const asInput = (value: number | null): string => value == null ? '' : formatNumberInput(String(value));

const draftFromRows = (rows: NetworkForecastMonth[]): Record<string, ForecastDraft> =>
  Object.fromEntries(rows.map((row) => [forecastKey(row.month, row.brand_as, row.sku), {
    rub: asInput(row.forecast_rub),
    units: asInput(row.forecast_units),
    investments: asInput(row.forecast_investments_rub),
    reason: row.adjustment_reason ?? '',
  }]));

const sameDraft = (left: ForecastDraft | undefined, right: ForecastDraft | undefined): boolean =>
  left?.rub === right?.rub
  && left?.units === right?.units
  && left?.investments === right?.investments
  && left?.reason === right?.reason;

const monthLabel = (month: number, row?: NetworkForecastMonth): string => {
  if (row?.is_closed) return `${MONTHS[month - 1]} · закрыт`;
  if (row?.is_current) return `${MONTHS[month - 1]} · текущий`;
  return MONTHS[month - 1];
};

function SummaryTile({ label, value, hint, warning = false }: {
  label: string;
  value: string;
  hint: string;
  warning?: boolean;
}) {
  return (
    <Paper variant="outlined" sx={{ p: 1.25, minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary">{label}</Typography>
      <Typography variant="h6" sx={{ mt: 0.25, fontVariantNumeric: 'tabular-nums' }}>{value}</Typography>
      <Typography variant="caption" color={warning ? 'warning.main' : 'text.secondary'}>{hint}</Typography>
    </Paper>
  );
}

export default function NetworkForecastTab({ networkId, year, canEdit }: Props) {
  const now = new Date();
  const defaultQuarter = year === now.getFullYear() ? Math.floor(now.getMonth() / 3) + 1 : 1;
  const [quarter, setQuarter] = useState(defaultQuarter);
  const [measure, setMeasure] = useState<Measure>('volume');
  const [unit, setUnit] = useState<Unit>('rub');
  const [draftEdits, setDraftEdits] = useState<Record<string, ForecastDraft> | null>(null);
  const [selectedBrand, setSelectedBrand] = useState<string | null>(null);
  const [importFile, setImportFile] = useState<File | null>(null);
  const [importPreview, setImportPreview] = useState<NetworkForecastImportPreview | null>(null);
  const [importDialogOpen, setImportDialogOpen] = useState(false);
  const [clearMonth, setClearMonth] = useState<number | null>(null);
  const [clearScope, setClearScope] = useState<NetworkForecastClearScope>('all');
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: ['network-forecast', networkId, year, quarter],
    queryFn: () => networkAPI.getForecast(networkId, year, quarter),
  });

  const baseDraft = useMemo(() => draftFromRows(query.data?.months ?? []), [query.data]);
  const draft = draftEdits ?? baseDraft;
  const dirty = draftEdits != null
    && Object.keys(draftEdits).some((key) => !sameDraft(draftEdits[key], baseDraft[key]));

  const applyForecastData = (data: NetworkForecastResponse) => {
    queryClient.setQueryData(['network-forecast', networkId, year, quarter], data);
    void queryClient.invalidateQueries({ queryKey: ['networkPlan', networkId, year] });
    void queryClient.invalidateQueries({ queryKey: ['networkAudit', networkId] });
    setDraftEdits(null);
  };

  const saveMutation = useMutation({
    mutationFn: (request: NetworkForecastSaveRequest) => networkAPI.saveForecast(networkId, request),
    onSuccess: (response) => {
      applyForecastData(response.data);
    },
  });

  const importPreviewMutation = useMutation({
    mutationFn: (file: File) => networkAPI.previewForecastImport(networkId, year, quarter, file),
    onSuccess: (preview, file) => {
      setImportFile(file);
      setImportPreview(preview);
      setImportDialogOpen(true);
    },
  });

  const importMutation = useMutation({
    mutationFn: (file: File) => networkAPI.importForecast(networkId, year, quarter, file),
    onSuccess: (response) => {
      applyForecastData(response.data);
      setImportDialogOpen(false);
      setImportFile(null);
      setImportPreview(null);
    },
  });

  const clearMutation = useMutation({
    mutationFn: (request: { month: number; scope: NetworkForecastClearScope }) =>
      networkAPI.clearForecastMonth(networkId, { year, ...request }),
    onSuccess: (response) => {
      applyForecastData(response.data);
      setClearMonth(null);
    },
  });

  const monthNumbers = useMemo(() => {
    const start = (quarter - 1) * 3 + 1;
    return [start, start + 1, start + 2];
  }, [quarter]);

  const brandRows = useMemo(() => (query.data?.months ?? []).filter((row) => row.sku == null), [query.data]);
  const skuRows = useMemo(() => (query.data?.months ?? []).filter((row) => row.sku != null), [query.data]);
  const rowsByKey = useMemo(() => new Map((query.data?.months ?? []).map((row) => [
    forecastKey(row.month, row.brand_as, row.sku), row,
  ])), [query.data]);

  const updateDraft = (row: NetworkForecastMonth, patch: Partial<ForecastDraft>) => {
    const key = forecastKey(row.month, row.brand_as, row.sku);
    setDraftEdits((current) => ({
      ...(current ?? baseDraft),
      [key]: { ...((current ?? baseDraft)[key] ?? { rub: '', units: '', investments: '', reason: '' }), ...patch },
    }));
  };

  const save = () => {
    const lines: NetworkForecastInput[] = Object.entries(draft).flatMap(([key, value]) => {
      const row = rowsByKey.get(key);
      if (!row || sameDraft(value, baseDraft[key])) return [];
      const rub = parseNumberInput(value.rub);
      const units = parseNumberInput(value.units);
      const investments = parseNumberInput(value.investments);
      return [{
        month: row.month,
        brand_as: row.brand_as,
        sku: row.sku,
        forecast_rub: rub,
        forecast_units: units,
        forecast_investments_rub: investments,
        adjustment_reason: value.reason.trim() || null,
        updated_at: row.updated_at,
      }];
    });
    saveMutation.mutate({ year, quarter, lines });
  };

  const selectImportFile = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;
    setImportFile(file);
    setImportPreview(null);
    importPreviewMutation.mutate(file);
  };

  if (query.isLoading) return <Box sx={{ p: 4, textAlign: 'center' }}><CircularProgress /></Box>;
  if (query.isError || !query.data) return <Alert severity="error">Не удалось загрузить прогноз.</Alert>;

  const { totals } = query.data;
  const completion = totals.completion_pct == null ? '—' : `${totals.completion_pct.toLocaleString('ru-RU', { maximumFractionDigits: 1 })}%`;
  const investPct = totals.plan_investments_rub > 0
    ? (totals.investment_variance_rub / totals.plan_investments_rub) * 100
    : null;
  const clearAffectedRows = clearMonth == null ? 0 : query.data.months.filter((row) => {
    if (row.month !== clearMonth) return false;
    if (clearScope === 'rub') return row.forecast_rub != null;
    if (clearScope === 'units') return row.forecast_units != null;
    return row.forecast_rub != null || row.forecast_units != null;
  }).length;

  const renderMonthCell = (row: NetworkForecastMonth | undefined) => {
    if (!row) return <Typography color="text.disabled">—</Typography>;
    const value = draft[forecastKey(row.month, row.brand_as, row.sku)] ?? { rub: '', units: '', investments: '', reason: '' };
    const promoTitle = [
      `${row.promo_count} промо`,
      `согласовано: ${row.approved_promo_count}`,
      `не согласовано: ${row.draft_promo_count}`,
      `uplift: ${formatRubShort(row.promo_uplift_rub)} ₽`,
      `инвестиции: ${formatRubShort(row.promo_investments_rub)} ₽`,
    ].join(' · ');

    if (measure === 'investments') {
      return (
        <Box sx={{ minWidth: 132 }}>
          <Typography variant="caption" color="text.secondary">П {formatRubShort(row.plan_investments_rub)} ₽</Typography>
          <Typography variant="caption" sx={{ display: 'block' }}>Ф {formatRubShort(row.fact_investments_rub)} ₽</Typography>
          {row.is_closed ? (
            <Typography variant="body2" sx={{ mt: 0.5, fontWeight: 600 }}>{formatRubShort(row.eac_investments_rub)} ₽</Typography>
          ) : (
            <TextField
              size="small"
              value={value.investments}
              disabled={!canEdit}
              placeholder={row.eac_investments_rub == null ? 'Прогноз' : formatRub(row.eac_investments_rub)}
              onChange={(event) => updateDraft(row, { investments: event.target.value })}
              sx={{ mt: 0.5, width: 128 }}
              slotProps={{ htmlInput: { inputMode: 'decimal', 'aria-label': `Прогноз инвестиций ${row.brand_as} ${MONTHS[row.month - 1]}` } }}
            />
          )}
        </Box>
      );
    }

    const isRub = unit === 'rub';
    const plan = isRub ? row.plan_rub : null;
    const fact = isRub ? row.fact_rub : row.fact_units;
    const forecast = isRub ? value.rub : value.units;
    const system = isRub ? row.system_forecast_rub : row.system_forecast_units;
    return (
      <Box sx={{ minWidth: 138 }}>
        {isRub && <Typography variant="caption" color="text.secondary">П {formatRubShort(plan)} ₽</Typography>}
        <Typography variant="caption" sx={{ display: 'block' }}>
          Ф {fact == null ? '—' : isRub ? `${formatRubShort(fact)} ₽` : `${formatRub(fact)} уп.`}
        </Typography>
        {row.is_closed ? (
          <Typography variant="body2" sx={{ mt: 0.5, fontWeight: 600 }}>
            {fact == null ? 'Факт не загружен' : isRub ? `${formatRubShort(fact)} ₽` : `${formatRub(fact)} уп.`}
          </Typography>
        ) : (
          <TextField
            size="small"
            value={forecast}
            disabled={!canEdit}
            placeholder={system == null ? 'Прогноз' : formatRub(system)}
            helperText={system == null ? 'нет базы' : `рекомендация ${isRub ? `${formatRubShort(system)} ₽` : `${formatRub(system)} уп.`}`}
            onChange={(event) => updateDraft(row, isRub ? { rub: event.target.value } : { units: event.target.value })}
            sx={{ mt: 0.5, width: 138 }}
            slotProps={{ htmlInput: { inputMode: 'decimal', 'aria-label': `Прогноз ${row.brand_as} ${MONTHS[row.month - 1]}` } }}
          />
        )}
        {row.promo_count > 0 && (
          <Tooltip title={promoTitle}>
            <Chip
              size="small"
              color={row.approved_promo_count > 0 ? 'warning' : 'default'}
              variant={row.approved_promo_count > 0 ? 'filled' : 'outlined'}
              label={`${row.promo_count} промо`}
              sx={{ mt: 0.5, height: 20, '& .MuiChip-label': { px: 0.75, fontSize: 10 } }}
            />
          </Tooltip>
        )}
      </Box>
    );
  };

  const selectedSKUs = selectedBrand
    ? Array.from(new Set(skuRows.filter((row) => row.brand_as === selectedBrand).map((row) => row.sku!))).sort()
    : [];

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
        <ToggleButtonGroup
          size="small"
          exclusive
          value={quarter}
          onChange={(_, value: number | null) => {
            if (value == null) return;
            setQuarter(value);
            setDraftEdits(null);
            setSelectedBrand(null);
          }}
        >
          {QUARTERS.map((value) => <ToggleButton key={value} value={value}>Q{value}</ToggleButton>)}
        </ToggleButtonGroup>
        <ToggleButtonGroup
          size="small"
          exclusive
          value={measure}
          onChange={(_, value: Measure | null) => value != null && setMeasure(value)}
        >
          <ToggleButton value="volume">Объём</ToggleButton>
          <ToggleButton value="investments">Инвестиции</ToggleButton>
        </ToggleButtonGroup>
        {measure === 'volume' && (
          <ToggleButtonGroup
            size="small"
            exclusive
            value={unit}
            onChange={(_, value: Unit | null) => value != null && setUnit(value)}
          >
            <ToggleButton value="rub">₽</ToggleButton>
            <ToggleButton value="units">Упаковки</ToggleButton>
          </ToggleButtonGroup>
        )}
        <Box sx={{ flex: 1 }} />
        {dirty && <Typography variant="caption" color="warning.main">Есть несохранённые изменения</Typography>}
        {canEdit && (
          <Button
            component="label"
            variant="outlined"
            size="small"
            startIcon={<ImportIcon />}
            disabled={dirty || importPreviewMutation.isPending || importMutation.isPending}
          >
            {importPreviewMutation.isPending ? 'Проверка…' : 'Импорт из Excel'}
            <input hidden type="file" accept=".xlsx,.xlsm" onChange={selectImportFile} />
          </Button>
        )}
        {canEdit && (
          <Button variant="contained" size="small" startIcon={<SaveIcon />} disabled={!dirty || saveMutation.isPending} onClick={save}>
            Сохранить прогноз
          </Button>
        )}
      </Box>

      {saveMutation.isError && <Alert severity="error">{(saveMutation.error as Error).message}</Alert>}
      {importPreviewMutation.isError && <Alert severity="error">{(importPreviewMutation.error as Error).message}</Alert>}

      {measure === 'volume' && unit === 'units' ? (
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)' }, gap: 1 }}>
          <SummaryTile label="Факт на дату" value={`${formatRub(totals.fact_units)} уп.`} hint="загруженные месяцы и MTD" />
          <SummaryTile label="Прогноз EAC" value={`${formatRub(totals.eac_units)} уп.`} hint="внесённый прогноз или системная рекомендация" />
        </Box>
      ) : (
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: 'repeat(2, 1fr)', lg: 'repeat(4, 1fr)' }, gap: 1 }}>
          <SummaryTile label="План квартала" value={`${formatRubShort(totals.plan_rub)} ₽`} hint="утверждённое обязательство" />
          <SummaryTile label="Факт на дату" value={`${formatRubShort(totals.fact_rub)} ₽`} hint="загруженные месяцы и MTD" />
          <SummaryTile
            label="Прогноз EAC"
            value={`${formatRubShort(totals.eac_rub)} ₽`}
            hint={`${completion} плана · ${formatRubShort(totals.gap_rub)} ₽`}
            warning={totals.gap_rub < 0}
          />
          <SummaryTile
            label="Инвестиции EAC"
            value={`${formatRubShort(totals.eac_investments_rub)} ₽`}
            hint={investPct == null ? 'нет бюджета' : `${investPct >= 0 ? '+' : ''}${investPct.toLocaleString('ru-RU', { maximumFractionDigits: 1 })}% к бюджету · промо ${totals.promo_count}`}
            warning={totals.investment_variance_rub > 0}
          />
        </Box>
      )}

      <Paper variant="outlined" sx={{ overflowX: 'auto' }}>
        <Table size="small" sx={{ minWidth: 850 }}>
          <TableHead>
            <TableRow>
              <TableCell>Бренд</TableCell>
              {monthNumbers.map((month) => {
                const sample = brandRows.find((row) => row.month === month);
                return (
                  <TableCell key={month}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                      <Typography variant="body2" sx={{ fontWeight: 600 }}>{monthLabel(month, sample)}</Typography>
                      {canEdit && measure === 'volume' && !sample?.is_closed && (
                        <Tooltip title={dirty ? 'Сначала сохраните ручные изменения' : `Очистить прогноз за ${MONTHS[month - 1].toLowerCase()}`}>
                          <span>
                            <IconButton
                              size="small"
                              disabled={dirty}
                              aria-label={`Очистить прогноз за ${MONTHS[month - 1]}`}
                              onClick={() => {
                                setClearScope('all');
                                setClearMonth(month);
                              }}
                            >
                              <ClearIcon fontSize="small" />
                            </IconButton>
                          </span>
                        </Tooltip>
                      )}
                    </Box>
                  </TableCell>
                );
              })}
              <TableCell align="right">EAC Q{quarter}</TableCell>
              <TableCell align="right">К плану</TableCell>
              <TableCell />
            </TableRow>
          </TableHead>
          <TableBody>
            {query.data.brands.map((brandTotal) => (
              <TableRow key={brandTotal.brand_as} hover>
                <TableCell sx={{ fontWeight: 600, whiteSpace: 'nowrap' }}>{brandTotal.brand_as}</TableCell>
                {monthNumbers.map((month) => (
                  <TableCell key={month}>
                    {renderMonthCell(brandRows.find((row) => row.brand_as === brandTotal.brand_as && row.month === month))}
                  </TableCell>
                ))}
                <TableCell align="right" sx={{ fontWeight: 600, whiteSpace: 'nowrap' }}>
                  {measure === 'investments'
                    ? `${formatRubShort(brandTotal.eac_investments_rub)} ₽`
                    : unit === 'rub'
                      ? `${formatRubShort(brandTotal.eac_rub)} ₽`
                      : `${formatRub(brandTotal.eac_units)} уп.`}
                </TableCell>
                <TableCell align="right">
                  {measure === 'volume' && unit === 'units' ? (
                    <Typography variant="caption" color="text.secondary">план ведётся в ₽</Typography>
                  ) : (
                    <Typography
                      variant="body2"
                      color={(brandTotal.completion_pct ?? 0) < 100 ? 'warning.main' : 'success.main'}
                      sx={{ fontWeight: 600 }}
                    >
                      {brandTotal.completion_pct == null ? '—' : `${brandTotal.completion_pct.toLocaleString('ru-RU', { maximumFractionDigits: 1 })}%`}
                    </Typography>
                  )}
                </TableCell>
                <TableCell>
                  <Tooltip title={`Прогноз по SKU в ${unit === 'rub' ? 'рублях' : 'упаковках'}`}>
                    <Button size="small" startIcon={<SKUIcon />} onClick={() => setSelectedBrand(brandTotal.brand_as)}>SKU</Button>
                  </Tooltip>
                </TableCell>
              </TableRow>
            ))}
            {query.data.brands.length === 0 && (
              <TableRow><TableCell colSpan={7}>Сначала добавьте бренды и квартальный план.</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </Paper>

      <Typography variant="caption" color="text.secondary">
        Рекомендация строится по аналогичному месяцу прошлого года, последним трём месяцам и согласованному промо-uplift. План используется только для оценки разрыва.
      </Typography>

      <Drawer anchor="right" open={selectedBrand != null} onClose={() => setSelectedBrand(null)}>
        <Box sx={{ width: { xs: '100vw', sm: 620 }, p: 2 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
            <SKUIcon color="primary" />
            <Box>
              <Typography variant="h6">{selectedBrand}</Typography>
              <Typography variant="caption" color="text.secondary">
                SKU-прогноз в {unit === 'rub' ? 'рублях' : 'упаковках'} · Q{quarter} {year}
              </Typography>
            </Box>
            <Box sx={{ flex: 1 }} />
            <IconButton onClick={() => setSelectedBrand(null)}><CloseIcon /></IconButton>
          </Box>
          {selectedSKUs.length === 0 ? (
            <Alert severity="info">Добавьте SKU и контрактные цены во вкладке «Цены и SKU».</Alert>
          ) : (
            <Paper variant="outlined" sx={{ overflowX: 'auto' }}>
              <Table size="small" sx={{ minWidth: 560 }}>
                <TableHead>
                  <TableRow>
                    <TableCell>SKU</TableCell>
                    {monthNumbers.map((month) => <TableCell key={month}>{MONTHS[month - 1]}</TableCell>)}
                  </TableRow>
                </TableHead>
                <TableBody>
                  {selectedSKUs.map((sku) => (
                    <TableRow key={sku}>
                      <TableCell sx={{ fontWeight: 600, minWidth: 130 }}>{sku}</TableCell>
                      {monthNumbers.map((month) => {
                        const row = skuRows.find((item) => item.brand_as === selectedBrand && item.sku === sku && item.month === month);
                        if (!row) return <TableCell key={month}>—</TableCell>;
                        const value = draft[forecastKey(month, row.brand_as, sku)] ?? { rub: '', units: '', investments: '', reason: '' };
                        const isRub = unit === 'rub';
                        const forecastValue = isRub ? value.rub : value.units;
                        const units = parseNumberInput(value.units) ?? row.system_forecast_units;
                        const rub = parseNumberInput(value.rub)
                          ?? (units != null && row.contract_price != null ? units * row.contract_price : null);
                        const systemRub = row.system_forecast_rub
                          ?? (row.system_forecast_units != null && row.contract_price != null
                            ? row.system_forecast_units * row.contract_price
                            : null);
                        const helper = isRub
                          ? [
                            systemRub == null ? null : `рек. ${formatRubShort(systemRub)} ₽`,
                            units == null ? null : `${formatRub(units)} уп.`,
                          ].filter(Boolean).join(' · ') || '—'
                          : [
                            row.system_forecast_units == null ? null : `рек. ${formatRub(row.system_forecast_units)} уп.`,
                            rub == null ? null : `≈ ${formatRubShort(rub)} ₽`,
                          ].filter(Boolean).join(' · ') || '—';
                        return (
                          <TableCell key={month} sx={{ minWidth: 135 }}>
                            <Typography variant="caption" color="text.secondary">
                              Цена {row.contract_price == null ? 'не задана' : `${formatRub(row.contract_price, 2)} ₽`}
                            </Typography>
                            <TextField
                              size="small"
                              value={forecastValue}
                              disabled={!canEdit || row.is_closed}
                              label={isRub ? 'Прогноз, ₽' : 'Прогноз, уп.'}
                              placeholder={isRub
                                ? (systemRub == null ? '' : formatRub(systemRub))
                                : (row.system_forecast_units == null ? '' : formatRub(row.system_forecast_units))}
                              onChange={(event) => updateDraft(row, isRub
                                ? { rub: event.target.value }
                                : { units: event.target.value })}
                              helperText={helper}
                              sx={{ mt: 0.5, width: 125 }}
                              slotProps={{ htmlInput: { inputMode: 'decimal' } }}
                            />
                          </TableCell>
                        );
                      })}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </Paper>
          )}
        </Box>
      </Drawer>

      <Dialog
        open={importDialogOpen}
        onClose={() => {
          if (importMutation.isPending) return;
          setImportDialogOpen(false);
        }}
        fullWidth
        maxWidth="sm"
      >
        <DialogTitle>Импорт прогноза из Excel</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, pt: '8px !important' }}>
          {importPreview && (
            <>
              <Typography variant="body2">
                Файл: <strong>{importPreview.file_name}</strong>
              </Typography>
              <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 1 }}>
                <Paper variant="outlined" sx={{ p: 1 }}>
                  <Typography variant="caption" color="text.secondary">Готово к загрузке</Typography>
                  <Typography variant="h6">{importPreview.valid_rows}</Typography>
                </Paper>
                <Paper variant="outlined" sx={{ p: 1 }}>
                  <Typography variant="caption" color="text.secondary">Брендов затронуто</Typography>
                  <Typography variant="h6">{importPreview.affected_brands}</Typography>
                </Paper>
              </Box>
              <Typography variant="caption" color="text.secondary">
                Новых: {importPreview.added_rows} · изменится: {importPreview.updated_rows} · без изменений: {importPreview.unchanged_rows}
              </Typography>
              {importPreview.errors.length > 0 && (
                <Alert severity="error">
                  Файл не будет загружен, пока не исправлены ошибки:
                  <Box component="ul" sx={{ my: 0.5, pl: 2.5 }}>
                    {importPreview.errors.slice(0, 12).map((issue, index) => (
                      <li key={`${issue.row}-${index}`}>Строка {issue.row}: {issue.message}</li>
                    ))}
                  </Box>
                  {importPreview.errors.length > 12 && `Ещё ошибок: ${importPreview.errors.length - 12}`}
                </Alert>
              )}
              {importPreview.warnings.length > 0 && (
                <Alert severity="warning">
                  <Box component="ul" sx={{ my: 0, pl: 2.5 }}>
                    {importPreview.warnings.slice(0, 8).map((issue, index) => (
                      <li key={`${issue.row}-${index}`}>Строка {issue.row}: {issue.message}</li>
                    ))}
                  </Box>
                </Alert>
              )}
              {importPreview.errors.length === 0 && (
                <Alert severity="info">
                  Указанные SKU будут обновлены, остальные строки прогноза сохранятся. Итоги брендов пересчитаются автоматически.
                </Alert>
              )}
            </>
          )}
          {importMutation.isError && <Alert severity="error">{(importMutation.error as Error).message}</Alert>}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setImportDialogOpen(false)} disabled={importMutation.isPending}>Отмена</Button>
          <Button
            variant="contained"
            startIcon={<ImportIcon />}
            disabled={!importFile || !importPreview || importPreview.errors.length > 0 || importMutation.isPending}
            onClick={() => importFile && importMutation.mutate(importFile)}
          >
            {importMutation.isPending ? 'Импорт…' : 'Импортировать'}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={clearMonth != null} onClose={() => !clearMutation.isPending && setClearMonth(null)} fullWidth maxWidth="xs">
        <DialogTitle>Очистить прогноз за месяц?</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, pt: '8px !important' }}>
          <Alert severity="warning">
            {clearMonth == null ? '' : `${MONTHS[clearMonth - 1]} ${year}`}: внесённые значения КАМ будут очищены.
            После полной очистки EAC снова будет использовать системную рекомендацию. Чтобы зафиксировать нулевой прогноз, введите 0 вручную.
          </Alert>
          <FormControl>
            <RadioGroup
              value={clearScope}
              onChange={(event) => setClearScope(event.target.value as NetworkForecastClearScope)}
            >
              <FormControlLabel value="rub" control={<Radio />} label="Только прогноз в рублях" />
              <FormControlLabel value="units" control={<Radio />} label="Только прогноз в упаковках" />
              <FormControlLabel value="all" control={<Radio />} label="Весь прогноз ТО месяца" />
            </RadioGroup>
          </FormControl>
          <Typography variant="body2" color="text.secondary">
            Будет очищено строк: {clearAffectedRows}. Факт, системная рекомендация и прогноз инвестиций не изменятся.
          </Typography>
          {clearMutation.isError && <Alert severity="error">{(clearMutation.error as Error).message}</Alert>}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setClearMonth(null)} disabled={clearMutation.isPending}>Отмена</Button>
          <Button
            color="error"
            variant="contained"
            startIcon={<ClearIcon />}
            disabled={clearMonth == null || clearAffectedRows === 0 || clearMutation.isPending}
            onClick={() => clearMonth != null && clearMutation.mutate({ month: clearMonth, scope: clearScope })}
          >
            {clearMutation.isPending ? 'Очистка…' : 'Очистить'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
