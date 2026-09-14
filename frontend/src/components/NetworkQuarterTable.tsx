// Детальный разрез одного квартала: план, факт, прогноз и инвестиции рядом.
// Бренды разложены на две группы — входящие в валовый объём контракта и
// планируемые отдельно, потому что валовый объём применяется к брендам,
// а не к контракту целиком.
//
// Ступени контракта раскрываются под строкой бренда, а не расползаются по
// колонкам: свёрнутая строка — сегодняшняя строка плюс значок лесенки, и КАМ,
// которому ступени не нужны, их не видит. Раскрытая строка показывает каждую
// ступень под-строкой, крышку владельца порога и лесенку.

import { Fragment, useState } from 'react';
import {
  Box,
  Button,
  Chip,
  IconButton,
  Menu,
  MenuItem,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
  alpha,
} from '@mui/material';
import {
  ChatBubbleOutlineOutlined as CommentIcon,
  DeleteOutlined as DeleteIcon,
  ExpandLess as ExpandLessIcon,
  ExpandMore as ExpandMoreIcon,
  MoreVert as MoreIcon,
} from '@mui/icons-material';
import {
  CAP_MODES,
  capModeLabel,
  canAddScale,
  deltaPct,
  formatRubShort,
  formatSignedPct,
  parseNumberInput,
  planKey,
  pluralRu,
  scaleAmountsOf,
  scaleErrors,
  scaleThreshold,
  skusOf,
  stepLabel,
  upperScales,
} from '../utils/networkPlan';
import type { CapMode, CellAmounts, DraftCell, DraftSKU, DraftScale } from '../utils/networkPlan';
import { EMPTY_AMOUNTS, EMPTY_CELL } from '../utils/networkPlan';
import type { NetworkPlanTotals } from '../types/network';
import { PlanNumberField, ValueCell } from './networkPlanCells';
import { TONE_COLOR, completionTone, deviationTone } from '../utils/networkPlanView';
import NetworkScaleLadder from './NetworkScaleLadder';
import NetworkScaleSKUDialog from './NetworkScaleSKUDialog';

const COLUMNS = ['20%', '14%', '11%', '14%', '9%', '10%', '11%', '11%', '44px'];

interface NetworkQuarterTableProps {
  networkId: number;
  year: number;
  quarter: number;
  // Сколько ступеней разрешает квартал: настройка профиля сети.
  quarterScales: number;
  brands: string[];
  draft: Record<string, DraftCell>;
  amounts: Record<string, CellAmounts>;
  totals: NetworkPlanTotals;
  canEdit: boolean;
  commentedCells: Set<string>;
  onCellChange: (brand: string | null, patch: Partial<DraftCell>) => void;
  onScaleChange: (brand: string | null, scaleNo: number, patch: Partial<DraftScale>) => void;
  onAddScale: (brand: string | null) => void;
  onRemoveScale: (brand: string | null, scaleNo: number) => void;
  onSkusChange: (brand: string, scaleNo: number, skus: DraftSKU[]) => void;
  onToggleGross: (brand: string, next: boolean, allQuarters: boolean) => void;
  onRemoveBrand: (brand: string) => void;
  onComment: (brand: string | null) => void;
  onDistributeRest: (scaleNo: number) => void;
}

// Значок лесенки в свёрнутой строке: сколько ступеней и какие пройдены прогнозом.
function ScaleBadge({ count, reached }: { count: number; reached: number }) {
  if (count <= 1) return null;
  return (
    <Tooltip title={`${count} ${pluralRu(count, 'ступень', 'ступени', 'ступеней')} · достигнута ${reached === 0 ? 'ни одна' : `ступень ${reached}`}`}>
      <Box sx={{ display: 'inline-flex', gap: '2px', alignItems: 'flex-end', height: 12, ml: 0.5 }}>
        {Array.from({ length: count }, (_, i) => (
          <Box
            key={i}
            sx={{
              width: 3, height: 5 + i * 3, borderRadius: '1px',
              bgcolor: i < reached ? 'primary.main' : 'primary.light',
              opacity: i < reached ? 1 : 0.45,
            }}
          />
        ))}
      </Box>
    </Tooltip>
  );
}

const pctLabel = (value: number | null) =>
  value == null ? '—' : `${value.toLocaleString('ru-RU', { maximumFractionDigits: 1 })} %`;

