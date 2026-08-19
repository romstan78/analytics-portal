import { Alert, Box, Chip, CircularProgress, Grid, Paper, Stack, Typography } from '@mui/material';
import {
  Bar, BarChart, CartesianGrid, Cell, Legend, Line, LineChart, ResponsiveContainer,
  Tooltip, XAxis, YAxis,
} from 'recharts';

interface DashboardPoint {
  year: number;
  month: number;
  value: number;
}

interface DashboardRank {
  name: string;
  value: number;
}

interface DashboardSeriesPoint extends DashboardPoint {
  name: string;
}

interface DashboardFocusPoint extends DashboardSeriesPoint {
  type: 'product' | 'network';
}

interface DashboardNetworkBreakdown {
  network: string;
  channel: string;
  segment: string;
  value: number;
}

interface DashboardSummary {
  total: number;
  averagePerMonth: number;
  activeNetworks: number;
  activeProducts: number;
  periods: number;
  latestYear: number;
  latestMonth: number;
  latestValue: number | null;
  previousValue: number | null;
  yearAgoValue: number | null;
}

export interface InternetSalesDashboardData {
  channel: string;
  channelSegments: string[];
  segment: string;
  unit: 'руб' | 'уп';
  summary: DashboardSummary;
  trend: DashboardPoint[];
  focusTrends: DashboardFocusPoint[];
  topNetworks: DashboardRank[];
  topProducts: DashboardRank[];
  segmentTotals: DashboardRank[];
  networkTrends: DashboardSeriesPoint[];
  channelTrends: DashboardSeriesPoint[];
  networkBreakdown: DashboardNetworkBreakdown[];
}

export interface DashboardFocus {
  type: 'product' | 'network';
  name: string;
}

interface InternetSalesDashboardProps {
  data: InternetSalesDashboardData | null;
  loading: boolean;
  error: string;
  focuses: DashboardFocus[];
  onProductSelect: (name: string, additive: boolean) => void;
  onNetworkSelect: (name: string, additive: boolean) => void;
  onSegmentSelect: (name: string) => void;
  onRemoveFocus: (focus: DashboardFocus) => void;
  onClearFocus: () => void;
}

const compactNumber = (value: number) => new Intl.NumberFormat('ru-RU', {
  notation: 'compact',
  maximumFractionDigits: 1,
}).format(Number(value) || 0);

const fullNumber = (value: number, unit: string) => `${Number(value || 0).toLocaleString('ru-RU', {
  maximumFractionDigits: unit === 'руб' ? 0 : 0,
})} ${unit === 'руб' ? '₽' : 'уп.'}`;

const changeLabel = (current: number | null, previous: number | null, suffix: string) => {
  if (current == null || previous == null || previous === 0) return `нет данных ${suffix}`;
  const change = ((current - previous) / previous) * 100;
  return `${change >= 0 ? '+' : ''}${change.toFixed(1)}% ${suffix}`;
};

function KpiCard({ label, value, hint, accent }: { label: string; value: string; hint: string; accent: string }) {
  return (
    <Paper variant="outlined" sx={{ p: 2, height: '100%', borderRadius: 3, borderColor: '#e2e8f0' }}>
      <Box sx={{ width: 36, height: 4, bgcolor: accent, borderRadius: 4, mb: 1.25 }} />
      <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>{label}</Typography>
      <Typography sx={{ fontSize: '1.55rem', fontWeight: 750, lineHeight: 1.25, mt: 0.25 }}>{value}</Typography>
      <Typography variant="caption" color="text.secondary">{hint}</Typography>
    </Paper>
  );
}

