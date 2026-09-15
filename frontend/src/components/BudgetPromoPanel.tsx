import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Alert, Box, Button, Checkbox, Drawer, FormControlLabel, MenuItem, Pagination, Stack, TextField, Typography, CircularProgress } from '@mui/material';
import { budgetAPI } from '../api/budget';
import { budgetFormat } from '../utils/budget';
import type { BudgetParams } from '../types/budget';
interface Props {
  params: BudgetParams;
  brand: string;
  quarter: number;
  versionId: number;
  editable: boolean;
  onClose: () => void;
}
export default function BudgetPromoPanel({ params, brand, quarter, versionId, editable, onClose }: Props) {
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState('');
  const [type, setType] = useState('');
  const [selected, setSelected] = useState<number[]>([]);
  const client = useQueryClient();
  const q = useQuery({ queryKey: ['budget', 'promos', params, brand, quarter, page, status, type], queryFn: () => budgetAPI.promos(params, brand, quarter, page, status, type) });
  const save = useMutation({ mutationFn: ({ ids, included }: {
      ids: number[];
      included: boolean;
    }) => budgetAPI.include(versionId, ids, included, q.data?.updatedAt ?? ''), onSuccess: () => { setSelected([]); void client.invalidateQueries({ queryKey: ['budget'] }); } });
  return <Drawer anchor="right" open onClose={onClose} slotProps={{ paper: { sx: { width: { xs: '100%', md: 680 }, p: 3 } } }}>
  <Stack direction="row" sx={{ justifyContent: "space-between" }}><Typography variant="h6">Промо · {brand || 'Портфель'} · Q{quarter}</Typography><Button onClick={onClose}>Закрыть</Button></Stack>
  <Stack direction="row" sx={{ gap: 2, my: 2 }}><TextField select size="small" label="Статус" value={status} onChange={e => { setStatus(e.target.value); setPage(1); setSelected([]); }} sx={{ minWidth: 180 }}><MenuItem value="">Все</MenuItem>{['проведено', 'согласовано', 'на согласовании', 'черновик', 'исключено'].map(s => <MenuItem value={s} key={s}>{s}</MenuItem>)}</TextField><TextField select size="small" label="Тип" value={type} onChange={e => { setType(e.target.value); setPage(1); setSelected([]); }} sx={{ minWidth: 120 }}><MenuItem value="">Все</MenuItem><MenuItem value="gtn">GTN</MenuItem><MenuItem value="opex">OPEX</MenuItem></TextField></Stack>
  {q.isPending && <CircularProgress />}{q.error && <Alert severity="error">{q.error.message}</Alert>}{save.error && <Alert severity="error">{save.error.message}</Alert>}
  {q.data && <><Typography>{q.data.total} промо · {budgetFormat(q.data.sum)} млн ₽ · топ-10: {budgetFormat(q.data.top10Pct, true)}</Typography>
   {editable && <Stack direction="row" sx={{ my: 1 }}><Button disabled={!selected.length || save.isPending} onClick={() => save.mutate({ ids: selected, included: true })}>Включить выбранные</Button><Button disabled={!selected.length || save.isPending} onClick={() => save.mutate({ ids: selected, included: false })}>Исключить выбранные</Button></Stack>}
   {q.data.data.map(p => <Box key={p.id} sx={{ borderBottom: '1px solid #e2e8f0', py: 1.5 }}>
    <Stack direction="row" sx={{ justifyContent: "space-between", alignItems: "center" }}><Box>{editable && <Checkbox size="small" checked={selected.includes(p.id)} onChange={(_, checked) => setSelected(s => checked ? [...s, p.id] : s.filter(id => id !== p.id))} slotProps={{ input: { 'aria-label': `Выбрать промо ${p.id}` } }}/>}<a href={`/promo-analysis?promo=${p.id}`}>#{p.id} · {p.mechanics}</a></Box><b>{budgetFormat(p.a)} млн ₽</b></Stack>
    <Typography variant="body2">{p.network} · {p.brand} · {p.status} · {p.type || 'GTN'}</Typography>
    {params.compare && <Typography variant="body2">{params.version}: {budgetFormat(p.a)} · {params.compare}: {budgetFormat(p.b)} · Δ {budgetFormat(p.delta)} · {p.change}</Typography>}
    <FormControlLabel control={<Checkbox size="small" checked={p.included} disabled={!editable || save.isPending || p.status === 'исключено'} onChange={(_, included) => save.mutate({ ids: [p.id], included })}/>} label="В бюджете"/>
   </Box>)}<Pagination sx={{ mt: 2 }} count={Math.max(1, Math.ceil(q.data.total / 50))} page={page} onChange={(_, n) => { setPage(n); setSelected([]); }}/></>}
 </Drawer>;
}
