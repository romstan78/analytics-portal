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
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const filtersKey = JSON.stringify(filters);

  // --- Свой тулбар ---
  const [searchText, setSearchText] = useState('');
  const [columnsAnchor, setColumnsAnchor] = useState(null);
  const [visibleColumns, setVisibleColumns] = useState(() => {
    const map = {};
    columns.forEach(c => { map[c.field] = true; });
    return map;
  });
  const apiRef = useRef(null);

  // Фильтрация по поиску (клиентская)
  const filteredRows = useMemo(() => {
    if (!searchText.trim()) return rows;
    const lower = searchText.toLowerCase();
    return rows.filter(row =>
      Object.values(row).some(val =>
        val != null && String(val).toLowerCase().includes(lower)
      )
    );
  }, [rows, searchText]);

  // Видимые колонки
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

  const handleExport = () => {
    if (apiRef.current) {
      apiRef.current.exportDataAsCsv({
        utf8WithBom: true,
        fileName: exportFileName,
      });
    }
  };

  // --- Загрузка данных ---
  const fetchData = useCallback(async () => {
    setLoading(true); setError(null);
    try {
      const params = new URLSearchParams();
      Object.entries(filters).forEach(([key, value]) => { 
        if (Array.isArray(value)) { 
          value.forEach(v => { if (v !== '' && v != null) params.append(key, String(v)); }); 
        } else if (value !== '' && value != null) { 
          params.set(key, String(value)); 
        } 
      });
      const url = `${apiUrl}${params.toString() ? '?' + params.toString() : ''}`;
      const response = await fetch(url);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const json = await response.json();
      setRows(json.data || []);
      if (onDataLoaded) onDataLoaded(json.data || []);
    } catch (err) { setError(err.message); } finally { setLoading(false); }
  }, [apiUrl, filtersKey]);

  useEffect(() => { fetchData(); }, [fetchData, refreshKey]);

  if (error) return <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>Ошибка загрузки: {error}</Alert>;

  return (
    <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', height: '100%' }}>
      
      {/* Свой тулбар */}
      <Box sx={{ 
        display: 'flex', alignItems: 'center', gap: 1, 
        px: 2, py: 1,
        bgcolor: '#f1f5f9', 
        borderRadius: '12px 12px 0 0',
        border: '1px solid #e2e8f0',
        borderBottom: 'none',
      }}>
        <Button 
          size="small" 
          startIcon={<ColumnsIcon />}
          onClick={(e) => setColumnsAnchor(e.currentTarget)}
          sx={{ color: '#475569', fontWeight: 500 }}
        >
          Колонки
        </Button>
        <Menu
          anchorEl={columnsAnchor}
          open={Boolean(columnsAnchor)}
          onClose={() => setColumnsAnchor(null)}
          slotProps={{ paper: { sx: { maxHeight: 400, minWidth: 220 } } }}
        >
          <MenuItem dense onClick={showAllColumns}>
            <Typography variant="caption" color="primary" sx={{ fontWeight: 600 }}>Показать все</Typography>
          </MenuItem>
          <MenuItem dense onClick={hideAllColumns}>
            <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>Скрыть все</Typography>
          </MenuItem>
          <Divider />
          {columns.map(col => (
            <MenuItem key={col.field} dense onClick={() => toggleColumn(col.field)}>
              <Checkbox size="small" checked={visibleColumns[col.field] !== false} />
              <ListItemText primary={col.headerName || col.field} primaryTypographyProps={{ fontSize: 13 }} />
            </MenuItem>
          ))}
        </Menu>

        <TextField
          size="small"
          placeholder="Поиск..."
          value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          InputProps={{ startAdornment: <SearchIcon sx={{ fontSize: 18, color: '#94a3b8', mr: 0.5 }} /> }}
          sx={{ 
            width: 220,
            '& .MuiOutlinedInput-root': { 
              bgcolor: '#fff', 
              borderRadius: 2,
              '& fieldset': { borderColor: '#e2e8f0' },
              '&:hover fieldset': { borderColor: '#cbd5e1' },
            },
            '& .MuiInputBase-input': { fontSize: '0.875rem', py: 0.75 }
          }}
        />

        <Box sx={{ flex: 1 }} />

        <Button 
          size="small"
          startIcon={<ExportIcon />}
          onClick={handleExport}
          sx={{ color: '#475569', fontWeight: 500 }}
        >
          CSV
        </Button>
      </Box>

      {/* Таблица */}
      <DataGrid 
        apiRef={apiRef}
        rows={filteredRows} 
        columns={visibleCols} 
        loading={loading} 
        sortingMode="client" 
        disableColumnFilter
        onRowClick={onRowClick}
        initialState={{ 
          pagination: { paginationModel: { pageSize: defaultPageSize } }, 
          sorting: { sortModel: [{ field: 'year', sort: 'desc' }] } 
        }}
        pageSizeOptions={[25, 50, 100]} 
        disableRowSelectionOnClick
        sx={{ 
          flex: 1,
          border: '1px solid #e2e8f0',
          borderTop: 'none',
          borderRadius: '0 0 12px 12px',
          '& .MuiDataGrid-columnHeaders': { 
            borderRadius: 0,
          },
          '& .MuiDataGrid-row': { cursor: onRowClick ? 'pointer' : 'default' } 
        }} 
      />
    </Box>
  );
}