function RankList({ title, rows, unit, color, selectedNames, onSelect }: {
  title: string;
  rows: DashboardRank[];
  unit: string;
  color: string;
  selectedNames: Set<string>;
  onSelect?: (name: string, additive: boolean) => void;
}) {
  const max = Math.max(...rows.map(row => row.value), 1);
  return (
    <Paper variant="outlined" sx={{ p: 2, borderRadius: 3, borderColor: '#e2e8f0', height: '100%' }}>
      <Typography variant="subtitle1" sx={{ fontWeight: 700, mb: 1.5 }}>{title}</Typography>
      <Stack spacing={1.15}>
        {rows.map((row, index) => (
          <Box key={row.name} component="button" type="button" onClick={(event) => onSelect?.(row.name, event.ctrlKey || event.metaKey)}
            sx={{ display: 'block', width: '100%', p: 0.6, mx: -0.6, border: selectedNames.has(row.name) ? '1px solid #818cf8' : '1px solid transparent', borderRadius: 1.5, bgcolor: selectedNames.has(row.name) ? '#eef2ff' : 'transparent', textAlign: 'left', cursor: onSelect ? 'pointer' : 'default', '&:hover': { bgcolor: onSelect ? '#f8fafc' : undefined } }}>
            <Stack direction="row" justifyContent="space-between" spacing={1}>
              <Typography variant="body2" noWrap title={row.name} sx={{ maxWidth: '68%' }}>
                <Box component="span" sx={{ color: 'text.secondary', mr: 0.75 }}>{index + 1}.</Box>{row.name}
              </Typography>
              <Typography variant="body2" sx={{ fontWeight: 700, whiteSpace: 'nowrap' }}>{compactNumber(row.value)} {unit === 'руб' ? '₽' : 'уп.'}</Typography>
            </Stack>
            <Box sx={{ height: 5, bgcolor: '#f1f5f9', borderRadius: 4, mt: 0.45, overflow: 'hidden' }}>
              <Box sx={{ height: '100%', width: `${Math.max((row.value / max) * 100, 2)}%`, bgcolor: color, borderRadius: 4 }} />
            </Box>
          </Box>
        ))}
        {rows.length === 0 && <Typography color="text.secondary">Нет данных</Typography>}
      </Stack>
    </Paper>
  );
}

function NetworkHeatmap({ rows, networkOrder, unit, selectedNames, onSelect }: {
  rows: DashboardSeriesPoint[];
  networkOrder: string[];
  unit: string;
  selectedNames: Set<string>;
  onSelect: (name: string, additive: boolean) => void;
}) {
  const periods = [...new Set(rows.map(point => `${point.year}-${String(point.month).padStart(2, '0')}`))].sort();
  const valueMap = new Map(rows.map(point => [`${point.name}\u0000${point.year}-${String(point.month).padStart(2, '0')}`, point.value]));
  const rowMax = new Map(networkOrder.map(name => [
    name,
    Math.max(...periods.map(period => valueMap.get(`${name}\u0000${period}`) || 0), 1),
  ]));
  const gridTemplate = `180px repeat(${periods.length}, 32px)`;

  return (
    <Paper variant="outlined" sx={{ p: 2, mt: 1.5, borderRadius: 3, borderColor: '#e2e8f0' }}>
      <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>Тепловая карта: сети × месяцы</Typography>
      <Typography variant="caption" color="text.secondary">
        Цвет нормализован внутри каждой сети и показывает её сезонность. Нажмите на строку или ячейку, чтобы вывести сеть на основном графике.
      </Typography>
      <Box sx={{ overflowX: 'auto', mt: 1.5, pb: 0.5 }}>
        <Box sx={{ minWidth: Math.max(720, 180 + periods.length * 32) }}>
          <Box sx={{ display: 'grid', gridTemplateColumns: gridTemplate, gap: 0.35, mb: 0.5 }}>
            <Box sx={{ position: 'sticky', left: 0, zIndex: 2, bgcolor: '#fff' }} />
            {periods.map(period => (
              <Typography key={period} variant="caption" color="text.secondary" sx={{ fontSize: 9, textAlign: 'center', writingMode: 'vertical-rl', transform: 'rotate(180deg)', height: 46 }}>
                {period.slice(5)}.{period.slice(2, 4)}
              </Typography>
            ))}
          </Box>
          {networkOrder.map(name => (
            <Box key={name} sx={{ display: 'grid', gridTemplateColumns: gridTemplate, gap: 0.35, mb: 0.35 }}>
              <Box component="button" type="button" onClick={(event) => onSelect(name, event.ctrlKey || event.metaKey)} title={name}
                sx={{ position: 'sticky', left: 0, zIndex: 2, border: 0, borderRadius: 1, bgcolor: selectedNames.has(name) ? '#eef2ff' : '#fff', px: 0.75, textAlign: 'left', cursor: 'pointer', overflow: 'hidden' }}>
                <Typography variant="caption" noWrap sx={{ display: 'block', fontWeight: selectedNames.has(name) ? 700 : 500 }}>{name}</Typography>
              </Box>
              {periods.map(period => {
                const value = valueMap.get(`${name}\u0000${period}`) || 0;
                const intensity = value / (rowMax.get(name) || 1);
                return (
                  <Box key={period} component="button" type="button" onClick={(event) => onSelect(name, event.ctrlKey || event.metaKey)}
                    title={`${name} · ${period}: ${fullNumber(value, unit)}`}
                    sx={{ height: 24, border: selectedNames.has(name) ? '1px solid #6366f1' : '1px solid transparent', borderRadius: 0.75, bgcolor: value === 0 ? '#f8fafc' : `rgba(99, 102, 241, ${0.12 + intensity * 0.82})`, cursor: 'pointer' }} />
                );
              })}
            </Box>
          ))}
        </Box>
      </Box>
    </Paper>
  );
}

