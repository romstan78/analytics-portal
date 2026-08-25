import { lazy, Suspense, useState, useEffect, useMemo, useCallback, useRef } from 'react';
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
import { DataGrid, type GridColDef, type GridRenderCellParams, type GridRowParams } from '@mui/x-data-grid';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import FilterPanel, { type ExtraFilter } from '../components/FilterPanel';
import type { PromoRow } from '../types/promo';
import PromoEditDialog from '../components/PromoEditDialog';
import { promoAPI } from '../api/promo';
import { usePromoFilters } from '../hooks/usePromoFilters';
import { usePromoData } from '../hooks/usePromoData';
import { usePromoForm } from '../hooks/usePromoForm';
import { usePromoCalculations } from '../hooks/usePromoCalculations';

const PromoForm = lazy(() => import('./PromoForm'));
const PromoApproval = lazy(() => import('./PromoApproval'));
const PromoDashboard = lazy(() => import('../components/PromoDashboard'));

const FILTERS_STORAGE_KEY = 'promo_filters_v20';
const PERSIST_FLAG_KEY = 'promo_persist_v20';
const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080';
const EXPORT_WARNING_THRESHOLD = 10000;

const renderAgreement = (value: unknown) => {
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
const COLUMNS: GridColDef[] = [
  { field: 'year', headerName: 'Год', width: 70, type: 'number', valueFormatter: (v: number) => v },
  { field: 'month', headerName: 'Мес', width: 60, type: 'number' },
  { field: 'channel', headerName: 'Канал', width: 90 },
  { field: 'network_name', headerName: 'Сеть', width: 180 },
  { field: 'brand_as', headerName: 'Бренд', width: 130 },
  { field: 'sku', headerName: 'SKU', width: 130 },
  { field: 'mechanics', headerName: 'Механика', width: 180 },
  { field: 'plan_promo_units', headerName: 'План (уп)', width: 110, type: 'number',
    valueFormatter: (v: number | null) => v != null ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 0 }) : '' },
  { field: 'actual_promo_sales_units', headerName: 'Факт (уп)', width: 110, type: 'number',
    valueFormatter: (v: number | null) => (v != null && v !== 0) ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 0 }) : '' },
  { field: 'plan_investments_rub', headerName: 'План инвест.', width: 130, type: 'number',
    valueFormatter: (v: number | null) => (v != null && v !== 0) ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2 }) : '' },
  { field: 'actual_investments', headerName: 'Факт инвест.', width: 130, type: 'number',
    valueFormatter: (v: number | null) => (v != null && v !== 0) ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2 }) : '' },
  { field: 'agreement1', headerName: 'Согласование 1', width: 160,
    renderCell: (params: GridRenderCellParams) => renderAgreement(params.value) },
  { field: 'agreement2', headerName: 'Согласование 2', width: 160,
    renderCell: (params: GridRenderCellParams) => renderAgreement(params.value) },
  { field: 'status', headerName: 'Статус', width: 140 },
];

const EMPTY_FILTERS: Record<string, unknown> = {
  yearFrom: '', yearTo: '', months: [], kam: [], brand: [], sku: [],
  network_name: [], mechanics: [], channel: [], status: []
};
const EXTRA_FILTERS: ExtraFilter[] = [
  { type: 'year', field: 'yearFrom', label: 'Год от' },
  { type: 'year', field: 'yearTo', label: 'Год до' },
  { type: 'months', field: 'months', label: 'Месяцы' }
];
const PROMO_VISIBLE_FILTERS = ['kam', 'network_name', 'brand', 'sku', 'mechanics', 'channel', 'status'];

// ─── Компонент ─────────────────────────────────────────────────────────────
// role — передаётся из App.jsx (admin / agreement1 / agreement2)
interface PromoAnalysisProps {
  role: string | null;
}

