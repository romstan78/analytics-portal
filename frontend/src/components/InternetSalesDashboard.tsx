import { useEffect, useMemo, useState } from 'react';
import {
  Alert, Box, Chip, CircularProgress, Grid, Paper, Stack, Tab, Table, TableBody,
  TableCell, TableHead, TableRow, Tabs, ToggleButton, ToggleButtonGroup, Typography,
} from '@mui/material';
import {
  Bar, BarChart, CartesianGrid, Cell, Legend, Line, LineChart, ReferenceLine,
  ResponsiveContainer, Tooltip, XAxis, YAxis,
} from 'recharts';

interface DashboardPoint { year: number; month: number; value: number }
interface DashboardRank { name: string; value: number }
interface DashboardSeriesPoint extends DashboardPoint { name: string }
interface DashboardFocusPoint extends DashboardSeriesPoint { type: 'product' | 'network' }
interface DashboardNetworkBreakdown { network: string; channel: string; segment: string; value: number }
interface DashboardComparison { current: number; previous: number }
interface DashboardDriver { name: string; current: number; previous: number; delta: number; deltaPercent: number | null }
interface DashboardRankDetail { name: string; value: number; previous: number; yoyPercent: number | null; share: number; rank: number; rankChange: number }
interface DashboardEcomShare {
  applicable: boolean; family: string; full: number; withoutEcom: number; ecom: number;
  share: number | null; previousFull: number; previousEcom: number; previousShare: number | null;
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
  analysisYear: number;
  channel: string;
  channelSegments: string[];
  segment: string;
  unit: 'руб' | 'евро' | 'уп';
  summary: DashboardSummary;
  trend: DashboardPoint[];
  previousYearTrend: DashboardPoint[];
  metricComparisons: { rub: DashboardComparison; eur: DashboardComparison; units: DashboardComparison };
  currencySource: string;
  ecomShare: DashboardEcomShare;
  networkDrivers: DashboardDriver[];
  productDrivers: DashboardDriver[];
  networkRanking: DashboardRankDetail[];
  productRanking: DashboardRankDetail[];
  focusTrends: DashboardFocusPoint[];
  topNetworks: DashboardRank[];
  topProducts: DashboardRank[];
  segmentTotals: DashboardRank[];
  networkTrends: DashboardSeriesPoint[];
  channelTrends: DashboardSeriesPoint[];
  networkBreakdown: DashboardNetworkBreakdown[];
}

export interface DashboardFocus { type: 'product' | 'network'; name: string }

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

const MONTHS = ['Янв', 'Фев', 'Мар', 'Апр', 'Май', 'Июн', 'Июл', 'Авг', 'Сен', 'Окт', 'Ноя', 'Дек'];
const SERIES_COLORS = ['#5b5bd6', '#d14f8a', '#d58a20', '#168b7a', '#7a52b3'];
const CHANNEL_COLORS = ['#087f8c', '#1875c1', '#08705f', '#8b4bb3', '#bd6428'];

const compactNumber = (value: number) => new Intl.NumberFormat('ru-RU', { notation: 'compact', maximumFractionDigits: 1 }).format(Number(value) || 0);
const rawNumber = (value: number, digits = 0) => Number(value || 0).toLocaleString('ru-RU', { maximumFractionDigits: digits });
const unitSymbol = (unit: string) => unit === 'руб' ? '₽' : unit === 'евро' ? '€' : 'шт.';
const fullNumber = (value: number, unit: string) => `${rawNumber(value, unit === 'евро' ? 2 : 0)} ${unitSymbol(unit)}`;
const percent = (current: number, previous: number) => previous ? ((current - previous) / previous) * 100 : null;
const percentLabel = (value: number | null) => value == null ? 'нет базы сравнения' : `${value >= 0 ? '+' : ''}${value.toFixed(1)}%`;

