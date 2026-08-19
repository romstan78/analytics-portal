import { lazy, Suspense, useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Autocomplete, Box, Button, CircularProgress, Paper, Stack, Tab, Tabs,
  TextField, ToggleButton, ToggleButtonGroup, Typography,
} from '@mui/material';
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material';
import FilterPanel from '../components/FilterPanel';
import DataTable from '../components/DataTable';
import InternetSalesDashboard, { type InternetSalesDashboardData } from '../components/InternetSalesDashboard';
import { salesAPI } from '../api/promo';

const DrilldownModal = lazy(() => import('../components/DrilldownModal'));

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080';
const FILTERS_STORAGE_KEY = 'internet_sales_filters_v8';
const PERSIST_FLAG_KEY = 'internet_sales_persist_v8';

const COLUMNS = [
  { field: 'year', headerName: 'Год', width: 90, type: 'number', valueFormatter: (v) => v },
  { field: 'month', headerName: 'Месяц', width: 80, type: 'number' },
  { field: 'brandName', headerName: 'Бренд', width: 150 },
  { field: 'productName', headerName: 'SKU', width: 250 },
  { field: 'networkName', headerName: 'Сеть', width: 200 },
  { field: 'metricType', headerName: 'Показатель', width: 140 },
  { field: 'metricValue', headerName: 'Значение', width: 130, type: 'number',
    valueFormatter: (v) => v != null ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : '' },
  { field: 'un_rub', headerName: 'Уп/Руб', width: 100 },
  { field: 'segment', headerName: 'Сегмент', width: 170 },
  { field: 'channel', headerName: 'Канал', width: 160 },
  { field: 'updated_at', headerName: 'Обновлено', width: 160 },
  { field: 'id', headerName: 'ID', width: 70, type: 'number' },
];

interface SalesFilters {
  yearFrom: string;
  yearTo: string;
  months: number[];
  quarters: number[];
  brandName: string[];
  productName: string[];
  networkName: string[];
}

const EMPTY_FILTERS: SalesFilters = {
  yearFrom: '', yearTo: '', months: [], quarters: [],
  brandName: [], productName: [], networkName: [],
};

const EXTRA_FILTERS = [
  { type: 'year' as const, field: 'yearFrom', label: 'Год от' },
  { type: 'year' as const, field: 'yearTo', label: 'Год до' },
  { type: 'months' as const, field: 'months', label: 'Месяцы' },
  { type: 'quarters' as const, field: 'quarters', label: 'Кварталы' },
];

interface SalesMeta {
  brandName: string[];
  productName: string[];
  networkName: string[];
  segment: string[];
  channel: string[];
  channelSegmentMap: Record<string, string[]>;
  loading: boolean;
  error: string | null;
}

