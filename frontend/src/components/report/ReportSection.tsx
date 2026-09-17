// Секция печатного отчёта — один блок: страница PDF и слайд PPTX.
//
// Разметка одна для всех блоков: заголовок, плитки, график, таблица,
// примечания; какие части есть — решил сервер. Титул и методика — текстовые
// блоки без плиток-графиков, кроме трёх главных плиток на титуле.
//
// [data-slide] помечает секцию, [data-slide-image] — её графическую часть:
// рендер PPTX снимает только её, таблицы кладёт нативно.

import { Box, Stack, Typography } from '@mui/material';
import ReportCards from './ReportCards';
import ReportChart from './ReportChart';
import ReportTable from './ReportTable';
import { BORDER, SERIES_PLAN } from '../../utils/networkDashboard';
import type { ReportPrintPage } from '../../types/reports';

export default function ReportSection({ page, index, total }: {
  page: ReportPrintPage;
  index: number;
  total: number;
}) {
  const isCover = page.block === 'cover';
  const hasVisual = (page.cards && page.cards.length > 0) || page.chart != null;
  return (
    <Box component="section" className="report-section" data-slide={page.block} data-slide-index={index}>
      {isCover ? (
        <CoverHeader page={page} />
      ) : (
        <Stack direction="row" sx={{ alignItems: 'baseline', justifyContent: 'space-between', gap: 2, mb: 1.25 }}>
          <Box>
            <Typography variant="h5" sx={{ fontWeight: 700 }}>{page.title}</Typography>
            {page.subtitle && <Typography variant="body2" color="text.secondary">{page.subtitle}</Typography>}
          </Box>
          <Typography variant="caption" color="text.secondary" sx={{ whiteSpace: 'nowrap' }}>
            {index + 1} / {total}
          </Typography>
        </Stack>
      )}

      {page.lines && page.lines.length > 0 && !isCover && (
        <Stack spacing={1} sx={{ maxWidth: 900, mb: 1.5 }}>
          {page.lines.map((line, i) => (
            <Typography key={i} variant="body2" sx={{ lineHeight: 1.55 }}>{line}</Typography>
          ))}
        </Stack>
      )}

      {hasVisual && (
        <Stack spacing={1.25} data-slide-image sx={{ mb: page.table ? 1.25 : 0 }}>
          {page.cards && page.cards.length > 0 && (
            <ReportCards cards={page.cards} columns={isCover ? 3 : undefined} />
          )}
          {page.chart && <ReportChart chart={page.chart} />}
        </Stack>
      )}

      {page.table && <ReportTable table={page.table} />}

      {page.notes && page.notes.length > 0 && (
        <Stack spacing={0.25} sx={{ mt: 1.25, pt: 0.75, borderTop: `1px dashed ${BORDER}` }}>
          {page.notes.map((note, i) => (
            <Typography key={i} variant="caption" color="text.secondary" sx={{ display: 'block', lineHeight: 1.4 }}>
              {note}
            </Typography>
          ))}
        </Stack>
      )}
    </Box>
  );
}

// Титул: название крупно, период, абзацы области и автора; плитки — ниже,
// общим блоком.
function CoverHeader({ page }: { page: ReportPrintPage }) {
  return (
    <Box sx={{ pt: 6, pb: 4, mb: 3, borderBottom: `1px solid ${BORDER}` }}>
      <Box sx={{ width: 56, height: 5, borderRadius: 3, bgcolor: SERIES_PLAN, mb: 2.5 }} />
      <Typography variant="h3" sx={{ fontWeight: 700, letterSpacing: '-0.02em', maxWidth: 900 }}>{page.title}</Typography>
      {page.subtitle && (
        <Typography variant="h6" color="text.secondary" sx={{ mt: 1, fontWeight: 500 }}>{page.subtitle}</Typography>
      )}
      {page.lines && page.lines.length > 0 && (
        <Stack spacing={0.5} sx={{ mt: 2.5, maxWidth: 900 }}>
          {page.lines.map((line, i) => (
            <Typography key={i} variant="body2" color="text.secondary">{line}</Typography>
          ))}
        </Stack>
      )}
    </Box>
  );
}
