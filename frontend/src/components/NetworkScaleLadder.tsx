// Лесенка ступеней: полоса объёма с рисками порогов, факт, прогноз и крышка.
//
// Показывает всё правило одним взглядом: какие пороги пройдены прогнозом,
// где факт, и что срезает крышка. Рисуется у владельца порога — пула или
// отдельного бренда; расчёты не делает, только кладёт готовые числа на ось.

import { Box, useTheme } from '@mui/material';
import { formatRubShort } from '../utils/networkPlan';

export interface LadderThreshold {
  scaleNo: number;
  value: number;
  reached: boolean;
}

interface Props {
  thresholds: LadderThreshold[];
  fact: number | null;
  forecast: number | null;
  // Предел базы к оплате на последней ступени (план × (1 + c %)); только у
  // процентной крышки. Всё, что правее, — срез.
  capLimit: number | null;
}

const WIDTH = 640;
const HEIGHT = 48;
const LEFT = 10;
const RIGHT = 610;

export default function NetworkScaleLadder({ thresholds, fact, forecast, capLimit }: Props) {
  const theme = useTheme();
  const values = [
    ...thresholds.map((t) => t.value),
    fact ?? 0,
    forecast ?? 0,
    capLimit ?? 0,
  ].filter((v) => v > 0);
  if (values.length === 0) return null;
  // Ось начинается не с нуля: пороги стоят в пределах нескольких процентов
  // друг от друга, и от нуля они слились бы в одну риску. Начало — чуть ниже
  // наименьшей величины, чтобы разница между порогами была видна.
  const max = Math.max(...values) * 1.04;
  const origin = Math.min(...values) * 0.85;
  const x = (value: number) => LEFT + ((RIGHT - LEFT) * Math.min(Math.max(value, origin) - origin, max - origin)) / (max - origin);

  const good = theme.palette.success.main;
  const warn = theme.palette.warning.main;
  const muted = theme.palette.text.disabled;
  const primary = theme.palette.primary.main;
  const light = theme.palette.primary.light;
  const track = theme.palette.action.hover;
  const secondary = theme.palette.text.secondary;

  const forecastX = forecast != null ? x(forecast) : LEFT;
  const factX = fact != null ? x(fact) : LEFT;
  const cut = capLimit != null && forecast != null && forecast > capLimit;

  return (
    <Box sx={{ maxWidth: WIDTH, my: 0.5 }}>
      <svg
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        width="100%"
        height={HEIGHT}
        role="img"
        aria-label="Лесенка ступеней: пороги, факт, прогноз и крышка"
        style={{ display: 'block' }}
      >
        <defs>
          <pattern id="ladder-cut" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
            <rect width="6" height="6" fill={theme.palette.warning.light} opacity="0.5" />
            <line x1="0" y1="0" x2="0" y2="6" stroke={warn} strokeWidth="1.5" />
          </pattern>
        </defs>
        <rect x={LEFT} y={19} width={RIGHT - LEFT} height={10} rx={5} fill={track} />
        {forecast != null && (
          <rect x={LEFT} y={19} width={Math.max(0, (cut ? x(capLimit) : forecastX) - LEFT)} height={10} rx={5} fill={light} opacity="0.55" />
        )}
        {cut && capLimit != null && (
          <rect x={x(capLimit)} y={19} width={Math.max(0, forecastX - x(capLimit))} height={10} fill="url(#ladder-cut)" />
        )}
        {fact != null && fact > 0 && (
          <rect x={LEFT} y={19} width={Math.max(0, factX - LEFT)} height={10} rx={5} fill={primary} />
        )}
        {thresholds.map((t, i) => {
          // Близкие риски подписываются по очереди слева и справа от риски,
          // а последняя — слева, если сразу за ней стоит крышка.
          const capClose = capLimit != null && capLimit > t.value && x(capLimit) - x(t.value) < 70;
          const prevClose = i > 0 && x(t.value) - x(thresholds[i - 1].value) < 70;
          const anchor = capClose || (prevClose && i % 2 === 1) ? 'end' : prevClose ? 'start' : 'middle';
          const dx = anchor === 'end' ? -3 : anchor === 'start' ? 3 : 0;
          return (
            <g key={t.scaleNo}>
              <line
                x1={x(t.value)} y1={11} x2={x(t.value)} y2={37}
                stroke={t.reached ? good : muted} strokeWidth={2}
                strokeDasharray={t.reached ? undefined : '3 2'}
              />
              <text x={x(t.value) + dx} y={8} fontSize={10} textAnchor={anchor} fill={t.reached ? good : secondary} fontWeight={600}>
                {`ст.${t.scaleNo} · ${formatRubShort(t.value)}`}
              </text>
            </g>
          );
        })}
        {capLimit != null && (
          <g>
            <line x1={x(capLimit)} y1={9} x2={x(capLimit)} y2={39} stroke={warn} strokeWidth={2} strokeDasharray="3 2" />
            {/* Закрытая крышка совпадает с последним порогом — подпись не дублируется. */}
            {!thresholds.some((t) => Math.abs(t.value - capLimit) < 1e-6) && (
              <text x={Math.min(x(capLimit) + 4, RIGHT - 60)} y={8} fontSize={10} fill={warn} fontWeight={600}>
                {`крышка · ${formatRubShort(capLimit)}`}
              </text>
            )}
          </g>
        )}
        {fact != null && fact > 0 && (
          <text x={factX} y={46} fontSize={10} textAnchor="middle" fill={primary} fontWeight={600}>
            {`факт ${formatRubShort(fact)}`}
          </text>
        )}
        {forecast != null && forecast > 0 && (
          <text
            x={Math.abs(forecastX - factX) < 80 ? Math.min(Math.max(forecastX, factX) + 44, RIGHT - 70) : Math.min(forecastX, RIGHT - 70)}
            y={46} fontSize={10} textAnchor="start" fill={secondary}
          >
            {`прогноз ${formatRubShort(forecast)}`}
          </text>
        )}
        <text x={LEFT} y={46} fontSize={10} fill={muted}>{formatRubShort(origin)}</text>
      </svg>
    </Box>
  );
}
