import { Alert, Box, Chip, CircularProgress, Grid, Paper, Stack, Typography } from '@mui/material';
import {
  Bar, BarChart, CartesianGrid, Legend, Line, LineChart, ResponsiveContainer,
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
  focusTrend: DashboardPoint[];
  topNetworks: DashboardRank[];
  topProducts: DashboardRank[];
  segmentTotals: DashboardRank[];
  networkTrends: DashboardSeriesPoint[];
}

export interface DashboardFocus {
  type: 'product' | 'network';
  name: string;
}

interface InternetSalesDashboardProps {
  data: InternetSalesDashboardData | null;
  loading: boolean;
  error: string;
  focus: DashboardFocus | null;
  onProductSelect: (name: string) => void;
  onNetworkSelect: (name: string) => void;
  onSegmentSelect: (name: string) => void;
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

function RankList({ title, rows, unit, color, selectedName, onSelect }: {
  title: string;
  rows: DashboardRank[];
  unit: string;
  color: string;
  selectedName?: string;
  onSelect?: (name: string) => void;
}) {
  const max = Math.max(...rows.map(row => row.value), 1);
  return (
    <Paper variant="outlined" sx={{ p: 2, borderRadius: 3, borderColor: '#e2e8f0', height: '100%' }}>
      <Typography variant="subtitle1" sx={{ fontWeight: 700, mb: 1.5 }}>{title}</Typography>
      <Stack spacing={1.15}>
        {rows.map((row, index) => (
          <Box key={row.name} component="button" type="button" onClick={() => onSelect?.(row.name)}
            sx={{ display: 'block', width: '100%', p: 0.6, mx: -0.6, border: 0, borderRadius: 1.5, bgcolor: selectedName === row.name ? '#eef2ff' : 'transparent', textAlign: 'left', cursor: onSelect ? 'pointer' : 'default', '&:hover': { bgcolor: onSelect ? '#f8fafc' : undefined } }}>
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

function NetworkHeatmap({ rows, networkOrder, unit, selectedName, onSelect }: {
  rows: DashboardSeriesPoint[];
  networkOrder: string[];
  unit: string;
  selectedName?: string;
  onSelect: (name: string) => void;
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
              <Box component="button" type="button" onClick={() => onSelect(name)} title={name}
                sx={{ position: 'sticky', left: 0, zIndex: 2, border: 0, borderRadius: 1, bgcolor: selectedName === name ? '#eef2ff' : '#fff', px: 0.75, textAlign: 'left', cursor: 'pointer', overflow: 'hidden' }}>
                <Typography variant="caption" noWrap sx={{ display: 'block', fontWeight: selectedName === name ? 700 : 500 }}>{name}</Typography>
              </Box>
              {periods.map(period => {
                const value = valueMap.get(`${name}\u0000${period}`) || 0;
                const intensity = value / (rowMax.get(name) || 1);
                return (
                  <Box key={period} component="button" type="button" onClick={() => onSelect(name)}
                    title={`${name} · ${period}: ${fullNumber(value, unit)}`}
                    sx={{ height: 24, border: selectedName === name ? '1px solid #6366f1' : '1px solid transparent', borderRadius: 0.75, bgcolor: value === 0 ? '#f8fafc' : `rgba(99, 102, 241, ${0.12 + intensity * 0.82})`, cursor: 'pointer' }} />
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
  data, loading, error, focus,
  onProductSelect, onNetworkSelect, onSegmentSelect, onClearFocus,
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
  const focusByPeriod = new Map(data.focusTrend.map(point => [
    `${point.year}-${String(point.month).padStart(2, '0')}`,
    point.value,
  ]));
  const chartTrend = trend.map(point => ({
    ...point,
    focusValue: focusByPeriod.get(point.period),
  }));

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
  const focusLabel = focus?.type === 'product' ? 'SKU' : 'Сеть';
  const showSegmentComparison = data.channelSegments.length > 1 && data.segmentTotals.length > 1;
  const showNetworkDynamics = !showSegmentComparison && data.topProducts.length <= 1 && networkNames.length > 1;
  const seriesColors = ['#6366f1', '#0ea5e9', '#10b981', '#f59e0b', '#ec4899'];

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
                  {focus && <Chip size="small" label={`${focusLabel}: ${focus.name}`} onDelete={onClearFocus} color="primary" variant="outlined" />}
                </Stack>
                <Typography variant="caption" color="text.secondary">
                  {data.channel ? `${data.channel} → ` : ''}{data.segment} · {data.unit === 'руб' ? 'в рублях' : 'в упаковках'}
                </Typography>
              </Box>
              {loading && <CircularProgress size={18} />}
            </Stack>
            <ResponsiveContainer width="100%" height={315}>
              <LineChart data={chartTrend} margin={{ top: 10, right: 18, left: 4, bottom: 8 }}>
                <CartesianGrid stroke="#e2e8f0" strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="period" tick={{ fontSize: 11 }} minTickGap={24} />
                <YAxis tickFormatter={compactNumber} tick={{ fontSize: 11 }} width={58} />
                <Tooltip formatter={(value, name) => [fullNumber(Number(value), data.unit), name]} labelFormatter={(label) => `Период: ${label}`} />
                {focus && <Legend />}
                <Line type="monotone" dataKey="value" name={focus ? 'Общий результат' : 'Продажи'} stroke={focus ? '#94a3b8' : '#6366f1'} strokeWidth={focus ? 2 : 3} strokeDasharray={focus ? '6 4' : undefined} dot={false} activeDot={{ r: 5 }} />
                {focus && <Line type="monotone" dataKey="focusValue" name={focus.name} stroke="#6366f1" strokeWidth={3} dot={false} activeDot={{ r: 5 }} connectNulls />}
              </LineChart>
            </ResponsiveContainer>
          </Paper>
        </Grid>
        <Grid size={{ xs: 12, lg: 4 }}>
          <RankList title="Лидеры среди сетей" rows={data.topNetworks} unit={data.unit} color="#0ea5e9"
            selectedName={focus?.type === 'network' ? focus.name : undefined} onSelect={onNetworkSelect} />
        </Grid>
      </Grid>

      <Paper variant="outlined" sx={{ p: 2, height: 350, borderRadius: 3, borderColor: '#e2e8f0' }}>
        {showSegmentComparison ? (
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
                  <Line key={name} type="monotone" dataKey={name} stroke={seriesColors[index % seriesColors.length]} strokeWidth={2.2} dot={false} connectNulls cursor="pointer" onClick={() => onNetworkSelect(name)} />
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
                <Bar dataKey="value" fill="#8b5cf6" radius={[0, 5, 5, 0]} cursor="pointer"
                  onClick={(entry) => onProductSelect((entry as DashboardRank).name)} />
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
          selectedName={focus?.type === 'network' ? focus.name : undefined}
          onSelect={onNetworkSelect}
        />
      )}
    </Box>
  );
}
