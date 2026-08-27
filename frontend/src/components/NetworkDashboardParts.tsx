// Общие блоки витрины реестра и разбора одной сети.
//
// Компоненты только рисуют: суммы приходят посчитанными с сервера тем же
// кодом, что считает карточку сети, и здесь ничего не пересчитывается.

import { useMemo } from 'react';
import { Box, Paper, Stack, Tooltip as MuiTooltip, Typography } from '@mui/material';
import { Area, AreaChart, ResponsiveContainer } from 'recharts';
import { pluralRu } from '../utils/networkPlan';
import {
  BORDER,
  CHANNEL_COLOR,
  INK_MUTED,
  NEUTRAL,
  POLARITY_NEGATIVE,
  POLARITY_POSITIVE,
  growthLabel,
} from '../utils/networkDashboard';
import type { NetworkDashboardPromoTag } from '../types/network';

// Спарклайн в карточке: форма ряда без осей и подписей — она отвечает на
// вопрос «как шло», а точные числа стоят рядом цифрами.
export function Sparkline({ values, color }: { values: number[]; color: string }) {
  const points = useMemo(() => values.map((value, index) => ({ index, value })), [values]);
  if (points.length < 2) return null;
  return (
    <Box sx={{ height: 34, mt: 0.5, mx: -0.5 }}>
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={points} margin={{ top: 2, right: 2, left: 2, bottom: 0 }}>
          <defs>
            <linearGradient id={`spark-${color.replace('#', '')}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity={0.35} />
              <stop offset="100%" stopColor={color} stopOpacity={0.02} />
            </linearGradient>
          </defs>
          <Area
            dataKey="value"
            stroke={color}
            strokeWidth={2}
            fill={`url(#spark-${color.replace('#', '')})`}
            isAnimationActive={false}
            dot={false}
          />
        </AreaChart>
      </ResponsiveContainer>
    </Box>
  );
}

export function KpiCard({
  label, primary, secondary, hint, accent, trend, growth,
}: {
  label: string;
  primary: string;
  secondary?: string;
  hint?: string;
  accent: string;
  trend?: number[];
  growth?: number | null;
}) {
  return (
    <Paper
      variant="outlined"
      sx={{
        p: 1.6, height: '100%', borderRadius: 3, borderColor: BORDER,
        borderTop: `3px solid ${accent}`, display: 'flex', flexDirection: 'column',
      }}
    >
      <Stack direction="row" sx={{ alignItems: 'baseline', justifyContent: 'space-between', gap: 1 }}>
        <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 650 }}>{label}</Typography>
        {growth != null && (
          <Typography
            variant="caption"
            sx={{ fontWeight: 750, color: growth >= 0 ? POLARITY_POSITIVE : POLARITY_NEGATIVE }}
          >
            {growthLabel(growth)}
          </Typography>
        )}
      </Stack>
      <Typography variant="h6" sx={{ mt: 0.35, fontWeight: 780, lineHeight: 1.2 }}>{primary}</Typography>
      {secondary && <Typography variant="body2" sx={{ mt: 0.45, fontWeight: 600 }}>{secondary}</Typography>}
      {hint && <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.35 }}>{hint}</Typography>}
      <Box sx={{ flex: 1 }} />
      {trend && trend.length > 1 && <Sparkline values={trend} color={accent} />}
    </Paper>
  );
}

export function ChartPaper({
  title, subtitle, action, legend, height = 300, children,
}: {
  title: string;
  subtitle: string;
  action?: React.ReactNode;
  legend?: React.ReactNode;
  height?: number;
  children: React.ReactNode;
}) {
  return (
    <Paper variant="outlined" sx={{ p: 1.6, height: '100%', borderRadius: 3, borderColor: BORDER }}>
      <Stack direction={{ xs: 'column', md: 'row' }} spacing={1} sx={{ justifyContent: 'space-between', alignItems: { xs: 'stretch', md: 'flex-start' } }}>
        <Box>
          <Typography variant="subtitle1" sx={{ fontWeight: 750 }}>{title}</Typography>
          <Typography variant="caption" color="text.secondary">{subtitle}</Typography>
          {legend}
        </Box>
        {action}
      </Stack>
      <Box sx={{ height, mt: 0.75 }}>{children}</Box>
    </Paper>
  );
}

// Своя легенда вместо recharts: там порядок элементов обратен порядку серий
// и подписи не совпадают с тем, что читается слева направо.
export function SeriesLegend({ items }: { items: Array<{ label: string; color: string; dashed?: boolean }> }) {
  return (
    <Stack direction="row" spacing={1.5} useFlexGap sx={{ flexWrap: 'wrap', mt: 0.75 }}>
      {items.map((item) => (
        <Stack key={item.label} direction="row" spacing={0.6} sx={{ alignItems: 'center' }}>
          <Box
            sx={{
              width: 12, height: item.dashed ? 0 : 10, borderRadius: 0.5,
              bgcolor: item.dashed ? 'transparent' : item.color,
              borderTop: item.dashed ? `2px dashed ${item.color}` : 'none',
            }}
          />
          <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>{item.label}</Typography>
        </Stack>
      ))}
    </Stack>
  );
}

// Метки промо: короткий код механики, цветом — канал. Полное название и
// количество остаются в подсказке, чтобы плитка не разрасталась.
export function PromoTags({ tags, limit = 3 }: { tags: NetworkDashboardPromoTag[]; limit?: number }) {
  if (tags.length === 0) return null;
  const shown = tags.slice(0, limit);
  const rest = tags.length - shown.length;
  return (
    <Stack direction="row" spacing={0.3} useFlexGap sx={{ flexWrap: 'wrap', justifyContent: 'center', mt: 0.3 }}>
      {shown.map((tag) => (
        <MuiTooltip
          key={`${tag.code}|${tag.channel}`}
          arrow
          title={`${tag.mechanics} · ${tag.channel} · ${tag.count} ${pluralRu(tag.count, 'промо', 'промо', 'промо')}`}
        >
          <Box
            component="span"
            sx={{
              fontSize: 9, fontWeight: 700, lineHeight: 1.5, px: 0.4, borderRadius: 0.5,
              color: '#fff', bgcolor: CHANNEL_COLOR[tag.channel] ?? NEUTRAL, whiteSpace: 'nowrap',
            }}
          >
            {tag.code}
          </Box>
        </MuiTooltip>
      ))}
      {rest > 0 && (
        <Box component="span" sx={{ fontSize: 9, fontWeight: 700, lineHeight: 1.5, color: INK_MUTED }}>
          +{rest}
        </Box>
      )}
    </Stack>
  );
}
