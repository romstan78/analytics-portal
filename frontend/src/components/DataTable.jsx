import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { DataGrid } from '@mui/x-data-grid';
import { 
  Box, Alert, TextField, Button, Menu, MenuItem, 
  Checkbox, ListItemText, Typography, Divider 
} from '@mui/material';
import { 
  ViewColumn as ColumnsIcon,
  FileDownload as ExportIcon,
  Search as SearchIcon,
} from '@mui/icons-material';

export default function DataTable({ 
  columns, apiUrl, filters = {}, defaultPageSize = 100, 
  exportFileName = 'export', onDataLoaded = null, 
  onRowClick = null, refreshKey 
}) {
  const [rawRows, setRawRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const filtersKey = JSON.stringify(filters);

  // Серверная пагинация
  const [paginationModel, setPaginationModel] = useState({
    page: 0,
    pageSize: defaultPageSize,
  });
  const [totalRows, setTotalRows] = useState(0);

  // Тулбар
  const [searchText, setSearchText] = useState('');
  const [columnsAnchor, setColumnsAnchor] = useState(null);
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

  // Клиентский поиск (по текущей странице)
  const filteredRows = useMemo(() => {
    if (!searchText.trim()) return rows;
    const lower = searchText.toLowerCase();
    return rows.filter(row =>
      Object.values(row).some(val =>
        val != null && String(val).toLowerCase().includes(lower)
      )
    );
  }, [rows, searchText]);

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

  const handleExport = async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      params.set('all', 'true');
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
      const data = json.data || [];

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
      const link = document.createElement('a');
      link.href = URL.createObjectURL(blob);
      link.download = `${exportFileName}.csv`;
      link.click();
      URL.revokeObjectURL(link.href);
    } catch (err) {
      console.error('Ошибка экспорта:', err);
    } finally {
      setLoading(false);
    }
  };

  // Загрузка данных с пагинацией
  const fetchData = useCallback(async () => {
    setLoading(true); setError(null);
    try {
      const params = new URLSearchParams();
      params.set('page', String(paginationModel.page));
      params.set('pageSize', String(paginationModel.pageSize));
      
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
  }, [apiUrl, filtersKey, paginationModel.page, paginationModel.pageSize]);

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

        <TextField size="small" placeholder="Поиск по странице..." value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          InputProps={{ startAdornment: <SearchIcon sx={{ fontSize: 18, color: '#94a3b8', mr: 0.5 }} /> }}
          sx={{ width: 240, '& .MuiOutlinedInput-root': { bgcolor: '#fff', borderRadius: 2 }, '& .MuiInputBase-input': { fontSize: '0.875rem', py: 0.75 } }} />

        <Box sx={{ flex: 1 }} />

        {totalRows > 0 && (
          <Typography variant="caption" color="text.secondary" sx={{ mr: 1 }}>
            {totalRows.toLocaleString('ru-RU')} строк
          </Typography>
        )}

        <Button size="small" startIcon={<ExportIcon />} onClick={handleExport}
          sx={{ color: '#475569', fontWeight: 500 }}>CSV</Button>
      </Box>

      {/* Таблица */}
      <DataGrid 
        apiRef={apiRef}
        rows={filteredRows} 
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