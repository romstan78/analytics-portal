import { Fragment, useRef, useState } from 'react';
import { Box, Button, IconButton, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, TextField, Tooltip, Typography } from '@mui/material';
import { Link } from 'react-router-dom';
import type { BudgetResponse, BudgetBrand, BudgetLine, BudgetCell } from '../types/budget';
import { budgetFormat, budgetParse, budgetMarks, budgetMarkTitles } from '../utils/budget';
interface Props {
  data: BudgetResponse;
  expanded: string[];
  onExpand: (brand: string) => void;
  sales: boolean;
  composition: boolean;
  source: string;
  kind: string;
  delta: string;
  busy: boolean;
  onEdit: (brand: string, q: number, metric: string, value: number, reset?: boolean) => void;
  onPromo: (brand: string, q: number) => void;
}
function EditableCell({ cell, brand, q, metric, busy, onEdit }: {
  cell: BudgetCell;
  brand: string;
  q: number;
  metric: string;
  busy: boolean;
  onEdit: Props['onEdit'];
}) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState('');
  const submitted = useRef(false);
  const commit = () => { if (submitted.current)
    return; const n = budgetParse(value); if (n == null)
    return; submitted.current = true; setEditing(false); onEdit(brand, q, metric, n); };
  if (editing)
    return <TextField autoFocus size="small" value={value} onChange={e => setValue(e.target.value)} onBlur={commit} error={budgetParse(value) == null} slotProps={{ htmlInput: { 'aria-label': `Новое значение ${brand}, Q${q}, млн ₽`, style: { textAlign: 'right', width: 105 } } }} onKeyDown={e => { if (e.key === 'Enter')
      commit(); if (e.key === 'Escape') {
      submitted.current = true;
      setEditing(false);
    } }}/>;
  return <Box sx={{ display: 'inline-flex', alignItems: 'center', gap: .5 }}>
  <Tooltip title={cell.edited ? `Реестр: ${budgetFormat(cell.edited.base)} → ${budgetFormat(cell.edited.value)} · ${cell.edited.who}, ${cell.edited.when}` : budgetMarkTitles[cell.mark] || ''}>
   <Button size="small" disabled={!cell.editable || busy} sx={{ minWidth: 0, p: 0, font: 'inherit', color: 'inherit', '&.Mui-disabled': { color: 'inherit' } }} onClick={() => { submitted.current = false; setValue(String((cell.a ?? 0) / 1000000)); setEditing(true); }}>{budgetFormat(cell.a)} {cell.edited ? '✎' : budgetMarks[cell.mark]}</Button>
  </Tooltip>
  {cell.edited && cell.editable && <IconButton size="small" disabled={busy} aria-label={`Вернуть значение реестра ${brand} Q${q}`} onClick={() => onEdit(brand, q, metric, 0, true)}>↺</IconButton>}
 </Box>;
}
export default function BudgetTable({ data, expanded, onExpand, sales, composition, source, kind, delta, busy, onEdit, onPromo }: Props) {
  const compared = !!data.compare;
  const lines = (b: BudgetBrand, network = false, total = false) => {
    const items: [
      string,
      BudgetLine,
      string,
      boolean
    ][] = [['ТО', b.to, 'to', false], ['Инвестиции', b.investments, 'inv', false]];
    if (composition) {
      if (source !== 'promo' && kind !== 'opex') items.push(['GTN · контракт', b.gtnContract, 'gtnC', false]);
      if (source !== 'promo' && kind !== 'gtn') items.push(['OPEX · контракт', b.opexContract, 'opexC', false]);
      if (source !== 'contract' && kind !== 'opex') items.push(['GTN · промо', b.gtnPromo, 'gtnP', true]);
      if (source !== 'contract' && kind !== 'gtn') items.push(['OPEX · промо', b.opexPromo, 'opexP', true]);
    }
    if (sales)
      items.push(['Продажи', b.sales, 'sales', false]);
    items.push(['%', b.pct, 'pct', false]);
    return items.map(([label, line, metric, promo], index) => <TableRow key={metric} sx={{ bgcolor: total ? '#f8fafc' : network ? '#fcfcfd' : 'background.paper', '& td': { borderTop: index === 0 ? '1px solid #cbd5e1' : undefined } }}>
   <TableCell sx={{ position: 'sticky', left: 0, zIndex: 2, bgcolor: 'inherit', pl: network ? 4 : index ? 3 : 1, minWidth: 240 }}>
    {index === 0 ? <Box>{network ? <Link to={`/network-registry?network=${b.networkId}&year=${data.year}`}>{b.brand}</Link> : total ? <b>Итого</b> : <Button size="small" color="inherit" onClick={() => onExpand(b.brand)}>{expanded.includes(b.brand) ? '▾' : '▸'} {b.brand}</Button>}<Typography component="span" variant="caption" sx={{ ml: 1 }}>ТО</Typography></Box> : label}
   </TableCell>
   {[...line.q, line.year].map((cell, q) => <Fragment key={q}>
    <TableCell align="right" sx={{ borderLeft: '1px solid #e2e8f0', bgcolor: cell.edited ? '#fffbeb' : q < 4 && data.quarterStates[q] !== 'closed' ? '#f5f3ff' : undefined, fontWeight: metric === 'pct' || q === 4 ? 600 : undefined }}>
     {promo && q < 4 && !network ? <Button size="small" sx={{ minWidth: 0, p: 0 }} onClick={() => onPromo(total ? '' : b.brand, q + 1)}>{budgetFormat(cell.a)}</Button> : metric === 'pct' ? budgetFormat(cell.a, true) : <EditableCell cell={cell} brand={b.brand} q={q + 1} metric={metric} busy={busy} onEdit={onEdit}/>}
    </TableCell>
    {compared && <><TableCell align="right">{budgetFormat(cell.b, metric === 'pct')}</TableCell><TableCell align="right" sx={{ color: cell.delta == null || cell.delta === 0 || (metric !== 'to' && metric !== 'pct') ? 'text.secondary' : (metric === 'pct' ? cell.delta < 0 : cell.delta > 0) ? 'success.main' : 'error.main' }}>{metric === 'pct' ? `${budgetFormat(cell.delta, true).replace(' %', '')}${cell.delta == null ? '' : ' п.п.'}` : budgetFormat(cell.delta, delta === 'pct')}</TableCell></>}
   </Fragment>)}
  </TableRow>);
  };
  return <TableContainer sx={{ maxHeight: '72vh', border: '1px solid #e2e8f0', borderRadius: 2 }}><Table size="small" stickyHeader sx={{ fontVariantNumeric: 'tabular-nums', '& td': { py: .5, whiteSpace: 'nowrap', fontSize: 13 } }}>
  <TableHead><TableRow><TableCell>Бренд / показатель · млн ₽</TableCell>{['Q1', 'Q2', 'Q3', 'Q4', 'Год'].map((q, i) => <TableCell key={q} align="center" colSpan={compared ? 3 : 1}>{q}{i < 4 && <Typography variant="caption" sx={{ display: "block" }}>{({ closed: 'закрыт', current: 'текущий', future: 'будущий', budget: 'план' } as Record<string, string>)[data.quarterStates[i]]}</Typography>}</TableCell>)}</TableRow>{compared && <TableRow><TableCell />{Array.from({ length: 5 }, (_, i) => <Fragment key={i}><TableCell align="right">{data.version}{i < 4 && <Typography variant="caption" sx={{ display: 'block' }}>{({ closed: 'закрыт', current: 'текущий', future: 'будущий', budget: 'план' } as Record<string, string>)[data.quarterStates[i]]}</Typography>}</TableCell><TableCell align="right">{data.compare}{i < 4 && <Typography variant="caption" sx={{ display: 'block' }}>{({ closed: 'закрыт', current: 'текущий', future: 'будущий', budget: 'план' } as Record<string, string>)[data.compareStates[i]]}</Typography>}</TableCell><TableCell align="right">Δ</TableCell></Fragment>)}</TableRow>}</TableHead>
  <TableBody>{data.brands.map(b => <Fragment key={b.brand}>{lines(b)}{expanded.includes(b.brand) && b.networks.map(n => <Fragment key={n.networkId}>{lines(n, true)}</Fragment>)}</Fragment>)}{lines(data.total, false, true)}</TableBody>
 </Table></TableContainer>;
}
