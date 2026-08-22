import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Alert, Box, Button, Chip, CircularProgress, IconButton, Paper, Stack,
  Table, TableBody, TableCell, TableContainer, TableFooter, TableHead, TableRow,
  TextField, ToggleButton, ToggleButtonGroup, Tooltip, Typography,
} from '@mui/material';
import {
  ChevronRight as ChevronRightIcon,
  ExpandMore as ExpandMoreIcon,
  FileDownload as ExportIcon,
  Search as SearchIcon,
  UnfoldLess as CollapseIcon,
  UnfoldMore as ExpandIcon,
} from '@mui/icons-material';
import { saveAs } from 'file-saver';
import { salesAPI } from '../api/promo';
import type { SalesPivotNode } from '../types/sales';
import {
  allSalesPivotExpansion,
  defaultSalesPivotExpansion,
  filterSalesPivotTree,
  flattenSalesPivotTree,
  salesPivotComparison,
} from '../utils/salesPivot';

export type SalesPivotGranularity = 'year' | 'quarter' | 'month';

interface InternetSalesSummaryTableProps {
  analysisYear: string;
  filters: Record<string, unknown>;
  channel: string;
  segments: string[];
  unit: 'руб' | 'евро' | 'уп';
  granularity: SalesPivotGranularity;
  onGranularityChange: (value: SalesPivotGranularity) => void;
}

const levelBackground: Record<string, string> = {
  channel: '#eef0ff',
  segment: '#f5f6ff',
  network: '#fafbfe',
  sku: '#ffffff',
};

