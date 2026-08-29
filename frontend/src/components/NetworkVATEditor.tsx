import { useState } from 'react';
import {
  Box,
  Button,
  Chip,
  Collapse,
  InputAdornment,
  Paper,
  Switch,
  TextField,
  Typography,
} from '@mui/material';
import { ExpandLess as ExpandLessIcon, Tune as TuneIcon } from '@mui/icons-material';
import { isVATRateValid } from '../utils/networkPlan';

// Кварталы идут строками, а не карточками: значений всего два на квартал, и в
// сетке карточек подпись «Работает с НДС» повторялась четыре раза, занимая
// больше места, чем сами поля. Статус подписан рядом с переключателем, поэтому
// строка читается слева направо: квартал — режим — ставка.

export interface NetworkVATPeriod {
  quarter: number;
  vatIncluded: boolean;
  vatRate: string;
}

interface Props {
  year: number;
  values: NetworkVATPeriod[];
  canEdit: boolean;
  ready: boolean;
  onChange: (quarter: number, next: { vatIncluded: boolean; vatRate: string }) => void;
}

export default function NetworkVATEditor({ year, values, canEdit, ready, onChange }: Props) {
  const [expanded, setExpanded] = useState(false);
  const invalid = values.some(({ vatRate }) => !isVATRateValid(vatRate));
  const first = values[0];
  const sameForYear = values.every((value) => (
    value.vatIncluded === first?.vatIncluded && value.vatRate === first?.vatRate
  ));
  const summary = first == null
    ? 'Нет данных'
    : sameForYear
      ? first.vatIncluded ? `С НДС · ${first.vatRate}% весь год` : 'Без НДС весь год'
      : 'Есть отличия по кварталам';

  return (
    <Paper variant="outlined" sx={{ overflow: 'hidden' }}>
      <Box sx={{ p: 1.5, display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
        <Box sx={{ minWidth: 180, flex: 1 }}>
          <Typography variant="subtitle2">НДС · {year}</Typography>
          <Typography variant="body2" color={sameForYear ? 'text.secondary' : 'warning.main'}>
            {summary}
          </Typography>
        </Box>
        {!sameForYear && values.map(({ quarter, vatIncluded, vatRate }) => (
          <Chip
            key={quarter}
            size="small"
            variant="outlined"
            label={`Q${quarter} · ${vatIncluded ? `${vatRate}%` : 'без НДС'}`}
          />
        ))}
        <Button
          size="small"
          startIcon={expanded ? <ExpandLessIcon /> : <TuneIcon />}
          onClick={() => setExpanded((value) => !value)}
        >
          {expanded ? 'Свернуть' : canEdit ? 'Настроить' : 'Подробнее'}
        </Button>
      </Box>

      <Collapse in={expanded}>
        <Box sx={{ px: 1.5, pb: 1.5 }}>
          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.75 }}>
            Ставка задаётся на каждый квартал и применяется только к инвестициям.
          </Typography>
          {values.map(({ quarter, vatIncluded, vatRate }, index) => (
          <Box
            key={quarter}
            sx={{
              display: 'grid',
              gridTemplateColumns: '1.75rem minmax(0, 1fr) 7rem',
              alignItems: 'center',
              columnGap: 1,
              py: 0.5,
              borderTop: index === 0 ? 'none' : '1px solid',
              borderColor: 'divider',
            }}
          >
            <Typography variant="body2" sx={{ fontWeight: 600, color: 'text.secondary' }}>
              Q{quarter}
            </Typography>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, minWidth: 0 }}>
              <Switch
                size="small"
                checked={vatIncluded}
                disabled={!canEdit || !ready}
                onChange={(event) => onChange(quarter, {
                  vatIncluded: event.target.checked,
                  vatRate,
                })}
                slotProps={{ input: { 'aria-label': `Сеть работает с НДС в Q${quarter}` } }}
              />
              <Typography variant="body2" noWrap color={vatIncluded ? 'text.primary' : 'text.disabled'}>
                {vatIncluded ? 'с НДС' : 'без НДС'}
              </Typography>
            </Box>
            <TextField
              size="small"
              value={vatRate}
              disabled={!canEdit || !ready || !vatIncluded}
              error={!isVATRateValid(vatRate)}
              onChange={(event) => onChange(quarter, {
                vatIncluded,
                vatRate: event.target.value,
              })}
              slotProps={{
                input: { endAdornment: <InputAdornment position="end">%</InputAdornment> },
                htmlInput: { inputMode: 'decimal', 'aria-label': `Ставка НДС в Q${quarter}` },
              }}
            />
          </Box>
          ))}

          {invalid && (
            <Typography variant="caption" color="error" sx={{ display: 'block', mt: 0.75 }}>
              Ставка НДС — число от 0 до 99,99. Пока это не так, профиль не сохранится.
            </Typography>
          )}
        </Box>
      </Collapse>
    </Paper>
  );
}
