// Плитки показателей печатного отчёта — те же KpiCard, что на витрине
// «Итоги»: подпись, число, отклонение с оценкой, пояснение, спарклайн.

import { Box } from '@mui/material';
import { KpiCard } from '../NetworkDashboardParts';
import { REPORT_TONE_ACCENT, REPORT_TONE_COLOR, reportTone } from '../../utils/reportFormat';
import type { ReportPrintCard } from '../../types/reports';

export default function ReportCards({ cards, columns }: { cards: ReportPrintCard[]; columns?: number }) {
  if (cards.length === 0) return null;
  const cols = columns ?? Math.min(cards.length, 3);
  return (
    <Box
      className="report-keep"
      sx={{ display: 'grid', gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))`, gap: 1.25 }}
    >
      {cards.map((card, index) => {
        const tone = reportTone(card.deltaTone);
        return (
          <KpiCard
            key={`${index}-${card.label}`}
            label={card.label}
            primary={card.value}
            secondary={card.sub}
            accent={REPORT_TONE_ACCENT[tone]}
            trend={card.spark}
            badge={card.delta ? { text: card.delta, color: REPORT_TONE_COLOR[tone] } : undefined}
          />
        );
      })}
    </Box>
  );
}
