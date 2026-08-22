// Детальный разрез одного квартала: план, факт, прогноз и инвестиции рядом.
// Бренды разложены на две группы — входящие в валовый объём контракта и
// планируемые отдельно, потому что валовый объём применяется к брендам,
// а не к контракту целиком.

import { useState } from 'react';
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
  Tooltip,
  Typography,
} from '@mui/material';
import {
  ChatBubbleOutlineOutlined as CommentIcon,
  MoreVert as MoreIcon,
} from '@mui/icons-material';
import {
  deltaPct,
  formatRubShort,
  formatSignedPct,
  planKey,
  pluralRu,
} from '../utils/networkPlan';
import type { DraftCell, QuarterSettings, QuarterTotals } from '../utils/networkPlan';
import { calcCell, EMPTY_CELL } from '../utils/networkPlan';
import { PlanNumberField, ValueCell } from './networkPlanCells';
import { TONE_COLOR, completionTone, deviationTone } from '../utils/networkPlanView';

const COLUMNS = ['20%', '14%', '11%', '14%', '9%', '10%', '11%', '11%', '44px'];

interface NetworkQuarterTableProps {
  quarter: number;
  brands: string[];
  draft: Record<string, DraftCell>;
  setting: QuarterSettings;
  totals: QuarterTotals;
  canEdit: boolean;
  commentedCells: Set<string>;
  onCellChange: (brand: string | null, patch: Partial<DraftCell>) => void;
  onToggleGross: (brand: string, next: boolean, allQuarters: boolean) => void;
  onRemoveBrand: (brand: string) => void;
  onComment: (brand: string | null) => void;
  onDistributeRest: () => void;
}