export default function InternetSalesSummaryTable({
  analysisYear, filters, channel, segments, unit, granularity, onGranularityChange,
}: InternetSalesSummaryTableProps) {
  const [search, setSearch] = useState('');
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [expandedRows, setExpandedRows] = useState<SalesPivotNode[] | null>(null);
  const [exporting, setExporting] = useState(false);

  const pivotParams = useMemo(() => ({
    ...filters,
    analysisYear,
    focusChannel: channel,
    focusSegments: segments,
    unit,
    granularity,
  }), [analysisYear, channel, filters, granularity, segments, unit]);

  const { data, isFetching, error, refetch } = useQuery({
    queryKey: ['salesPivot', pivotParams] as const,
    enabled: Boolean(analysisYear && channel && segments.length),
    queryFn: () => salesAPI.getPivot(pivotParams),
    placeholderData: previousData => previousData,
  });

  // При новом ответе раскрываем канал и сегменты, чтобы сети были видны сразу,
  // а SKU оставались сгруппированы внутри сети.
  if ((data?.rows || null) !== expandedRows) {
    setExpandedRows(data?.rows || null);
    setExpanded(defaultSalesPivotExpansion(data?.rows || []));
  }

  const filteredTree = useMemo(() => filterSalesPivotTree(data?.rows || [], search), [data?.rows, search]);
  const flatRows = useMemo(
    () => flattenSalesPivotTree(filteredTree, expanded, Boolean(search.trim())),
    [expanded, filteredTree, search],
  );
  const yearGroups = useMemo(() => {
    const result: Array<{ year: number; count: number }> = [];
    (data?.periods || []).forEach(period => {
      const last = result.at(-1);
      if (last?.year === period.year) last.count++;
      else result.push({ year: period.year, count: 1 });
    });
    return result;
  }, [data?.periods]);

  const formatNumber = (value: number) => {
    if (!value) return '—';
    return value.toLocaleString('ru-RU', {
      minimumFractionDigits: unit === 'евро' ? 2 : 0,
      maximumFractionDigits: unit === 'евро' ? 2 : 0,
    });
  };
  const unitLabel = unit === 'руб' ? '₽' : unit === 'евро' ? '€' : 'шт.';

  const toggleRow = (node: SalesPivotNode) => {
    if (!node.children?.length) return;
    setExpanded(current => {
      const next = new Set(current);
      if (next.has(node.id)) next.delete(node.id);
      else next.add(node.id);
      return next;
    });
  };

  const exportExcel = async () => {
    setExporting(true);
    try {
      const blob = await salesAPI.exportPivot(pivotParams);
      saveAs(blob, `internet-sales-pivot-${analysisYear}-${granularity}-${new Date().toISOString().slice(0, 10)}.xlsx`);
    } catch (exportError) {
      window.alert(exportError instanceof Error ? exportError.message : 'Не удалось выгрузить сводную таблицу.');
    } finally {
      setExporting(false);
    }
  };

  const comparisonCells = (values: Record<string, number>, emphasized = false) => {
    if (!data) return null;
    const comparison = salesPivotComparison(values, data.previousTotalKey, data.currentTotalKey);
    const color = comparison.delta > 0 ? '#12805c' : comparison.delta < 0 ? '#c14545' : '#64748b';
    return (
      <>
        <TableCell align="center" sx={{ fontWeight: emphasized ? 750 : 650, color, whiteSpace: 'nowrap' }}>
          {comparison.delta > 0 ? '+' : ''}{formatNumber(comparison.delta)}
        </TableCell>
        <TableCell align="center" sx={{ fontWeight: emphasized ? 750 : 650, color, whiteSpace: 'nowrap' }}>
          {comparison.yoy == null ? '—' : `${comparison.yoy > 0 ? '+' : ''}${comparison.yoy.toFixed(1)}%`}
        </TableCell>
      </>
    );
  };

  if (!data && isFetching) {
    return <Box sx={{ flex: 1, display: 'grid', placeItems: 'center' }}><CircularProgress /></Box>;
  }
  if (!data && error) {
    return <Alert severity="error" action={<Button color="inherit" size="small" onClick={() => refetch()}>Повторить</Button>}>
      {error instanceof Error ? error.message : 'Не удалось загрузить сводную таблицу'}
    </Alert>;
  }
  if (!data) return <Alert severity="info">Нет данных для сводной таблицы.</Alert>;

  const minimumWidth = 390 + data.periods.length * 118 + 240;

  return (
    <Paper variant="outlined" sx={{ flex: 1, minHeight: 0, borderRadius: 3, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      <Stack direction={{ xs: 'column', lg: 'row' }} spacing={1.25} sx={{ alignItems: { xs: 'stretch', lg: 'center' }, p: 1.5, borderBottom: '1px solid #e2e8f0' }}>
        <Box sx={{ minWidth: 210 }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 750 }}>Сводная интернет-продаж</Typography>
          <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center', flexWrap: 'wrap', rowGap: 0.5 }}>
            <Chip size="small" variant="outlined" color="primary" label={data.channel || channel} />
            {data.segments.map(segment => <Chip key={segment} size="small" variant="outlined" label={segment} />)}
            <Typography variant="caption" color="text.secondary">{data.leafRows.toLocaleString('ru-RU')} SKU-связей</Typography>
          </Stack>
        </Box>
        <ToggleButtonGroup size="small" exclusive value={granularity} onChange={(_, value) => value && onGranularityChange(value)}>
          <ToggleButton value="year">Год</ToggleButton>
          <ToggleButton value="quarter">Квартал</ToggleButton>
          <ToggleButton value="month">Месяц</ToggleButton>
        </ToggleButtonGroup>
        <TextField
          size="small"
          placeholder="Найти сеть или SKU..."
          value={search}
          onChange={event => setSearch(event.target.value)}
          slotProps={{ input: { startAdornment: <SearchIcon sx={{ fontSize: 18, color: '#94a3b8', mr: 0.5 }} /> } }}
          sx={{ width: { xs: '100%', lg: 260 } }}
        />
        <Tooltip title="Раскрыть все уровни, включая SKU">
          <IconButton size="small" onClick={() => setExpanded(allSalesPivotExpansion(data.rows))}><ExpandIcon /></IconButton>
        </Tooltip>
        <Tooltip title="Свернуть до каналов и сегментов">
          <IconButton size="small" onClick={() => setExpanded(defaultSalesPivotExpansion(data.rows))}><CollapseIcon /></IconButton>
        </Tooltip>
        <Box sx={{ flex: 1 }} />
        {isFetching && <CircularProgress size={18} />}
        <Tooltip title="Выгружаются все строки по фильтрам, включая свернутые SKU">
          <span>
            <Button size="small" startIcon={exporting ? <CircularProgress size={15} /> : <ExportIcon />} onClick={exportExcel} disabled={exporting || !data.rows.length}>
              Excel
            </Button>
          </span>
        </Tooltip>
      </Stack>

      {error && (
        <Alert severity="warning" sx={{ borderRadius: 0 }} action={<Button color="inherit" size="small" onClick={() => refetch()}>Повторить</Button>}>
          Показаны последние загруженные данные. Обновление не удалось.
        </Alert>
      )}

      {!data.rows.length ? (
        <Alert severity="info" sx={{ m: 1.5 }}>По выбранным фильтрам данных нет.</Alert>
      ) : (
        <TableContainer sx={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
          <Table stickyHeader size="small" sx={{ minWidth: minimumWidth, tableLayout: 'fixed', '& td, & th': { height: 44, verticalAlign: 'middle' } }}>
            <TableHead>
              <TableRow>
                <TableCell rowSpan={2} sx={{ width: 390, minWidth: 390, position: 'sticky', left: 0, zIndex: 5, bgcolor: '#eef1f7', fontWeight: 750 }}>
                  Канал / сегмент / сеть / SKU
                </TableCell>
                {yearGroups.map(group => (
                  <TableCell key={group.year} align="center" colSpan={group.count} sx={{ bgcolor: '#e8eaf8', fontWeight: 750, borderLeft: '1px solid #cbd5e1' }}>
                    {group.year} · {unitLabel}
                  </TableCell>
                ))}
                <TableCell rowSpan={2} align="center" sx={{ width: 135, bgcolor: '#eef1f7', fontWeight: 750 }}>Отклонение</TableCell>
                <TableCell rowSpan={2} align="center" sx={{ width: 105, bgcolor: '#eef1f7', fontWeight: 750 }}>YoY</TableCell>
              </TableRow>
              <TableRow>
                {data.periods.map(period => (
                  <TableCell key={period.key} align="center" sx={{ width: 118, bgcolor: period.kind === 'total' ? '#e9edf5' : '#f5f7fb', fontWeight: period.kind === 'total' ? 750 : 650, whiteSpace: 'nowrap' }}>
                    {period.label}
                  </TableCell>
                ))}
              </TableRow>
            </TableHead>
            <TableBody>
              {flatRows.map(({ node, depth }) => {
                const hasChildren = Boolean(node.children?.length);
                const isExpanded = expanded.has(node.id) || Boolean(search.trim());
                const background = levelBackground[node.level] || '#fff';
                return (
                  <TableRow key={node.id} hover sx={{ bgcolor: background }}>
                    <TableCell sx={{ position: 'sticky', left: 0, zIndex: 2, bgcolor: background, pl: 1 + depth * 2.5, fontWeight: node.level === 'sku' ? 450 : 700, borderRight: '1px solid #e2e8f0' }}>
                      <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center', minWidth: 0 }}>
                        {hasChildren ? (
                          <IconButton size="small" onClick={() => toggleRow(node)} sx={{ p: 0.25 }}>
                            {isExpanded ? <ExpandMoreIcon fontSize="small" /> : <ChevronRightIcon fontSize="small" />}
                          </IconButton>
                        ) : <Box sx={{ width: 29 }} />}
                        <Typography variant="body2" noWrap title={node.name} sx={{ fontWeight: 'inherit' }}>{node.name}</Typography>
                      </Stack>
                    </TableCell>
                    {data.periods.map(period => (
                      <TableCell key={period.key} align="center" sx={{ fontVariantNumeric: 'tabular-nums', fontWeight: period.kind === 'total' || node.level !== 'sku' ? 650 : 450, bgcolor: period.kind === 'total' ? 'rgba(226,232,240,0.28)' : undefined, whiteSpace: 'nowrap' }}>
                        {formatNumber(node.values[period.key] || 0)}
                      </TableCell>
                    ))}
                    {comparisonCells(node.values)}
                  </TableRow>
                );
              })}
            </TableBody>
            <TableFooter sx={{ position: 'sticky', bottom: 0, zIndex: 4 }}>
              <TableRow sx={{ '& td': { bgcolor: '#dde3f0', borderTop: '2px solid #94a3b8', fontWeight: 800 } }}>
                <TableCell sx={{ position: 'sticky', left: 0, zIndex: 5 }}>ИТОГО</TableCell>
                {data.periods.map(period => (
                  <TableCell key={period.key} align="center" sx={{ fontVariantNumeric: 'tabular-nums', whiteSpace: 'nowrap' }}>
                    {formatNumber(data.totals[period.key] || 0)}
                  </TableCell>
                ))}
                {comparisonCells(data.totals, true)}
              </TableRow>
            </TableFooter>
          </Table>
        </TableContainer>
      )}
    </Paper>
  );
}
