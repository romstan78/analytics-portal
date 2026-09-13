// Диалог «SKU бренда на ступени»: план SKU, процент и крышка с наследованием
// от бренда. Открывается с чипа ступени в квартальной таблице.
//
// Список SKU — цены контракта бренда плюс SKU, уже заведённые на ступени.
// Значения вводятся в единице бренда; вторую метрику пары и итог «к выплате»
// считает сервер пересчётом черновика — здесь только ввод и остаток до плана.

import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Alert,
  Autocomplete,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
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
import { DeleteOutlined as DeleteIcon } from '@mui/icons-material';
import { networkAPI } from '../api/networks';
import {
  CAP_MODES,
  capModeLabel,
  formatNumberInput,
  formatRubShort,
  parseNumberInput,
  round2,
} from '../utils/networkPlan';
import type { CapMode, DraftSKU, ScaleAmounts } from '../utils/networkPlan';
import { PlanNumberField, ValueCell } from './networkPlanCells';

interface Props {
  open: boolean;
  networkId: number;
  year: number;
  quarter: number;
  brand: string;
  scaleNo: number;
  entryUnit: 'rub' | 'units';
  // План бренда на ступени в единице ввода: до него считается остаток.
  brandPlan: number | null;
  brandPct: string;
  ownerCapMode: CapMode;
  ownerCapPct: string;
  skus: DraftSKU[];
  amounts?: ScaleAmounts;
  canEdit: boolean;
  onApply: (skus: DraftSKU[]) => void;
  onClose: () => void;
}

const EMPTY_SKU = (sku: string): DraftSKU => ({
  sku, planRub: '', planUnits: '', investmentsPct: '', capMode: '', capPct: '',
});

// Строка без единого значения хранить нечего: она уходит из запроса.
const isBlank = (row: DraftSKU) =>
  row.planRub.trim() === '' && row.planUnits.trim() === '' && row.investmentsPct.trim() === '' && row.capMode === '';

