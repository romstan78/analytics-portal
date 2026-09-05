import { useMemo, useState } from 'react';
import {
  Alert, Box, CircularProgress, FormControlLabel, Grid, MenuItem, Paper, Select, Stack,
  Switch, Tab, Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  Tabs, ToggleButton, ToggleButtonGroup, Tooltip as MuiTooltip, Typography,
} from '@mui/material';
import {
  Bar, BarChart, CartesianGrid, Cell, LabelList, Legend, Line, LineChart,
  ReferenceLine, ResponsiveContainer, Scatter, ScatterChart,
  Tooltip, XAxis, YAxis, ZAxis,
} from 'recharts';
import type {
  PromoDashboardBreakdown,
  PromoDashboardCalendarPoint,
  PromoDashboardMetrics,
  PromoDashboardResponse,
} from '../types/promo';
import {
  DRIVER_METRIC_LABEL, driverColor, driverUnitLabel, driverVariance, shownUnit, unitSwitchable,
} from '../utils/promoDrivers';
import type { DriverMetric, DriverUnit } from '../utils/promoDrivers';

const MONTHS = ['Янв', 'Фев', 'Мар', 'Апр', 'Май', 'Июн', 'Июл', 'Авг', 'Сен', 'Окт', 'Ноя', 'Дек'];
const SERIES_PLAN = '#6366f1';
const SERIES_FACT = '#149174';
const SERIES_NEUTRAL = '#8793a5';
// Подписи значений на графиках — в тон реестру сетей: приглушённый цвет, чтобы
// они читались как разметка, а не спорили с самими рядами.
//
// У всех рядов с подписями анимация выключена намеренно. Recharts отдаёт
// LabelList данные только когда ряд не анимируется (showLabels: !isAnimating),
// а состояние «анимируется» снимается лишь по событию конца анимации. Включить
// анимацию обратно — значит получить подписи, которые то появляются с задержкой,
// то не появляются вовсе. В витрине реестра она выключена по той же причине.
const LABEL_INK = '#64748b';
const BAR_LABEL_STYLE = { fontSize: 9, fontWeight: 700, fill: LABEL_INK } as const;
const LINE_LABEL_STYLE = { fontSize: 10, fontWeight: 700, fill: LABEL_INK } as const;

type DashboardView = 'overview' | 'calendar';
type BreakdownDimension = 'network' | 'brand' | 'sku' | 'mechanics';
type CalendarDimension = 'network' | 'brand';
type CalendarMetric = 'count' | 'investments' | 'completion' | 'roi';

interface PromoDashboardProps {
  data: PromoDashboardResponse | null;
  loading: boolean;
  error: string | null;
  onDrilldown: (filters: Record<string, unknown>) => void;
}

interface TrendRow {
  period: string;
  year: number;
  month: number;
  planUnits: number;
  actualUnits: number | null;
  planInvestments: number;
  actualInvestments: number | null;
  planRoi: number | null;
  actualRoi: number | null;
  planUplift: number;
  actualUplift: number | null;
  coverage: number | null;
}

interface BubblePoint {
  name: string;
  completion: number;
  roi: number;
  investments: number;
  coverage: number | null;
  color: string;
  breakdown: PromoDashboardBreakdown;
}

interface DriverPoint {
  name: string;
  value: number;
  breakdown: PromoDashboardBreakdown;
}

const compactNumber = (value: number | null | undefined) => {
  if (value == null || !Number.isFinite(value)) return '—';
  return new Intl.NumberFormat('ru-RU', { notation: 'compact', maximumFractionDigits: 1 }).format(value);
};

const fullNumber = (value: number | null | undefined, digits = 0) => {
  if (value == null || !Number.isFinite(value)) return '—';
  return new Intl.NumberFormat('ru-RU', { maximumFractionDigits: digits }).format(value);
};

const percentLabel = (value: number | null | undefined, digits = 1) =>
  value == null || !Number.isFinite(value) ? '—' : `${fullNumber(value, digits)}%`;

// Пустое значение не подписываем вовсе: у промо без факта столбца нет, и «—»
// повисло бы в пустоте вместо отсутствующего ряда.
const labelText = (value: unknown, format: (numeric: number) => string) =>
  value == null || !Number.isFinite(Number(value)) ? '' : format(Number(value));

// Recharts сортирует легенду по value, а подсказку по name, поэтому факт вставал
// перед планом независимо от порядка рядов в разметке. Ранжируем явно: план
// первым, факт вторым.
const planFirst = (item: { value?: unknown; name?: unknown; dataKey?: unknown }) =>
  String(item.dataKey ?? item.name ?? item.value ?? '').startsWith('plan') ? 0 : 1;

const dimensionFilter: Record<BreakdownDimension, string> = {
  network: 'network_name', brand: 'brand', sku: 'sku', mechanics: 'mechanics',
};

