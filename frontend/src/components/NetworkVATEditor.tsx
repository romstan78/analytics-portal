import { Box, InputAdornment, Paper, Switch, TextField, Typography } from '@mui/material';
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
  const invalid = values.some(({ vatRate }) => !isVATRateValid(vatRate));

  return (
    <Paper variant="outlined" sx={{ p: 1.5 }}>
      <Typography variant="subtitle2">НДС · {year}</Typography>
      <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.75 }}>
        Ставка задаётся на каждый квартал и применяется только к инвестициям.
      </Typography>

      <Box sx={{ display: 'grid' }}>
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
      </Box>

      {invalid && (
        <Typography variant="caption" color="error" sx={{ display: 'block', mt: 0.75 }}>
          Ставка НДС — число от 0 до 99,99. Пока это не так, профиль не сохранится.
        </Typography>
      )}
    </Paper>
  );
}
