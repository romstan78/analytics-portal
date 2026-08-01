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

// ═══════════════════════════════════════════════════════════════════════════
// Утилиты
// ═══════════════════════════════════════════════════════════════════════════

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

// ═══════════════════════════════════════════════════════════════════════════
// Карточка промо (вынесена в memo для производительности)
// ═══════════════════════════════════════════════════════════════════════════

const ApprovalCard = memo(function ApprovalCard({
  item, expanded, submitting, commentRef,
  onToggleExpand, onOpenConfirm, onCommentOnly,
}) {
  const id = item.id;
  const isSubmitting = submitting[id] || false;

  return (
    <Grid item xs={12} sm={6} md={4} lg={3}>
      <Card elevation={2} sx={{ borderRadius: 3, transition: 'all 0.2s', '&:hover': { boxShadow: 6 }, height: '100%', display: 'flex', flexDirection: 'column' }}>
        <CardContent sx={{ flex: 1, pb: 1 }}>
          {/* Заголовок: сеть */}
          <Typography variant="subtitle1" sx={{ fontWeight: 700, mb: 0.5 }}>
            {item.network_name || '—'}
          </Typography>

          {/* Чипы: SKU + Механика */}
          <Box sx={{ display: 'flex', gap: 0.5, mb: 1, flexWrap: 'wrap' }}>
            <Chip label={item.sku || '—'} size="small" variant="outlined" />
            <Chip label={item.mechanics || '—'} size="small" color="primary" variant="outlined" />
          </Box>

          {/* Период */}
          {item.year && item.month && (
            <Typography variant="caption" color="text.secondary" sx={{ mb: 1, display: 'block' }}>
              Период: {MONTHS.find(m => m.value === item.month)?.label || item.month} {item.year}
            </Typography>
          )}

          {/* Показатели */}
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

          {/* ROI */}
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

          {/* История */}
          <Box sx={{ bgcolor: '#f1f5f9', borderRadius: 1.5, p: 1, mb: 1, display: 'flex', gap: 2 }}>
            <Typography variant="caption" color="text.secondary">История: {item.historical_count} промо</Typography>
            <Typography variant="caption" color="text.secondary">
              Средний ROI: {item.avg_historical_roi != null ? `${Number(item.avg_historical_roi).toFixed(1)}%` : '—'}
            </Typography>
          </Box>

          {/* Условия (сворачиваемые) */}
          {item.conditions && (
            <Box sx={{ mb: 1 }}>
              <Button size="small" onClick={() => onToggleExpand(id)}
                endIcon={<ExpandMoreIcon sx={{ transform: expanded ? 'rotate(180deg)' : 'rotate(0)', transition: 'transform 0.2s' }} />}
                sx={{ color: '#64748b', textTransform: 'none', p: 0 }}>
                Условия
              </Button>
              <Collapse in={expanded}>
                <Typography variant="body2" sx={{ mt: 0.5, p: 1, bgcolor: '#f8fafc', borderRadius: 1, fontSize: '0.8rem', color: '#475569' }}>
                  {item.conditions}
                </Typography>
              </Collapse>
            </Box>
          )}

          {/* Комментарий — используем defaultValue + ref, чтобы не триггерить ререндер родителя */}
          <TextField
            size="small"
            fullWidth
            multiline
            minRows={1}
            maxRows={3}
            placeholder="Комментарий (необязательно)"
            defaultValue=""
            inputRef={(el) => { commentRef.current[id] = el; }}
            sx={{ mb: 1 }}
          />
        </CardContent>

        <CardActions sx={{ justifyContent: 'space-between', px: 2, pb: 2, gap: 0.5, mt: 'auto' }}>
          <Button size="small" variant="outlined"
            startIcon={<CommentIcon />}
            onClick={() => onCommentOnly(id)}
            disabled={isSubmitting}
            sx={{ borderRadius: 2, flex: 1, fontSize: '0.75rem' }}>
            Комментарий
          </Button>
          <Button size="small" variant="contained" color="success"
            startIcon={<ApproveIcon />}
            onClick={() => onOpenConfirm(id, 'согласовано')}
            disabled={isSubmitting}
            sx={{ borderRadius: 2, flex: 1, fontSize: '0.75rem' }}>
            Согласовано
          </Button>
          <Button size="small" variant="contained" color="error"
            startIcon={<RejectIcon />}
            onClick={() => onOpenConfirm(id, 'отклонено')}
            disabled={isSubmitting}
            sx={{ borderRadius: 2, flex: 1, fontSize: '0.75rem' }}>
            Отклонено
          </Button>
        </CardActions>
      </Card>
    </Grid>
  );
});

// ═══════════════════════════════════════════════════════════════════════════
// Основной компонент
// ═══════════════════════════════════════════════════════════════════════════