const dimensionLabel: Record<BreakdownDimension, string> = {
  network: 'Сети', brand: 'Бренды', sku: 'SKU', mechanics: 'Механики',
};

const comparablePlan = (metrics: PromoDashboardMetrics, field: 'units' | 'investments') =>
  field === 'units' ? metrics.comparablePlanUnits : metrics.comparablePlanInvestmentsRub;

function KpiCard({
  label, primary, secondary, hint, accent,
}: { label: string; primary: string; secondary?: string; hint?: string; accent: string }) {
  return (
    <Paper variant="outlined" sx={{ p: 1.6, height: '100%', borderRadius: 3, borderColor: '#dfe5ee', borderTop: `3px solid ${accent}` }}>
      <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 650 }}>{label}</Typography>
      <Typography variant="h6" sx={{ mt: 0.35, fontWeight: 780, lineHeight: 1.2 }}>{primary}</Typography>
      {secondary && <Typography variant="body2" sx={{ mt: 0.45, fontWeight: 600 }}>{secondary}</Typography>}
      {hint && <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.35 }}>{hint}</Typography>}
    </Paper>
  );
}

function ChartPaper({ title, subtitle, children }: { title: string; subtitle: string; children: React.ReactNode }) {
  return (
    <Paper variant="outlined" sx={{ p: 1.6, height: '100%', borderRadius: 3, borderColor: '#dfe5ee' }}>
      <Typography variant="subtitle1" sx={{ fontWeight: 750 }}>{title}</Typography>
      <Typography variant="caption" color="text.secondary">{subtitle}</Typography>
      <Box sx={{ height: 260, mt: 0.5 }}>{children}</Box>
    </Paper>
  );
}

function BubbleTooltip({ active, payload }: { active?: boolean; payload?: Array<{ payload: BubblePoint }> }) {
  const point = payload?.[0]?.payload;
  if (!active || !point) return null;
  // План сопоставимый: выполнение считается от него же, иначе подсказка
  // расходилась бы с осью выполнения.
  const metrics = point.breakdown.metrics;
  return (
    <Paper sx={{ p: 1.25, border: '1px solid #dfe5ee', maxWidth: 280 }}>
      <Typography variant="subtitle2" sx={{ fontWeight: 750 }}>{point.name}</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>Продажи: план {compactNumber(metrics.comparablePlanUnits)} · факт {compactNumber(metrics.actualUnits)} уп.</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>Инвестиции: план {fullNumber(metrics.comparablePlanInvestmentsRub)} · факт {fullNumber(point.investments)} ₽</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>ROI: план {percentLabel(metrics.comparablePlanRoi)} · факт {percentLabel(point.roi)}</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>Выполнение продаж: {percentLabel(point.completion)}</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>Покрытие фактом: {percentLabel(point.coverage)}</Typography>
    </Paper>
  );
}

function breakdownColor(completion: number, roiValue: number) {
  if (completion >= 100 && roiValue >= 0) return '#149174';
  if (completion < 100 && roiValue < 0) return '#d15d50';
  return '#d18a2e';
}

