// Разбор одной сети: тот же ответ витрины, но собранный под один вопрос —
// что происходит внутри сети и за счёт чего.
//
// Компонент только рисует. Считает всё сервер тем же кодом, что и карточку
// сети, поэтому разбор обязан сходиться с ней до копейки.
//
// Порядок полос — по убыванию срочности вопроса: сначала пять чисел «как
// дела», потом «как шли», потом «кто дал отклонение», и лишь затем бренды
// построчно и инвестиции. Плотно, но читается сверху вниз без прыжков.

import { Fragment, useMemo, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  FormControlLabel,
  Grid,
  IconButton,
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
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import OpenInNewIcon from '@mui/icons-material/OpenInNew';
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  ComposedChart,
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
  GRID,
  INK_MUTED,
  NUMERIC_CELL,
  POLARITY_NEGATIVE,
  POLARITY_POSITIVE,
  SERIES_EAC,
  SERIES_FACT,
  SERIES_PLAN,
  SERIES_PREV,
  amount,
  amountFull,
  completionColor,
  eacCompletionOf,
  gapColor,
  growthLabel,
  growthOf,
  metricEAC,
  metricFact,
  metricPlan,
  metricPrevFact,
  pctLabel,
  signedShort,
} from '../utils/networkDashboard';
import type { Unit } from '../utils/networkDashboard';
import { ChartPaper, KpiCard, PromoTags, SeriesLegend } from './NetworkDashboardParts';
import type {
  NetworkDashboardBrandMonth,
  NetworkDashboardBrandQuarter,
  NetworkDashboardBreakdown,
  NetworkDashboardResponse,
  NetworkDashboardSKU,
} from '../types/network';

const MONTH_FULL = [
  'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
];

// Что показывает раскрытие строки бренда. Кварталы — те же строки плана,
// SKU — объяснение снизу, где плановых величин нет.
type DetailKind = 'quarters' | 'skus';

// Подписи значений — как в витрине по всем сетям: приглушённый цвет, чтобы
// читались как разметка, а не спорили с самими рядами. Ряды с подписями не
// анимируются: recharts отдаёт LabelList данные только неанимируемому ряду.
const BAR_LABEL_STYLE = { fontSize: 9, fontWeight: 700, fill: INK_MUTED } as const;

// Пустое значение не подписываем: у закрытого месяца нет прогноза, у открытого
// нет факта, и подпись повисла бы в пустоте.
const labelText = (value: unknown, format: (numeric: number) => string) =>
  value == null || !Number.isFinite(Number(value)) ? '' : format(Number(value));

interface NetworkDetailViewProps {
  data: NetworkDashboardResponse | null;
  loading: boolean;
  error: string | null;
  // Возврат ко всему портфелю: снимает выбор сети, а не уводит со страницы.
  onBackToAll: () => void;
  // КАМ как ступень возврата — только когда фильтр по КАМу вообще задан.
  onBackToKAM?: () => void;
  kamCrumb?: string;
  onOpenCard: (networkId: number) => void;
}

// Точка помесячного ряда. Промо живут здесь же: их всплеск объясняет форму
// ряда, и отдельным блоком связь потерялась бы.
interface DetailMonthPoint {
  key: string;
  tick: string;
  label: string;
  plan: number;
  fact: number | null;
  eac: number | null;
  prevFact: number | null;
  promoCount: number;
  closed: boolean;
  withoutForecast: number;
}

function MonthTooltip({ active, payload, unit }: {
  active?: boolean;
  payload?: Array<{ payload: DetailMonthPoint }>;
  unit: Unit;
}) {
  const point = payload?.[0]?.payload;
  if (!active || !point) return null;
  return (
    <Paper sx={{ p: 1.25, border: `1px solid ${BORDER}`, maxWidth: 300 }}>
      <Typography variant="subtitle2" sx={{ fontWeight: 750 }}>{point.label}</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>
        План: {amountFull(point.plan, unit)}
      </Typography>
      {point.closed ? (
        <Typography variant="caption" sx={{ display: 'block' }}>
          Факт: {point.fact == null ? '—' : amountFull(point.fact, unit)}
        </Typography>
      ) : (
        <Typography variant="caption" sx={{ display: 'block' }}>
          Прогноз итога: {point.eac == null ? '—' : amountFull(point.eac, unit)}
        </Typography>
      )}
      {point.prevFact != null && (
        <Typography variant="caption" sx={{ display: 'block', color: INK_MUTED }}>
          Прошлый год: {amountFull(point.prevFact, unit)}
        </Typography>
      )}
      {point.promoCount > 0 && (
        <Typography variant="caption" sx={{ display: 'block', mt: 0.3 }}>
          {point.promoCount} {pluralRu(point.promoCount, 'промо', 'промо', 'промо')} в месяце
        </Typography>
      )}
      {!point.closed && point.withoutForecast > 0 && (
        <Typography variant="caption" sx={{ display: 'block', color: POLARITY_NEGATIVE }}>
          Без прогноза: {point.withoutForecast} {pluralRu(point.withoutForecast, 'ячейка', 'ячейки', 'ячеек')}
        </Typography>
      )}
    </Paper>
  );
}

function GapTooltip({ active, payload, unit }: {
  active?: boolean;
  payload?: Array<{ payload: { name: string; value: number; item: NetworkDashboardBreakdown } }>;
  unit: Unit;
}) {
  const point = payload?.[0]?.payload;
  if (!active || !point) return null;
  const metrics = point.item.metrics;
  return (
    <Paper sx={{ p: 1.25, border: `1px solid ${BORDER}`, maxWidth: 320 }}>
      <Typography variant="subtitle2" sx={{ fontWeight: 750 }}>{point.name}</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>План: {amountFull(metricPlan(metrics, unit), unit)}</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>Прогноз итога: {amountFull(metricEAC(metrics, unit), unit)}</Typography>
      <Typography variant="caption" sx={{ display: 'block', fontWeight: 700, color: gapColor(point.value) }}>
        Отклонение: {signedShort(point.value)} ₽
      </Typography>
      <Typography variant="caption" sx={{ display: 'block', color: INK_MUTED }}>
        К прошлому году: {growthLabel(growthOf(metrics, unit))}
      </Typography>
    </Paper>
  );
}

