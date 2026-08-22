import { useMemo, useState } from 'react';
import {
  Button, Chip, Dialog, DialogActions, DialogContent, DialogTitle,
  Stack, TextField, Tooltip, Typography,
} from '@mui/material';
import { BookmarkAdd as BookmarkAddIcon } from '@mui/icons-material';
import { getUsername } from '../api/auth';

export interface InternetSalesViewSnapshot {
  view: 'dashboard' | 'summary' | 'details';
  analysisYear: string;
  filters: {
    yearFrom: string;
    yearTo: string;
    months: number[];
    quarters: number[];
    brandName: string[];
    productName: string[];
    networkName: string[];
  };
  focusChannel: string;
  focusSegments: string[];
  comparisonChannels: string[];
  unit: 'руб' | 'евро' | 'уп';
  summaryGranularity: 'year' | 'quarter' | 'month';
}

interface SavedView {
  id: string;
  name: string;
  snapshot: InternetSalesViewSnapshot;
}

interface InternetSalesSavedViewsProps {
  current: InternetSalesViewSnapshot;
  onApply: (snapshot: InternetSalesViewSnapshot) => void;
}

const MAX_SAVED_VIEWS = 12;

function storageKey() {
  return `internet_sales_saved_views_v1:${getUsername() || 'local'}`;
}

function loadSavedViews(): SavedView[] {
  try {
    const parsed = JSON.parse(localStorage.getItem(storageKey()) || '[]') as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((item): item is SavedView => Boolean(
      item && typeof item === 'object'
      && typeof (item as SavedView).id === 'string'
      && typeof (item as SavedView).name === 'string'
      && (item as SavedView).snapshot,
    )).slice(0, MAX_SAVED_VIEWS);
  } catch {
    return [];
  }
}

export default function InternetSalesSavedViews({ current, onApply }: InternetSalesSavedViewsProps) {
  const [views, setViews] = useState<SavedView[]>(loadSavedViews);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [name, setName] = useState('');

  const normalizedName = name.trim();
  const duplicate = useMemo(
    () => views.some(view => view.name.toLocaleLowerCase('ru-RU') === normalizedName.toLocaleLowerCase('ru-RU')),
    [normalizedName, views],
  );

  const persist = (next: SavedView[]) => {
    setViews(next);
    localStorage.setItem(storageKey(), JSON.stringify(next));
  };

  const saveCurrent = () => {
    if (!normalizedName) return;
    const saved: SavedView = {
      id: crypto.randomUUID?.() || `${Date.now()}-${Math.random()}`,
      name: normalizedName,
      snapshot: current,
    };
    const next = duplicate
      ? views.map(view => view.name.toLocaleLowerCase('ru-RU') === normalizedName.toLocaleLowerCase('ru-RU') ? { ...saved, id: view.id } : view)
      : [...views, saved].slice(-MAX_SAVED_VIEWS);
    persist(next);
    setName('');
    setDialogOpen(false);
  };

  return (
    <>
      <Stack direction="row" spacing={0.75} sx={{ alignItems: 'center', flexWrap: 'wrap', rowGap: 0.75 }}>
        <Typography variant="caption" color="text.secondary" sx={{ mr: 0.25 }}>Представления:</Typography>
        {views.map(saved => (
          <Tooltip key={saved.id} title="Применить сохранённые фильтры и режим">
            <Chip
              size="small"
              variant="outlined"
              label={saved.name}
              onClick={() => onApply(saved.snapshot)}
              onDelete={() => persist(views.filter(view => view.id !== saved.id))}
            />
          </Tooltip>
        ))}
        <Button size="small" startIcon={<BookmarkAddIcon />} onClick={() => setDialogOpen(true)}>
          Сохранить вид
        </Button>
      </Stack>

      <Dialog open={dialogOpen} onClose={() => setDialogOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>Сохранить представление</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
            Сохранятся режим, период, фильтры, канал, сегменты и единица. Например: «Мои сети» или «Топ SKU».
          </Typography>
          <TextField
            autoFocus
            fullWidth
            size="small"
            label="Название"
            value={name}
            onChange={event => setName(event.target.value.slice(0, 50))}
            onKeyDown={event => { if (event.key === 'Enter') saveCurrent(); }}
            helperText={duplicate ? 'Представление с таким именем будет обновлено' : `${views.length} из ${MAX_SAVED_VIEWS}`}
          />
        </DialogContent>
        <DialogActions>
          <Button color="inherit" onClick={() => setDialogOpen(false)}>Отмена</Button>
          <Button variant="contained" disabled={!normalizedName} onClick={saveCurrent}>{duplicate ? 'Обновить' : 'Сохранить'}</Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