function OverviewDashboard({ data, onDrilldown, showValues }: {
  data: PromoDashboardResponse;
  onDrilldown: PromoDashboardProps['onDrilldown'];
  showValues: boolean;
}) {
  const [dimension, setDimension] = useState<BreakdownDimension>('network');
  const [driverMetric, setDriverMetric] = useState<DriverMetric>('sales');
  const [driverUnit, setDriverUnit] = useState<DriverUnit>('units');

  const trend = useMemo<TrendRow[]>(() => data.trend.map(point => ({
    period: `${point.year}-${String(point.month).padStart(2, '0')}`,
    year: point.year,
    month: point.month,
    planUnits: point.metrics.planUnits,
    actualUnits: point.metrics.actualUnits,
    planInvestments: point.metrics.planInvestmentsRub,
    actualInvestments: point.metrics.actualInvestmentsRub,
    planRoi: point.metrics.comparablePlanRoi,
    actualRoi: point.metrics.actualRoi,
    planUplift: point.metrics.planUpliftUnits,
    actualUplift: point.metrics.actualUpliftUnits,
    coverage: point.metrics.factCoveragePct,
  })), [data.trend]);

  const breakdown = useMemo(() => {
    if (dimension === 'network') return data.networks;
    if (dimension === 'brand') return data.brands;
    if (dimension === 'sku') return data.skus;
    return data.mechanics;
  }, [data, dimension]);

  const drivers = useMemo<DriverPoint[]>(() => {
    const points = breakdown.flatMap(item => {
      const value = driverVariance(item.metrics, driverMetric, driverUnit);
      return value == null ? [] : [{ name: item.name, value, breakdown: item }];
    });
    return points.sort((a, b) => Math.abs(b.value) - Math.abs(a.value)).slice(0, 12).reverse();
  }, [breakdown, driverMetric, driverUnit]);

  const bubbles = useMemo<BubblePoint[]>(() => breakdown.flatMap(item => {
    const completion = item.metrics.salesCompletionPct;
    const roiValue = item.metrics.actualRoi;
    const investments = item.metrics.actualInvestmentsRub;
    if (completion == null || roiValue == null || investments == null || investments <= 0) return [];
    return [{
      name: item.name,
      completion,
      roi: roiValue,
      investments,
      coverage: item.metrics.factCoveragePct,
      color: breakdownColor(completion, roiValue),
      breakdown: item,
    }];
  }), [breakdown]);

  const openBreakdown = (item: PromoDashboardBreakdown) => {
    if (item.name === 'Не указано') return;
    onDrilldown({ [dimensionFilter[dimension]]: [item.name] });
  };
  const openPeriod = (row: TrendRow) => onDrilldown({ yearFrom: row.year, yearTo: row.year, months: [row.month] });
  const summary = data.summary;
  const driverUnitText = driverUnitLabel(driverMetric, driverUnit);

  return (
    <>
      <Grid container spacing={1.25} sx={{ mb: 1.25 }}>
        <Grid size={{ xs: 12, sm: 6, xl: 2.4 }}><KpiCard label="Покрытие фактом" primary={percentLabel(summary.factCoveragePct)} secondary={`${summary.factReadyCount.toLocaleString('ru-RU')} из ${summary.promoCount.toLocaleString('ru-RU')} промо`} hint="Факт продаж + факт инвестиций" accent="#64748b" /></Grid>
        <Grid size={{ xs: 12, sm: 6, xl: 2.4 }}><KpiCard label="Продажи, сопоставимый срез" primary={`План ${compactNumber(comparablePlan(summary, 'units'))} уп.`} secondary={`Факт ${compactNumber(summary.actualUnits)} уп. · ${percentLabel(summary.salesCompletionPct)}`} accent={SERIES_PLAN} /></Grid>
        <Grid size={{ xs: 12, sm: 6, xl: 2.4 }}><KpiCard label="Инвестиции, сопоставимый срез" primary={`План ${compactNumber(comparablePlan(summary, 'investments'))} ₽`} secondary={`Факт ${compactNumber(summary.actualInvestmentsRub)} ₽ · ${percentLabel(summary.investmentCompletionPct)}`} accent="#c57a24" /></Grid>
        <Grid size={{ xs: 12, sm: 6, xl: 2.4 }}><KpiCard label="Weighted ROI" primary={`План ${percentLabel(summary.comparablePlanRoi)}`} secondary={`Факт ${percentLabel(summary.actualRoi)}`} hint="Отношение сумм, не среднее ROI" accent={SERIES_FACT} /></Grid>
        <Grid size={{ xs: 12, sm: 6, xl: 2.4 }}><KpiCard label="Uplift, сопоставимый срез" primary={`План ${compactNumber(summary.comparablePlanUpliftUnits)} уп.`} secondary={`Факт ${compactNumber(summary.actualUpliftUnits)} уп.`} accent="#8b5fbf" /></Grid>
      </Grid>

      <Grid container spacing={1.25} sx={{ mb: 1.25 }}>
        <Grid size={{ xs: 12, xl: 6 }}>
          <ChartPaper title="План–факт продаж" subtitle="Общий план периода; отсутствие столбца факта означает незаполненный факт">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={trend} margin={{ top: showValues ? 20 : 10, right: 12, left: 4, bottom: 0 }} onClick={(state) => {
                const row = trend[Number(state?.activeTooltipIndex)];
                if (row) openPeriod(row);
              }}>
                <CartesianGrid stroke="#e9edf2" strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="period" tick={{ fontSize: 10 }} minTickGap={22} />
                <YAxis tickFormatter={compactNumber} tick={{ fontSize: 10 }} width={60} />
                <Tooltip itemSorter={planFirst} formatter={(value, name) => [`${fullNumber(Number(value))} уп.`, name === 'planUnits' ? 'План' : 'Факт']} />
                <Legend itemSorter={planFirst} formatter={(value) => value === 'planUnits' ? 'План' : 'Факт'} />
                <Bar dataKey="planUnits" fill={SERIES_PLAN} opacity={0.72} radius={[3, 3, 0, 0]} cursor="pointer" isAnimationActive={false}>
                  {showValues && <LabelList dataKey="planUnits" position="top" formatter={(value) => labelText(value, compactNumber)} style={BAR_LABEL_STYLE} />}
                </Bar>
                <Bar dataKey="actualUnits" fill={SERIES_FACT} radius={[3, 3, 0, 0]} cursor="pointer" isAnimationActive={false}>
                  {showValues && <LabelList dataKey="actualUnits" position="top" formatter={(value) => labelText(value, compactNumber)} style={BAR_LABEL_STYLE} />}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </ChartPaper>
        </Grid>
        <Grid size={{ xs: 12, xl: 6 }}>
          <ChartPaper title="План–факт инвестиций" subtitle="Фактические инвестиции показаны только для сопоставимого фактического среза">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={trend} margin={{ top: showValues ? 20 : 10, right: 12, left: 4, bottom: 0 }} onClick={(state) => {
                const row = trend[Number(state?.activeTooltipIndex)];
                if (row) openPeriod(row);
              }}>
                <CartesianGrid stroke="#e9edf2" strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="period" tick={{ fontSize: 10 }} minTickGap={22} />
                <YAxis tickFormatter={compactNumber} tick={{ fontSize: 10 }} width={60} />
                <Tooltip itemSorter={planFirst} formatter={(value, name) => [`${fullNumber(Number(value))} ₽`, name === 'planInvestments' ? 'План' : 'Факт']} />
                <Legend itemSorter={planFirst} formatter={(value) => value === 'planInvestments' ? 'План' : 'Факт'} />
                <Bar dataKey="planInvestments" fill="#c57a24" opacity={0.72} radius={[3, 3, 0, 0]} cursor="pointer" isAnimationActive={false}>
                  {showValues && <LabelList dataKey="planInvestments" position="top" formatter={(value) => labelText(value, compactNumber)} style={BAR_LABEL_STYLE} />}
                </Bar>
                <Bar dataKey="actualInvestments" fill={SERIES_FACT} radius={[3, 3, 0, 0]} cursor="pointer" isAnimationActive={false}>
                  {showValues && <LabelList dataKey="actualInvestments" position="top" formatter={(value) => labelText(value, compactNumber)} style={BAR_LABEL_STYLE} />}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </ChartPaper>
        </Grid>
      </Grid>

      <Grid container spacing={1.25} sx={{ mb: 1.25 }}>
        <Grid size={{ xs: 12, xl: 5 }}>
          <ChartPaper title="Weighted ROI по месяцам" subtitle="План и факт рассчитаны на одинаковом фактическом срезе">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={trend} margin={{ top: showValues ? 20 : 10, right: 16, left: 6, bottom: 0 }}>
                <CartesianGrid stroke="#e9edf2" strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="period" tick={{ fontSize: 10 }} minTickGap={22} />
                <YAxis tickFormatter={(value) => `${fullNumber(Number(value))}%`} tick={{ fontSize: 10 }} width={62} />
                <ReferenceLine y={0} stroke={SERIES_NEUTRAL} />
                <Tooltip itemSorter={planFirst} formatter={(value, name) => [percentLabel(Number(value)), name === 'planRoi' ? 'План ROI' : 'Факт ROI']} />
                <Legend itemSorter={planFirst} formatter={(value) => value === 'planRoi' ? 'План ROI' : 'Факт ROI'} />
                <Line dataKey="planRoi" stroke={SERIES_PLAN} strokeWidth={2.4} dot={false} connectNulls={false} isAnimationActive={false}>
                  {showValues && <LabelList dataKey="planRoi" position="top" offset={8} formatter={(value) => labelText(value, (numeric) => percentLabel(numeric, 0))} style={LINE_LABEL_STYLE} />}
                </Line>
                <Line dataKey="actualRoi" stroke={SERIES_FACT} strokeWidth={2.8} dot={false} connectNulls={false} isAnimationActive={false}>
                  {showValues && <LabelList dataKey="actualRoi" position="bottom" offset={8} formatter={(value) => labelText(value, (numeric) => percentLabel(numeric, 0))} style={LINE_LABEL_STYLE} />}
                </Line>
              </LineChart>
            </ResponsiveContainer>
          </ChartPaper>
        </Grid>
        <Grid size={{ xs: 12, xl: 7 }}>
          <Paper variant="outlined" sx={{ p: 1.6, height: '100%', borderRadius: 3, borderColor: '#dfe5ee' }}>
            {/* Три группы переключателей рядом с заголовком не помещаются и
                ломаются на две строки. Заголовок занимает свою строку целиком,
                переключатели — свою, и переносить их уже нечему. */}
            <Stack direction="column" spacing={0.75}>
              <Box><Typography variant="subtitle1" sx={{ fontWeight: 750 }}>Крупнейшие отклонения</Typography><Typography variant="caption" color="text.secondary">Нажмите на столбец для перехода к исходным промо.</Typography></Box>
              <Stack direction="row" spacing={0.75} useFlexGap sx={{ flexWrap: 'nowrap', overflowX: 'auto', pb: 0.25 }}>
                <ToggleButtonGroup size="small" exclusive value={dimension} onChange={(_, value) => value && setDimension(value)}>
                  {Object.entries(dimensionLabel).map(([value, label]) => <ToggleButton key={value} value={value}>{label}</ToggleButton>)}
                </ToggleButtonGroup>
                <ToggleButtonGroup size="small" exclusive value={driverMetric} onChange={(_, value) => value && setDriverMetric(value)}>
                  {Object.entries(DRIVER_METRIC_LABEL).map(([value, label]) => <ToggleButton key={value} value={value}>{label}</ToggleButton>)}
                </ToggleButtonGroup>
                <ToggleButtonGroup
                  size="small" exclusive
                  value={shownUnit(driverMetric, driverUnit)} disabled={!unitSwitchable(driverMetric)}
                  onChange={(_, value) => value && setDriverUnit(value)}
                >
                  <ToggleButton value="units">Уп.</ToggleButton><ToggleButton value="rub">₽</ToggleButton>
                </ToggleButtonGroup>
              </Stack>
            </Stack>
            <Box sx={{ height: 260, mt: 0.5 }}>
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={drivers} layout="vertical" margin={{ top: 8, right: 18, left: 28, bottom: 0 }}>
                  <CartesianGrid stroke="#edf0f4" horizontal={false} />
                  {/* Подпись отрицательного столбца recharts рисует слева от него,
                      за его пределами. Раздвигать под неё домен бессмысленно: при
                      малых по модулю минусах и крупных плюсах отрицательная
                      половина оси занимает считанные пиксели, и подпись садится на
                      названия. padding задаёт запас прямо в пикселях и от разброса
                      данных не зависит.

                      Слева запас нужен только при отрицательных значениях: без
                      них он просто отодвинул бы столбцы от нуля. */}
                  <XAxis
                    type="number"
                    padding={showValues ? { left: drivers.some(point => point.value < 0) ? 80 : 0, right: 80 } : undefined}
                    tickFormatter={(value) => driverMetric === 'roi'
                      ? `${fullNumber(Number(value))} ${driverUnitText}`
                      : `${compactNumber(Number(value))} ${driverUnitText}`}
                    tick={{ fontSize: 10 }}
                  />
                  <YAxis type="category" dataKey="name" width={135} tick={{ fontSize: 10 }} tickFormatter={(value) => String(value).length > 20 ? `${String(value).slice(0, 18)}…` : String(value)} />
                  <ReferenceLine x={0} stroke={SERIES_NEUTRAL} />
                  <Tooltip formatter={(value) => [driverMetric === 'roi'
                    ? `${fullNumber(Number(value), 1)} ${driverUnitText}`
                    : `${fullNumber(Number(value))} ${driverUnitText}`, 'Отклонение']} />
                  <Bar dataKey="value" radius={[0, 4, 4, 0]} isAnimationActive={false}>
                    {showValues && (
                      <LabelList
                        dataKey="value" position="right"
                        formatter={(value) => labelText(value, driverMetric === 'roi'
                          ? (numeric) => `${fullNumber(numeric, 1)} ${driverUnitText}`
                          : (numeric) => `${compactNumber(numeric)} ${driverUnitText}`)}
                        style={BAR_LABEL_STYLE}
                      />
                    )}
                    {drivers.map(point => <Cell key={point.name} fill={driverColor(point.value, driverMetric)} cursor="pointer" onClick={() => openBreakdown(point.breakdown)} />)}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            </Box>
          </Paper>
        </Grid>
      </Grid>

      <Paper variant="outlined" sx={{ p: 1.6, borderRadius: 3, borderColor: '#dfe5ee' }}>
        <Stack direction={{ xs: 'column', md: 'row' }} spacing={1} sx={{ justifyContent: 'space-between', alignItems: { xs: 'stretch', md: 'flex-start' } }}>
          <Box><Typography variant="subtitle1" sx={{ fontWeight: 750 }}>Карта эффективности</Typography><Typography variant="caption" color="text.secondary">X — выполнение продаж, Y — фактический ROI, размер — фактические инвестиции. Нажмите на пузырёк для детализации.</Typography></Box>
          <ToggleButtonGroup size="small" exclusive value={dimension} onChange={(_, value) => value && setDimension(value)}>
            {Object.entries(dimensionLabel).map(([value, label]) => <ToggleButton key={value} value={value}>{label}</ToggleButton>)}
          </ToggleButtonGroup>
        </Stack>
        <Box sx={{ height: 390, mt: 0.75 }}>
          {bubbles.length > 0 ? (
            <ResponsiveContainer width="100%" height="100%">
              <ScatterChart margin={{ top: 18, right: 28, left: 16, bottom: 18 }}>
                <CartesianGrid stroke="#e7ebf0" strokeDasharray="3 3" />
                <XAxis type="number" dataKey="completion" name="Выполнение продаж" unit="%" tick={{ fontSize: 10 }} label={{ value: 'Выполнение плана продаж, %', position: 'insideBottom', offset: -10 }} />
                <YAxis type="number" dataKey="roi" name="ROI факт" unit="%" tick={{ fontSize: 10 }} width={66} label={{ value: 'ROI факт, %', angle: -90, position: 'insideLeft' }} />
                <ZAxis type="number" dataKey="investments" range={[70, 850]} />
                <ReferenceLine x={100} stroke={SERIES_NEUTRAL} strokeDasharray="5 5" />
                <ReferenceLine y={0} stroke={SERIES_NEUTRAL} strokeDasharray="5 5" />
                <Tooltip content={<BubbleTooltip />} />
                <Scatter data={bubbles}>
                  {bubbles.map(point => <Cell key={point.name} fill={point.color} fillOpacity={0.78} stroke={point.color} cursor="pointer" onClick={() => openBreakdown(point.breakdown)} />)}
                </Scatter>
              </ScatterChart>
            </ResponsiveContainer>
          ) : <Box sx={{ height: '100%', display: 'grid', placeItems: 'center' }}><Typography color="text.secondary">Недостаточно сопоставимого факта для выбранного среза.</Typography></Box>}
        </Box>
      </Paper>
    </>
  );
}