export default function NetworkScaleSKUDialog({
  open, networkId, year, quarter, brand, scaleNo, entryUnit,
  brandPlan, brandPct, ownerCapMode, ownerCapPct, skus, amounts, canEdit, onApply, onClose,
}: Props) {
  const pricesQuery = useQuery({
    queryKey: ['networkPrices', networkId, year],
    queryFn: () => networkAPI.getPrices(networkId, year),
    enabled: open,
    staleTime: 60_000,
  });
  const pricedSkus = useMemo(() => {
    const set = new Set<string>();
    pricesQuery.data?.data.forEach((price) => {
      if (price.brand_as === brand) set.add(price.sku);
    });
    return set;
  }, [pricesQuery.data, brand]);
  const skuOptions = useMemo(() => {
    const set = new Set<string>(pricedSkus);
    pricesQuery.data?.sku_options.forEach((option) => {
      if (option.brand_as === brand) set.add(option.sku);
    });
    return Array.from(set).sort((a, b) => a.localeCompare(b, 'ru'));
  }, [pricesQuery.data, pricedSkus, brand]);

  // Начальный список: заведённые SKU плюс SKU с ценой контракта — пустыми.
  const [rows, setRows] = useState<DraftSKU[]>(() => skus);
  const [seeded, setSeeded] = useState(false);
  if (!seeded && pricesQuery.data) {
    setSeeded(true);
    const known = new Set(rows.map((row) => row.sku));
    const extra = Array.from(pricedSkus).filter((sku) => !known.has(sku)).sort((a, b) => a.localeCompare(b, 'ru'));
    if (extra.length > 0) setRows((current) => [...current, ...extra.map(EMPTY_SKU)]);
  }

  const planField: 'planRub' | 'planUnits' = entryUnit === 'units' ? 'planUnits' : 'planRub';
  const unitLabel = entryUnit === 'units' ? 'уп' : '₽';
  const skuPlanSum = rows.reduce((sum, row) => sum + (parseNumberInput(row[planField]) ?? 0), 0);
  const rest = brandPlan == null ? null : round2(brandPlan - skuPlanSum);
  const amountsBySku = new Map((amounts?.skus ?? []).map((sku) => [sku.sku, sku]));

  const setRow = (index: number, patch: Partial<DraftSKU>) =>
    setRows((current) => current.map((row, i) => (i === index ? { ...row, ...patch } : row)));

  // Раскладка плана бренда по SKU долями; без долей — поровну. Последний SKU
  // забирает остаток округления: сумма равна плану.
  const fillByShares = (shares: Map<string, number>) => {
    if (brandPlan == null || rows.length === 0) return;
    const known = rows.filter((row) => (shares.get(row.sku) ?? 0) > 0);
    const knownTotal = known.reduce((sum, row) => sum + (shares.get(row.sku) ?? 0), 0);
    const unknown = rows.length - known.length;
    const restShare = Math.max(0, 1 - knownTotal);
    const shareOf = (row: DraftSKU) => {
      const own = shares.get(row.sku) ?? 0;
      if (own > 0) return own;
      return unknown > 0 ? restShare / unknown : 0;
    };
    let assigned = 0;
    setRows((current) => current.map((row, i) => {
      const value = i === current.length - 1
        ? round2(brandPlan - assigned)
        : round2(brandPlan * shareOf(row));
      assigned = round2(assigned + value);
      return { ...row, [planField]: formatNumberInput(String(value)) };
    }));
  };

  const fillEqually = () => fillByShares(new Map());

  const addSku = (sku: string | null) => {
    if (!sku || rows.some((row) => row.sku === sku)) return;
    setRows((current) => [...current, EMPTY_SKU(sku)]);
  };

  const apply = () => onApply(rows.filter((row) => !isBlank(row)));

  const missingPrice = entryUnit === 'units'
    ? rows.filter((row) => !isBlank(row) && !pricedSkus.has(row.sku)).map((row) => row.sku)
    : [];

  return (
    <Dialog open={open} onClose={onClose} maxWidth="lg" fullWidth>
      <DialogTitle sx={{ pb: 0.5 }}>
        {brand} · ступень {scaleNo} · Q{quarter} {year}
        <Typography variant="body2" color="text.secondary">
          ввод в {entryUnit === 'units' ? 'упаковках' : 'рублях'}
          {brandPlan != null && ` · план бренда ${formatNumberInput(String(brandPlan))} ${unitLabel}`}
          {' · '}{capModeLabel(ownerCapMode, ownerCapPct)}
          {brandPct.trim() !== '' && ` · ${brandPct} %`}
        </Typography>
      </DialogTitle>
      <DialogContent>
        <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', flexWrap: 'wrap', mb: 1 }}>
          <Typography variant="caption" color="text.secondary">
            Пустые процент и крышка — как у бренда. Остаток до плана бренда считается в остатке
            бренда с его процентом и крышкой.
          </Typography>
          <Box sx={{ flex: 1 }} />
          {canEdit && (
            <Autocomplete
              size="small"
              options={skuOptions.filter((sku) => !rows.some((row) => row.sku === sku))}
              value={null}
              blurOnSelect
              onChange={(_, value) => addSku(value)}
              sx={{ minWidth: 240 }}
              renderInput={(params) => <TextField {...params} label="Добавить SKU" />}
            />
          )}
          {canEdit && (
            <Button size="small" disabled={brandPlan == null || rows.length === 0} onClick={fillEqually}>
              Заполнить поровну
            </Button>
          )}
        </Box>

        {missingPrice.length > 0 && (
          <Alert severity="warning" sx={{ mb: 1 }}>
            Нет цены контракта: {missingPrice.join(', ')}. План в упаковках без цены не пересчитается
            в рубли — добавьте цену во вкладке «Цены и SKU» или введите план в рублях.
          </Alert>
        )}

        <Table size="small" sx={{ '& th': { fontSize: 12, whiteSpace: 'nowrap' } }}>
          <TableHead>
            <TableRow>
              <TableCell>SKU</TableCell>
              <TableCell align="right">План, {unitLabel}</TableCell>
              <TableCell align="right">{entryUnit === 'units' ? 'План, ₽' : 'План, уп'}</TableCell>
              <TableCell align="right">Прогноз, ₽</TableCell>
              <TableCell align="right">Инв., %</TableCell>
              <TableCell>Крышка</TableCell>
              <TableCell align="right">База</TableCell>
              <TableCell align="right">К выплате</TableCell>
              <TableCell padding="none" />
            </TableRow>
          </TableHead>
          <TableBody>
            {rows.map((row, index) => {
              const calc = amountsBySku.get(row.sku);
              const pairValue = entryUnit === 'units' ? calc?.plan : calc?.planUnits;
              return (
                <TableRow key={row.sku} hover>
                  <TableCell>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                      <Typography variant="body2" noWrap title={row.sku}>{row.sku}</Typography>
                      {!isBlank(row) && <Chip size="small" color="warning" variant="outlined" label="иначе" />}
                      {entryUnit === 'units' && !pricedSkus.has(row.sku) && (
                        <Chip size="small" variant="outlined" label="нет цены" />
                      )}
                    </Box>
                  </TableCell>
                  <TableCell sx={{ width: 150 }}>
                    <PlanNumberField
                      value={row[planField]}
                      disabled={!canEdit}
                      onChange={(value) => setRow(index, {
                        [planField]: value,
                        // Вторая метрика пары считается сервером заново.
                        [entryUnit === 'units' ? 'planRub' : 'planUnits']: '',
                      })}
                    />
                  </TableCell>
                  <TableCell align="right">
                    <ValueCell value={pairValue ?? null} muted hint={pairValue == null && !isBlank(row) ? 'после пересчёта' : null} />
                  </TableCell>
                  <TableCell align="right">
                    <ValueCell value={calc?.forecast ?? null} muted={calc?.forecast == null} hint={calc && calc.forecast == null ? 'нет объёма по SKU' : null} />
                  </TableCell>
                  <TableCell sx={{ width: 90 }}>
                    <PlanNumberField
                      value={row.investmentsPct}
                      disabled={!canEdit}
                      placeholder={brandPct || '—'}
                      onChange={(value) => setRow(index, { investmentsPct: value })}
                    />
                  </TableCell>
                  <TableCell>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                      <ToggleButtonGroup
                        size="small"
                        exclusive
                        value={row.capMode}
                        disabled={!canEdit}
                        onChange={(_, value: '' | CapMode | null) => setRow(index, { capMode: value ?? '' })}
                        sx={{ '& .MuiToggleButton-root': { textTransform: 'none', px: 0.75, py: 0.25, fontSize: 11 } }}
                      >
                        <ToggleButton value="">как у бренда</ToggleButton>
                        {CAP_MODES.map((mode) => (
                          <ToggleButton key={mode.value} value={mode.value}>{mode.short}</ToggleButton>
                        ))}
                      </ToggleButtonGroup>
                      {row.capMode === 'pct' && (
                        <Box sx={{ width: 64 }}>
                          <PlanNumberField
                            value={row.capPct}
                            disabled={!canEdit}
                            suffix="%"
                            onChange={(value) => setRow(index, { capPct: value })}
                          />
                        </Box>
                      )}
                    </Box>
                  </TableCell>
                  <TableCell align="right">
                    <ValueCell value={calc?.forecastBase ?? null} muted={calc?.forecastBase == null} />
                  </TableCell>
                  <TableCell align="right">
                    <ValueCell value={calc?.forecastInvest ?? null} muted={calc?.forecastInvest == null} />
                  </TableCell>
                  <TableCell padding="none">
                    {canEdit && (
                      <Tooltip title="Убрать SKU со ступени">
                        <IconButton size="small" onClick={() => setRows((current) => current.filter((_, i) => i !== index))}>
                          <DeleteIcon fontSize="inherit" />
                        </IconButton>
                      </Tooltip>
                    )}
                  </TableCell>
                </TableRow>
              );
            })}
            {rows.length === 0 && (
              <TableRow>
                <TableCell colSpan={9}>
                  <Typography variant="body2" color="text.secondary" sx={{ py: 1 }}>
                    {pricesQuery.isLoading
                      ? 'Загружаются SKU бренда…'
                      : 'У бренда нет SKU с ценой контракта. Добавьте SKU из списка выше.'}
                  </Typography>
                </TableCell>
              </TableRow>
            )}
            <TableRow sx={{ bgcolor: 'action.hover' }}>
              <TableCell sx={{ fontWeight: 600 }}>Итого по SKU</TableCell>
              <TableCell align="right">
                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 1 }}>
                  <Typography variant="body2" sx={{ fontWeight: 600, fontVariantNumeric: 'tabular-nums' }}>
                    {formatNumberInput(String(round2(skuPlanSum)))}
                  </Typography>
                  {rest != null && (
                    <Chip
                      size="small"
                      variant={rest === 0 ? 'outlined' : 'filled'}
                      color={rest === 0 ? 'success' : rest < 0 ? 'error' : 'warning'}
                      label={`остаток ${entryUnit === 'units' ? formatNumberInput(String(rest)) : formatRubShort(rest)}`}
                    />
                  )}
                </Box>
              </TableCell>
              <TableCell colSpan={7} />
            </TableRow>
          </TableBody>
        </Table>
      </DialogContent>
      <DialogActions>
        {canEdit && <Button color="inherit" onClick={() => setRows([])}>Сбросить к бренду</Button>}
        <Box sx={{ flex: 1 }} />
        <Button onClick={onClose}>{canEdit ? 'Отмена' : 'Закрыть'}</Button>
        {canEdit && <Button variant="contained" onClick={apply}>Применить</Button>}
      </DialogActions>
    </Dialog>
  );
}
