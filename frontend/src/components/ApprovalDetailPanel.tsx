import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Divider,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import {
  CheckCircle as ApproveIcon,
  Cancel as RejectIcon,
  Comment as CommentIcon,
} from '@mui/icons-material';
import { FIELD_GROUPS, type CardField } from '../utils/cardFields';
import { promoAPI } from '../api/promo';
import type { ApprovalRow, CommentRow } from '../types/promo';

const MONTHS = [
  'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
];

const ROLE_COLORS: Record<string, { bg: string; text: string; border: string }> = {
  admin: { bg: '#fef2f2', text: '#b91c1c', border: '#ef4444' },
  agreement1: { bg: '#f0fdf4', text: '#15803d', border: '#22c55e' },
  agreement2: { bg: '#eff6ff', text: '#1d4ed8', border: '#3b82f6' },
  'согласование1': { bg: '#f0fdf4', text: '#15803d', border: '#22c55e' },
  'согласование2': { bg: '#eff6ff', text: '#1d4ed8', border: '#3b82f6' },
  'КАМ': { bg: '#f5f3ff', text: '#6d28d9', border: '#8b5cf6' },
};

function formatValue(item: ApprovalRow, field: CardField): string {
  const value = item[field.id as keyof ApprovalRow];
  if (value == null || value === '') return '—';
  if (field.isRoi || field.isPercent) return `${Number(value).toLocaleString('ru-RU', { minimumFractionDigits: 1, maximumFractionDigits: 1 })}%`;
  if (field.isMoney) return Number(value).toLocaleString('ru-RU', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  if (typeof value === 'number') return value.toLocaleString('ru-RU');
  return String(value);
}

function statusLabel(status: string | null): string {
  if (status === 'approved') return 'Согласовано';
  if (status === 'rejected') return 'Отклонено';
  if (status === 'commented') return 'Есть комментарий';
  return 'На согласовании';
}

function approvalStatusLabel(status: string | null, legacyValue: string | null): string {
  if (status === 'approved') return 'Согласовано';
  if (status === 'rejected') return 'Отклонено';
  if (status === 'commented') return 'Комментарий без решения';
  if (status === 'pending') return 'На согласовании';
  return legacyValue || 'Статус не указан';
}

function approvalCommentText(status: string | null, explicitComment: string | null, legacyValue: string | null): string | null {
  const explicit = explicitComment?.trim();
  if (explicit) return explicit;

  const legacy = legacyValue?.trim();
  if (legacy) {
    if (status === 'commented') return legacy;
    const decisionComment = legacy.match(/^(?:согласовано|отклонено)\s*[:;,—-]\s*(.+)$/i);
    if (decisionComment?.[1]) return decisionComment[1].trim();
  }

  return status === 'commented' ? 'Текст комментария отсутствует в данных.' : null;
}

interface ApprovalDetailPanelProps {
  item: ApprovalRow | null;
  approvalRole: string;
  visibleFields: string[];
  submitting: boolean;
  onOpenConfirm: (id: number, status: string, comment: string) => void;
  onCommentOnly: (id: number, comment: string) => Promise<boolean>;
}

export default function ApprovalDetailPanel({
  item,
  approvalRole,
  visibleFields,
  submitting,
  onOpenConfirm,
  onCommentOnly,
}: ApprovalDetailPanelProps) {
  const [commentDrafts, setCommentDrafts] = useState<Record<number, string>>({});
  const comment = item ? commentDrafts[item.id] || '' : '';
  const statusField = approvalRole === 'agreement2' ? 'agreement2_status' : 'agreement1_status';
  const currentStatus = item?.[statusField] || 'pending';

  const { data: comments = [], isLoading: commentsLoading } = useQuery<CommentRow[]>({
    queryKey: ['comments', item?.id],
    queryFn: async () => {
      const response = await promoAPI.getComments(item!.id);
      const list = (response as { data?: CommentRow[] })?.data;
      return Array.isArray(list) ? list : [];
    },
    enabled: Boolean(item?.id),
  });

  if (!item) {
    return (
      <Box sx={{ minHeight: 560, display: 'grid', placeItems: 'center', p: 4, bgcolor: '#f8fafc' }}>
        <Box sx={{ textAlign: 'center' }}>
          <Typography variant="h6" color="text.secondary">Выберите промо</Typography>
          <Typography variant="body2" color="text.secondary">Подробности и действия появятся здесь.</Typography>
        </Box>
      </Box>
    );
  }

  const visibleGroups = FIELD_GROUPS.map(group => ({
    ...group,
    fields: group.fields.filter(field => visibleFields.includes(field.id)),
  })).filter(group => group.fields.length > 0);
  const stage1Comment = approvalCommentText(item.agreement1_status, item.agreement1_comment, item.agreement1);
  const stage2Comment = approvalCommentText(item.agreement2_status, item.agreement2_comment, item.agreement2);
  const stageApprovalComments = [
    stage1Comment ? { key: 'agreement1', role: 'Согласование 1', text: stage1Comment } : null,
    stage2Comment ? { key: 'agreement2', role: 'Согласование 2', text: stage2Comment } : null,
  ].filter((entry): entry is { key: string; role: string; text: string } => entry !== null);
  const approvalComments = stageApprovalComments.length === 0 && item.comments?.trim()
    ? [{ key: 'general', role: 'Архивный комментарий', text: item.comments.trim() }]
    : stageApprovalComments;
  const hasCommentHistory = approvalComments.length > 0 || comments.length > 0;

  const updateComment = (value: string) => {
    setCommentDrafts(previous => ({ ...previous, [item.id]: value }));
  };

  const submitComment = async () => {
    const saved = await onCommentOnly(item.id, comment);
    if (saved) updateComment('');
  };

  return (
    <Box sx={{ minWidth: 0, display: 'flex', flexDirection: 'column', bgcolor: '#f8fafc', maxHeight: 720 }}>
      <Box sx={{ px: 3, py: 2.5, bgcolor: 'white', borderBottom: '1px solid #e2e8f0' }}>
        <Stack direction="row" spacing={1.5} sx={{ alignItems: 'flex-start', justifyContent: 'space-between' }}>
          <Box sx={{ minWidth: 0 }}>
            <Typography variant="h6" sx={{ fontWeight: 700, lineHeight: 1.25 }}>
              {visibleFields.includes('network_name') ? item.network_name || `Промо #${item.id}` : `Промо #${item.id}`}
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
              {item.month ? MONTHS[item.month - 1] : 'Месяц не указан'} {item.year}
            </Typography>
          </Box>
          <Chip
            size="small"
            label={statusLabel(currentStatus)}
            color={currentStatus === 'approved' ? 'success' : currentStatus === 'rejected' ? 'error' : 'default'}
            sx={{ fontWeight: 600 }}
          />
        </Stack>
        <Stack direction="row" spacing={0.75} sx={{ flexWrap: 'wrap', gap: 0.75, mt: 1.5 }}>
          {visibleFields.includes('brand_as') && item.brand_as && <Chip size="small" label={item.brand_as} variant="outlined" />}
          {visibleFields.includes('sku') && item.sku && <Chip size="small" label={item.sku} variant="outlined" />}
          {visibleFields.includes('mechanics') && item.mechanics && <Chip size="small" label={item.mechanics} color="primary" variant="outlined" />}
        </Stack>
      </Box>

      <Box sx={{ flex: 1, overflowY: 'auto', p: 2.5 }}>
        {visibleGroups.map(group => {
          const metricFields = group.fields.filter(field => !['network_name', 'brand_as', 'sku', 'mechanics'].includes(field.id));
          if (metricFields.length === 0) return null;
          return (
            <Box key={group.group} sx={{ mb: 2.5 }}>
              <Typography variant="overline" color="text.secondary" sx={{ fontWeight: 700, letterSpacing: '0.08em' }}>
                {group.group}
              </Typography>
              <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 1, mt: 0.5 }}>
                {metricFields.map(field => (
                  <Paper key={field.id} variant="outlined" sx={{ p: 1.25, borderRadius: 2, bgcolor: 'white' }}>
                    <Typography variant="caption" color="text.secondary">{field.label}</Typography>
                    <Typography sx={{ fontWeight: 700, mt: 0.25, color: field.isRoi && Number(item[field.id as keyof ApprovalRow]) < 0 ? '#dc2626' : 'text.primary' }}>
                      {formatValue(item, field)}
                    </Typography>
                  </Paper>
                ))}
              </Box>
            </Box>
          );
        })}

        {item.conditions && (
          <Box sx={{ mb: 2.5 }}>
            <Typography variant="overline" color="text.secondary" sx={{ fontWeight: 700 }}>Условия</Typography>
            <Paper variant="outlined" sx={{ p: 1.5, borderRadius: 2, mt: 0.5, bgcolor: 'white' }}>
              <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>{item.conditions}</Typography>
            </Paper>
          </Box>
        )}

        {(item.agreement1 || item.agreement2 || item.agreement1_status || item.agreement2_status) && (
          <Box sx={{ mb: 2.5 }}>
            <Typography variant="overline" color="text.secondary" sx={{ fontWeight: 700 }}>Маршрут согласования</Typography>
            <Stack spacing={0.75} sx={{ mt: 0.5 }}>
              {(item.agreement1 || item.agreement1_status) && (
                <Alert severity="info" icon={false} sx={{ py: 0.5 }}>
                  <b>Этап 1:</b> {approvalStatusLabel(item.agreement1_status, item.agreement1)}
                </Alert>
              )}
              {(item.agreement2 || item.agreement2_status) && (
                <Alert severity="info" icon={false} sx={{ py: 0.5 }}>
                  <b>Этап 2:</b> {approvalStatusLabel(item.agreement2_status, item.agreement2)}
                </Alert>
              )}
            </Stack>
          </Box>
        )}

        <Box>
          <Typography variant="overline" color="text.secondary" sx={{ fontWeight: 700 }}>Комментарии и решения</Typography>
          {commentsLoading ? (
            <CircularProgress size={18} sx={{ display: 'block', my: 1 }} />
          ) : !hasCommentHistory ? (
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>Комментариев пока нет.</Typography>
          ) : (
            <Stack spacing={0.75} sx={{ mt: 0.5 }}>
              {approvalComments.map(message => {
                const style = ROLE_COLORS[message.key] || ROLE_COLORS['КАМ'];
                return (
                  <Box key={message.key} sx={{ px: 1.25, py: 1, borderRadius: 2, bgcolor: style.bg, borderLeft: `3px solid ${style.border}` }}>
                    <Typography variant="caption" sx={{ fontWeight: 700, color: style.text }}>{message.role}</Typography>
                    <Typography variant="body2" sx={{ mt: 0.25, whiteSpace: 'pre-wrap' }}>{message.text}</Typography>
                  </Box>
                );
              })}
              {comments.map(message => {
                const style = ROLE_COLORS[message.role] || ROLE_COLORS['КАМ'];
                return (
                  <Box key={message.id} sx={{ px: 1.25, py: 1, borderRadius: 2, bgcolor: style.bg, borderLeft: `3px solid ${style.border}` }}>
                    <Typography variant="caption" sx={{ fontWeight: 700, color: style.text }}>
                      {message.role === 'КАМ' ? message.user_name : message.role}
                      {message.created_at ? ` · ${new Date(message.created_at).toLocaleString('ru-RU')}` : ''}
                    </Typography>
                    <Typography variant="body2" sx={{ mt: 0.25, whiteSpace: 'pre-wrap' }}>{message.comment_text}</Typography>
                  </Box>
                );
              })}
            </Stack>
          )}
        </Box>
      </Box>

      <Divider />
      <Box sx={{ p: 2, bgcolor: 'white' }}>
        <TextField
          fullWidth
          multiline
          minRows={2}
          maxRows={4}
          placeholder="Комментарий к решению или сообщение для КАМ"
          value={comment}
          onChange={event => updateComment(event.target.value)}
          disabled={submitting}
        />
        <Stack direction="row" spacing={1} sx={{ mt: 1.25 }}>
          <Button
            variant="outlined"
            startIcon={<CommentIcon />}
            disabled={submitting || !comment.trim()}
            onClick={submitComment}
            sx={{ flex: 1 }}
          >
            Комментарий
          </Button>
          <Button
            variant="contained"
            color="error"
            startIcon={<RejectIcon />}
            disabled={submitting}
            onClick={() => onOpenConfirm(item.id, 'отклонено', comment)}
          >
            Отклонить
          </Button>
          <Button
            variant="contained"
            color="success"
            startIcon={<ApproveIcon />}
            disabled={submitting}
            onClick={() => onOpenConfirm(item.id, 'согласовано', comment)}
          >
            Согласовать
          </Button>
        </Stack>
      </Box>
    </Box>
  );
}