function calendarValue(point: PromoDashboardCalendarPoint | undefined, metric: CalendarMetric): number | null {
  if (!point) return null;
  if (metric === 'count') return point.metrics.promoCount;
  if (metric === 'investments') return point.metrics.planInvestmentsRub;
  if (metric === 'completion') return point.metrics.salesCompletionPct;
  return point.metrics.actualRoi;
}

function calendarLabel(value: number | null, metric: CalendarMetric) {
  if (value == null) return '—';
  if (metric === 'count') return fullNumber(value);
  if (metric === 'investments') return compactNumber(value);
  return percentLabel(value, 0);
}

function calendarColor(value: number | null, metric: CalendarMetric, maxValue: number) {
  if (value == null) return { bgcolor: '#f3f5f8', color: '#9aa4b2' };
  if (metric === 'completion') {
    if (value >= 100) return { bgcolor: '#d7f1e8', color: '#116b54' };
    if (value >= 80) return { bgcolor: '#fff0c7', color: '#8a5a12' };
    return { bgcolor: '#f8d8d4', color: '#9a3d34' };
  }
  if (metric === 'roi') {
    if (value >= 0) return { bgcolor: '#d7f1e8', color: '#116b54' };
    return { bgcolor: '#f8d8d4', color: '#9a3d34' };
  }
  const intensity = maxValue > 0 ? Math.min(1, Math.max(0, value / maxValue)) : 0;
  return { bgcolor: `rgba(99, 102, 241, ${0.08 + intensity * 0.66})`, color: intensity > 0.56 ? '#fff' : '#27304a' };
}