export default function InternetSales() {
  const navigate = useNavigate();
  const [view, setView] = useState<'dashboard' | 'details'>('dashboard');
  const [meta, setMeta] = useState<SalesMeta>({
    brandName: [], productName: [], networkName: [], segment: [], channel: [], channelSegmentMap: {}, loading: true, error: null,
  });
  const [focusChannel, setFocusChannel] = useState('OLAP SS');
  const [focusSegment, setFocusSegment] = useState('OLAP SS');
  const [unit, setUnit] = useState<'руб' | 'уп'>('руб');
  const [dashboardFocus, setDashboardFocus] = useState<{ type: 'product' | 'network'; name: string } | null>(null);
  const [dashboard, setDashboard] = useState<InternetSalesDashboardData | null>(null);
  const [dashboardLoading, setDashboardLoading] = useState(true);
  const [dashboardError, setDashboardError] = useState('');
  const [rowCount, setRowCount] = useState(0);
  const [drilldownRow, setDrilldownRow] = useState(null);

  const [filters, setFilters] = useState<SalesFilters>(() => {
    try {
      if (localStorage.getItem(PERSIST_FLAG_KEY) === 'true') {
        const saved = sessionStorage.getItem(FILTERS_STORAGE_KEY);
        if (saved) {
          const parsed = JSON.parse(saved);
          if (parsed && Array.isArray(parsed.months)) return { ...EMPTY_FILTERS, ...parsed };
        }
      }
    } catch { /* используем пустые фильтры */ }
    return { ...EMPTY_FILTERS };
  });
  const [appliedFilters, setAppliedFilters] = useState(filters);
  const [persistFilters, setPersistFilters] = useState(() => localStorage.getItem(PERSIST_FLAG_KEY) === 'true');

  useEffect(() => {
    salesAPI.getFilters()
      .then(raw => {
        const data = raw as Record<string, unknown>;
        const segments = (data.segment as string[]) || [];
        const channels = (data.channel as string[]) || [];
        const channelSegmentMap = (data.channelSegmentMap as Record<string, string[]>) || {};
        setMeta({
          brandName: (data.brandName as string[]) || [],
          productName: (data.productName as string[]) || [],
          networkName: (data.networkName as string[]) || [],
          segment: segments,
          channel: channels,
          channelSegmentMap,
          loading: false,
          error: null,
        });
        const defaultChannel = channelSegmentMap['OLAP SS'] ? 'OLAP SS' : (channels[0] || '');
        const defaultSegments = channelSegmentMap[defaultChannel] || segments;
        setFocusChannel(defaultChannel);
        setFocusSegment(defaultSegments.includes('OLAP SS') ? 'OLAP SS' : (defaultSegments[0] || ''));
      })
      .catch((err: Error) => setMeta(prev => ({ ...prev, loading: false, error: err.message })));
  }, []);

  useEffect(() => {
    let active = true;
    setDashboardLoading(true);
    setDashboardError('');
    salesAPI.getDashboard({
      ...appliedFilters,
      focusChannel,
      focusSegment,
      unit,
      ...(dashboardFocus?.type === 'product' ? { focusProduct: dashboardFocus.name } : {}),
      ...(dashboardFocus?.type === 'network' ? { focusNetwork: dashboardFocus.name } : {}),
    })
      .then(raw => { if (active) setDashboard(raw as InternetSalesDashboardData); })
      .catch((err: { message?: string }) => { if (active) setDashboardError(err.message || 'Не удалось загрузить дашборд'); })
      .finally(() => { if (active) setDashboardLoading(false); });
    return () => { active = false; };
  }, [appliedFilters, focusChannel, focusSegment, unit, dashboardFocus]);

  const segmentOptions = useMemo(
    () => meta.channelSegmentMap[focusChannel] || meta.segment,
    [meta.channelSegmentMap, meta.segment, focusChannel],
  );

  const filterOptions = useMemo(() => ({
    brandName: meta.brandName,
    productName: meta.productName,
    networkName: meta.networkName,
  }), [meta]);

  const detailFilters = useMemo(() => ({
    ...appliedFilters,
    segment: focusSegment ? [focusSegment] : [],
    un_rub: [unit],
  }), [appliedFilters, focusSegment, unit]);

  const handleSearch = useCallback(() => {
    setAppliedFilters({ ...filters });
    setDashboardFocus(null);
    if (persistFilters) sessionStorage.setItem(FILTERS_STORAGE_KEY, JSON.stringify(filters));
  }, [filters, persistFilters]);

  const handleReset = useCallback(() => {
    const empty = { ...EMPTY_FILTERS };
    setFilters(empty);
    setAppliedFilters(empty);
    const defaultChannel = meta.channelSegmentMap['OLAP SS'] ? 'OLAP SS' : (meta.channel[0] || '');
    const defaultSegments = meta.channelSegmentMap[defaultChannel] || meta.segment;
    setFocusChannel(defaultChannel);
    setFocusSegment(defaultSegments.includes('OLAP SS') ? 'OLAP SS' : (defaultSegments[0] || ''));
    setUnit('руб');
    setDashboardFocus(null);
    setRowCount(0);
    setDrilldownRow(null);
    sessionStorage.removeItem(FILTERS_STORAGE_KEY);
  }, [meta.channel, meta.channelSegmentMap, meta.segment]);

  const handleChannelChange = useCallback((value: string) => {
    const nextSegments = meta.channelSegmentMap[value] || [];
    setFocusChannel(value);
    setFocusSegment(nextSegments[0] || '');
    setDashboardFocus(null);
  }, [meta.channelSegmentMap]);

  const toggleDashboardFocus = useCallback((type: 'product' | 'network', name: string) => {
    setDashboardFocus(current => current?.type === type && current.name === name ? null : { type, name });
  }, []);

  const handlePersistChange = useCallback((checked: boolean) => {
    setPersistFilters(checked);
    localStorage.setItem(PERSIST_FLAG_KEY, String(checked));
    if (checked) sessionStorage.setItem(FILTERS_STORAGE_KEY, JSON.stringify(filters));
    else sessionStorage.removeItem(FILTERS_STORAGE_KEY);
  }, [filters]);

  const handleRowClick = useCallback((params) => {
    if (params.row.networkName && params.row.brandName) setDrilldownRow(params.row);
  }, []);

  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column', p: 2, bgcolor: '#f8fafc' }}>
      <Stack direction="row" alignItems="center" spacing={2} sx={{ mb: 1.5 }}>
        <Button startIcon={<ArrowBackIcon />} onClick={() => navigate('/')}>На главную</Button>
        <Box>
          <Typography variant="h5" sx={{ fontWeight: 700 }}>Интернет-продажи</Typography>
          <Typography variant="caption" color="text.secondary">Динамика, лидеры и детализация онлайн-канала</Typography>
        </Box>
        {meta.loading && <CircularProgress size={20} />}
        <Box sx={{ flex: 1 }} />
        {view === 'details' && rowCount > 0 && <Typography variant="body2" color="text.secondary">{rowCount.toLocaleString('ru-RU')} строк</Typography>}
      </Stack>

      <Paper variant="outlined" sx={{ p: 1.5, mb: 1.5, borderRadius: 3, borderColor: '#e2e8f0' }}>
        <FilterPanel
          filters={filters}
          filterOptions={filterOptions}
          onFiltersChange={(next) => setFilters(next as SalesFilters)}
          onSearch={handleSearch}
          onReset={handleReset}
          loading={dashboardLoading}
          extraFilters={EXTRA_FILTERS}
          persistFilters={persistFilters}
          onPersistChange={handlePersistChange}
          labels={{ productName: 'SKU' }}
        />
        <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.5} alignItems={{ xs: 'stretch', md: 'center' }} sx={{ mt: 1.5, pt: 1.5, borderTop: '1px solid #e2e8f0' }}>
          <Autocomplete
            size="small"
            options={meta.channel}
            value={focusChannel || null}
            onChange={(_, value) => value && handleChannelChange(value)}
            renderInput={(params) => <TextField {...params} label="Канал" />}
            sx={{ minWidth: 210 }}
            disableClearable
          />
          <Autocomplete
            size="small"
            options={segmentOptions}
            value={focusSegment || null}
            onChange={(_, value) => {
              if (value) {
                setFocusSegment(value);
                setDashboardFocus(null);
              }
            }}
            renderInput={(params) => <TextField {...params} label="Сегмент канала" />}
            sx={{ minWidth: 240 }}
            disableClearable
          />
          <ToggleButtonGroup size="small" exclusive value={unit} onChange={(_, value) => value && setUnit(value)}>
            <ToggleButton value="руб">Рубли</ToggleButton>
            <ToggleButton value="уп">Упаковки</ToggleButton>
          </ToggleButtonGroup>
          <Typography variant="caption" color="text.secondary" sx={{ maxWidth: 310 }}>
            Список сегментов определяется выбранным каналом.
          </Typography>
          <Box sx={{ flex: 1 }} />
          <Tabs value={view} onChange={(_, value) => setView(value)} sx={{ minHeight: 36, '& .MuiTab-root': { minHeight: 36, py: 0.5 } }}>
            <Tab value="dashboard" label="Обзор" />
            <Tab value="details" label="Детализация" />
          </Tabs>
        </Stack>
      </Paper>

      {meta.error && <Typography color="error" sx={{ mb: 1 }}>{meta.error}</Typography>}

      {view === 'dashboard' ? (
        <InternetSalesDashboard
          data={dashboard}
          loading={dashboardLoading}
          error={dashboardError}
          focus={dashboardFocus}
          onProductSelect={(name) => toggleDashboardFocus('product', name)}
          onNetworkSelect={(name) => toggleDashboardFocus('network', name)}
          onSegmentSelect={(name) => {
            setFocusSegment(name);
            setDashboardFocus(null);
          }}
          onClearFocus={() => setDashboardFocus(null)}
        />
      ) : (
        <Box sx={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>
          <DataTable
            columns={COLUMNS}
            apiUrl={`${API_BASE}/api/data`}
            filters={detailFilters}
            exportFileName="internet-sales"
            exportXlsxUrl={`${API_BASE}/api/data/export-xlsx`}
            onDataLoaded={(data) => setRowCount(data.length)}
            onRowClick={handleRowClick}
          />
        </Box>
      )}

      {drilldownRow && (
        <Suspense fallback={<CircularProgress size={24} />}>
          <DrilldownModal open onClose={() => setDrilldownRow(null)} rowData={drilldownRow} appliedFilters={detailFilters} />
        </Suspense>
      )}
    </Box>
  );
}
