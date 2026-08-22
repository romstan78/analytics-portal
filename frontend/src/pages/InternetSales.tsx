import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  Alert, Autocomplete, Box, Button, Chip, CircularProgress, Collapse, FormControlLabel,
  Paper, Stack, Switch, Tab, Tabs, TextField, ToggleButton, ToggleButtonGroup,
  Tooltip, Typography,
} from '@mui/material';
import {
  ArrowBack as ArrowBackIcon,
  Analytics as AnalyticsIcon,
  ExpandMore as ExpandMoreIcon,
  RestartAlt as RestartAltIcon,
  Summarize as SummarizeIcon,
  TableRows as TableRowsIcon,
  Tune as TuneIcon,
} from '@mui/icons-material';
import type { GridColDef } from '@mui/x-data-grid';
import DataTable from '../components/DataTable';
import InternetSalesDashboard, { type DashboardFocus, type InternetSalesDashboardData } from '../components/InternetSalesDashboard';
import InternetSalesSummaryTable, { type SalesPivotGranularity } from '../components/InternetSalesSummaryTable';
import InternetSalesSavedViews, { type InternetSalesViewSnapshot } from '../components/InternetSalesSavedViews';
import { salesAPI } from '../api/promo';

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080';
const FILTERS_STORAGE_KEY = 'internet_sales_filters_v9';
const PERSIST_FLAG_KEY = 'internet_sales_persist_v9';
const TABLE_PREFERENCES_KEY = 'internet_sales_table_columns_v1';
const MONTH_NAMES = ['Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь', 'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь'];

const formatUpdatedAt = (value: string | null) => {
  if (!value) return '';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('ru-RU', { dateStyle: 'short', timeStyle: 'short' }).format(date);
};

