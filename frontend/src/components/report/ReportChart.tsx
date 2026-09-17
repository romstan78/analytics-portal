// Графики печатного отчёта: та же палитра и те же приёмы Recharts, что на
// витрине «Итоги», но с явными размерами — ResponsiveContainer в печати
// мерит ширину до перестроения листа и обрезает правый край.
//
// Спецификация приходит с сервера (ReportPrintChart): месячная динамика,
// bullet-строки разреза или ступени водопада. Числа — исходные величины,
// оси подписываются в шкале страницы.

import { Box } from '@mui/material';
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
  XAxis,
  YAxis,
} from 'recharts';
import { ChartPaper, SeriesLegend } from '../NetworkDashboardParts';
import {
  GRID,
  INK_MUTED,
  NEUTRAL,
  POLARITY_NEGATIVE,
  POLARITY_POSITIVE,
  SERIES_EAC,
  SERIES_FACT,
  SERIES_PLAN,
  SERIES_PREV,
} from '../../utils/networkDashboard';
import { REPORT_TONE_COLOR, formatScaled, formatScaledSigned, reportTone } from '../../utils/reportFormat';
import type {
  ReportPrintBullet,
  ReportPrintChart,
  ReportPrintMonth,
  ReportPrintScale,
  ReportPrintStep,
} from '../../types/reports';

// Ширина графика — внутренняя ширина листа минус отступы карточки; высота
// bullet-графика растёт со строками, остальные фиксированы.
export const REPORT_CHART_WIDTH = 1000;
const TREND_HEIGHT = 290;
const WATERFALL_HEIGHT = 250;
const BULLET_ROW_HEIGHT = 34;

export default function ReportChart({ chart }: { chart: ReportPrintChart }) {
  const scale = chart.scale;
  let body: React.ReactNode;
  let legend: React.ReactNode = null;
  let height = TREND_HEIGHT;

  if (chart.months && chart.months.length > 0) {
    const hasPrev = chart.months.some((m) => m.prev != null);
    body = <TrendChart months={chart.months} scale={scale} hasPrev={hasPrev} />;
    legend = (
      <SeriesLegend
        items={[
          { label: 'Факт', color: SERIES_FACT },
          { label: 'Ожидаемый итог', color: SERIES_EAC },
          { label: 'План', color: SERIES_PLAN, dashed: true },
          ...(hasPrev ? [{ label: 'Прошлый год', color: SERIES_PREV, dashed: true }] : []),
        ]}
      />
    );
  } else if (chart.bullet && chart.bullet.length > 0) {
    height = chart.bullet.length * BULLET_ROW_HEIGHT + 36;
    body = <BulletChart rows={chart.bullet} scale={scale} height={height} />;
    legend = (
      <SeriesLegend
        items={[
          { label: 'План', color: PLAN_BAND },
          { label: 'Факт', color: SERIES_FACT },
          { label: 'Ожидаемый итог', color: SERIES_EAC, dashed: true },
        ]}
      />
    );
  } else if (chart.steps && chart.steps.length > 0) {
    height = WATERFALL_HEIGHT;
    body = <WaterfallChart steps={chart.steps} scale={scale} />;
  } else {
    return null;
  }

  return (
    <Box className="report-keep">
      <ChartPaper title={chart.title} subtitle={chart.subtitle ?? ''} legend={legend} height={height}>
        {body}
      </ChartPaper>
    </Box>
  );
}

// ─── Динамика по месяцам ────────────────────────────────────────────────────

