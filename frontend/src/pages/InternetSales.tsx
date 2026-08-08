import { useState, useEffect, useMemo, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button, Stack, Box, Typography, CircularProgress } from '@mui/material';
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material';
import FilterPanel from '../components/FilterPanel';
import DataTable from '../components/DataTable';
import DrilldownModal from '../components/DrilldownModal';
import { salesAPI } from '../api/promo';

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080';
const FILTERS_STORAGE_KEY = 'internet_sales_filters_v7';
const PERSIST_FLAG_KEY = 'internet_sales_persist_v7';

const COLUMNS = [
  { field: 'year', headerName: 'Год', width: 90, type: 'number', valueFormatter: (v) => v },
  { field: 'month', headerName: 'Месяц', width: 80, type: 'number' },
  { field: 'brandName', headerName: 'Бренд', width: 150 },
  { field: 'productName', headerName: 'Продукт', width: 250 },
  { field: 'networkName', headerName: 'Сеть', width: 200 },
  { field: 'metricType', headerName: 'Показатель', width: 140 },
  { field: 'metricValue', headerName: 'Значение', width: 130, type: 'number',
    valueFormatter: (v) => v != null ? Number(v).toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : '' },
  { field: 'un_rub', headerName: 'Уп/Руб', width: 100 },
  { field: 'segment', headerName: 'Сегмент', width: 150 },
  { field: 'channel', headerName: 'Канал', width: 150 },
  { field: 'updated_at', headerName: 'Обновлено', width: 160 },
  { field: 'id', headerName: 'ID', width: 70, type: 'number' },
];

const EMPTY_FILTERS = {
  yearFrom: '', yearTo: '', months: [],
  brandName: [], networkName: [], un_rub: [], segment: [], channel: []
};
const EXTRA_FILTERS = [
  { type: 'year', field: 'yearFrom', label: 'Год от' },
  { type: 'year', field: 'yearTo', label: 'Год до' },
  { type: 'months', field: 'months', label: 'Месяцы' }
];

