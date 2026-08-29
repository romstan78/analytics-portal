import { useState } from 'react';
import { Alert, Box, Button, Collapse, Paper, TextField, Typography } from '@mui/material';
import { ExpandLess as ExpandLessIcon, Tune as TuneIcon } from '@mui/icons-material';
import { isMonthDistributionValid, parseNumberInput } from '../utils/networkPlan';

interface Props {
  values: [string, string, string];
  canEdit: boolean;
  onChange: (index: 0 | 1 | 2, value: string) => void;
}

const MONTH_LABELS = ['1-й месяц квартала, %', '2-й месяц квартала, %', '3-й месяц квартала, %'];

export default function NetworkAllocationEditor({ values, canEdit, onChange }: Props) {
  const [expanded, setExpanded] = useState(false);
  const total = values.reduce((sum, value) => sum + (parseNumberInput(value) ?? 0), 0);
  const valid = isMonthDistributionValid(values);

  return (
    <Paper variant="outlined" sx={{ overflow: 'hidden' }}>
      <Box sx={{ p: 1.5, display: 'flex', alignItems: 'center', gap: 1.5 }}>
        <Box sx={{ flex: 1 }}>
          <Typography variant="subtitle2">Распределение плана по месяцам</Typography>
          <Typography variant="body2" color={valid ? 'text.secondary' : 'error'}>
            {values.join(' / ')} · итого {total.toLocaleString('ru-RU', { maximumFractionDigits: 2 })}%
          </Typography>
        </Box>
        <Button
          size="small"
          startIcon={expanded ? <ExpandLessIcon /> : <TuneIcon />}
          onClick={() => setExpanded((value) => !value)}
        >
          {expanded ? 'Свернуть' : canEdit ? 'Изменить' : 'Подробнее'}
        </Button>
      </Box>
      <Collapse in={expanded}>
        <Box sx={{ px: 1.5, pb: 1.5 }}>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
            Единая схема для всех брендов и кварталов сети.
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
          {!valid && <Alert severity="warning" sx={{ mt: 1 }}>Сумма долей должна быть равна 100%.</Alert>}
        </Box>
      </Collapse>
    </Paper>
  );
}
