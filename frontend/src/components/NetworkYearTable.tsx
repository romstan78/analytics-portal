// Годовой разрез: одна величина по четырём кварталам сразу.
// Так вносят план на год и сравнивают кварталы между собой, не открывая каждый.

import { useState } from 'react';
import {
  Box,
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
  Tooltip,
  Typography,
} from '@mui/material';
import { MoreVert as MoreIcon } from '@mui/icons-material';
import {
  QUARTERS,
  EMPTY_AMOUNTS,
  EMPTY_CELL,
  formatPct,
  formatRubShort,
  planKey,
  round2,
  scaleAmountsOf,
  upperScales,
} from '../utils/networkPlan';
import type { CellAmounts, DraftCell, DraftScale } from '../utils/networkPlan';
import type { NetworkPlanTotals } from '../types/network';
import { PlanNumberField, ValueCell } from './networkPlanCells';
import type { YearMetric } from '../utils/networkPlanView';

// Величины, которые вводятся руками; остальные считаются.
type EditableField = 'planRub' | 'investmentsPct';

const EDITABLE: Record<YearMetric, EditableField | null> = {
  plan: 'planRub',
  forecast: null,
  pct: 'investmentsPct',
  fact: null,
  investPlan: null,
  investForecast: null,
  investFact: null,
};

const COLUMNS = ['22%', '15%', '13%', '13%', '13%', '13%', '11%', '44px'];

interface NetworkYearTableProps {
  metric: YearMetric;
  // Ступень, которую показывают и вводят метрики «План», «Инв., %» и
  // «Инв. план». Ступень 1 — сама строка, верхние — её ступени.
  scale: number;
  // Сколько ступеней разрешает каждый квартал (по номеру квартала).
  quarterScales: Record<number, number>;
  brands: string[];
  draft: Record<string, DraftCell>;
  amounts: Record<string, CellAmounts>;
  totals: NetworkPlanTotals[];
  yearTotals: NetworkPlanTotals;
  canEdit: boolean;
  onCellChange: (quarter: number, brand: string | null, patch: Partial<DraftCell>) => void;
  onScaleChange: (quarter: number, brand: string | null, scaleNo: number, patch: Partial<DraftScale>) => void;
  onToggleGross: (brand: string, next: boolean, allQuarters: boolean, quarter?: number) => void;
  onRemoveBrand: (brand: string) => void;
}

