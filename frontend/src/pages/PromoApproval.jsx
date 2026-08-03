import { useState, useEffect, useCallback, useRef } from 'react';
import {
  Box, Typography, CircularProgress, Alert, Snackbar,
  TextField, MenuItem, Dialog,
  DialogTitle, DialogContent, DialogActions,
  Paper, Stack, Button,
} from '@mui/material';
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
  // Черновики фильтров (меняются сразу)
  const [draftKam, setDraftKam] = useState('');
  const [draftNetwork, setDraftNetwork] = useState('');
  const [draftBrand, setDraftBrand] = useState('');
  const [draftMechanics, setDraftMechanics] = useState('');
  const [draftStatus, setDraftStatus] = useState('pending');
  const [draftYear, setDraftYear] = useState('');
  const [draftMonth, setDraftMonth] = useState('');

  // Флаг: была ли нажата кнопка «Применить»
  const [hasApplied, setHasApplied] = useState(false);

  // Применённые фильтры (по кнопке «Применить»)
  const [appliedKam, setAppliedKam] = useState('');
  const [appliedNetwork, setAppliedNetwork] = useState('');
  const [appliedBrand, setAppliedBrand] = useState('');
  const [appliedMechanics, setAppliedMechanics] = useState('');
  const [appliedStatus, setAppliedStatus] = useState('pending');
  const [appliedYear, setAppliedYear] = useState('');
  const [appliedMonth, setAppliedMonth] = useState('');

  // Справочники (зависят от appliedStatus)
  const [kams, setKams] = useState([]);
  const [networks, setNetworks] = useState([]);
  const [brands, setBrands] = useState([]);
  const [mechanicsOptions, setMechanicsOptions] = useState([]);

  const [approvals, setApprovals] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [expandedCards, setExpandedCards] = useState({});
  const [submitting, setSubmitting] = useState({});

  const commentRefs = useRef({});
  const [confirmDialog, setConfirmDialog] = useState({ open: false, id: null, status: '' });
  const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'success' });
  const fetchIdRef = useRef(0);

  // Загрузка справочников при смене фильтров (включая KAM из того же запроса)
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
  }, [appliedStatus, appliedKam, appliedNetwork, appliedBrand, appliedMechanics, appliedYear, appliedMonth]);

  // Загрузка данных при изменении применённых фильтров (только после «Применить»)
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
      });
      if (currentFetchId !== fetchIdRef.current) return;

      let filtered = data.data || [];
      if (appliedNetwork) filtered = filtered.filter(a => a.network_name === appliedNetwork);
      if (appliedBrand) filtered = filtered.filter(a => a.sku && a.sku.includes(appliedBrand));
      if (appliedMechanics) filtered = filtered.filter(a => a.mechanics === appliedMechanics);
      if (appliedYear) filtered = filtered.filter(a => a.year === parseInt(appliedYear));
      if (appliedMonth) filtered = filtered.filter(a => a.month === parseInt(appliedMonth));

      // Очищаем старые DOM-рефы перед установкой новых данных
      commentRefs.current = {};
      setApprovals(filtered);
    } catch (err) {
      if (currentFetchId !== fetchIdRef.current) return;
      setError(err.message || 'Ошибка загрузки');
    } finally {
      if (currentFetchId === fetchIdRef.current) setLoading(false);
    }
  }, [hasApplied, appliedKam, appliedStatus, appliedNetwork, appliedBrand, appliedMechanics, appliedYear, appliedMonth]);

  useEffect(() => {
    fetchApprovals();
  }, [fetchApprovals]);

  // Кнопка «Применить» — только если выбран хоть один фильтр
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
    setDraftStatus('pending'); setDraftYear(''); setDraftMonth('');
    setAppliedKam(''); setAppliedNetwork(''); setAppliedBrand(''); setAppliedMechanics('');
    setAppliedStatus('pending'); setAppliedYear(''); setAppliedMonth('');
    setApprovals([]);
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
      setSnackbar({ open: true, message: status === 'согласовано' ? '✅ Согласовано' : status === 'отклонено' ? '❌ Отклонено' : '💬 Комментарий сохранён', severity: 'success' });
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

      {loading && <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}><CircularProgress /></Box>}
      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>{error}</Alert>}

      {!loading && !error && approvals.length === 0 && (
        <Box sx={{ textAlign: 'center', py: 6 }}>
          <Typography color="text.secondary" variant="h6">
            {appliedKam || appliedNetwork || appliedBrand ? 'Ничего не найдено' : 'Выберите фильтры и нажмите «Применить»'}
          </Typography>
        </Box>
      )}

      {!loading && approvals.length > 0 && (
        <>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>Найдено: {approvals.length} промо</Typography>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', md: '1fr 1fr 1fr', lg: '1fr 1fr 1fr 1fr' }, gap: 2 }}>
            {approvals.map(a => (
              <ApprovalCard key={a.id} item={a}
                expanded={expandedCards[a.id] || false} submitting={submitting}
                onCommentRef={handleCommentRef} onToggleExpand={toggleExpand}
                onOpenConfirm={openConfirm} onCommentOnly={handleCommentOnly} />
            ))}
          </Box>
        </>
      )}

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

      <Snackbar open={snackbar.open} autoHideDuration={3000} onClose={() => setSnackbar(s => ({ ...s, open: false }))}>
        <Alert severity={snackbar.severity} onClose={() => setSnackbar(s => ({ ...s, open: false }))}>{snackbar.message}</Alert>
      </Snackbar>
    </Box>
  );
}