interface InvestmentPoint {
  key: string;
  tick: string;
  plan: number;
  eac: number;
  effective: number | null;
  // Плановая ставка квартала. Подпись под столбцом — это ставка, поэтому и
  // сравнивать её надо со ставкой: перерасход в рублях при выросшем объёме
  // ставку не поднимает, и красить по суммам значило бы врать подписи.
  planRate: number | null;
}

// Подпись квартала: номер и эффективная ставка под ним. Красная там, где
// ожидаемые инвестиции выше плановых, — это и есть главный вывод графика,
// и держать его только в подсказке значит прятать.
function InvestmentTick(props: {
  x?: number;
  y?: number;
  payload?: { value?: string };
  points?: InvestmentPoint[];
}) {
  const { x = 0, y = 0, payload, points = [] } = props;
  const label = payload?.value ?? '';
  const point = points.find((item) => item.tick === label);
  const overrun = point?.effective != null && point.planRate != null && point.effective > point.planRate;
  return (
    <g transform={`translate(${x},${y})`}>
      <text textAnchor="middle" y={13} fontSize={11.5} fontWeight={650} fill={INK_MUTED}>{label}</text>
      {point?.effective != null && (
        <text
          textAnchor="middle"
          y={28}
          fontSize={11}
          fontWeight={overrun ? 750 : 600}
          fill={overrun ? POLARITY_NEGATIVE : INK_MUTED}
        >
          {pctLabel(point.effective)}
        </text>
      )}
    </g>
  );
}

function InvestmentTooltip({ active, payload, year }: {
  active?: boolean;
  payload?: Array<{ payload: InvestmentPoint }>;
  year: number;
}) {
  const point = payload?.[0]?.payload;
  if (!active || !point) return null;
  const variance = point.eac - point.plan;
  return (
    <Paper sx={{ p: 1.25, border: `1px solid ${BORDER}`, maxWidth: 280 }}>
      <Typography variant="subtitle2" sx={{ fontWeight: 750 }}>{point.tick} {year}</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>План: {formatRubShort(point.plan)} ₽</Typography>
      <Typography variant="caption" sx={{ display: 'block' }}>Ожидаемые: {formatRubShort(point.eac)} ₽</Typography>
      <Typography variant="caption" sx={{ display: 'block', fontWeight: 700, color: gapColor(-variance) }}>
        {variance >= 0 ? 'Перерасход' : 'Экономия'}: {signedShort(variance)} ₽
      </Typography>
      <Typography variant="caption" sx={{ display: 'block', color: INK_MUTED }}>
        Ставка: {pctLabel(point.effective)} при плановой {pctLabel(point.planRate)}
      </Typography>
    </Paper>
  );
}

// Полоса выполнения в строке бренда: рамка — обязательство, заливка — факт
// закрытых месяцев, штриховка — прогноз открытых. Три сценария различаются
// не только цветом, поэтому строка читается и в чёрно-белой печати.
function BulletBar({ plan, fact, eac, unit }: { plan: number; fact: number; eac: number; unit: Unit }) {
  // Шкала — от обязательства: перевыполнение обязано выходить за рамку, иначе
  // «сделали больше плана» и «сделали ровно план» выглядели бы одинаково.
  const scale = Math.max(plan, eac, fact) || 1;
  const pct = (value: number) => `${Math.min((value / scale) * 100, 100)}%`;
  return (
    <MuiTooltip
      arrow
      title={
        <Box sx={{ p: 0.25 }}>
          <Typography variant="caption" sx={{ display: 'block' }}>План: {amountFull(plan, unit)}</Typography>
          <Typography variant="caption" sx={{ display: 'block' }}>Факт: {amountFull(fact, unit)}</Typography>
          <Typography variant="caption" sx={{ display: 'block' }}>Прогноз итога: {amountFull(eac, unit)}</Typography>
        </Box>
      }
    >
      <Box sx={{ position: 'relative', height: 16, minWidth: 120 }}>
        <Box sx={{
          position: 'absolute', left: 0, top: 0, bottom: 0, width: pct(plan),
          border: `1.5px solid ${SERIES_PLAN}`, borderRadius: 0.5,
        }} />
        <Box sx={{
          position: 'absolute', left: 0, top: 3, bottom: 3, width: pct(eac), borderRadius: 0.5,
          backgroundImage: `repeating-linear-gradient(45deg, ${SERIES_EAC} 0 3px, transparent 3px 6px)`,
          borderTop: `1px solid ${SERIES_EAC}`, borderBottom: `1px solid ${SERIES_EAC}`,
        }} />
        <Box sx={{
          position: 'absolute', left: 0, top: 3, bottom: 3, width: pct(fact),
          bgcolor: SERIES_FACT, borderRadius: 0.5,
        }} />
      </Box>
    </MuiTooltip>
  );
}

