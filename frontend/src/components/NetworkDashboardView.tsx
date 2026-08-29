// Витрина реестра сетей: итоги периода, сравнение с прошлым годом и риски.
//
// Компонент только рисует. Все суммы приходят посчитанными с сервера тем же
// кодом, что считает карточку сети, — здесь ничего не пересчитывается, иначе
// витрина и карточка разошлись бы.

import { useMemo, useState } from 'react';
import {
  Alert,
  Box,
  Chip,
  CircularProgress,
  FormControlLabel,
  Grid,
  Paper,
  Stack,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip as MuiTooltip,
  Typography,
} from '@mui/material';
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  LabelList,
  Line,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { formatRubShort, pluralRu } from '../utils/networkPlan';
import {
  BORDER,
  CHANNEL_COLOR,
  DIMENSION_COLUMN,
  DIMENSION_LABEL,
  GRID,
  INK_MUTED,
  MONTH_LABELS,
  NEUTRAL,
  NUMERIC_CELL,
  POLARITY_NEGATIVE,
  POLARITY_POSITIVE,
  QUARTERS,
  SERIES_EAC,
  SERIES_FACT,
  SERIES_PLAN,
  SERIES_PREV,
  amount,
  amountFull,
  completionColor,
  completionOf,
  eacCompletionOf,
  growthLabel,
  growthOf,
  metricEAC,
  metricFact,
  metricPlan,
  metricGap,
  metricPrevFact,
  pctLabel,
  signedAmount,
  signedShort,
} from '../utils/networkDashboard';
import type { Dimension, Grain, Unit } from '../utils/networkDashboard';
import { ChartPaper, KpiCard, PromoTags, SeriesLegend } from './NetworkDashboardParts';
import type {
  NetworkDashboardBreakdown,
  NetworkDashboardMetrics,
  NetworkDashboardMonthPoint,
  NetworkDashboardPromoTag,
  NetworkDashboardResponse,
} from '../types/network';

interface NetworkDashboardViewProps {
  data: NetworkDashboardResponse | null;
  loading: boolean;
  error: string | null;
  onOpenNetwork: (networkId: number) => void;
}

// TrendPoint — точка тренда любой гранулярности. Метрики есть только у
// квартала: на месяце реальны не все величины, и подсказка это учитывает.
interface TrendPoint {
  key: string;
  label: string;
  // tick — подпись на оси. Хранится в самой точке, а не выводится из данных:
  // у пропущенного месяца данных нет, и вывести её оттуда нельзя.
  tick: string;
  // null означает пропуск: месяц не входит в выбранные кварталы. Разрыв нужен,
  // чтобы площадь между несмежными кварталами не читалась как непрерывный ряд.
  plan: number | null;
  fact: number | null;
  eac: number | null;
  prevFact: number | null;
  quarter: number;
  metrics: NetworkDashboardMetrics | null;
  month: NetworkDashboardMonthPoint | null;
}

