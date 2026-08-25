import { useState } from 'react';
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  MenuItem,
  Switch,
  TextField,
  Typography,
} from '@mui/material';
import type { NetworkType } from '../types/network';

const YEARS = [2026, 2027, 2028];
const DEFAULT_YEAR = 2027;

export interface NewNetworkValues {
  name: string;
  kam: string;
  network_type: NetworkType;
  vat_included: boolean;
  vat_rate: number;
  year: number;
}

interface NewNetworkDialogProps {
  open: boolean;
  saving: boolean;
  error?: string | null;
  onClose: () => void;
  onSubmit: (values: NewNetworkValues) => void;
}

// Заведение сети: тип, контракт и первый год открываются сразу,
// чтобы КАМ попал в готовую сетку планов, а не в пустой экран.
export default function NewNetworkDialog({ open, saving, error, onClose, onSubmit }: NewNetworkDialogProps) {
  const [values, setValues] = useState<NewNetworkValues>({
    name: '',
    kam: '',
    network_type: 'regular',
    vat_included: true,
    vat_rate: 20,
    year: DEFAULT_YEAR,
  });

  const set = <K extends keyof NewNetworkValues>(key: K, value: NewNetworkValues[K]) =>
    setValues((prev) => ({ ...prev, [key]: value }));

  const nameError = values.name.trim() === '';

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Новая сеть</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
        {error && <Alert severity="error">{error}</Alert>}

        <TextField
          label="Название сети"
          value={values.name}
          onChange={(e) => set('name', e.target.value)}
          error={nameError}
          helperText={nameError ? 'Название обязательно' : ' '}
          autoFocus
        />
        <TextField label="КАМ" value={values.kam} onChange={(e) => set('kam', e.target.value)} />

        <TextField
          select
          label="Тип сети"
          value={values.network_type}
          onChange={(e) => set('network_type', e.target.value as NetworkType)}
          helperText="У складской сети свой процесс прогнозирования объёмов"
        >
          <MenuItem value="regular">Обычная</MenuItem>
          <MenuItem value="warehouse">Складская</MenuItem>
        </TextField>

        <FormControlLabel
          control={<Switch checked={values.vat_included} onChange={(e) => set('vat_included', e.target.checked)} />}
          label="Кварталы первого года работают с НДС"
        />
        <Typography variant="caption" color="text.secondary" sx={{ mt: -1.5 }}>
          НДС применяется только к инвестициям: помимо суммы до вычета показывается сумма с вычетом ставки.
        </Typography>
        {values.vat_included && (
          <TextField
            label="Ставка НДС, %"
            value={values.vat_rate}
            onChange={(e) => set('vat_rate', Number(e.target.value.replace(',', '.')) || 0)}
          />
        )}

        <TextField
          select
          label="Открыть год"
          value={values.year}
          onChange={(e) => set('year', Number(e.target.value))}
        >
          {YEARS.map((year) => <MenuItem key={year} value={year}>{year}</MenuItem>)}
        </TextField>
        <Typography variant="caption" color="text.secondary">
          Начальная настройка НДС применяется к Q1–Q4 выбранного года. После создания каждый
          квартал можно изменить отдельно в «Профиле сети». Валовый объём
          отмечается на брендах во вкладке «План и факт»: часть брендов может входить
          в общий объём контракта, часть планироваться отдельно.
        </Typography>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Отмена</Button>
        <Button
          variant="contained"
          disabled={saving || nameError}
          onClick={() => onSubmit({ ...values, name: values.name.trim() })}
        >
          Создать
        </Button>
      </DialogActions>
    </Dialog>
  );
}
