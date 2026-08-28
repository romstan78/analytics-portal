// Сводка выбранного периода: то, ради чего форму открывают в обычный день —
// сколько обещали, сколько отгрузили, куда идёт прогноз и во что он обойдётся.

import { Box, Paper, Tooltip, Typography } from '@mui/material';
import { deltaPct, formatRub, formatRubShort, formatSignedPct } from '../utils/networkPlan';
import type { NetworkPlanTotals } from '../types/network';
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
  totals: NetworkPlanTotals;
  periodLabel: string;
}

export default function NetworkPlanSummary({ totals, periodLabel }: NetworkPlanSummaryProps) {
  const factPct = deltaPct(totals.fact_rub, totals.contract_plan_rub);
  const forecastPct = deltaPct(totals.forecast_rub, totals.contract_plan_rub);
  // Инвестиции сравниваем между собой по одной базе — до вычета НДС.
  const factInvestPct = deltaPct(totals.fact_investments_rub, totals.investments_rub);

  // Доля инвестиций в фактическом объёме — то, на что смотрят при разборе квартала.
  const factShare = totals.fact_rub > 0 ? (totals.fact_investments_rub / totals.fact_rub) * 100 : null;

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
        value={totals.contract_plan_rub}
        hint={totals.undistributed != null && totals.undistributed !== 0
          ? `валовый остаток ${formatRubShort(totals.undistributed)}`
          : 'обязательство по контракту'}
        tone={totals.undistributed != null && totals.undistributed !== 0 ? 'warn' : 'neutral'}
        accent
      />
      <SummaryCard
        label="Факт"
        value={totals.fact_rub > 0 ? totals.fact_rub : null}
        hint={factPct == null ? 'не загружен' : `${(100 + factPct).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} % плана`}
        tone={completionTone(factPct == null ? null : 100 + factPct)}
      />
      <SummaryCard
        label="Прогноз"
        value={totals.forecast_rub > 0 ? totals.forecast_rub : null}
        hint={pctHint(forecastPct, 'к плану', 'не внесён')}
        tone={deviationTone(forecastPct)}
      />
      <SummaryCard
        label="Инв. план"
        value={totals.investments_rub > 0 ? totals.investments_rub : null}
        hint={totals.investments_rub > 0 ? `без НДС ${formatRubShort(totals.investments_rub_net)}` : 'нет процента'}
      />
      <SummaryCard
		label="К выплате"
		value={totals.payable_investments_rub > 0 ? totals.payable_investments_rub : null}
		hint={totals.completed
			? 'порог периода 100% выполнен'
			: totals.payable_investments_rub > 0
				? 'есть строки с правом на выплату'
				: 'нет строк с правом на выплату'}
		tone={totals.completed ? 'good' : totals.payable_investments_rub > 0 ? 'neutral' : 'warn'}
      />
      <SummaryCard
        label="Инв. факт"
        value={totals.fact_investments_rub > 0 ? totals.fact_investments_rub : null}
        hint={factShare != null
          ? `${factShare.toLocaleString('ru-RU', { maximumFractionDigits: 2 })} % от факта`
          : pctHint(factInvestPct, 'к плану', 'не загружен')}
        tone={deviationTone(factInvestPct)}
      />
    </Box>
  );
}
