import { useState, useEffect, useCallback, useMemo, startTransition } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Box, Typography, CircularProgress, Alert, Snackbar,
  TextField, MenuItem, Dialog,
  DialogTitle, DialogContent, DialogActions,
  Paper, Stack, Button, ToggleButtonGroup, ToggleButton,
  Chip,
  Checkbox, Table, TableBody, TableCell, TableContainer,
  TableHead, TableRow, TablePagination,
  Drawer, Accordion, AccordionSummary, AccordionDetails,
  FormControlLabel, InputAdornment,
} from '@mui/material';
import {
  ViewModule as CardIcon,
  TableRows as TableIcon,
  Settings as SettingsIcon,
  ExpandMore,
  Search as SearchIcon,
} from '@mui/icons-material';
import ApprovalCard from '../components/ApprovalCard';
import ApprovalDetailPanel from '../components/ApprovalDetailPanel';
import { promoAPI } from '../api/promo';
import type { ApprovalRow, ApprovalsResponse } from '../types/promo';
import { FIELD_GROUPS, DEFAULT_VISIBLE_FIELDS, normalizeVisibleFields } from '../utils/cardFields';

type ApprovalRoleName = 'agreement1' | 'agreement2';

// promoAPI бросает объект { status, message }, а не Error.
interface ApiErrorLike {
  status?: number;
  message?: string;
}

const asApiError = (err: unknown): ApiErrorLike =>
  typeof err === 'object' && err !== null ? err as ApiErrorLike : {};

interface ConfirmDialogState {
  open: boolean;
  id: number | null;
  updatedAt: string;
  status: string;
  comment: string;
  warning: boolean;
  currentStatus: string;
}

const MONTHS = [
  { label: 'Январь', value: 1 }, { label: 'Февраль', value: 2 }, { label: 'Март', value: 3 },
  { label: 'Апрель', value: 4 }, { label: 'Май', value: 5 }, { label: 'Июнь', value: 6 },
  { label: 'Июль', value: 7 }, { label: 'Август', value: 8 }, { label: 'Сентябрь', value: 9 },
  { label: 'Октябрь', value: 10 }, { label: 'Ноябрь', value: 11 }, { label: 'Декабрь', value: 12 },
];

const APPROVAL_STATUSES = [
  { label: 'На согласовании', value: 'pending' },
  { label: 'С комментариями', value: 'commented' },
  { label: 'Согласовано', value: 'approved' },
  { label: 'Отклонено', value: 'rejected' },
  { label: 'Все', value: 'all' },
];

interface PromoApprovalProps {
  role: string | null;
  onDataChanged?: () => void;
}

