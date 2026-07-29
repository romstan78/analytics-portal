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
  const [refreshKey, setRefreshKey] = useState(0);
  const [allSkuOptions, setAllSkuOptions] = useState([]);
  const [allNetworkOptions, setAllNetworkOptions] = useState([]);
  const [investmentTypes, setInvestmentTypes] = useState([]);
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'success' });

  // --- Свой тулбар ---
  const [searchText, setSearchText] = useState('');
  const [columnsAnchor, setColumnsAnchor] = useState(null);
  const [visibleColumns, setVisibleColumns] = useState(() => {
    const map = {};
    COLUMNS.forEach(c => { map[c.field] = true; });
    return map;
  });
  const apiRef = useRef(null);

  const toggleColumn = (field) => {
    setVisibleColumns(prev => ({ ...prev, [field]: !prev[field] }));
  };

  const showAllColumns = () => {
    const map = {};
    COLUMNS.forEach(c => { map[c.field] = true; });
    setVisibleColumns(map);
  };

  const hideAllColumns = () => {
    const map = {};
    COLUMNS.forEach(c => { map[c.field] = false; });
    setVisibleColumns(map);
  };

  const visibleCols = useMemo(
    () => COLUMNS.filter(c => visibleColumns[c.field] !== false),
    [visibleColumns]
  );

  const handleExport = () => {
    if (apiRef.current) {
      apiRef.current.exportDataAsCsv({
        utf8WithBom: true,
        fileName: 'promo-analysis',
      });
    }
  };

  // --- Данные и фильтры ---
  const { meta, filters, setFilters, appliedFilters, persistFilters, handleSearch, handleReset, handlePersistChange, fetchMeta } = 
    usePromoFilters(EMPTY_FILTERS, FILTERS_STORAGE_KEY, PERSIST_FLAG_KEY);
  const { rows, loading: dataLoading, error: dataError } = usePromoData(appliedFilters, refreshKey);

  // Поиск
  const filteredRows = useMemo(() => {
    if (!searchText.trim()) return rows;
    const lower = searchText.toLowerCase();
    return rows.filter(row =>
      Object.values(row).some(val =>
        val != null && String(val).toLowerCase().includes(lower)
      )
    );
  }, [rows, searchText]);

  const { form, setForm, saving, deleting, handleRowClick: formHandleRowClick, handleSave: formHandleSave, handleDelete: formHandleDelete } = 
    usePromoForm(() => { setRefreshKey(prev => prev + 1); setEditDialogOpen(false); });
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
    setSnackbar({ open: true, message: result.message, severity: result.success ? 'success' : 'error' }); 
  };
  const handleDelete = async () => { 
    const result = await formHandleDelete(); 
    if (result.success) { 
      setDeleteDialogOpen(false); 
      setEditDialogOpen(false); 
      setRefreshKey(prev => prev + 1); 
    } 
    setSnackbar({ open: true, message: result.message, severity: result.success ? 'success' : 'error' }); 
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
          <FilterPanel 
            filters={filters} filterOptions={filterOptions} onFiltersChange={setFilters} 
            onSearch={handleSearch} onReset={handleReset} extraFilters={EXTRA_FILTERS}
            persistFilters={persistFilters} onPersistChange={handlePersistChange} 
            visibleFilters={PROMO_VISIBLE_FILTERS} 
          />
        </Box>
        {meta.error && 
          <Button variant="outlined" color="warning" onClick={() => fetchMeta(filters)} sx={{ mb: 2 }}>
            Ошибка загрузки справочников. Повторить
          </Button>}

        {/* Таблица с кастомным тулбаром */}
        <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
          
          {/* Тулбар */}
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
              {COLUMNS.map(col => (
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

          {/* DataGrid */}
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
              flex: 1,
              border: '1px solid #e2e8f0',
              borderTop: 'none',
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

      {tab === 1 && 
        <PromoForm onSave={() => { 
          setRefreshKey(prev => prev + 1); 
          setSnackbar({ open: true, message: '✅ Сохранено', severity: 'success' }); 
        }} />}

      <Snackbar open={snackbar.open} autoHideDuration={3000} 
        onClose={() => setSnackbar(s => ({ ...s, open: false }))}>
        <Alert severity={snackbar.severity} onClose={() => setSnackbar(s => ({ ...s, open: false }))}>
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>
  );
}