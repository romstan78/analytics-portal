import { useState, useEffect, useCallback } from 'react';
import {
  Box, Typography, Card, CardContent, CardActions,
  Button, Chip, CircularProgress, Alert,
  FormControl, InputLabel, Select, MenuItem,
  Collapse, IconButton, Grid, TextField,
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

// Месяцы для фильтра
const MONTHS = [
  { label: 'Январь', value: 1 }, { label: 'Февраль', value: 2 }, { label: 'Март', value: 3 },
  { label: 'Апрель', value: 4 }, { label: 'Май', value: 5 }, { label: 'Июнь', value: 6 },
  { label: 'Июль', value: 7 }, { label: 'Август', value: 8 }, { label: 'Сентябрь', value: 9 },
  { label: 'Октябрь', value: 10 }, { label: 'Ноябрь', value: 11 }, { label: 'Декабрь', value: 12 },
];

export default function PromoApproval({ role }) {
  // Фильтры
  const [kams, setKams] = useState([]);
  const [selectedKam, setSelectedKam] = useState('');
  const [networks, setNetworks] = useState([]);
  const [selectedNetwork, setSelectedNetwork] = useState('');
  const [brands, setBrands] = useState([]);
  const [selectedBrand, setSelectedBrand] = useState('');
  const [selectedYear, setSelectedYear] = useState('');
  const [selectedMonth, setSelectedMonth] = useState('');

  // Данные
  const [approvals, setApprovals] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [expandedCards, setExpandedCards] = useState({});
  const [submitting, setSubmitting] = useState({});
  const [comments, setComments] = useState({}); // id → текст комментария

  // Загрузка KAM'ов
  useEffect(() => {
    promoAPI.getApprovalKAMs()
      .then(data => {
        setKams(data.data || []);
        if (data.data && data.data.length === 1) setSelectedKam(data.data[0]);
      })
      .catch(() => {});
  }, []);

  // При смене KAM → загружаем сети
  useEffect(() => {
    if (selectedKam) {
      promoAPI.getApprovalNetworks(selectedKam)
        .then(data => setNetworks(data.data || []))
        .catch(() => {});
      setSelectedNetwork('');
      setSelectedBrand('');
    }
  }, [selectedKam]);

  // При смене сети → загружаем бренды
  useEffect(() => {
    if (selectedKam) {
      promoAPI.getApprovalBrands(selectedKam, selectedNetwork)
        .then(data => setBrands(data.data || []))
        .catch(() => {});
      setSelectedBrand('');
    }
  }, [selectedKam, selectedNetwork]);

  // Загрузка промо
  const fetchApprovals = useCallback(async () => {
    setLoading(true); setError(null);
    try {
      const data = await promoAPI.getApprovals(selectedKam);
      let filtered = data.data || [];

      // Клиентская фильтрация (бэкенд тоже фильтрует, но для верности)
      if (selectedNetwork) filtered = filtered.filter(a => a.network_name === selectedNetwork);
      if (selectedBrand) filtered = filtered.filter(a => a.sku && a.sku.includes(selectedBrand)); // бренд_as не приходит с бэка — фильтруем по SKU как fallback
      if (selectedYear) filtered = filtered.filter(a => a.year === parseInt(selectedYear));
      if (selectedMonth) filtered = filtered.filter(a => a.month === parseInt(selectedMonth));

      setApprovals(filtered);
    } catch (err) {
      setError(err.message || 'Ошибка загрузки');
    } finally {
      setLoading(false);
    }
  }, [selectedKam, selectedNetwork, selectedBrand, selectedYear, selectedMonth]);

  useEffect(() => {
    if (selectedKam) fetchApprovals();
    else setApprovals([]);
  }, [fetchApprovals, selectedKam]);

  // Действия
  const handleApprove = async (id, status) => {
    const comment = comments[id] || '';
    setSubmitting(prev => ({ ...prev, [id]: true }));
    try {
      await promoAPI.approve(id, status, comment);
      setApprovals(prev => prev.filter(a => a.id !== id));
      setComments(prev => { const next = { ...prev }; delete next[id]; return next; });
    } catch (err) {
      // остаётся
    } finally {
      setSubmitting(prev => ({ ...prev, [id]: false }));
    }
  };

  const handleCommentOnly = async (id) => {
    const comment = comments[id] || '';
    if (!comment.trim()) return;
    setSubmitting(prev => ({ ...prev, [id]: true }));
    try {
      await promoAPI.approve(id, 'comment', comment);
      setApprovals(prev => prev.filter(a => a.id !== id));
      setComments(prev => { const next = { ...prev }; delete next[id]; return next; });
    } catch (err) {
      // остаётся
    } finally {
      setSubmitting(prev => ({ ...prev, [id]: false }));
    }
  };

  const toggleExpand = (id) => setExpandedCards(prev => ({ ...prev, [id]: !prev[id] }));
  const updateComment = (id, text) => setComments(prev => ({ ...prev, [id]: text }));

  return (
    <Box sx={{ flex: 1, overflow: 'auto', px: 2 }}>
      {/* Фильтры */}
      <Box sx={{ display: 'flex', gap: 1.5, mb: 3, flexWrap: 'wrap', alignItems: 'center' }}>
        <FormControl size="small" sx={{ minWidth: 200 }}>
          <InputLabel>KAM</InputLabel>
          <Select value={selectedKam} label="KAM" onChange={(e) => setSelectedKam(e.target.value)}>
            {kams.map(k => <MenuItem key={k} value={k}>{k}</MenuItem>)}
          </Select>
        </FormControl>

        <FormControl size="small" sx={{ minWidth: 200 }}>
          <InputLabel>Сеть</InputLabel>
          <Select value={selectedNetwork} label="Сеть" onChange={(e) => setSelectedNetwork(e.target.value)}>
            <MenuItem value="">Все</MenuItem>
            {networks.map(n => <MenuItem key={n} value={n}>{n}</MenuItem>)}
          </Select>
        </FormControl>

        <FormControl size="small" sx={{ minWidth: 160 }}>
          <InputLabel>Бренд</InputLabel>
          <Select value={selectedBrand} label="Бренд" onChange={(e) => setSelectedBrand(e.target.value)}>
            <MenuItem value="">Все</MenuItem>
            {brands.map(b => <MenuItem key={b} value={b}>{b}</MenuItem>)}
          </Select>
        </FormControl>

        <TextField label="Год" type="number" size="small" value={selectedYear}
          onChange={(e) => setSelectedYear(e.target.value)}
          sx={{ width: 90 }} slotProps={{ htmlInput: { min: 2020, max: 2030 } }} />

        <FormControl size="small" sx={{ minWidth: 130 }}>
          <InputLabel>Месяц</InputLabel>
          <Select value={selectedMonth} label="Месяц" onChange={(e) => setSelectedMonth(e.target.value)}>
            <MenuItem value="">Все</MenuItem>
            {MONTHS.map(m => <MenuItem key={m.value} value={m.value}>{m.label}</MenuItem>)}
          </Select>
        </FormControl>

        <Button variant="outlined" size="small" onClick={() => {
          setSelectedNetwork(''); setSelectedBrand(''); setSelectedYear(''); setSelectedMonth('');
        }}>Сброс</Button>
      </Box>

      {loading && <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}><CircularProgress /></Box>}
      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>{error}</Alert>}

      {!loading && !error && selectedKam && approvals.length === 0 && (
        <Box sx={{ textAlign: 'center', py: 6 }}>
          <Typography color="text.secondary" variant="h6">Нет промо на согласовании</Typography>
        </Box>
      )}

      {!loading && approvals.length > 0 && (
        <Grid container spacing={2}>
          {approvals.map(a => (
            <Grid item xs={12} sm={6} md={4} key={a.id}>
              <Card elevation={2} sx={{ borderRadius: 3, maxWidth: 400, transition: 'all 0.2s', '&:hover': { boxShadow: 6 } }}>
                <CardContent sx={{ pb: 1 }}>
                  <Typography variant="subtitle1" sx={{ fontWeight: 700, mb: 0.5 }}>
                    {a.network_name || '—'}
                  </Typography>

                  <Box sx={{ display: 'flex', gap: 0.5, mb: 1, flexWrap: 'wrap' }}>
                    <Chip label={a.sku || '—'} size="small" variant="outlined" />
                    <Chip label={a.mechanics || '—'} size="small" color="primary" variant="outlined" />
                  </Box>

                  {/* Дата промо */}
                  {a.year && a.month && (
                    <Typography variant="caption" color="text.secondary" sx={{ mb: 1, display: 'block' }}>
                      Период: {MONTHS.find(m => m.value === a.month)?.label || a.month} {a.year}
                    </Typography>
                  )}

                  <Grid container spacing={1} sx={{ mb: 1 }}>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Baseline</Typography>
                      <Typography variant="body2" sx={{ fontWeight: 600 }}>{fmtNum(a.baseline_units)} уп</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">План</Typography>
                      <Typography variant="body2" sx={{ fontWeight: 600 }}>{fmtNum(a.plan_promo_units)} уп</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Факт продаж</Typography>
                      <Typography variant="body2" sx={{ fontWeight: 600 }}>{fmtNum(a.actual_promo_sales_units)} уп</Typography>
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">Инвестиции</Typography>
                      <Typography variant="body2" sx={{ fontWeight: 600 }}>{fmtNum(a.plan_investments_rub, 2)} ₽</Typography>
                    </Grid>
                  </Grid>

                  <Box sx={{ display: 'flex', gap: 2, mb: 1 }}>
                    <Box>
                      <Typography variant="caption" color="text.secondary">ROI план</Typography>
                      <Typography variant="body2" sx={{ fontWeight: 700, color: roiColor(a.plan_roi) }}>
                        {a.plan_roi != null ? `${Number(a.plan_roi).toFixed(1)}%` : '—'}
                      </Typography>
                    </Box>
                    <Box>
                      <Typography variant="caption" color="text.secondary">ROI факт</Typography>
                      <Typography variant="body2" sx={{ fontWeight: 700, color: roiColor(a.actual_roi) }}>
                        {a.actual_roi != null ? `${Number(a.actual_roi).toFixed(1)}%` : '—'}
                      </Typography>
                    </Box>
                  </Box>

                  <Box sx={{ bgcolor: '#f1f5f9', borderRadius: 1.5, p: 1, mb: 1, display: 'flex', gap: 2 }}>
                    <Typography variant="caption" color="text.secondary">История: {a.historical_count} промо</Typography>
                    <Typography variant="caption" color="text.secondary">
                      Средний ROI: {a.avg_historical_roi != null ? `${Number(a.avg_historical_roi).toFixed(1)}%` : '—'}
                    </Typography>
                  </Box>

                  {a.conditions && (
                    <Box sx={{ mb: 1 }}>
                      <Button size="small" onClick={() => toggleExpand(a.id)}
                        endIcon={<ExpandMoreIcon sx={{ transform: expandedCards[a.id] ? 'rotate(180deg)' : 'rotate(0)', transition: 'transform 0.2s' }} />}
                        sx={{ color: '#64748b', textTransform: 'none', p: 0 }}>
                        Условия
                      </Button>
                      <Collapse in={expandedCards[a.id]}>
                        <Typography variant="body2" sx={{ mt: 0.5, p: 1, bgcolor: '#f8fafc', borderRadius: 1, fontSize: '0.8rem', color: '#475569' }}>
                          {a.conditions}
                        </Typography>
                      </Collapse>
                    </Box>
                  )}

                  {/* Комментарий согласующего */}
                  <TextField
                    size="small"
                    fullWidth
                    multiline
                    minRows={1}
                    maxRows={2}
                    placeholder="Комментарий (необязательно)"
                    value={comments[a.id] || ''}
                    onChange={(e) => updateComment(a.id, e.target.value)}
                    sx={{ mb: 1 }}
                  />
                </CardContent>

                <CardActions sx={{ justifyContent: 'space-between', px: 2, pb: 2, gap: 0.5 }}>
                  <Button size="small" variant="outlined"
                    startIcon={<CommentIcon />}
                    onClick={() => handleCommentOnly(a.id)}
                    disabled={submitting[a.id] || !(comments[a.id] || '').trim()}
                    sx={{ borderRadius: 2, flex: 1, fontSize: '0.75rem' }}>
                    Комм.
                  </Button>
                  <Button size="small" variant="contained" color="success"
                    startIcon={<ApproveIcon />}
                    onClick={() => handleApprove(a.id, 'согласовано')}
                    disabled={submitting[a.id]}
                    sx={{ borderRadius: 2, flex: 1, fontSize: '0.75rem' }}>
                    Согласовано
                  </Button>
                  <Button size="small" variant="contained" color="error"
                    startIcon={<RejectIcon />}
                    onClick={() => handleApprove(a.id, 'отклонено')}
                    disabled={submitting[a.id]}
                    sx={{ borderRadius: 2, flex: 1, fontSize: '0.75rem' }}>
                    Отклонено
                  </Button>
                </CardActions>
              </Card>
            </Grid>
          ))}
        </Grid>
      )}
    </Box>
  );
}