const COLUMNS: GridColDef[] = [
  { field: 'year', headerName: 'Год', width: 90, type: 'number', valueFormatter: (v: number) => v },
  { field: 'month', headerName: 'Месяц', width: 120, type: 'number', valueFormatter: (v: number) => MONTH_NAMES[v - 1] || v },
  { field: 'brandName', headerName: 'Бренд', width: 150 },
  { field: 'productName', headerName: 'SKU', width: 250 },
  { field: 'networkName', headerName: 'Сеть', width: 200 },
  { field: 'metricType', headerName: 'Показатель', width: 140 },
  { field: 'metricValue', headerName: 'Значение', width: 130, type: 'number',
    valueFormatter: (v: number | null) => v != null ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : '' },
  { field: 'un_rub', headerName: 'Единица', width: 100, valueFormatter: (v: string | null) => v === 'уп' ? 'шт.' : v || '' },
  { field: 'segment', headerName: 'Сегмент', width: 170 },
  { field: 'channel', headerName: 'Канал', width: 160 },
  { field: 'updated_at', headerName: 'Обновлено', width: 170, valueFormatter: formatUpdatedAt },
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

interface SalesMeta {
  year: string[];
  brandName: string[];
  productName: string[];
  segment: string[];
  channel: string[];
  channelSegmentMap: Record<string, string[]>;
  loading: boolean;
  error: string | null;
}

const normalizeStringList = (value: unknown) => Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : [];

export default function InternetSales() {
  const navigate = useNavigate();
  const [view, setView] = useState<'dashboard' | 'summary' | 'details'>('summary');
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [meta, setMeta] = useState<SalesMeta>({
    year: [], brandName: [], productName: [], segment: [], channel: [], channelSegmentMap: {}, loading: true, error: null,
  });
  const [networkOptions, setNetworkOptions] = useState<string[]>([]);
  const [networkOptionsLoading, setNetworkOptionsLoading] = useState(false);
  const [analysisYear, setAnalysisYear] = useState('');
  const [focusChannel, setFocusChannel] = useState('OLAP SS');
  const [focusSegments, setFocusSegments] = useState<string[]>(['OLAP SS']);
  const [comparisonChannels, setComparisonChannels] = useState<string[]>([]);
  const [unit, setUnit] = useState<'руб' | 'евро' | 'уп'>('руб');
  const [summaryGranularity, setSummaryGranularity] = useState<SalesPivotGranularity>('year');
  const [dashboardFocus, setDashboardFocus] = useState<DashboardFocus[]>([]);
  const [rowCount, setRowCount] = useState(0);

  const [filters, setFilters] = useState<SalesFilters>(() => {
    try {
      if (localStorage.getItem(PERSIST_FLAG_KEY) === 'true') {
        const saved = localStorage.getItem(FILTERS_STORAGE_KEY);
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
      .then(data => {
        const years = normalizeStringList(data.year).sort((a, b) => Number(a) - Number(b));
        const segments = normalizeStringList(data.segment);
        const channels = normalizeStringList(data.channel);
        const channelSegmentMap = data.channelSegmentMap || {};
        setMeta({
          year: years,
          brandName: normalizeStringList(data.brandName),
          productName: normalizeStringList(data.productName),
          segment: segments,
          channel: channels,
          channelSegmentMap,
          loading: false,
          error: null,
        });
        const latestYear = filters.yearTo || filters.yearFrom || years.at(-1) || '';
        setAnalysisYear(latestYear);
        setFilters(current => ({ ...current, yearFrom: latestYear, yearTo: latestYear }));
        const defaultChannel = channelSegmentMap['OLAP SS'] ? 'OLAP SS' : (channels[0] || '');
        const defaultSegments = channelSegmentMap[defaultChannel] || segments;
        setFocusChannel(defaultChannel);
        setFocusSegments(defaultSegments.includes('OLAP SS') ? ['OLAP SS'] : defaultSegments);
      })
      .catch((err: Error) => setMeta(prev => ({ ...prev, loading: false, error: err.message })));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setAppliedFilters(filters);
      if (persistFilters) localStorage.setItem(FILTERS_STORAGE_KEY, JSON.stringify(filters));
    }, 300);
    return () => window.clearTimeout(timer);
  }, [filters, persistFilters]);

  useEffect(() => {
    if (focusSegments.length === 0 || !analysisYear) return;
    let active = true;
    const timer = window.setTimeout(() => {
      setNetworkOptionsLoading(true);
      const filtersWithoutNetworks = { ...filters } as Partial<SalesFilters>;
      delete filtersWithoutNetworks.networkName;
      salesAPI.getNetworkOptions({ ...filtersWithoutNetworks, focusChannel, focusSegments, unit })
        .then(raw => {
          if (!active) return;
          const options = normalizeStringList(raw.networkName);
          setNetworkOptions(options);
          const available = new Set(options);
          setFilters(current => {
            const validNetworks = current.networkName.filter(name => available.has(name));
            return validNetworks.length === current.networkName.length ? current : { ...current, networkName: validNetworks };
          });
          setDashboardFocus(current => current.filter(item => item.type !== 'network' || available.has(item.name)));
        })
        .catch(() => { if (active) setNetworkOptions([]); })
        .finally(() => { if (active) setNetworkOptionsLoading(false); });
    }, 250);
    return () => { active = false; window.clearTimeout(timer); };
    // Зависимости перечислены по полям намеренно: эффект сам правит
    // filters.networkName, и полный объект filters зациклил бы его.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [analysisYear, filters.yearFrom, filters.yearTo, filters.months, filters.quarters, filters.brandName, filters.productName, focusChannel, focusSegments, unit]);

  const dashboardEnabled = view === 'dashboard' && Boolean(analysisYear) && focusSegments.length > 0;
  const { data: dashboardData, isFetching: dashboardFetching, error: dashboardQueryError } = useQuery({
    queryKey: ['salesDashboard', analysisYear, appliedFilters, focusChannel, focusSegments, unit, comparisonChannels, dashboardFocus] as const,
    enabled: dashboardEnabled,
    queryFn: () => salesAPI.getDashboard({
      ...appliedFilters,
      analysisYear,
      focusChannel,
      focusSegments,
      unit,
      compareChannels: comparisonChannels,
      focusProducts: dashboardFocus.filter(item => item.type === 'product').map(item => item.name),
      focusNetworks: dashboardFocus.filter(item => item.type === 'network').map(item => item.name),
    }),
  });

  const dashboard = (dashboardData as InternetSalesDashboardData | undefined) ?? null;
  // До первой применимой комбинации фильтров показываем индикатор, как раньше.
  const dashboardLoading = dashboardEnabled && dashboardFetching;
  const dashboardError = dashboardQueryError
    ? ((dashboardQueryError as { message?: string }).message || 'Не удалось загрузить дашборд')
    : '';

  const segmentOptions = useMemo(
    () => meta.channelSegmentMap[focusChannel] || meta.segment,
    [meta.channelSegmentMap, meta.segment, focusChannel],
  );

  const detailFilters = useMemo(() => ({
    ...appliedFilters,
    segment: focusSegments,
    channel: focusChannel ? [focusChannel] : [],
    un_rub: unit === 'евро' ? [] : [unit],
  }), [appliedFilters, focusChannel, focusSegments, unit]);

  const updateYear = useCallback((year: string) => {
    setAnalysisYear(year);
    setFilters(current => ({ ...current, yearFrom: year, yearTo: year }));
    setDashboardFocus([]);
  }, []);

  const handleReset = useCallback(() => {
    const latestYear = meta.year.at(-1) || analysisYear;
    setAnalysisYear(latestYear);
    setFilters({ ...EMPTY_FILTERS, yearFrom: latestYear, yearTo: latestYear });
    const defaultChannel = meta.channelSegmentMap['OLAP SS'] ? 'OLAP SS' : (meta.channel[0] || '');
    const defaultSegments = meta.channelSegmentMap[defaultChannel] || meta.segment;
    setFocusChannel(defaultChannel);
    setFocusSegments(defaultSegments.includes('OLAP SS') ? ['OLAP SS'] : defaultSegments);
    setComparisonChannels([]);
    setUnit('руб');
    setSummaryGranularity('year');
    setDashboardFocus([]);
    setRowCount(0);
    localStorage.removeItem(FILTERS_STORAGE_KEY);
  }, [analysisYear, meta.channel, meta.channelSegmentMap, meta.segment, meta.year]);

  const handleChannelChange = useCallback((value: string) => {
    const nextSegments = meta.channelSegmentMap[value] || [];
    setFocusChannel(value);
    setFocusSegments(nextSegments);
    setDashboardFocus([]);
  }, [meta.channelSegmentMap]);

  const toggleDashboardFocus = useCallback((type: DashboardFocus['type'], name: string, additive: boolean) => {
    setDashboardFocus(current => {
      const sameType = current.filter(item => item.type === type);
      const exists = sameType.some(item => item.name === name);
      if (!additive) return exists && sameType.length === 1 && current.length === 1 ? [] : [{ type, name }];
      if (current.some(item => item.type !== type)) return [{ type, name }];
      if (exists) return current.filter(item => !(item.type === type && item.name === name));
      return current.length >= 5 ? current : [...current, { type, name }];
    });
  }, []);

  const handlePersistChange = useCallback((checked: boolean) => {
    setPersistFilters(checked);
    localStorage.setItem(PERSIST_FLAG_KEY, String(checked));
    if (checked) localStorage.setItem(FILTERS_STORAGE_KEY, JSON.stringify(filters));
    else localStorage.removeItem(FILTERS_STORAGE_KEY);
  }, [filters]);

  const savedViewSnapshot = useMemo<InternetSalesViewSnapshot>(() => ({
    view,
    analysisYear,
    filters,
    focusChannel,
    focusSegments,
    comparisonChannels,
    unit,
    summaryGranularity,
  }), [analysisYear, comparisonChannels, filters, focusChannel, focusSegments, summaryGranularity, unit, view]);

  const applySavedView = useCallback((snapshot: InternetSalesViewSnapshot) => {
    const nextFilters = {
      ...EMPTY_FILTERS,
      ...snapshot.filters,
      months: [...(snapshot.filters.months || [])],
      quarters: [...(snapshot.filters.quarters || [])],
      brandName: [...(snapshot.filters.brandName || [])],
      productName: [...(snapshot.filters.productName || [])],
      networkName: [...(snapshot.filters.networkName || [])],
    };
    setView(snapshot.view);
    setAnalysisYear(snapshot.analysisYear);
    setFilters(nextFilters);
    setAppliedFilters(nextFilters);
    setFocusChannel(snapshot.focusChannel);
    setFocusSegments([...(snapshot.focusSegments || [])]);
    setComparisonChannels([...(snapshot.comparisonChannels || [])]);
    setUnit(snapshot.unit);
    setSummaryGranularity(snapshot.summaryGranularity || 'year');
    setDashboardFocus([]);
  }, []);

  const activeFilterCount = filters.brandName.length + filters.productName.length + filters.networkName.length + filters.months.length + filters.quarters.length;

  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column', p: 2, bgcolor: '#f6f8fb' }}>
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center', mb: 1.5 }}>
        <Button startIcon={<ArrowBackIcon />} onClick={() => navigate('/')}>На главную</Button>
        <Box>
          <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
            <Typography variant="h5" sx={{ fontWeight: 750 }}>Интернет-продажи</Typography>
            <Chip size="small" color="primary" variant="outlined" label="Иерархическая сводная" />
          </Stack>
          <Typography variant="caption" color="text.secondary">Аналитика, сводная и исходные строки — независимые рабочие режимы</Typography>
        </Box>
        {meta.loading && <CircularProgress size={20} />}
        <Box sx={{ flex: 1 }} />
        {view === 'details' && rowCount > 0 && <Typography variant="body2" color="text.secondary">{rowCount.toLocaleString('ru-RU')} строк</Typography>}
        <Tabs value={view} onChange={(_, value) => setView(value)} sx={{ minHeight: 36, '& .MuiTab-root': { minHeight: 36, py: 0.5 } }}>
          <Tab value="dashboard" icon={<AnalyticsIcon fontSize="small" />} iconPosition="start" label="Аналитика" />
          <Tab value="summary" icon={<SummarizeIcon fontSize="small" />} iconPosition="start" label="Сводная таблица" />
          <Tab value="details" icon={<TableRowsIcon fontSize="small" />} iconPosition="start" label="Исходные данные" />
        </Tabs>
      </Stack>

      <Paper variant="outlined" sx={{ p: 1.5, mb: 1.5, borderRadius: 3, borderColor: '#dfe5ee' }}>
        <Stack direction={{ xs: 'column', lg: 'row' }} spacing={1.25} sx={{ alignItems: { xs: 'stretch', lg: 'center' } }}>
          <ToggleButtonGroup size="small" exclusive value={analysisYear} onChange={(_, value) => value && updateYear(value)}>
            {meta.year.slice(-4).map(year => <ToggleButton key={year} value={year}>{year}</ToggleButton>)}
          </ToggleButtonGroup>
          <ToggleButtonGroup size="small" value={filters.quarters.length ? filters.quarters.map(String) : ['all']}
            onChange={(_, values: string[]) => setFilters(current => {
              if (current.quarters.length > 0 && values.includes('all')) return { ...current, quarters: [], months: [] };
              const quarters = values.filter(value => value !== 'all').map(Number).filter(value => value >= 1 && value <= 4);
              return { ...current, quarters, months: [] };
            })}>
            <ToggleButton value="all">Весь год</ToggleButton>
            {[1, 2, 3, 4].map(quarter => <ToggleButton key={quarter} value={String(quarter)}>Q{quarter}</ToggleButton>)}
          </ToggleButtonGroup>
          <Autocomplete multiple size="small" options={meta.brandName} value={filters.brandName}
            onChange={(_, values) => setFilters(current => ({ ...current, brandName: values }))}
            renderInput={(params) => <TextField {...params} label="Бренд" />}
            limitTags={1} sx={{ minWidth: 190, flex: 1 }} />
          <Autocomplete multiple size="small" options={meta.productName} value={filters.productName}
            onChange={(_, values) => setFilters(current => ({ ...current, productName: values }))}
            renderInput={(params) => <TextField {...params} label="SKU" />}
            limitTags={1} sx={{ minWidth: 210, flex: 1.2 }} />
          <Tooltip title="Список содержит только сети, по которым есть данные при выбранных фильтрах">
            <Box sx={{ minWidth: 210, flex: 1.2 }}>
              <Autocomplete multiple size="small" options={networkOptions} value={filters.networkName} loading={networkOptionsLoading}
                onChange={(_, values) => setFilters(current => ({ ...current, networkName: values }))}
                renderInput={(params) => <TextField {...params} label="Сеть" />}
                limitTags={1} />
            </Box>
          </Tooltip>
          <ToggleButtonGroup size="small" exclusive value={unit} onChange={(_, value) => {
            if (!value) return;
            setUnit(value);
            if (value === 'евро') {
              setRowCount(0);
            }
          }}>
            <ToggleButton value="руб">₽</ToggleButton>
            <ToggleButton value="уп">Шт.</ToggleButton>
            <Tooltip title="Расчёт по среднему официальному курсу ЦБ РФ за месяц">
              <ToggleButton value="евро">€</ToggleButton>
            </Tooltip>
          </ToggleButtonGroup>
        </Stack>

        <Stack direction="row" spacing={1} sx={{ alignItems: 'center', mt: 1 }}>
          <Button size="small" startIcon={<TuneIcon />} endIcon={<ExpandMoreIcon sx={{ transform: advancedOpen ? 'rotate(180deg)' : 'none', transition: '0.2s' }} />}
            onClick={() => setAdvancedOpen(value => !value)}>Дополнительные фильтры</Button>
          {activeFilterCount > 0 && <Chip size="small" label={`Активно: ${activeFilterCount}`} />}
          <Button size="small" color="inherit" startIcon={<RestartAltIcon />} onClick={handleReset}>Сбросить</Button>
          <Box sx={{ flex: 1 }} />
          {view === 'dashboard' && dashboardLoading && <Stack direction="row" spacing={0.75} sx={{ alignItems: 'center' }}><CircularProgress size={15} /><Typography variant="caption" color="text.secondary">Обновление</Typography></Stack>}
        </Stack>

        <Collapse in={advancedOpen}>
          <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.25} sx={{ alignItems: { xs: 'stretch', md: 'center' }, mt: 1.25, pt: 1.25, borderTop: '1px solid #e7ebf1' }}>
            <Autocomplete size="small" options={meta.channel} value={focusChannel || undefined} disableClearable
              onChange={(_, value) => value && handleChannelChange(value)}
              renderInput={(params) => <TextField {...params} label="Канал" />} sx={{ minWidth: 190 }} />
            <Autocomplete multiple size="small" options={segmentOptions} value={focusSegments} disableClearable
              onChange={(_, values) => { if (values.length > 0) { setFocusSegments(values); setDashboardFocus([]); } }}
              renderInput={(params) => <TextField {...params} label="Сегменты канала" />}
              limitTags={2} sx={{ minWidth: 245 }} />
            {view === 'dashboard' && (
              <Autocomplete multiple size="small" options={meta.channel} value={comparisonChannels}
                onChange={(_, values) => setComparisonChannels(values.slice(0, 5))}
                renderInput={(params) => <TextField {...params} label="Сравнить каналы" />}
                limitTags={1} sx={{ width: 190, minWidth: 190, flex: '0 0 190px' }} />
            )}
            <Autocomplete multiple size="small" options={Array.from({ length: 12 }, (_, index) => index + 1)} value={filters.months}
              getOptionLabel={(value) => new Intl.DateTimeFormat('ru-RU', { month: 'short' }).format(new Date(2026, value - 1, 1))}
              onChange={(_, values) => setFilters(current => ({ ...current, months: values, quarters: [] }))}
              renderInput={(params) => <TextField {...params} label="Отдельные месяцы" />}
              limitTags={1} sx={{ minWidth: 210 }} />
            <FormControlLabel control={<Switch size="small" checked={persistFilters} onChange={(_, checked) => handlePersistChange(checked)} />} label={<Typography variant="caption">Запомнить</Typography>} />
          </Stack>
          {view === 'dashboard' && (
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.75 }}>
              Ctrl/⌘ + клик по сети или SKU добавляет до пяти рядов на сравнительный график.
            </Typography>
          )}
        </Collapse>

        <Box sx={{ mt: 1.25, pt: 1.25, borderTop: '1px solid #e7ebf1' }}>
          <InternetSalesSavedViews current={savedViewSnapshot} onApply={applySavedView} />
        </Box>
      </Paper>

      {meta.error && <Typography color="error" sx={{ mb: 1 }}>{meta.error}</Typography>}

      {view === 'dashboard' ? (
        <InternetSalesDashboard
          data={dashboard}
          loading={dashboardLoading}
          error={dashboardError}
          focuses={dashboardFocus}
          onProductSelect={(name, additive) => toggleDashboardFocus('product', name, additive)}
          onNetworkSelect={(name, additive) => toggleDashboardFocus('network', name, additive)}
          onSegmentSelect={(name) => { setFocusSegments([name]); setDashboardFocus([]); }}
          onRemoveFocus={(focus) => setDashboardFocus(current => current.filter(item => item.type !== focus.type || item.name !== focus.name))}
          onClearFocus={() => setDashboardFocus([])}
        />
      ) : view === 'summary' ? (
        <InternetSalesSummaryTable
          analysisYear={analysisYear}
          filters={{ ...appliedFilters }}
          channel={focusChannel}
          segments={focusSegments}
          unit={unit}
          granularity={summaryGranularity}
          onGranularityChange={setSummaryGranularity}
        />
      ) : unit === 'евро' ? (
        <Alert severity="info" action={<Button color="inherit" size="small" onClick={() => setUnit('руб')}>Показать в ₽</Button>}>
          Исходные строки хранятся в рублях и упаковках. Пересчёт в евро доступен в аналитике и сводной таблице.
        </Alert>
      ) : (
        <Box sx={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>
          <DataTable columns={COLUMNS} apiUrl={`${API_BASE}/api/data`} filters={detailFilters}
            exportFileName="internet-sales" exportXlsxUrl={`${API_BASE}/api/data/export-xlsx`}
            backgroundExportUrl={`${API_BASE}/api/data/export-jobs`}
            defaultHiddenColumns={['updated_at', 'id']} preferencesKey={TABLE_PREFERENCES_KEY}
            onDataLoaded={(_, totalRows) => setRowCount(totalRows)} />
        </Box>
      )}
    </Box>
  );
}
