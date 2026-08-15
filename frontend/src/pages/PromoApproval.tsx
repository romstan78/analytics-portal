import { useState, useEffect, useCallback, useRef, startTransition } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import {
  Box, Typography, CircularProgress, Alert, Snackbar,
  TextField, MenuItem, Dialog,
  DialogTitle, DialogContent, DialogActions,
  Paper, Stack, Button, ToggleButtonGroup, ToggleButton,
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
import { promoAPI } from '../api/promo';
import { FIELD_GROUPS, DEFAULT_VISIBLE_FIELDS } from '../utils/cardFields';

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

export default function PromoApproval({ role, onDataChanged }) {
  const queryClient = useQueryClient();
  const [adminApprovalRole, setAdminApprovalRole] = useState('agreement1');
  const approvalRole = role === 'admin' ? adminApprovalRole : role;
  // Вид: cards | table
  const [viewMode, setViewMode] = useState('cards');

  // Настройка карточек (Drawer)
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [searchField, setSearchField] = useState('');
  const [visibleFields, setVisibleFields] = useState(() => {
    try {
      const saved = localStorage.getItem('promo_card_fields_v2');
      return saved ? JSON.parse(saved) : DEFAULT_VISIBLE_FIELDS;
    } catch { return DEFAULT_VISIBLE_FIELDS; }
  });

  const toggleField = (id) => {
    setVisibleFields(prev => {
      const next = prev.includes(id) ? prev.filter(f => f !== id) : [...prev, id];
      localStorage.setItem('promo_card_fields_v2', JSON.stringify(next));
      return next;
    });
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
  const [kams, setKams] = useState([]);
  const [networks, setNetworks] = useState([]);
  const [brands, setBrands] = useState([]);
  const [mechanicsOptions, setMechanicsOptions] = useState([]);

  const [approvals, setApprovals] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [expandedCards, setExpandedCards] = useState({});
  const [submitting, setSubmitting] = useState({});

  // Массовое согласование
  const [selectedIds, setSelectedIds] = useState(new Set());
  const [batchDialog, setBatchDialog] = useState({ open: false, status: '' });

  const [confirmDialog, setConfirmDialog] = useState({ open: false, id: null, status: '', comment: '', warning: false, currentStatus: 'pending' });
  const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'success' });
  const [refreshFilters, setRefreshFilters] = useState(0);
  const fetchIdRef = useRef(0);

  // Пагинация
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(50);
  const [total, setTotal] = useState(0);

  // Загрузка справочников
  useEffect(() => {
    promoAPI.getApprovalFilters({
      approval_role: approvalRole,
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
  }, [approvalRole, appliedStatus, appliedKam, appliedNetwork, appliedBrand, appliedMechanics, appliedYear, appliedMonth, refreshFilters]);

  // Загрузка данных
  const fetchApprovals = useCallback(async () => {
    if (!hasApplied) return;
    const currentFetchId = ++fetchIdRef.current;
    setLoading(true);
    setError(null);

    try {
      const data = await promoAPI.getApprovals({
        approval_role: approvalRole,
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
      });
      if (currentFetchId !== fetchIdRef.current) return;

      setApprovals(data.data || []);
      setTotal(data.total || 0);
      setSelectedIds(new Set()); // сбрасываем выделение при новой загрузке
    } catch (err) {
      if (currentFetchId !== fetchIdRef.current) return;
      setError(err.message || 'Ошибка загрузки');
    } finally {
      if (currentFetchId === fetchIdRef.current) setLoading(false);
    }
  }, [approvalRole, hasApplied, appliedKam, appliedStatus, appliedNetwork, appliedBrand, appliedMechanics, appliedYear, appliedMonth, appliedHasComments, page, pageSize]);

  useEffect(() => { fetchApprovals(); }, [fetchApprovals]);

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
    setApprovals([]);
    setSelectedIds(new Set());
    setHasApplied(false);
    setPage(0);
    setTotal(0);
  };

  const openConfirm = (id, status, comment) => {
    const item = approvals.find(a => a.id === id);
    const statusField = approvalRole === 'agreement2' ? 'agreement2_status' : 'agreement1_status';
    const currentStatus = item?.[statusField] || 'pending';

    // Предупреждение: если статус уже approved/rejected и меняем его на согласовано/отклонено
    const needsWarning = (currentStatus === 'approved' || currentStatus === 'rejected') &&
                         (status === 'согласовано' || status === 'отклонено');

    setConfirmDialog({
      open: true, id, status, comment: comment || '',
      warning: needsWarning,
      currentStatus,
    });
  };

  const handleConfirmedAction = async () => {
    const { id, status, comment } = confirmDialog;
    setConfirmDialog({ open: false, id: null, status: '', comment: '', warning: false, currentStatus: 'pending' });
    if (!id) return;
    setSubmitting(prev => ({ ...prev, [id]: true }));
    try {
      await promoAPI.approve(id, status, comment, approvalRole);
      // Инвалидируем кэш комментариев для этой карточки
      queryClient.invalidateQueries({ queryKey: ['comments', id] });
      // Инвалидируем approvals
      queryClient.invalidateQueries({ queryKey: ['approvals'] });
      setApprovals(prev => prev.filter(a => a.id !== id));
      setSnackbar({ open: true, message: status === 'comment' ? '✅ Комментарий сохранён' : '✅ Выполнено', severity: 'success' });
      setRefreshFilters(prev => prev + 1);
      if (onDataChanged) onDataChanged();
    } catch (err) {
      setSnackbar({ open: true, message: '❌ Ошибка: ' + (err.message || 'не удалось'), severity: 'error' });
    } finally {
      setSubmitting(prev => ({ ...prev, [id]: false }));
    }
  };

  const handleCommentOnly = (id, comment) => {
    if (!comment || !comment.trim()) return;
    setConfirmDialog({ open: true, id, status: 'comment', comment });
    // Сразу выполняем действие, т.к. для "comment" не нужен доп. диалог подтверждения
    setConfirmDialog({ open: false, id: null, status: '', comment: '' });
    handleQuickAction(id, 'comment', comment);
  };

  // Быстрое действие без диалога подтверждения (comment-only).
  // После сохранения инвалидируем кэш комментариев — карточка остаётся.
  const handleQuickAction = async (id, status, comment) => {
    setSubmitting(prev => ({ ...prev, [id]: true }));
    try {
      await promoAPI.approve(id, status, comment, approvalRole);
      // Инвалидируем кэш комментариев — useQuery в ApprovalCard перезапросит
      queryClient.invalidateQueries({ queryKey: ['comments', id] });
      // Инвалидируем approvals
      queryClient.invalidateQueries({ queryKey: ['approvals'] });
      setSnackbar({ open: true, message: '✅ Комментарий сохранён', severity: 'success' });
      setRefreshFilters(prev => prev + 1);
      if (onDataChanged) onDataChanged();
    } catch (err) {
      setSnackbar({ open: true, message: '❌ Ошибка: ' + (err.message || 'не удалось'), severity: 'error' });
    } finally {
      setSubmitting(prev => ({ ...prev, [id]: false }));
    }
  };
  const toggleExpand = (id) => setExpandedCards(prev => ({ ...prev, [id]: !prev[id] }));

  // Чекбоксы
  const toggleSelect = (id) => {
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
  const openBatchDialog = (status) => {
    if (selectedIds.size === 0) return;
    setBatchDialog({ open: true, status });
  };
  const handleBatchAction = async () => {
    const ids = Array.from(selectedIds);
    const status = batchDialog.status;
    setBatchDialog({ open: false, status: '' });
    setSubmitting(prev => {
      const next = { ...prev };
      ids.forEach(id => { next[id] = true; });
      return next;
    });
    try {
      await promoAPI.batchApprove(ids, status, '', approvalRole);
      setApprovals(prev => prev.filter(a => !selectedIds.has(a.id)));
      setSelectedIds(new Set());
      setSnackbar({ open: true, message: `✅ ${ids.length} промо обновлено`, severity: 'success' });
      setRefreshFilters(prev => prev + 1);
      if (onDataChanged) onDataChanged();
    } catch (err) {
      setSnackbar({ open: true, message: '❌ Ошибка: ' + (err.message || 'не удалось'), severity: 'error' });
    } finally {
      setSubmitting(prev => {
        const next = { ...prev };
        ids.forEach(id => { delete next[id]; });
        return next;
      });
    }
  };

  // Форматирование ROI с цветом (всегда 1 десятичный знак)
  const formatROI = (value) => {
    if (value == null) return '';
    const num = Number(value);
    return num.toLocaleString('ru-RU', { minimumFractionDigits: 1, maximumFractionDigits: 1 });
  };
  const getROIColor = (value) => {
    if (value == null) return undefined;
    const num = Number(value);
    if (num > 0) return '#16a34a';
    if (num < 0) return '#dc2626';
    return undefined;
  };

  return (
    <Box sx={{ flex: 1, overflow: 'auto', px: 2, pb: 4 }}>
      {/* Sticky: фильтры + переключатель вида — закреплены при скролле */}
      <Box sx={{ position: 'sticky', top: 0, zIndex: 10, bgcolor: '#f5f7fa', pb: 2, pt: 2 }}>
      {role === 'admin' && (
        <Paper variant="outlined" sx={{ p: 2, mb: 2, borderRadius: 3 }}>
          <Stack direction="row" spacing={2} alignItems="center">
            <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>Этап согласования:</Typography>
            <ToggleButtonGroup
              value={adminApprovalRole}
              exclusive
              size="small"
              onChange={(_, nextRole) => {
                if (!nextRole) return;
                setAdminApprovalRole(nextRole);
                setApprovals([]);
                setSelectedIds(new Set());
                setHasApplied(false);
                setTotal(0);
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
        <Stack direction="row" spacing={1.5} flexWrap="wrap" useFlexGap alignItems="center">
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
            <ToggleButton value="cards"><CardIcon sx={{ mr: 0.5, fontSize: 18 }} />Карточки</ToggleButton>
            <ToggleButton value="table"><TableIcon sx={{ mr: 0.5, fontSize: 18 }} />Таблица</ToggleButton>
          </ToggleButtonGroup>
          {/* Кнопка настроек карточек — только в режиме карточек */}
          {viewMode === 'cards' && (
            <Button
              size="small"
              startIcon={<SettingsIcon />}
              onClick={() => setSettingsOpen(true)}
              sx={{ color: '#475569', fontWeight: 500, ml: 1 }}
            >
              Поля
            </Button>
          )}
          <Box sx={{ flex: 1 }} />
          {/* Кнопки массовых действий — только в режиме таблицы */}
          {viewMode === 'table' && (
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
      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>{error}</Alert>}

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
              Найдено: {approvals.length} промо{remaining > 0 ? ` • Показаны первые ${MAX_CARDS}` : ''}
            </Typography>
            {remaining > 0 && (
              <Alert severity="info" sx={{ mb: 2, borderRadius: 2 }}>
                Показаны первые {MAX_CARDS} из {approvals.length} промо. Осталось ещё {remaining}.<br />
                Для массовых действий переключитесь в табличный вид.
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

      {/* Вид: Таблица */}
      {!loading && approvals.length > 0 && viewMode === 'table' && (
        <TableContainer component={Paper} variant="outlined" sx={{ borderRadius: 3 }}>
          <Table size="small">
            <TableHead>
              <TableRow sx={{ bgcolor: '#f1f5f9' }}>
                <TableCell padding="checkbox">
                  <Checkbox
                    size="small"
                    indeterminate={selectedIds.size > 0 && selectedIds.size < approvals.length}
                    checked={selectedIds.size === approvals.length}
                    onChange={toggleSelectAll}
                  />
                </TableCell>
                <TableCell>Период</TableCell>
                <TableCell>Сеть</TableCell>
                <TableCell>Бренд</TableCell>
                <TableCell>SKU</TableCell>
                <TableCell>Механика</TableCell>
                <TableCell align="right">План (уп)</TableCell>
                <TableCell align="right">Факт (уп)</TableCell>
                <TableCell align="right">Инвестиции</TableCell>
                <TableCell align="right">ROI</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {approvals.map(a => (
                <TableRow key={a.id} hover selected={selectedIds.has(a.id)}>
                  <TableCell padding="checkbox">
                    <Checkbox
                      size="small"
                      checked={selectedIds.has(a.id)}
                      onChange={() => toggleSelect(a.id)}
                    />
                  </TableCell>
                  <TableCell>{a.year}.{String(a.month).padStart(2, '0')}</TableCell>
                  <TableCell>{a.network_name}</TableCell>
                  <TableCell>{a.brand_as}</TableCell>
                  <TableCell>{a.sku}</TableCell>
                  <TableCell>{a.mechanics}</TableCell>
                  <TableCell align="right">
                    {a.plan_promo_units != null ? Number(a.plan_promo_units).toLocaleString('ru-RU') : ''}
                  </TableCell>
                  <TableCell align="right">
                    {a.actual_promo_sales_units != null ? Number(a.actual_promo_sales_units).toLocaleString('ru-RU') : ''}
                  </TableCell>
                  <TableCell align="right">
                    {a.plan_investments_rub != null ? Number(a.plan_investments_rub).toLocaleString('ru-RU') : ''}
                  </TableCell>
                  <TableCell align="right">
                    <Typography fontWeight={600} color={getROIColor(a.plan_roi)}>
                      {formatROI(a.plan_roi)}
                    </Typography>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}

      {/* Диалог подтверждения (единичное действие) */}
      <Dialog open={confirmDialog.open} onClose={() => setConfirmDialog({ open: false, id: null, status: '', warning: false, currentStatus: 'pending' })}>
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
          <Button onClick={() => setConfirmDialog({ open: false, id: null, status: '', warning: false, currentStatus: 'pending' })}>Отмена</Button>
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

      {/* Drawer настройки полей карточки */}
      <Drawer anchor="right" open={settingsOpen} onClose={() => setSettingsOpen(false)}>
        <Box sx={{ width: 350, p: 3 }}>
          <Typography variant="h6" mb={2}>Настройка карточки</Typography>

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
                  <Typography fontWeight={600}>{group.group}</Typography>
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
        </Box>
      </Drawer>
    </Box>
  );
}