export default function NetworkQuarterTable({
  quarter,
  brands,
  draft,
  setting,
  totals,
  canEdit,
  commentedCells,
  onCellChange,
  onToggleGross,
  onRemoveBrand,
  onComment,
  onDistributeRest,
}: NetworkQuarterTableProps) {
  const [menu, setMenu] = useState<{ anchor: HTMLElement; brand: string } | null>(null);

  const cellOf = (brand: string | null): DraftCell => draft[planKey(quarter, brand)] ?? EMPTY_CELL;
  const grossBrands = brands.filter((b) => cellOf(b).inGross);
  const separateBrands = brands.filter((b) => !cellOf(b).inGross);
  const pool = calcCell(draft[planKey(quarter, null)], setting);
  const hasPool = grossBrands.length > 0 || pool.plan != null || pool.forecast != null;

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

  // Строка бренда: два поля ввода объёма, процент инвестиций и расчётные суммы.
  const brandRow = (brand: string) => {
    const cell = cellOf(brand);
    const amounts = calcCell(cell, setting);
    const factPct = amounts.plan ? deltaPct(amounts.fact, amounts.plan) : null;
    const forecastPct = deltaPct(amounts.forecast, amounts.plan);
    const hasComment = commentedCells.has(planKey(quarter, brand));

    return (
      <TableRow key={brand} hover>
        <TableCell>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, minWidth: 0 }}>
            <Typography variant="body2" noWrap title={brand}>{brand}</Typography>
            <Tooltip title={hasComment ? 'Есть комментарий' : 'Комментарий к бренду за квартал'}>
              <IconButton size="small" onClick={() => onComment(brand)}>
                <CommentIcon fontSize="inherit" color={hasComment ? 'warning' : 'disabled'} />
              </IconButton>
            </Tooltip>
          </Box>
        </TableCell>
        <TableCell>
          <PlanNumberField
            value={cell.planRub}
            disabled={!canEdit}
            onChange={(v) => onCellChange(brand, { planRub: v })}
          />
        </TableCell>
        <TableCell align="right">
          <ValueCell
            value={amounts.fact}
            hint={factPct == null ? null : `${(100 + factPct).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} %`}
            tone={completionTone(factPct == null ? null : 100 + factPct)}
          />
        </TableCell>
        <TableCell>
          <PlanNumberField
            value={cell.forecastRub}
            disabled={!canEdit}
            onChange={(v) => onCellChange(brand, { forecastRub: v })}
          />
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
        </TableCell>
        <TableCell align="right">
          <ValueCell value={amounts.investPlan} netValue={amounts.investPlanNet} />
        </TableCell>
        <TableCell align="right">
          <ValueCell value={amounts.investForecast} netValue={amounts.investForecastNet} />
        </TableCell>
        <TableCell align="right">
          <ValueCell value={amounts.investFact} netValue={amounts.investFactNet} />
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
        const a = calcCell(cellOf(brand), setting);
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
            <TableCell align="right">Инв. прогноз</TableCell>
            <TableCell align="right">Инв. факт</TableCell>
            <TableCell padding="none" />
          </TableRow>
        </TableHead>
        <TableBody>
          {hasPool && (
            <>
              {sectionRow(
                'Валовый объём контракта',
                `${grossBrands.length} ${pluralRu(grossBrands.length, 'бренд', 'бренда', 'брендов')} распределяют общий объём`,
              )}
              <TableRow sx={{ bgcolor: 'action.selected' }}>
                <TableCell>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                    <Typography variant="body2" sx={{ fontWeight: 600 }}>Общий объём</Typography>
                    <Tooltip title="Комментарий к общему объёму">
                      <IconButton size="small" onClick={() => onComment(null)}>
                        <CommentIcon
                          fontSize="inherit"
                          color={commentedCells.has(planKey(quarter, null)) ? 'warning' : 'disabled'}
                        />
                      </IconButton>
                    </Tooltip>
                  </Box>
                </TableCell>
                <TableCell>
                  <PlanNumberField
                    value={draft[planKey(quarter, null)]?.planRub ?? ''}
                    disabled={!canEdit}
                    onChange={(v) => onCellChange(null, { planRub: v })}
                  />
                </TableCell>
                <TableCell align="right">
                  <ValueCell value={totals.grossPoolFactRub || null} muted={!totals.grossPoolFactRub} />
                </TableCell>
                <TableCell>
                  <PlanNumberField
                    value={draft[planKey(quarter, null)]?.forecastRub ?? ''}
                    disabled={!canEdit}
                    onChange={(v) => onCellChange(null, { forecastRub: v })}
                  />
                </TableCell>
                <TableCell align="right"><Typography variant="body2" color="text.disabled">—</Typography></TableCell>
                <TableCell align="right"><Typography variant="body2" color="text.disabled">—</Typography></TableCell>
                <TableCell align="right"><Typography variant="body2" color="text.disabled">—</Typography></TableCell>
                <TableCell align="right"><Typography variant="body2" color="text.disabled">—</Typography></TableCell>
                <TableCell />
              </TableRow>

              {totals.undistributed != null && (
                <TableRow>
                  <TableCell colSpan={COLUMNS.length} sx={{ py: 0.5 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
                      <Typography variant="caption" color="text.secondary">Остаток к распределению</Typography>
                      <Chip
                        size="small"
                        variant={totals.undistributed === 0 ? 'outlined' : 'filled'}
                        color={totals.undistributed === 0 ? 'success' : 'warning'}
                        label={formatRubShort(totals.undistributed)}
                      />
                      {canEdit && totals.undistributed !== 0 && grossBrands.length > 0 && (
                        <Button size="small" onClick={onDistributeRest}>Распределить поровну</Button>
                      )}
                    </Box>
                  </TableCell>
                </TableRow>
              )}

              {grossBrands.map(brandRow)}
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

          {sectionRow('Отдельные бренды', 'планируются вне общего объёма')}
          {separateBrands.map(brandRow)}
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
                value={totals.contractPlanRub}
                hint={totals.grossPoolRub != null ? 'пул + отдельные' : null}
                bold
              />
            </TableCell>
            <TableCell align="right">
              <ValueCell
                value={totals.factRub || null}
                hint={deltaPct(totals.factRub, totals.contractPlanRub) == null
                  ? null
                  : `${(100 + (deltaPct(totals.factRub, totals.contractPlanRub) ?? 0)).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} %`}
                tone={completionTone(
                  deltaPct(totals.factRub, totals.contractPlanRub) == null
                    ? null
                    : 100 + (deltaPct(totals.factRub, totals.contractPlanRub) ?? 0),
                )}
                bold
              />
            </TableCell>
            <TableCell align="right">
              <ValueCell
                value={totals.forecastRub || null}
                hint={deltaPct(totals.forecastRub, totals.contractPlanRub)
                  ? `${formatSignedPct(deltaPct(totals.forecastRub, totals.contractPlanRub))} к плану`
                  : null}
                tone={deviationTone(deltaPct(totals.forecastRub, totals.contractPlanRub))}
                bold
              />
            </TableCell>
            <TableCell />
            <TableCell align="right">
              <ValueCell value={totals.investmentsRub || null} netValue={totals.investmentsRubNet || null} bold />
            </TableCell>
            <TableCell align="right">
              <ValueCell
                value={totals.forecastInvestmentsRub || null}
                netValue={totals.forecastInvestmentsRubNet || null}
                bold
              />
            </TableCell>
            <TableCell align="right">
              <ValueCell
                value={totals.factInvestmentsRub || null}
                netValue={totals.factInvestmentsRubNet || null}
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
