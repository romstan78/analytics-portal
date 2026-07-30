import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { 
  Button, Stack, Box, Typography, CircularProgress, Tabs, Tab, 
  Alert, Snackbar, Dialog, DialogTitle, DialogContent, DialogActions,
  TextField, Menu, MenuItem, Checkbox, ListItemText, Divider,
} from '@mui/material';
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material';
import { 
  ViewColumn as ColumnsIcon,
  FileDownload as ExportIcon,
  Search as SearchIcon,
} from '@mui/icons-material';
import { DataGrid } from '@mui/x-data-grid';
import FilterPanel from '../components/FilterPanel';
import PromoForm from './PromoForm';
import PromoEditDialog from '../components/PromoEditDialog';
import { promoAPI } from '../api/promo';
import { usePromoFilters } from '../hooks/usePromoFilters';
import { usePromoData } from '../hooks/usePromoData';
import { usePromoForm } from '../hooks/usePromoForm';
import { usePromoCalculations } from '../hooks/usePromoCalculations';

const FILTERS_STORAGE_KEY = 'promo_filters_v20';
const PERSIST_FLAG_KEY = 'promo_persist_v20';

const COLUMNS = [
  { field: 'year', headerName: 'Год', width: 70, type: 'number', valueFormatter: (v) => v },
  { field: 'month', headerName: 'Мес', width: 60, type: 'number' }, 
  { field: 'channel', headerName: 'Канал', width: 90 },
  { field: 'network_name', headerName: 'Сеть', width: 180 }, 
  { field: 'brand_as', headerName: 'Бренд', width: 130 },
  { field: 'sku', headerName: 'SKU', width: 130 }, 
  { field: 'mechanics', headerName: 'Механика', width: 180 },
  { field: 'plan_promo_units', headerName: 'План (уп)', width: 110, type: 'number', 
    valueFormatter: (v) => v != null ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 0 }) : '' },
  { field: 'actual_promo_sales_units', headerName: 'Факт (уп)', width: 110, type: 'number', 
    valueFormatter: (v) => v != null ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 0 }) : '' },
  { field: 'plan_investments_rub', headerName: 'План инвест.', width: 130, type: 'number', 
    valueFormatter: (v) => v != null ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2 }) : '' },
  { field: 'actual_investments', headerName: 'Факт инвест.', width: 130, type: 'number', 
    valueFormatter: (v) => v != null ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2 }) : '' },
  { field: 'status', headerName: 'Статус', width: 140 },
];

const EMPTY_FILTERS = { 
  yearFrom: '', yearTo: '', months: [], kam: [], brand: [], sku: [], 
  network_name: [], mechanics: [], channel: [], status: [] 
};
const EXTRA_FILTERS = [
  { type: 'year', field: 'yearFrom', label: 'Год от' }, 
  { type: 'year', field: 'yearTo', label: 'Год до' }, 
  { type: 'months', field: 'months', label: 'Месяцы' }
];
const PROMO_VISIBLE_FILTERS = ['kam', 'brand', 'sku', 'network_name', 'mechanics', 'channel', 'status'];

