// Сводка выбранного периода: то, ради чего форму открывают в обычный день —
// сколько обещали, сколько отгрузили, куда идёт прогноз и во что он обойдётся.

import { Box, Paper, Tooltip, Typography } from '@mui/material';
import { deltaPct, formatRub, formatRubShort, formatSignedPct } from '../utils/networkPlan';
import type { QuarterTotals } from '../utils/networkPlan';
import { TONE_COLOR, completionTone, deviationTone } from '../utils/networkPlanView';

interface CardProps {
  label: string;
  value: number | null;
  hint?: string | null;
  tone?: keyof typeof TONE_COLOR;
  accent?: boolean;
}

function SummaryCard({ label, value, hint, tone = 'neutral', accent }: CardProps) {
  return (
    <Paper
      variant="outlined"
      sx={{
        p: 1.5,
        borderColor: accent ? 'primary.main' : undefined,
        minWidth: 0,
      }}
    >
      <Typography variant="caption" color="text.secondary" noWrap sx={{ display: 'block' }}>
        {label}
      </Typography>
      <Tooltip title={value == null ? '' : `${formatRub(value, 2)} ₽`} placement="top">
        <Typography variant="h6" sx={{ fontVariantNumeric: 'tabular-nums', lineHeight: 1.35 }}>
          {formatRubShort(value)}
        </Typography>
      </Tooltip>
      <Typography variant="caption" sx={{ color: TONE_COLOR[tone], display: 'block', minHeight: 18 }}>
        {hint ?? ''}
      </Typography>
    </Paper>
  );
}

interface NetworkPlanSummaryProps {
  totals: QuarterTotals;
  periodLabel: string;
}

export default function NetworkPlanSummary({ totals, periodLabel }: NetworkPlanSummaryProps) {
  const factPct = deltaPct(totals.factRub, totals.contractPlanRub);
  const forecastPct = deltaPct(totals.forecastRub, totals.contractPlanRub);
  // Инвестиции сравниваем между собой по одной базе — до вычета НДС.
  const forecastInvestPct = deltaPct(totals.forecastInvestmentsRub, totals.investmentsRub);
  const factInvestPct = deltaPct(totals.factInvestmentsRub, totals.investmentsRub);

  // Доля инвестиций в фактическом объёме — то, на что смотрят при разборе квартала.
  const factShare = totals.factRub > 0 ? (totals.factInvestmentsRub / totals.factRub) * 100 : null;

  const pctHint = (value: number | null, suffix: string, empty: string) => {
    if (value == null) return empty;
    // Ровно по плану — это отдельный факт, а не «+0 %».
    return value === 0 ? 'как в плане' : `${formatSignedPct(value)} ${suffix}`;
  };

  return (
    <Box
      sx={{
        display: 'grid',
        // Три объёма и три инвестиции: пары «план / факт / прогноз» читаются
        // по столбцам, когда карточки встают в два ряда по три.
        gridTemplateColumns: { xs: 'repeat(2, 1fr)', sm: 'repeat(3, 1fr)', lg: 'repeat(6, 1fr)' },
        gap: 1.5,
      }}
    >
      <SummaryCard
        label={`План · ${periodLabel}`}
        value={totals.contractPlanRub}
        hint={totals.undistributed != null && totals.undistributed !== 0
          ? `валовый остаток ${formatRubShort(totals.undistributed)}`
          : 'обязательство по контракту'}
        tone={totals.undistributed != null && totals.undistributed !== 0 ? 'warn' : 'neutral'}
        accent
      />
      <SummaryCard
        label="Факт"
        value={totals.factRub > 0 ? totals.factRub : null}
        hint={factPct == null ? 'не загружен' : `${(100 + factPct).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} % плана`}
        tone={completionTone(factPct == null ? null : 100 + factPct)}
      />
      <SummaryCard
        label="Прогноз"
        value={totals.forecastRub > 0 ? totals.forecastRub : null}
        hint={pctHint(forecastPct, 'к плану', 'не внесён')}
        tone={deviationTone(forecastPct)}
      />
      <SummaryCard
        label="Инв. план"
        value={totals.investmentsRub > 0 ? totals.investmentsRub : null}
        hint={totals.investmentsRub > 0 ? `без НДС ${formatRubShort(totals.investmentsRubNet)}` : 'нет процента'}
      />
      <SummaryCard
        label="Инв. прогноз"
        value={totals.forecastInvestmentsRub > 0 ? totals.forecastInvestmentsRub : null}
        hint={pctHint(forecastInvestPct, 'к плану', 'не внесён')}
        tone={deviationTone(forecastInvestPct)}
      />
      <SummaryCard
        label="Инв. факт"
        value={totals.factInvestmentsRub > 0 ? totals.factInvestmentsRub : null}
        hint={factShare != null
          ? `${factShare.toLocaleString('ru-RU', { maximumFractionDigits: 2 })} % от факта`
          : pctHint(factInvestPct, 'к плану', 'не загружен')}
        tone={deviationTone(factInvestPct)}
      />
    </Box>
  );
}