export default function PromoApproval({ role, onDataChanged }: PromoApprovalProps) {
  const queryClient = useQueryClient();
  const [adminApprovalRole, setAdminApprovalRole] = useState<ApprovalRoleName>('agreement1');
  // Ступень, которую запрашивает интерфейс. У роли kam её в роли нет: ступень
  // задаёт закрепление за КАМами, и сервер подставит собственную. Отправляемое
  // значение остаётся неизменным, иначе ключ запроса зависел бы от ответа.
  const requestApprovalRole: ApprovalRoleName = role === 'admin'
    ? adminApprovalRole
    : (role === 'agreement2' ? 'agreement2' : 'agreement1');
  // Основной вид: очередь + детали. Карточки оставлены как дополнительный режим.
  const [viewMode, setViewMode] = useState('workspace');

  // Настройка карточек (Drawer)
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [searchField, setSearchField] = useState('');
  const [visibleFields, setVisibleFields] = useState<string[]>(() => {
    try {
      const saved = localStorage.getItem('promo_approval_fields_v3');
      return saved ? normalizeVisibleFields(JSON.parse(saved)) : [...DEFAULT_VISIBLE_FIELDS];
    } catch { return DEFAULT_VISIBLE_FIELDS; }
  });

  const toggleField = (id: string) => {
    setVisibleFields(prev => {
      const next = prev.includes(id) ? prev.filter(f => f !== id) : [...prev, id];
      localStorage.setItem('promo_approval_fields_v3', JSON.stringify(next));
      return next;
    });
  };

  const resetVisibleFields = () => {
    const defaults = [...DEFAULT_VISIBLE_FIELDS];
    localStorage.setItem('promo_approval_fields_v3', JSON.stringify(defaults));
    setVisibleFields(defaults);
  };

  // Черновики фильтров (меняются сразу)
  const [draftKam, setDraftKam] = useState('');
  const [draftNetwork, setDraftNetwork] = useState('');
  const [draftBrand, setDraftBrand] = useState('');
  const [draftMechanics, setDraftMechanics] = useState('');
  const [draftStatus, setDraftStatus] = useState('pending');
  const [draftYear, setDraftYear] = useState(String(new Date().getFullYear()));
  const [draftMonth, setDraftMonth] = useState('');
  const [draftHasComments, setDraftHasComments] = useState(false);

  // Флаг: была ли нажата кнопка «Применить»
  const [hasApplied, setHasApplied] = useState(false);

  // Применённые фильтры
  const [appliedKam, setAppliedKam] = useState('');
  const [appliedNetwork, setAppliedNetwork] = useState('');
  const [appliedBrand, setAppliedBrand] = useState('');
  const [appliedMechanics, setAppliedMechanics] = useState('');
  const [appliedStatus, setAppliedStatus] = useState('pending');
  const [appliedYear, setAppliedYear] = useState(String(new Date().getFullYear()));
  const [appliedMonth, setAppliedMonth] = useState('');
  const [appliedHasComments, setAppliedHasComments] = useState(false);

  // Справочники
  const [kams, setKams] = useState<string[]>([]);
  const [networks, setNetworks] = useState<string[]>([]);
  const [brands, setBrands] = useState<string[]>([]);
  const [mechanicsOptions, setMechanicsOptions] = useState<string[]>([]);

  const [activeApprovalId, setActiveApprovalId] = useState<number | null>(null);
  const [expandedCards, setExpandedCards] = useState<Record<number, boolean>>({});
  const [submitting, setSubmitting] = useState<Record<number, boolean>>({});

  // Массовое согласование
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [batchDialog, setBatchDialog] = useState<{ open: boolean; status: string }>({ open: false, status: '' });

  const [confirmDialog, setConfirmDialog] = useState<ConfirmDialogState>({ open: false, id: null, updatedAt: '', status: '', comment: '', warning: false, currentStatus: 'pending' });
  const [snackbar, setSnackbar] = useState<{ open: boolean; message: string; severity: 'success' | 'error' }>({ open: false, message: '', severity: 'success' });
  const [refreshFilters, setRefreshFilters] = useState(0);

  // Пагинация
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(50);

  // Загрузка справочников
  useEffect(() => {
    promoAPI.getApprovalFilters({
      approval_role: requestApprovalRole,
      approval_status: appliedStatus,
      kam: appliedKam,
      network_name: appliedNetwork,
      brand: appliedBrand,
      mechanics: appliedMechanics,
      year: appliedYear,
      month: appliedMonth,
    })
      .then(data => {
        setKams(data.kams || []);
        setNetworks(data.networks || []);
        setBrands(data.brands || []);
        setMechanicsOptions(data.mechanics || []);
      })
      .catch(err => console.error('Ошибка справочников:', err));
  }, [requestApprovalRole, appliedStatus, appliedKam, appliedNetwork, appliedBrand, appliedMechanics, appliedYear, appliedMonth, refreshFilters]);

  // Загрузка данных. Ключ содержит все применённые фильтры и страницу,
  // поэтому устаревшие ответы отбрасываются самим React Query.
  const approvalsQueryKey = useMemo(() => [
    'approvals', requestApprovalRole, appliedKam, appliedStatus, appliedYear, appliedMonth,
    appliedNetwork, appliedBrand, appliedMechanics, appliedHasComments, page, pageSize,
  ] as const, [requestApprovalRole, appliedKam, appliedStatus, appliedYear, appliedMonth, appliedNetwork, appliedBrand, appliedMechanics, appliedHasComments, page, pageSize]);

  const approvalsQuery = useQuery({
    queryKey: approvalsQueryKey,
    enabled: hasApplied,
    queryFn: () => promoAPI.getApprovals({
      approval_role: requestApprovalRole,
      kam: appliedKam || undefined,
      approval_status: appliedStatus,
      year: appliedYear,
      month: appliedMonth,
      network_name: appliedNetwork || undefined,
      brand: appliedBrand || undefined,
      mechanics: appliedMechanics || undefined,
      has_comments: appliedHasComments || undefined,
      page,
      pageSize,
    }),
  });

  const { data: approvalsData, isFetching, error: approvalsError, refetch } = approvalsQuery;

  // Действующая ступень — та, на которой сервер выполнил запрос. Для КАМа с
  // закреплением она может отличаться от запрошенной, и подписи, выбор поля
  // статуса и сами действия обязаны идти за ней, а не за отправленным значением.
  const approvalRole: ApprovalRoleName = approvalsData?.approval_role === 'agreement2'
    ? 'agreement2'
    : approvalsData?.approval_role === 'agreement1'
      ? 'agreement1'
      : requestApprovalRole;

  const approvals: ApprovalRow[] = useMemo(
    () => (hasApplied ? (approvalsData?.data ?? []) : []),
    [hasApplied, approvalsData],
  );
  const total = hasApplied ? (approvalsData?.total ?? 0) : 0;
  const loading = hasApplied && isFetching;
  // При сетевом сбое React Query переводит запрос в состояние paused и не
  // отдаёт ошибку, поэтому такое состояние показываем отдельно.
  const connectionPaused = approvalsQuery.fetchStatus === 'paused' && approvalsQuery.failureCount > 0;
  const error = approvalsError
    ? (asApiError(approvalsError).message || 'Ошибка загрузки')
    : (connectionPaused ? 'Нет связи с сервером. Запрос возобновится после восстановления соединения.' : null);

  const fetchApprovals = useCallback(async () => { await refetch(); }, [refetch]);

  // Оптимистичное удаление обработанных карточек из текущей выдачи.
  const removeApprovalsFromCache = useCallback((shouldRemove: (id: number) => boolean) => {
    queryClient.setQueryData(approvalsQueryKey, (current: ApprovalsResponse | undefined) => {
      if (!current) return current;
      const rows = (current.data || []).filter(item => !shouldRemove(item.id));
      return { ...current, data: rows };
    });
  }, [queryClient, approvalsQueryKey]);

  // Новая выдача сбрасывает выделение, активная карточка остаётся, если она в списке.
  const [appliedApprovals, setAppliedApprovals] = useState<ApprovalRow[] | null>(null);
  if (appliedApprovals !== approvals) {
    setAppliedApprovals(approvals);
    setSelectedIds(new Set());
    setActiveApprovalId(currentId => (
      approvals.some(item => item.id === currentId) ? currentId : approvals[0]?.id ?? null
    ));
  }

  const handleApply = () => {
    setHasApplied(true);
    setAppliedKam(draftKam);
    setAppliedNetwork(draftNetwork);
    setAppliedBrand(draftBrand);
    setAppliedMechanics(draftMechanics);
    setAppliedStatus(draftStatus);
    setAppliedYear(draftYear);
    setAppliedMonth(draftMonth);
    setAppliedHasComments(draftHasComments);
    setPage(0); // сброс страницы при новом поиске
  };

  const handleReset = () => {
    setDraftKam(''); setDraftNetwork(''); setDraftBrand(''); setDraftMechanics('');
    setDraftStatus('pending'); setDraftYear(String(new Date().getFullYear())); setDraftMonth('');
    setAppliedKam(''); setAppliedNetwork(''); setAppliedBrand(''); setAppliedMechanics('');
    setAppliedStatus('pending'); setAppliedYear(String(new Date().getFullYear())); setAppliedMonth('');
    setDraftHasComments(false); setAppliedHasComments(false);
    // hasApplied = false отключает запрос: список и счётчик обнуляются сами.
    setActiveApprovalId(null);
    setSelectedIds(new Set());
    setHasApplied(false);
    setPage(0);
  };

  const openConfirm = (id: number, status: string, comment: string) => {
    const item = approvals.find(a => a.id === id);
    const statusField = approvalRole === 'agreement2' ? 'agreement2_status' : 'agreement1_status';
    const currentStatus = item?.[statusField] || 'pending';

    // Предупреждение: если статус уже approved/rejected и меняем его на согласовано/отклонено
    const needsWarning = (currentStatus === 'approved' || currentStatus === 'rejected') &&
                         (status === 'согласовано' || status === 'отклонено');

    setConfirmDialog({
      open: true, id, updatedAt: item?.updated_at || '', status, comment: comment || '',
      warning: needsWarning,
      currentStatus,
    });
  };

  const handleConfirmedAction = async () => {
    const { id, updatedAt, status, comment } = confirmDialog;
    setConfirmDialog({ open: false, id: null, updatedAt: '', status: '', comment: '', warning: false, currentStatus: 'pending' });
    if (!id) return;
    setSubmitting(prev => ({ ...prev, [id]: true }));
    try {
      await promoAPI.approve(id, updatedAt, status, comment, approvalRole);
      // Инвалидируем кэш комментариев для этой карточки
      queryClient.invalidateQueries({ queryKey: ['comments', id] });
      // Инвалидируем approvals
      queryClient.invalidateQueries({ queryKey: ['approvals'] });
      // Убираем карточку из текущей выдачи сразу, не дожидаясь ответа сервера.
      removeApprovalsFromCache(promoId => promoId === id);
      setSnackbar({ open: true, message: status === 'comment' ? '✅ Комментарий сохранён' : '✅ Выполнено', severity: 'success' });
      setRefreshFilters(prev => prev + 1);
      if (onDataChanged) onDataChanged();
    } catch (err: unknown) {
      const apiError = asApiError(err);
      setSnackbar({ open: true, message: '❌ Ошибка: ' + (apiError.message || 'не удалось'), severity: 'error' });
      if (apiError.status === 409 || apiError.status === 404) {
        await fetchApprovals();
      }
    } finally {
      setSubmitting(prev => ({ ...prev, [id]: false }));
    }
  };

  const handleCommentOnly = async (id: number, comment: string) => {
    if (!comment || !comment.trim()) return false;
    return handleQuickAction(id, 'comment', comment);
  };

  // Быстрое действие без диалога подтверждения (comment-only).
  // После сохранения инвалидируем кэш комментариев — карточка остаётся.
  const handleQuickAction = async (id: number, status: string, comment: string) => {
    setSubmitting(prev => ({ ...prev, [id]: true }));
    try {
      const item = approvals.find(a => a.id === id);
      await promoAPI.approve(id, item?.updated_at || '', status, comment, approvalRole);
      // Инвалидируем кэш комментариев — useQuery в ApprovalCard перезапросит
      queryClient.invalidateQueries({ queryKey: ['comments', id] });
      // Инвалидируем approvals
      queryClient.invalidateQueries({ queryKey: ['approvals'] });
      setSnackbar({ open: true, message: '✅ Комментарий сохранён', severity: 'success' });
      setRefreshFilters(prev => prev + 1);
      await fetchApprovals();
      if (onDataChanged) onDataChanged();
      return true;
    } catch (err: unknown) {
      const apiError = asApiError(err);
      setSnackbar({ open: true, message: '❌ Ошибка: ' + (apiError.message || 'не удалось'), severity: 'error' });
      if (apiError.status === 409 || apiError.status === 404) {
        await fetchApprovals();
      }
      return false;
    } finally {
      setSubmitting(prev => ({ ...prev, [id]: false }));
    }
  };
  const toggleExpand = (id: number) => setExpandedCards(prev => ({ ...prev, [id]: !prev[id] }));

  // Чекбоксы
  const toggleSelect = (id: number) => {
    setSelectedIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  };
  const toggleSelectAll = () => {
    const allIds = approvals.map(a => a.id);
    if (selectedIds.size === allIds.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(allIds));
    }
  };

  // Массовое согласование
  const openBatchDialog = (status: string) => {
    if (selectedIds.size === 0) return;
    setBatchDialog({ open: true, status });
  };
  const handleBatchAction = async () => {
    const ids = Array.from(selectedIds);
    const items = ids.map(id => {
      const approval = approvals.find(a => a.id === id);
      return { id, updated_at: approval?.updated_at || '' };
    });
    if (items.some(item => !item.updated_at)) {
      setBatchDialog({ open: false, status: '' });
      setSnackbar({ open: true, message: '❌ Версия одной из карточек не определена. Обновите список', severity: 'error' });
      await fetchApprovals();
      return;
    }
    const status = batchDialog.status;
    setBatchDialog({ open: false, status: '' });
    setSubmitting(prev => {
      const next = { ...prev };
      ids.forEach(id => { next[id] = true; });
      return next;
    });
    try {
      await promoAPI.batchApprove(items, status, '', approvalRole);
      removeApprovalsFromCache(promoId => selectedIds.has(promoId));
      setSelectedIds(new Set());
      setSnackbar({ open: true, message: `✅ ${ids.length} промо обновлено`, severity: 'success' });
      setRefreshFilters(prev => prev + 1);
      if (onDataChanged) onDataChanged();
    } catch (err: unknown) {
      const apiError = asApiError(err);
      setSnackbar({ open: true, message: '❌ Ошибка: ' + (apiError.message || 'не удалось'), severity: 'error' });
      if (apiError.status === 409) {
        setSelectedIds(new Set());
        await fetchApprovals();
      }
    } finally {
      setSubmitting(prev => {
        const next = { ...prev };
        ids.forEach(id => { delete next[id]; });
        return next;
      });
    }
  };

  // Форматирование ROI с цветом (всегда 1 десятичный знак)
  const formatROI = (value: number | null | undefined) => {
    if (value == null) return '';
    const num = Number(value);
    return num.toLocaleString('ru-RU', { minimumFractionDigits: 1, maximumFractionDigits: 1 });
  };
  const getROIColor = (value: number | null | undefined) => {
    if (value == null) return undefined;
    const num = Number(value);
    if (num > 0) return '#16a34a';
    if (num < 0) return '#dc2626';
    return undefined;
  };

  const activeApproval = approvals.find(item => item.id === activeApprovalId) || null;
  const getCurrentStatus = (item: ApprovalRow) => (
    approvalRole === 'agreement2' ? item.agreement2_status : item.agreement1_status
  ) || 'pending';
  const statusMeta = (status: string): { label: string; color: 'success' | 'error' | 'info' | 'default' } => {
    if (status === 'approved') return { label: 'Согласовано', color: 'success' };
    if (status === 'rejected') return { label: 'Отклонено', color: 'error' };
    if (status === 'commented') return { label: 'Комментарий', color: 'info' };
    return { label: 'Ожидает', color: 'default' };
  };

  return (
    <Box sx={{ flex: 1, overflow: 'auto', px: 2, pb: 4 }}>
      <Box sx={{ pb: 1, pt: 2 }}>
      {role === 'admin' && (
        <Paper variant="outlined" sx={{ p: 2, mb: 2, borderRadius: 3 }}>
          <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
            <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>Этап согласования:</Typography>
            <ToggleButtonGroup
              value={adminApprovalRole}
              exclusive
              size="small"
              onChange={(_, nextRole) => {
                if (!nextRole) return;
                setAdminApprovalRole(nextRole);
                setActiveApprovalId(null);
                setSelectedIds(new Set());
                setHasApplied(false);
                setPage(0);
              }}
            >
              <ToggleButton value="agreement1">Согласование 1</ToggleButton>
              <ToggleButton value="agreement2">Согласование 2</ToggleButton>
            </ToggleButtonGroup>
          </Stack>
        </Paper>
      )}
      <Paper variant="outlined" sx={{ p: 2, mb: 2, borderRadius: 3 }}>
        <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1.5 }}>🔍 Фильтры</Typography>
        <Stack direction="row" spacing={1.5} useFlexGap sx={{ flexWrap: 'wrap', alignItems: 'center' }}>
          <TextField select size="small" label="KAM" value={draftKam}
            onChange={(e) => setDraftKam(e.target.value)} sx={{ minWidth: 180 }}>
            <MenuItem value="">Все</MenuItem>
            {kams.map(k => <MenuItem key={k} value={k}>{k}</MenuItem>)}
          </TextField>
          <TextField select size="small" label="Сеть" value={draftNetwork}
            onChange={(e) => setDraftNetwork(e.target.value)} sx={{ minWidth: 180 }}>
            <MenuItem value="">Все</MenuItem>
            {networks.map(n => <MenuItem key={n} value={n}>{n}</MenuItem>)}
          </TextField>
          <TextField select size="small" label="Бренд" value={draftBrand}
            onChange={(e) => setDraftBrand(e.target.value)} sx={{ minWidth: 160 }}>
            <MenuItem value="">Все</MenuItem>
            {brands.map(b => <MenuItem key={b} value={b}>{b}</MenuItem>)}
          </TextField>
          <TextField select size="small" label="Механика" value={draftMechanics}
            onChange={(e) => setDraftMechanics(e.target.value)} sx={{ minWidth: 160 }}>
            <MenuItem value="">Все</MenuItem>
            {mechanicsOptions.map(m => <MenuItem key={m} value={m}>{m}</MenuItem>)}
          </TextField>
          <TextField select size="small" label="Состояние" value={draftStatus}
            onChange={(e) => setDraftStatus(e.target.value)} sx={{ minWidth: 170 }}>
            {APPROVAL_STATUSES.map(s => <MenuItem key={s.value} value={s.value}>{s.label}</MenuItem>)}
          </TextField>
          <TextField label="Год" type="number" size="small" value={draftYear}
            onChange={(e) => setDraftYear(e.target.value)} sx={{ width: 90 }}
            slotProps={{ htmlInput: { min: 2020, max: 2030 } }} />
          <TextField select size="small" label="Месяц" value={draftMonth}
            onChange={(e) => setDraftMonth(e.target.value)} sx={{ minWidth: 120 }}>
            <MenuItem value="">Все</MenuItem>
            {MONTHS.map(m => <MenuItem key={m.value} value={m.value}>{m.label}</MenuItem>)}
          </TextField>
          <Button variant="contained" size="small" onClick={handleApply} sx={{ alignSelf: 'center' }}>
            Применить
          </Button>
          <Button variant="outlined" size="small" onClick={handleReset} sx={{ alignSelf: 'center' }}>
            Сброс
          </Button>
          <FormControlLabel
            control={
              <Checkbox
                size="small"
                checked={draftHasComments}
                onChange={(e) => setDraftHasComments(e.target.checked)}
              />
            }
            label={<Typography variant="body2">Есть комментарии</Typography>}
            sx={{ ml: 1 }}
          />
        </Stack>
      </Paper>
      </Box>

      {/* Переключатель вида и массовые действия */}
      {approvals.length > 0 && (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
          <ToggleButtonGroup
            value={viewMode}
            exclusive
            onChange={(_, v) => {
              if (v) {
                startTransition(() => {
                  setViewMode(v);
                  setSelectedIds(new Set());
                });
              }
            }}
            size="small"
          >
            <ToggleButton value="workspace"><TableIcon sx={{ mr: 0.5, fontSize: 18 }} />Очередь</ToggleButton>
            <ToggleButton value="cards"><CardIcon sx={{ mr: 0.5, fontSize: 18 }} />Карточки</ToggleButton>
          </ToggleButtonGroup>
          <Button
            size="small"
            startIcon={<SettingsIcon />}
            onClick={() => setSettingsOpen(true)}
            sx={{ color: '#475569', fontWeight: 500, ml: 1 }}
          >
            Поля
          </Button>
          <Box sx={{ flex: 1 }} />
          {viewMode === 'workspace' && (
            <>
              <Button
                variant="contained" color="success" size="small"
                disabled={selectedIds.size === 0}
                onClick={() => openBatchDialog('согласовано')}
              >
                Согласовать ({selectedIds.size})
              </Button>
              <Button
                variant="contained" color="error" size="small"
                disabled={selectedIds.size === 0}
                onClick={() => openBatchDialog('отклонено')}
              >
                Отклонить ({selectedIds.size})
              </Button>
            </>
          )}
        </Box>
      )}

      {loading && <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}><CircularProgress /></Box>}
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

      {/* Пагинация (сверху) */}
      {total > 0 && (
        <TablePagination
          component="div"
          count={total}
          page={page}
          onPageChange={(_, newPage) => setPage(newPage)}
          rowsPerPage={pageSize}
          onRowsPerPageChange={(e) => { setPageSize(parseInt(e.target.value, 10)); setPage(0); }}
          rowsPerPageOptions={[25, 50, 100]}
          labelRowsPerPage="Строк:"
          sx={{ '.MuiTablePagination-toolbar': { minHeight: 44, px: 0 } }}
        />
      )}

      {!loading && !error && approvals.length === 0 && (
        <Box sx={{ textAlign: 'center', py: 6 }}>
          <Typography color="text.secondary" variant="h6">
            {appliedKam || appliedNetwork || appliedBrand ? 'Ничего не найдено' : 'Выберите фильтры и нажмите «Применить»'}
          </Typography>
        </Box>
      )}

      {/* Вид: Карточки */}
      {!loading && approvals.length > 0 && viewMode === 'cards' && (() => {
        const MAX_CARDS = 50;
        const cardsToShow = approvals.slice(0, MAX_CARDS);
        const remaining = approvals.length - MAX_CARDS;
        return (
          <>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              Найдено: {total} промо · на странице {approvals.length}{remaining > 0 ? ` • Показаны первые ${MAX_CARDS}` : ''}
            </Typography>
            {remaining > 0 && (
              <Alert severity="info" sx={{ mb: 2, borderRadius: 2 }}>
                Показаны первые {MAX_CARDS} из {approvals.length} промо на странице. Осталось ещё {remaining}.<br />
                Для полного списка и массовых действий переключитесь в режим «Очередь».
              </Alert>
            )}
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', md: '1fr 1fr 1fr', lg: '1fr 1fr 1fr 1fr' }, gap: 2 }}>
              {cardsToShow.map(a => (
                <ApprovalCard key={a.id} item={a}
                  expanded={expandedCards[a.id] || false} submitting={submitting}
                  visibleFields={visibleFields}
                  onToggleExpand={toggleExpand}
                  onOpenConfirm={openConfirm} onCommentOnly={handleCommentOnly} />
              ))}
            </Box>
          </>
        );
      })()}

      {/* Основной вид: очередь согласований + подробности */}
      {!loading && approvals.length > 0 && viewMode === 'workspace' && (
        <Paper
          variant="outlined"
          sx={{
            borderRadius: 3,
            overflow: 'hidden',
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', lg: 'minmax(620px, 1.25fr) minmax(380px, 0.75fr)' },
          }}
        >
          <Box sx={{ minWidth: 0, borderRight: { lg: '1px solid #e2e8f0' }, borderBottom: { xs: '1px solid #e2e8f0', lg: 0 } }}>
            <Box sx={{ px: 2, py: 1.5, display: 'flex', alignItems: 'center', gap: 1, bgcolor: '#f8fafc', borderBottom: '1px solid #e2e8f0' }}>
              <Box>
                <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>Очередь согласования</Typography>
                <Typography variant="caption" color="text.secondary">
                  На странице {approvals.length} из {total} · выбрано {selectedIds.size}
                </Typography>
              </Box>
              <Box sx={{ flex: 1 }} />
              <Chip size="small" label={`Этап ${approvalRole === 'agreement2' ? '2' : '1'}`} color="primary" variant="outlined" />
            </Box>
            <TableContainer sx={{ maxHeight: 720 }}>
              <Table stickyHeader size="small" aria-label="Очередь промо на согласование">
                <TableHead>
                  <TableRow>
                    <TableCell padding="checkbox">
                      <Checkbox
                        size="small"
                        indeterminate={selectedIds.size > 0 && selectedIds.size < approvals.length}
                        checked={approvals.length > 0 && selectedIds.size === approvals.length}
                        onClick={toggleSelectAll}
                        onChange={() => {}}
                        slotProps={{ input: { 'aria-label': 'Выбрать все промо на странице' } }}
                      />
                    </TableCell>
                    <TableCell>Промо</TableCell>
                    <TableCell>Период</TableCell>
                    <TableCell align="right">План / факт</TableCell>
                    <TableCell align="right">Инвестиции</TableCell>
                    <TableCell align="right">ROI</TableCell>
                    <TableCell>Статус</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {approvals.map(item => {
                    const currentStatus = getCurrentStatus(item);
                    const meta = statusMeta(currentStatus);
                    const isActive = activeApprovalId === item.id;
                    return (
                      <TableRow
                        key={item.id}
                        hover
                        selected={isActive}
                        onClick={() => setActiveApprovalId(item.id)}
                        sx={{
                          cursor: 'pointer',
                          '& td:first-of-type': { borderLeft: isActive ? '3px solid #6366f1' : '3px solid transparent' },
                          '&.Mui-selected': { bgcolor: '#eef2ff' },
                          '&.Mui-selected:hover': { bgcolor: '#e0e7ff' },
                        }}
                      >
                        <TableCell padding="checkbox">
                          <Checkbox
                            size="small"
                            checked={selectedIds.has(item.id)}
                            onChange={() => {}}
                            onClick={event => {
                              event.stopPropagation();
                              toggleSelect(item.id);
                            }}
                            slotProps={{ input: { 'aria-label': `Выбрать промо ${item.id}` } }}
                          />
                        </TableCell>
                        <TableCell sx={{ minWidth: 190 }}>
                          <Box
                            component="button"
                            type="button"
                            aria-label={`Открыть промо ${item.id}`}
                            onClick={() => setActiveApprovalId(item.id)}
                            sx={{
                              display: 'block', width: '100%', p: 0, border: 0, bgcolor: 'transparent',
                              color: 'inherit', font: 'inherit', textAlign: 'left', cursor: 'pointer',
                            }}
                          >
                            <Typography variant="body2" sx={{ fontWeight: 700 }}>{item.network_name || 'Сеть не указана'}</Typography>
                            <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }}>{item.brand_as || '—'} · {item.sku || '—'}</Typography>
                            <Typography variant="caption" color="text.secondary">{item.mechanics || 'Механика не указана'}</Typography>
                          </Box>
                        </TableCell>
                        <TableCell sx={{ whiteSpace: 'nowrap' }}>{item.year}.{String(item.month || '').padStart(2, '0')}</TableCell>
                        <TableCell align="right" sx={{ whiteSpace: 'nowrap' }}>
                          <Typography variant="body2" sx={{ fontWeight: 600 }}>
                            {item.plan_promo_units != null ? Number(item.plan_promo_units).toLocaleString('ru-RU') : '—'}
                          </Typography>
                          <Typography variant="caption" color="text.secondary">
                            факт {item.actual_promo_sales_units != null ? Number(item.actual_promo_sales_units).toLocaleString('ru-RU') : '—'}
                          </Typography>
                        </TableCell>
                        <TableCell align="right" sx={{ whiteSpace: 'nowrap' }}>
                          {item.plan_investments_rub != null ? Number(item.plan_investments_rub).toLocaleString('ru-RU') : '—'}
                        </TableCell>
                        <TableCell align="right">
                          <Typography variant="body2" sx={{ fontWeight: 700, color: getROIColor(item.plan_roi) }}>
                            {formatROI(item.plan_roi) || '—'}{item.plan_roi != null ? '%' : ''}
                          </Typography>
                          <Typography variant="caption" color="text.secondary">
                            факт {formatROI(item.actual_roi) || '—'}{item.actual_roi != null ? '%' : ''}
                          </Typography>
                        </TableCell>
                        <TableCell><Chip size="small" label={meta.label} color={meta.color} variant="outlined" /></TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </TableContainer>
          </Box>

          <ApprovalDetailPanel
            item={activeApproval}
            approvalRole={approvalRole}
            visibleFields={visibleFields}
            submitting={Boolean(activeApproval && submitting[activeApproval.id])}
            onOpenConfirm={openConfirm}
            onCommentOnly={handleCommentOnly}
          />
        </Paper>
      )}

      {/* Диалог подтверждения (единичное действие) */}
      <Dialog open={confirmDialog.open} onClose={() => setConfirmDialog({ open: false, id: null, updatedAt: '', status: '', comment: '', warning: false, currentStatus: 'pending' })}>
        <DialogTitle>
          {confirmDialog.status === 'comment' ? 'Сохранить комментарий?' :
           confirmDialog.warning ? '⚠️ Изменение статуса согласования' : 'Подтвердите действие'}
        </DialogTitle>
        <DialogContent>
          {confirmDialog.warning && (
            <Alert severity="warning" sx={{ mb: 1.5 }}>
              Текущий статус: <b>{confirmDialog.currentStatus === 'approved' ? 'Согласовано' : 'Отклонено'}</b>.
              Вы меняете его на <b>{confirmDialog.status === 'согласовано' ? 'Согласовано' : 'Отклонено'}</b>.
            </Alert>
          )}
          <Typography>
            {confirmDialog.status === 'согласовано' && 'Вы уверены, что хотите СОГЛАСОВАТЬ это промо?'}
            {confirmDialog.status === 'отклонено' && 'Вы уверены, что хотите ОТКЛОНИТЬ это промо?'}
            {confirmDialog.status === 'comment' && 'Комментарий будет сохранён, решение не принято.'}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmDialog({ open: false, id: null, updatedAt: '', status: '', comment: '', warning: false, currentStatus: 'pending' })}>Отмена</Button>
          <Button variant="contained"
            color={confirmDialog.status === 'отклонено' ? 'error' : confirmDialog.status === 'comment' ? 'primary' : 'success'}
            onClick={handleConfirmedAction}>
            {confirmDialog.status === 'comment' ? 'Отправить комментарий' : confirmDialog.status === 'согласовано' ? 'Согласовать' : 'Отклонить'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Диалог массового согласования */}
      <Dialog open={batchDialog.open} onClose={() => setBatchDialog({ open: false, status: '' })}>
        <DialogTitle>Массовое действие</DialogTitle>
        <DialogContent>
          <Typography>
            {batchDialog.status === 'согласовано' && `Согласовать ${selectedIds.size} промо?`}
            {batchDialog.status === 'отклонено' && `Отклонить ${selectedIds.size} промо?`}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setBatchDialog({ open: false, status: '' })}>Отмена</Button>
          <Button variant="contained"
            color={batchDialog.status === 'отклонено' ? 'error' : 'success'}
            onClick={handleBatchAction}>
            {batchDialog.status === 'согласовано' ? 'Согласовать все' : 'Отклонить все'}
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar open={snackbar.open} autoHideDuration={3000} onClose={() => setSnackbar(s => ({ ...s, open: false }))}>
        <Alert severity={snackbar.severity} onClose={() => setSnackbar(s => ({ ...s, open: false }))}>{snackbar.message}</Alert>
      </Snackbar>

      {/* Настройка полей подробной панели и карточек */}
      <Drawer anchor="right" open={settingsOpen} onClose={() => setSettingsOpen(false)}>
        <Box sx={{ width: 350, p: 3 }}>
          <Typography variant="h6" sx={{ mb: 0.5 }}>Поля отображения</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Настройка применяется к подробной панели и карточкам. Доступны только поля, которые загружает API.
          </Typography>

          <TextField
            fullWidth size="small" placeholder="Поиск поля..."
            value={searchField}
            onChange={e => setSearchField(e.target.value)}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon sx={{ fontSize: 18, color: 'gray' }} />
                  </InputAdornment>
                ),
              },
            }}
            sx={{ mb: 2 }}
          />

          {FIELD_GROUPS.map((group, idx) => {
            const filteredFields = group.fields.filter(f =>
              f.label.toLowerCase().includes(searchField.toLowerCase())
            );
            if (filteredFields.length === 0) return null;

            return (
              <Accordion key={idx} defaultExpanded disableGutters elevation={0}
                sx={{ borderBottom: '1px solid #eee' }}>
                <AccordionSummary expandIcon={<ExpandMore />}>
                  <Typography sx={{ fontWeight: 600 }}>{group.group}</Typography>
                </AccordionSummary>
                <AccordionDetails sx={{ pt: 0, display: 'flex', flexDirection: 'column', gap: 1 }}>
                  {filteredFields.map(field => (
                    <FormControlLabel
                      key={field.id}
                      control={
                        <Checkbox
                          size="small"
                          checked={visibleFields.includes(field.id)}
                          onChange={() => toggleField(field.id)}
                        />
                      }
                      label={<Typography variant="body2">{field.label}</Typography>}
                    />
                  ))}
                </AccordionDetails>
              </Accordion>
            );
          })}

          <Button variant="outlined" fullWidth onClick={resetVisibleFields} sx={{ mt: 2 }}>
            Восстановить стандартные поля
          </Button>
        </Box>
      </Drawer>
    </Box>
  );
}