export default function NetworkDetailView({
  data, loading, error, onBackToAll, onBackToKAM, kamCrumb, onOpenCard,
}: NetworkDetailViewProps) {
  const [unit, setUnit] = useState<Unit>('rub');
  const [showValues, setShowValues] = useState(false);
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const [detailKind, setDetailKind] = useState<DetailKind>('quarters');

  const network = data?.networks[0] ?? null;

  // У SKU свои поля, а не models.NetworkDashboardMetrics, поэтому величина под
  // единицу измерения выбирается здесь — подделывать метрику ради одного числа
  // значило бы завести вторую арифметику.
  const skuFactOf = (row: NetworkDashboardSKU) => (unit === 'rub' ? row.factRub : row.factUnits);
  const skuEACOf = (row: NetworkDashboardSKU) => (unit === 'rub' ? row.eacRub : row.eacUnits);
  const skuPrevOf = (row: NetworkDashboardSKU) => (unit === 'rub' ? row.prevFactRub : row.prevFactUnits);
  const skuGrowthOf = (row: NetworkDashboardSKU) => {
    if (unit === 'rub') return row.factYoyPct;
    const prev = row.prevFactUnits;
    if (prev == null || prev === 0) return null;
    return Math.round(((row.factUnits - prev) / prev) * 10000) / 100;
  };

  const toggleBrand = (brand: string) => setExpanded((current) => {
    const next = new Set(current);
    if (!next.delete(brand)) next.add(brand);
    return next;
  });

  const months = useMemo<DetailMonthPoint[]>(() => {
    if (!data) return [];
    return data.months.map((point) => ({
      key: `m${point.month}`,
      tick: MONTH_FULL[point.month - 1].slice(0, 3),
      label: `${MONTH_FULL[point.month - 1]} ${point.year}`,
      plan: unit === 'rub' ? point.planRub : point.planUnits,
      fact: point.closed ? (unit === 'rub' ? point.factRub : point.factUnits) : null,
      eac: point.closed ? null : (unit === 'rub' ? point.eacRub : point.eacUnits),
      prevFact: unit === 'rub' ? point.prevFactRub : point.prevFactUnits,
      promoCount: point.promoCount,
      closed: point.closed,
      withoutForecast: point.cellsWithoutForecast,
    }));
  }, [data, unit]);

  // Выполнение нарастающим итогом: доля результата к обязательству на конец
  // каждого месяца. Именно нарастающим, а не помесячно, — карточка показывает
  // выполнение за период целиком, и ряд обязан приходить к той же цифре;
  // помесячный процент рассказывал бы про декабрь, а не про период.
  const completionTrend = useMemo(() => {
    let plan = 0;
    let result = 0;
    const series: number[] = [];
    months.forEach((point) => {
      plan += point.plan;
      result += point.fact ?? point.eac ?? 0;
      // Месяцы без плана процента не дают: делить на ноль нечем, а тянуть
      // предыдущее значение значило бы дорисовать выполнение, которого нет.
      if (plan > 0) series.push((result / plan) * 100);
    });
    return series;
  }, [months]);

  // Отклонение от плана по брендам: разрыв прогноза итога к обязательству.
  // Сортировка по величине, а не по знаку — разговор начинается с крупного,
  // в какую бы сторону он ни ушёл.
  const gaps = useMemo(() => {
    if (!data) return [];
    return [...data.brands]
      .map((item) => ({ name: item.name, value: item.metrics.gapRub, item }))
      .sort((a, b) => Math.abs(b.value) - Math.abs(a.value))
      .slice(0, 14)
      .reverse();
  }, [data]);

  const brands = useMemo(() => {
    if (!data) return [];
    return [...data.brands].sort((a, b) => metricPlan(b.metrics, unit) - metricPlan(a.metrics, unit));
  }, [data, unit]);

  // Кварталы бренда для раскрытия строки. Пустая карта означает, что сервер
  // разрез не прислал, — тогда раскрывать нечего и стрелки не показываются.
  const quartersByBrand = useMemo(() => {
    const map = new Map<string, NetworkDashboardBrandQuarter[]>();
    (data?.brandQuarters ?? []).forEach((row) => {
      const list = map.get(row.brand) ?? [];
      list.push(row);
      map.set(row.brand, list);
    });
    return map;
  }, [data]);

  // Промо-календарь: месяцы выбранных кварталов и бренды строками. Бренд без
  // единого промо остаётся пустой строкой — это тоже ответ, и в списке промо
  // его было бы не видно.
  const calendarMonths = useMemo(() => {
    if (!data) return [];
    return data.selectedQuarters.flatMap((quarter) => [0, 1, 2].map((index) => (quarter - 1) * 3 + 1 + index));
  }, [data]);

  const promoByBrandMonth = useMemo(() => {
    const map = new Map<string, NetworkDashboardBrandMonth>();
    (data?.brandMonths ?? []).forEach((cell) => map.set(`${cell.brand}|${cell.month}`, cell));
    return map;
  }, [data]);

  // Бренд, у которого промо есть, а строки плана нет, обязан попасть в
  // календарь: промо провели, и потерять его здесь значило бы соврать.
  const calendarBrands = useMemo(() => {
    const names = brands.map((item) => item.name);
    const known = new Set(names);
    (data?.brandMonths ?? []).forEach((cell) => {
      if (!known.has(cell.brand)) {
        known.add(cell.brand);
        names.push(cell.brand);
      }
    });
    return names;
  }, [brands, data]);

  const hasPromo = (data?.brandMonths ?? []).length > 0;

  // SKU бренда. Плана на них в реестре нет, поэтому в раскрытии у них пусты
  // именно плановые колонки — это не пропуск данных.
  const skusByBrand = useMemo(() => {
    const map = new Map<string, NetworkDashboardSKU[]>();
    (data?.skus ?? []).forEach((row) => {
      const list = map.get(row.brand) ?? [];
      list.push(row);
      map.set(row.brand, list);
    });
    return map;
  }, [data]);

  const investments = useMemo(() => {
    if (!data) return [];
    return data.quarters.map((point) => ({
      key: `q${point.quarter}`,
      tick: `Q${point.quarter}`,
      plan: point.metrics.planInvestmentsRubNet,
      eac: point.metrics.eacInvestmentsRubNet,
      effective: point.metrics.effectiveInvestmentsPct,
      planRate: point.metrics.planRub > 0
        ? Math.round((point.metrics.planInvestmentsRub / point.metrics.planRub) * 10000) / 100
        : null,
    }));
  }, [data]);

  if (loading && !data) {
    return <Box sx={{ flex: 1, display: 'grid', placeItems: 'center', minHeight: 320 }}><CircularProgress /></Box>;
  }
  if (error) return <Alert severity="error">{error}</Alert>;
  if (!data || !network || data.summary.networkCount === 0) {
    return (
      <Paper variant="outlined" sx={{ p: 5, textAlign: 'center', borderRadius: 3, borderColor: BORDER }}>
        <Typography color="text.secondary" sx={{ mb: 2 }}>
          За выбранный период планов по этой сети нет.
        </Typography>
        <Button onClick={onBackToAll}>Вернуться ко всем сетям</Button>
      </Paper>
    );
  }

  const summary = data.summary;
  const overrun = summary.investmentVarianceRub > 0;
  const eacCompletion = eacCompletionOf(summary, unit);

  return (
    <Box>
      {/* Крошки показывают глубину и путь назад; кнопка браузера при этом
          остаётся свободной, потому что со страницы мы не уходили. */}
      <Stack
        direction="row"
        spacing={1}
        useFlexGap
        sx={{ alignItems: 'center', flexWrap: 'wrap', mb: 1.5 }}
      >
        <Button size="small" onClick={onBackToAll} sx={{ minWidth: 0, px: 0.75 }}>
          Итоги · все сети
        </Button>
        {kamCrumb && onBackToKAM && (
          <>
            <Typography variant="body2" color="text.disabled">/</Typography>
            <Button size="small" onClick={onBackToKAM} sx={{ minWidth: 0, px: 0.75 }}>{kamCrumb}</Button>
          </>
        )}
        <Typography variant="body2" color="text.disabled">/</Typography>
        <Typography variant="body2" sx={{ fontWeight: 700 }}>{network.name}</Typography>
      </Stack>

      <Paper variant="outlined" sx={{ p: 1.6, borderRadius: 3, borderColor: BORDER, mb: 1.25 }}>
        <Stack
          direction={{ xs: 'column', md: 'row' }}
          spacing={1.5}
          sx={{ alignItems: { xs: 'stretch', md: 'center' }, justifyContent: 'space-between' }}
        >
          <Box>
            <Typography variant="h6" sx={{ fontWeight: 780, lineHeight: 1.2 }}>{network.name}</Typography>
            <Stack direction="row" spacing={1} useFlexGap sx={{ mt: 0.6, flexWrap: 'wrap', alignItems: 'center' }}>
              {network.kam && <Chip size="small" variant="outlined" label={network.kam} />}
              <Typography variant="caption" color="text.secondary">
                {summary.brandCount} {pluralRu(summary.brandCount, 'бренд', 'бренда', 'брендов')}
                {' · '}
                {data.selectedQuarters.length === 4
                  ? `${data.year} год целиком`
                  : `${data.year}: ${data.selectedQuarters.map((q) => `Q${q}`).join(', ')}`}
              </Typography>
            </Stack>
          </Box>
          <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
            <ToggleButtonGroup
              size="small"
              exclusive
              value={unit}
              onChange={(_, value) => value && setUnit(value as Unit)}
            >
              <ToggleButton value="rub">₽</ToggleButton>
              <ToggleButton value="units">уп.</ToggleButton>
            </ToggleButtonGroup>
            <FormControlLabel
              control={<Switch size="small" checked={showValues} onChange={(event) => setShowValues(event.target.checked)} />}
              label={<Typography variant="body2">Значения на графиках</Typography>}
              sx={{ mr: 0 }}
            />
            <Button
              variant="contained"
              size="small"
              startIcon={<OpenInNewIcon />}
              onClick={() => network.networkId != null && onOpenCard(network.networkId)}
              disabled={network.networkId == null}
            >
              Открыть карточку сети
            </Button>
          </Stack>
        </Stack>
      </Paper>

      {/* ── Полоса 1: пять чисел ─────────────────────────────────────────── */}
      <Grid container spacing={1.25} sx={{ mb: 1.25 }}>
        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2.4 }}>
          <KpiCard
            label="Прогноз итога"
            primary={amount(metricEAC(summary, unit), unit)}
            secondary={`План ${amount(metricPlan(summary, unit), unit)}`}
            hint={`Факт ${amount(metricFact(summary, unit), unit)}`}
            accent={SERIES_EAC}
            trend={months.map((point) => point.fact ?? point.eac ?? 0)}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2.4 }}>
          <KpiCard
            label="Выполнение плана"
            primary={pctLabel(eacCompletion)}
            secondary={`${signedShort(summary.gapRub)} ₽ к обязательству`}
            hint="Прогноз итога к плану"
            accent={eacCompletion != null && eacCompletion >= 100 ? POLARITY_POSITIVE : POLARITY_NEGATIVE}
            trend={completionTrend}
            trendScale="range"
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2.4 }}>
          <KpiCard
            label="К прошлому году"
            primary={growthLabel(growthOf(summary, unit))}
            secondary={
              metricPrevFact(summary, unit) == null
                ? 'Сопоставимого периода нет'
                : `${data.year - 1}: ${amount(metricPrevFact(summary, unit) ?? 0, unit)}`
            }
            hint="Факт к факту того же периода"
            accent={SERIES_PREV}
            trend={months.map((point) => point.prevFact ?? 0)}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2.4 }}>
          <KpiCard
            label="Инвестиции"
            primary={pctLabel(summary.effectiveInvestmentsPct)}
            secondary={`${signedShort(summary.investmentVarianceRub)} ₽ к плану`}
            hint={overrun ? 'Перерасход, база без НДС' : 'Экономия, база без НДС'}
            accent={overrun ? POLARITY_NEGATIVE : POLARITY_POSITIVE}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2.4 }}>
          <KpiCard
            label="Готовность данных"
            primary={`${summary.closedCellsWithFact} / ${summary.closedCells}`}
            secondary={
              summary.openCellsWithoutForecast > 0
                ? `${summary.openCellsWithoutForecast} ${pluralRu(summary.openCellsWithoutForecast, 'ячейка', 'ячейки', 'ячеек')} без прогноза`
                : 'Открытые месяцы закрыты прогнозом'
            }
            hint="Закрытые ячейки с фактом"
            accent={summary.openCellsWithoutForecast > 0 ? POLARITY_NEGATIVE : POLARITY_POSITIVE}
          />
        </Grid>
      </Grid>

      {/* ── Полоса 2: ход года и отклонения ──────────────────────────────── */}
      <Grid container spacing={1.25} sx={{ mb: 1.25 }}>
        <Grid size={{ xs: 12, xl: 7 }}>
          <ChartPaper
            title="Ход года"
            subtitle="Закрытые месяцы — факт, открытые — прогноз. Линия — тот же месяц прошлого года."
            legend={(
              <SeriesLegend
                items={[
                  { label: 'План', color: SERIES_PLAN },
                  { label: 'Факт', color: SERIES_FACT },
                  { label: 'Прогноз', color: SERIES_EAC },
                  { label: 'Прошлый год', color: SERIES_PREV, dashed: true },
                ]}
              />
            )}
            height={320}
          >
            <ResponsiveContainer width="100%" height="100%">
              <ComposedChart data={months} margin={{ top: showValues ? 26 : 18, right: 8, left: 0, bottom: 0 }}>
                {/* Сценарий кодируется заливкой, а не только цветом: план —
                    контур, факт — сплошной, прогноз — штриховка. Так три ряда
                    различимы и при дальтонизме, и в чёрно-белой печати. */}
                <defs>
                  <pattern id="detail-hatch-year" width="6" height="6" patternTransform="rotate(45)" patternUnits="userSpaceOnUse">
                    <rect width="6" height="6" fill={SERIES_EAC} fillOpacity={0.26} />
                    <line x1="0" y1="0" x2="0" y2="6" stroke={SERIES_EAC} strokeWidth={3} />
                  </pattern>
                </defs>
                <CartesianGrid stroke={GRID} vertical={false} />
                <XAxis dataKey="tick" tick={{ fontSize: 11, fill: INK_MUTED }} tickLine={false} axisLine={{ stroke: GRID }} />
                <YAxis
                  tick={{ fontSize: 11, fill: INK_MUTED }}
                  tickLine={false}
                  axisLine={false}
                  tickFormatter={(value: number) => formatRubShort(value)}
                  width={66}
                />
                <Tooltip content={<MonthTooltip unit={unit} />} cursor={{ fill: 'rgba(99,102,241,.06)' }} />
                <Bar dataKey="plan" fill={SERIES_PLAN} fillOpacity={0.10} stroke={SERIES_PLAN} strokeWidth={1.75} radius={[3, 3, 0, 0]} isAnimationActive={false} />
                {/* Подписан результат месяца, а не все три ряда: факт и прогноз
                    друг друга исключают, поэтому на месяц приходится ровно одна
                    подпись. План рядом с ними встал бы вплотную и слился. */}
                <Bar dataKey="fact" fill={SERIES_FACT} radius={[3, 3, 0, 0]} isAnimationActive={false}>
                  {showValues && (
                    <LabelList
                      dataKey="fact" position="top" offset={6}
                      formatter={(value) => labelText(value, formatRubShort)}
                      style={BAR_LABEL_STYLE}
                    />
                  )}
                </Bar>
                <Bar dataKey="eac" fill="url(#detail-hatch-year)" stroke={SERIES_EAC} strokeWidth={1} radius={[3, 3, 0, 0]} isAnimationActive={false}>
                  {showValues && (
                    <LabelList
                      dataKey="eac" position="top" offset={6}
                      formatter={(value) => labelText(value, formatRubShort)}
                      style={BAR_LABEL_STYLE}
                    />
                  )}
                </Bar>
                <Line
                  dataKey="prevFact"
                  stroke={SERIES_PREV}
                  strokeWidth={2}
                  strokeDasharray="4 3"
                  dot={false}
                  isAnimationActive={false}
                  connectNulls
                />
              </ComposedChart>
            </ResponsiveContainer>
          </ChartPaper>
        </Grid>

        <Grid size={{ xs: 12, xl: 5 }}>
          <ChartPaper
            title="Отклонение от плана по брендам"
            subtitle="Прогноз итога минус обязательство. Знак означает «хорошо/плохо», а не «больше/меньше»."
            height={320}
          >
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={gaps} layout="vertical" margin={{ top: 4, right: 46, left: 4, bottom: 4 }}>
                <CartesianGrid stroke={GRID} horizontal={false} />
                <XAxis
                  type="number"
                  tick={{ fontSize: 11, fill: INK_MUTED }}
                  tickLine={false}
                  axisLine={false}
                  tickFormatter={(value: number) => formatRubShort(value)}
                />
                <YAxis
                  type="category"
                  dataKey="name"
                  tick={{ fontSize: 11, fill: INK_MUTED }}
                  tickLine={false}
                  axisLine={false}
                  width={130}
                />
                <ReferenceLine x={0} stroke={INK_MUTED} strokeWidth={1} />
                <Tooltip content={<GapTooltip unit={unit} />} cursor={{ fill: 'rgba(99,102,241,.06)' }} />
                <Bar dataKey="value" radius={[3, 3, 3, 3]} isAnimationActive={false}>
                  {showValues && (
                    <LabelList
                      dataKey="value" position="right"
                      formatter={(value) => labelText(value, signedShort)}
                      style={BAR_LABEL_STYLE}
                    />
                  )}
                  {gaps.map((point) => (
                    <Cell key={point.name} fill={gapColor(point.value)} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </ChartPaper>
        </Grid>
      </Grid>

      {/* ── Полоса 3: бренды построчно ───────────────────────────────────── */}
      <Paper variant="outlined" sx={{ borderRadius: 3, borderColor: BORDER, overflow: 'hidden', mb: 1.25 }}>
        <Stack
          direction={{ xs: 'column', md: 'row' }}
          spacing={1}
          sx={{
            px: 1.6, py: 1.25, borderBottom: `1px solid ${BORDER}`,
            justifyContent: 'space-between', alignItems: { xs: 'stretch', md: 'center' },
          }}
        >
          <Box>
            <Typography variant="subtitle1" sx={{ fontWeight: 750 }}>Бренды сети</Typography>
            <Typography variant="caption" color="text.secondary">
              Полоса: рамка — план, заливка — факт закрытых месяцев, штриховка — прогноз открытых.
              {detailKind === 'quarters'
                ? ' Стрелка раскрывает кварталы бренда.'
                : ' Стрелка раскрывает SKU — плана на них в реестре нет.'}
            </Typography>
          </Box>
          {/* Разрез общий для всей таблицы, а не свой у каждой строки: два
              переключателя в одной строке спорили бы друг с другом. */}
          {(quartersByBrand.size > 0 || skusByBrand.size > 0) && (
            <ToggleButtonGroup
              size="small"
              exclusive
              value={detailKind}
              onChange={(_, value) => value && setDetailKind(value as DetailKind)}
            >
              <ToggleButton value="quarters" disabled={quartersByBrand.size === 0}>Кварталы</ToggleButton>
              <ToggleButton value="skus" disabled={skusByBrand.size === 0}>SKU</ToggleButton>
            </ToggleButtonGroup>
          )}
        </Stack>
        <TableContainer sx={{ maxHeight: 520 }}>
          <Table stickyHeader size="small" sx={{ minWidth: 880 }}>
            <TableHead>
              <TableRow>
                <TableCell sx={{ fontWeight: 750, minWidth: 190 }}>Бренд</TableCell>
                <TableCell sx={{ fontWeight: 750, minWidth: 130 }}>План · факт · прогноз</TableCell>
                <TableCell align="right" sx={{ fontWeight: 750 }}>План</TableCell>
                <TableCell align="right" sx={{ fontWeight: 750 }}>Прогноз итога</TableCell>
                <TableCell align="center" sx={{ fontWeight: 750 }}>Вып.</TableCell>
                <TableCell align="right" sx={{ fontWeight: 750 }}>Δ к плану</TableCell>
                <TableCell align="right" sx={{ fontWeight: 750 }}>Δ к ПГ</TableCell>
                <TableCell align="right" sx={{ fontWeight: 750 }}>Инвест.</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {brands.map((item) => {
                const completion = eacCompletionOf(item.metrics, unit);
                const color = completionColor(completion);
                const growth = growthOf(item.metrics, unit);
                const quarters = quartersByBrand.get(item.name) ?? [];
                const skus = skusByBrand.get(item.name) ?? [];
                const rows = detailKind === 'quarters' ? quarters.length : skus.length;
                const isOpen = expanded.has(item.name) && rows > 0;
                // Сколько объёма бренда объяснено его SKU. Официальный прогноз
                // ведётся на бренде, поэтому недотяг здесь — норма, а не ошибка,
                // и молчать о нём нельзя.
                const skuEAC = skus.reduce((sum, row) => sum + skuEACOf(row), 0);
                const brandEAC = metricEAC(item.metrics, unit);
                const skuGap = brandEAC > 0 && skuEAC < brandEAC * 0.995;
                return (
                  <Fragment key={item.name}>
                  <TableRow hover>
                    <TableCell sx={{ fontWeight: 650 }}>
                      {rows > 0 && (
                        <IconButton
                          size="small"
                          onClick={() => toggleBrand(item.name)}
                          aria-label={`${isOpen ? 'Свернуть' : 'Раскрыть'} ${detailKind === 'quarters' ? 'кварталы' : 'SKU'}: ${item.name}`}
                          sx={{ mr: 0.5, p: 0.25 }}
                        >
                          {isOpen ? <ExpandMoreIcon fontSize="small" /> : <ChevronRightIcon fontSize="small" />}
                        </IconButton>
                      )}
                      {item.name}
                    </TableCell>
                    <TableCell>
                      <BulletBar
                        plan={metricPlan(item.metrics, unit)}
                        fact={metricFact(item.metrics, unit)}
                        eac={metricEAC(item.metrics, unit)}
                        unit={unit}
                      />
                    </TableCell>
                    <TableCell align="right" sx={NUMERIC_CELL}>{amount(metricPlan(item.metrics, unit), unit)}</TableCell>
                    <TableCell align="right" sx={{ ...NUMERIC_CELL, fontWeight: 650 }}>
                      {amount(metricEAC(item.metrics, unit), unit)}
                    </TableCell>
                    <TableCell align="center" sx={{ p: 0.5 }}>
                      <Box sx={{
                        px: 0.6, py: 0.2, borderRadius: 1, fontSize: 12, fontWeight: 750,
                        display: 'inline-block', minWidth: 46, ...color,
                      }}>
                        {completion == null ? '—' : `${Math.round(completion)}%`}
                      </Box>
                    </TableCell>
                    <TableCell align="right" sx={{ ...NUMERIC_CELL, color: gapColor(item.metrics.gapRub), fontWeight: 650 }}>
                      {signedShort(item.metrics.gapRub)} ₽
                    </TableCell>
                    <TableCell align="right" sx={{
                      ...NUMERIC_CELL,
                      color: growth == null ? INK_MUTED : (growth >= 0 ? POLARITY_POSITIVE : POLARITY_NEGATIVE),
                    }}>
                      {growthLabel(growth)}
                    </TableCell>
                    <TableCell align="right" sx={NUMERIC_CELL}>{pctLabel(item.metrics.effectiveInvestmentsPct)}</TableCell>
                  </TableRow>
                  {isOpen && detailKind === 'quarters' && quarters.map((row) => {
                    const rowCompletion = eacCompletionOf(row.metrics, unit);
                    const rowGrowth = growthOf(row.metrics, unit);
                    return (
                      <TableRow key={`${item.name}|${row.quarter}`} sx={{ bgcolor: 'action.hover' }}>
                        <TableCell sx={{ pl: 5.5, color: 'text.secondary' }}>
                          Q{row.quarter}
                          {(row.promoTags ?? []).length > 0 && <PromoTags tags={row.promoTags ?? []} limit={2} />}
                        </TableCell>
                        <TableCell>
                          <BulletBar
                            plan={metricPlan(row.metrics, unit)}
                            fact={metricFact(row.metrics, unit)}
                            eac={metricEAC(row.metrics, unit)}
                            unit={unit}
                          />
                        </TableCell>
                        <TableCell align="right" sx={NUMERIC_CELL}>{amount(metricPlan(row.metrics, unit), unit)}</TableCell>
                        <TableCell align="right" sx={NUMERIC_CELL}>{amount(metricEAC(row.metrics, unit), unit)}</TableCell>
                        <TableCell align="center" sx={{ p: 0.5 }}>
                          <Box sx={{
                            px: 0.6, py: 0.2, borderRadius: 1, fontSize: 11.5, fontWeight: 700,
                            display: 'inline-block', minWidth: 46, ...completionColor(rowCompletion),
                          }}>
                            {rowCompletion == null ? '—' : `${Math.round(rowCompletion)}%`}
                          </Box>
                        </TableCell>
                        <TableCell align="right" sx={{ ...NUMERIC_CELL, color: gapColor(row.metrics.gapRub) }}>
                          {signedShort(row.metrics.gapRub)} ₽
                        </TableCell>
                        <TableCell align="right" sx={{
                          ...NUMERIC_CELL,
                          color: rowGrowth == null ? INK_MUTED : (rowGrowth >= 0 ? POLARITY_POSITIVE : POLARITY_NEGATIVE),
                        }}>
                          {growthLabel(rowGrowth)}
                        </TableCell>
                        <TableCell align="right" sx={NUMERIC_CELL}>{pctLabel(row.metrics.effectiveInvestmentsPct)}</TableCell>
                      </TableRow>
                    );
                  })}
                  {isOpen && detailKind === 'skus' && skus.map((row) => {
                    const growth = skuGrowthOf(row);
                    const prev = skuPrevOf(row);
                    return (
                      <TableRow key={`${item.name}|${row.sku}`} sx={{ bgcolor: 'action.hover' }}>
                        <TableCell sx={{ pl: 5.5, color: 'text.secondary' }}>
                          {row.sku}
                          {row.shareOfBrandPct != null && (
                            <Typography component="span" variant="caption" sx={{ ml: 0.75, color: INK_MUTED }}>
                              · {pctLabel(row.shareOfBrandPct)} бренда
                            </Typography>
                          )}
                        </TableCell>
                        {/* Плановые колонки у SKU пусты не по недосмотру:
                            в реестре план заводится брендом. Доля бренда,
                            выданная за план, была бы вычисленным под видом
                            измеренного. */}
                        <TableCell />
                        <TableCell align="right" sx={{ ...NUMERIC_CELL, color: 'text.disabled' }}>—</TableCell>
                        <TableCell align="right" sx={NUMERIC_CELL}>{amount(skuEACOf(row), unit)}</TableCell>
                        <TableCell align="center" sx={{ color: 'text.disabled' }}>—</TableCell>
                        <TableCell align="right" sx={{ ...NUMERIC_CELL, color: 'text.disabled' }}>—</TableCell>
                        <TableCell align="right" sx={{
                          ...NUMERIC_CELL,
                          color: growth == null ? INK_MUTED : (growth >= 0 ? POLARITY_POSITIVE : POLARITY_NEGATIVE),
                        }}>
                          <MuiTooltip
                            arrow
                            title={prev == null
                              ? 'Сопоставимого периода нет'
                              : `Факт ${amountFull(skuFactOf(row), unit)} против ${amountFull(prev, unit)}`}
                          >
                            <span>{growthLabel(growth)}</span>
                          </MuiTooltip>
                        </TableCell>
                        <TableCell align="right" sx={NUMERIC_CELL}>
                          {row.factInvestmentsRub > 0 ? `${formatRubShort(row.factInvestmentsRub)} ₽` : '—'}
                        </TableCell>
                      </TableRow>
                    );
                  })}
                  {isOpen && detailKind === 'skus' && skuGap && (
                    <TableRow sx={{ bgcolor: 'action.hover' }}>
                      <TableCell colSpan={8} sx={{ pl: 5.5, py: 0.75 }}>
                        <Typography variant="caption" color="text.secondary">
                          SKU объясняют {amount(skuEAC, unit)} из {amount(brandEAC, unit)}: официальный
                          прогноз ведётся на бренде, и месяцы без SKU-строк прогноза в эту сумму не входят.
                        </Typography>
                      </TableCell>
                    </TableRow>
                  )}
                  </Fragment>
                );
              })}
            </TableBody>
          </Table>
        </TableContainer>
        {summary.undistributedRub != null && summary.undistributedRub !== 0 && (
          <Box sx={{ px: 1.6, py: 1, borderTop: `1px solid ${BORDER}` }}>
            <Typography variant="caption" color="text.secondary">
              Не разобрано брендами из валового пула: {formatRubShort(summary.undistributedRub)} ₽. В сумму
              брендов этот остаток не входит — он часть обязательства по контракту, а не отдельный бренд.
            </Typography>
          </Box>
        )}
      </Paper>

      {/* ── Полоса 4: промо-календарь ────────────────────────────────────── */}
      {hasPromo && (
        <Paper variant="outlined" sx={{ borderRadius: 3, borderColor: BORDER, overflow: 'hidden', mb: 1.25 }}>
          <Box sx={{ px: 1.6, py: 1.25, borderBottom: `1px solid ${BORDER}` }}>
            <Typography variant="subtitle1" sx={{ fontWeight: 750 }}>Промо-календарь</Typography>
            <Typography variant="caption" color="text.secondary">
              Бренд × месяц. Число — сколько промо, цвет — преобладающий канал:{' '}
              <Box component="span" sx={{ color: CHANNEL_COLOR['онлайн'], fontWeight: 700 }}>онлайн</Box>,{' '}
              <Box component="span" sx={{ color: CHANNEL_COLOR['оффлайн'], fontWeight: 700 }}>оффлайн</Box>.
              Пустая строка означает год без единого промо.
            </Typography>
          </Box>
          <Box sx={{ overflowX: 'auto', p: 1.25 }}>
            <Box sx={{
              display: 'grid',
              gridTemplateColumns: `minmax(150px, 190px) repeat(${calendarMonths.length}, minmax(28px, 1fr))`,
              gap: 0.4,
              minWidth: 520,
            }}>
              <Box />
              {calendarMonths.map((month) => (
                <Typography
                  key={`head-${month}`}
                  variant="caption"
                  sx={{ textAlign: 'center', fontWeight: 700, color: 'text.secondary', fontSize: 10.5 }}
                >
                  {MONTH_FULL[month - 1].slice(0, 3)}
                </Typography>
              ))}
              {calendarBrands.map((brand) => (
                <Fragment key={`row-${brand}`}>
                  <Typography
                    variant="caption"
                    sx={{
                      alignSelf: 'center', pr: 1, overflow: 'hidden',
                      textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 12,
                    }}
                    title={brand}
                  >
                    {brand}
                  </Typography>
                  {calendarMonths.map((month) => {
                    const cell = promoByBrandMonth.get(`${brand}|${month}`);
                    if (!cell) {
                      return <Box key={`${brand}|${month}`} sx={{ height: 26, borderRadius: 1, bgcolor: 'action.hover' }} />;
                    }
                    const channel = cell.promoOnlineCount >= cell.promoOfflineCount ? 'онлайн' : 'оффлайн';
                    return (
                      <MuiTooltip
                        key={`${brand}|${month}`}
                        arrow
                        title={
                          <Box sx={{ p: 0.25 }}>
                            <Typography variant="caption" sx={{ display: 'block', fontWeight: 750 }}>
                              {brand} · {MONTH_FULL[month - 1]}
                            </Typography>
                            <Typography variant="caption" sx={{ display: 'block' }}>
                              {cell.promoCount} {pluralRu(cell.promoCount, 'промо', 'промо', 'промо')}
                              {cell.promoOnlineCount > 0 && `, онлайн ${cell.promoOnlineCount}`}
                              {cell.promoOfflineCount > 0 && `, оффлайн ${cell.promoOfflineCount}`}
                            </Typography>
                            {cell.promoInvestmentsRub > 0 && (
                              <Typography variant="caption" sx={{ display: 'block' }}>
                                Плановые инвестиции: {formatRubShort(cell.promoInvestmentsRub)} ₽
                              </Typography>
                            )}
                            {(cell.promoTags ?? []).map((tag) => (
                              <Typography key={`${tag.code}|${tag.channel}`} variant="caption" sx={{ display: 'block' }}>
                                {tag.mechanics} · {tag.channel} · {tag.count}
                              </Typography>
                            ))}
                          </Box>
                        }
                      >
                        <Box sx={{
                          height: 26, borderRadius: 1, display: 'grid', placeItems: 'center',
                          bgcolor: CHANNEL_COLOR[channel], color: '#fff',
                          fontSize: 11, fontWeight: 750, cursor: 'default',
                        }}>
                          {cell.promoCount}
                        </Box>
                      </MuiTooltip>
                    );
                  })}
                </Fragment>
              ))}
            </Box>
          </Box>
        </Paper>
      )}

      {/* ── Полоса 5: инвестиции ─────────────────────────────────────────── */}
      <ChartPaper
        title="Инвестиции по кварталам"
        subtitle="План против ожидаемого, база без НДС: сети работают на разных ставках, и сравнивать их можно только так."
        legend={(
          <SeriesLegend
            items={[
              { label: 'План', color: SERIES_PLAN },
              { label: 'Ожидаемые', color: SERIES_EAC },
            ]}
          />
        )}
        height={290}
      >
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={investments} margin={{ top: showValues ? 26 : 18, right: 8, left: 0, bottom: 0 }} maxBarSize={78}>
            <defs>
              <pattern id="detail-hatch-invest" width="6" height="6" patternTransform="rotate(45)" patternUnits="userSpaceOnUse">
                <rect width="6" height="6" fill={SERIES_EAC} fillOpacity={0.26} />
                <line x1="0" y1="0" x2="0" y2="6" stroke={SERIES_EAC} strokeWidth={3} />
              </pattern>
            </defs>
            <CartesianGrid stroke={GRID} vertical={false} />
            <XAxis
              dataKey="tick"
              tick={<InvestmentTick points={investments} />}
              tickLine={false}
              axisLine={{ stroke: GRID }}
              height={40}
              interval={0}
            />
            <YAxis
              tick={{ fontSize: 11, fill: INK_MUTED }}
              tickLine={false}
              axisLine={false}
              tickFormatter={(value: number) => formatRubShort(value)}
              width={66}
            />
            <Tooltip content={<InvestmentTooltip year={data.year} />} cursor={{ fill: 'rgba(99,102,241,.06)' }} />
            {/* Здесь подписаны оба ряда: кварталов не больше четырёх, места
                хватает, а весь смысл графика — расхождение плана с ожидаемым. */}
            <Bar dataKey="plan" name="План" fill={SERIES_PLAN} fillOpacity={0.10} stroke={SERIES_PLAN} strokeWidth={1.75} radius={[3, 3, 0, 0]} isAnimationActive={false}>
              {showValues && (
                <LabelList
                  dataKey="plan" position="top" offset={6}
                  formatter={(value) => labelText(value, formatRubShort)}
                  style={BAR_LABEL_STYLE}
                />
              )}
            </Bar>
            <Bar dataKey="eac" name="Ожидаемые" fill="url(#detail-hatch-invest)" stroke={SERIES_EAC} strokeWidth={1} radius={[3, 3, 0, 0]} isAnimationActive={false}>
              {showValues && (
                <LabelList
                  dataKey="eac" position="top" offset={6}
                  formatter={(value) => labelText(value, formatRubShort)}
                  style={BAR_LABEL_STYLE}
                />
              )}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </ChartPaper>
    </Box>
  );
}