export default function InternetSales() {
  const navigate = useNavigate();

  const [meta, setMeta] = useState({
    brandName: [], networkName: [], un_rub: [], segment: [], channel: [],
    segmentChannelMap: {}, channelSegmentMap: {},
    loading: true, error: null
  });

  const [filters, setFilters] = useState(() => {
    try {
      if (localStorage.getItem(PERSIST_FLAG_KEY) === 'true') {
        const saved = sessionStorage.getItem(FILTERS_STORAGE_KEY);
        if (saved) {
          const parsed = JSON.parse(saved);
          if (parsed && Array.isArray(parsed.months)) return parsed;
        }
      }
    } catch (e) {}
    return { ...EMPTY_FILTERS };
  });

  const [persistFilters, setPersistFilters] = useState(
    () => localStorage.getItem(PERSIST_FLAG_KEY) === 'true'
  );
  const [appliedFilters, setAppliedFilters] = useState(filters);
  const [rowCount, setRowCount] = useState(0);
  const [drilldownRow, setDrilldownRow] = useState(null);

  // Загрузка справочников через API-слой
  useEffect(() => {
    salesAPI.getFilters()
      .then(data => setMeta({
        brandName: data.brandName || [],
        networkName: data.networkName || [],
        un_rub: data.un_rub || [],
        segment: data.segment || [],
        channel: data.channel || [],
        segmentChannelMap: data.segmentChannelMap || {},
        channelSegmentMap: data.channelSegmentMap || {},
        loading: false,
        error: null
      }))
      .catch(err => setMeta(prev => ({ ...prev, loading: false, error: err.message })));
  }, []);

  // Видимые опции фильтров с учётом каскада
  const filterOptions = useMemo(() => {
    let segments = [...meta.segment];
    const channels = [...meta.channel];

    if (filters.channel.length > 0) {
      const allowed = new Set();
      filters.channel.forEach(ch => {
        (meta.channelSegmentMap[ch] || []).forEach(seg => allowed.add(seg));
      });
      segments = segments.filter(seg => allowed.has(seg));
    }

    return {
      brandName: meta.brandName,
      networkName: meta.networkName,
      un_rub: meta.un_rub,
      channel: channels,
      segment: segments,
    };
  }, [meta, filters.channel]);

  // Каскадная фильтрация
  const handleFiltersChange = useCallback((newFilters) => {
    let updated = { ...newFilters };

    if (JSON.stringify(newFilters.segment) !== JSON.stringify(filters.segment)) {
      const addedSegments = newFilters.segment.filter(seg => !filters.segment.includes(seg));
      const removedSegments = filters.segment.filter(seg => !newFilters.segment.includes(seg));

      if (addedSegments.length > 0) {
        const channelsToAdd = new Set(updated.channel);
        addedSegments.forEach(seg => {
          (meta.segmentChannelMap[seg] || []).forEach(ch => channelsToAdd.add(ch));
        });
        updated.channel = Array.from(channelsToAdd);
      }

      if (removedSegments.length > 0) {
        const channelsToRemove = new Set();
        removedSegments.forEach(seg => {
          const linked = meta.segmentChannelMap[seg] || [];
          linked.forEach(ch => {
            const allSegs = meta.channelSegmentMap[ch] || [];
            if (allSegs.filter(s => updated.segment.includes(s)).length === 0) {
              channelsToRemove.add(ch);
            }
          });
        });
        updated.channel = updated.channel.filter(ch => !channelsToRemove.has(ch));
      }
    }

    if (JSON.stringify(newFilters.channel) !== JSON.stringify(filters.channel)) {
      const removedChannels = filters.channel.filter(ch => !newFilters.channel.includes(ch));
      const addedChannels = newFilters.channel.filter(ch => !filters.channel.includes(ch));

      if (removedChannels.length > 0) {
        const segsToRemove = new Set();
        removedChannels.forEach(ch => {
          (meta.channelSegmentMap[ch] || []).forEach(seg => segsToRemove.add(seg));
        });
        updated.segment = updated.segment.filter(seg => !segsToRemove.has(seg));
      }

      if (addedChannels.length > 0) {
        const segsToAdd = new Set(updated.segment);
        addedChannels.forEach(ch => {
          (meta.channelSegmentMap[ch] || []).forEach(seg => segsToAdd.add(seg));
        });
        updated.segment = Array.from(segsToAdd);
      }
    }

    setFilters(updated);
  }, [filters, meta]);

  const handleSearch = useCallback(() => {
    setAppliedFilters({ ...filters });
    if (persistFilters) sessionStorage.setItem(FILTERS_STORAGE_KEY, JSON.stringify(filters));
  }, [filters, persistFilters]);

  const handleReset = useCallback(() => {
    const empty = { ...EMPTY_FILTERS };
    setFilters(empty);
    setAppliedFilters(empty);
    sessionStorage.removeItem(FILTERS_STORAGE_KEY);
    setRowCount(0);
    setDrilldownRow(null);
  }, []);

  const handlePersistChange = useCallback((checked) => {
    setPersistFilters(checked);
    localStorage.setItem(PERSIST_FLAG_KEY, String(checked));
    if (checked) {
      sessionStorage.setItem(FILTERS_STORAGE_KEY, JSON.stringify(filters));
    } else {
      sessionStorage.removeItem(FILTERS_STORAGE_KEY);
    }
  }, [filters]);

  const handleDataLoaded = useCallback((data) => setRowCount(data.length), []);
  const handleRowClick = useCallback((params) => {
    if (params.row.networkName && params.row.brandName) setDrilldownRow(params.row);
  }, []);
  const handleCloseDrilldown = useCallback(() => setDrilldownRow(null), []);

  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column', p: 2 }}>
      <Stack direction="row" alignItems="center" spacing={2} sx={{ mb: 2 }}>
        <Button startIcon={<ArrowBackIcon />} onClick={() => navigate('/')}>На главную</Button>
        <Typography variant="h5" sx={{ fontWeight: 600 }}>Интернет-продажи</Typography>
        {meta.loading && <CircularProgress size={20} />}
        {rowCount > 0 && (
          <Typography variant="body2" color="text.secondary">
            Загружено: {rowCount.toLocaleString('ru-RU')} строк
          </Typography>
        )}
      </Stack>

      <Box sx={{ mb: 2 }}>
        <FilterPanel
          filters={filters}
          filterOptions={filterOptions}
          onFiltersChange={handleFiltersChange}
          onSearch={handleSearch}
          onReset={handleReset}
          extraFilters={EXTRA_FILTERS}
          persistFilters={persistFilters}
          onPersistChange={handlePersistChange}
        />
      </Box>

      {meta.error && (
        <Button
          variant="outlined" color="warning"
          onClick={() => window.location.reload()}
          sx={{ mb: 2, alignSelf: 'flex-start' }}
        >
          Ошибка загрузки справочников
        </Button>
      )}

      <Box sx={{ flex: 1, overflow: 'hidden' }}>
        <DataTable
          columns={COLUMNS}
          apiUrl={`${API_BASE}/api/data`}
          filters={appliedFilters}
          exportFileName="internet-sales"
          exportXlsxUrl={`${API_BASE}/api/data/export-xlsx`}
          onDataLoaded={handleDataLoaded}
          onRowClick={handleRowClick}
        />
      </Box>

      <DrilldownModal
        open={!!drilldownRow}
        onClose={handleCloseDrilldown}
        rowData={drilldownRow}
        appliedFilters={appliedFilters}
      />
    </Box>
  );
}