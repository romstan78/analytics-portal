import { Box, Paper, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import type { BudgetResponse } from '../types/budget';
import { budgetFormat } from '../utils/budget';

interface Props {
  data: BudgetResponse;
  investmentMode: 'investments' | 'pct';
  onInvestmentMode: (value: 'investments' | 'pct') => void;
  deltaMetric: 'to' | 'pct';
  onDeltaMetric: (value: 'to' | 'pct') => void;
}

export default function BudgetVisualization({ data, investmentMode, onInvestmentMode, deltaMetric, onDeltaMetric }: Props) {
  const brands = data.brands.filter(b => b.brand !== 'Нераспределённый остаток пула');
  const matrix = (metric: 'to' | 'investments' | 'pct', side: 'a' | 'b' | 'delta') => {
    const isDelta = side === 'delta';
    const percentage = metric === 'pct';
    const title = metric === 'to' ? 'ТО' : 'Инвестиции';
    const version = side === 'a' ? data.version : data.compare;
    const label = isDelta ? `Δ ${title} · ${data.version} − ${data.compare || '…'}` : `${title} · ${version || 'Бюджет для сравнения'}`;
    const unit = percentage ? isDelta ? 'п.п.' : '% от ТО' : 'млн ₽';
    return <Paper variant="outlined" sx={{ minWidth: 0, overflow: 'hidden', borderRadius: 2 }}>
      <Box sx={{ px: 1.25, py: .75, bgcolor: side === 'a' ? '#eef2ff' : '#f8fafc' }}>
        <Typography component="h3" variant="subtitle2" sx={{ fontWeight: 600 }}>{label}</Typography>
        <Typography variant="caption" color="text.secondary">{isDelta ? 'Изменение выбранного бюджета' : side === 'a' ? 'Выбранный бюджет' : 'Бюджет для сравнения'} · {unit}</Typography>
      </Box>
      {!version ? <Typography color="text.secondary" sx={{ p: 1.5, fontSize: 12 }}>Выберите бюджет в поле «Сравнить с».</Typography> :
        <TableContainer sx={{ maxHeight: 390 }}>
          <Table size="small" stickyHeader aria-label={label} sx={{ tableLayout: 'fixed', fontVariantNumeric: 'tabular-nums', '& th, & td': { px: .6, py: .45, fontSize: 11.5, lineHeight: 1.35 } }}>
            <TableHead><TableRow>
              <TableCell sx={{ width: '31%' }}>Бренд</TableCell>
              {['Q1', 'Q2', 'Q3', 'Q4', 'Год'].map(q => <TableCell key={q} align="right" sx={{ fontWeight: 600 }}>{q}</TableCell>)}
            </TableRow></TableHead>
            <TableBody>
              {[...brands, data.total].map((brand, index) => {
                const total = index === brands.length;
                return <TableRow key={total ? 'total' : brand.brand} sx={{ bgcolor: total ? '#f8fafc' : undefined }}>
                  <TableCell component="th" scope="row" title={brand.brand} sx={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontWeight: total ? 700 : 400 }}>{total ? 'Итого' : brand.brand}</TableCell>
                  {[...brand[metric].q, brand[metric].year].map((cell, q) => {
                    const value = cell[side];
                    const good = value != null && (percentage ? value < 0 : value > 0);
                    const formatted = budgetFormat(value, percentage).replace(' %', '');
                    return <TableCell key={q} align="right" sx={{ whiteSpace: 'nowrap', fontWeight: total || q === 4 ? 700 : 400, bgcolor: q === 4 ? '#f8fafc' : undefined, color: isDelta && value ? good ? 'success.main' : 'error.main' : 'text.primary' }}>{isDelta && value != null && value > 0 ? '+' : ''}{formatted}</TableCell>;
                  })}
                </TableRow>;
              })}
            </TableBody>
          </Table>
        </TableContainer>}
    </Paper>;
  };

  return <Box sx={{ overflowX: 'auto' }}><Box sx={{ display: 'grid', gridTemplateColumns: { xs: 'minmax(0, 1fr)', md: 'repeat(3, minmax(380px, 1fr))' }, gap: 1.5, alignItems: 'start' }}>
    <Box sx={{ display: 'grid', gap: 1.5, minWidth: 0 }}>
      <Box sx={{ height: 34, display: 'flex', alignItems: 'center' }}><Typography variant="subtitle2">Товарооборот · млн ₽</Typography></Box>
      {matrix('to', 'b')}
      {matrix('to', 'a')}
    </Box>
    <Box sx={{ display: 'grid', gap: 1.5, minWidth: 0 }}>
      <ToggleButtonGroup exclusive size="small" value={investmentMode} onChange={(_, value) => { if (value) onInvestmentMode(value); }} aria-label="Отображение инвестиций" sx={{ height: 34 }}>
        <ToggleButton value="investments">Инвестиции · млн ₽</ToggleButton>
        <ToggleButton value="pct">% от ТО</ToggleButton>
      </ToggleButtonGroup>
      {matrix(investmentMode, 'b')}
      {matrix(investmentMode, 'a')}
    </Box>
    <Box sx={{ display: 'grid', gap: 1.5, minWidth: 0 }}>
      <ToggleButtonGroup exclusive size="small" value={deltaMetric} onChange={(_, value) => { if (value) onDeltaMetric(value); }} aria-label="Показатель изменения" sx={{ height: 34 }}>
        <ToggleButton value="to">Δ ТО · млн ₽</ToggleButton>
        <ToggleButton value="pct">Δ инвестиций · п.п.</ToggleButton>
      </ToggleButtonGroup>
      {matrix(deltaMetric, 'delta')}
    </Box>
  </Box></Box>;
}
