// Детализация промо под строкой бренда во вкладке «Прогноз».
//
// Счётчик промо в прогнозе отвечал на вопрос «сколько», но не на вопрос
// «какие», и за ответом приходилось уходить на другую страницу, теряя контекст
// прогноза. Здесь список раскрывается на месте.
//
// Своего эндпоинта нет и не нужно: /api/promo/data принимает сеть, бренд, год и
// месяцы — ровно тот срез, который показывает строка бренда за квартал.

import { useQuery } from '@tanstack/react-query';
import {
  Alert, Box, Chip, CircularProgress, Table, TableBody, TableCell,
  TableHead, TableRow, Typography,
} from '@mui/material';
import { promoAPI } from '../api/promo';
import { formatRubShort } from '../utils/networkPlan';

const MONTH_LABELS = ['янв', 'фев', 'мар', 'апр', 'май', 'июн', 'июл', 'авг', 'сен', 'окт', 'ноя', 'дек'];

interface Props {
  networkName: string;
  brand: string;
  year: number;
  months: number[];
}

const amount = (value: number | null): string => value == null ? '—' : `${formatRubShort(value)} ₽`;

const units = (value: number | null): string =>
  value == null ? '—' : Math.round(value).toLocaleString('ru-RU');

const pct = (value: number | null): string =>
  value == null ? '—' : `${value.toFixed(1)}%`;

// Цвет статуса промо. Берётся собственный статус строки, а не ступени
// согласования: в списке промо они приходят текстом свободной формы, и
// разбирать его здесь значило бы гадать.
function statusColor(status: string | null): 'success' | 'warning' | 'info' | 'default' {
  switch ((status ?? '').toLowerCase()) {
    case 'проведено':
    case 'финализировано':
      return 'success';
    case 'в процессе согласования':
      return 'warning';
    case 'в процессе':
      return 'info';
    default:
      return 'default';
  }
}

export default function NetworkPromoDetail({ networkName, brand, year, months }: Props) {
  const query = useQuery({
    queryKey: ['network-promo-detail', networkName, brand, year, months],
    queryFn: () => promoAPI.getData({
      network_name: [networkName],
      brand: [brand],
      yearFrom: String(year),
      yearTo: String(year),
      months: months.map(String),
    }),
    staleTime: 60 * 1000,
  });

  if (query.isLoading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', py: 2 }}>
        <CircularProgress size={20} />
      </Box>
    );
  }
  if (query.isError) {
    return <Alert severity="error" sx={{ my: 1 }}>Не удалось загрузить промо бренда</Alert>;
  }

  const rows = query.data?.data ?? [];
  if (rows.length === 0) {
    // Пустой список здесь двусмысленный: промо может не быть, а может не быть
    // доступа. Прямая оговорка избавляет от неверного вывода «промо нет».
    return (
      <Typography variant="body2" color="text.secondary" sx={{ py: 1.5, px: 1 }}>
        Промо не найдены — либо их нет за выбранный период, либо они вне вашей области видимости.
      </Typography>
    );
  }

  return (
    <Box sx={{ py: 1 }}>
      <Table size="small" sx={{ '& td, & th': { py: 0.5 } }}>
        <TableHead>
          <TableRow>
            <TableCell sx={{ width: 56 }}>Месяц</TableCell>
            <TableCell>SKU</TableCell>
            <TableCell>Механика</TableCell>
            <TableCell align="right">План, уп</TableCell>
            <TableCell align="right">Uplift план</TableCell>
            <TableCell align="right">Uplift факт</TableCell>
            <TableCell align="right">Инвестиции</TableCell>
            <TableCell align="right">ROI план</TableCell>
            <TableCell align="right">ROI факт</TableCell>
            <TableCell>Статус</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.map((row) => (
              <TableRow key={row.id} hover>
                <TableCell>{row.month ? MONTH_LABELS[row.month - 1] : '—'}</TableCell>
                <TableCell sx={{ maxWidth: 220 }}>
                  <Typography variant="body2" noWrap title={row.sku ?? ''}>{row.sku ?? '—'}</Typography>
                </TableCell>
                <TableCell sx={{ maxWidth: 160 }}>
                  <Typography variant="body2" noWrap title={row.mechanics ?? ''}>{row.mechanics ?? '—'}</Typography>
                </TableCell>
                <TableCell align="right">{units(row.plan_promo_units)}</TableCell>
                <TableCell align="right">{amount(row.plan_promo_uplift_rub)}</TableCell>
                <TableCell align="right" sx={{ fontWeight: 600 }}>{amount(row.actual_promo_uplift_rub)}</TableCell>
                <TableCell align="right">{amount(row.plan_investments_rub)}</TableCell>
                <TableCell align="right">{pct(row.plan_roi)}</TableCell>
                <TableCell align="right">{pct(row.actual_roi)}</TableCell>
                <TableCell>
                  <Chip
                    size="small"
                    variant="outlined"
                    color={statusColor(row.status)}
                    label={row.status || '—'}
                    sx={{ height: 20, '& .MuiChip-label': { px: 0.75, fontSize: 11 } }}
                  />
                </TableCell>
              </TableRow>
          ))}
        </TableBody>
      </Table>
    </Box>
  );
}