export default function NetworkQuarterTable({
  networkId,
  year,
  quarter,
  quarterScales,
  brands,
  draft,
  amounts,
  totals,
  canEdit,
  commentedCells,
  onCellChange,
  onScaleChange,
  onAddScale,
  onRemoveScale,
  onSkusChange,
  onToggleGross,
  onRemoveBrand,
  onComment,
  onDistributeRest,
}: NetworkQuarterTableProps) {
  const [menu, setMenu] = useState<{ anchor: HTMLElement; brand: string } | null>(null);
  const [skuDialog, setSkuDialog] = useState<{ brand: string; scaleNo: number } | null>(null);

  const cellOf = (brand: string | null): DraftCell => draft[planKey(quarter, brand)] ?? EMPTY_CELL;
  const amountsOf = (brand: string | null): CellAmounts => amounts[planKey(quarter, brand)] ?? EMPTY_AMOUNTS;
  const grossBrands = brands.filter((b) => cellOf(b).inGross);
  const separateBrands = brands.filter((b) => !cellOf(b).inGross);
  const pool = amountsOf(null);
  const poolCell = cellOf(null);
  const hasPool = grossBrands.length > 0 || pool.plan != null || pool.forecast != null;

  // Раскрытые строки. Владелец порога с настроенными ступенями или крышкой
  // раскрывается сам при первом показе квартала: иначе настройка была бы
  // спрятана. Валовые бренды остаются свёрнутыми — их лестница повторяет пул,
  // и семь раскрытых брендов утопили бы таблицу; исключения по SKU раскрывают.
  const [expanded, setExpanded] = useState<Set<string>>(() => {
    const initial = new Set<string>();
    [null, ...brands].forEach((brand) => {
      const cell = cellOf(brand);
      const owner = brand == null || !cell.inGross;
      const hasSkus = cell.scales.some((s) => s.skus.length > 0);
      if (hasSkus || (owner && (upperScales(cell).length > 0 || cell.capMode !== 'open'))) {
        initial.add(brand ?? '');
      }
    });
    return initial;
  });
  const isExpanded = (brand: string | null) => expanded.has(brand ?? '');
  const toggleExpanded = (brand: string | null) => setExpanded((current) => {
    const next = new Set(current);
    const key = brand ?? '';
    if (next.has(key)) next.delete(key); else next.add(key);
    return next;
  });

  const scaleCount = (cell: DraftCell) => 1 + upperScales(cell).length;
  // Лестница области квартала: у пула — его пороги, у отдельного бренда — свои.
  const ownerLadderLength = (brand: string | null) =>
    brand != null && cellOf(brand).inGross ? scaleCount(poolCell) : scaleCount(cellOf(brand));

  const sectionRow = (title: string, note?: string) => (
    <TableRow sx={{ bgcolor: 'action.hover' }}>
      <TableCell colSpan={COLUMNS.length} sx={{ py: 0.75 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
          <Typography variant="subtitle2">{title}</Typography>
          {note && <Typography variant="caption" color="text.secondary">{note}</Typography>}
        </Box>
      </TableCell>
    </TableRow>
  );

  const expandButton = (brand: string | null) => (
    <IconButton size="small" sx={{ p: 0.25, ml: -0.75 }} onClick={() => toggleExpanded(brand)} aria-label={isExpanded(brand) ? 'Свернуть ступени' : 'Раскрыть ступени'}>
      {isExpanded(brand) ? <ExpandLessIcon fontSize="inherit" /> : <ExpandMoreIcon fontSize="inherit" />}
    </IconButton>
  );

  // Чип исключений по SKU ступени: открывает диалог.
  const skuChip = (brand: string, scaleNo: number) => {
    const count = skusOf(cellOf(brand), scaleNo).filter((s) =>
      s.investmentsPct.trim() !== '' || s.capMode !== '' || s.planRub.trim() !== '' || s.planUnits.trim() !== '').length;
    return (
      <Chip
        size="small"
        variant={count > 0 ? 'filled' : 'outlined'}
        color={count > 0 ? 'warning' : 'default'}
        label={count > 0 ? `SKU · ${count}` : 'SKU'}
        title={count > 0 ? `${count} SKU с отличиями от бренда` : 'Исключения по SKU: как у бренда'}
        onClick={() => setSkuDialog({ brand, scaleNo })}
        sx={{ fontSize: 11, height: 22 }}
      />
    );
  };

  // Под-строка ступени. У владельца порога — порог, у валового бренда — план на
  // ступени с порогом пула в подсказке. Ступень 1 показывается зеркалом строки.
  const scaleRow = (brand: string | null, scaleNo: number) => {
    const cell = cellOf(brand);
    const row = amountsOf(brand);
    const calc = scaleAmountsOf(row, scaleNo);
    const owner = brand == null || !cell.inGross;
    const isFirst = scaleNo === 1;
    const scale = cell.scales.find((s) => s.scaleNo === scaleNo);
    const isLast = scaleNo === ownerLadderLength(brand);
    const threshold = scaleThreshold(cell, scaleNo);
    const previous = scaleNo > 1 ? scaleThreshold(cell, scaleNo - 1) : null;
    const errors = scaleErrors(cell, owner);
    const poolThreshold = brand != null && cell.inGross ? scaleThreshold(poolCell, scaleNo) : null;
    const forecastPct = threshold && threshold > 0 && row.forecast != null ? (row.forecast / threshold) * 100 : null;
    const factPct = threshold && threshold > 0 && row.fact != null ? (row.fact / threshold) * 100 : null;
    const reachedByForecast = calc?.forecastReached ?? false;
    const effective = row.forecastScale === scaleNo;
    const poolReached = brand == null ? scaleAmountsOf(pool, scaleNo)?.forecastReached : undefined;
    const restOfScale = brand == null ? totals.scales?.find((s) => s.scale_no === scaleNo)?.undistributed ?? null : null;
    // Объёмы пула живут в итогах квартала, а не в строке пула.
    const poolFact = totals.gross_pool_fact_rub || null;
    const poolForecast = totals.gross_pool_forecast_rub;

    return (
      <TableRow
        key={`${brand ?? ''}-scale-${scaleNo}`}
        sx={{ '& td': { py: 0.5, bgcolor: (theme) => (effective ? alpha(theme.palette.success.main, 0.08) : theme.palette.action.hover) } }}
      >
        <TableCell sx={{ pl: 4 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
            <Typography variant="caption" sx={{ fontWeight: 600 }}>Ступень {scaleNo}</Typography>
            {isLast && scaleCount(owner ? cell : poolCell) > 1 && <Chip size="small" label="последняя" sx={{ fontSize: 10, height: 18 }} />}
            {canEdit && !isFirst && (
              <Tooltip title={`Убрать ступень ${scaleNo}${scaleNo < 3 ? ' и выше' : ''}`}>
                <IconButton size="small" onClick={() => onRemoveScale(brand, scaleNo)}>
                  <DeleteIcon fontSize="inherit" />
                </IconButton>
              </Tooltip>
            )}
          </Box>
        </TableCell>
        <TableCell>
          {isFirst ? (
            <ValueCell value={threshold} hint={owner ? 'порог — план строки' : 'план строки'} />
          ) : (
            <>
              <PlanNumberField
                value={scale?.planRub ?? ''}
                disabled={!canEdit}
                onChange={(v) => onScaleChange(brand, scaleNo, { planRub: v, planUnits: '' })}
              />
              <Typography variant="caption" sx={{ display: 'block', color: errors[scaleNo] ? 'error.main' : 'text.secondary', lineHeight: 1.3 }}>
                {errors[scaleNo]
                  ?? (owner
                    ? stepLabel(previous, threshold) || 'шаг от предыдущей'
                    : poolThreshold != null ? `порог пула ${formatRubShort(poolThreshold)}` : `у пула нет ступени ${scaleNo}`)}
              </Typography>
            </>
          )}
          {brand == null && restOfScale != null && (
            <Box sx={{ mt: 0.5, display: 'flex', alignItems: 'center', gap: 0.5, flexWrap: 'wrap' }}>
              <Chip
                size="small"
                variant={restOfScale === 0 ? 'outlined' : 'filled'}
                color={restOfScale === 0 ? 'success' : 'warning'}
                label={`остаток ${formatRubShort(restOfScale)}`}
                sx={{ fontSize: 10, height: 18 }}
              />
              {canEdit && restOfScale !== 0 && grossBrands.length > 0 && !isFirst && (
                <Button size="small" sx={{ fontSize: 11, py: 0, minWidth: 0 }} onClick={() => onDistributeRest(scaleNo)}>поровну</Button>
              )}
            </Box>
          )}
        </TableCell>
        <TableCell align="right">
          {brand == null
            ? <Typography variant="caption" sx={{ color: TONE_COLOR[completionTone(poolFact != null && threshold ? (poolFact / threshold) * 100 : null)] }}>{pctLabel(poolFact != null && threshold ? (poolFact / threshold) * 100 : null)}</Typography>
            : owner && <Typography variant="caption" sx={{ color: TONE_COLOR[completionTone(factPct)] }}>{pctLabel(factPct)}</Typography>}
        </TableCell>
        <TableCell align="right">
          {brand == null ? (
            poolReached
              ? <Chip size="small" color="success" variant="outlined" label={`${isLast ? 'достигнута' : 'пройдена'} · ${pctLabel(poolForecast != null && threshold ? (poolForecast / threshold) * 100 : null)}`} sx={{ fontSize: 11, height: 20 }} />
              : <Typography variant="caption" color="text.secondary">
                  {threshold && poolForecast != null ? `${pctLabel((poolForecast / threshold) * 100)} · не хватает ${formatRubShort(threshold - poolForecast)}` : '—'}
                </Typography>
          ) : owner ? (
            reachedByForecast
              ? <Chip size="small" color="success" variant="outlined" label={`${isLast ? 'достигнута' : 'пройдена'} · ${pctLabel(forecastPct)}`} sx={{ fontSize: 11, height: 20 }} />
              : <Typography variant="caption" color="text.secondary">
                  {threshold && row.forecast != null ? `${pctLabel(forecastPct)} · не хватает ${formatRubShort(threshold - row.forecast)}` : '—'}
                </Typography>
          ) : (
            reachedByForecast
              ? <Chip size="small" color="success" variant="outlined" label="по пулу: пройдена" sx={{ fontSize: 11, height: 20 }} />
              : <Typography variant="caption" color="text.secondary">по пулу: не пройдена</Typography>
          )}
        </TableCell>
        <TableCell>
          {brand != null && (
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
              {isFirst
                ? (
                  <Typography variant="body2" sx={{ textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
                    {cell.investmentsPct.trim() === '' ? '—' : `${cell.investmentsPct} %`}
                  </Typography>
                ) : (
                  <PlanNumberField
                    value={scale?.investmentsPct ?? ''}
                    disabled={!canEdit}
                    placeholder={calc?.effectiveInvestmentsPct != null ? `${calc.effectiveInvestmentsPct}` : '—'}
                    onChange={(v) => onScaleChange(brand, scaleNo, { investmentsPct: v })}
                  />
                )}
              {!isFirst && (scale?.investmentsPct ?? '').trim() === '' && calc?.effectiveInvestmentsPct != null && (
                <Typography variant="caption" color="text.secondary" sx={{ lineHeight: 1.2 }}>как у ступени ниже</Typography>
              )}
              {skuChip(brand, scaleNo)}
            </Box>
          )}
          {brand == null && <Typography variant="body2" color="text.disabled">—</Typography>}
        </TableCell>
        <TableCell align="right">
          {brand != null
            ? <ValueCell value={calc?.investPlan ?? null} netValue={calc?.investPlanNet ?? null} muted={calc?.investPlan == null} />
            : <Typography variant="body2" color="text.disabled">—</Typography>}
        </TableCell>
        <TableCell align="right">
          {brand != null ? (
            <ValueCell
              value={calc?.investForecast ?? null}
              netValue={calc?.investForecastNet ?? null}
              muted={!effective}
              tone={effective ? 'good' : 'neutral'}
              hint={calc?.investForecast == null
                ? null
                : effective
                  ? calc.forecastBase != null && row.forecast != null && calc.forecastBase !== row.forecast
                    ? `база ${formatRubShort(calc.forecastBase)} из ${formatRubShort(row.forecast)}`
                    : 'достигнута'
                  : reachedByForecast ? 'пройдена' : 'если достигнута'}
            />
          ) : <Typography variant="body2" color="text.disabled">—</Typography>}
        </TableCell>
        <TableCell align="right">
          {brand != null && calc?.factReached && row.factScale === scaleNo
            ? <ValueCell value={calc.investFact} netValue={calc.investFactNet} hint="по факту" />
            : <Typography variant="body2" color="text.disabled">—</Typography>}
        </TableCell>
        <TableCell />
      </TableRow>
    );
  };

  // Строка крышки владельца порога и кнопка добавления ступени.
  const capRow = (brand: string | null) => {
    const cell = cellOf(brand);
    const owner = brand == null || !cell.inGross;
    const canAdd = canEdit && canAddScale(cell, quarterScales);
    return (
      <TableRow key={`${brand ?? ''}-cap`} sx={{ '& td': { py: 0.5, bgcolor: 'action.hover' } }}>
        <TableCell colSpan={COLUMNS.length} sx={{ pl: 4 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
            {owner ? (
              <>
                <Typography variant="caption" sx={{ fontWeight: 600 }}>Крышка перевыполнения</Typography>
                <ToggleButtonGroup
                  size="small"
                  exclusive
                  value={cell.capMode}
                  disabled={!canEdit}
                  onChange={(_, value: CapMode | null) => value && onCellChange(brand, { capMode: value })}
                  sx={{ '& .MuiToggleButton-root': { textTransform: 'none', px: 1, py: 0.25, fontSize: 12 } }}
                >
                  {CAP_MODES.map((mode) => <ToggleButton key={mode.value} value={mode.value}>{mode.label}</ToggleButton>)}
                </ToggleButtonGroup>
                {cell.capMode === 'pct' && (
                  <Box sx={{ width: 90 }}>
                    <PlanNumberField value={cell.capPct} disabled={!canEdit} suffix="%" onChange={(v) => onCellChange(brand, { capPct: v })} />
                  </Box>
                )}
                <Typography variant="caption" color="text.secondary">
                  {cell.capMode === 'closed'
                    ? 'не больше плана достигнутой ступени; коридоры между ступенями не оплачиваются'
                    : cell.capMode === 'pct'
                      ? 'на последней ступени база не выше плана × (1 + %); коридоры между ступенями оплачиваются целиком'
                      : 'весь объём; коридоры между ступенями оплачиваются целиком'}
                </Typography>
              </>
            ) : (
              <Typography variant="caption" color="text.secondary">
                Пороги и крышка — у пула ({capModeLabel(poolCell.capMode, poolCell.capPct)}); здесь — план бренда на каждой ступени.
              </Typography>
            )}
            <Box sx={{ flex: 1 }} />
            {canAdd && (
              <Button size="small" onClick={() => onAddScale(brand)}>
                + ступень {scaleCount(cell) + 1}
              </Button>
            )}
            {!canAdd && canEdit && scaleCount(cell) < 3 && (
              <Tooltip title="Число ступеней квартала задаётся в профиле сети">
                <Typography variant="caption" color="text.disabled">ступеней в квартале: {quarterScales}</Typography>
              </Tooltip>
            )}
          </Box>
        </TableCell>
      </TableRow>
    );
  };

  // Лесенка владельца порога: у пула — по объёму пула, у отдельного бренда — своему.
  const ladderRow = (brand: string | null) => {
    const cell = cellOf(brand);
    const row = amountsOf(brand);
    const thresholds = Array.from({ length: scaleCount(cell) }, (_, i) => i + 1)
      .map((no) => ({ scaleNo: no, value: scaleThreshold(cell, no) ?? 0, reached: scaleAmountsOf(row, no)?.forecastReached ?? false }))
      .filter((t) => t.value > 0);
    if (thresholds.length === 0) return null;
    const fact = brand == null ? (totals.gross_pool_fact_rub || null) : row.fact;
    const forecast = brand == null ? totals.gross_pool_forecast_rub : row.forecast;
    const last = thresholds[thresholds.length - 1];
    const capPct = parseNumberInput(cell.capPct);
    const capLimit = cell.capMode === 'pct' && capPct != null ? last.value * (1 + capPct / 100)
      : cell.capMode === 'closed' ? last.value : null;
    return (
      <TableRow key={`${brand ?? ''}-ladder`}>
        <TableCell colSpan={COLUMNS.length} sx={{ pl: 4, py: 0.5 }}>
          <NetworkScaleLadder thresholds={thresholds} fact={fact} forecast={forecast} capLimit={capLimit} />
        </TableCell>
      </TableRow>
    );
  };

  const expandedRows = (brand: string | null) => {
    if (!isExpanded(brand)) return null;
    const cell = cellOf(brand);
    const owner = brand == null || !cell.inGross;
    const count = scaleCount(cell);
    return (
      <>
        {Array.from({ length: count }, (_, i) => scaleRow(brand, i + 1))}
        {capRow(brand)}
        {owner && ladderRow(brand)}
      </>
    );
  };

  // Строка бренда: два поля ввода объёма, процент инвестиций и расчётные суммы.
  const brandRow = (brand: string) => {
    const cell = cellOf(brand);
    const row = amountsOf(brand);
    const factPct = row.plan ? deltaPct(row.fact, row.plan) : null;
    const forecastPct = deltaPct(row.forecast, row.plan);
    const hasComment = commentedCells.has(planKey(quarter, brand));
    const investmentPeriod = row.investmentPeriodStart === row.investmentPeriodEnd
      ? `Q${row.investmentPeriodStart}`
      : `Q${row.investmentPeriodStart}–Q${row.investmentPeriodEnd}`;
    const unitsEntry = cell.entryUnit === 'units';
    const ladder = ownerLadderLength(brand);
    const baseDiffers = row.forecastScale > 0 && row.forecastBase != null && row.forecast != null && row.forecastBase !== row.forecast;
    const payoutHint = row.payFromFact
      ? 'от факта · без порога'
      : row.forecastCompletionPct == null
        ? 'нет базы выполнения'
        : `${investmentPeriod} · ${row.forecastCompletionPct.toLocaleString('ru-RU', { maximumFractionDigits: 1 })} %`
          + (row.forecastScale > 0 && ladder > 1 ? ` · ст.${row.forecastScale}` : '')
          + (baseDiffers ? ` · база ${formatRubShort(row.forecastBase)} из ${formatRubShort(row.forecast)}` : '');

    return (
      <TableRow key={brand} hover>
        <TableCell>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25, minWidth: 0 }}>
            {expandButton(brand)}
            <Typography variant="body2" noWrap title={brand}>{brand}</Typography>
            <Tooltip title={hasComment ? 'Есть комментарий' : 'Комментарий к бренду за квартал'}>
              <IconButton size="small" sx={{ p: 0.25 }} onClick={() => onComment(brand)}>
                <CommentIcon fontSize="inherit" color={hasComment ? 'warning' : 'disabled'} />
              </IconButton>
            </Tooltip>
            <ScaleBadge count={ladder} reached={row.forecastScale} />
          </Box>
        </TableCell>
        <TableCell>
          <PlanNumberField
            value={unitsEntry ? cell.planUnits : cell.planRub}
            disabled={!canEdit}
            suffix={unitsEntry ? 'уп' : undefined}
            onChange={(v) => onCellChange(brand, unitsEntry ? { planUnits: v, planRub: '' } : { planRub: v, planUnits: '' })}
          />
          {unitsEntry && (
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', lineHeight: 1.3 }}>
              {row.plan != null ? `= ${formatRubShort(row.plan)} по ценам` : cell.planUnits.trim() ? 'нет цены контракта' : ''}
            </Typography>
          )}
          {!unitsEntry && row.planUnits != null && (
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', lineHeight: 1.3 }}>
              ≈ {row.planUnits.toLocaleString('ru-RU', { maximumFractionDigits: 0 })} уп
            </Typography>
          )}
        </TableCell>
        <TableCell align="right">
          <ValueCell
            value={row.fact}
            hint={factPct == null ? null : `${(100 + factPct).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} %`}
            tone={completionTone(factPct == null ? null : 100 + factPct)}
          />
        </TableCell>
        <TableCell align="right">
          <ValueCell value={row.forecast} />
          {!!forecastPct && (
            <Typography
              variant="caption"
              sx={{ display: 'block', textAlign: 'right', color: TONE_COLOR[deviationTone(forecastPct)] }}
            >
              {formatSignedPct(forecastPct)} к плану
            </Typography>
          )}
        </TableCell>
        <TableCell>
          <PlanNumberField
            value={cell.investmentsPct}
            disabled={!canEdit}
            onChange={(v) => onCellChange(brand, { investmentsPct: v })}
          />
          {ladder > 1 && upperScales(cell).length > 0 && (
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', lineHeight: 1.3 }}>
              {[cell.investmentsPct || '—', ...upperScales(cell).map((s) =>
                s.investmentsPct || (scaleAmountsOf(row, s.scaleNo)?.effectiveInvestmentsPct ?? '—'))].join(' · ')}
            </Typography>
          )}
        </TableCell>
        <TableCell align="right">
          <ValueCell value={row.investPlan} netValue={row.investPlanNet} />
        </TableCell>
        <TableCell align="right">
          <ValueCell value={row.investForecast} netValue={row.investForecastNet} />
          <Typography
            variant="caption"
            sx={{ display: 'block', color: row.forecastEarned ? 'text.secondary' : 'warning.main' }}
          >
            {payoutHint}
          </Typography>
        </TableCell>
        <TableCell align="right">
          <ValueCell value={row.investFact} netValue={row.investFactNet} />
        </TableCell>
        <TableCell padding="none">
          {canEdit && (
            <IconButton size="small" onClick={(e) => setMenu({ anchor: e.currentTarget, brand })}>
              <MoreIcon fontSize="inherit" />
            </IconButton>
          )}
        </TableCell>
      </TableRow>
    );
  };

  // Подытог группы: расчётные суммы без полей ввода.
  const subtotalRow = (
    label: string,
    values: {
      plan: number; fact: number; forecast: number;
      investPlan: number; investPlanNet: number;
      investForecast: number; investForecastNet: number;
      investFact: number; investFactNet: number;
    },
  ) => {
    const factPct = deltaPct(values.fact, values.plan);
    const forecastPct = deltaPct(values.forecast, values.plan);
    return (
      <TableRow>
        <TableCell sx={{ color: 'text.secondary' }}>{label}</TableCell>
        <TableCell align="right"><ValueCell value={values.plan} bold /></TableCell>
        <TableCell align="right">
          <ValueCell
            value={values.fact || null}
            hint={factPct == null ? null : `${(100 + factPct).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} %`}
            tone={completionTone(factPct == null ? null : 100 + factPct)}
            bold
          />
        </TableCell>
        <TableCell align="right">
          <ValueCell
            value={values.forecast || null}
            hint={forecastPct ? `${formatSignedPct(forecastPct)} к плану` : null}
            tone={deviationTone(forecastPct)}
            bold
          />
        </TableCell>
        <TableCell />
        <TableCell align="right">
          <ValueCell value={values.investPlan || null} netValue={values.investPlanNet || null} bold />
        </TableCell>
        <TableCell align="right">
          <ValueCell value={values.investForecast || null} netValue={values.investForecastNet || null} bold />
        </TableCell>
        <TableCell align="right">
          <ValueCell value={values.investFact || null} netValue={values.investFactNet || null} bold />
        </TableCell>
        <TableCell />
      </TableRow>
    );
  };

  const groupSums = (list: string[]) =>
    list.reduce(
      (acc, brand) => {
        const a = amountsOf(brand);
        acc.plan += a.plan ?? 0;
        acc.fact += a.fact ?? 0;
        acc.forecast += a.forecast ?? 0;
        acc.investPlan += a.investPlan ?? 0;
        acc.investPlanNet += a.investPlanNet ?? 0;
        acc.investForecast += a.investForecast ?? 0;
        acc.investForecastNet += a.investForecastNet ?? 0;
        acc.investFact += a.investFact ?? 0;
        acc.investFactNet += a.investFactNet ?? 0;
        return acc;
      },
      {
        plan: 0, fact: 0, forecast: 0,
        investPlan: 0, investPlanNet: 0,
        investForecast: 0, investForecastNet: 0,
        investFact: 0, investFactNet: 0,
      },
    );

  const menuBrandInGross = menu ? cellOf(menu.brand).inGross : false;
  const dialogCell = skuDialog ? cellOf(skuDialog.brand) : null;
  const dialogScale = skuDialog && dialogCell ? dialogCell.scales.find((s) => s.scaleNo === skuDialog.scaleNo) : undefined;
  const dialogBrandPlan = skuDialog && dialogCell
    ? (skuDialog.scaleNo === 1
      ? parseNumberInput(dialogCell.entryUnit === 'units' ? dialogCell.planUnits : dialogCell.planRub)
      : parseNumberInput(dialogCell.entryUnit === 'units' ? (dialogScale?.planUnits ?? '') : (dialogScale?.planRub ?? '')))
    : null;
  const dialogOwner = dialogCell ? (dialogCell.inGross ? poolCell : dialogCell) : null;

  return (
    <Paper variant="outlined" sx={{ overflowX: 'auto' }}>
      <Table size="small" sx={{
          tableLayout: 'fixed',
          minWidth: 880,
          '& thead th': { fontSize: 12, px: 1, whiteSpace: 'nowrap', lineHeight: 1.3 },
        }}>
        <colgroup>
          {COLUMNS.map((width, index) => <col key={index} style={{ width }} />)}
        </colgroup>
        <TableHead>
          <TableRow>
            <TableCell>Бренд</TableCell>
            <TableCell align="right">План, ₽</TableCell>
            <TableCell align="right">Факт, ₽</TableCell>
            <TableCell align="right">Прогноз, ₽</TableCell>
            <TableCell align="right">Инв., %</TableCell>
            <TableCell align="right">Инв. план</TableCell>
            <TableCell align="right">К выплате</TableCell>
            <TableCell align="right">Инв. факт</TableCell>
            <TableCell padding="none" />
          </TableRow>
        </TableHead>
        <TableBody>
          {hasPool && (
            <>
              {sectionRow(
                'Валовый объём контракта',
                `${grossBrands.length} ${pluralRu(grossBrands.length, 'бренд', 'бренда', 'брендов')} распределяют общий объём`
                + (scaleCount(poolCell) > 1 || poolCell.capMode !== 'open' ? ` · ${scaleCount(poolCell)} ${pluralRu(scaleCount(poolCell), 'ступень', 'ступени', 'ступеней')}, ${capModeLabel(poolCell.capMode, poolCell.capPct)}` : ''),
              )}
              <TableRow sx={{ bgcolor: 'action.selected' }}>
                <TableCell>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25 }}>
                    {expandButton(null)}
                    <Typography variant="body2" sx={{ fontWeight: 600 }}>Общий объём</Typography>
                    <Tooltip title="Комментарий к общему объёму">
                      <IconButton size="small" onClick={() => onComment(null)}>
                        <CommentIcon
                          fontSize="inherit"
                          color={commentedCells.has(planKey(quarter, null)) ? 'warning' : 'disabled'}
                        />
                      </IconButton>
                    </Tooltip>
                    <ScaleBadge count={scaleCount(poolCell)} reached={pool.forecastScale} />
                  </Box>
                </TableCell>
                <TableCell>
                  <PlanNumberField
                    value={poolCell.planRub}
                    disabled={!canEdit}
                    onChange={(v) => onCellChange(null, { planRub: v })}
                  />
                </TableCell>
                <TableCell align="right">
                  <ValueCell value={totals.gross_pool_fact_rub || null} muted={!totals.gross_pool_fact_rub} />
                </TableCell>
                <TableCell align="right">
                  <ValueCell value={totals.gross_pool_forecast_rub} muted={totals.gross_pool_forecast_rub == null} />
                </TableCell>
                <TableCell align="right"><Typography variant="body2" color="text.disabled">—</Typography></TableCell>
                <TableCell align="right"><Typography variant="body2" color="text.disabled">—</Typography></TableCell>
                <TableCell align="right">
                  {pool.forecastScale > 0 && scaleCount(poolCell) > 1
                    ? <Typography variant="caption" color="success.main">достигнута ступень {pool.forecastScale}</Typography>
                    : <Typography variant="body2" color="text.disabled">—</Typography>}
                </TableCell>
                <TableCell align="right"><Typography variant="body2" color="text.disabled">—</Typography></TableCell>
                <TableCell />
              </TableRow>
              {expandedRows(null)}

              {totals.undistributed != null && (
                <TableRow>
                  <TableCell colSpan={COLUMNS.length} sx={{ py: 0.5 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
                      <Typography variant="caption" color="text.secondary">Остаток к распределению{scaleCount(poolCell) > 1 ? ' · ступень 1' : ''}</Typography>
                      <Chip
                        size="small"
                        variant={totals.undistributed === 0 ? 'outlined' : 'filled'}
                        color={totals.undistributed === 0 ? 'success' : 'warning'}
                        label={formatRubShort(totals.undistributed)}
                      />
                      {canEdit && totals.undistributed !== 0 && grossBrands.length > 0 && (
                        <Button size="small" onClick={() => onDistributeRest(1)}>Распределить поровну</Button>
                      )}
                    </Box>
                  </TableCell>
                </TableRow>
              )}

              {grossBrands.map((brand) => (
                <Fragment key={brand}>
                  {brandRow(brand)}
                  {expandedRows(brand)}
                </Fragment>
              ))}
              {grossBrands.length === 0 && (
                <TableRow>
                  <TableCell colSpan={COLUMNS.length}>
                    <Typography variant="body2" color="text.secondary" sx={{ py: 1 }}>
                      В валовый объём пока не отнесён ни один бренд. Откройте меню строки бренда
                      и выберите «Перевести в валовый объём».
                    </Typography>
                  </TableCell>
                </TableRow>
              )}
              {grossBrands.length > 0 && subtotalRow('Итого в валовом объёме', groupSums(grossBrands))}
            </>
          )}

          {sectionRow('Отдельные бренды', 'планируются вне общего объёма · крышка на бренде')}
          {separateBrands.map((brand) => (
            <Fragment key={brand}>
              {brandRow(brand)}
              {expandedRows(brand)}
            </Fragment>
          ))}
          {separateBrands.length === 0 && (
            <TableRow>
              <TableCell colSpan={COLUMNS.length}>
                <Typography variant="body2" color="text.secondary" sx={{ py: 1 }}>
                  {brands.length === 0
                    ? 'Брендов в плане пока нет. Добавьте бренд, чтобы внести суммы.'
                    : 'Все бренды входят в валовый объём.'}
                </Typography>
              </TableCell>
            </TableRow>
          )}
          {separateBrands.length > 0 && subtotalRow('Итого отдельно', groupSums(separateBrands))}

          <TableRow sx={{ bgcolor: 'action.hover' }}>
            <TableCell sx={{ fontWeight: 600 }}>Итого Q{quarter}</TableCell>
            <TableCell align="right">
              <ValueCell
                value={totals.contract_plan_rub}
                hint={totals.gross_pool_rub != null ? 'пул + отдельные' : null}
                bold
              />
            </TableCell>
            <TableCell align="right">
              <ValueCell
                value={totals.fact_rub || null}
                hint={deltaPct(totals.fact_rub, totals.contract_plan_rub) == null
                  ? null
                  : `${(100 + (deltaPct(totals.fact_rub, totals.contract_plan_rub) ?? 0)).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} %`}
                tone={completionTone(
                  deltaPct(totals.fact_rub, totals.contract_plan_rub) == null
                    ? null
                    : 100 + (deltaPct(totals.fact_rub, totals.contract_plan_rub) ?? 0),
                )}
                bold
              />
            </TableCell>
            <TableCell align="right">
              <ValueCell
                value={totals.forecast_rub || null}
                hint={deltaPct(totals.forecast_rub, totals.contract_plan_rub)
                  ? `${formatSignedPct(deltaPct(totals.forecast_rub, totals.contract_plan_rub))} к плану`
                  : null}
                tone={deviationTone(deltaPct(totals.forecast_rub, totals.contract_plan_rub))}
                bold
              />
            </TableCell>
            <TableCell />
            <TableCell align="right">
              <ValueCell value={totals.investments_rub || null} netValue={totals.investments_rub_net || null} bold />
            </TableCell>
            <TableCell align="right">
              <ValueCell
                value={totals.forecast_investments_rub || null}
                netValue={totals.forecast_investments_rub_net || null}
                bold
              />
            </TableCell>
            <TableCell align="right">
              <ValueCell
                value={totals.fact_investments_rub || null}
                netValue={totals.fact_investments_rub_net || null}
                bold
              />
            </TableCell>
            <TableCell />
          </TableRow>
        </TableBody>
      </Table>

      <Menu anchorEl={menu?.anchor ?? null} open={!!menu} onClose={() => setMenu(null)}>
        <MenuItem
          onClick={() => {
            if (menu) onToggleGross(menu.brand, !menuBrandInGross, false);
            setMenu(null);
          }}
        >
          {menuBrandInGross ? `Вывести из валового объёма (Q${quarter})` : `Перевести в валовый объём (Q${quarter})`}
        </MenuItem>
        <MenuItem
          onClick={() => {
            if (menu) onToggleGross(menu.brand, !menuBrandInGross, true);
            setMenu(null);
          }}
        >
          {menuBrandInGross ? 'Вывести из валового объёма во всех кварталах' : 'Перевести в валовый объём во всех кварталах'}
        </MenuItem>
        {menu && canAddScale(cellOf(menu.brand), quarterScales) && (
          <MenuItem
            onClick={() => {
              onAddScale(menu.brand);
              setExpanded((current) => new Set(current).add(menu.brand));
              setMenu(null);
            }}
          >
            Добавить ступень {scaleCount(cellOf(menu.brand)) + 1}
          </MenuItem>
        )}
        <MenuItem
          onClick={() => {
            if (menu) onRemoveBrand(menu.brand);
            setMenu(null);
          }}
        >
          Убрать бренд из плана
        </MenuItem>
      </Menu>

      {skuDialog && dialogCell && dialogOwner && (
        <NetworkScaleSKUDialog
          open
          networkId={networkId}
          year={year}
          quarter={quarter}
          brand={skuDialog.brand}
          scaleNo={skuDialog.scaleNo}
          entryUnit={dialogCell.entryUnit}
          brandPlan={dialogBrandPlan}
          brandPct={skuDialog.scaleNo === 1 ? dialogCell.investmentsPct : (dialogScale?.investmentsPct ?? '')}
          ownerCapMode={dialogOwner.capMode}
          ownerCapPct={dialogOwner.capPct}
          skus={skusOf(dialogCell, skuDialog.scaleNo)}
          amounts={scaleAmountsOf(amountsOf(skuDialog.brand), skuDialog.scaleNo)}
          canEdit={canEdit}
          onApply={(skus) => {
            onSkusChange(skuDialog.brand, skuDialog.scaleNo, skus);
            setSkuDialog(null);
          }}
          onClose={() => setSkuDialog(null)}
        />
      )}
    </Paper>
  );
}
