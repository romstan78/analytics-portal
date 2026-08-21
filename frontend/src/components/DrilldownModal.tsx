import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Dialog, DialogTitle, DialogContent, IconButton, Typography,
  Box, Tabs, Tab, CircularProgress, Alert
} from '@mui/material';
import { Close as CloseIcon } from '@mui/icons-material';
import { DataGrid, type GridColDef } from '@mui/x-data-grid';
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend,
  ResponsiveContainer
} from 'recharts';
import { salesAPI } from '../api/promo';
import type { DrilldownRow } from '../types/sales';

const COLORS = [
  '#8884d8', '#82ca9d', '#ffc658', '#ff7300', '#a4de6c',
  '#d0ed57', '#83a6ed', '#8dd1e1', '#82ca9d', '#a4de6c',
  '#d0ed57', '#ffc658', '#ff7300', '#8884d8', '#83a6ed',
];

interface DrilldownRowData {
  brandName?: string;
  networkName?: string;
}

interface AppliedFilters {
  yearFrom?: string;
  yearTo?: string;
  months?: number[];
  segment?: string[];
  channel?: string[];
}

interface DrilldownModalProps {
  open: boolean;
  onClose: () => void;
  rowData: DrilldownRowData | null;
  appliedFilters?: AppliedFilters;
}

export default function DrilldownModal({ open, onClose, rowData, appliedFilters = {} }: DrilldownModalProps) {
  const [tab, setTab] = useState(0);
  const [viewType, setViewType] = useState('total');

  const brandName = rowData?.brandName;
  const networkName = rowData?.networkName;

  const { data: response, isFetching: loading, error: queryError } = useQuery({
    queryKey: ['drilldown', brandName, networkName, appliedFilters] as const,
    enabled: Boolean(open && brandName && networkName),
    queryFn: () => salesAPI.getDrilldown({
      brandName,
      networkName,
      yearFrom: appliedFilters.yearFrom,
      yearTo: appliedFilters.yearTo,
      months: appliedFilters.months,
      segment: appliedFilters.segment,
      channel: appliedFilters.channel,
    }),
  });

  const data = useMemo(() => response?.data ?? [], [response]);
  const error = queryError ? (queryError instanceof Error ? queryError.message : String(queryError)) : null;

  const chartData = prepareChartData(data);

  const columns: GridColDef[] = [
    { field: 'year', headerName: 'Год', width: 80, type: 'number', valueFormatter: (value: number) => value },
    { field: 'month', headerName: 'Месяц', width: 80, type: 'number' },
    { field: 'metricType', headerName: 'Показатель', width: 130 },
    { field: 'totalValue', headerName: 'Значение', width: 140, type: 'number',
      valueFormatter: (value: number | null) => value != null ? Number(value).toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : '' },
    { field: 'un_rub', headerName: 'Уп/Руб', width: 90 },
    { field: 'segment', headerName: 'Сегмент', width: 140 },
    { field: 'channel', headerName: 'Канал', width: 140 },
  ];

  if (!rowData) return null;

  return (
    <Dialog open={open} onClose={onClose} maxWidth="lg" fullWidth slotProps={{ paper: { sx: { height: '80vh' } } }}>
      <DialogTitle sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Box>
          <Typography variant="h6">Детализация: {rowData.brandName}</Typography>
          <Typography variant="body2" color="text.secondary">
            Сеть: {rowData.networkName}
            {appliedFilters.yearFrom && ` • Годы: ${appliedFilters.yearFrom}${appliedFilters.yearTo ? `–${appliedFilters.yearTo}` : '+'}`}
          </Typography>
        </Box>
        <IconButton onClick={onClose} size="small"><CloseIcon /></IconButton>
      </DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <Box sx={{ borderBottom: 1, borderColor: 'divider', mb: 2 }}>
          <Tabs value={tab} onChange={(_, v) => setTab(v)}>
            <Tab label="График" />
            <Tab label="Таблица" />
          </Tabs>
          {tab === 0 && chartData.length > 0 && (
            <Tabs value={viewType} onChange={(_, v) => setViewType(v)}
              sx={{ minHeight: 40, '& .MuiTab-root': { minHeight: 40, py: 0.5 } }}>
              <Tab label="Уп/Руб" value="total" />
              <Tab label="По сегментам" value="segments" />
              <Tab label="По каналам" value="channels" />
            </Tabs>
          )}
        </Box>

        {loading && <Box sx={{ display: 'flex', justifyContent: 'center', p: 4 }}><CircularProgress /></Box>}
        {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

        {!loading && !error && (
          <>
            {tab === 0 && (
              <Box sx={{ flex: 1, minHeight: 400, height: '100%' }}>
                {chartData.length > 0 ? (
                  <ResponsiveContainer width="100%" height={400}>
                    {viewType === 'total' ? (
                      <BarChart data={chartData} margin={{ top: 20, right: 30, left: 20, bottom: 5 }}>
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="period" tick={{ fontSize: 12 }} angle={-45} textAnchor="end" height={60} />
                        <YAxis />
                        <Tooltip formatter={(v) => Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2 })} />
                        <Legend />
                        <Bar dataKey="упаковки" fill="#8884d8" radius={[4, 4, 0, 0]} />
                        <Bar dataKey="рубли" fill="#82ca9d" radius={[4, 4, 0, 0]} />
                      </BarChart>
                    ) : viewType === 'segments' ? (
                      <BarChart data={chartData} margin={{ top: 20, right: 30, left: 20, bottom: 5 }}>
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="period" tick={{ fontSize: 12 }} angle={-45} textAnchor="end" height={60} />
                        <YAxis />
                        <Tooltip formatter={(v) => Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2 })} />
                        <Legend />
                        {getUniqueKeys(chartData, 'segments').map((segKey, idx) => {
                          const originalName = segKey.replace(/_/g, '.');
                          return <Bar key={segKey} dataKey={`segments.${segKey}`} name={originalName} fill={COLORS[idx % COLORS.length]} radius={[4, 4, 0, 0]} />;
                        })}
                      </BarChart>
                    ) : (
                      <BarChart data={chartData} margin={{ top: 20, right: 30, left: 20, bottom: 5 }}>
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="period" tick={{ fontSize: 12 }} angle={-45} textAnchor="end" height={60} />
                        <YAxis />
                        <Tooltip formatter={(v) => Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2 })} />
                        <Legend />
                        {getUniqueKeys(chartData, 'channels').map((chKey, idx) => {
                          const originalName = chKey.replace(/_/g, '.');
                          return <Bar key={chKey} dataKey={`channels.${chKey}`} name={originalName} fill={COLORS[idx % COLORS.length]} radius={[4, 4, 0, 0]} />;
                        })}
                      </BarChart>
                    )}
                  </ResponsiveContainer>
                ) : (
                  <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
                    <Typography color="text.secondary">Нет данных для отображения графика</Typography>
                  </Box>
                )}
              </Box>
            )}

            {tab === 1 && (
              <Box sx={{ flex: 1 }}>
                <DataGrid
                  rows={data.map((row, idx) => ({ ...row, id: idx }))} columns={columns}
                  initialState={{ pagination: { paginationModel: { pageSize: 50 } }, sorting: { sortModel: [{ field: 'year', sort: 'desc' }, { field: 'month', sort: 'asc' }] } }}
                  pageSizeOptions={[25, 50, 100]} disableRowSelectionOnClick sx={{ height: '100%' }} />
              </Box>
            )}
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