function KpiCard({ label, value, previous, change, accent }: { label: string; value: string; previous: string; change?: number | null; accent: string }) {
  const positive = change != null && change >= 0;
  return (
    <Paper variant="outlined" sx={{ p: 1.75, height: '100%', borderRadius: 3, borderColor: '#dfe5ee', position: 'relative', overflow: 'hidden' }}>
      <Box sx={{ position: 'absolute', inset: '0 auto 0 0', width: 4, bgcolor: accent }} />
      <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 650 }}>{label}</Typography>
      <Typography sx={{ fontSize: { xs: '1.25rem', xl: '1.55rem' }, fontWeight: 760, lineHeight: 1.3, mt: 0.25 }}>{value}</Typography>
      {change === undefined ? (
        <Typography variant="caption" color="text.secondary">{previous}</Typography>
      ) : (
        <Stack direction="row" spacing={0.75} alignItems="baseline">
          <Typography variant="caption" sx={{ color: change == null ? 'text.secondary' : positive ? '#12805c' : '#c14545', fontWeight: 700 }}>{percentLabel(change)}</Typography>
          <Typography variant="caption" color="text.secondary">к {previous}</Typography>
        </Stack>
      )}
    </Paper>
  );
}

function EcomKpiCard({ data, unit }: { data: DashboardEcomShare; unit: string }) {
  const delta = data.share != null && data.previousShare != null ? data.share - data.previousShare : null;
  return (
    <Paper variant="outlined" sx={{ p: 1.75, height: '100%', borderRadius: 3, borderColor: '#dfe5ee', position: 'relative', overflow: 'hidden' }}>
      <Box sx={{ position: 'absolute', inset: '0 auto 0 0', width: 4, bgcolor: '#7a52b3' }} />
      <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 650 }}>Доля Ecom · {data.family}</Typography>
      <Typography sx={{ fontSize: { xs: '1.25rem', xl: '1.55rem' }, fontWeight: 760, lineHeight: 1.3, mt: 0.25 }}>
        {data.share == null ? '—' : `${data.share.toFixed(1)}%`}
      </Typography>
      <Typography variant="caption" sx={{ color: delta == null ? 'text.secondary' : delta >= 0 ? '#12805c' : '#c14545', fontWeight: 700 }}>
        {delta == null ? 'нет базы сравнения' : `${delta >= 0 ? '+' : ''}${delta.toFixed(1)} п.п.`}
      </Typography>
      <Typography variant="caption" color="text.secondary"> · Ecom {fullNumber(data.ecom, unit)}</Typography>
    </Paper>
  );
}

function NetworkHeatmap({ rows, networkOrder, unit, selectedNames, onSelect }: {
  rows: DashboardSeriesPoint[]; networkOrder: string[]; unit: string; selectedNames: Set<string>;
  onSelect: (name: string, additive: boolean) => void;
}) {
  const periods = [...new Set(rows.map(point => point.month))].sort((a, b) => a - b);
  const values = new Map(rows.map(point => [`${point.name}\u0000${point.month}`, point.value]));
  const rowMax = new Map(networkOrder.map(name => [name, Math.max(...periods.map(month => values.get(`${name}\u0000${month}`) || 0), 1)]));
  return (
    <Box sx={{ overflowX: 'auto', pb: 0.5 }}>
      <Box sx={{ minWidth: 690 }}>
        <Box sx={{ display: 'grid', gridTemplateColumns: 'minmax(180px, 1.4fr) repeat(12, minmax(34px, 1fr))', gap: 0.5, mb: 0.5 }}>
          <Box />
          {MONTHS.map(month => <Typography key={month} variant="caption" color="text.secondary" textAlign="center">{month}</Typography>)}
        </Box>
        {networkOrder.map(name => (
          <Box key={name} sx={{ display: 'grid', gridTemplateColumns: 'minmax(180px, 1.4fr) repeat(12, minmax(34px, 1fr))', gap: 0.5, mb: 0.5 }}>
            <Box component="button" type="button" onClick={(event) => onSelect(name, event.ctrlKey || event.metaKey)}
              sx={{ border: 0, borderRadius: 1, bgcolor: selectedNames.has(name) ? '#eef0ff' : 'transparent', px: 0.75, textAlign: 'left', cursor: 'pointer', overflow: 'hidden' }}>
              <Typography variant="caption" noWrap sx={{ display: 'block', fontWeight: selectedNames.has(name) ? 750 : 500 }}>{name}</Typography>
            </Box>
            {Array.from({ length: 12 }, (_, index) => index + 1).map(month => {
              const value = values.get(`${name}\u0000${month}`) || 0;
              const intensity = value / (rowMax.get(name) || 1);
              return <Box key={month} component="button" type="button" onClick={(event) => onSelect(name, event.ctrlKey || event.metaKey)}
                title={`${name} · ${MONTHS[month - 1]}: ${fullNumber(value, unit)}`}
                sx={{ height: 27, border: selectedNames.has(name) ? '1px solid #5558d5' : '1px solid #edf0f4', borderRadius: 0.8,
                  bgcolor: value === 0 ? '#f7f8fa' : `rgba(34, 139, 126, ${0.12 + intensity * 0.78})`, cursor: 'pointer' }} />;
            })}
          </Box>
        ))}
      </Box>
    </Box>
  );
}

