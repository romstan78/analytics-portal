import { useEffect, useState } from 'react';
import { saveAs } from 'file-saver';
import { Link } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Autocomplete, Box, Button, Checkbox, CircularProgress, Dialog, DialogActions, DialogContent, DialogTitle, FormControlLabel, MenuItem, Stack, Tab, Tabs, TextField, Typography } from '@mui/material';
import { budgetAPI } from '../api/budget';
import type { BudgetParams, BudgetSources } from '../types/budget';
import BudgetTable from '../components/BudgetTable';
import BudgetVisualization from '../components/BudgetVisualization';
import BudgetPromoPanel from '../components/BudgetPromoPanel';
export default function Budget({ role }: {
  role: string | null;
}) {
  const client = useQueryClient();
  const [view, setView] = useState<'table' | 'visualization'>('table');
  const [investmentMode, setInvestmentMode] = useState<'investments' | 'pct'>('investments');
  const [deltaMetric, setDeltaMetric] = useState<'to' | 'pct'>('to');
  const [download, setDownload] = useState<{ url: string; name: string } | null>(null);
  useEffect(() => () => { if (download) URL.revokeObjectURL(download.url); }, [download]);
  const writer = role === 'admin' || role === 'analyst';
  const [expanded, setExpanded] = useState<string[]>([]);
  const [base, setBase] = useState('reg-contract');
  const [year, setYear] = useState(new Date().getFullYear());
  const [version, setVersion] = useState('LIVE');
  const [compare, setCompare] = useState('');
  const [delta, setDelta] = useState('abs');
  const [src, setSrc] = useState('all');
  const [kind, setKind] = useState('all');
  const [gross, setGross] = useState(false);
  const [types, setTypes] = useState<string[]>([]);
  const [composition, setComposition] = useState(true);
  const [sources, setSources] = useState<BudgetSources>({ to: ['forecast', 'forecast', 'forecast', 'forecast'], investments: ['forecast', 'forecast', 'forecast', 'forecast'] });
  const [createOpen, setCreateOpen] = useState(false);
  const [newCode, setNewCode] = useState('F1');
  const [freezeOpen, setFreezeOpen] = useState(false);
  const [promo, setPromo] = useState<{
    brand: string;
    q: number;
  } | null>(null);
  const activeBase = view === 'visualization' && !base.startsWith('reg-') ? 'reg-contract' : base;
  const params: BudgetParams = { composition: composition ? 'expanded' : 'collapsed', expand: expanded, year, version, compare, delta: view === 'visualization' ? 'abs' : delta, types, src, kind, vat: gross ? 'gross' : 'net', base: activeBase, to_sources: sources.to.join(','), investment_sources: sources.investments.join(',') };
  const versions = useQuery({ queryKey: ['budget', 'versions', year], queryFn: () => budgetAPI.versions(year) });
  const query = useQuery({ queryKey: ['budget', 'table', params], queryFn: () => budgetAPI.get(params) });
  const refresh = () => { void client.invalidateQueries({ queryKey: ['budget'] }); };
  const updateSources = useMutation({ mutationFn: (s: BudgetSources) => budgetAPI.sources(query.data!.versionInfo.id, s, query.data!.versionInfo.updatedAt), onSuccess: refresh });
  const excel = useMutation({ mutationFn: () => budgetAPI.export(params), onSuccess: blob => {
    const name = `budget-${year}-${version}${compare ? `_vs_${compare}` : ''}-${new Date().toISOString().slice(0, 10)}.xlsx`;
    setDownload({ url: URL.createObjectURL(blob), name });
    saveAs(blob, name);
  } });
  const create = useMutation({ mutationFn: () => budgetAPI.create(year, newCode, sources), onSuccess: v => { setVersion(v.code); setCreateOpen(false); refresh(); } });
  const freeze = useMutation({ mutationFn: () => budgetAPI.freeze(query.data!.versionInfo.id, query.data!.versionInfo.updatedAt), onSuccess: () => { setFreezeOpen(false); refresh(); } });
  const edit = useMutation({ mutationFn: ({ brand, q, metric, value, reset }: {
      brand: string;
      q: number;
      metric: string;
      value: number;
      reset?: boolean;
    }) => budgetAPI.cell(query.data!.versionInfo.id, params, { brand, quarter: q, metric, value, updated_at: query.data!.versionInfo.updatedAt }, reset), onSuccess: refresh });
  const available = [{ code: 'LIVE', name: 'Текущее' }, { code: 'PY', name: 'Факт прошлого года' }, ...(versions.data ?? []).filter(v => v.code !== 'LIVE')];
  const sourceControls = (s: BudgetSources, update: (v: BudgetSources) => void, disabled = false) => <Box sx={{ display: 'grid', gridTemplateColumns: '140px repeat(4, minmax(100px, 1fr))', gap: 1, my: 2, overflowX: 'auto' }}><span />{[1, 2, 3, 4].map(q => <Typography key={q} sx={{ textAlign: "center" }}>Q{q}</Typography>)}{(['to', 'investments'] as const).map(key => <Box key={key} sx={{ display: 'contents' }}><Typography sx={{ alignSelf: "center" }}>{key === 'to' ? 'Источник ТО' : 'Источник GTN'}</Typography>{s[key].map((value, i) => <TextField key={i} select size="small" value={value} disabled={disabled} onChange={e => update({ ...s, [key]: s[key].map((v, q) => q === i ? e.target.value : v) })} slotProps={{ select: { inputProps: { 'aria-label': `${key === 'to' ? 'ТО' : 'GTN'} Q${i + 1}` } } }}>{[['plan', 'План'], ['fact', 'Факт'], ['forecast', 'Прогноз']].map(([v, name]) => <MenuItem key={v} value={v}>{name}</MenuItem>)}</TextField>)}</Box>)}</Box>;
  return <Box sx={{ maxWidth: view === 'visualization' ? 1900 : 1600, mx: 'auto', p: { xs: 2, md: 3 } }}>
  <Button component={Link} to="/" sx={{ mb: 1 }}>На главную</Button><Stack direction="row" sx={{ justifyContent: "space-between", alignItems: "center" }}><Box><Typography variant="h4">Бюджет</Typography><Typography color="text.secondary">ТО и инвестиции по брендам · млн ₽</Typography></Box><Stack direction="row" sx={{ gap: 1 }}><Button variant="outlined" disabled={excel.isPending || !query.data} onClick={() => excel.mutate()}>{excel.isPending ? <CircularProgress size={16}/> : 'Excel'}</Button>{writer && <Button variant="outlined" disabled={versions.data?.length === 5} onClick={() => { setNewCode(['B', 'F1', 'F2', 'F3', 'LIVE'].find(v => !versions.data?.some(x => x.code === v)) ?? 'B'); if (query.data?.versionInfo.sources)
    setSources(query.data.versionInfo.sources); setCreateOpen(true); }}>Создать версию</Button>}{writer && query.data?.versionInfo.id && query.data.versionInfo.status === 'draft' ? <Button variant="contained" onClick={() => setFreezeOpen(true)}>Заморозить</Button> : null}</Stack></Stack>
  <Tabs value={view} onChange={(_, value) => setView(value)} aria-label="Представление бюджета" sx={{ mt: 2, borderBottom: 1, borderColor: 'divider' }}>
    <Tab value="table" label="Таблица" id="budget-table-tab" aria-controls="budget-table-panel" />
    <Tab value="visualization" label="Визуализация" id="budget-visualization-tab" aria-controls="budget-visualization-panel" />
  </Tabs>
  {download && <Alert severity="success" sx={{ mt: 2 }}>Excel готов. <a href={download.url} download={download.name}>Скачать файл</a></Alert>}
  <Stack direction="row" sx={{ flexWrap: "wrap", gap: 2, my: 3 }}>
   <TextField size="small" type="number" label="Год" value={year} onChange={e => { const y = Number(e.target.value); if (y >= 2000 && y <= 2100) {
    setYear(y);
    setVersion('LIVE');
    setCompare('');
  } }} sx={{ width: 110 }}/>
   <TextField select size="small" label="Версия" value={version} onChange={e => { setVersion(e.target.value); if (compare === e.target.value)
    setCompare(''); }} sx={{ minWidth: 140 }}>{available.map(v => <MenuItem key={v.code} value={v.code}>{v.name}</MenuItem>)}</TextField>
   <TextField select size="small" label="Сравнить с" slotProps={{ inputLabel: { shrink: true }, select: { displayEmpty: true } }} value={compare} onChange={e => setCompare(e.target.value)} sx={{ minWidth: 140 }}><MenuItem value="">Не сравнивать</MenuItem>{available.filter(v => v.code !== version).map(v => <MenuItem key={v.code} value={v.code}>{v.name}</MenuItem>)}</TextField>
   {view === 'table' && !!compare && <TextField select size="small" label="Δ" value={delta} onChange={e => setDelta(e.target.value)}><MenuItem value="abs">млн ₽</MenuItem><MenuItem value="pct">%</MenuItem></TextField>}
   <TextField select size="small" label={view === 'visualization' ? 'База ТО' : 'База процента'} value={activeBase} onChange={e => setBase(e.target.value)} sx={{ minWidth: 180 }}>{[['reg-contract', 'ТО · цена контракта'], ['reg-olap', 'ТО · цена OLAP'], ['ss', 'OLAP SS'], ['sswo', 'OLAP SS wo Ecom'], ['pure', 'PURE'], ['omni', 'OMNI'], ['mp', 'MP']].filter(([v]) => view === 'table' || v.startsWith('reg-')).map(([v, n]) => <MenuItem key={v} value={v}>{n}</MenuItem>)}</TextField>
   <Autocomplete multiple disableCloseOnSelect size="small" limitTags={1}
    options={query.data?.networkTypes ?? []} value={types} onChange={(_, values) => setTypes(values)}
    clearText="Все типы" openText="Выбрать типы" closeText="Закрыть"
    sx={{ width: 280 }}
    renderOption={(props, option, { selected }) => {
      const { key, ...optionProps } = props;
      return <li key={key} {...optionProps}><Checkbox checked={selected} tabIndex={-1} sx={{ mr: 1, p: .5 }}/>{option}</li>;
    }}
    renderInput={params => <TextField {...params} label="Тип сети" placeholder={types.length ? '' : 'Все типы'} slotProps={{ ...params.slotProps, inputLabel: { ...params.slotProps.inputLabel, shrink: true } }}/>} />
   <TextField select size="small" label="Источник инвестиций" value={src} onChange={e => setSrc(e.target.value)} sx={{ minWidth: 160 }}>{[['all', 'Всего'], ['contract', 'Контракт'], ['promo', 'Промо']].map(([v, name]) => <MenuItem value={v} key={v}>{name}</MenuItem>)}</TextField>
   <TextField select size="small" label="Тип инвестиций" value={kind} onChange={e => setKind(e.target.value)} sx={{ minWidth: 140 }}>{[['all', 'Всего'], ['gtn', 'GTN'], ['opex', 'OPEX']].map(([v, name]) => <MenuItem value={v} key={v}>{name}</MenuItem>)}</TextField>
   <FormControlLabel control={<Checkbox checked={gross} onChange={(_, v) => setGross(v)}/>} label="С НДС"/>{view === 'table' && <FormControlLabel control={<Checkbox checked={composition} onChange={(_, v) => setComposition(v)}/>} label="Развернуть состав"/>}
  </Stack>
  {view === 'table' && (query.data && (query.data.versionInfo.id > 0 || version === 'PY') ? <>{sourceControls(query.data.versionInfo.sources, s => updateSources.mutate(s), !writer || query.data.versionInfo.status !== 'draft' || updateSources.isPending)}<Typography variant="caption">{query.data.versionInfo.status === 'frozen' ? `Заморожена ${query.data.versionInfo.frozenAt}` : 'Черновик · реестр читается живьём'}</Typography></> : sourceControls(sources, setSources, !writer))}
  {query.isPending && <CircularProgress aria-label="Загрузка бюджета"/>}{[query.error, versions.error, edit.error, freeze.error, excel.error, updateSources.error].filter(Boolean).map((e, i) => <Alert key={i} severity="error" sx={{ mb: 1 }}>{e?.message}</Alert>)}
  {query.data && view === 'table' && <Box role="tabpanel" id="budget-table-panel" aria-labelledby="budget-table-tab"><Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 1 }}>{query.data.editReason || 'Клик по ТО или контрактным инвестициям — правка в млн ₽; Enter — сохранить, Esc — отменить.'}</Typography><Typography variant="caption" sx={{ display: 'block', mb: 1 }}>Официальный прогноз: {query.data.forecastCoveragePct == null ? '—' : `${query.data.forecastCoveragePct.toLocaleString('ru-RU')} %`} открытых квартальных ячеек. Нераспределённый остаток пула входит в ТО при источнике «План».</Typography><BudgetTable data={query.data} expanded={expanded} onExpand={brand => setExpanded(s => s.includes(brand) ? s.filter(b => b !== brand) : [...s, brand])} sales={!base.startsWith('reg-')} composition={composition} source={src} kind={kind} delta={delta} busy={edit.isPending || query.isFetching} onEdit={(brand, q, metric, value, reset) => edit.mutate({ brand, q, metric, value, reset })} onPromo={(brand, q) => setPromo({ brand, q })}/></Box>}
  {query.data && view === 'visualization' && <Box role="tabpanel" id="budget-visualization-panel" aria-labelledby="budget-visualization-tab"><BudgetVisualization data={query.data} investmentMode={investmentMode} onInvestmentMode={setInvestmentMode} deltaMetric={deltaMetric} onDeltaMetric={setDeltaMetric}/></Box>}
  {promo && query.data && <BudgetPromoPanel key={`${version}-${promo.brand}-${promo.q}`} params={params} brand={promo.brand} quarter={promo.q} versionId={query.data.versionInfo.id} editable={writer && query.data.versionInfo.status === 'draft' && query.data.versionInfo.id > 0 && query.data.quarterStates[promo.q - 1] !== 'closed'} onClose={() => setPromo(null)}/>}
  <Dialog open={createOpen} onClose={() => setCreateOpen(false)} maxWidth="md" fullWidth><DialogTitle>Новая версия бюджета</DialogTitle><DialogContent><TextField select label="Код" value={newCode} onChange={e => setNewCode(e.target.value)} sx={{ mt: 1, minWidth: 160 }}>{['B', 'F1', 'F2', 'F3', 'LIVE'].filter(v => !versions.data?.some(x => x.code === v)).map(v => <MenuItem key={v} value={v}>{v === 'LIVE' ? 'Текущее' : v}</MenuItem>)}</TextField>{sourceControls(sources, setSources)}{create.error && <Alert severity="error">{create.error.message}</Alert>}</DialogContent><DialogActions><Button onClick={() => setCreateOpen(false)}>Отмена</Button><Button disabled={create.isPending} onClick={() => create.mutate()}>Создать</Button></DialogActions></Dialog>
  <Dialog open={freezeOpen} onClose={() => setFreezeOpen(false)}><DialogTitle>Заморозить {version}?</DialogTitle><DialogContent>Будут зафиксированы суммы, выбранные источники и состав промо. Изменять эту версию после заморозки нельзя.</DialogContent><DialogActions><Button onClick={() => setFreezeOpen(false)}>Отмена</Button><Button disabled={freeze.isPending} onClick={() => freeze.mutate()}>Заморозить</Button></DialogActions></Dialog>
 </Box>;
}
