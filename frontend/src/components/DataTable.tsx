import { useState, useEffect, useMemo, useRef, type MouseEvent } from 'react';
import { useQuery } from '@tanstack/react-query';
import { DataGrid, type GridColDef, type GridPaginationModel, type GridRowParams } from '@mui/x-data-grid';
import {
  Box, Alert, TextField, Button, Menu, MenuItem,
  Checkbox, ListItemText, Typography, Divider, ButtonGroup,
} from '@mui/material';
import {
  ViewColumn as ColumnsIcon,
  FileDownload as ExportIcon,
  Search as SearchIcon,
} from '@mui/icons-material';
import { saveAs } from 'file-saver';

const EXPORT_WARNING_THRESHOLD = 10000;

interface DataTableProps {
  columns: GridColDef[];
  apiUrl: string;
  filters?: Record<string, unknown>;
  defaultPageSize?: number;
  exportFileName?: string;
  exportXlsxUrl?: string;
  onDataLoaded?: (data: unknown[]) => void;
  onRowClick?: (params: GridRowParams) => void;
  refreshKey?: number;
}

export default function DataTable({
  columns, apiUrl, filters = {}, defaultPageSize = 100,
  exportFileName = 'export', exportXlsxUrl, onDataLoaded,
  onRowClick, refreshKey,
}: DataTableProps) {
  const filtersKey = useMemo(() => JSON.stringify(filters), [filters]);

  // Серверная пагинация
  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: defaultPageSize,
  });

  // Тулбар
  const [searchText, setSearchText] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [columnsAnchor, setColumnsAnchor] = useState<HTMLElement | null>(null);
  const [visibleColumns, setVisibleColumns] = useState<Record<string, boolean>>(() => {
    const map: Record<string, boolean> = {};
    columns.forEach(c => { map[c.field] = true; });
    return map;
  });
  const apiRef = useRef(null);
  // Индикатор выгрузки: раньше переиспользовался loading от загрузки страницы.
  const [exporting, setExporting] = useState(false);

  // Сброс страницы при смене фильтров или поискового запроса. Правка состояния
  // во время рендера — рекомендованная React альтернатива эффекту-синхронизатору.
  const resetKey = `${filtersKey}|${debouncedSearch}`;
  const [prevResetKey, setPrevResetKey] = useState(resetKey);
  if (prevResetKey !== resetKey) {
    setPrevResetKey(resetKey);
    if (paginationModel.page !== 0) setPaginationModel(prev => ({ ...prev, page: 0 }));
  }

  // Debounce поиска (400ms)
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchText.trim());
    }, 400);
    return () => clearTimeout(timer);
  }, [searchText]);

  const visibleCols = useMemo(
    () => columns.filter(c => visibleColumns[c.field] !== false),
    [columns, visibleColumns]
  );

  const toggleColumn = (field: string) => {
    setVisibleColumns(prev => ({ ...prev, [field]: !prev[field] }));
  };

  const showAllColumns = () => {
    const map: Record<string, boolean> = {};
    columns.forEach(c => { map[c.field] = true; });
    setVisibleColumns(map);
  };

  const hideAllColumns = () => {
    const map: Record<string, boolean> = {};
    columns.forEach(c => { map[c.field] = false; });
    setVisibleColumns(map);
  };

  const fetchExportData = async (): Promise<unknown[]> => {
    const params = new URLSearchParams();
    params.set('all', 'true');
    if (debouncedSearch) {
      params.set('search', debouncedSearch);
    }
    Object.entries(filters).forEach(([key, value]) => {
      if (Array.isArray(value)) {
        value.forEach(v => { if (v !== '' && v != null) params.append(key, String(v)); });
      } else if (value !== '' && value != null) {
        params.set(key, String(value));
      }
    });
    const url = `${apiUrl}?${params.toString()}`;
    const response = await fetch(url, {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`,
      },
    });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const json = await response.json();
    return (json.data || []) as unknown[];
  };

  const handleExportCSV = async () => {
    setExporting(true);
    try {
      const data = await fetchExportData();
      if (!data.length) {
        window.alert('Нет данных для выгрузки.');
        return;
      }
      if (data.length > EXPORT_WARNING_THRESHOLD) {
        const ok = window.confirm(
          `Будет выгружено более ${EXPORT_WARNING_THRESHOLD.toLocaleString('ru-RU')} строк (${data.length.toLocaleString('ru-RU')}). Продолжить?`
        );
        if (!ok) return;
      }
      const headers = visibleCols.map(c => c.headerName || c.field);
      const fields = visibleCols.map(c => c.field);

      let csv = '\uFEFF' + headers.join(';') + '\n';
      data.forEach(row => {
        const record = row as Record<string, unknown>;
        const line = fields.map(f => {
          const raw = record[f];
          if (raw == null) return '';
          let val = String(raw);
          if (val.includes(';') || val.includes('"') || val.includes('\n')) {
            val = '"' + val.replace(/"/g, '""') + '"';
          }
          return val;
        }).join(';');
        csv += line + '\n';
      });

      const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
      saveAs(blob, `${exportFileName}_${new Date().toISOString().split('T')[0]}.csv`);
    } catch (err) {
      console.error('Ошибка экспорта CSV:', err);
      window.alert('Ошибка при выгрузке CSV.');
    } finally {
      setExporting(false);
    }
  };

  // Выгрузка в Excel через серверный эндпоинт
  const handleExportXLSX = async () => {
    if (!exportXlsxUrl) {
      window.alert('Экспорт в Excel не настроен для этой страницы.');
      return;
    }
    setExporting(true);
    try {
      const data = await fetchExportData();
      const count = Array.isArray(data) ? data.length : 0;
      if (!count) {
        window.alert('Нет данных для выгрузки.');
        return;
      }
      if (count > EXPORT_WARNING_THRESHOLD) {
        const ok = window.confirm(
          `Будет выгружено более ${EXPORT_WARNING_THRESHOLD.toLocaleString('ru-RU')} строк (${count.toLocaleString('ru-RU')}). Продолжить?`
        );
        if (!ok) return;
      }
      const params = new URLSearchParams();
      params.set('all', 'true');
      if (debouncedSearch) params.set('search', debouncedSearch);
      Object.entries(filters).forEach(([key, value]) => {
        if (Array.isArray(value)) {
          value.forEach(v => { if (v !== '' && v != null) params.append(key, String(v)); });
        } else if (value !== '' && value != null) {
          params.set(key, String(value));
        }
      });
      const resp = await fetch(`${exportXlsxUrl}?${params.toString()}`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
      });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const blob = await resp.blob();
      saveAs(blob, `${exportFileName}_${new Date().toISOString().split('T')[0]}.xlsx`);
    } catch (err) {
      console.error('Ошибка экспорта Excel:', err);
      window.alert('Ошибка при выгрузке Excel.');
    } finally {
      setExporting(false);
    }
  };

  // Загрузка данных с пагинацией через React Query: запрос перезапускается
  // при изменении любой части ключа.
  const pageQuery = useQuery({
    queryKey: ['dataTable', apiUrl, filtersKey, paginationModel.page, paginationModel.pageSize, debouncedSearch, refreshKey] as const,
    queryFn: async ({ signal }) => {
      const params = new URLSearchParams();
      params.set('page', String(paginationModel.page));
      params.set('pageSize', String(paginationModel.pageSize));
      if (debouncedSearch) {
        params.set('search', debouncedSearch);
      }

      Object.entries(filters).forEach(([key, value]) => {
        if (Array.isArray(value)) {
          value.forEach(v => { if (v !== '' && v != null) params.append(key, String(v)); });
        } else if (value !== '' && value != null) {
          params.set(key, String(value));
        }
      });
      const qs = params.toString();
      const url = `${apiUrl}?${qs}`;
      const response = await fetch(url, {
        signal,
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const json = await response.json() as { data?: unknown[]; totalRows?: number };
      const rows = json.data || [];
      return { rows, totalRows: json.totalRows || rows.length };
    },
  });

  const { data: pageData, isFetching, error: queryError } = pageQuery;

  const totalRows = pageData?.totalRows ?? 0;
  const loading = isFetching || exporting;

  // Уникальные ключи строк
  const rows = useMemo(() => {
    return (pageData?.rows ?? []).map((row, idx) => ({
      ...(row as Record<string, unknown>),
      _rowId: `${(row as Record<string, unknown>).id ?? 'row'}_${paginationModel.page}_${idx}`,
    }));
  }, [pageData, paginationModel.page]);

  // onDataLoaded — побочный эффект для родителя, вызываем после успешной загрузки.
  useEffect(() => {
    if (pageData && onDataLoaded) onDataLoaded(pageData.rows);
  }, [pageData, onDataLoaded]);

  // Сетевой сбой переводит запрос в paused без ошибки — показываем отдельно.
  const connectionPaused = pageQuery.fetchStatus === 'paused' && pageQuery.failureCount > 0;
  const error = queryError
    ? (queryError as Error).message
    : (connectionPaused ? 'Нет связи с сервером' : null);

  if (error) return <Alert severity="error" sx={{ mb: 2 }}>Ошибка загрузки: {error}</Alert>;

  return (
    <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* Тулбар */}
      <Box sx={{
        display: 'flex', alignItems: 'center', gap: 1,
        px: 2, py: 1, bgcolor: '#f1f5f9',
        borderRadius: '12px 12px 0 0',
        border: '1px solid #e2e8f0', borderBottom: 'none',
      }}>
        <Button size="small" startIcon={<ColumnsIcon />}
          onClick={(e: MouseEvent<HTMLButtonElement>) => setColumnsAnchor(e.currentTarget)}
          sx={{ color: '#475569', fontWeight: 500 }}>Колонки</Button>
        <Menu anchorEl={columnsAnchor} open={Boolean(columnsAnchor)}
          onClose={() => setColumnsAnchor(null)}
          slotProps={{ paper: { sx: { maxHeight: 400, minWidth: 220 } } }}>
          <MenuItem dense onClick={showAllColumns}>
            <Typography variant="caption" color="primary" sx={{ fontWeight: 600 }}>Показать все</Typography></MenuItem>
          <MenuItem dense onClick={hideAllColumns}>
            <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>Скрыть все</Typography></MenuItem>
          <Divider />
          {columns.map(col => (
            <MenuItem key={col.field} dense onClick={() => toggleColumn(col.field)}>
              <Checkbox size="small" checked={visibleColumns[col.field] !== false} />
              <ListItemText primary={col.headerName || col.field} slotProps={{ primary: { sx: { fontSize: 13 } } }} />
            </MenuItem>
          ))}
        </Menu>

        <TextField size="small" placeholder="Поиск..." value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          slotProps={{ input: { startAdornment: <SearchIcon sx={{ fontSize: 18, color: '#94a3b8', mr: 0.5 }} /> } }}
          sx={{ width: 240, '& .MuiOutlinedInput-root': { bgcolor: '#fff', borderRadius: 2 }, '& .MuiInputBase-input': { fontSize: '0.875rem', py: 0.75 } }} />

        <Box sx={{ flex: 1 }} />

        {totalRows > 0 && (
          <Typography variant="caption" color="text.secondary" sx={{ mr: 1 }}>
            {totalRows.toLocaleString('ru-RU')} строк
          </Typography>
        )}

        <ButtonGroup size="small" variant="text">
          <Button startIcon={<ExportIcon />} onClick={handleExportCSV}
            sx={{ color: '#475569', fontWeight: 500 }}>CSV</Button>
          {exportXlsxUrl && (
            <Button startIcon={<ExportIcon />} onClick={handleExportXLSX}
              sx={{ color: '#475569', fontWeight: 500 }}>Excel</Button>
          )}
        </ButtonGroup>
      </Box>

      {/* Таблица */}
      <DataGrid
        apiRef={apiRef}
        rows={rows}
        columns={visibleCols}
        getRowId={(row) => row._rowId as string}
        loading={loading}
        sortingMode="server"
        paginationMode="server"
        rowCount={totalRows}
        paginationModel={paginationModel}
        onPaginationModelChange={setPaginationModel}
        disableColumnFilter
        onRowClick={onRowClick}
        pageSizeOptions={[25, 50, 100]}
        disableRowSelectionOnClick
        sx={{
          flex: 1, border: '1px solid #e2e8f0', borderTop: 'none',
          borderRadius: '0 0 12px 12px',
          '& .MuiDataGrid-columnHeaders': { borderRadius: 0 },
          '& .MuiDataGrid-row': { cursor: onRowClick != null ? 'pointer' : 'default' }
        }}
      />
    </Box>
  );
}