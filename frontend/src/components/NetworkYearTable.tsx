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
  calcCell,
  EMPTY_CELL,
  formatPct,
  formatRubShort,
  planKey,
  round2,
} from '../utils/networkPlan';
import type { DraftCell, QuarterSettings, QuarterTotals } from '../utils/networkPlan';
import { PlanNumberField, ValueCell } from './networkPlanCells';
import type { YearMetric } from '../utils/networkPlanView';

// Величины, которые вводятся руками; остальные считаются.
type EditableField = 'planRub' | 'forecastRub' | 'investmentsPct';

const EDITABLE: Record<YearMetric, EditableField | null> = {
  plan: 'planRub',
  forecast: 'forecastRub',
  pct: 'investmentsPct',
  fact: null,
  investPlan: null,
  investForecast: null,
  investFact: null,
};

const COLUMNS = ['22%', '15%', '13%', '13%', '13%', '13%', '11%', '44px'];

interface NetworkYearTableProps {
  metric: YearMetric;
  brands: string[];
  draft: Record<string, DraftCell>;
  settings: Record<number, QuarterSettings>;
  totals: QuarterTotals[];
  canEdit: boolean;
  onCellChange: (quarter: number, brand: string | null, patch: Partial<DraftCell>) => void;
  onToggleGross: (brand: string, next: boolean, allQuarters: boolean, quarter?: number) => void;
  onRemoveBrand: (brand: string) => void;
}

export default function NetworkYearTable({
  metric,
  brands,
  draft,
  settings,
  totals,
  canEdit,
  onCellChange,
  onToggleGross,
  onRemoveBrand,
}: NetworkYearTableProps) {
  const [menu, setMenu] = useState<{ anchor: HTMLElement; brand: string } | null>(null);

  const cellOf = (quarter: number, brand: string | null): DraftCell =>
    draft[planKey(quarter, brand)] ?? EMPTY_CELL;

  const field = EDITABLE[metric];
  const isMoney = metric !== 'pct';
  const hasAnyGross = QUARTERS.some((q) => totals[q - 1].grossBrandsCount > 0 || totals[q - 1].grossPoolRub != null);

  // Рассчитанное значение метрики в ячейке бренда.
  const computed = (quarter: number, brand: string): number | null => {
    const amounts = calcCell(cellOf(quarter, brand), settings[quarter]);
    switch (metric) {
      case 'fact': return amounts.fact;
      case 'investPlan': return amounts.investPlan;
      case 'investForecast': return amounts.investForecast;
      case 'investFact': return amounts.investFact;
      case 'plan': return amounts.plan;
      case 'forecast': return amounts.forecast;
      case 'pct': return null;
      default: return null;
    }
  };

  // Итог года по бренду: суммы складываются, процент считается средневзвешенным.
  const brandYearTotal = (brand: string): number | null => {
    if (metric === 'pct') {
      let plan = 0;
      let investments = 0;
      QUARTERS.forEach((q) => {
        const a = calcCell(cellOf(q, brand), settings[q]);
        plan += a.plan ?? 0;
        investments += a.investPlan ?? 0;
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
          {field ? (
            <PlanNumberField
              value={cellOf(quarter, brand)[field]}
              disabled={!canEdit}
              onChange={(v) => onCellChange(quarter, brand, { [field]: v })}
            />
          ) : (
            <ValueCell value={computed(quarter, brand)} />
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
  const poolField = metric === 'plan' ? 'planRub' : metric === 'forecast' ? 'forecastRub' : null;
  const poolValue = (quarter: number): number | null => {
    if (metric === 'fact') return totals[quarter - 1].grossPoolFactRub || null;
    if (metric === 'plan') return totals[quarter - 1].grossPoolRub;
    if (metric === 'forecast') return totals[quarter - 1].grossPoolForecastRub;
    return null;
  };

  const yearSum = (pick: (t: QuarterTotals) => number | null): number | null => {
    let sum = 0;
    let seen = false;
    totals.forEach((t) => {
      const value = pick(t);
      if (value != null) {
        sum = round2(sum + value);
        seen = true;
      }
    });
    return seen ? sum : null;
  };

  const totalsRowValue = (t: QuarterTotals): number | null => {
    switch (metric) {
      case 'plan': return t.contractPlanRub || null;
      case 'fact': return t.factRub || null;
      case 'forecast': return t.forecastRub || null;
      case 'investPlan': return t.investmentsRub || null;
      case 'investForecast': return t.forecastInvestmentsRub || null;
      case 'investFact': return t.factInvestmentsRub || null;
      case 'pct': return t.planRub > 0 ? round2((t.investmentsRub / t.planRub) * 100) : null;
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
                  {poolField ? (
                    <PlanNumberField
                      value={draft[planKey(quarter, null)]?.[poolField] ?? ''}
                      disabled={!canEdit}
                      onChange={(v) => onCellChange(quarter, null, { [poolField]: v })}
                    />
                  ) : (
                    <ValueCell value={poolValue(quarter)} muted={poolValue(quarter) == null} />
                  )}
                </TableCell>
              ))}
              <TableCell align="right">
                <ValueCell value={yearSum((t) => poolValue(t.quarter))} bold />
              </TableCell>
              <TableCell />
            </TableRow>
          )}

          {hasAnyGross && metric === 'plan' && (
            <TableRow>
              <TableCell sx={{ color: 'text.secondary' }} colSpan={2}>
                <Typography variant="caption">Остаток к распределению</Typography>
              </TableCell>
              {QUARTERS.map((quarter) => {
                const rest = totals[quarter - 1].undistributed;
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
                <ValueCell value={yearSum((t) => t.undistributed)} />
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
                <ValueCell value={yearSum(totalsRowValue)} bold />
              ) : (
                <Typography variant="body2" sx={{ fontWeight: 600 }}>
                  {formatPct(
                    totals.reduce((sum, t) => sum + t.planRub, 0) > 0
                      ? round2(
                          (totals.reduce((sum, t) => sum + t.investmentsRub, 0) /
                            totals.reduce((sum, t) => sum + t.planRub, 0)) * 100,
                        )
                      : null,
                  )}
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
