// Вкладка «Прогноз» реестра сетей.
//
// Одна сетка на все способы ведения: строка бренда — свод за квартал, внутри
// раскрывается ввод по месяцам и детализация по SKU. Что именно вводится,
// решает режим бренда (уровень и единица), а не режим формы: в одной сети часть
// брендов ведут в рублях по бренду, часть — в упаковках по SKU, и обе группы
// должны быть видны рядом.
//
// Расчётов здесь нет. EAC, инвестиции и разложение по миксу считает backend
// (backend/services/network_forecast_service.go); форма показывает пришедшее и
// отправляет обратно только введённую метрику.

import { type ChangeEvent, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Collapse,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  IconButton,
  MenuItem,
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
  DeleteSweepOutlined as ClearIcon,
  ExpandMore as ExpandIcon,
  RestartAlt as ResetIcon,
  Save as SaveIcon,
  UploadFileOutlined as ImportIcon,
} from '@mui/icons-material';
import { networkAPI } from '../api/networks';
import type {
  NetworkEntryUnit,
  NetworkForecastInput,
  NetworkForecastClearScope,
  NetworkForecastImportPreview,
  NetworkForecastMonth,
  NetworkForecastResponse,
  NetworkForecastSaveRequest,
  NetworkInvestmentsSource,
} from '../types/network';
import {
  formatNumberInput,
  formatPct,
  formatRub,
  formatRubShort,
  parseNumberInput,
} from '../utils/networkPlan';
import { EntryModeChip, MonthCell } from './NetworkForecastCells';
import NetworkPromoDetail from './NetworkPromoDetail';
import type { EntryMode } from '../utils/networkForecastView';
import { MONTHS, amountLabel } from '../utils/networkForecastView';

const QUARTERS = [1, 2, 3, 4] as const;

interface ForecastDraft {
  rub: string;
  units: string;
  investments: string;
}

interface Props {
  networkId: number;
  year: number;
  canEdit: boolean;
}

const EMPTY_DRAFT: ForecastDraft = { rub: '', units: '', investments: '' };

// Свод по промо бренда за квартал. Разбивка приходит помесячно
// (NetworkForecastMonth) и в квартальный итог бренда не сворачивается, поэтому
// складывается здесь — данные уже загружены, лишнего запроса не нужно.
function promoSummary(months: NetworkForecastMonth[]): string {
  const sum = (pick: (row: NetworkForecastMonth) => number) =>
    months.reduce((total, row) => total + pick(row), 0);
  return [
    `Промо за квартал: ${sum((row) => row.promo_count)}`,
    `согласовано ${sum((row) => row.approved_promo_count)}, черновики ${sum((row) => row.draft_promo_count)}`,
    `План промо: ${formatRubShort(sum((row) => row.promo_plan_rub))} ₽`,
    `Uplift плана: ${formatRubShort(sum((row) => row.promo_uplift_rub))} ₽`,
    `Инвестиции промо: ${formatRubShort(sum((row) => row.promo_investments_rub))} ₽`,
  ].join(' · ') + ' — нажмите, чтобы раскрыть список';
}

const forecastKey = (month: number, brand: string, sku: string | null): string =>
  `${month}|${brand}|${sku ?? ''}`;

const asInput = (value: number | null): string => value == null ? '' : formatNumberInput(String(value));

const draftFromRows = (rows: NetworkForecastMonth[]): Record<string, ForecastDraft> =>
  Object.fromEntries(rows.map((row) => [forecastKey(row.month, row.brand_as, row.sku), {
    rub: asInput(row.forecast_rub),
    units: asInput(row.forecast_units),
    investments: asInput(row.forecast_investments_rub),
  }]));

const sameDraft = (left: ForecastDraft | undefined, right: ForecastDraft | undefined): boolean =>
  left?.rub === right?.rub && left?.units === right?.units && left?.investments === right?.investments;

