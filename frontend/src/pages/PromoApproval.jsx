import { useState, useEffect, useCallback, useRef } from 'react';
import {
  Box, Typography, CircularProgress, Alert, Snackbar,
  TextField, MenuItem, Dialog,
  DialogTitle, DialogContent, DialogActions,
  Paper, Stack, Button, ToggleButtonGroup, ToggleButton,
  Checkbox, Table, TableBody, TableCell, TableContainer,
  TableHead, TableRow,
} from '@mui/material';
import {
  ViewModule as CardIcon,
  TableRows as TableIcon,
} from '@mui/icons-material';
import ApprovalCard from '../components/ApprovalCard';
import { promoAPI } from '../api/promo';

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
  // Вид: cards | table
  const [viewMode, setViewMode] = useState('cards');

  // Черновики фильтров (меняются сразу)
  const [draftKam, setDraftKam] = useState('');
  const [draftNetwork, setDraftNetwork] = useState('');
  const [draftBrand, setDraftBrand] = useState('');
  const [draftMechanics, setDraftMechanics] = useState('');
  const [draftStatus, setDraftStatus] = useState('pending');
  const [draftYear, setDraftYear] = useState(String(new Date().getFullYear()));
  const [draftMonth, setDraftMonth] = useState('');

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

  const commentRefs = useRef({});
  const [confirmDialog, setConfirmDialog] = useState({ open: false, id: null, status: '' });
  const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'success' });
  const [refreshFilters, setRefreshFilters] = useState(0);
  const fetchIdRef = useRef(0);

  // Загрузка справочников
  useEffect(() => {
    promoAPI.getApprovalFilters({
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
  }, [appliedStatus, appliedKam, appliedNetwork, appliedBrand, appliedMechanics, appliedYear, appliedMonth, refreshFilters]);

  // Загрузка данных
  const fetchApprovals = useCallback(async () => {
    if (!hasApplied || (!appliedKam && !appliedNetwork && !appliedBrand && !appliedMechanics && !appliedYear && !appliedMonth)) return;
    const currentFetchId = ++fetchIdRef.current;
    setLoading(true);
    setError(null);

    try {
      const data = await promoAPI.getApprovals({
        kam: appliedKam || undefined,
        approval_status: appliedStatus,
        year: appliedYear,
        month: appliedMonth,
        network_name: appliedNetwork || undefined,
        brand: appliedBrand || undefined,
        mechanics: appliedMechanics || undefined,
      });
      if (currentFetchId !== fetchIdRef.current) return;

      commentRefs.current = {};
      setApprovals(data.data || []);
      setSelectedIds(new Set()); // сбрасываем выделение при новой загрузке
    } catch (err) {
      if (currentFetchId !== fetchIdRef.current) return;
      setError(err.message || 'Ошибка загрузки');
    } finally {
      if (currentFetchId === fetchIdRef.current) setLoading(false);
    }
  }, [hasApplied, appliedKam, appliedStatus, appliedNetwork, appliedBrand, appliedMechanics, appliedYear, appliedMonth]);

  useEffect(() => { fetchApprovals(); }, [fetchApprovals]);

  const handleApply = () => {
    const hasAnyFilter = draftKam || draftNetwork || draftBrand || draftMechanics || draftYear || draftMonth;
    if (!hasAnyFilter) return;
    setHasApplied(true);
    setAppliedKam(draftKam);
    setAppliedNetwork(draftNetwork);
    setAppliedBrand(draftBrand);
    setAppliedMechanics(draftMechanics);
    setAppliedStatus(draftStatus);
    setAppliedYear(draftYear);
    setAppliedMonth(draftMonth);
  };

  const handleReset = () => {
    setDraftKam(''); setDraftNetwork(''); setDraftBrand(''); setDraftMechanics('');
    setDraftStatus('pending'); setDraftYear(String(new Date().getFullYear())); setDraftMonth('');
    setAppliedKam(''); setAppliedNetwork(''); setAppliedBrand(''); setAppliedMechanics('');
    setAppliedStatus('pending'); setAppliedYear(String(new Date().getFullYear())); setAppliedMonth('');
    setApprovals([]);
    setSelectedIds(new Set());
    setHasApplied(false);
  };

  const handleCommentRef = useCallback((id, el) => { commentRefs.current[id] = el; }, []);
  const openConfirm = (id, status) => setConfirmDialog({ open: true, id, status });

  const handleConfirmedAction = async () => {
    const { id, status } = confirmDialog;
    setConfirmDialog({ open: false, id: null, status: '' });
    if (!id) return;
    const inputEl = commentRefs.current[id];
    const comment = inputEl ? inputEl.value : '';
    setSubmitting(prev => ({ ...prev, [id]: true }));
    try {
      await promoAPI.approve(id, status, comment);
      setApprovals(prev => prev.filter(a => a.id !== id));
      delete commentRefs.current[id];
      setSnackbar({ open: true, message: '✅ Выполнено', severity: 'success' });
      setRefreshFilters(prev => prev + 1);
      if (onDataChanged) onDataChanged();
    } catch (err) {
      setSnackbar({ open: true, message: '❌ Ошибка: ' + (err.message || 'не удалось'), severity: 'error' });
    } finally {
      setSubmitting(prev => ({ ...prev, [id]: false }));
    }
  };

  const handleCommentOnly = (id) => {
    const inputEl = commentRefs.current[id];
    if (!inputEl || !inputEl.value.trim()) return;
    openConfirm(id, 'comment');
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
      await promoAPI.batchApprove(ids, status, '');
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
      <Paper variant="outlined" sx={{ p: 2, mb: 3, borderRadius: 3 }}>
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
        </Stack>
      </Paper>

      {/* Переключатель вида и массовые действия */}
      {approvals.length > 0 && (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
          <ToggleButtonGroup
            value={viewMode}
            exclusive
            onChange={(_, v) => v && setViewMode(v)}
            size="small"
          >
            <ToggleButton value="cards"><CardIcon sx={{ mr: 0.5, fontSize: 18 }} />Карточки</ToggleButton>
            <ToggleButton value="table"><TableIcon sx={{ mr: 0.5, fontSize: 18 }} />Таблица</ToggleButton>
          </ToggleButtonGroup>
          <Box sx={{ flex: 1 }} />
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
        </Box>
      )}

      {loading && <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}><CircularProgress /></Box>}
      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>{error}</Alert>}

      {!loading && !error && approvals.length === 0 && (
        <Box sx={{ textAlign: 'center', py: 6 }}>
          <Typography color="text.secondary" variant="h6">
            {appliedKam || appliedNetwork || appliedBrand ? 'Ничего не найдено' : 'Выберите фильтры и нажмите «Применить»'}
          </Typography>
        </Box>
      )}

      {/* Вид: Карточки */}
      {!loading && approvals.length > 0 && viewMode === 'cards' && (
        <>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>Найдено: {approvals.length} промо</Typography>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', md: '1fr 1fr 1fr', lg: '1fr 1fr 1fr 1fr' }, gap: 2 }}>
            {approvals.map(a => (
              <ApprovalCard key={a.id} item={a}
                expanded={expandedCards[a.id] || false} submitting={submitting}
                selected={selectedIds.has(a.id)} onToggleSelect={() => toggleSelect(a.id)}
                onCommentRef={handleCommentRef} onToggleExpand={toggleExpand}
                onOpenConfirm={openConfirm} onCommentOnly={handleCommentOnly} />
            ))}
          </Box>
        </>
      )}

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
      <Dialog open={confirmDialog.open} onClose={() => setConfirmDialog({ open: false, id: null, status: '' })}>
        <DialogTitle>{confirmDialog.status === 'comment' ? 'Сохранить комментарий?' : 'Подтвердите действие'}</DialogTitle>
        <DialogContent>
          <Typography>
            {confirmDialog.status === 'согласовано' && 'Вы уверены, что хотите СОГЛАСОВАТЬ это промо?'}
            {confirmDialog.status === 'отклонено' && 'Вы уверены, что хотите ОТКЛОНИТЬ это промо?'}
            {confirmDialog.status === 'comment' && 'Комментарий будет сохранён, решение не принято.'}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmDialog({ open: false, id: null, status: '' })}>Отмена</Button>
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
    </Box>
  );
}