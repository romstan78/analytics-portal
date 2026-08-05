import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { DataGrid } from '@mui/x-data-grid';
import { 
  Box, Alert, TextField, Button, Menu, MenuItem, 
  Checkbox, ListItemText, Typography, Divider, ButtonGroup,
  Dialog, DialogTitle, DialogContent, DialogActions,
} from '@mui/material';
import { 
  ViewColumn as ColumnsIcon,
  FileDownload as ExportIcon,
  Search as SearchIcon,
} from '@mui/icons-material';
import ExcelJS from 'exceljs';
import { saveAs } from 'file-saver';

export default function DataTable({ 
  columns, apiUrl, filters = {}, defaultPageSize = 100, 
  exportFileName = 'export', onDataLoaded = null, 
  onRowClick = null, refreshKey 
}) {
  const [rawRows, setRawRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const filtersKey = useMemo(() => JSON.stringify(filters), [filters]);

  // Серверная пагинация
  const [paginationModel, setPaginationModel] = useState({
    page: 0,
    pageSize: defaultPageSize,
  });
  const [totalRows, setTotalRows] = useState(0);

  // Тулбар
  const [searchText, setSearchText] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [columnsAnchor, setColumnsAnchor] = useState(null);
  const [exportDialog, setExportDialog] = useState({ open: false, data: null });
  const [visibleColumns, setVisibleColumns] = useState(() => {
    const map = {};
    columns.forEach(c => { map[c.field] = true; });
    return map;
  });
  const apiRef = useRef(null);

  // Уникальные ключи
  const rows = useMemo(() => {
    return rawRows.map((row, idx) => ({
      ...row,
      _rowId: `${row.id ?? 'row'}_${paginationModel.page}_${idx}`,
    }));
  }, [rawRows, paginationModel.page]);

  // Debounce поиска (400ms) — чтобы не дёргать API на каждый символ
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchText.trim());
    }, 400);
    return () => clearTimeout(timer);
  }, [searchText]);

  // Сброс страницы при изменении поиска
  useEffect(() => {
    setPaginationModel(prev => ({ ...prev, page: 0 }));
  }, [debouncedSearch]);

  const visibleCols = useMemo(
    () => columns.filter(c => visibleColumns[c.field] !== false),
    [columns, visibleColumns]
  );

  const toggleColumn = (field) => {
    setVisibleColumns(prev => ({ ...prev, [field]: !prev[field] }));
  };

  const showAllColumns = () => {
    const map = {};
    columns.forEach(c => { map[c.field] = true; });
    setVisibleColumns(map);
  };

  const hideAllColumns = () => {
    const map = {};
    columns.forEach(c => { map[c.field] = false; });
    setVisibleColumns(map);
  };

  const fetchExportData = async () => {
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
    return json.data || [];
  };

  const handleExportCSV = async () => {
    setLoading(true);
    try {
      const data = await fetchExportData();
      const headers = visibleCols.map(c => c.headerName || c.field);
      const fields = visibleCols.map(c => c.field);

      let csv = '\uFEFF' + headers.join(';') + '\n';
      data.forEach(row => {
        const line = fields.map(f => {
          let val = row[f];
          if (val == null) return '';
          val = String(val);
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
    } finally {
      setLoading(false);
    }
  };

  const confirmedExportXLSX = async (data) => {
    setLoading(true);
    try {
      const workbook = new ExcelJS.Workbook();
      const worksheet = workbook.addWorksheet('Export Data');

      worksheet.columns = visibleCols.map(c => ({
        header: c.headerName || c.field,
        key: c.field,
        width: c.width ? c.width / 7 : 20,
      }));

      const headerRow = worksheet.getRow(1);
      headerRow.font = { bold: true, color: { argb: 'FFFFFFFF' } };
      headerRow.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF6366F1' } };
      headerRow.alignment = { vertical: 'middle', horizontal: 'center' };

      data.forEach(row => worksheet.addRow(row));

      const buffer = await workbook.xlsx.writeBuffer();
      saveAs(new Blob([buffer]), `${exportFileName}_${new Date().toISOString().split('T')[0]}.xlsx`);
    } catch (err) {
      console.error('Ошибка экспорта Excel:', err);
    } finally {
      setLoading(false);
      setExportDialog({ open: false, data: null });
    }
  };

  const handleExportXLSX = async () => {
    const data = await fetchExportData();
    if (data.length > 10000) {
      setExportDialog({ open: true, data });
    } else {
      confirmedExportXLSX(data);
    }
  };

  // Загрузка данных с пагинацией
  const fetchData = useCallback(async () => {
    setLoading(true); setError(null);
    try {
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
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const json = await response.json();
      const data = json.data || [];
      setRawRows(data);
      setTotalRows(json.totalRows || data.length);
      if (onDataLoaded) onDataLoaded(data);
    } catch (err) { setError(err.message); } finally { setLoading(false); }
  }, [apiUrl, filtersKey, paginationModel.page, paginationModel.pageSize, debouncedSearch]);

  // Сброс страницы при смене фильтров
  useEffect(() => {
    setPaginationModel(prev => ({ ...prev, page: 0 }));
  }, [filtersKey]);

  useEffect(() => { fetchData(); }, [fetchData, refreshKey]);

  if (error) return <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>Ошибка загрузки: {error}</Alert>;

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
          onClick={(e) => setColumnsAnchor(e.currentTarget)}
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
              <ListItemText primary={col.headerName || col.field} primaryTypographyProps={{ fontSize: 13 }} />
            </MenuItem>
          ))}
        </Menu>

        <TextField size="small" placeholder="Поиск..." value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          InputProps={{ startAdornment: <SearchIcon sx={{ fontSize: 18, color: '#94a3b8', mr: 0.5 }} /> }}
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
          <Button startIcon={<ExportIcon />} onClick={handleExportXLSX}
            sx={{ color: '#475569', fontWeight: 500 }}>Excel</Button>
        </ButtonGroup>
      </Box>

      {/* Диалог подтверждения большого экспорта */}
      <Dialog open={exportDialog.open} onClose={() => setExportDialog({ open: false, data: null })}>
        <DialogTitle>Подтверждение экспорта</DialogTitle>
        <DialogContent>
          <Typography>
            Количество строк для экспорта: {(exportDialog.data?.length || 0).toLocaleString('ru-RU')}.<br />
            Экспорт в Excel может занять длительное время и создать файл большого объёма.<br />
            Продолжить?
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setExportDialog({ open: false, data: null })}>Отмена</Button>
          <Button variant="contained" onClick={() => confirmedExportXLSX(exportDialog.data)}>
            Экспортировать
          </Button>
        </DialogActions>
      </Dialog>

      {/* Таблица */}
      <DataGrid 
        apiRef={apiRef}
        rows={rows} 
        columns={visibleCols}
        getRowId={(row) => row._rowId}
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
          '& .MuiDataGrid-row': { cursor: onRowClick ? 'pointer' : 'default' } 
        }} 
      />
    </Box>
  );
}