export default function PromoAnalysis({ role }: PromoAnalysisProps) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [tab, setTab] = useState(0);
  const [allSkuOptions, setAllSkuOptions] = useState<string[]>([]);
  const [allNetworkOptions, setAllNetworkOptions] = useState<string[]>([]);
  const [investmentTypes, setInvestmentTypes] = useState<string[]>([]);
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [promoViewOnly, setPromoViewOnly] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [snackbar, setSnackbar] = useState<{ open: boolean; message: string; severity: 'success' | 'error' }>({ open: false, message: '', severity: 'success' });
  const [deletedFilter, setDeletedFilter] = useState(''); // "" = active, "deleted" = only deleted, "all" = both

  // ─── Пользовательский тулбар таблицы ──────────────────────────────────
  const [searchText, setSearchText] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [columnsAnchor, setColumnsAnchor] = useState<HTMLElement | null>(null);
  const [visibleColumns, setVisibleColumns] = useState<Record<string, boolean>>(() => {
    const map: Record<string, boolean> = {};
    COLUMNS.forEach(c => { map[c.field] = true; });
    return map;
  });
  const apiRef = useRef(null);

  // ─── Фильтры и данные ─────────────────────────────────────────────────
  const { meta, filters, setFilters, appliedFilters, persistFilters, handleSearch, handleReset, handlePersistChange, fetchMeta, applyFilters } =
    usePromoFilters(EMPTY_FILTERS, FILTERS_STORAGE_KEY, PERSIST_FLAG_KEY);
  const appliedWithDeleted = useMemo(() => ({
    ...appliedFilters,
    ...(deletedFilter ? { deletedFilter } : {}),
  }), [appliedFilters, deletedFilter]);

  const { rows, loading: dataLoading, refetch } = usePromoData(appliedWithDeleted, tab === 1);
  const { data: dashboardData, isFetching: dashboardLoading, error: dashboardQueryError } = useQuery({
    queryKey: ['promoDashboard', appliedWithDeleted] as const,
    enabled: tab === 0,
    queryFn: () => promoAPI.getDashboard(appliedWithDeleted),
  });
  const dashboardError = dashboardQueryError
    ? (dashboardQueryError instanceof Error ? dashboardQueryError.message : String(dashboardQueryError))
    : null;

  // После редактирования/удаления/создания — сбрасываем кеш и перезапрашиваем
  const handleDataChanged = useCallback(() => {
    // Удаляем все закэшированные данные промо, чтобы гарантировать свежий запрос
    queryClient.removeQueries({ queryKey: ['promoData'] });
    // Принудительный refetch всех запросов промо-данных
    queryClient.refetchQueries({ queryKey: ['promoData'] });
    queryClient.invalidateQueries({ queryKey: ['promoDashboard'] });
  }, [queryClient]);

  // ─── Форма редактирования ─────────────────────────────────────────────
  const { form, setForm, saving, deleting, handleRowClick: formHandleRowClick, handleSave: formHandleSave, handleDelete: formHandleDelete } = 
    usePromoForm({ onEditSuccess: handleDataChanged, onDeleteSuccess: handleDataChanged, onCreateSuccess: handleDataChanged });
  const { recalcPlan, recalcActual } = usePromoCalculations(form);

  // ─── Refetch при возврате на вкладку "Просмотр данных" ────────────────
  useEffect(() => {
    if (tab === 1) refetch();
  }, [tab, refetch]);

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
  const handleRowClick = (params: GridRowParams) => { formHandleRowClick(params.row as PromoRow); setPromoViewOnly(false); setEditDialogOpen(true); };

  const handleSave = async (commentOverride?: string | null) => {
    const result = await formHandleSave(commentOverride); 
    setSnackbar({ open: true, message: result.message, severity: result.success ? 'success' : 'error' }); 
  };

  const handleDelete = async () => { 
    const result = await formHandleDelete(); 
    if (result.success) { setDeleteDialogOpen(false); setEditDialogOpen(false); }
    setSnackbar({ open: true, message: result.message, severity: result.success ? 'success' : 'error' }); 
  };

  const handlePromoFormSave = () => {
    queryClient.invalidateQueries({ queryKey: ['promoData'] });
    queryClient.invalidateQueries({ queryKey: ['promoDashboard'] });
  };

  const handleDashboardDrilldown = useCallback((nextFilters: Record<string, unknown>) => {
    applyFilters({ ...appliedFilters, ...nextFilters });
    setTab(1);
  }, [appliedFilters, applyFilters]);

  const handleHistoryPromoOpen = async (id: number) => {
    try {
      const promo = await promoAPI.getById(id);
      formHandleRowClick(promo);
      setPromoViewOnly(true);
      setEditDialogOpen(true);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setSnackbar({ open: true, message: `❌ Не удалось открыть промо: ${message}`, severity: 'error' });
    }
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

  const toggleColumn = (f: string) => setVisibleColumns(prev => ({ ...prev, [f]: !prev[f] }));

  const getRowClassName = (params: GridRowParams) => {
    const row = params.row as PromoRow;
    return row.deleted_at != null ? 'deleted-row' : '';
  };

  // ─── Экспорт CSV (клиентский — выгружаем отфильтрованные строки) ─────
  const handleExportCSV = () => {
    try {
      const data = filteredRows;
      if (!data.length) { window.alert('Нет данных для выгрузки.'); return; }
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
        const cells = row as unknown as Record<string, unknown>;
        csv += fields.map(f => {
          const raw = cells[f]; if (raw == null) return '';
          let v = String(raw);
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
      if (rows.length > EXPORT_WARNING_THRESHOLD) {
        const ok = window.confirm(
          `Будет выгружено более ${EXPORT_WARNING_THRESHOLD.toLocaleString('ru-RU')} строк (${rows.length.toLocaleString('ru-RU')}). Продолжить?`
        );
        if (!ok) return;
      }
      const qs = new URLSearchParams();
      Object.entries(appliedFilters).forEach(([key, value]) => {
        if (Array.isArray(value)) {
          value.forEach(v => { if (v !== '' && v != null) qs.append(key, String(v)); });
        } else if (value !== '' && value != null) {
          qs.set(key, String(value));
        }
      });
      const resp = await fetch(`${API_BASE}/api/promo/export-xlsx?${qs}`, {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const blob = await resp.blob();
      saveAs(blob, `promo-export_${new Date().toISOString().split('T')[0]}.xlsx`);
    } catch (e) { console.error('Export error:', e); window.alert('Ошибка при выгрузке Excel.'); }
  };

  // ─── Рендер ───────────────────────────────────────────────────────────
  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column', p: 2 }}>
      {/* Шапка */}
      <Stack direction="row" spacing={2} sx={{ mb: 2, alignItems: 'center' }}>
        <Button startIcon={<ArrowBackIcon />} onClick={() => navigate('/')}>На главную</Button>
        <Typography variant="h5" sx={{ fontWeight: 600 }}>Анализ промо</Typography>
        {meta.loading && <CircularProgress size={20} />}
        {rows.length > 0 && tab === 1 &&
          <Typography variant="body2" color="text.secondary">Загружено: {rows.length} записей</Typography>}
      </Stack>

      {/* ─── Вкладки ─────────────────────────────────────────────────── */}
      <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ mb: 2 }}>
        <Tab label="Дашборд" />
        <Tab label="Просмотр данных" />
        <Tab label="Новое промо" />
        {(role === 'admin' || role === 'agreement1' || role === 'agreement2') && (
          <Tab label="Согласование" />
        )}
      </Tabs>

      {(tab === 0 || tab === 1) && (
        <Box sx={{ mb: 2 }}>
          <FilterPanel filters={filters} filterOptions={filterOptions} onFiltersChange={setFilters}
            onSearch={handleSearch} onReset={handleReset} extraFilters={EXTRA_FILTERS}
            persistFilters={persistFilters} onPersistChange={handlePersistChange}
            visibleFilters={PROMO_VISIBLE_FILTERS} />
        </Box>
      )}

      {tab === 0 && (
        <Suspense fallback={<Box sx={{ display: 'grid', flex: 1, placeItems: 'center' }}><CircularProgress /></Box>}>
          <PromoDashboard
            data={dashboardData || null}
            loading={dashboardLoading}
            error={dashboardError}
            onDrilldown={handleDashboardDrilldown}
          />
        </Suspense>
      )}

      {/* ─── Tab 1: Просмотр данных ──────────────────────────────────── */}
      {tab === 1 && (<>
        {meta.error && 
          <Button variant="outlined" color="warning" onClick={() => fetchMeta()} sx={{ mb: 2 }}>
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
                  <ListItemText primary={col.headerName || col.field} slotProps={{ primary: { sx: { fontSize: 13 } } }} />
                </MenuItem>
              ))}
            </Menu>
            <TextField size="small" placeholder="Поиск по таблице..." value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
              slotProps={{ input: { startAdornment: <SearchIcon sx={{ fontSize: 18, color: '#94a3b8', mr: 0.5 }} /> } }}
              sx={{ width: 240, '& .MuiOutlinedInput-root': { bgcolor: '#fff', borderRadius: 2 }, '& .MuiInputBase-input': { fontSize: '0.875rem', py: 0.75 } }} />
            <Box sx={{ flex: 1 }} />
            {role === 'admin' && (
              <TextField
                select
                size="small"
                value={deletedFilter}
                onChange={(e) => setDeletedFilter(e.target.value)}
                label="Состояние"
                sx={{ width: 140, mr: 1, '& .MuiInputBase-input': { fontSize: '0.8rem', py: 0.75 }, '& .MuiInputLabel-root': { fontSize: '0.8rem' } }}
              >
                <MenuItem value="">Актуальные</MenuItem>
                <MenuItem value="all">Все</MenuItem>
                <MenuItem value="deleted">Удалённые</MenuItem>
              </TextField>
            )}
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
            getRowClassName={getRowClassName}
            sx={{ 
              flex: 1, border: '1px solid #e2e8f0', borderTop: 'none',
              borderRadius: '0 0 12px 12px',
              '& .MuiDataGrid-columnHeaders': { borderRadius: 0 },
              '& .MuiDataGrid-row': { cursor: 'pointer' },
              '& .deleted-row': { bgcolor: '#f1f5f9', opacity: 0.7 },
            }} 
          />
        </Box>

      </>)}

      {/* ─── Tab 2: Новое промо ────────────────────────────────────────── */}
      {tab === 2 && (
        <Suspense fallback={<Box sx={{ display: 'grid', flex: 1, placeItems: 'center' }}><CircularProgress /></Box>}>
          <PromoForm onSave={handlePromoFormSave} onOpenPromo={handleHistoryPromoOpen} />
        </Suspense>
      )}

      {/* ─── Tab 3: Согласование ──────────────────────────────────────── */}
      {tab === 3 && (
        <Suspense fallback={<Box sx={{ display: 'grid', flex: 1, placeItems: 'center' }}><CircularProgress /></Box>}>
          <PromoApproval role={role} onDataChanged={() => {
            queryClient.invalidateQueries({ queryKey: ['promoData'] });
            queryClient.invalidateQueries({ queryKey: ['promoDashboard'] });
          }} />
        </Suspense>
      )}

      {/* Карточка доступна и из таблицы, и из истории на форме создания */}
      <PromoEditDialog
        open={editDialogOpen} onClose={() => { setEditDialogOpen(false); setPromoViewOnly(false); }}
        form={form} setForm={setForm} recalcPlan={recalcPlan} recalcActual={recalcActual}
        onSave={handleSave} onDelete={() => setDeleteDialogOpen(true)}
        saving={saving} deleting={deleting} meta={meta}
        allSkuOptions={allSkuOptions} allNetworkOptions={allNetworkOptions}
        investmentTypes={investmentTypes}
        role={role}
        readOnly={promoViewOnly}
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
