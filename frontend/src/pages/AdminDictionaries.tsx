import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import {
  Alert, Box, Button, CircularProgress, Dialog, DialogActions, DialogContent,
  DialogTitle, IconButton, InputAdornment, Paper, Snackbar, Tab, Table,
  TableBody, TableCell, TableContainer, TableHead, TableRow, Tabs, TextField,
  Typography,
} from '@mui/material';
import {
  Add as AddIcon, ArrowBack as ArrowBackIcon, EditOutlined as EditIcon,
  MenuBookOutlined as DictionaryIcon, Search as SearchIcon,
} from '@mui/icons-material';
import {
  dictionariesAPI, type DictionaryData, type DictionaryKind, type DictionaryRow,
} from '../api/dictionaries';

type DataKey = keyof Pick<DictionaryData, 'skus' | 'networks' | 'kam_networks' | 'mechanics'>;
type Field = { key: string; label: string; required?: boolean; type?: string; immutable?: boolean; width?: number };
type DictionaryConfig = { key: DataKey; apiKind: DictionaryKind; label: string; singular: string; fields: Field[] };

const CONFIGS: DictionaryConfig[] = [
  {
    key: 'skus', apiKind: 'skus', label: 'SKU', singular: 'SKU',
    fields: [
      { key: 'sku', label: 'SKU', required: true, immutable: true, width: 260 },
      { key: 'brand', label: 'Бренд', width: 220 },
      { key: 'brand_as', label: 'Бренд АС', width: 220 },
    ],
  },
  {
    key: 'networks', apiKind: 'networks', label: 'Сети', singular: 'сеть',
    fields: [
      { key: 'network_name', label: 'Название сети', required: true, immutable: true, width: 260 },
      { key: 'kam', label: 'КАМ', width: 210 },
      { key: 'network_type', label: 'Тип сети', width: 150 },
      { key: 'top20_segment', label: 'Сегмент', width: 160 },
      { key: 'key_region', label: 'Ключевой регион', width: 190 },
    ],
  },
  {
    key: 'kam_networks', apiKind: 'kam-networks', label: 'Закрепления КАМ', singular: 'закрепление',
    fields: [
      { key: 'kam', label: 'КАМ', required: true, width: 240 },
      { key: 'network_name', label: 'Название сети', required: true, width: 280 },
      { key: 'valid_from', label: 'Действует с', required: true, type: 'date', width: 160 },
    ],
  },
  {
    key: 'mechanics', apiKind: 'mechanics', label: 'Механики промо', singular: 'механику',
    fields: [
      { key: 'mechanics', label: 'Механика', required: true, immutable: true, width: 360 },
      { key: 'short_code', label: 'Короткий код', width: 150 },
      { key: 'channel', label: 'Канал', required: true, width: 180 },
    ],
  },
];

const emptyFor = (config: DictionaryConfig): Record<string, string | number> =>
  Object.fromEntries([['id', 0], ...config.fields.map(field => [field.key, field.type === 'date' ? new Date().toISOString().slice(0, 10) : ''])]);

const errorText = (error: unknown) => {
  if (error && typeof error === 'object' && 'message' in error) return String(error.message);
  return 'Не удалось сохранить запись';
};