function calendarTooltip(point: PromoDashboardCalendarPoint) {
  const metrics = point.metrics;
  return (
    <Box sx={{ p: 0.25 }}>
      <Typography variant="caption" sx={{ display: 'block', fontWeight: 750 }}>{point.name} · {MONTHS[point.month - 1]} {point.year}</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>Промо: {metrics.promoCount} · факт: {metrics.factReadyCount}</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>Продажи: план {compactNumber(metrics.planUnits)} · факт {compactNumber(metrics.actualUnits)}</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>Инвестиции: план {compactNumber(metrics.planInvestmentsRub)} · факт {compactNumber(metrics.actualInvestmentsRub)}</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>ROI: план {percentLabel(metrics.comparablePlanRoi)} · факт {percentLabel(metrics.actualRoi)}</Typography>
    </Box>
  );
}

function CalendarDashboard({ data, onDrilldown, showValues }: {
  data: PromoDashboardResponse;
  onDrilldown: PromoDashboardProps['onDrilldown'];
  showValues: boolean;
}) {
  const [dimension, setDimension] = useState<CalendarDimension>('network');
  const [metric, setMetric] = useState<CalendarMetric>('count');
  const [calendarYear, setCalendarYear] = useState(new Date().getFullYear());
  const years = data.availableYears;
  const activeYear = years.includes(calendarYear) ? calendarYear : (years.includes(new Date().getFullYear()) ? new Date().getFullYear() : years[years.length - 1]);
  const source = dimension === 'network' ? data.networkCalendar : data.brandCalendar;

  const yearPoints = useMemo(() => source.filter(point => point.year === activeYear), [activeYear, source]);
  const pointMap = useMemo(() => new Map(yearPoints.map(point => [`${point.name}|${point.month}`, point])), [yearPoints]);
  const names = useMemo(() => {
    const totals = new Map<string, number>();
    yearPoints.forEach(point => totals.set(point.name, (totals.get(point.name) || 0) + point.metrics.planInvestmentsRub));
    return [...totals.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0], 'ru')).map(([name]) => name);
  }, [yearPoints]);
  const maxValue = useMemo(() => Math.max(0, ...yearPoints.map(point => calendarValue(point, metric) || 0)), [metric, yearPoints]);
  const yearTrend = useMemo<TrendRow[]>(() => data.trend.filter(point => point.year === activeYear).map(point => ({
    period: MONTHS[point.month - 1], year: point.year, month: point.month,
    planUnits: point.metrics.planUnits, actualUnits: point.metrics.actualUnits,
    planInvestments: point.metrics.planInvestmentsRub, actualInvestments: point.metrics.actualInvestmentsRub,
    planRoi: point.metrics.comparablePlanRoi, actualRoi: point.metrics.actualRoi,
    planUplift: point.metrics.planUpliftUnits, actualUplift: point.metrics.actualUpliftUnits,
    coverage: point.metrics.factCoveragePct,
  })), [activeYear, data.trend]);

  const openCell = (point: PromoDashboardCalendarPoint) => {
    if (point.name === 'Не указано') return;
    onDrilldown({
      yearFrom: point.year,
      yearTo: point.year,
      months: [point.month],
      [dimension === 'network' ? 'network_name' : 'brand']: [point.name],
    });
  };

  return (
    <>
      <Paper variant="outlined" sx={{ borderRadius: 3, borderColor: '#dfe5ee', overflow: 'hidden', mb: 1.25 }}>
        <Stack direction={{ xs: 'column', lg: 'row' }} spacing={1} sx={{ px: 1.6, py: 1.25, justifyContent: 'space-between', alignItems: { xs: 'stretch', lg: 'center' }, borderBottom: '1px solid #e7ebf1' }}>
          <Box><Typography variant="subtitle1" sx={{ fontWeight: 750 }}>Промо-календарь</Typography><Typography variant="caption" color="text.secondary">Строки отсортированы по плановым инвестициям; нажмите на ячейку для детализации.</Typography></Box>
          <Stack direction="row" spacing={0.75} useFlexGap sx={{ flexWrap: 'wrap' }}>
            <ToggleButtonGroup size="small" exclusive value={dimension} onChange={(_, value) => value && setDimension(value)}><ToggleButton value="network">Сети</ToggleButton><ToggleButton value="brand">Бренды</ToggleButton></ToggleButtonGroup>
            <ToggleButtonGroup size="small" exclusive value={metric} onChange={(_, value) => value && setMetric(value)}><ToggleButton value="count">Промо</ToggleButton><ToggleButton value="investments">Инвестиции</ToggleButton><ToggleButton value="completion">Выполнение</ToggleButton><ToggleButton value="roi">ROI</ToggleButton></ToggleButtonGroup>
            <Select size="small" value={activeYear || ''} onChange={(event) => setCalendarYear(Number(event.target.value))} sx={{ minWidth: 92 }}>
              {years.map(year => <MenuItem key={year} value={year}>{year}</MenuItem>)}
            </Select>
          </Stack>
        </Stack>
        {names.length > 0 ? (
          <TableContainer sx={{ maxHeight: 560 }}>
            <Table stickyHeader size="small" sx={{ minWidth: 1040, tableLayout: 'fixed' }}>
              <TableHead><TableRow><TableCell sx={{ width: 210, fontWeight: 750 }}>{dimension === 'network' ? 'Сеть' : 'Бренд'}</TableCell>{MONTHS.map(month => <TableCell key={month} align="center" sx={{ fontWeight: 750 }}>{month}</TableCell>)}</TableRow></TableHead>
              <TableBody>{names.map(name => (
                <TableRow key={name} hover>
                  <TableCell sx={{ fontWeight: 650, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{name}</TableCell>
                  {MONTHS.map((month, index) => {
                    const point = pointMap.get(`${name}|${index + 1}`);
                    const value = calendarValue(point, metric);
                    const color = calendarColor(value, metric, maxValue);
                    return (
                      <TableCell key={month} align="center" sx={{ p: 0.45 }}>
                        {point ? (
                          <MuiTooltip arrow title={calendarTooltip(point)}>
                            <Box component="button" type="button" onClick={() => openCell(point)} sx={{ width: '100%', minHeight: 34, px: 0.4, border: 0, borderRadius: 1, cursor: point.name === 'Не указано' ? 'default' : 'pointer', font: 'inherit', fontSize: 12, fontWeight: 750, ...color }}>
                              {calendarLabel(value, metric)}
                            </Box>
                          </MuiTooltip>
                        ) : <Typography variant="caption" color="text.disabled">—</Typography>}
                      </TableCell>
                    );
                  })}
                </TableRow>
              ))}</TableBody>
            </Table>
          </TableContainer>
        ) : <Box sx={{ p: 5, textAlign: 'center' }}><Typography color="text.secondary">Для выбранного года данных нет.</Typography></Box>}
      </Paper>

      <Grid container spacing={1.25}>
        <Grid size={{ xs: 12, xl: 7 }}>
          <ChartPaper title={`Сезонность uplift · ${activeYear || '—'}`} subtitle="План содержит все промо месяца; факт отсутствует, если сопоставимый срез не заполнен">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={yearTrend} margin={{ top: showValues ? 20 : 10, right: 12, left: 4, bottom: 0 }}>
                <CartesianGrid stroke="#e9edf2" strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="period" tick={{ fontSize: 10 }} />
                <YAxis tickFormatter={compactNumber} tick={{ fontSize: 10 }} width={60} />
                <Tooltip itemSorter={planFirst} formatter={(value, name) => [`${fullNumber(Number(value))} уп.`, name === 'planUplift' ? 'План uplift' : 'Факт uplift']} />
                <Legend itemSorter={planFirst} formatter={(value) => value === 'planUplift' ? 'План uplift' : 'Факт uplift'} />
                <Bar dataKey="planUplift" fill={SERIES_PLAN} opacity={0.72} radius={[3, 3, 0, 0]} isAnimationActive={false}>
                  {showValues && <LabelList dataKey="planUplift" position="top" formatter={(value) => labelText(value, compactNumber)} style={BAR_LABEL_STYLE} />}
                </Bar>
                <Bar dataKey="actualUplift" fill={SERIES_FACT} radius={[3, 3, 0, 0]} isAnimationActive={false}>
                  {showValues && <LabelList dataKey="actualUplift" position="top" formatter={(value) => labelText(value, compactNumber)} style={BAR_LABEL_STYLE} />}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </ChartPaper>
        </Grid>
        <Grid size={{ xs: 12, xl: 5 }}>
          <ChartPaper title={`Сезонность ROI · ${activeYear || '—'}`} subtitle="Weighted ROI на сопоставимом фактическом срезе">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={yearTrend} margin={{ top: showValues ? 20 : 10, right: 16, left: 6, bottom: 0 }}>
                <CartesianGrid stroke="#e9edf2" strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="period" tick={{ fontSize: 10 }} />
                <YAxis tickFormatter={(value) => `${fullNumber(Number(value))}%`} tick={{ fontSize: 10 }} width={62} />
                <ReferenceLine y={0} stroke={SERIES_NEUTRAL} />
                <Tooltip itemSorter={planFirst} formatter={(value, name) => [percentLabel(Number(value)), name === 'planRoi' ? 'План ROI' : 'Факт ROI']} />
                <Legend itemSorter={planFirst} formatter={(value) => value === 'planRoi' ? 'План ROI' : 'Факт ROI'} />
                <Line dataKey="planRoi" stroke={SERIES_PLAN} strokeWidth={2.4} dot={{ r: 2 }} connectNulls={false} isAnimationActive={false}>
                  {showValues && <LabelList dataKey="planRoi" position="top" offset={8} formatter={(value) => labelText(value, (numeric) => percentLabel(numeric, 0))} style={LINE_LABEL_STYLE} />}
                </Line>
                <Line dataKey="actualRoi" stroke={SERIES_FACT} strokeWidth={2.8} dot={{ r: 2 }} connectNulls={false} isAnimationActive={false}>
                  {showValues && <LabelList dataKey="actualRoi" position="bottom" offset={8} formatter={(value) => labelText(value, (numeric) => percentLabel(numeric, 0))} style={LINE_LABEL_STYLE} />}
                </Line>
              </LineChart>
            </ResponsiveContainer>
          </ChartPaper>
        </Grid>
      </Grid>
    </>
  );
}

export default function PromoDashboard({ data, loading, error, onDrilldown }: PromoDashboardProps) {
  const [view, setView] = useState<DashboardView>('overview');
  // Переключатель живёт здесь, а не внутри вкладок: подписи включают один раз и
  // ждут их на обеих — как в витрине реестра сетей.
  const [showValues, setShowValues] = useState(false);

  if (loading && !data) return <Box sx={{ flex: 1, display: 'grid', placeItems: 'center' }}><CircularProgress /></Box>;
  if (error) return <Alert severity="error">{error}</Alert>;
  if (!data || data.summary.promoCount === 0) return <Paper variant="outlined" sx={{ p: 5, textAlign: 'center', borderRadius: 3 }}><Typography color="text.secondary">По выбранным фильтрам промо не найдены.</Typography></Paper>;

  return (
    <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', pr: 0.5 }}>
      <Stack direction="row" sx={{ alignItems: 'center', justifyContent: 'space-between', mb: 1.25, borderBottom: '1px solid #e3e8ef' }}>
        <Tabs value={view} onChange={(_, value) => setView(value)}>
          <Tab value="overview" label="План–факт и эффективность" />
          <Tab value="calendar" label="Календарь и сезонность" />
        </Tabs>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center', pr: 1 }}>
          <FormControlLabel
            control={<Switch size="small" checked={showValues} onChange={(event) => setShowValues(event.target.checked)} />}
            label={<Typography variant="body2">Значения на графике</Typography>}
          />
          {loading && <Stack direction="row" spacing={0.75} sx={{ alignItems: 'center' }}><CircularProgress size={15} /><Typography variant="caption" color="text.secondary">Обновление</Typography></Stack>}
        </Stack>
      </Stack>
      {view === 'overview'
        ? <OverviewDashboard data={data} onDrilldown={onDrilldown} showValues={showValues} />
        : <CalendarDashboard data={data} onDrilldown={onDrilldown} showValues={showValues} />}
    </Box>
  );
}