interface ChartPoint {
  period: string;
  упаковки: number;
  рубли: number;
  segments: Record<string, number>;
  channels: Record<string, number>;
}

function prepareChartData(data: DrilldownRow[]): ChartPoint[] {
  const grouped: Record<string, ChartPoint> = {};
  data.forEach((row) => {
    const key = `${row.year}-${String(row.month).padStart(2, '0')}`;
    if (!grouped[key]) grouped[key] = { period: key, упаковки: 0, рубли: 0, segments: {}, channels: {} };
    if (row.un_rub === 'уп') grouped[key].упаковки += row.totalValue;
    else if (row.un_rub === 'руб') grouped[key].рубли += row.totalValue;
    const segmentKey = (row.segment || 'Без сегмента').replace(/\./g, '_');
    if (!grouped[key].segments[segmentKey]) grouped[key].segments[segmentKey] = 0;
    grouped[key].segments[segmentKey] += row.totalValue;
    const channelKey = (row.channel || 'Без канала').replace(/\./g, '_');
    if (!grouped[key].channels[channelKey]) grouped[key].channels[channelKey] = 0;
    grouped[key].channels[channelKey] += row.totalValue;
  });
  return Object.values(grouped).sort((a, b) => a.period.localeCompare(b.period));
}

function getUniqueKeys(data: ChartPoint[], type: 'segments' | 'channels'): string[] {
  const keys = new Set<string>();
  data.forEach(item => { Object.keys(item[type] || {}).forEach(key => keys.add(key)); });
  return Array.from(keys).sort();
}