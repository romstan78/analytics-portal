// Документ печатного отчёта: секции по блокам и колонтитул.
//
// Колонтитул на экране стоит один раз под документом. В печати он идёт в
// поля листа через @page-боксы: только там есть счётчик страниц, а
// position: fixed Chromium при печати кладёт поверх содержимого. Текст
// колонтитула — из данных, поэтому правило @page собирается здесь, а не в
// print.css.

import { Box } from '@mui/material';
import ReportSection from './ReportSection';
import type { ReportPrint } from '../../types/reports';

// Строка CSS: кавычки и обратные слэши экранируются, управляющие символы
// убираются — в content они не нужны, а перевод строки CSS не примет.
function cssString(text: string): string {
  const clean = Array.from(text, (ch) => (ch.charCodeAt(0) < 0x20 ? ' ' : ch)).join('');
  return `"${clean.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`;
}

export default function ReportDocument({ report }: { report: ReportPrint }) {
  const period = report.filterLabels[0] ?? '';
  const left = [report.title, period].filter(Boolean).join(' · ');
  const right = `${report.owner} · ${report.createdAt}`;
  return (
    <Box className="report-root">
      <style>{`@page {
  @bottom-left { content: ${cssString(left)}; }
  @bottom-center { content: ${cssString(right)}; }
}`}</style>
      {report.pages.map((page, index) => (
        <ReportSection key={`${page.block}-${index}`} page={page} index={index} total={report.pages.length} />
      ))}
      <Box component="footer" className="report-footer">
        <span>{left}</span>
        <span>{right}</span>
      </Box>
    </Box>
  );
}