export default function InternetSalesDashboard({
  data, loading, error, focuses,
  onProductSelect, onNetworkSelect, onSegmentSelect, onRemoveFocus, onClearFocus,
}: InternetSalesDashboardProps) {
  if (loading && !data) {
    return <Box sx={{ flex: 1, display: 'grid', placeItems: 'center' }}><CircularProgress /></Box>;
  }
  if (error) return <Alert severity="error">{error}</Alert>;
  if (!data || data.trend.length === 0) {
    return <Paper variant="outlined" sx={{ p: 5, textAlign: 'center', borderRadius: 3 }}><Typography color="text.secondary">По выбранным параметрам данных нет.</Typography></Paper>;
  }

  const trend = data.trend.map(point => ({
    ...point,
    period: `${point.year}-${String(point.month).padStart(2, '0')}`,
  }));
  const focusSeries = focuses.map((focus, index) => ({ ...focus, key: `focus_${index}` }));
  const channelNames = [...new Set(data.channelTrends.map(point => point.name))];
  const channelSeries = channelNames.map((name, index) => ({ name, key: `channel_${index}` }));
  const chartByPeriod = new Map<string, Record<string, number | string>>(
    trend.map(point => [point.period, { period: point.period, value: point.value }]),
  );
  data.focusTrends.forEach(point => {
    const series = focusSeries.find(item => item.type === point.type && item.name === point.name);
    if (!series) return;
    const period = `${point.year}-${String(point.month).padStart(2, '0')}`;
    const current = chartByPeriod.get(period) || { period };
    current[series.key] = point.value;
    chartByPeriod.set(period, current);
  });
  data.channelTrends.forEach(point => {
    const series = channelSeries.find(item => item.name === point.name);
    if (!series) return;
    const period = `${point.year}-${String(point.month).padStart(2, '0')}`;
    const current = chartByPeriod.get(period) || { period };
    current[series.key] = point.value;
    chartByPeriod.set(period, current);
  });
  const chartTrend = [...chartByPeriod.values()].sort((a, b) => String(a.period).localeCompare(String(b.period)));

  const networkTrendByPeriod = new Map<string, Record<string, number | string>>();
  data.networkTrends.forEach(point => {
    const period = `${point.year}-${String(point.month).padStart(2, '0')}`;
    const current = networkTrendByPeriod.get(period) || { period };
    current[point.name] = point.value;
    networkTrendByPeriod.set(period, current);
  });
  const networkTrend = [...networkTrendByPeriod.values()].sort((a, b) => String(a.period).localeCompare(String(b.period)));
  const networkNames = [...new Set(data.networkTrends.map(point => point.name))];
  const networkLineNames = networkNames.slice(0, 5);
  const latest = trend.at(-1)!;
  const monthChange = data.summary.previousValue
    ? ((Number(data.summary.latestValue) - data.summary.previousValue) / data.summary.previousValue) * 100
    : null;
  const unitLabel = data.unit === 'руб' ? '₽' : 'уп.';
  const networkFocusNames = new Set(focuses.filter(item => item.type === 'network').map(item => item.name));
  const productFocusNames = new Set(focuses.filter(item => item.type === 'product').map(item => item.name));
  const showNetworkBreakdown = data.networkBreakdown.length > 1 && (networkFocusNames.size > 0 || data.topNetworks.length <= 1);
  const showSegmentComparison = !showNetworkBreakdown && data.channelSegments.length > 1 && data.segmentTotals.length > 1;
  const showNetworkDynamics = !showNetworkBreakdown && !showSegmentComparison && data.topProducts.length <= 1 && networkNames.length > 1;
  const focusColors = ['#6366f1', '#ec4899', '#f59e0b', '#10b981', '#8b5cf6'];
  const channelColors = ['#0891b2', '#0f766e', '#2563eb', '#9333ea', '#c2410c'];
  const maxOverall = Math.max(...data.trend.map(point => point.value), 0);
  const maxFocus = Math.max(...data.focusTrends.map(point => point.value), 0);
  const useFocusAxis = focusSeries.length > 0 && maxFocus > 0 && maxOverall / maxFocus >= 4;
  const multipleBreakdownNetworks = new Set(data.networkBreakdown.map(item => item.network)).size > 1;
  const breakdownRows = data.networkBreakdown.map(item => ({
    name: multipleBreakdownNetworks
      ? `${item.network} · ${item.channel} · ${item.segment}`
      : `${item.channel} · ${item.segment}`,
    value: item.value,
  }));

  const mainTooltipFormatter = (value: number | string, name: string, item: { payload?: Record<string, unknown> }) => {
    const numericValue = Number(value);
    const networkSeries = focusSeries.find(series => series.type === 'network' && series.name === name);
    if (networkSeries) {
      const total = Number(item.payload?.value || 0);
      const share = total > 0 ? ` · доля ${((numericValue / total) * 100).toFixed(1)}%` : '';
      return [`${fullNumber(numericValue, data.unit)}${share}`, name];
    }
    return [fullNumber(numericValue, data.unit), name];
  };

  return (
    <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', pr: 0.5 }}>
      <Grid container spacing={1.5} sx={{ mb: 1.5 }}>
        <Grid size={{ xs: 12, sm: 6, lg: 3 }}><KpiCard label="За выбранный период" value={fullNumber(data.summary.total, data.unit)} hint={`${data.summary.periods} мес. в расчёте`} accent="#6366f1" /></Grid>
        <Grid size={{ xs: 12, sm: 6, lg: 3 }}><KpiCard label="Среднее за месяц" value={fullNumber(data.summary.averagePerMonth, data.unit)} hint={`Сегмент: ${data.segment}`} accent="#0ea5e9" /></Grid>
        <Grid size={{ xs: 12, sm: 6, lg: 3 }}><KpiCard label={`Последний период · ${latest.period}`} value={fullNumber(latest.value, data.unit)} hint={`${changeLabel(data.summary.latestValue, data.summary.previousValue, 'к пред. месяцу')} · ${changeLabel(data.summary.latestValue, data.summary.yearAgoValue, 'год к году')}`} accent={monthChange != null && monthChange < 0 ? '#ef4444' : '#10b981'} /></Grid>
        <Grid size={{ xs: 12, sm: 6, lg: 3 }}><KpiCard label="Охват" value={`${data.summary.activeNetworks} сетей`} hint={`${data.summary.activeProducts} продуктов`} accent="#f59e0b" /></Grid>
      </Grid>

      <Grid container spacing={1.5} sx={{ mb: 1.5 }}>
        <Grid size={{ xs: 12, lg: 8 }}>
          <Paper variant="outlined" sx={{ p: 2, height: 390, borderRadius: 3, borderColor: '#e2e8f0' }}>
            <Stack direction="row" justifyContent="space-between" alignItems="baseline" sx={{ mb: 1 }}>
              <Box>
                <Stack direction="row" spacing={1} alignItems="center">
                  <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>Динамика продаж</Typography>
                  {focuses.map(focus => (
                    <Chip key={`${focus.type}:${focus.name}`} size="small"
                      label={`${focus.type === 'product' ? 'SKU' : 'Сеть'}: ${focus.name}`}
                      onDelete={() => onRemoveFocus(focus)} color="primary" variant="outlined" />
                  ))}
                  {focuses.length > 1 && <Chip size="small" label="Сбросить выбор" onClick={onClearFocus} />}
                </Stack>
                <Typography variant="caption" color="text.secondary">
                  {data.channel ? `${data.channel} → ` : ''}{data.segment} · {data.unit === 'руб' ? 'в рублях' : 'в упаковках'}
                  {useFocusAxis ? ' · выбранные серии по правой шкале' : ''}
                </Typography>
              </Box>
              {loading && <CircularProgress size={18} />}
            </Stack>
            <ResponsiveContainer width="100%" height={315}>
              <LineChart data={chartTrend} margin={{ top: 10, right: 18, left: 4, bottom: 8 }}>
                <CartesianGrid stroke="#e2e8f0" strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="period" tick={{ fontSize: 11 }} minTickGap={24} />
                <YAxis yAxisId="main" tickFormatter={compactNumber} tick={{ fontSize: 11 }} width={58} />
                {useFocusAxis && <YAxis yAxisId="focus" orientation="right" tickFormatter={compactNumber} tick={{ fontSize: 11 }} width={58} />}
                <Tooltip formatter={mainTooltipFormatter} labelFormatter={(label) => `Период: ${label}`} />
                {(focusSeries.length > 0 || channelSeries.length > 0) && <Legend />}
                <Line yAxisId="main" type="monotone" dataKey="value" name={focusSeries.length || channelSeries.length ? 'Основной срез' : 'Продажи'} stroke={focusSeries.length || channelSeries.length ? '#94a3b8' : '#6366f1'} strokeWidth={focusSeries.length || channelSeries.length ? 2 : 3} strokeDasharray={focusSeries.length || channelSeries.length ? '6 4' : undefined} dot={false} activeDot={{ r: 5 }} />
                {channelSeries.map((series, index) => (
                  <Line key={series.key} yAxisId="main" type="monotone" dataKey={series.key} name={`Канал: ${series.name}`}
                    stroke={channelColors[index % channelColors.length]} strokeWidth={2.5} dot={false} activeDot={{ r: 4 }} connectNulls />
                ))}
                {focusSeries.map((series, index) => (
                  <Line key={series.key} yAxisId={useFocusAxis ? 'focus' : 'main'} type="monotone" dataKey={series.key} name={series.name}
                    stroke={focusColors[index % focusColors.length]} strokeWidth={3} dot={false} activeDot={{ r: 5 }} connectNulls />
                ))}
              </LineChart>
            </ResponsiveContainer>
          </Paper>
        </Grid>
        <Grid size={{ xs: 12, lg: 4 }}>
          <RankList title="Лидеры среди сетей" rows={data.topNetworks} unit={data.unit} color="#0ea5e9"
            selectedNames={networkFocusNames} onSelect={onNetworkSelect} />
        </Grid>
      </Grid>

      <Paper variant="outlined" sx={{ p: 2, height: 350, borderRadius: 3, borderColor: '#e2e8f0' }}>
        {showNetworkBreakdown ? (
          <>
            <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>Детализация сети по каналам и сегментам</Typography>
            <Typography variant="caption" color="text.secondary">Срезы показаны отдельно и не складываются между собой, если они пересекаются</Typography>
            <ResponsiveContainer width="100%" height={285}>
              <BarChart data={[...breakdownRows].reverse()} layout="vertical" margin={{ top: 12, right: 24, left: 16, bottom: 0 }}>
                <CartesianGrid stroke="#f1f5f9" horizontal={false} />
                <XAxis type="number" tickFormatter={compactNumber} tick={{ fontSize: 10 }} />
                <YAxis type="category" dataKey="name" width={multipleBreakdownNetworks ? 330 : 250} tick={{ fontSize: 10 }} />
                <Tooltip formatter={(value) => [`${Number(value).toLocaleString('ru-RU')} ${unitLabel}`, 'Продажи']} />
                <Bar dataKey="value" fill="#0f766e" radius={[0, 5, 5, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </>
        ) : showSegmentComparison ? (
          <>
            <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>Сегменты канала «{data.channel}»</Typography>
            <Typography variant="caption" color="text.secondary">Нажмите на сегмент, чтобы перестроить основной график</Typography>
            <ResponsiveContainer width="100%" height={285}>
              <BarChart data={[...data.segmentTotals].reverse()} layout="vertical" margin={{ top: 12, right: 24, left: 16, bottom: 0 }}>
                <CartesianGrid stroke="#f1f5f9" horizontal={false} />
                <XAxis type="number" tickFormatter={compactNumber} tick={{ fontSize: 10 }} />
                <YAxis type="category" dataKey="name" width={210} tick={{ fontSize: 10 }} />
                <Tooltip formatter={(value) => [`${Number(value).toLocaleString('ru-RU')} ${unitLabel}`, 'Продажи']} />
                <Bar dataKey="value" fill="#14b8a6" radius={[0, 5, 5, 0]} cursor="pointer"
                  onClick={(entry) => onSegmentSelect((entry as DashboardRank).name)} />
              </BarChart>
            </ResponsiveContainer>
          </>
        ) : showNetworkDynamics ? (
          <>
            <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>Динамика SKU по ведущим сетям</Typography>
            <Typography variant="caption" color="text.secondary">Показывается вместо неинформативного графика с единственным SKU</Typography>
            <ResponsiveContainer width="100%" height={285}>
              <LineChart data={networkTrend} margin={{ top: 12, right: 24, left: 4, bottom: 8 }}>
                <CartesianGrid stroke="#e2e8f0" strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="period" tick={{ fontSize: 10 }} minTickGap={24} />
                <YAxis tickFormatter={compactNumber} tick={{ fontSize: 10 }} width={58} />
                <Tooltip formatter={(value, name) => [fullNumber(Number(value), data.unit), name]} />
                <Legend />
                {networkLineNames.map((name, index) => (
                  <Line key={name} type="monotone" dataKey={name} stroke={focusColors[index % focusColors.length]} strokeWidth={2.2} dot={false} connectNulls cursor="pointer"
                    onClick={(_entry, _index, event) => onNetworkSelect(name, Boolean(event?.ctrlKey || event?.metaKey))} />
                ))}
              </LineChart>
            </ResponsiveContainer>
          </>
        ) : (
          <>
            <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>Топ SKU</Typography>
            <Typography variant="caption" color="text.secondary">Нажмите на SKU, чтобы сравнить его динамику с общим результатом</Typography>
            <ResponsiveContainer width="100%" height={285}>
              <BarChart data={[...data.topProducts].reverse()} layout="vertical" margin={{ top: 12, right: 24, left: 16, bottom: 0 }}>
                <CartesianGrid stroke="#f1f5f9" horizontal={false} />
                <XAxis type="number" tickFormatter={compactNumber} tick={{ fontSize: 10 }} />
                <YAxis type="category" dataKey="name" width={210} tick={{ fontSize: 10 }} />
                <Tooltip formatter={(value) => [`${Number(value).toLocaleString('ru-RU')} ${unitLabel}`, 'Продажи']} />
                <Bar dataKey="value" radius={[0, 5, 5, 0]} cursor="pointer"
                  onClick={(entry, _index, event) => onProductSelect((entry as DashboardRank).name, Boolean(event?.ctrlKey || event?.metaKey))}>
                  {[...data.topProducts].reverse().map(item => (
                    <Cell key={item.name} fill={productFocusNames.has(item.name) ? '#4f46e5' : '#8b5cf6'}
                      stroke={productFocusNames.has(item.name) ? '#312e81' : 'none'} strokeWidth={2} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </>
        )}
      </Paper>

      {data.networkTrends.length > 0 && (
        <NetworkHeatmap
          rows={data.networkTrends}
          networkOrder={data.topNetworks.map(item => item.name)}
          unit={data.unit}
          selectedNames={networkFocusNames}
          onSelect={onNetworkSelect}
        />
      )}
    </Box>
  );
}