export default function AdminDictionaries() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [tab, setTab] = useState(0);
  const [search, setSearch] = useState('');
  const [editor, setEditor] = useState<Record<string, string | number> | null>(null);
  const [notice, setNotice] = useState<{ severity: 'success' | 'error'; message: string } | null>(null);
  const config = CONFIGS[tab];

  const query = useQuery({ queryKey: ['admin-dictionaries'], queryFn: dictionariesAPI.getAll });
  const rows = useMemo(() => {
    const source = query.data?.[config.key] ?? [];
    const needle = search.trim().toLocaleLowerCase('ru');
    if (!needle) return source;
    return source.filter(row => config.fields.some(field => String((row as unknown as Record<string, unknown>)[field.key] ?? '').toLocaleLowerCase('ru').includes(needle)));
  }, [config, query.data, search]);

  const mutation = useMutation({
    mutationFn: (row: Partial<DictionaryRow>) => dictionariesAPI.save(config.apiKind, row),
    onSuccess: async row => {
      await queryClient.invalidateQueries({ queryKey: ['admin-dictionaries'] });
      await queryClient.invalidateQueries({ queryKey: ['promoFilters'] });
      setEditor(null);
      setNotice({ severity: 'success', message: row.id ? 'Запись справочника сохранена' : 'Запись добавлена' });
    },
    onError: error => setNotice({ severity: 'error', message: errorText(error) }),
  });

  const openCreate = () => setEditor(emptyFor(config));
  const openEdit = (row: DictionaryRow) => setEditor({ ...(row as unknown as Record<string, string | number>) });
  const isEdit = Number(editor?.id ?? 0) > 0;
  const isValid = editor != null && config.fields.every(field => !field.required || String(editor[field.key] ?? '').trim() !== '');

  if (query.isLoading) return <Box sx={{ display: 'grid', placeItems: 'center', minHeight: '70vh' }}><CircularProgress /></Box>;

  return (
    <Box sx={{ p: { xs: 2, md: 4 }, maxWidth: 1500, mx: 'auto', width: '100%' }}>
      <Button startIcon={<ArrowBackIcon />} onClick={() => navigate('/')} sx={{ mb: 2 }}>На главную</Button>
      <Box sx={{ display: 'flex', alignItems: { xs: 'flex-start', sm: 'center' }, flexDirection: { xs: 'column', sm: 'row' }, gap: 2, mb: 3 }}>
        <Box sx={{ width: 52, height: 52, display: 'grid', placeItems: 'center', borderRadius: 3, bgcolor: '#eef2ff', color: '#4f46e5' }}><DictionaryIcon /></Box>
        <Box sx={{ flex: 1 }}>
          <Typography variant="h4" sx={{ fontWeight: 750 }}>Справочники</Typography>
          <Typography color="text.secondary">Мастер-данные промо, продаж и реестра сетей</Typography>
        </Box>
        <Button variant="contained" startIcon={<AddIcon />} onClick={openCreate}>Добавить</Button>
      </Box>

      {query.isError && <Alert severity="error" sx={{ mb: 2 }}>Не удалось загрузить справочники</Alert>}
      <Paper variant="outlined" sx={{ borderRadius: 3, overflow: 'hidden' }}>
        <Tabs value={tab} onChange={(_, value) => { setTab(value); setSearch(''); }} variant="scrollable" scrollButtons="auto" sx={{ px: 1, borderBottom: '1px solid', borderColor: 'divider' }}>
          {CONFIGS.map(item => <Tab key={item.key} label={`${item.label} · ${query.data?.[item.key]?.length ?? 0}`} />)}
        </Tabs>
        <Box sx={{ p: 2, display: 'flex', alignItems: 'center', gap: 2 }}>
          <TextField
            size="small" value={search} onChange={event => setSearch(event.target.value)} placeholder={`Поиск: ${config.label.toLocaleLowerCase('ru')}`}
            sx={{ width: { xs: '100%', sm: 380 } }}
            slotProps={{ input: { startAdornment: <InputAdornment position="start"><SearchIcon fontSize="small" /></InputAdornment> } }}
          />
          <Typography variant="body2" color="text.secondary" sx={{ ml: 'auto' }}>Найдено: {rows.length}</Typography>
        </Box>
        <TableContainer sx={{ maxHeight: 'calc(100vh - 310px)', minHeight: 320 }}>
          <Table stickyHeader size="small">
            <TableHead><TableRow>
              {config.fields.map(field => <TableCell key={field.key} sx={{ minWidth: field.width, fontWeight: 700 }}>{field.label}</TableCell>)}
              <TableCell align="right" sx={{ width: 70, fontWeight: 700 }}>Правка</TableCell>
            </TableRow></TableHead>
            <TableBody>
              {rows.map(row => <TableRow key={row.id} hover>
                {config.fields.map(field => <TableCell key={field.key}>{String((row as unknown as Record<string, unknown>)[field.key] ?? '') || '—'}</TableCell>)}
                <TableCell align="right"><IconButton size="small" aria-label="Редактировать" onClick={() => openEdit(row)}><EditIcon fontSize="small" /></IconButton></TableCell>
              </TableRow>)}
              {!rows.length && <TableRow><TableCell colSpan={config.fields.length + 1} align="center" sx={{ py: 7, color: 'text.secondary' }}>Записи не найдены</TableCell></TableRow>}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>

      <Dialog open={editor !== null} onClose={() => !mutation.isPending && setEditor(null)} fullWidth maxWidth="sm">
        <DialogTitle>{isEdit ? `Изменить ${config.singular}` : `Добавить ${config.singular}`}</DialogTitle>
        <DialogContent sx={{ display: 'grid', gap: 2, pt: '12px !important' }}>
          {isEdit && config.fields.some(field => field.immutable) && <Alert severity="info">Ключевое название защищено от переименования, чтобы не разорвать связи с историческими данными.</Alert>}
          {config.fields.map(field => <TextField
            key={field.key} label={field.label} required={field.required} type={field.type ?? 'text'}
            value={editor?.[field.key] ?? ''} disabled={isEdit && field.immutable}
            onChange={event => setEditor(current => current ? { ...current, [field.key]: event.target.value } : current)}
            slotProps={{
              ...(field.type === 'date' ? { inputLabel: { shrink: true } } : {}),
              ...(field.key === 'short_code' ? { htmlInput: { maxLength: 12 } } : {}),
            }}
          />)}
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button onClick={() => setEditor(null)} disabled={mutation.isPending}>Отмена</Button>
          <Button variant="contained" disabled={!isValid || mutation.isPending} onClick={() => editor && mutation.mutate(editor as Partial<DictionaryRow>)}>
            {mutation.isPending ? 'Сохранение…' : 'Сохранить'}
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar open={notice !== null} autoHideDuration={3500} onClose={() => setNotice(null)}>
        <Alert severity={notice?.severity ?? 'success'} onClose={() => setNotice(null)}>{notice?.message}</Alert>
      </Snackbar>
    </Box>
  );
}