export default function NetworkYearTable({
  metric,
  scale,
  quarterScales,
  brands,
  draft,
  amounts,
  totals,
  yearTotals,
  canEdit,
  onCellChange,
  onScaleChange,
  onToggleGross,
  onRemoveBrand,
}: NetworkYearTableProps) {
  const [menu, setMenu] = useState<{ anchor: HTMLElement; brand: string } | null>(null);

  const cellOf = (quarter: number, brand: string | null): DraftCell =>
    draft[planKey(quarter, brand)] ?? EMPTY_CELL;
  const amountsOf = (quarter: number, brand: string | null): CellAmounts =>
    amounts[planKey(quarter, brand)] ?? EMPTY_AMOUNTS;

  const field = EDITABLE[metric];
  const isMoney = metric !== 'pct';
  const hasAnyGross = QUARTERS.some((q) => totals[q - 1].gross_brands_count > 0 || totals[q - 1].gross_pool_rub != null);
  const upper = scale > 1;
  const scaleTotalsOf = (t: NetworkPlanTotals) => t.scales?.find((s) => s.scale_no === scale);

  // Верхнюю ступень можно вводить, если квартал её разрешает и ступень ниже
  // уже есть: дыр в лестнице не бывает.
  const canEnterScale = (quarter: number, brand: string | null): boolean => {
    if (!upper) return true;
    if ((quarterScales[quarter] ?? 1) < scale) return false;
    const cell = cellOf(quarter, brand);
    return scale === 2 || upperScales(cell).some((s) => s.scaleNo === scale - 1);
  };
  const scaleValue = (quarter: number, brand: string | null, key: 'planRub' | 'investmentsPct'): string =>
    cellOf(quarter, brand).scales.find((s) => s.scaleNo === scale)?.[key] ?? '';

  // Рассчитанное значение метрики в ячейке бренда.
  const computed = (quarter: number, brand: string): number | null => {
    const row = amountsOf(quarter, brand);
    switch (metric) {
      case 'fact': return row.fact;
      case 'investPlan': return upper ? scaleAmountsOf(row, scale)?.investPlan ?? null : row.investPlan;
      case 'investForecast': return row.investForecast;
      case 'investFact': return row.investFact;
      case 'plan': return upper ? scaleAmountsOf(row, scale)?.plan ?? null : row.plan;
      case 'forecast': return row.forecast;
      case 'pct': return null;
      default: return null;
    }
  };

  // Достигнутая ступень — надстрочным индексом у прогнозных и фактических
  // инвестиций, когда у строки больше одной ступени.
  const reachedIndex = (quarter: number, brand: string): string | null => {
    const row = amountsOf(quarter, brand);
    const cell = cellOf(quarter, brand);
    const ladder = cell.inGross ? 1 + upperScales(cellOf(quarter, null)).length : 1 + upperScales(cell).length;
    if (ladder <= 1) return null;
    const reached = metric === 'investForecast' ? row.forecastScale : metric === 'investFact' ? row.factScale : 0;
    return reached > 0 ? `ст.${reached}` : null;
  };

  // Итог года по бренду: суммы складываются, процент считается средневзвешенным.
  const brandYearTotal = (brand: string): number | null => {
    if (metric === 'pct') {
      let plan = 0;
      let investments = 0;
      QUARTERS.forEach((q) => {
        const a = amountsOf(q, brand);
        const s = upper ? scaleAmountsOf(a, scale) : null;
        plan += (upper ? s?.plan : a.plan) ?? 0;
        investments += (upper ? s?.investPlan : a.investPlan) ?? 0;
      });
      return plan > 0 ? round2((investments / plan) * 100) : null;
    }
    let sum = 0;
    let seen = false;
    QUARTERS.forEach((q) => {
      const value = computed(q, brand);
      if (value != null) {
        sum = round2(sum + value);
        seen = true;
      }
    });
    return seen ? sum : null;
  };

  const brandRow = (brand: string) => (
    <TableRow key={brand} hover>
      <TableCell>
        <Typography variant="body2" noWrap title={brand}>{brand}</Typography>
      </TableCell>
      <TableCell>
        <Box sx={{ display: 'flex', gap: 0.5 }}>
          {QUARTERS.map((quarter) => {
            const active = cellOf(quarter, brand).inGross;
            return (
              <Tooltip
                key={quarter}
                title={active ? `Q${quarter}: в валовом объёме` : `Q${quarter}: отдельно`}
              >
                <Chip
                  size="small"
                  label={`Q${quarter}`}
                  color={active ? 'primary' : 'default'}
                  variant={active ? 'filled' : 'outlined'}
                  onClick={canEdit ? () => onToggleGross(brand, !active, false, quarter) : undefined}
                  sx={{ minWidth: 38, '& .MuiChip-label': { px: 0.75, fontSize: 11 } }}
                />
              </Tooltip>
            );
          })}
        </Box>
      </TableCell>
      {QUARTERS.map((quarter) => (
        <TableCell key={quarter} align="right">
          {field && !upper ? (
            <PlanNumberField
              value={cellOf(quarter, brand)[field]}
              disabled={!canEdit}
              onChange={(v) => onCellChange(quarter, brand, { [field]: v })}
            />
          ) : field && upper ? (
            canEnterScale(quarter, brand) ? (
              <PlanNumberField
                value={scaleValue(quarter, brand, field)}
                disabled={!canEdit}
                placeholder={field === 'investmentsPct'
                  ? `${scaleAmountsOf(amountsOf(quarter, brand), scale)?.effectiveInvestmentsPct ?? '—'}`
                  : undefined}
                onChange={(v) => onScaleChange(quarter, brand, scale, field === 'planRub' ? { planRub: v, planUnits: '' } : { investmentsPct: v })}
              />
            ) : (
              <Tooltip title={(quarterScales[quarter] ?? 1) < scale ? `Квартал допускает ${quarterScales[quarter] ?? 1} ${(quarterScales[quarter] ?? 1) === 1 ? 'ступень' : 'ступени'}` : `Сначала заведите ступень ${scale - 1}`}>
                <Typography variant="body2" color="text.disabled">—</Typography>
              </Tooltip>
            )
          ) : (
            <ValueCell value={computed(quarter, brand)} hint={reachedIndex(quarter, brand)} />
          )}
        </TableCell>
      ))}
      <TableCell align="right">
        {isMoney ? (
          <ValueCell value={brandYearTotal(brand)} bold />
        ) : (
          <Typography variant="body2" sx={{ fontWeight: 600 }}>{formatPct(brandYearTotal(brand))}</Typography>
        )}
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

  // Строка валового пула: у неё есть только объём — план и прогноз.
  const poolField = metric === 'plan' ? 'planRub' : null;
  const poolOf = (t: NetworkPlanTotals): number | null => {
    if (metric === 'fact') return t.gross_pool_fact_rub || null;
    if (metric === 'plan') return upper ? scaleTotalsOf(t)?.gross_pool_rub ?? null : t.gross_pool_rub;
    if (metric === 'forecast') return t.gross_pool_forecast_rub;
    return null;
  };
  const poolValue = (quarter: number): number | null => poolOf(totals[quarter - 1]);
  const poolYearValue = (): number | null => poolOf(yearTotals);

  const totalsRowValue = (t: NetworkPlanTotals): number | null => {
    const st = upper ? scaleTotalsOf(t) : null;
    switch (metric) {
      case 'plan': return (upper ? st?.contract_plan_rub : t.contract_plan_rub) || null;
      case 'fact': return t.fact_rub || null;
      case 'forecast': return t.forecast_rub || null;
      case 'investPlan': return (upper ? st?.investments_rub : t.investments_rub) || null;
      case 'investForecast': return t.forecast_investments_rub || null;
      case 'investFact': return t.fact_investments_rub || null;
      case 'pct': {
        const plan = upper ? (st?.gross_brands_plan ?? 0) + (st?.separate_plan_rub ?? 0) : t.plan_rub;
        const invest = upper ? st?.investments_rub ?? 0 : t.investments_rub;
        return plan > 0 ? round2((invest / plan) * 100) : null;
      }
      default: return null;
    }
  };

  const menuBrandGrossEverywhere = menu ? QUARTERS.every((q) => cellOf(q, menu.brand).inGross) : false;

  return (
    <Paper variant="outlined" sx={{ overflowX: 'auto' }}>
      <Table size="small" sx={{
          tableLayout: 'fixed',
          minWidth: 860,
          '& thead th': { fontSize: 12, px: 1, whiteSpace: 'nowrap', lineHeight: 1.3 },
        }}>
        <colgroup>
          {COLUMNS.map((width, index) => <col key={index} style={{ width }} />)}
        </colgroup>
        <TableHead>
          <TableRow>
            <TableCell>Бренд</TableCell>
            <TableCell>
              <Tooltip title="В каких кварталах бренд входит в валовый объём контракта">
                <span>Валовый объём</span>
              </Tooltip>
            </TableCell>
            {QUARTERS.map((quarter) => (
              <TableCell key={quarter} align="right">Q{quarter}</TableCell>
            ))}
            <TableCell align="right">Год</TableCell>
            <TableCell padding="none" />
          </TableRow>
        </TableHead>
        <TableBody>
          {hasAnyGross && (
            <TableRow sx={{ bgcolor: 'action.selected' }}>
              <TableCell sx={{ fontWeight: 600 }}>Общий объём</TableCell>
              <TableCell>
                <Typography variant="caption" color="text.secondary">валовый пул</Typography>
              </TableCell>
              {QUARTERS.map((quarter) => (
                <TableCell key={quarter} align="right">
                  {poolField && !upper ? (
                    <PlanNumberField
                      value={draft[planKey(quarter, null)]?.[poolField] ?? ''}
                      disabled={!canEdit}
                      onChange={(v) => onCellChange(quarter, null, { [poolField]: v })}
                    />
                  ) : poolField && upper ? (
                    canEnterScale(quarter, null) ? (
                      <PlanNumberField
                        value={scaleValue(quarter, null, 'planRub')}
                        disabled={!canEdit}
                        onChange={(v) => onScaleChange(quarter, null, scale, { planRub: v, planUnits: '' })}
                      />
                    ) : (
                      <Typography variant="body2" color="text.disabled">—</Typography>
                    )
                  ) : (
                    <ValueCell value={poolValue(quarter)} muted={poolValue(quarter) == null} />
                  )}
                </TableCell>
              ))}
              <TableCell align="right">
                <ValueCell value={poolYearValue()} bold />
              </TableCell>
              <TableCell />
            </TableRow>
          )}

          {hasAnyGross && metric === 'plan' && (
            <TableRow>
              <TableCell sx={{ color: 'text.secondary' }} colSpan={2}>
                <Typography variant="caption">Остаток к распределению{upper ? ` · ступень ${scale}` : ''}</Typography>
              </TableCell>
              {QUARTERS.map((quarter) => {
                const rest = upper ? scaleTotalsOf(totals[quarter - 1])?.undistributed ?? null : totals[quarter - 1].undistributed;
                return (
                  <TableCell key={quarter} align="right">
                    {rest == null ? (
                      <Typography variant="body2" color="text.disabled">—</Typography>
                    ) : (
                      <Chip
                        size="small"
                        variant={rest === 0 ? 'outlined' : 'filled'}
                        color={rest === 0 ? 'success' : 'warning'}
                        label={formatRubShort(rest)}
                      />
                    )}
                  </TableCell>
                );
              })}
              <TableCell align="right">
                <ValueCell value={upper ? scaleTotalsOf(yearTotals)?.undistributed ?? null : yearTotals.undistributed} />
              </TableCell>
              <TableCell />
            </TableRow>
          )}

          {brands.map(brandRow)}

          {brands.length === 0 && (
            <TableRow>
              <TableCell colSpan={COLUMNS.length}>
                <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
                  Брендов в плане пока нет. Добавьте бренд, чтобы внести суммы по кварталам.
                </Typography>
              </TableCell>
            </TableRow>
          )}

          <TableRow sx={{ bgcolor: 'action.hover' }}>
            <TableCell sx={{ fontWeight: 600 }} colSpan={2}>Итого по сети</TableCell>
            {QUARTERS.map((quarter) => (
              <TableCell key={quarter} align="right">
                {isMoney ? (
                  <ValueCell value={totalsRowValue(totals[quarter - 1])} bold />
                ) : (
                  <Typography variant="body2" sx={{ fontWeight: 600 }}>
                    {formatPct(totalsRowValue(totals[quarter - 1]))}
                  </Typography>
                )}
              </TableCell>
            ))}
            <TableCell align="right">
              {isMoney ? (
                <ValueCell value={totalsRowValue(yearTotals)} bold />
              ) : (
                <Typography variant="body2" sx={{ fontWeight: 600 }}>
                  {formatPct(totalsRowValue(yearTotals))}
                </Typography>
              )}
            </TableCell>
            <TableCell />
          </TableRow>
        </TableBody>
      </Table>

      <Menu anchorEl={menu?.anchor ?? null} open={!!menu} onClose={() => setMenu(null)}>
        <MenuItem
          onClick={() => {
            if (menu) onToggleGross(menu.brand, !menuBrandGrossEverywhere, true);
            setMenu(null);
          }}
        >
          {menuBrandGrossEverywhere
            ? 'Вывести из валового объёма во всех кварталах'
            : 'Перевести в валовый объём во всех кварталах'}
        </MenuItem>
        <MenuItem
          onClick={() => {
            if (menu) onRemoveBrand(menu.brand);
            setMenu(null);
          }}
        >
          Убрать бренд из плана
        </MenuItem>
      </Menu>
    </Paper>
  );
}