const monthLabel = (month: number, row?: NetworkForecastMonth): string => {
  if (row?.is_closed) return `${MONTHS[month - 1]} · закрыт`;
  if (row?.is_current) return `${MONTHS[month - 1]} · текущий`;
  return MONTHS[month - 1];
};

const pctLabel = (value: number | null): string =>
  value == null ? '—' : `${value.toLocaleString('ru-RU', { maximumFractionDigits: 1 })} %`;

// Подпись, откуда взялась сумма инвестиций. Вводить её нельзя: это процент
// бренда из квартального плана, применённый к EAC объёма.
const investmentsNote = (row: NetworkForecastMonth): string => ({
  fact: 'факт выплат',
  pct: row.investments_pct == null ? 'нет процента' : `${formatPct(row.investments_pct)} % от прогноза`,
  override: 'переопределено вручную',
  none: row.investments_pct == null ? 'процент не задан' : 'нет прогноза объёма',
}[row.investments_source as NetworkInvestmentsSource] ?? '');

export default function NetworkForecastTab({ networkId, year, canEdit }: Props) {
  const now = new Date();
  const defaultQuarter = year === now.getFullYear() ? Math.floor(now.getMonth() / 3) + 1 : 1;
  const [quarter, setQuarter] = useState(defaultQuarter);
  const [displayUnit, setDisplayUnit] = useState<NetworkEntryUnit>('rub');
  const [draftEdits, setDraftEdits] = useState<Record<string, ForecastDraft> | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);
  // Детализация промо раскрывается отдельно от ввода по месяцам: это
  // справка к счётчику, а не часть формы.
  const [promoDetail, setPromoDetail] = useState<string | null>(null);
  const [importFile, setImportFile] = useState<File | null>(null);
  const [importPreview, setImportPreview] = useState<NetworkForecastImportPreview | null>(null);
  const [importDialogOpen, setImportDialogOpen] = useState(false);
  const [clearOpen, setClearOpen] = useState(false);
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
    // Прогноз — это прогноз итога витрины. Она живёт отдельным запросом и без
    // сброса ещё пять минут показывала бы результат до правки.
    void queryClient.invalidateQueries({ queryKey: ['networkDashboard'] });
    setDraftEdits(null);
  };

  const saveMutation = useMutation({
    mutationFn: (request: NetworkForecastSaveRequest) => networkAPI.saveForecast(networkId, request),
    onSuccess: (response) => applyForecastData(response.data),
  });

  const entryModeMutation = useMutation({
    mutationFn: (input: { brand: string; mode: EntryMode }) => networkAPI.setEntryMode(networkId, {
      year, quarter, brand_as: input.brand,
      entry_level: input.mode.level, entry_unit: input.mode.unit,
    }),
    onSuccess: (response) => applyForecastData(response.data),
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
      setClearOpen(false);
    },
  });

  const monthNumbers = useMemo(() => {
    const start = (quarter - 1) * 3 + 1;
    return [start, start + 1, start + 2];
  }, [quarter]);

  const rowsByKey = useMemo(() => new Map((query.data?.months ?? []).map((row) => [
    forecastKey(row.month, row.brand_as, row.sku), row,
  ])), [query.data]);

  const skusByBrand = useMemo(() => {
    const map = new Map<string, string[]>();
    (query.data?.months ?? []).forEach((row) => {
      if (row.sku == null) return;
      const list = map.get(row.brand_as) ?? [];
      if (!list.includes(row.sku)) list.push(row.sku);
      map.set(row.brand_as, list);
    });
    map.forEach((list) => list.sort((a, b) => a.localeCompare(b, 'ru')));
    return map;
  }, [query.data]);

  const updateDraft = (row: NetworkForecastMonth, patch: Partial<ForecastDraft>) => {
    const key = forecastKey(row.month, row.brand_as, row.sku);
    setDraftEdits((current) => ({
      ...(current ?? baseDraft),
      [key]: { ...((current ?? baseDraft)[key] ?? EMPTY_DRAFT), ...patch },
    }));
  };

  // Отправляем только введённую метрику: парную считает бэкенд по цене
  // контракта того же месяца и сам же записывает её в БД. Прислать обе значило
  // бы дать форме второй источник истины о цене.
  const save = () => {
    const lines: NetworkForecastInput[] = Object.entries(draft).flatMap(([key, value]) => {
      const row = rowsByKey.get(key);
      if (!row || row.is_derived || sameDraft(value, baseDraft[key])) return [];
      const inUnits = row.entry_unit === 'units';
      const owned = parseNumberInput(inUnits ? value.units : value.rub);
      return [{
        month: row.month,
        brand_as: row.brand_as,
        sku: row.sku,
        forecast_rub: inUnits ? null : owned,
        forecast_units: inUnits ? owned : null,
        forecast_investments_rub: parseNumberInput(value.investments),
        adjustment_reason: null,
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
  const clearAffectedRows = clearMonth == null ? 0 : query.data.months.filter((row) => {
    if (row.month !== clearMonth || row.is_derived) return false;
    if (clearScope === 'rub') return row.forecast_rub != null;
    if (clearScope === 'units') return row.forecast_units != null;
    return row.forecast_rub != null || row.forecast_units != null;
  }).length;

  const cellValue = (row: NetworkForecastMonth): string => {
    const value = draft[forecastKey(row.month, row.brand_as, row.sku)] ?? EMPTY_DRAFT;
    return row.entry_unit === 'units' ? value.units : value.rub;
  };

  const changeCell = (row: NetworkForecastMonth, next: string) =>
    updateDraft(row, row.entry_unit === 'units' ? { units: next } : { rub: next });

  // Раскрытая часть строки бренда: помесячный ввод и детализация по SKU.
  const renderDetail = (brand: string, mode: EntryMode) => {
    const brandMonths = monthNumbers.map((month) => rowsByKey.get(forecastKey(month, brand, null)));
    const skus = skusByBrand.get(brand) ?? [];

    return (
      <Box sx={{ p: 1.5, bgcolor: 'action.hover' }}>
        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 1.5 }}>
          {monthNumbers.map((month, index) => {
            const row = brandMonths[index];
            return (
              <Box key={month}>
                <Typography variant="caption" sx={{ fontWeight: 600, display: 'block', mb: 0.25 }}>
                  {monthLabel(month, row)}
                </Typography>
                {row == null ? (
                  <Typography variant="body2" color="text.disabled">—</Typography>
                ) : (
                  <MonthCell
                    row={row}
                    value={cellValue(row)}
                    unit={mode.unit}
                    canEdit={canEdit}
                    showPlan
                    onChange={(next) => changeCell(row, next)}
                  />
                )}
              </Box>
            );
          })}
        </Box>

        {/* Инвестиции идут строкой под объёмом: сумма расчётная, но видеть её
            нужно рядом с прогнозом, от которого она и считается. */}
        <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 1.5, mt: 1 }}>
          <Typography variant="caption" color="text.secondary">Инвестиции:</Typography>
          {monthNumbers.map((month, index) => {
            const row = brandMonths[index];
            if (row == null) return null;
            return (
              <Box key={month} sx={{ display: 'flex', alignItems: 'center', gap: 0.25 }}>
                <Typography
                  variant="caption"
                  color={row.investments_source === 'override' ? 'warning.main' : 'text.secondary'}
                >
                  {MONTHS[row.month - 1].slice(0, 3)} {formatRubShort(row.eac_investments_rub)} ₽
                  {' · '}{investmentsNote(row)}
                </Typography>
                {canEdit && !row.is_closed && row.investments_source === 'override' && (
                  <Tooltip title="Вернуть расчёт по проценту инвестиций">
                    <IconButton
                      size="small"
                      aria-label={`Сбросить переопределение инвестиций ${brand} ${MONTHS[row.month - 1]}`}
                      onClick={() => updateDraft(row, { investments: '' })}
                    >
                      <ResetIcon fontSize="inherit" />
                    </IconButton>
                  </Tooltip>
                )}
              </Box>
            );
          })}
        </Box>

        {skus.length > 0 && (
          <Box sx={{ mt: 1.5 }}>
            <Typography variant="caption" color="text.secondary">
              {mode.level === 'sku'
                ? 'Прогноз вводится по SKU, строка бренда равна их сумме.'
                : 'Разложение по SKU расчётное — по миксу факта. Чтобы вводить SKU, переключите режим бренда.'}
            </Typography>
            <Table size="small" sx={{ mt: 0.5 }}>
              <TableHead>
                <TableRow>
                  <TableCell sx={{ width: '34%' }}>SKU · цена</TableCell>
                  {monthNumbers.map((month) => (
                    <TableCell key={month}>{MONTHS[month - 1]}</TableCell>
                  ))}
                </TableRow>
              </TableHead>
              <TableBody>
                {skus.map((sku) => {
                  const sample = rowsByKey.get(forecastKey(monthNumbers[0], brand, sku));
                  return (
                    <TableRow key={sku}>
                      <TableCell>
                        <Typography variant="body2" noWrap title={sku}>{sku}</Typography>
                        <Typography variant="caption" color="text.secondary">
                          {sample?.contract_price == null
                            ? 'цена не задана'
                            : `${formatRub(sample.contract_price, 2)} ₽`}
                        </Typography>
                      </TableCell>
                      {monthNumbers.map((month) => {
                        const row = rowsByKey.get(forecastKey(month, brand, sku));
                        if (!row) return <TableCell key={month}>—</TableCell>;
                        return (
                          <TableCell key={month} sx={{ minWidth: 120 }}>
                            <MonthCell
                              row={row}
                              value={cellValue(row)}
                              unit={mode.unit}
                              canEdit={canEdit}
                              showPlan={false}
                              onChange={(next) => changeCell(row, next)}
                            />
                          </TableCell>
                        );
                      })}
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </Box>
        )}
      </Box>
    );
  };

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.25 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
        <ToggleButtonGroup
          size="small"
          exclusive
          value={quarter}
          onChange={(_, value: number | null) => {
            if (value == null) return;
            setQuarter(value);
            setDraftEdits(null);
            setExpanded(null);
          }}
        >
          {QUARTERS.map((value) => <ToggleButton key={value} value={value}>Q{value}</ToggleButton>)}
        </ToggleButtonGroup>

        {/* Переключатель показа ничего не пишет: в какой единице бренд ведут,
            задаёт его режим, а здесь выбирается только колонка свода. */}
        <ToggleButtonGroup
          size="small"
          exclusive
          value={displayUnit}
          onChange={(_, value: NetworkEntryUnit | null) => value != null && setDisplayUnit(value)}
        >
          <ToggleButton value="rub">Свод в ₽</ToggleButton>
          <ToggleButton value="units">Свод в уп.</ToggleButton>
        </ToggleButtonGroup>

        <Box sx={{ flex: 1 }} />
        {dirty && <Typography variant="caption" color="warning.main">Есть несохранённые изменения</Typography>}
        {canEdit && (
          <Button
            size="small"
            startIcon={<ClearIcon />}
            disabled={dirty}
            onClick={() => {
              setClearMonth(monthNumbers[0]);
              setClearScope('all');
              setClearOpen(true);
            }}
          >
            Очистить месяц
          </Button>
        )}
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
          <Button
            variant="contained"
            size="small"
            startIcon={<SaveIcon />}
            disabled={!dirty || saveMutation.isPending}
            onClick={save}
          >
            Сохранить прогноз
          </Button>
        )}
      </Box>

      {saveMutation.isError && <Alert severity="error">{(saveMutation.error as Error).message}</Alert>}
      {entryModeMutation.isError && <Alert severity="error">{(entryModeMutation.error as Error).message}</Alert>}
      {importPreviewMutation.isError && <Alert severity="error">{(importPreviewMutation.error as Error).message}</Alert>}

      {/* Итоги квартала одной полосой: четыре карточки занимали экран до того,
          как показывалась первая цифра по брендам. */}
      <Paper variant="outlined" sx={{ px: 1.5, py: 1 }}>
        <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 2.5, fontVariantNumeric: 'tabular-nums' }}>
          {[
            { label: 'План', value: `${formatRubShort(totals.plan_rub)} ₽` },
            {
              label: 'Факт',
              value: displayUnit === 'units'
                ? `${formatRub(totals.fact_units)} уп.`
                : `${formatRubShort(totals.fact_rub)} ₽`,
            },
            {
              label: 'EAC',
              value: displayUnit === 'units'
                ? `${formatRub(totals.eac_units)} уп.`
                : `${formatRubShort(totals.eac_rub)} ₽`,
            },
            { label: 'К плану', value: pctLabel(totals.completion_pct), warning: totals.gap_rub < 0 },
            {
              label: 'Инвестиции',
              value: `${formatRubShort(totals.eac_investments_rub)} ₽`,
              warning: totals.investment_variance_rub > 0,
            },
            { label: 'Промо', value: String(totals.promo_count) },
          ].map((tile) => (
            <Box key={tile.label} sx={{ minWidth: 92 }}>
              <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }}>{tile.label}</Typography>
              <Typography variant="subtitle2" color={tile.warning ? 'warning.main' : 'text.primary'}>
                {tile.value}
              </Typography>
            </Box>
          ))}
        </Box>
      </Paper>

      <Paper variant="outlined" sx={{ overflowX: 'auto' }}>
        <Table size="small" sx={{ minWidth: 720 }}>
          <TableHead>
            <TableRow>
              <TableCell sx={{ width: '38%' }}>Бренд</TableCell>
              <TableCell align="right">План</TableCell>
              <TableCell align="right">Факт</TableCell>
              <TableCell align="right">EAC Q{quarter}</TableCell>
              <TableCell align="right">К плану</TableCell>
              <TableCell align="right">Инвестиции</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {query.data.brands.flatMap((brandTotal) => {
              const brand = brandTotal.brand_as;
              const sample = rowsByKey.get(forecastKey(monthNumbers[0], brand, null));
              const mode: EntryMode = {
                level: sample?.entry_level === 'sku' ? 'sku' : 'brand',
                unit: sample?.entry_unit === 'units' ? 'units' : 'rub',
              };
              const open = expanded === brand;
              const promoOpen = promoDetail === brand;
              const brandMonths = monthNumbers
                .map((month) => rowsByKey.get(forecastKey(month, brand, null)))
                .filter((row): row is NetworkForecastMonth => row != null);

              return [
                <TableRow key={brand} hover>
                  <TableCell>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, minWidth: 0 }}>
                      <IconButton
                        size="small"
                        aria-label={open ? `Свернуть ${brand}` : `Раскрыть ${brand}`}
                        onClick={() => setExpanded(open ? null : brand)}
                      >
                        <ExpandIcon
                          fontSize="small"
                          sx={{ transform: open ? 'none' : 'rotate(-90deg)', transition: '.15s' }}
                        />
                      </IconButton>
                      <Typography variant="body2" sx={{ fontWeight: 600 }} noWrap title={brand}>{brand}</Typography>
                      <EntryModeChip
                        mode={mode}
                        disabled={!canEdit || dirty || entryModeMutation.isPending}
                        onChange={(next) => entryModeMutation.mutate({ brand, mode: next })}
                      />
                      {brandTotal.promo_count > 0 && (
                        <Tooltip title={promoSummary(brandMonths)}>
                          <Chip
                            size="small"
                            variant={promoOpen ? 'filled' : 'outlined'}
                            color={promoOpen ? 'primary' : 'default'}
                            label={`промо ${brandTotal.promo_count}`}
                            onClick={() => setPromoDetail(promoOpen ? null : brand)}
                            sx={{ height: 20, cursor: 'pointer', '& .MuiChip-label': { px: 0.75, fontSize: 11 } }}
                          />
                        </Tooltip>
                      )}
                    </Box>
                  </TableCell>
                  <TableCell align="right">{formatRubShort(brandTotal.plan_rub)} ₽</TableCell>
                  <TableCell align="right">
                    {amountLabel(displayUnit === 'units' ? brandTotal.fact_units : brandTotal.fact_rub, displayUnit)}
                  </TableCell>
                  <TableCell align="right" sx={{ fontWeight: 600 }}>
                    {amountLabel(displayUnit === 'units' ? brandTotal.eac_units : brandTotal.eac_rub, displayUnit)}
                  </TableCell>
                  <TableCell align="right">
                    <Typography
                      variant="body2"
                      sx={{ fontWeight: 600 }}
                      color={(brandTotal.completion_pct ?? 0) < 100 ? 'warning.main' : 'success.main'}
                    >
                      {pctLabel(brandTotal.completion_pct)}
                    </Typography>
                  </TableCell>
                  <TableCell align="right">{formatRubShort(brandTotal.eac_investments_rub)} ₽</TableCell>
                </TableRow>,
                <TableRow key={`${brand}-promo`}>
                  <TableCell colSpan={6} sx={{ p: 0, border: 0 }}>
                    <Collapse in={promoOpen} unmountOnExit>
                      <NetworkPromoDetail
                        networkName={query.data.network.name}
                        brand={brand}
                        year={year}
                        months={monthNumbers}
                      />
                    </Collapse>
                  </TableCell>
                </TableRow>,
                <TableRow key={`${brand}-detail`}>
                  <TableCell colSpan={6} sx={{ p: 0, border: 0 }}>
                    <Collapse in={open} unmountOnExit>{renderDetail(brand, mode)}</Collapse>
                  </TableCell>
                </TableRow>,
              ];
            })}
            {query.data.brands.length === 0 && (
              <TableRow><TableCell colSpan={6}>Сначала добавьте бренды и квартальный план.</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </Paper>

      <Typography variant="caption" color="text.secondary">
        Режим бренда задаёт, что вводится: уровень (бренд или SKU) и единица (рубли или упаковки).
        Вторая метрика пересчитывается по цене контракта, а разложение бренда по SKU строится
        по миксу факта и потому не редактируется. Рекомендация в подсказке поля собирается из
        аналогичного месяца прошлого года, последних трёх месяцев и согласованного промо-uplift;
        план используется только для оценки разрыва. Прогноз инвестиций не вводится: это процент
        бренда из квартального плана, применённый к EAC объёма, а закрытый месяц берёт факт выплат.
      </Typography>

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
              <Typography variant="body2">Файл: <strong>{importPreview.file_name}</strong></Typography>
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
                  Указанные SKU будут обновлены, остальные строки прогноза сохранятся.
                  Итоги брендов пересчитаются автоматически.
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

      <Dialog open={clearOpen} onClose={() => !clearMutation.isPending && setClearOpen(false)} fullWidth maxWidth="xs">
        <DialogTitle>Очистить прогноз за месяц?</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, pt: '8px !important' }}>
          <TextField
            select
            size="small"
            label="Месяц"
            value={clearMonth ?? monthNumbers[0]}
            onChange={(event) => setClearMonth(Number(event.target.value))}
          >
            {monthNumbers.map((month) => (
              <MenuItem key={month} value={month}>{MONTHS[month - 1]} {year}</MenuItem>
            ))}
          </TextField>
          <Alert severity="warning">
            Внесённые значения КАМ будут очищены. После полной очистки EAC снова будет
            использовать системную рекомендацию. Чтобы зафиксировать нулевой прогноз, введите 0 вручную.
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
            Будет очищено строк: {clearAffectedRows}. Факт, системная рекомендация и процент инвестиций не изменятся.
          </Typography>
          {clearMutation.isError && <Alert severity="error">{(clearMutation.error as Error).message}</Alert>}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setClearOpen(false)} disabled={clearMutation.isPending}>Отмена</Button>
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
