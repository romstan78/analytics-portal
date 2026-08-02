import { useState, useEffect, useCallback, useRef, memo } from 'react';
import {
  Box, Typography, Card, CardContent, CardActions,
  Button, Chip, CircularProgress, Alert, Snackbar,
  TextField, MenuItem, Collapse, Grid, Dialog,
  DialogTitle, DialogContent, DialogActions,
  Paper, Stack,
} from '@mui/material';
import {
  ExpandMore as ExpandMoreIcon,
  CheckCircle as ApproveIcon,
  Cancel as RejectIcon,
  Comment as CommentIcon,
} from '@mui/icons-material';
import { promoAPI } from '../api/promo';

const fmtNum = (v, decimals = 0) => {
  if (v == null) return '—';
  return Number(v).toLocaleString('ru-RU', { minimumFractionDigits: decimals, maximumFractionDigits: decimals });
};

const roiColor = (roi) => {
  if (roi == null) return '#94a3b8';
  return roi >= 0 ? '#16a34a' : '#dc2626';
};

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

const ApprovalCard = memo(function ApprovalCard({
  item, expanded, submitting, onCommentRef,
  onToggleExpand, onOpenConfirm, onCommentOnly,
}) {
  const id = item.id;
  const isSubmitting = submitting[id] || false;

  return (
    <Box>
      <Card elevation={2} sx={{ borderRadius: 3, transition: 'all 0.2s', '&:hover': { boxShadow: 6 }, height: '100%', display: 'flex', flexDirection: 'column' }}>
        <CardContent sx={{ flex: 1, pb: 1 }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 700, mb: 0.5 }}>
            {item.network_name || '—'}
          </Typography>
          <Box sx={{ display: 'flex', gap: 0.5, mb: 1, flexWrap: 'wrap' }}>
            <Chip label={item.sku || '—'} size="small" variant="outlined" />
            <Chip label={item.mechanics || '—'} size="small" color="primary" variant="outlined" />
          </Box>
          {item.year && item.month && (
            <Typography variant="caption" color="text.secondary" sx={{ mb: 1, display: 'block' }}>
              Период: {MONTHS.find(m => m.value === item.month)?.label || item.month} {item.year}
            </Typography>
          )}
          <Grid container spacing={1} sx={{ mb: 1 }}>
            <Grid item xs={6}>
              <Typography variant="caption" color="text.secondary">Baseline</Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>{fmtNum(item.baseline_units)} уп</Typography>
            </Grid>
            <Grid item xs={6}>
              <Typography variant="caption" color="text.secondary">План</Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>{fmtNum(item.plan_promo_units)} уп</Typography>
            </Grid>
            <Grid item xs={6}>
              <Typography variant="caption" color="text.secondary">Факт продаж</Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>{fmtNum(item.actual_promo_sales_units)} уп</Typography>
            </Grid>
            <Grid item xs={6}>
              <Typography variant="caption" color="text.secondary">Инвестиции</Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>{fmtNum(item.plan_investments_rub, 2)} ₽</Typography>
            </Grid>
          </Grid>
          <Box sx={{ display: 'flex', gap: 2, mb: 1 }}>
            <Box>
              <Typography variant="caption" color="text.secondary">ROI план</Typography>
              <Typography variant="body2" sx={{ fontWeight: 700, color: roiColor(item.plan_roi) }}>
                {item.plan_roi != null ? `${Number(item.plan_roi).toFixed(1)}%` : '—'}
              </Typography>
            </Box>
            <Box>
              <Typography variant="caption" color="text.secondary">ROI факт</Typography>
              <Typography variant="body2" sx={{ fontWeight: 700, color: roiColor(item.actual_roi) }}>
                {item.actual_roi != null ? `${Number(item.actual_roi).toFixed(1)}%` : '—'}
              </Typography>
            </Box>
          </Box>
          <Box sx={{ bgcolor: '#f1f5f9', borderRadius: 1.5, p: 1, mb: 1, display: 'flex', gap: 2 }}>
            <Typography variant="caption" color="text.secondary">История: {item.historical_count} промо</Typography>
            <Typography variant="caption" color="text.secondary">
              Средний ROI: {item.avg_historical_roi != null ? `${Number(item.avg_historical_roi).toFixed(1)}%` : '—'}
            </Typography>
          </Box>
          {/* Комментарии согласующих */}
          {(item.agreement1 || item.agreement2) && (
            <Box sx={{ mb: 1, display: 'flex', flexDirection: 'column', gap: 0.5 }}>
              {item.agreement1 && (
                <Typography variant="caption" sx={{ 
                  p: 0.75, borderRadius: 1, fontSize: '0.72rem',
                  bgcolor: String(item.agreement1).startsWith('согласовано') ? '#f0fdf4' : 
                           String(item.agreement1).startsWith('отклонено') ? '#fef2f2' : '#eef2ff',
                  color: String(item.agreement1).startsWith('согласовано') ? '#16a34a' : 
                         String(item.agreement1).startsWith('отклонено') ? '#dc2626' : '#6366f1',
                }}>
                  <b>Согл. 1:</b> {item.agreement1}
                </Typography>
              )}
              {item.agreement2 && (
                <Typography variant="caption" sx={{ 
                  p: 0.75, borderRadius: 1, fontSize: '0.72rem',
                  bgcolor: String(item.agreement2).startsWith('согласовано') ? '#f0fdf4' : 
                           String(item.agreement2).startsWith('отклонено') ? '#fef2f2' : '#eef2ff',
                  color: String(item.agreement2).startsWith('согласовано') ? '#16a34a' : 
                         String(item.agreement2).startsWith('отклонено') ? '#dc2626' : '#6366f1',
                }}>
                  <b>Согл. 2:</b> {item.agreement2}
                </Typography>
              )}
            </Box>
          )}

          {item.conditions && (
            <Box sx={{ mb: 1 }}>
              <Button size="small" onClick={() => onToggleExpand(id)}
                endIcon={<ExpandMoreIcon sx={{ transform: expanded ? 'rotate(180deg)' : 'rotate(0)', transition: 'transform 0.2s' }} />}
                sx={{ color: '#64748b', textTransform: 'none', p: 0 }}>Условия</Button>
              <Collapse in={expanded}>
                <Typography variant="body2" sx={{ mt: 0.5, p: 1, bgcolor: '#f8fafc', borderRadius: 1, fontSize: '0.8rem', color: '#475569' }}>
                  {item.conditions}
                </Typography>
              </Collapse>
            </Box>
          )}
          <TextField size="small" fullWidth multiline minRows={1} maxRows={3}
            placeholder="Комментарий (необязательно)"
            inputRef={(el) => { if (el && onCommentRef) onCommentRef(id, el); }}
            sx={{ mb: 1 }} />
        </CardContent>
        <CardActions sx={{ justifyContent: 'space-between', px: 2, pb: 2, gap: 0.5, mt: 'auto' }}>
          <Button size="small" variant="outlined" startIcon={<CommentIcon />}
            onClick={() => onCommentOnly(id)} disabled={isSubmitting}
            sx={{ borderRadius: 2, flex: 1, fontSize: '0.75rem' }}>Комментарий</Button>
          <Button size="small" variant="contained" color="success" startIcon={<ApproveIcon />}
            onClick={() => onOpenConfirm(id, 'согласовано')} disabled={isSubmitting}
            sx={{ borderRadius: 2, flex: 1, fontSize: '0.75rem' }}>Согласовано</Button>
          <Button size="small" variant="contained" color="error" startIcon={<RejectIcon />}
            onClick={() => onOpenConfirm(id, 'отклонено')} disabled={isSubmitting}
            sx={{ borderRadius: 2, flex: 1, fontSize: '0.75rem' }}>Отклонено</Button>
        </CardActions>
      </Card>
    </Box>
  );
});

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