function TrendTooltip({ active, payload, unit }: {
  active?: boolean;
  payload?: Array<{ payload: TrendPoint }>;
  unit: Unit;
}) {
  const point = payload?.[0]?.payload;
  if (!active || !point) return null;
  if (point.plan == null && point.fact == null) {
    return (
      <Paper sx={{ p: 1.25, border: `1px solid ${BORDER}` }}>
        <Typography variant="subtitle2" sx={{ fontWeight: 750 }}>{point.label}</Typography>
        <Typography variant="caption" color="text.secondary">Месяц вне выбранных кварталов</Typography>
      </Paper>
    );
  }
  const metrics = point.metrics;
  const month = point.month;
  return (
    <Paper sx={{ p: 1.25, border: `1px solid ${BORDER}`, maxWidth: 320 }}>
      <Stack direction="row" spacing={0.75} sx={{ alignItems: 'baseline' }}>
        <Typography variant="subtitle2" sx={{ fontWeight: 750 }}>{point.label}</Typography>
        {month && !month.closed && (
          <Typography variant="caption" color="text.secondary">месяц не закрыт</Typography>
        )}
      </Stack>
      <Typography variant="caption" sx={{ display: 'block' }}>План: {amountFull(point.plan ?? 0, unit)}</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>Факт: {amountFull(point.fact ?? 0, unit)}</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>Прогноз итога: {amountFull(point.eac ?? 0, unit)}</Typography>
      {point.prevFact != null && (
        <Typography variant="caption" sx={{ display: 'block' }}>
          Прошлый год: {amountFull(point.prevFact, unit)}
          {metrics ? ` · ${growthLabel(growthOf(metrics, unit))}` : ''}
        </Typography>
      )}
      {metrics && (
        <Typography variant="caption" sx={{ display: 'block', mt: 0.5 }}>
          Выполнение по прогнозу: {pctLabel(eacCompletionOf(metrics, unit))}
        </Typography>
      )}
      {month && (
        <Typography variant="caption" sx={{ display: 'block', mt: 0.5 }}>
          План месяца — квартальный, разложенный по схеме сети
        </Typography>
      )}
      {(metrics?.promoCount ?? month?.promoCount ?? 0) > 0 && (
        <Typography variant="caption" sx={{ display: 'block' }}>
          Промо: {metrics?.promoCount ?? month?.promoCount} · онлайн{' '}
          {metrics?.promoOnlineCount ?? month?.promoOnlineCount} · оффлайн{' '}
          {metrics?.promoOfflineCount ?? month?.promoOfflineCount}
        </Typography>
      )}
      {(metrics?.openCellsWithoutForecast ?? month?.cellsWithoutForecast ?? 0) > 0 && (
        <Typography variant="caption" sx={{ display: 'block', mt: 0.5, color: 'warning.dark' }}>
          Без прогноза: {metrics?.openCellsWithoutForecast ?? month?.cellsWithoutForecast}{' '}
          {pluralRu(metrics?.openCellsWithoutForecast ?? month?.cellsWithoutForecast ?? 0, 'ячейка', 'ячейки', 'ячеек')}
        </Typography>
      )}
    </Paper>
  );
}

function BreakdownTooltip({ active, payload, unit }: {
  active?: boolean;
  payload?: Array<{ payload: { name: string; metrics: NetworkDashboardMetrics } }>;
  unit: Unit;
}) {
  const point = payload?.[0]?.payload;
  if (!active || !point) return null;
  const metrics = point.metrics;
  return (
    <Paper sx={{ p: 1.25, border: `1px solid ${BORDER}`, maxWidth: 340 }}>
      <Typography variant="subtitle2" sx={{ fontWeight: 750 }}>{point.name}</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>
        План {amount(metricPlan(metrics, unit), unit)} · прогноз итога {amount(metricEAC(metrics, unit), unit)}
      </Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>
        Разрыв: {signedAmount(metricGap(metrics, unit), unit)} · выполнение {pctLabel(eacCompletionOf(metrics, unit))}
      </Typography>
      {metrics.prevFactRub != null && (
        <Typography variant="caption" sx={{ display: 'block' }}>
          К прошлому году: {growthLabel(growthOf(metrics, unit))}
        </Typography>
      )}
      <Typography variant="caption" sx={{ display: 'block' }}>
        Инвестиции без НДС: план {formatRubShort(metrics.planInvestmentsRubNet)} · прогноз{' '}
        {formatRubShort(metrics.eacInvestmentsRubNet)} ₽
      </Typography>
    </Paper>
  );
}