function TrendChart({ months, scale, hasPrev }: {
  months: ReportPrintMonth[];
  scale: ReportPrintScale;
  hasPrev: boolean;
}) {
  // Факт есть только у закрытых месяцев: у открытых он частичный, и его
  // площадь читалась бы как провал до нуля.
  const data = months.map((m) => ({
    label: m.label, plan: m.plan, fact: m.closed ? m.fact : null, eac: m.eac, prev: m.prev,
  }));
  return (
    <AreaChart width={REPORT_CHART_WIDTH} height={TREND_HEIGHT} data={data} margin={{ top: 24, right: 24, left: 4, bottom: 0 }}>
      <defs>
        <linearGradient id="reportAreaFact" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={SERIES_FACT} stopOpacity={0.38} />
          <stop offset="100%" stopColor={SERIES_FACT} stopOpacity={0.03} />
        </linearGradient>
        <linearGradient id="reportAreaEac" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={SERIES_EAC} stopOpacity={0.28} />
          <stop offset="100%" stopColor={SERIES_EAC} stopOpacity={0.02} />
        </linearGradient>
      </defs>
      <CartesianGrid stroke={GRID} vertical={false} />
      <XAxis dataKey="label" tick={{ fontSize: 11 }} stroke={NEUTRAL} />
      <YAxis tickFormatter={(value) => formatScaled(Number(value), scale)} tick={{ fontSize: 11 }} width={64} stroke={NEUTRAL} />
      <Area
        dataKey="eac" stroke={SERIES_EAC} strokeWidth={2} fill="url(#reportAreaEac)"
        isAnimationActive={false} dot={{ r: 3, strokeWidth: 0, fill: SERIES_EAC }}
      >
        <LabelList
          dataKey="eac" position="top" offset={10}
          formatter={(value) => formatScaled(Number(value), scale)}
          style={{ fontSize: 10, fontWeight: 700, fill: INK_MUTED }}
        />
      </Area>
      <Area
        dataKey="fact" stroke={SERIES_FACT} strokeWidth={2.4} fill="url(#reportAreaFact)"
        isAnimationActive={false} dot={{ r: 3, strokeWidth: 0, fill: SERIES_FACT }} connectNulls={false}
      />
      <Line dataKey="plan" stroke={SERIES_PLAN} strokeWidth={2} strokeDasharray="5 4" dot={false} isAnimationActive={false} />
      {hasPrev && (
        <Line dataKey="prev" stroke={SERIES_PREV} strokeWidth={2} strokeDasharray="3 3" dot={false} isAnimationActive={false} connectNulls={false} />
      )}
    </AreaChart>
  );
}

// ─── Bullet-график разреза ──────────────────────────────────────────────────
//
// Строка — квартал, сеть или бренд: широкая светлая полоса — план, узкая
// зелёная внутри — факт, янтарная риска — ожидаемый итог. Все три в одной
// шкале, поэтому разрыв виден без чтения чисел. Столбец Recharts несёт
// максимум из трёх величин, а фигура рисует три части сама.

const PLAN_BAND = '#c7cbf7';

type BulletPoint = {
  name: string;
  extent: number;
  plan: number;
  fact: number;
  eac: number;
  pctText: string;
  pctColor: string;
};

function BulletChart({ rows, scale, height }: {
  rows: ReportPrintBullet[];
  scale: ReportPrintScale;
  height: number;
}) {
  // Название и подпись (КАМ, тип) — двумя строками подписи оси: разделитель
  // \n разбирает CategoryTick.
  const data: BulletPoint[] = rows.map((row) => ({
    name: row.sub ? `${row.label}\n${row.sub}` : row.label,
    extent: Math.max(row.plan, row.fact, row.eac, 0),
    plan: row.plan,
    fact: row.fact,
    eac: row.eac,
    pctText: row.pctLabel,
    pctColor: REPORT_TONE_COLOR[reportTone(row.tone)],
  }));
  return (
    <BarChart
      width={REPORT_CHART_WIDTH}
      height={height}
      data={data}
      layout="vertical"
      margin={{ top: 4, right: 64, left: 8, bottom: 0 }}
      barCategoryGap={6}
    >
      <CartesianGrid stroke={GRID} horizontal={false} />
      <XAxis type="number" tickFormatter={(value) => formatScaled(Number(value), scale)} tick={{ fontSize: 11 }} stroke={NEUTRAL} />
      <YAxis type="category" dataKey="name" width={230} interval={0} tick={<CategoryTick />} stroke={NEUTRAL} />
      <Bar dataKey="extent" isAnimationActive={false} shape={<BulletShape />}>
        <LabelList
          dataKey="pctText"
          position="right"
          offset={8}
          content={(props) => <PctLabel {...props} data={data} />}
        />
      </Bar>
    </BarChart>
  );
}

// Подпись строки: название и под ним подпись мельче. Штатный tick Recharts
// переносит длинное название сам, и строки наезжают на соседние, поэтому
// обрезка своя.
const clip = (text: string, max: number) => (text.length > max ? `${text.slice(0, max - 2)}…` : text);

