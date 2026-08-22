import { useMemo, useState } from 'react';
import { Alert, Box, Button, Chip, CircularProgress, Paper, Stack, TextField, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import { FileDownload as ExportIcon, Search as SearchIcon } from '@mui/icons-material';
import { DataGrid, type GridColDef } from '@mui/x-data-grid';
import { saveAs } from 'file-saver';
import type { InternetSalesDashboardData } from './InternetSalesDashboard';

interface InternetSalesSummaryTableProps {
  data: InternetSalesDashboardData | null;
  loading: boolean;
  error: string;
}

interface SummaryRow {
  id: string;
  rank: number;
  name: string;
  current: number;
  previous: number;
  delta: number;
  yoyPercent: number | null;
  share: number;
  rankChange: number;
}

const number = (value: number, digits = 0) => Number(value || 0).toLocaleString('ru-RU', { maximumFractionDigits: digits });
const signedPercent = (value: number | null) => value == null ? '—' : `${value >= 0 ? '+' : ''}${value.toFixed(1)}%`;

export default function InternetSalesSummaryTable({ data, loading, error }: InternetSalesSummaryTableProps) {
  const [dimension, setDimension] = useState<'network' | 'product'>('network');
  const [search, setSearch] = useState('');

  const unit = data?.unit === 'руб' ? '₽' : data?.unit === 'евро' ? '€' : 'шт.';
  const rows = useMemo<SummaryRow[]>(() => {
    const source = dimension === 'network' ? data?.networkRanking : data?.productRanking;
    const query = search.trim().toLocaleLowerCase('ru-RU');
    return (source || [])
      .filter(row => !query || row.name.toLocaleLowerCase('ru-RU').includes(query))
      .map(row => ({
        id: row.name,
        rank: row.rank,
        name: row.name,
        current: row.value,
        previous: row.previous,
        delta: row.value - row.previous,
        yoyPercent: row.yoyPercent,
        share: row.share,
        rankChange: row.rankChange,
      }));
  }, [data, dimension, search]);

  const valueCell = (value: number, signed = false) => (
    <Typography variant="body2" sx={{ width: '100%', textAlign: 'right', fontVariantNumeric: 'tabular-nums', fontWeight: signed ? 650 : 500,
      color: signed ? (value >= 0 ? '#12805c' : '#c14545') : 'inherit' }}>
      {signed && value > 0 ? '+' : ''}{number(value, data?.unit === 'евро' ? 2 : 0)}
    </Typography>
  );

  const columns = useMemo<GridColDef<SummaryRow>[]>(() => [
    { field: 'rank', headerName: 'Место', width: 80, type: 'number', align: 'center', headerAlign: 'center' },
    { field: 'name', headerName: dimension === 'network' ? 'Сеть' : 'SKU', minWidth: 260, flex: 1 },
    { field: 'current', headerName: `${data?.analysisYear || 'Текущий'} · ${unit}`, width: 165, type: 'number', align: 'right', headerAlign: 'right', renderCell: params => valueCell(params.value) },
    { field: 'previous', headerName: `${data ? data.analysisYear - 1 : 'Прошлый'} · ${unit}`, width: 165, type: 'number', align: 'right', headerAlign: 'right', renderCell: params => valueCell(params.value) },
    { field: 'delta', headerName: `Отклонение · ${unit}`, width: 170, type: 'number', align: 'right', headerAlign: 'right', renderCell: params => valueCell(params.value, true) },
    { field: 'yoyPercent', headerName: 'YoY', width: 110, type: 'number', align: 'right', headerAlign: 'right',
      renderCell: params => <Typography variant="body2" sx={{ width: '100%', textAlign: 'right', fontWeight: 700, color: params.value == null ? 'text.secondary' : params.value >= 0 ? '#12805c' : '#c14545' }}>{signedPercent(params.value)}</Typography> },
    { field: 'share', headerName: 'Доля', width: 100, type: 'number', align: 'right', headerAlign: 'right', valueFormatter: (value: number) => `${value.toFixed(1)}%` },
    { field: 'rankChange', headerName: 'Δ места', width: 105, type: 'number', align: 'center', headerAlign: 'center',
      renderCell: params => params.value === 0 ? <Typography color="text.secondary">—</Typography> : <Chip size="small" color={params.value > 0 ? 'success' : 'error'} variant="outlined" label={`${params.value > 0 ? '+' : ''}${params.value}`} /> },
  ], [data, dimension, unit]); // eslint-disable-line react-hooks/exhaustive-deps

  const exportCSV = () => {
    if (!rows.length) return;
    const headers = ['Место', dimension === 'network' ? 'Сеть' : 'SKU', String(data?.analysisYear || ''), String((data?.analysisYear || 1) - 1), 'Отклонение', 'YoY, %', 'Доля, %', 'Изменение места'];
    const escape = (value: unknown) => `"${String(value ?? '').replace(/"/g, '""')}"`;
    const lines = rows.map(row => [row.rank, row.name, row.current, row.previous, row.delta, row.yoyPercent ?? '', row.share, row.rankChange].map(escape).join(';'));
    saveAs(new Blob([`\uFEFF${headers.map(escape).join(';')}\n${lines.join('\n')}`], { type: 'text/csv;charset=utf-8;' }), `internet-sales-summary-${dimension}-${new Date().toISOString().slice(0, 10)}.csv`);
  };

  if (error) return <Alert severity="error">{error}</Alert>;
  if (loading && !data) return <Box sx={{ flex: 1, display: 'grid', placeItems: 'center' }}><CircularProgress /></Box>;
  if (!data) return <Alert severity="info">Нет данных для сводной таблицы.</Alert>;

  return (
    <Paper variant="outlined" sx={{ flex: 1, minHeight: 0, p: 1.5, borderRadius: 3, display: 'flex', flexDirection: 'column' }}>
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.25} sx={{ alignItems: { xs: 'stretch', sm: 'center' }, mb: 1.25 }}>
        <Box>
          <Typography variant="subtitle1" sx={{ fontWeight: 750 }}>Рейтинг и динамика</Typography>
          <Typography variant="caption" color="text.secondary">Топ-10 · {data.analysisYear} к {data.analysisYear - 1} · {data.segment}</Typography>
        </Box>
        <ToggleButtonGroup size="small" exclusive value={dimension} onChange={(_, value) => value && setDimension(value)}>
          <ToggleButton value="network">Сети</ToggleButton>
          <ToggleButton value="product">SKU</ToggleButton>
        </ToggleButtonGroup>
        <TextField size="small" placeholder={dimension === 'network' ? 'Найти сеть...' : 'Найти SKU...'} value={search} onChange={event => setSearch(event.target.value)}
          slotProps={{ input: { startAdornment: <SearchIcon sx={{ fontSize: 18, color: '#94a3b8', mr: 0.5 }} /> } }} sx={{ width: { xs: '100%', sm: 260 } }} />
        <Box sx={{ flex: 1 }} />
        {loading && <CircularProgress size={18} />}
        <Button size="small" startIcon={<ExportIcon />} onClick={exportCSV} disabled={!rows.length}>CSV</Button>
      </Stack>

      <DataGrid rows={rows} columns={columns} disableRowSelectionOnClick hideFooter
        initialState={{ sorting: { sortModel: [{ field: 'current', sort: 'desc' }] } }}
        sx={{ flex: 1, minHeight: 360, border: '1px solid #e2e8f0', boxShadow: 'none' }} />
    </Paper>
  );
}