export default function InternetSalesDashboard({
  data, loading, error, focuses, onProductSelect, onNetworkSelect, onSegmentSelect, onRemoveFocus, onClearFocus,
}: InternetSalesDashboardProps) {
  const [trendMode, setTrendMode] = useState<'year' | 'comparison'>('year');
  const [cumulative, setCumulative] = useState(false);
  const [driverDimension, setDriverDimension] = useState<'network' | 'product'>('network');
  const [driverMetric, setDriverMetric] = useState<'delta' | 'percent'>('delta');
  const [bottomTab, setBottomTab] = useState<'ranking' | 'heatmap' | 'detail'>('ranking');
  const [rankDimension, setRankDimension] = useState<'network' | 'product'>('network');

  useEffect(() => {
    if (focuses.length > 0 || (data?.channelTrends.length || 0) > 0) setTrendMode('comparison');
  }, [focuses.length, data?.channelTrends.length]);

  const yearTrend = useMemo(() => {
    const currentByMonth = new Map((data?.trend || []).map(point => [point.month, point.value]));
    const previousByMonth = new Map((data?.previousYearTrend || []).map(point => [point.month, point.value]));
    let currentSum = 0;
    let previousSum = 0;
    return Array.from({ length: 12 }, (_, index) => {
      const month = index + 1;
      const currentRaw = currentByMonth.get(month);
      const previousRaw = previousByMonth.get(month);
      if (currentRaw != null) currentSum += currentRaw;
      if (previousRaw != null) previousSum += previousRaw;
      const current = currentRaw == null ? null : cumulative ? currentSum : currentRaw;
      const previous = previousRaw == null ? null : cumulative ? previousSum : previousRaw;
      return { month, label: MONTHS[index], current, previous, yoy: current != null && previous != null && previous !== 0 ? ((current - previous) / previous) * 100 : null };
    });
  }, [cumulative, data]);

  const comparisonData = useMemo(() => {
    if (!data) return [];
    const rows = new Map<string, Record<string, number | string>>();
    data.trend.forEach(point => {
      const key = `${point.year}-${String(point.month).padStart(2, '0')}`;
      rows.set(key, { period: key, overall: point.value });
    });
    focuses.forEach((focus, index) => {
      data.focusTrends.filter(point => point.type === focus.type && point.name === focus.name).forEach(point => {
        const key = `${point.year}-${String(point.month).padStart(2, '0')}`;
        const row = rows.get(key) || { period: key };
        row[`focus_${index}`] = point.value;
        rows.set(key, row);
      });
    });
    [...new Set(data.channelTrends.map(point => point.name))].forEach((name, index) => {
      data.channelTrends.filter(point => point.name === name).forEach(point => {
        const key = `${point.year}-${String(point.month).padStart(2, '0')}`;
        const row = rows.get(key) || { period: key };
        row[`channel_${index}`] = point.value;
        rows.set(key, row);
      });
    });
    return [...rows.values()].sort((a, b) => String(a.period).localeCompare(String(b.period)));
  }, [data, focuses]);

  if (loading && !data) return <Box sx={{ flex: 1, display: 'grid', placeItems: 'center' }}><CircularProgress /></Box>;
  if (error) return <Alert severity="error">{error}</Alert>;
  if (!data || data.trend.length === 0) return <Paper variant="outlined" sx={{ p: 5, textAlign: 'center', borderRadius: 3 }}><Typography color="text.secondary">По выбранным параметрам данных нет.</Typography></Paper>;

  const salesComparison = data.unit === 'евро' ? data.metricComparisons.eur : data.metricComparisons.rub;
  const salesChange = percent(salesComparison.current, salesComparison.previous);
  const unitsChange = percent(data.metricComparisons.units.current, data.metricComparisons.units.previous);
  const avgPrice = data.metricComparisons.units.current ? salesComparison.current / data.metricComparisons.units.current : 0;
  const previousAvgPrice = data.metricComparisons.units.previous ? salesComparison.previous / data.metricComparisons.units.previous : 0;
  const avgPriceChange = percent(avgPrice, previousAvgPrice);
  const unitLabel = unitSymbol(data.unit);
  const previousLabel = `${data.analysisYear - 1}`;
  const networkFocusNames = new Set(focuses.filter(item => item.type === 'network').map(item => item.name));
  const productFocusNames = new Set(focuses.filter(item => item.type === 'product').map(item => item.name));
  const channels = [...new Set(data.channelTrends.map(point => point.name))];
  const drivers = driverDimension === 'network' ? data.networkDrivers : data.productDrivers;
  const driverRows = drivers.map(item => ({ ...item, chartValue: driverMetric === 'delta' ? item.delta : item.deltaPercent })).filter(item => item.chartValue != null);
  const ranking = rankDimension === 'network' ? data.networkRanking : data.productRanking;
  const selectedRankNames = rankDimension === 'network' ? networkFocusNames : productFocusNames;
  const maxOverall = Math.max(...data.trend.map(point => point.value), 0);
  const maxSelected = Math.max(...data.focusTrends.map(point => point.value), 0);
  const useFocusAxis = focuses.length > 0 && maxSelected > 0 && maxOverall / maxSelected >= 4;

  const comparisonTooltip = (value: number | string, name: string, item: { payload?: Record<string, unknown> }) => {
    const numericValue = Number(value);
    const focusIndex = focuses.findIndex(focus => focus.name === name);
    if (focusIndex >= 0 && focuses[focusIndex].type === 'network') {
      const total = Number(item.payload?.overall || 0);
      const share = total > 0 ? ` · доля ${((numericValue / total) * 100).toFixed(1)}%` : '';
      return [`${fullNumber(numericValue, data.unit)}${share}`, name];
    }
    return [fullNumber(numericValue, data.unit), name];
  };

  return (
    <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', pr: 0.5 }}>
      <Grid container spacing={1.25} sx={{ mb: 1.25 }}>
        <Grid size={{ xs: 12, sm: 6, lg: 3 }}><KpiCard label={data.unit === 'евро' ? 'Продажи в EUR' : 'Продажи'} value={`${compactNumber(salesComparison.current)} ${data.unit === 'евро' ? '€' : '₽'}`} previous={`${compactNumber(salesComparison.previous)} ${data.unit === 'евро' ? '€' : '₽'}`} change={salesChange} accent="#5558d5" /></Grid>
        <Grid size={{ xs: 12, sm: 6, lg: 3 }}><KpiCard label="Количество" value={`${compactNumber(data.metricComparisons.units.current)} шт.`} previous={`${compactNumber(data.metricComparisons.units.previous)} шт.`} change={unitsChange} accent="#228b7e" /></Grid>
        <Grid size={{ xs: 12, sm: 6, lg: 3 }}><KpiCard label="Средняя цена" value={`${rawNumber(avgPrice, 2)} ${data.unit === 'евро' ? '€' : '₽'}`} previous={`${rawNumber(previousAvgPrice, 2)} ${data.unit === 'евро' ? '€' : '₽'}`} change={avgPriceChange} accent="#c77c1d" /></Grid>
        <Grid size={{ xs: 12, sm: 6, lg: 3 }}>
          {data.ecomShare?.applicable
            ? <EcomKpiCard data={data.ecomShare} unit={data.unit} />
            : <KpiCard label="Охват" value={`${data.summary.activeNetworks} сетей`} previous={`${data.summary.activeProducts} SKU · ${data.summary.periods} мес.`} accent="#7a52b3" />}
        </Grid>
      </Grid>

      <Grid container spacing={1.25} sx={{ mb: 1.25 }}>
        <Grid size={{ xs: 12, lg: 8 }}>
          <Paper variant="outlined" sx={{ p: 1.75, height: 455, borderRadius: 3, borderColor: '#dfe5ee' }}>
            <Stack direction={{ xs: 'column', sm: 'row' }} justifyContent="space-between" alignItems={{ xs: 'stretch', sm: 'flex-start' }} spacing={1}>
              <Box sx={{ minWidth: 0 }}>
                <Stack direction="row" spacing={0.75} alignItems="center" flexWrap="wrap" useFlexGap>
                  <Typography variant="subtitle1" sx={{ fontWeight: 750 }}>Динамика и сравнение</Typography>
                  {focuses.map(focus => <Chip key={`${focus.type}:${focus.name}`} size="small" variant="outlined" color="primary"
                    label={`${focus.type === 'product' ? 'SKU' : 'Сеть'}: ${focus.name}`} onDelete={() => onRemoveFocus(focus)} />)}
                  {focuses.length > 1 && <Chip size="small" label="Очистить" onClick={onClearFocus} />}
                </Stack>
                <Typography variant="caption" color="text.secondary">{data.channel} → {data.segment} · {data.analysisYear} против {previousLabel}{data.currencySource ? ` · ${data.currencySource}` : ''}</Typography>
              </Box>
              <Stack direction="row" spacing={0.75}>
                <ToggleButtonGroup size="small" exclusive value={trendMode} onChange={(_, value) => value && setTrendMode(value)}>
                  <ToggleButton value="year">Год к году</ToggleButton>
                  <ToggleButton value="comparison">Выбранные</ToggleButton>
                </ToggleButtonGroup>
                {trendMode === 'year' && <ToggleButtonGroup size="small" exclusive value={cumulative ? 'cumulative' : 'monthly'} onChange={(_, value) => value && setCumulative(value === 'cumulative')}>
                  <ToggleButton value="monthly">Месяц</ToggleButton><ToggleButton value="cumulative">Накоп.</ToggleButton>
                </ToggleButtonGroup>}
              </Stack>
            </Stack>

            {trendMode === 'year' ? (
              <>
                <ResponsiveContainer width="100%" height={270}>
                  <LineChart data={yearTrend} margin={{ top: 18, right: 18, left: 4, bottom: 0 }}>
                    <CartesianGrid stroke="#e8ebf0" strokeDasharray="3 3" vertical={false} />
                    <XAxis dataKey="label" tick={{ fontSize: 11 }} />
                    <YAxis tickFormatter={compactNumber} tick={{ fontSize: 11 }} width={58} />
                    <Tooltip formatter={(value, name) => [fullNumber(Number(value), data.unit), name === 'current' ? String(data.analysisYear) : previousLabel]} />
                    <Legend formatter={(value) => value === 'current' ? String(data.analysisYear) : previousLabel} />
                    <Line type="monotone" dataKey="current" connectNulls={false} stroke="#5558d5" strokeWidth={3} dot={false} activeDot={{ r: 5 }} />
                    <Line type="monotone" dataKey="previous" connectNulls stroke="#64748b" strokeWidth={2.2} strokeDasharray="7 5" dot={false} />
                  </LineChart>
                </ResponsiveContainer>
                <Box sx={{ height: 100, mt: -0.5 }}>
                  <Typography variant="caption" color="text.secondary" sx={{ ml: 1 }}>Изменение год к году, %</Typography>
                  <ResponsiveContainer width="100%" height="86%">
                    <BarChart data={yearTrend} margin={{ top: 4, right: 18, left: 4, bottom: 0 }}>
                      <XAxis dataKey="label" tick={{ fontSize: 10 }} axisLine={false} tickLine={false} />
                      <YAxis tickFormatter={(value) => `${value}%`} tick={{ fontSize: 9 }} width={58} domain={['auto', 'auto']} />
                      <ReferenceLine y={0} stroke="#94a3b8" />
                      <Tooltip formatter={(value) => [`${Number(value).toFixed(1)}%`, 'YoY']} />
                      <Bar dataKey="yoy" radius={[3, 3, 0, 0]}>{yearTrend.map((row) => <Cell key={row.month} fill={(row.yoy || 0) >= 0 ? '#2b9a78' : '#dc6b55'} />)}</Bar>
                    </BarChart>
                  </ResponsiveContainer>
                </Box>
              </>
            ) : (
              <ResponsiveContainer width="100%" height={360}>
                <LineChart data={comparisonData} margin={{ top: 22, right: 18, left: 4, bottom: 8 }}>
                  <CartesianGrid stroke="#e8ebf0" strokeDasharray="3 3" vertical={false} />
                  <XAxis dataKey="period" tick={{ fontSize: 11 }} minTickGap={24} />
                  <YAxis yAxisId="main" tickFormatter={compactNumber} tick={{ fontSize: 11 }} width={58} />
                  {useFocusAxis && <YAxis yAxisId="selected" orientation="right" tickFormatter={compactNumber} tick={{ fontSize: 11 }} width={58} />}
                  <Tooltip formatter={comparisonTooltip} />
                  <Legend />
                  <Line yAxisId="main" type="monotone" dataKey="overall" name="Общий срез" stroke="#a0a8b5" strokeWidth={2} strokeDasharray="6 4" dot={false} />
                  {channels.map((name, index) => <Line key={name} yAxisId="main" type="monotone" dataKey={`channel_${index}`} name={`Канал: ${name}`} stroke={CHANNEL_COLORS[index % CHANNEL_COLORS.length]} strokeWidth={2.5} dot={false} connectNulls />)}
                  {focuses.map((focus, index) => <Line key={`${focus.type}:${focus.name}`} yAxisId={useFocusAxis ? 'selected' : 'main'} type="monotone" dataKey={`focus_${index}`} name={focus.name} stroke={SERIES_COLORS[index % SERIES_COLORS.length]} strokeWidth={3} dot={false} connectNulls />)}
                </LineChart>
              </ResponsiveContainer>
            )}
          </Paper>
        </Grid>

        <Grid size={{ xs: 12, lg: 4 }}>
          <Paper variant="outlined" sx={{ p: 1.75, height: 455, borderRadius: 3, borderColor: '#dfe5ee' }}>
            <Stack direction="row" justifyContent="space-between" alignItems="flex-start" spacing={1}>
              <Box><Typography variant="subtitle1" sx={{ fontWeight: 750 }}>Что изменило результат</Typography><Typography variant="caption" color="text.secondary">Вклад относительно {previousLabel}</Typography></Box>
              <Stack spacing={0.65} alignItems="flex-end">
                <ToggleButtonGroup size="small" exclusive value={driverDimension} onChange={(_, value) => value && setDriverDimension(value)}><ToggleButton value="network">Сети</ToggleButton><ToggleButton value="product">SKU</ToggleButton></ToggleButtonGroup>
                <ToggleButtonGroup size="small" exclusive value={driverMetric} onChange={(_, value) => value && setDriverMetric(value)}><ToggleButton value="delta">Вклад</ToggleButton><ToggleButton value="percent">YoY %</ToggleButton></ToggleButtonGroup>
              </Stack>
            </Stack>
            <ResponsiveContainer width="100%" height={365}>
              <BarChart data={[...driverRows].reverse()} layout="vertical" margin={{ top: 12, right: 16, left: 20, bottom: 0 }}>
                <CartesianGrid stroke="#edf0f4" horizontal={false} />
                <XAxis type="number" tickFormatter={driverMetric === 'delta' ? compactNumber : (value) => `${value}%`} tick={{ fontSize: 10 }} />
                <YAxis type="category" dataKey="name" width={128} tick={{ fontSize: 10 }} tickFormatter={(value) => String(value).length > 20 ? `${String(value).slice(0, 18)}…` : String(value)} />
                <ReferenceLine x={0} stroke="#7b8797" />
                <Tooltip formatter={(value) => [driverMetric === 'delta' ? `${rawNumber(Number(value))} ${unitLabel}` : `${Number(value).toFixed(1)}%`, driverMetric === 'delta' ? 'Вклад' : 'YoY']} />
                <Bar dataKey="chartValue" radius={[0, 4, 4, 0]}>{[...driverRows].reverse().map(item => <Cell key={item.name} fill={Number(item.chartValue) >= 0 ? '#2b9a78' : '#dc6b55'} />)}</Bar>
              </BarChart>
            </ResponsiveContainer>
          </Paper>
        </Grid>
      </Grid>

      <Paper variant="outlined" sx={{ borderRadius: 3, borderColor: '#dfe5ee', overflow: 'hidden', mb: 1.25 }}>
        <Stack direction={{ xs: 'column', sm: 'row' }} justifyContent="space-between" alignItems={{ xs: 'stretch', sm: 'center' }} sx={{ px: 1.75, borderBottom: '1px solid #e7ebf1' }}>
          <Tabs value={bottomTab} onChange={(_, value) => setBottomTab(value)}>
            <Tab value="ranking" label="Рейтинг" /><Tab value="heatmap" label="Сезонность" /><Tab value="detail" label="Детализация" />
          </Tabs>
          {bottomTab === 'ranking' && <ToggleButtonGroup size="small" exclusive value={rankDimension} onChange={(_, value) => value && setRankDimension(value)}><ToggleButton value="network">Сети</ToggleButton><ToggleButton value="product">SKU</ToggleButton></ToggleButtonGroup>}
        </Stack>

        <Box sx={{ p: 1.75, minHeight: 315 }}>
          {bottomTab === 'ranking' && (
            <>
              <Typography variant="caption" color="text.secondary">Нажмите на строку для сравнения; Ctrl/⌘ добавляет несколько позиций.</Typography>
              <Table size="small" sx={{ mt: 0.75 }}>
                <TableHead><TableRow><TableCell width={55}>№</TableCell><TableCell>{rankDimension === 'network' ? 'Сеть' : 'SKU'}</TableCell><TableCell align="right">{data.analysisYear}</TableCell><TableCell align="right">YoY</TableCell><TableCell align="right">Доля</TableCell><TableCell align="right">Позиция</TableCell></TableRow></TableHead>
                <TableBody>{ranking.map(row => <TableRow key={row.name} hover selected={selectedRankNames.has(row.name)} sx={{ cursor: 'pointer' }}
                  onClick={(event) => rankDimension === 'network' ? onNetworkSelect(row.name, event.ctrlKey || event.metaKey) : onProductSelect(row.name, event.ctrlKey || event.metaKey)}>
                  <TableCell>{row.rank}</TableCell><TableCell sx={{ fontWeight: 600 }}>{row.name}</TableCell><TableCell align="right">{compactNumber(row.value)} {unitLabel}</TableCell>
                  <TableCell align="right" sx={{ color: row.yoyPercent == null ? 'text.secondary' : row.yoyPercent >= 0 ? '#12805c' : '#c14545', fontWeight: 650 }}>{percentLabel(row.yoyPercent)}</TableCell>
                  <TableCell align="right">{row.share.toFixed(1)}%</TableCell><TableCell align="right">{row.rankChange === 0 ? '—' : `${row.rankChange > 0 ? '↑' : '↓'} ${Math.abs(row.rankChange)}`}</TableCell>
                </TableRow>)}</TableBody>
              </Table>
            </>
          )}

          {bottomTab === 'heatmap' && (
            <><Typography variant="subtitle2" sx={{ fontWeight: 750 }}>Сезонность ведущих сетей</Typography><Typography variant="caption" color="text.secondary">Интенсивность цвета нормализована внутри каждой сети: так видны её сильные и слабые месяцы.</Typography>
              <Box sx={{ mt: 1.5 }}><NetworkHeatmap rows={data.networkTrends} networkOrder={data.topNetworks.map(item => item.name)} unit={data.unit} selectedNames={networkFocusNames} onSelect={onNetworkSelect} /></Box></>
          )}

          {bottomTab === 'detail' && (
            data.networkBreakdown.length > 0 ? (
              <><Typography variant="subtitle2" sx={{ fontWeight: 750 }}>Сеть по каналам и сегментам</Typography><Typography variant="caption" color="text.secondary">Пересекающиеся срезы показаны отдельно и не складываются.</Typography>
                <ResponsiveContainer width="100%" height={255}>
                  <BarChart data={[...data.networkBreakdown].sort((a, b) => a.value - b.value).map(item => ({ ...item, label: `${item.network} · ${item.channel} · ${item.segment}` }))} layout="vertical" margin={{ top: 12, right: 24, left: 30, bottom: 0 }}>
                    <CartesianGrid stroke="#edf0f4" horizontal={false} /><XAxis type="number" tickFormatter={compactNumber} tick={{ fontSize: 10 }} /><YAxis type="category" dataKey="label" width={270} tick={{ fontSize: 10 }} />
                    <Tooltip formatter={(value) => [fullNumber(Number(value), data.unit), 'Продажи']} /><Bar dataKey="value" fill="#187f75" radius={[0, 4, 4, 0]} />
                  </BarChart>
                </ResponsiveContainer></>
            ) : data.segmentTotals.length > 1 ? (
              <><Typography variant="subtitle2" sx={{ fontWeight: 750 }}>Сегменты канала «{data.channel}»</Typography><Typography variant="caption" color="text.secondary">Нажмите на сегмент, чтобы перестроить дашборд.</Typography>
                <ResponsiveContainer width="100%" height={255}><BarChart data={[...data.segmentTotals].reverse()} layout="vertical" margin={{ top: 12, right: 24, left: 20, bottom: 0 }}><CartesianGrid stroke="#edf0f4" horizontal={false} /><XAxis type="number" tickFormatter={compactNumber} /><YAxis type="category" dataKey="name" width={220} /><Tooltip formatter={(value) => [fullNumber(Number(value), data.unit), 'Продажи']} /><Bar dataKey="value" fill="#5558d5" cursor="pointer" onClick={(entry) => onSegmentSelect((entry as DashboardRank).name)} /></BarChart></ResponsiveContainer></>
            ) : <Box sx={{ height: 260, display: 'grid', placeItems: 'center', textAlign: 'center' }}><Box><Typography sx={{ fontWeight: 700 }}>Выберите сеть</Typography><Typography variant="body2" color="text.secondary">Детализация покажет её данные по каналам и сегментам.</Typography></Box></Box>
          )}
        </Box>
      </Paper>
    </Box>
  );
}