function CategoryTick(props: { x?: number; y?: number; payload?: { value?: unknown } }) {
  const { x = 0, y = 0, payload } = props;
  const [label, sub] = String(payload?.value ?? '').split('\n');
  if (!sub) {
    return <text x={x - 6} y={y} dy={4} textAnchor="end" fontSize={11} fill={INK_MUTED}>{clip(label, 36)}</text>;
  }
  return (
    <text x={x - 6} y={y} textAnchor="end" fill={INK_MUTED}>
      <tspan x={x - 6} dy={-2} fontSize={11}>{clip(label, 36)}</tspan>
      <tspan x={x - 6} dy={12} fontSize={9.5} fill={NEUTRAL}>{clip(sub, 40)}</tspan>
    </text>
  );
}

function BulletShape(props: { x?: number; y?: number; width?: number; height?: number; payload?: BulletPoint }) {
  const { x = 0, y = 0, width = 0, height = 0, payload } = props;
  if (!payload || payload.extent <= 0 || width <= 0) return null;
  const k = width / payload.extent;
  const planW = Math.max(0, payload.plan * k);
  const factW = Math.max(0, payload.fact * k);
  const eacX = x + Math.max(0, payload.eac * k);
  const inner = height * 0.46;
  const innerY = y + (height - inner) / 2;
  return (
    <g>
      <rect x={x} y={y} width={planW} height={height} rx={4} fill={PLAN_BAND} />
      <rect x={x} y={innerY} width={factW} height={inner} rx={3} fill={SERIES_FACT} />
      <rect x={eacX - 1.5} y={y - 3} width={3} height={height + 6} rx={1} fill={SERIES_EAC} />
    </g>
  );
}

function PctLabel(props: { x?: number | string; y?: number | string; width?: number | string; height?: number | string; index?: number; data: BulletPoint[] }) {
  const { index, data } = props;
  const x = Number(props.x ?? 0) + Number(props.width ?? 0) + 8;
  const y = Number(props.y ?? 0) + Number(props.height ?? 0) / 2;
  const point = index == null ? undefined : data[index];
  if (!point) return null;
  return (
    <text x={x} y={y} dominantBaseline="middle" fontSize={11} fontWeight={700} fill={point.pctColor}>
      {point.pctText}
    </text>
  );
}

// ─── Водопад «от плана к ожидаемому итогу» ──────────────────────────────────

type WaterfallPoint = { label: string; base: number; delta: number; text: string; color: string };

// Ступени в столбцы: опорная — от нуля, промежуточная — от предыдущего
// уровня; последняя опорная — ожидаемый итог, янтарный, как везде.
function waterfallPoints(steps: ReportPrintStep[], scale: ReportPrintScale): WaterfallPoint[] {
  const points: WaterfallPoint[] = [];
  let running = 0;
  steps.forEach((step) => {
    if (step.total) {
      running = step.value;
      points.push({ label: step.label, base: 0, delta: step.value, text: formatScaled(step.value, scale), color: SERIES_PLAN });
      return;
    }
    const from = running;
    running += step.value;
    points.push({
      label: step.label,
      base: Math.min(from, running),
      delta: Math.abs(step.value),
      text: formatScaledSigned(step.value, scale),
      color: step.value >= 0 ? POLARITY_POSITIVE : POLARITY_NEGATIVE,
    });
  });
  const lastTotal = steps.map((s, i) => (s.total ? i : -1)).filter((i) => i > 0).pop();
  if (lastTotal != null) points[lastTotal].color = SERIES_EAC;
  return points;
}

function WaterfallChart({ steps, scale }: { steps: ReportPrintStep[]; scale: ReportPrintScale }) {
  const data = waterfallPoints(steps, scale);
  return (
    <BarChart width={REPORT_CHART_WIDTH} height={WATERFALL_HEIGHT} data={data} margin={{ top: 20, right: 16, left: 4, bottom: 0 }} barCategoryGap="35%">
      <CartesianGrid stroke={GRID} vertical={false} />
      <XAxis dataKey="label" tick={{ fontSize: 11 }} stroke={NEUTRAL} />
      <YAxis tickFormatter={(value) => formatScaled(Number(value), scale)} tick={{ fontSize: 11 }} width={64} stroke={NEUTRAL} />
      <ReferenceLine y={0} stroke={NEUTRAL} />
      <Bar dataKey="base" stackId="w" fill="transparent" isAnimationActive={false} />
      <Bar dataKey="delta" stackId="w" isAnimationActive={false} radius={[4, 4, 0, 0]}>
        <LabelList dataKey="text" position="top" style={{ fontSize: 11, fontWeight: 700, fill: INK_MUTED }} />
        {data.map((point) => <Cell key={point.label} fill={point.color} />)}
      </Bar>
    </BarChart>
  );
}