export default function PromoAnalysis() {
  const navigate = useNavigate();
  const [tab, setTab] = useState(0);
  const [refreshTrigger, setRefreshTrigger] = useState(0);
  const [allSkuOptions, setAllSkuOptions] = useState([]);
  const [allNetworkOptions, setAllNetworkOptions] = useState([]);
  const [investmentTypes, setInvestmentTypes] = useState([]);
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'success' });

  // Тулбар
  const [searchText, setSearchText] = useState('');
  const [columnsAnchor, setColumnsAnchor] = useState(null);
  const [visibleColumns, setVisibleColumns] = useState(() => {
    const map = {};
    COLUMNS.forEach(c => { map[c.field] = true; });
    return map;
  });
  const apiRef = useRef(null);

  const { meta, filters, setFilters, appliedFilters, persistFilters, handleSearch, handleReset, handlePersistChange, fetchMeta } = 
    usePromoFilters(EMPTY_FILTERS, FILTERS_STORAGE_KEY, PERSIST_FLAG_KEY);
  const { rows, setRows, loading: dataLoading, error: dataError, refetch } = usePromoData(appliedFilters, refreshTrigger);

  // Локальное обновление после редактирования
  const handleEditSuccess = useCallback((editedId, updatedData) => {
    setRows(prev => prev.map(row => 
      row.id === editedId ? { ...row, ...updatedData } : row
    ));
  }, [setRows]);

  // Локальное удаление
  const handleDeleteSuccess = useCallback((deletedId) => {
    setRows(prev => prev.filter(row => row.id !== deletedId));
  }, [setRows]);

  // После создания нового промо — перезагрузка
  const handleCreateSuccess = useCallback(() => {
    setRefreshTrigger(prev => prev + 1);
  }, []);

  const { form, setForm, saving, deleting, handleRowClick: formHandleRowClick, handleSave: formHandleSave, handleDelete: formHandleDelete, resetForm } = 
    usePromoForm({ onEditSuccess: handleEditSuccess, onDeleteSuccess: handleDeleteSuccess, onCreateSuccess: handleCreateSuccess });
  const { recalcPlan, recalcActual } = usePromoCalculations(form);

  useEffect(() => { promoAPI.getInvestmentTypes().then(data => setInvestmentTypes(data.data || [])); }, []);
  useEffect(() => { 
    promoAPI.getFilters().then(data => { 
      setAllSkuOptions(data.sku || []); 
      setAllNetworkOptions(data.network_name || []); 
    }); 
  }, []);

  const filterOptions = useMemo(() => ({
    kam: meta.kam || [], brand: meta.brand || [], sku: meta.sku || [],
    network_name: meta.network_name || [], mechanics: meta.mechanics || [],
    channel: meta.channel || [], status: meta.status || []
  }), [meta]);

  const handleRowClick = (params) => { formHandleRowClick(params.row); setEditDialogOpen(true); };

  const handleSave = async () => { 
    const result = await formHandleSave(); 
    if (result.success) setEditDialogOpen(false);
    setSnackbar({ open: true, message: result.message, severity: result.success ? 'success' : 'error' }); 
  };

  const handleDelete = async () => { 
    const result = await formHandleDelete(); 
    if (result.success) { setDeleteDialogOpen(false); setEditDialogOpen(false); }
    setSnackbar({ open: true, message: result.message, severity: result.success ? 'success' : 'error' }); 
  };

  const handlePromoFormSave = () => {
    setRefreshTrigger(prev => prev + 1);
    setSnackbar({ open: true, message: '✅ Сохранено', severity: 'success' });
  };

  // Поиск по таблице (клиентский)
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
    () => COLUMNS.filter(c => visibleColumns[c.field] !== false),
    [visibleColumns]
  );

  const toggleColumn = (f) => setVisibleColumns(prev => ({ ...prev, [f]: !prev[f] }));

  const handleExport = async () => {
    try {
      const params = new URLSearchParams();
      params.set('all', 'true');
      Object.entries(appliedFilters).forEach(([k, v]) => {
        if (Array.isArray(v)) v.forEach(x => { if (x) params.append(k, String(x)); });
        else if (v) params.set(k, String(v));
      });
      const res = await fetch(`http://localhost:8080/api/promo/data?${params}`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
      });
      const json = await res.json();
      const data = json.data || [];
      const headers = visibleCols.map(c => c.headerName || c.field);
      const fields = visibleCols.map(c => c.field);
      let csv = '\uFEFF' + headers.join(';') + '\n';
      data.forEach(row => {
        csv += fields.map(f => {
          let v = row[f]; if (v == null) return '';
          v = String(v);
          if (v.includes(';') || v.includes('"') || v.includes('\n')) v = '"' + v.replace(/"/g, '""') + '"';
          return v;
        }).join(';') + '\n';
      });
      const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
      const link = document.createElement('a');
      link.href = URL.createObjectURL(blob);
      link.download = 'promo-analysis.csv';
      link.click();
      URL.revokeObjectURL(link.href);
    } catch (e) { console.error('Export error:', e); }
  };

  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column', p: 2 }}>
      <Stack direction="row" alignItems="center" spacing={2} sx={{ mb: 2 }}>
        <Button startIcon={<ArrowBackIcon />} onClick={() => navigate('/')}>На главную</Button>
        <Typography variant="h5" sx={{ fontWeight: 600 }}>Анализ промо</Typography>
        {meta.loading && <CircularProgress size={20} />}
        {rows.length > 0 && tab === 0 && 
          <Typography variant="body2" color="text.secondary">Загружено: {rows.length} записей</Typography>}
      </Stack>

      <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ mb: 2 }}>
        <Tab label="Просмотр данных" />
        <Tab label="Новое промо" />
      </Tabs>

      {tab === 0 && (<>
        <Box sx={{ mb: 2 }}>
          <FilterPanel filters={filters} filterOptions={filterOptions} onFiltersChange={setFilters}
            onSearch={handleSearch} onReset={handleReset} extraFilters={EXTRA_FILTERS}
            persistFilters={persistFilters} onPersistChange={handlePersistChange} 
            visibleFilters={PROMO_VISIBLE_FILTERS} />
        </Box>
        {meta.error && 
          <Button variant="outlined" color="warning" onClick={() => fetchMeta(filters)} sx={{ mb: 2 }}>
            Ошибка загрузки справочников. Повторить
          </Button>}

        <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
          <Box sx={{ 
            display: 'flex', alignItems: 'center', gap: 1, px: 2, py: 1,
            bgcolor: '#f1f5f9', borderRadius: '12px 12px 0 0',
            border: '1px solid #e2e8f0', borderBottom: 'none',
          }}>
            <Button size="small" startIcon={<ColumnsIcon />}
              onClick={(e) => setColumnsAnchor(e.currentTarget)}
              sx={{ color: '#475569', fontWeight: 500 }}>Колонки</Button>
            <Menu anchorEl={columnsAnchor} open={Boolean(columnsAnchor)}
              onClose={() => setColumnsAnchor(null)}
              slotProps={{ paper: { sx: { maxHeight: 400, minWidth: 220 } } }}>
              <MenuItem dense onClick={() => setVisibleColumns(Object.fromEntries(COLUMNS.map(c => [c.field, true])))}>
                <Typography variant="caption" color="primary" sx={{ fontWeight: 600 }}>Показать все</Typography></MenuItem>
              <MenuItem dense onClick={() => setVisibleColumns(Object.fromEntries(COLUMNS.map(c => [c.field, false])))}>
                <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>Скрыть все</Typography></MenuItem>
              <Divider />
              {COLUMNS.map(col => (
                <MenuItem key={col.field} dense onClick={() => toggleColumn(col.field)}>
                  <Checkbox size="small" checked={visibleColumns[col.field] !== false} />
                  <ListItemText primary={col.headerName || col.field} primaryTypographyProps={{ fontSize: 13 }} />
                </MenuItem>
              ))}
            </Menu>
            <TextField size="small" placeholder="Поиск по таблице..." value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
              InputProps={{ startAdornment: <SearchIcon sx={{ fontSize: 18, color: '#94a3b8', mr: 0.5 }} /> }}
              sx={{ width: 240, '& .MuiOutlinedInput-root': { bgcolor: '#fff', borderRadius: 2 }, '& .MuiInputBase-input': { fontSize: '0.875rem', py: 0.75 } }} />
            <Box sx={{ flex: 1 }} />
            {rows.length > 0 && (
              <Typography variant="caption" color="text.secondary" sx={{ mr: 1 }}>
                {rows.length.toLocaleString('ru-RU')} строк
              </Typography>
            )}
            <Button size="small" startIcon={<ExportIcon />} onClick={handleExport}
              sx={{ color: '#475569', fontWeight: 500 }}>CSV</Button>
          </Box>

          <DataGrid 
            apiRef={apiRef}
            rows={filteredRows} 
            columns={visibleCols} 
            loading={dataLoading} 
            sortingMode="client" 
            disableColumnFilter
            onRowClick={handleRowClick}
            initialState={{ 
              pagination: { paginationModel: { pageSize: 100 } }, 
              sorting: { sortModel: [{ field: 'year', sort: 'desc' }] } 
            }}
            pageSizeOptions={[25, 50, 100]} 
            disableRowSelectionOnClick 
            sx={{ 
              flex: 1, border: '1px solid #e2e8f0', borderTop: 'none',
              borderRadius: '0 0 12px 12px',
              '& .MuiDataGrid-columnHeaders': { borderRadius: 0 },
              '& .MuiDataGrid-row': { cursor: 'pointer' } 
            }} 
          />
        </Box>

        <PromoEditDialog 
          open={editDialogOpen} onClose={() => setEditDialogOpen(false)}
          form={form} setForm={setForm} recalcPlan={recalcPlan} recalcActual={recalcActual}
          onSave={handleSave} onDelete={() => setDeleteDialogOpen(true)}
          saving={saving} deleting={deleting} meta={meta} 
          allSkuOptions={allSkuOptions} allNetworkOptions={allNetworkOptions} 
          investmentTypes={investmentTypes} 
        />

        <Dialog open={deleteDialogOpen} onClose={() => setDeleteDialogOpen(false)}>
          <DialogTitle>Удалить промо #{form.id}?</DialogTitle>
          <DialogContent><Typography>Это действие нельзя отменить.</Typography></DialogContent>
          <DialogActions>
            <Button onClick={() => setDeleteDialogOpen(false)}>Отмена</Button>
            <Button color="error" variant="contained" onClick={handleDelete} disabled={deleting}>
              {deleting ? 'Удаление...' : 'Удалить'}
            </Button>
          </DialogActions>
        </Dialog>
      </>)}

      {tab === 1 && <PromoForm onSave={handlePromoFormSave} />}

      <Snackbar open={snackbar.open} autoHideDuration={3000} 
        onClose={() => setSnackbar(s => ({ ...s, open: false }))}>
        <Alert severity={snackbar.severity} onClose={() => setSnackbar(s => ({ ...s, open: false }))}>
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>
  );
}