export default function NetworkDashboardView({
  data, loading, error, onOpenNetwork,
}: NetworkDashboardViewProps) {
  // null — разрез не выбирали руками: тогда он следует из области. На одной
  // сети «Сети» и «КАМы» вырождаются в строку самой себя, поэтому там разрез
  // открывается брендами — единственным содержательным внутри сети.
  const [dimensionChoice, setDimensionChoice] = useState<Dimension | null>(null);
  const [unit, setUnit] = useState<Unit>('rub');
  // null — гранулярность не выбирали руками: тогда она следует из ширины
  // периода. На одном квартале четыре точки вырождаются в одну, поэтому
  // узкий период открывается сразу помесячно.
  const [grainChoice, setGrainChoice] = useState<Grain | null>(null);
  const [showValues, setShowValues] = useState(false);
  const [showPromo, setShowPromo] = useState(true);

  const singleNetwork = data?.networks.length === 1;
  const dimension: Dimension = dimensionChoice ?? (singleNetwork ? 'brands' : 'networks');

  const breakdown: NetworkDashboardBreakdown[] = useMemo(() => {
    if (!data) return [];
    if (dimension === 'brands') return data.brands;
    if (dimension === 'kams') return data.kams;
    return data.networks;
  }, [data, dimension]);

  // Крупнейшие отклонения считаются в выбранной единице целиком: сеть, которая
  // просела в рублях из-за цены, и сеть, недогрузившая упаковки, — разные
  // сети, и отбирать их по рублёвому разрыву в режиме упаковок нельзя.
  const drivers = useMemo(() => {
    return [...breakdown]
      .map((item) => ({ name: item.name, value: metricGap(item.metrics, unit), metrics: item.metrics, item }))
      .sort((a, b) => Math.abs(b.value) - Math.abs(a.value))
      .slice(0, 12)
      .reverse();
  }, [breakdown, unit]);

  const quarterCount = data?.selectedQuarters.length ?? 4;
  const grain: Grain = grainChoice ?? (quarterCount > 1 ? 'quarter' : 'month');

  const trendSeries = useMemo<TrendPoint[]>(() => {
    if (!data) return [];
    if (grain === 'month') {
      const byMonth = new Map(data.months.map((point) => [point.month, point]));
      const present = data.months.map((point) => point.month);
      if (present.length === 0) return [];
      // Пропущенные месяцы остаются на оси пустыми точками: без них
      // март и июль встали бы рядом, и провал между кварталами исчез бы.
      const series: TrendPoint[] = [];
      for (let month = present[0]; month <= present[present.length - 1]; month += 1) {
        const point = byMonth.get(month);
        series.push({
          key: `m${month}`,
          label: `${MONTH_LABELS[month - 1]} ${data.year}`,
          tick: MONTH_LABELS[month - 1],
          plan: point ? (unit === 'rub' ? point.planRub : point.planUnits) : null,
          fact: point ? (unit === 'rub' ? point.factRub : point.factUnits) : null,
          eac: point ? (unit === 'rub' ? point.eacRub : point.eacUnits) : null,
          prevFact: point ? (unit === 'rub' ? point.prevFactRub : point.prevFactUnits) : null,
          quarter: Math.floor((month - 1) / 3) + 1,
          metrics: null,
          month: point ?? null,
        });
      }
      return series;
    }
    return data.quarters.map((point) => ({
      key: `q${point.quarter}`,
      label: `Q${point.quarter} ${point.year}`,
      tick: `Q${point.quarter}`,
      plan: metricPlan(point.metrics, unit),
      fact: metricFact(point.metrics, unit),
      eac: metricEAC(point.metrics, unit),
      prevFact: metricPrevFact(point.metrics, unit),
      quarter: point.quarter,
      metrics: point.metrics,
      month: null,
    }));
  }, [data, grain, unit]);

  const heatmapCells = useMemo(() => {
    const cells = new Map<string, { metrics: NetworkDashboardMetrics; tags: NetworkDashboardPromoTag[] }>();
    (data?.networkQuarters ?? []).forEach((cell) => {
      cells.set(`${cell.networkId}|${cell.quarter}`, { metrics: cell.metrics, tags: cell.promoTags ?? [] });
    });
    return cells;
  }, [data]);

  if (loading && !data) {
    return <Box sx={{ flex: 1, display: 'grid', placeItems: 'center', minHeight: 320 }}><CircularProgress /></Box>;
  }
  if (error) return <Alert severity="error">{error}</Alert>;
  if (!data || data.summary.networkCount === 0) {
    return (
      <Paper variant="outlined" sx={{ p: 5, textAlign: 'center', borderRadius: 3, borderColor: BORDER }}>
        <Typography color="text.secondary">За выбранный период планов по доступным сетям нет.</Typography>
      </Paper>
    );
  }

  const summary = data.summary;
  const investmentOverrun = summary.investmentVarianceRub > 0;
  // Инвестиции, недополученные из-за невыполнения плана: разница между тем,
  // что дал бы процент, и тем, что осталось после порога. Это не экономия,
  // поэтому и подписывается иначе.
  const missedInvestments = Math.max(0, summary.planInvestmentsRubNet - summary.eacInvestmentsRubNet);
  const numbersOf = (pick: (point: TrendPoint) => number | null) =>
    trendSeries.map(pick).filter((value): value is number => value != null);
  const factTrend = numbersOf((point) => point.fact);
  const eacTrend = numbersOf((point) => point.eac);
  const planTrend = numbersOf((point) => point.plan);
  const hasPrev = trendSeries.some((point) => point.prevFact != null);

  return (
    <Box sx={{ flex: 1, minHeight: 0 }}>
      <Stack
        direction="row"
        spacing={1}
        useFlexGap
        sx={{ flexWrap: 'wrap', alignItems: 'center', mb: 1.25 }}
      >
        <ToggleButtonGroup size="small" exclusive value={unit} onChange={(_, value) => value && setUnit(value)}>
          <ToggleButton value="rub">Рубли</ToggleButton>
          <ToggleButton value="units">Упаковки</ToggleButton>
        </ToggleButtonGroup>
        <FormControlLabel
          control={<Switch size="small" checked={showValues} onChange={(event) => setShowValues(event.target.checked)} />}
          label={<Typography variant="body2">Значения на графике</Typography>}
        />
        <FormControlLabel
          control={<Switch size="small" checked={showPromo} onChange={(event) => setShowPromo(event.target.checked)} />}
          label={<Typography variant="body2">Метки промо</Typography>}
        />
      </Stack>

      <Grid container spacing={1.25} sx={{ mb: 1.25 }}>
        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2.4 }}>
          <KpiCard
            label="Обязательство по контракту"
            primary={amount(metricPlan(summary, unit), unit)}
            secondary={`${summary.networkCount} ${pluralRu(summary.networkCount, 'сеть', 'сети', 'сетей')} · ${summary.brandCount} ${pluralRu(summary.brandCount, 'бренд', 'бренда', 'брендов')}`}
            hint="Валовый пул считается целиком"
            accent={SERIES_PLAN}
            trend={planTrend}
            growth={summary.planYoyPct}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2.4 }}>
          <KpiCard
            label="Факт отгрузок"
            primary={amount(metricFact(summary, unit), unit)}
            secondary={`Выполнение ${pctLabel(completionOf(summary, unit))}`}
            hint={metricPrevFact(summary, unit) != null
              ? `Прошлый год ${amount(metricPrevFact(summary, unit) as number, unit)}`
              : 'Сопоставимого периода прошлого года нет'}
            accent={SERIES_FACT}
            trend={factTrend}
            growth={growthOf(summary, unit)}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2.4 }}>
          <KpiCard
            label="Прогноз итога периода"
            primary={amount(metricEAC(summary, unit), unit)}
            secondary={`Разрыв ${signedAmount(metricGap(summary, unit), unit)} · ${pctLabel(eacCompletionOf(summary, unit))}`}
            hint="Факт закрытых месяцев плюс прогноз открытых"
            accent={SERIES_EAC}
            trend={eacTrend}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2.4 }}>
          <KpiCard
            label="Инвестиции без НДС"
            primary={`${formatRubShort(summary.eacInvestmentsRubNet)} ₽`}
            secondary={`План ${formatRubShort(summary.planInvestmentsRubNet)} · ${signedShort(summary.investmentVarianceRub)} ₽`}
            hint={missedInvestments > 0
              ? `Недобор ${formatRubShort(missedInvestments)} ₽: план выполнен не всеми брендами`
              : `Ставка к объёму ${pctLabel(summary.effectiveInvestmentsPct)}${investmentOverrun ? ' · перерасход' : ''}`}
            accent={investmentOverrun ? POLARITY_NEGATIVE : NEUTRAL}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2.4 }}>
          <KpiCard
            label="Промо в периоде"
            primary={`${summary.promoCount}`}
            secondary={`Онлайн ${summary.promoOnlineCount} · оффлайн ${summary.promoOfflineCount}`}
            hint={`Инвестиции в промо ${formatRubShort(summary.promoInvestmentsRub)} ₽`}
            accent={CHANNEL_COLOR['онлайн']}
          />
        </Grid>
      </Grid>

      {summary.openCellsWithoutForecast > 0 && (
        <Alert severity="warning" sx={{ mb: 1.25, borderRadius: 3 }}>
          В {summary.openCellsWithoutForecast} {pluralRu(summary.openCellsWithoutForecast, 'ячейке', 'ячейках', 'ячейках')}
          {' '}открытых месяцев нет официального прогноза. В прогноз итога для них включена системная рекомендация;
          заполните официальный прогноз, чтобы зафиксировать ожидание КАМа. Покрытие фактом закрытых месяцев — {pctLabel(summary.factCoveragePct)}.
        </Alert>
      )}

      <Grid container spacing={1.25} sx={{ mb: 1.25 }}>
        <Grid size={{ xs: 12, xl: 7 }}>
          <ChartPaper
            title={grain === 'month' ? 'Динамика по месяцам' : 'Динамика по кварталам'}
            subtitle={[
              hasPrev
                ? 'Факт и прогноз итога залиты, план и прошлый год — линии сравнения'
                : 'Факт и прогноз итога залиты, план — линия сравнения',
              grain === 'month' ? 'План месяца разложен из квартального по схеме сети' : '',
            ].filter(Boolean).join('. ')}
            action={
              <ToggleButtonGroup
                size="small"
                exclusive
                value={grain}
                onChange={(_, value) => value && setGrainChoice(value as Grain)}
              >
                <ToggleButton value="quarter">Кварталы</ToggleButton>
                <ToggleButton value="month">Месяцы</ToggleButton>
              </ToggleButtonGroup>
            }
            legend={
              <SeriesLegend
                items={[
                  { label: 'Факт', color: SERIES_FACT },
                  { label: 'Прогноз итога', color: SERIES_EAC },
                  { label: 'План', color: SERIES_PLAN, dashed: true },
                  ...(hasPrev ? [{ label: 'Прошлый год', color: SERIES_PREV, dashed: true }] : []),
                ]}
              />
            }
          >
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={trendSeries} margin={{ top: 16, right: 16, left: 4, bottom: 0 }}>
                <defs>
                  <linearGradient id="areaFact" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={SERIES_FACT} stopOpacity={0.38} />
                    <stop offset="100%" stopColor={SERIES_FACT} stopOpacity={0.03} />
                  </linearGradient>
                  <linearGradient id="areaEac" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={SERIES_EAC} stopOpacity={0.28} />
                    <stop offset="100%" stopColor={SERIES_EAC} stopOpacity={0.02} />
                  </linearGradient>
                </defs>
                <CartesianGrid stroke={GRID} vertical={false} />
                <XAxis
                  dataKey="key"
                  tickFormatter={(value) => trendSeries.find((item) => item.key === value)?.tick ?? String(value)}
                  tick={{ fontSize: 11 }}
                  stroke={NEUTRAL}
                />
                <YAxis tickFormatter={(value) => formatRubShort(Number(value))} tick={{ fontSize: 11 }} width={72} stroke={NEUTRAL} />
                <Tooltip content={<TrendTooltip unit={unit} />} cursor={{ stroke: NEUTRAL, strokeWidth: 1 }} />
                <Area
                  dataKey="eac" stroke={SERIES_EAC} strokeWidth={2} fill="url(#areaEac)"
                  isAnimationActive={false} connectNulls={false}
                  dot={{ r: 3, strokeWidth: 0, fill: SERIES_EAC }}
                >
                  {showValues && (
                    <LabelList
                      dataKey="eac" position="top" offset={10}
                      formatter={(value) => formatRubShort(Number(value))}
                      style={{ fontSize: 10, fontWeight: 700, fill: INK_MUTED }}
                    />
                  )}
                </Area>
                <Area
                  dataKey="fact" stroke={SERIES_FACT} strokeWidth={2.4} fill="url(#areaFact)"
                  isAnimationActive={false} connectNulls={false}
                  dot={{ r: 3, strokeWidth: 0, fill: SERIES_FACT }}
                />
                <Line
                  dataKey="plan" stroke={SERIES_PLAN} strokeWidth={2} strokeDasharray="5 4"
                  dot={false} isAnimationActive={false} connectNulls={false}
                />
                {hasPrev && (
                  <Line
                    dataKey="prevFact" stroke={SERIES_PREV} strokeWidth={2} strokeDasharray="3 3"
                    dot={false} isAnimationActive={false} connectNulls={false}
                  />
                )}
              </AreaChart>
            </ResponsiveContainer>
          </ChartPaper>
        </Grid>

        <Grid size={{ xs: 12, xl: 5 }}>
          <ChartPaper
            title="Крупнейшие отклонения от плана"
            subtitle="Разрыв между прогнозом итога и обязательством. Нажмите на столбец сети, чтобы открыть её карточку."
            height={340}
            action={
              <ToggleButtonGroup size="small" exclusive value={dimension} onChange={(_, value) => value && setDimensionChoice(value)}>
                {(Object.keys(DIMENSION_LABEL) as Dimension[]).map((key) => (
                  <ToggleButton key={key} value={key}>{DIMENSION_LABEL[key]}</ToggleButton>
                ))}
              </ToggleButtonGroup>
            }
          >
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={drivers} layout="vertical" margin={{ top: 8, right: 44, left: 8, bottom: 0 }}>
                <CartesianGrid stroke={GRID} horizontal={false} />
                <XAxis type="number" tickFormatter={(value) => formatRubShort(Number(value))} tick={{ fontSize: 11 }} stroke={NEUTRAL} />
                <YAxis
                  type="category"
                  dataKey="name"
                  width={176}
                  interval={0}
                  tick={{ fontSize: 10 }}
                  stroke={NEUTRAL}
                  tickFormatter={(value) => String(value).length > 24 ? `${String(value).slice(0, 22)}…` : String(value)}
                />
                <ReferenceLine x={0} stroke={NEUTRAL} />
                <Tooltip content={<BreakdownTooltip unit={unit} />} cursor={{ fill: 'rgba(99,102,241,0.06)' }} />
                <Bar dataKey="value" radius={[0, 4, 4, 0]} isAnimationActive={false}>
                  {showValues && (
                    <LabelList
                      dataKey="value" position="right"
                      formatter={(value) => signedShort(Number(value))}
                      style={{ fontSize: 9, fontWeight: 700, fill: INK_MUTED }}
                    />
                  )}
                  {drivers.map((point) => (
                    <Cell
                      key={point.name}
                      fill={point.value >= 0 ? POLARITY_POSITIVE : POLARITY_NEGATIVE}
                      cursor={point.item.networkId != null ? 'pointer' : 'default'}
                      onClick={() => point.item.networkId != null && onOpenNetwork(point.item.networkId)}
                    />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </ChartPaper>
        </Grid>
      </Grid>

      <Paper variant="outlined" sx={{ borderRadius: 3, borderColor: BORDER, overflow: 'hidden', mb: 1.25 }}>
        <Box sx={{ px: 1.6, py: 1.25, borderBottom: `1px solid ${BORDER}` }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 750 }}>Выполнение плана по кварталам</Typography>
          <Typography variant="caption" color="text.secondary">
            Процент прогноза итога к обязательству. Строки отсортированы по размеру плана; нажмите на строку, чтобы открыть карточку.
            {showPromo && ' Под процентом — метки проведённых промо: цвет означает канал.'}
          </Typography>
        </Box>
        <TableContainer sx={{ maxHeight: 520 }}>
          <Table stickyHeader size="small" sx={{ minWidth: 820, tableLayout: 'fixed' }}>
            <TableHead>
              <TableRow>
                <TableCell sx={{ width: 260, fontWeight: 750 }}>Сеть</TableCell>
                {QUARTERS.map((quarter) => (
                  <TableCell key={quarter} align="center" sx={{ fontWeight: 750 }}>Q{quarter}</TableCell>
                ))}
                <TableCell align="right" sx={{ width: 130, fontWeight: 750 }}>План за период</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {data.networks.map((row) => {
                const networkId = row.networkId;
                return (
                  <TableRow key={row.name} hover>
                    <TableCell
                      sx={{
                        fontWeight: 650, overflow: 'hidden', textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap', cursor: networkId ? 'pointer' : 'default',
                      }}
                      onClick={() => networkId && onOpenNetwork(networkId)}
                    >
                      {row.name}
                    </TableCell>
                    {QUARTERS.map((quarter) => {
                      const cell = heatmapCells.get(`${networkId}|${quarter}`);
                      const value = cell ? eacCompletionOf(cell.metrics, unit) : null;
                      const color = completionColor(value);
                      return (
                        <TableCell key={quarter} align="center" sx={{ p: 0.45, verticalAlign: 'top' }}>
                          {cell ? (
                            <>
                              <MuiTooltip
                                arrow
                                title={
                                  <Box sx={{ p: 0.25 }}>
                                    <Typography variant="caption" sx={{ display: 'block', fontWeight: 750 }}>{row.name} · Q{quarter}</Typography>
                                    <Typography variant="caption" sx={{ display: 'block' }}>План: {amountFull(metricPlan(cell.metrics, unit), unit)}</Typography>
                                    <Typography variant="caption" sx={{ display: 'block' }}>Факт: {amountFull(metricFact(cell.metrics, unit), unit)}</Typography>
                                    <Typography variant="caption" sx={{ display: 'block' }}>Прогноз итога: {amountFull(metricEAC(cell.metrics, unit), unit)}</Typography>
                                    {cell.metrics.prevFactRub != null && (
                                      <Typography variant="caption" sx={{ display: 'block' }}>
                                        Прошлый год: {amountFull(metricPrevFact(cell.metrics, unit) ?? 0, unit)} · {growthLabel(growthOf(cell.metrics, unit))}
                                      </Typography>
                                    )}
                                    <Typography variant="caption" sx={{ display: 'block' }}>Разрыв: {signedAmount(metricGap(cell.metrics, unit), unit)}</Typography>
                                    {cell.metrics.openCellsWithoutForecast > 0 && (
                                      <Typography variant="caption" sx={{ display: 'block' }}>
                                        Без прогноза: {cell.metrics.openCellsWithoutForecast}
                                      </Typography>
                                    )}
                                  </Box>
                                }
                              >
                                <Box
                                  component="button"
                                  type="button"
                                  onClick={() => networkId && onOpenNetwork(networkId)}
                                  sx={{
                                    width: '100%', minHeight: 30, px: 0.4, border: 0, borderRadius: 1,
                                    cursor: networkId ? 'pointer' : 'default', font: 'inherit',
                                    fontSize: 12, fontWeight: 750, ...color,
                                  }}
                                >
                                  {value == null ? '—' : `${Math.round(value)}%`}
                                </Box>
                              </MuiTooltip>
                              {showPromo && <PromoTags tags={cell.tags} />}
                            </>
                          ) : <Typography variant="caption" color="text.disabled">—</Typography>}
                        </TableCell>
                      );
                    })}
                    <TableCell align="right" sx={{ ...NUMERIC_CELL, fontWeight: 650 }}>
                      {amount(metricPlan(row.metrics, unit), unit)}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>

      <Paper variant="outlined" sx={{ borderRadius: 3, borderColor: BORDER, overflow: 'hidden' }}>
        <Stack
          direction={{ xs: 'column', md: 'row' }}
          spacing={1}
          sx={{ px: 1.6, py: 1.25, justifyContent: 'space-between', alignItems: { xs: 'stretch', md: 'center' }, borderBottom: `1px solid ${BORDER}` }}
        >
          <Box>
            <Typography variant="subtitle1" sx={{ fontWeight: 750 }}>Разрез периода</Typography>
            <Typography variant="caption" color="text.secondary">
              Инвестиции показаны в базе без НДС: сети работают с разными ставками, в валовой базе их суммы несопоставимы.
            </Typography>
          </Box>
          <ToggleButtonGroup size="small" exclusive value={dimension} onChange={(_, value) => value && setDimensionChoice(value)}>
            {(Object.keys(DIMENSION_LABEL) as Dimension[]).map((key) => (
              <ToggleButton key={key} value={key}>{DIMENSION_LABEL[key]}</ToggleButton>
            ))}
          </ToggleButtonGroup>
        </Stack>
        <TableContainer sx={{ maxHeight: 520 }}>
          <Table stickyHeader size="small" sx={{ minWidth: 1280 }}>
            <TableHead>
              <TableRow>
                <TableCell sx={{ fontWeight: 750 }}>{DIMENSION_COLUMN[dimension]}</TableCell>
                <TableCell align="right" sx={{ fontWeight: 750 }}>План</TableCell>
                <TableCell align="right" sx={{ fontWeight: 750 }}>Факт</TableCell>
                <TableCell align="right" sx={{ fontWeight: 750 }}>Прошлый год</TableCell>
                <TableCell align="right" sx={{ fontWeight: 750 }}>Прирост</TableCell>
                <TableCell align="right" sx={{ fontWeight: 750 }}>Прогноз итога</TableCell>
                <TableCell align="right" sx={{ fontWeight: 750 }}>Выполнение</TableCell>
                <TableCell align="right" sx={{ fontWeight: 750 }}>Разрыв</TableCell>
                <TableCell align="right" sx={{ fontWeight: 750 }}>Инвест. план</TableCell>
                <TableCell align="right" sx={{ fontWeight: 750 }}>Инвест. прогноз</TableCell>
                <TableCell align="right" sx={{ fontWeight: 750 }}>Отклонение</TableCell>
                <TableCell align="right" sx={{ fontWeight: 750 }}>Промо</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {breakdown.map((item) => {
                const prevFact = metricPrevFact(item.metrics, unit);
                return (
                  <TableRow
                    key={item.name}
                    hover
                    sx={{ cursor: item.networkId != null ? 'pointer' : 'default' }}
                    onClick={() => item.networkId != null && onOpenNetwork(item.networkId)}
                  >
                    <TableCell sx={{ fontWeight: 650 }}>
                      {item.name}
                      {item.kam && dimension === 'networks' && (
                        <Chip size="small" label={item.kam} variant="outlined" sx={{ ml: 1, height: 20 }} />
                      )}
                    </TableCell>
                    <TableCell align="right" sx={NUMERIC_CELL}>{formatRubShort(metricPlan(item.metrics, unit))}</TableCell>
                    <TableCell align="right" sx={NUMERIC_CELL}>{formatRubShort(metricFact(item.metrics, unit))}</TableCell>
                    <TableCell align="right" sx={NUMERIC_CELL}>{prevFact == null ? '—' : formatRubShort(prevFact)}</TableCell>
                    <TableCell
                      align="right"
                      sx={{
                        ...NUMERIC_CELL, fontWeight: 700,
                        color: growthOf(item.metrics, unit) == null
                          ? 'text.disabled'
                          : (growthOf(item.metrics, unit) as number) >= 0 ? POLARITY_POSITIVE : POLARITY_NEGATIVE,
                      }}
                    >
                      {growthLabel(growthOf(item.metrics, unit))}
                    </TableCell>
                    <TableCell align="right" sx={NUMERIC_CELL}>{formatRubShort(metricEAC(item.metrics, unit))}</TableCell>
                    <TableCell align="right" sx={NUMERIC_CELL}>{pctLabel(eacCompletionOf(item.metrics, unit))}</TableCell>
                    <TableCell
                      align="right"
                      sx={{ ...NUMERIC_CELL, fontWeight: 700, color: metricGap(item.metrics, unit) >= 0 ? POLARITY_POSITIVE : POLARITY_NEGATIVE }}
                    >
                      {signedShort(metricGap(item.metrics, unit))}
                    </TableCell>
                    <TableCell align="right" sx={NUMERIC_CELL}>{formatRubShort(item.metrics.planInvestmentsRubNet)}</TableCell>
                    <TableCell align="right" sx={NUMERIC_CELL}>
                      <MuiTooltip
                        arrow
                        title={item.metrics.eacInvestmentsRubNet < item.metrics.planInvestmentsRubNet
                          ? 'Часть брендов не закрыла план — по ним инвестиций нет'
                          : 'Порог выполнения пройден по всем брендам среза'}
                      >
                        <span>{formatRubShort(item.metrics.eacInvestmentsRubNet)}</span>
                      </MuiTooltip>
                    </TableCell>
                    <TableCell
                      align="right"
                      sx={{ ...NUMERIC_CELL, fontWeight: 700, color: item.metrics.investmentVarianceRub > 0 ? POLARITY_NEGATIVE : 'text.primary' }}
                    >
                      {signedShort(item.metrics.investmentVarianceRub)}
                    </TableCell>
                    <TableCell align="right" sx={NUMERIC_CELL}>
                      {item.metrics.promoCount === 0 ? '—' : (
                        <MuiTooltip
                          arrow
                          title={`Онлайн ${item.metrics.promoOnlineCount} · оффлайн ${item.metrics.promoOfflineCount}`}
                        >
                          <span>{item.metrics.promoCount}</span>
                        </MuiTooltip>
                      )}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>
    </Box>
  );
}