export default function PromoApproval({ role }) {
  // Фильтры
  const [kams, setKams] = useState([]);
  const [networks, setNetworks] = useState([]);
  const [brands, setBrands] = useState([]);
  const [mechanicsOptions, setMechanicsOptions] = useState([]);
  const [statusOptions] = useState(['Новый', 'В работе', 'Завершено', 'Отменено']);

  const [selectedKam, setSelectedKam] = useState('');
  const [selectedNetwork, setSelectedNetwork] = useState('');
  const [selectedBrand, setSelectedBrand] = useState('');
  const [selectedMechanics, setSelectedMechanics] = useState('');
  const [selectedStatus, setSelectedStatus] = useState('');
  const [selectedYear, setSelectedYear] = useState('');
  const [selectedMonth, setSelectedMonth] = useState('');

  // Данные
  const [approvals, setApprovals] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [expandedCards, setExpandedCards] = useState({});
  const [submitting, setSubmitting] = useState({});

  // Рефы для комментариев (избегаем ререндера родителя при вводе)
  const commentRefs = useRef({});

  // Диалог подтверждения
  const [confirmDialog, setConfirmDialog] = useState({ open: false, id: null, status: '' });
  // Снекбар
  const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'success' });

  // Защита от гонок запросов
  const fetchIdRef = useRef(0);

  // ═══════ Загрузка справочников (один раз) ═══════
  useEffect(() => {
    Promise.all([
      promoAPI.getApprovalKAMs().catch(err => { console.error('Ошибка загрузки KAM:', err); return { data: [] }; }),
      promoAPI.getFilters().catch(err => { console.error('Ошибка загрузки фильтров:', err); return { network_name: [], brand: [], mechanics: [] }; }),
    ]).then(([kamData, filterData]) => {
      setKams(kamData.data || []);
      setNetworks(filterData.network_name || []);
      setBrands(filterData.brand || []);
      setMechanicsOptions(filterData.mechanics || []);
    });
  }, []);

  // ═══════ Загрузка промо при изменении фильтров ═══════
  const fetchApprovals = useCallback(async () => {
    const currentFetchId = ++fetchIdRef.current;
    setLoading(true);
    setError(null);

    // Не загружаем ничего, пока не выбран KAM
    if (!selectedKam) {
      setApprovals([]);
      setLoading(false);
      return;
    }

    try {
      const data = await promoAPI.getApprovals(selectedKam);
      if (currentFetchId !== fetchIdRef.current) return;

      let filtered = data.data || [];

      if (selectedNetwork) filtered = filtered.filter(a => a.network_name === selectedNetwork);
      if (selectedBrand) filtered = filtered.filter(a => a.sku && a.sku.includes(selectedBrand));
      if (selectedMechanics) filtered = filtered.filter(a => a.mechanics === selectedMechanics);
      if (selectedStatus) filtered = filtered.filter(a => a.status === selectedStatus);
      if (selectedYear) filtered = filtered.filter(a => a.year === parseInt(selectedYear));
      if (selectedMonth) filtered = filtered.filter(a => a.month === parseInt(selectedMonth));

      setApprovals(filtered);
    } catch (err) {
      if (currentFetchId !== fetchIdRef.current) return;
      setError(err.message || 'Ошибка загрузки');
    } finally {
      if (currentFetchId === fetchIdRef.current) setLoading(false);
    }
  }, [selectedKam, selectedNetwork, selectedBrand, selectedMechanics, selectedStatus, selectedYear, selectedMonth]);

  useEffect(() => {
    fetchApprovals();
  }, [fetchApprovals]);

  // Очистка рефов при смене данных
  useEffect(() => {
    commentRefs.current = {};
  }, [approvals]);

  // ═══════ Действия ═══════
  const openConfirm = (id, status) => setConfirmDialog({ open: true, id, status });

  const handleConfirmedAction = async () => {
    const { id, status } = confirmDialog;
    setConfirmDialog({ open: false, id: null, status: '' });
    if (!id) return;

    // Читаем комментарий из DOM-рефа (без state)
    const inputEl = commentRefs.current[id];
    const comment = inputEl ? inputEl.value : '';

    setSubmitting(prev => ({ ...prev, [id]: true }));
    try {
      await promoAPI.approve(id, status, comment);
      setApprovals(prev => prev.filter(a => a.id !== id));
      delete commentRefs.current[id];
      const label = status === 'согласовано' ? '✅ Согласовано'
        : status === 'отклонено' ? '❌ Отклонено'
        : '💬 Комментарий сохранён';
      setSnackbar({ open: true, message: label, severity: 'success' });
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

  const handleResetFilters = () => {
    setSelectedNetwork('');
    setSelectedBrand('');
    setSelectedMechanics('');
    setSelectedStatus('');
    setSelectedYear('');
    setSelectedMonth('');
  };

  // ═══════ Рендер ═══════
  return (
    <Box sx={{ flex: 1, overflow: 'auto', px: 2, pb: 4 }}>
      {/* Панель фильтров — в стиле FilterPanel (Paper + Grid) */}
      <Paper variant="outlined" sx={{ p: 2, mb: 3, borderRadius: 3 }}>
        <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1.5 }}>🔍 Фильтры</Typography>
        <Stack direction="row" spacing={1.5} flexWrap="wrap" useFlexGap>
          <TextField select size="small" label="KAM" value={selectedKam}
            onChange={(e) => setSelectedKam(e.target.value)}
            sx={{ minWidth: 180 }}>
            <MenuItem value="">Все</MenuItem>
            {kams.map(k => <MenuItem key={k} value={k}>{k}</MenuItem>)}
          </TextField>

          <TextField select size="small" label="Сеть" value={selectedNetwork}
            onChange={(e) => setSelectedNetwork(e.target.value)}
            sx={{ minWidth: 180 }}>
            <MenuItem value="">Все</MenuItem>
            {networks.map(n => <MenuItem key={n} value={n}>{n}</MenuItem>)}
          </TextField>

          <TextField select size="small" label="Бренд" value={selectedBrand}
            onChange={(e) => setSelectedBrand(e.target.value)}
            sx={{ minWidth: 160 }}>
            <MenuItem value="">Все</MenuItem>
            {brands.map(b => <MenuItem key={b} value={b}>{b}</MenuItem>)}
          </TextField>

          <TextField select size="small" label="Механика" value={selectedMechanics}
            onChange={(e) => setSelectedMechanics(e.target.value)}
            sx={{ minWidth: 160 }}>
            <MenuItem value="">Все</MenuItem>
            {mechanicsOptions.map(m => <MenuItem key={m} value={m}>{m}</MenuItem>)}
          </TextField>

          <TextField select size="small" label="Статус" value={selectedStatus}
            onChange={(e) => setSelectedStatus(e.target.value)}
            sx={{ minWidth: 140 }}>
            <MenuItem value="">Все</MenuItem>
            {statusOptions.map(s => <MenuItem key={s} value={s}>{s}</MenuItem>)}
          </TextField>

          <TextField label="Год" type="number" size="small" value={selectedYear}
            onChange={(e) => setSelectedYear(e.target.value)}
            sx={{ width: 90 }}
            slotProps={{ htmlInput: { min: 2020, max: 2030 } }} />

          <TextField select size="small" label="Месяц" value={selectedMonth}
            onChange={(e) => setSelectedMonth(e.target.value)}
            sx={{ minWidth: 120 }}>
            <MenuItem value="">Все</MenuItem>
            {MONTHS.map(m => <MenuItem key={m.value} value={m.value}>{m.label}</MenuItem>)}
          </TextField>

          <Button variant="outlined" size="small" onClick={handleResetFilters}
            sx={{ alignSelf: 'center' }}>Сброс</Button>
        </Stack>
      </Paper>

      {/* Состояния загрузки / ошибки / пусто */}
      {loading && <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}><CircularProgress /></Box>}
      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>{error}</Alert>}

      {!loading && !error && approvals.length === 0 && (
        <Box sx={{ textAlign: 'center', py: 6 }}>
          <Typography color="text.secondary" variant="h6">
            {selectedKam || selectedNetwork || selectedBrand ? 'Ничего не найдено' : 'Выберите фильтры для отображения промо'}
          </Typography>
          {approvals.length === 0 && !selectedKam && !selectedNetwork && !selectedBrand && (
            <Typography color="text.secondary" variant="body2" sx={{ mt: 1 }}>
              Нажмите «Сброс» и выберите KAM для начала работы
            </Typography>
          )}
        </Box>
      )}

      {/* Карточки */}
      {!loading && approvals.length > 0 && (
        <>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Найдено: {approvals.length} промо
          </Typography>
          <Grid container spacing={2}>
            {approvals.map(a => (
              <ApprovalCard
                key={a.id}
                item={a}
                expanded={expandedCards[a.id] || false}
                submitting={submitting}
                commentRef={commentRefs}
                onToggleExpand={toggleExpand}
                onOpenConfirm={openConfirm}
                onCommentOnly={handleCommentOnly}
              />
            ))}
          </Grid>
        </>
      )}

      {/* Диалог подтверждения */}
      <Dialog open={confirmDialog.open} onClose={() => setConfirmDialog({ open: false, id: null, status: '' })}>
        <DialogTitle>
          {confirmDialog.status === 'comment' ? 'Сохранить комментарий?' : 'Подтвердите действие'}
        </DialogTitle>
        <DialogContent>
          <Typography>
            {confirmDialog.status === 'согласовано' && 'Вы уверены, что хотите СОГЛАСОВАТЬ это промо?'}
            {confirmDialog.status === 'отклонено' && 'Вы уверены, что хотите ОТКЛОНИТЬ это промо?'}
            {confirmDialog.status === 'comment' && 'Комментарий будет сохранён, решение не принято.'}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmDialog({ open: false, id: null, status: '' })}>Отмена</Button>
          <Button
            variant="contained"
            color={confirmDialog.status === 'отклонено' ? 'error' : confirmDialog.status === 'comment' ? 'primary' : 'success'}
            onClick={handleConfirmedAction}
          >
            {confirmDialog.status === 'comment' ? 'Отправить комментарий' : confirmDialog.status === 'согласовано' ? 'Согласовать' : 'Отклонить'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Снекбар */}
      <Snackbar open={snackbar.open} autoHideDuration={3000}
        onClose={() => setSnackbar(s => ({ ...s, open: false }))}>
        <Alert severity={snackbar.severity} onClose={() => setSnackbar(s => ({ ...s, open: false }))}>
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>
  );
}