import { Alert, Box, Paper, TextField, Typography } from '@mui/material';
import { isMonthDistributionValid, parseNumberInput } from '../utils/networkPlan';

interface Props {
  values: [string, string, string];
  canEdit: boolean;
  onChange: (index: 0 | 1 | 2, value: string) => void;
}

const MONTH_LABELS = ['1-й месяц квартала, %', '2-й месяц квартала, %', '3-й месяц квартала, %'];

export default function NetworkAllocationEditor({ values, canEdit, onChange }: Props) {
  const total = values.reduce((sum, value) => sum + (parseNumberInput(value) ?? 0), 0);
  const valid = isMonthDistributionValid(values);

  return (
    <Paper variant="outlined" sx={{ p: 2 }}>
      <Typography variant="subtitle2">Распределение плана по месяцам</Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5, mb: 1.5 }}>
        Единая схема для всех брендов и кварталов сети. По умолчанию — 30 / 30 / 40.
      </Typography>
      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: 'repeat(3, 1fr)' }, gap: 1.5 }}>
        {values.map((value, index) => (
          <TextField
            key={MONTH_LABELS[index]}
            label={MONTH_LABELS[index]}
            value={value}
            type="number"
            disabled={!canEdit}
            error={!valid}
            onChange={(event) => onChange(index as 0 | 1 | 2, event.target.value)}
            slotProps={{ htmlInput: { min: 0, max: 100, step: 0.01 } }}
          />
        ))}
      </Box>
      <Typography variant="caption" color={valid ? 'text.secondary' : 'error'} sx={{ display: 'block', mt: 1 }}>
        Итого: {total.toLocaleString('ru-RU', { maximumFractionDigits: 2 })}%
      </Typography>
      {!valid && <Alert severity="warning" sx={{ mt: 1 }}>Сумма долей должна быть равна 100%.</Alert>}
    </Paper>
  );
}
