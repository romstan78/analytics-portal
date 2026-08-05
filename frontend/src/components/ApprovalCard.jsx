import { memo, useMemo } from 'react';
import {
  Box, Typography, Card, CardContent, CardActions,
  Button, Chip, Collapse, Grid, TextField,
  LinearProgress, CircularProgress,
} from '@mui/material';
import {
  ExpandMore as ExpandMoreIcon,
  CheckCircle as ApproveIcon,
  Cancel as RejectIcon,
  Comment as CommentIcon,
} from '@mui/icons-material';
import { ALL_FIELDS_FLAT } from '../utils/cardFields';

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

// Эти поля всегда показываются на карточке, не выбираются в настройках
const SIDEBAR_FIELDS = new Set(['network_name', 'sku', 'mechanics', 'brand_as', 'kam']);

const ApprovalCard = memo(function ApprovalCard({
  item, expanded, submitting,
  onCommentChange, comments, onToggleExpand, onOpenConfirm, onCommentOnly,
  visibleFields,
}) {
  const id = item.id;
  const isSubmitting = submitting[id] || false;

  // Динамические поля из настроек, исключая sidebar
  const visibleData = useMemo(() => {
    if (!visibleFields || visibleFields.length === 0) return [];
    return ALL_FIELDS_FLAT.filter(f => visibleFields.includes(f.id) && !SIDEBAR_FIELDS.has(f.id));
  }, [visibleFields]);

  // Цвет левой рамки по ROI
  const leftBorderColor = item.plan_roi != null
    ? (Number(item.plan_roi) >= 0 ? '#16a34a' : '#dc2626')
    : '#94a3b8';

  return (
    <Box sx={{ position: 'relative' }}>
      {isSubmitting && <LinearProgress sx={{ position: 'absolute', top: 0, left: 0, right: 0, zIndex: 2, borderTopLeftRadius: 12, borderTopRightRadius: 12 }} />}
      <Card elevation={2} sx={{
        borderRadius: 3, transition: 'all 0.2s', '&:hover': { boxShadow: 6 },
        height: '100%', display: 'flex', flexDirection: 'column',
        opacity: isSubmitting ? 0.7 : 1,
        borderLeft: `4px solid ${leftBorderColor}`,
      }}>
        {isSubmitting && (
          <Box sx={{ position: 'absolute', top: 0, left: 0, right: 0, bottom: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1, bgcolor: 'rgba(255,255,255,0.4)', borderRadius: 3 }}>
            <CircularProgress size={32} />
          </Box>
        )}
        <CardContent sx={{ flex: 1, pb: 1, pt: 2 }}>
          {/* Заголовок и чипсы (фиксированные) */}
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

          {/* Динамические поля из настроек */}
          {visibleData.length > 0 && (
            <Grid container spacing={1.5} sx={{ mb: 1 }}>
              {visibleData.map(fieldConfig => (
                <Grid item xs={6} key={fieldConfig.id}>
                  <Typography variant="caption" color="text.secondary">{fieldConfig.label}</Typography>
                  <Typography variant="body2" sx={{
                    fontWeight: 600,
                    color: fieldConfig.isRoi ? roiColor(item[fieldConfig.id]) : 'inherit',
                  }}>
                    {fieldConfig.isRoi || fieldConfig.isPercent
                      ? (item[fieldConfig.id] != null ? `${Number(item[fieldConfig.id]).toFixed(1)}%` : '—')
                      : (item[fieldConfig.id] != null ? fmtNum(item[fieldConfig.id], fieldConfig.isMoney ? 2 : 0) : '—')}
                  </Typography>
                </Grid>
              ))}
            </Grid>
          )}

          {/* Блок исторических данных — показываем, если поля включены */}
          {(visibleFields?.includes('historical_count') || visibleFields?.includes('avg_historical_roi') || visibleFields?.includes('avg_historical_uplift')) && (
            <Box sx={{ bgcolor: '#f1f5f9', borderRadius: 1.5, p: 1, mb: 1, display: 'flex', gap: 2 }}>
              {visibleFields.includes('historical_count') && (
                <Typography variant="caption" color="text.secondary">История: {item.historical_count} промо</Typography>
              )}
              {visibleFields.includes('avg_historical_roi') && (
                <Typography variant="caption" color="text.secondary">
                  Средний ROI: {item.avg_historical_roi != null ? `${Number(item.avg_historical_roi).toFixed(1)}%` : '—'}
                </Typography>
              )}
            </Box>
          )}

          {/* Согласования — всегда показываем, если есть */}
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

          {/* Условия — всегда показываем, если есть */}
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

          {/* История переписки (поле comments из БД) */}
          {item.comments && (
            <Box sx={{ mb: 1 }}>
              <Button size="small" onClick={() => onToggleExpand(id)}
                endIcon={<ExpandMoreIcon sx={{ transform: expanded ? 'rotate(180deg)' : 'rotate(0)', transition: 'transform 0.2s' }} />}
                sx={{ color: '#64748b', textTransform: 'none', p: 0 }}>💬 Комментарии</Button>
              <Collapse in={expanded}>
                <Typography variant="body2" sx={{ mt: 0.5, p: 1, bgcolor: '#f8fafc', borderRadius: 1, fontSize: '0.75rem', color: '#475569', whiteSpace: 'pre-wrap', maxHeight: 120, overflow: 'auto' }}>
                  {item.comments}
                </Typography>
              </Collapse>
            </Box>
          )}

          <TextField size="small" fullWidth multiline minRows={1} maxRows={3}
            placeholder="Комментарий (необязательно)"
            value={comments[id] || ''}
            onChange={(e) => onCommentChange(id, e.target.value)}
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

export default ApprovalCard;