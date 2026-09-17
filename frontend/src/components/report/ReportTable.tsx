// Таблица печатного отчёта: простая <table> в стиле таблиц витрины — без
// DataGrid и виртуализации, которые в печати не работают. Ячейки приходят
// готовыми: текст, оценка цветом, доля полосы, жирность.

import { Paper, Table, TableBody, TableCell, TableHead, TableRow } from '@mui/material';
import { BORDER, SERIES_PLAN } from '../../utils/networkDashboard';
import { REPORT_TONE_COLOR, reportTone } from '../../utils/reportFormat';
import type { ReportPrintCell, ReportPrintColumn, ReportPrintTable } from '../../types/reports';

const HEAD_SX = { fontWeight: 750, fontSize: 11, lineHeight: 1.25, py: 0.6, px: 0.75, color: 'text.secondary', verticalAlign: 'bottom' } as const;
const CELL_SX = { fontSize: 11.5, lineHeight: 1.3, py: 0.45, px: 0.75, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' } as const;

// Узкие колонки («#», «Кв.») сервер задаёт малым весом; ниже этой доли номер
// строки «20» уже не помещается.
const MIN_WEIGHT = 0.45;
const columnWeight = (column: ReportPrintColumn) => Math.max(column.weight > 0 ? column.weight : 1, MIN_WEIGHT);

export default function ReportTable({ table }: { table: ReportPrintTable }) {
  const totalWeight = table.columns.reduce((sum, c) => sum + columnWeight(c), 0);
  return (
    <Paper variant="outlined" className="report-table-paper" sx={{ borderRadius: 3, borderColor: BORDER, overflow: 'hidden' }}>
      <Table
        size="small"
        className="report-table"
        sx={{
          tableLayout: 'fixed', width: '100%',
          // Крайние ячейки отступают от скруглённого края карточки.
          '& th:first-of-type, & td:first-of-type': { pl: 1.5 },
          '& th:last-of-type, & td:last-of-type': { pr: 1.5 },
        }}
      >
        <colgroup>
          {table.columns.map((column, index) => (
            <col key={index} style={{ width: `${(columnWeight(column) / totalWeight) * 100}%` }} />
          ))}
        </colgroup>
        <TableHead>
          <TableRow>
            {table.columns.map((column, index) => (
              <TableCell key={index} align={column.right ? 'right' : 'left'} sx={HEAD_SX}>{column.title}</TableCell>
            ))}
          </TableRow>
        </TableHead>
        <TableBody>
          {table.rows.map((row, rowIndex) => (
            <TableRow key={rowIndex} sx={{ '&:nth-of-type(even) td': { bgcolor: '#f8fafc' } }}>
              {row.map((cell, cellIndex) => (
                <ReportCellView key={cellIndex} cell={cell} column={table.columns[cellIndex]} />
              ))}
            </TableRow>
          ))}
          {table.total && table.total.length > 0 && (
            <TableRow sx={{ '& td': { borderTop: `2px solid ${BORDER}`, fontWeight: 750 } }}>
              {table.total.map((cell, cellIndex) => (
                <ReportCellView key={cellIndex} cell={{ ...cell, bold: true }} column={table.columns[cellIndex]} />
              ))}
            </TableRow>
          )}
        </TableBody>
      </Table>
    </Paper>
  );
}

// Полоса в ячейке — доля выполнения фоном: до 100 % светлым тоном плана,
// сверх — упирается в правый край.
function ReportCellView({ cell, column }: { cell: ReportPrintCell; column?: ReportPrintColumn }) {
  const tone = reportTone(cell.tone);
  const bar = cell.bar == null ? null : Math.max(0, Math.min(1, cell.bar));
  return (
    <TableCell
      align={column?.right ? 'right' : 'left'}
      title={cell.text}
      sx={{
        ...CELL_SX,
        fontWeight: cell.bold ? 750 : 500,
        color: tone === 'neutral' ? 'text.primary' : REPORT_TONE_COLOR[tone],
        fontVariantNumeric: column?.right ? 'tabular-nums' : undefined,
        ...(bar != null ? {
          backgroundImage: `linear-gradient(to right, ${SERIES_PLAN}22 ${bar * 100}%, transparent ${bar * 100}%)`,
        } : null),
      }}
    >
      {cell.text}
    </TableCell>
  );
}
