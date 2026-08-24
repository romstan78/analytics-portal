// Годовой кумулятив инвестиций. Все суммы и признаки выполнения приходят с
// backend preview/plan API; компонент только показывает расчёт и его условия.

import {
  Box,
  Chip,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from '@mui/material';
import { AccountBalanceWalletOutlined as WalletIcon } from '@mui/icons-material';
import type { NetworkAnnualInvestmentCumulative as Cumulative } from '../types/network';
import { formatPct } from '../utils/networkPlan';
import { ValueCell } from './networkPlanCells';

interface NetworkAnnualInvestmentCumulativeProps {
  year: number;
  data: Cumulative;
}

const scopeLabel = (scopeType: string, brand: string | null): string =>
  scopeType === 'gross' ? 'Валовый объём' : (brand ?? 'Без бренда');

export default function NetworkAnnualInvestmentCumulative({
  year,
  data,
}: NetworkAnnualInvestmentCumulativeProps) {
  const portfolioStatus = data.portfolio_completed ? 'Портфель выполнен' : 'Портфель не выполнен';

  return (
    <Paper variant="outlined" sx={{ overflow: 'hidden' }}>
      <Box sx={{ p: 1.5, display: 'flex', gap: 1.5, alignItems: 'center', flexWrap: 'wrap' }}>
        <WalletIcon fontSize="small" color="action" />
        <Box sx={{ minWidth: 240 }}>
          <Typography variant="subtitle2">Годовой кумулятив инвестиций</Typography>
          <Typography variant="caption" color="text.secondary">
            {year}: годовое начисление минус выплаты Q1–Q3 и прогноз выплат Q4
          </Typography>
        </Box>
        <Chip
          size="small"
          color={data.portfolio_completed ? 'success' : 'warning'}
          variant={data.portfolio_completed ? 'filled' : 'outlined'}
          label={`${portfolioStatus} · ${formatPct(data.portfolio_completion_pct)} %`}
        />
        <Box sx={{ display: 'flex', gap: 2, ml: { md: 'auto' }, alignItems: 'center' }}>
          <Box>
            <Typography variant="caption" color="text.secondary">План портфеля</Typography>
            <ValueCell value={data.portfolio_plan_rub || null} bold />
          </Box>
          <Box>
            <Typography variant="caption" color="text.secondary">EAC портфеля</Typography>
            <ValueCell value={data.portfolio_eac_rub || null} bold />
          </Box>
          <Box>
            <Typography variant="caption" color="text.secondary">Итого к доплате</Typography>
            <ValueCell
              value={data.total_supplement_rub}
              netValue={data.total_supplement_rub_net}
              bold
            />
          </Box>
        </Box>
      </Box>

      {data.rows.length === 0 ? (
        <Typography variant="body2" color="text.secondary" sx={{ px: 1.5, pb: 1.5 }}>
          Нет валового объёма или отдельных брендов для расчёта.
        </Typography>
      ) : (
        <Box sx={{ overflowX: 'auto', borderTop: 1, borderColor: 'divider' }}>
          <Table size="small" sx={{ minWidth: 1040 }}>
            <TableHead>
              <TableRow>
                <TableCell>Область доплаты</TableCell>
                <TableCell align="right">План года</TableCell>
                <TableCell align="right">EAC года</TableCell>
                <TableCell align="right">Выполнение</TableCell>
                <TableCell align="right">
                  <Tooltip title="Сумма EAC каждого квартала × процент инвестиций этого квартала">
                    <span>Начислено за год</span>
                  </Tooltip>
                </TableCell>
                <TableCell align="right">Выплачено Q1–Q3</TableCell>
                <TableCell align="right">Прогноз Q4</TableCell>
                <TableCell align="right">Доплата</TableCell>
                <TableCell>Статус</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {data.rows.map((row) => {
                const status = row.eligible
                  ? 'К доплате'
                  : data.portfolio_completed
                    ? 'План строки не выполнен'
                    : 'Портфель не выполнен';
                return (
                  <TableRow key={`${row.scope_type}|${row.brand_as ?? '*'}`} hover>
                    <TableCell sx={{ fontWeight: 600 }}>
                      {scopeLabel(row.scope_type, row.brand_as)}
                      {row.scope_type === 'gross' && (
                        <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }}>
                          бренды, входящие в валовый объём
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell align="right"><ValueCell value={row.plan_rub || null} /></TableCell>
                    <TableCell align="right"><ValueCell value={row.eac_rub || null} /></TableCell>
                    <TableCell align="right">
                      <Chip
                        size="small"
                        color={row.completed ? 'success' : 'default'}
                        variant="outlined"
                        label={`${formatPct(row.completion_pct)} %`}
                      />
                    </TableCell>
                    <TableCell align="right">
                      <ValueCell
                        value={row.accrued_investments_rub}
                        netValue={row.accrued_investments_rub_net}
                      />
                    </TableCell>
                    <TableCell align="right">
                      <ValueCell
                        value={row.paid_investments_rub}
                        netValue={row.paid_investments_rub_net}
                      />
                    </TableCell>
                    <TableCell align="right">
                      <ValueCell
                        value={row.q4_forecast_investments_rub}
                        netValue={row.q4_forecast_investments_rub_net}
                      />
                    </TableCell>
                    <TableCell align="right">
                      <ValueCell
                        value={row.eligible ? row.supplement_rub : null}
                        netValue={row.eligible ? row.supplement_rub_net : null}
                        bold
                      />
                    </TableCell>
                    <TableCell>
                      <Chip
                        size="small"
                        color={row.eligible ? 'success' : 'default'}
                        variant={row.eligible ? 'filled' : 'outlined'}
                        label={status}
                      />
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </Box>
      )}
    </Paper>
  );
}
