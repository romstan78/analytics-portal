import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { 
  Button, Stack, Box, Typography, CircularProgress, Tabs, Tab, 
  Alert, Snackbar, Dialog, DialogTitle, DialogContent, DialogActions,
  TextField, Menu, MenuItem, Checkbox, ListItemText, Divider,
  Tooltip, Chip,
} from '@mui/material';
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material';
import { 
  ViewColumn as ColumnsIcon,
  FileDownload as ExportIcon,
  Search as SearchIcon,
} from '@mui/icons-material';
import { saveAs } from 'file-saver';
import { ButtonGroup } from '@mui/material';
import { DataGrid } from '@mui/x-data-grid';
import FilterPanel from '../components/FilterPanel';
import PromoForm from './PromoForm';
import PromoEditDialog from '../components/PromoEditDialog';
import PromoApproval from './PromoApproval';
import { promoAPI } from '../api/promo';
import { usePromoFilters } from '../hooks/usePromoFilters';
import { usePromoData } from '../hooks/usePromoData';
import { usePromoForm } from '../hooks/usePromoForm';
import { usePromoCalculations } from '../hooks/usePromoCalculations.ts';

const FILTERS_STORAGE_KEY = 'promo_filters_v20';
const PERSIST_FLAG_KEY = 'promo_persist_v20';

const renderAgreement = (value) => {
  if (value == null || value === '' || value === '0') return '';
  const v = String(value);
  const lower = v.toLowerCase();

  const isApproved = lower.startsWith('согласовано');
  const isRejected = lower.startsWith('отклонено');

  if (isApproved) {
    const comment = v.substring('согласовано'.length).replace(/^[:\s]+/, '');
    return (
      <Tooltip title={comment || 'Согласовано'} arrow placement="top" disableHoverListener={!comment}>
        <Chip
          label={comment ? '✓ Согласовано + комм.' : '✓ Согласовано'}
          size="small"
          variant="filled"
          sx={{ bgcolor: '#f0fdf4', color: '#16a34a', fontWeight: 600, height: 24, fontSize: '0.75rem' }}
        />
      </Tooltip>
    );
  }

  if (isRejected) {
    const comment = v.substring('отклонено'.length).replace(/^[:\s]+/, '');
    return (
      <Tooltip title={comment || 'Отклонено'} arrow placement="top" disableHoverListener={!comment}>
        <Chip
          label={comment ? '✗ Отклонено + комм.' : '✗ Отклонено'}
          size="small"
          variant="filled"
          sx={{ bgcolor: '#fef2f2', color: '#dc2626', fontWeight: 600, height: 24, fontSize: '0.75rem' }}
        />
      </Tooltip>
    );
  }

  // Только комментарий
  return (
    <Tooltip title={v} arrow placement="top">
      <Chip
        label="💬 Комментарий"
        size="small"
        variant="filled"
        sx={{ bgcolor: '#eef2ff', color: '#6366f1', fontWeight: 600, height: 24, fontSize: '0.75rem' }}
      />
    </Tooltip>
  );
};

// ─── Колонки таблицы просмотра данных ──────────────────────────────────────
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
    valueFormatter: (v) => (v != null && v !== 0) ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 0 }) : '' },
  { field: 'plan_investments_rub', headerName: 'План инвест.', width: 130, type: 'number', 
    valueFormatter: (v) => (v != null && v !== 0) ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2 }) : '' },
  { field: 'actual_investments', headerName: 'Факт инвест.', width: 130, type: 'number', 
    valueFormatter: (v) => (v != null && v !== 0) ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2 }) : '' },
  { field: 'agreement1', headerName: 'Согласование 1', width: 160,
    renderCell: (params) => renderAgreement(params.value) },
  { field: 'agreement2', headerName: 'Согласование 2', width: 160,
    renderCell: (params) => renderAgreement(params.value) },
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

// ─── Компонент ─────────────────────────────────────────────────────────────
// role — передаётся из App.jsx (admin / agreement1 / agreement2)
export default function PromoAnalysis({ role }) {
  const navigate = useNavigate();
  const [tab, setTab] = useState(0);
  const [refreshTrigger, setRefreshTrigger] = useState(0);
  const [allSkuOptions, setAllSkuOptions] = useState([]);
  const [allNetworkOptions, setAllNetworkOptions] = useState([]);
  const [investmentTypes, setInvestmentTypes] = useState([]);
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'success' });

  // ─── Пользовательский тулбар таблицы ──────────────────────────────────
  const [searchText, setSearchText] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [columnsAnchor, setColumnsAnchor] = useState(null);
  const [visibleColumns, setVisibleColumns] = useState(() => {
    const map = {};
    COLUMNS.forEach(c => { map[c.field] = true; });
    return map;
  });
  const apiRef = useRef(null);

  // ─── Фильтры и данные ─────────────────────────────────────────────────
  const { meta, filters, setFilters, appliedFilters, persistFilters, handleSearch, handleReset, handlePersistChange, fetchMeta } = 
    usePromoFilters(EMPTY_FILTERS, FILTERS_STORAGE_KEY, PERSIST_FLAG_KEY);
  const { rows, setRows, loading: dataLoading, error: dataError, refetch } = usePromoData(appliedFilters, refreshTrigger);

  // После редактирования/удаления/создания — инвалидируем кеш React Query
  const handleDataChanged = useCallback(() => {
    setRefreshTrigger(prev => prev + 1);
  }, []);

  // ─── Форма редактирования ─────────────────────────────────────────────
  const { form, setForm, saving, deleting, handleRowClick: formHandleRowClick, handleSave: formHandleSave, handleDelete: formHandleDelete, resetForm } = 
    usePromoForm({ onEditSuccess: handleDataChanged, onDeleteSuccess: handleDataChanged, onCreateSuccess: handleDataChanged });
  const { recalcPlan, recalcActual } = usePromoCalculations(form);

  // ─── Refetch при возврате на вкладку "Просмотр данных" ────────────────
  useEffect(() => {
    if (tab === 0) refetch();
  }, [tab]);

  // ─── Загрузка справочников ────────────────────────────────────────────
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

  // ─── Обработчики действий ─────────────────────────────────────────────
  const handleRowClick = (params) => { formHandleRowClick(params.row); setEditDialogOpen(true); };

  const handleSave = async () => { 
    const result = await formHandleSave(); 
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

  // Debounce поиска (300ms) — чтобы не тормозить при вводе
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchText.trim());
    }, 300);
    return () => clearTimeout(timer);
  }, [searchText]);

  // ─── Поиск по таблице (клиентский, с debounce) ────────────────────────
  const filteredRows = useMemo(() => {
    if (!debouncedSearch) return rows;
    const lower = debouncedSearch.toLowerCase();
    return rows.filter(row =>
      Object.values(row).some(val =>
        val != null && String(val).toLowerCase().includes(lower)
      )
    );
  }, [rows, debouncedSearch]);

  const visibleCols = useMemo(
    () => COLUMNS.filter(c => visibleColumns[c.field] !== false),
    [visibleColumns]
  );

  const toggleColumn = (f) => setVisibleColumns(prev => ({ ...prev, [f]: !prev[f] }));

  // ─── Экспорт CSV (клиентский — выгружаем отфильтрованные строки) ─────
  const handleExportCSV = () => {
    try {
      const data = filteredRows;
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
      saveAs(blob, `promo-analysis_${new Date().toISOString().split('T')[0]}.csv`);
    } catch (e) { console.error('Export error:', e); }
  };

  // Экспорт XLSX через серверный эндпоинт
  const handleExportXLSX = async () => {
    try {
      const token = localStorage.getItem('token');
      const qs = new URLSearchParams();
      Object.entries(appliedFilters).forEach(([key, value]) => {
        if (Array.isArray(value)) {
          value.forEach(v => { if (v !== '' && v != null) qs.append(key, String(v)); });
        } else if (value !== '' && value != null) {
          qs.set(key, String(value));
        }
      });
      const resp = await fetch(`http://localhost:8080/api/promo/export-xlsx?${qs}`, {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const blob = await resp.blob();
      saveAs(blob, `promo-export_${new Date().toISOString().split('T')[0]}.xlsx`);
    } catch (e) { console.error('Export error:', e); }
  };

  // ─── Рендер ───────────────────────────────────────────────────────────
  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column', p: 2 }}>
      {/* Шапка */}
      <Stack direction="row" alignItems="center" spacing={2} sx={{ mb: 2 }}>
        <Button startIcon={<ArrowBackIcon />} onClick={() => navigate('/')}>На главную</Button>
        <Typography variant="h5" sx={{ fontWeight: 600 }}>Анализ промо</Typography>
        {meta.loading && <CircularProgress size={20} />}
        {rows.length > 0 && tab === 0 && 
          <Typography variant="body2" color="text.secondary">Загружено: {rows.length} записей</Typography>}
      </Stack>

      {/* ─── Вкладки ─────────────────────────────────────────────────── */}
      <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ mb: 2 }}>
        <Tab label="Просмотр данных" />
        <Tab label="Новое промо" />
        {(role === 'agreement1' || role === 'agreement2' || role === 'admin') && (
          <Tab label="Согласование" />
        )}
      </Tabs>

      {/* ─── Tab 0: Просмотр данных ──────────────────────────────────── */}
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

          <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', minHeight: 0 }}>
          {/* Пользовательский тулбар */}
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
            <ButtonGroup size="small" variant="text">
              <Button startIcon={<ExportIcon />} onClick={handleExportCSV}
                sx={{ color: '#475569', fontWeight: 500 }}>CSV</Button>
              <Button startIcon={<ExportIcon />} onClick={handleExportXLSX}
                sx={{ color: '#475569', fontWeight: 500 }}>Excel</Button>
            </ButtonGroup>
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

        {/* Диалог редактирования */}
        <PromoEditDialog 
          open={editDialogOpen} onClose={() => setEditDialogOpen(false)}
          form={form} setForm={setForm} recalcPlan={recalcPlan} recalcActual={recalcActual}
          onSave={handleSave} onDelete={() => setDeleteDialogOpen(true)}
          saving={saving} deleting={deleting} meta={meta} 
          allSkuOptions={allSkuOptions} allNetworkOptions={allNetworkOptions} 
          investmentTypes={investmentTypes}
          role={role}
        />

        {/* Диалог подтверждения удаления */}
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

      {/* ─── Tab 1: Новое промо ────────────────────────────────────────── */}
      {tab === 1 && <PromoForm onSave={handlePromoFormSave} />}

      {/* ─── Tab 2: Согласование ──────────────────────────────────────── */}
      {tab === 2 && <PromoApproval role={role} onDataChanged={() => setRefreshTrigger(prev => prev + 1)} />}

      {/* Снекбар уведомлений */}
      <Snackbar open={snackbar.open} autoHideDuration={3000} 
        onClose={() => setSnackbar(s => ({ ...s, open: false }))}>
        <Alert severity={snackbar.severity} onClose={() => setSnackbar(s => ({ ...s, open: false }))}>
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>
  );
}