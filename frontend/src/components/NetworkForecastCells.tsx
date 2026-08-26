// Ячейки сетки прогноза: подпись режима ведения и поле ввода месяца.
//
// Расчётов здесь нет. Что считается введённым, а что выводится, решает backend
// (backend/services/network_forecast_service.go): строка приходит с признаком
// is_derived, и форма только показывает разницу — расчётное значение видно,
// но не редактируется.

import { useState } from 'react';
import {
  Box,
  Chip,
  Menu,
  MenuItem,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import type { NetworkEntryUnit, NetworkForecastMonth } from '../types/network';
import { formatRub } from '../utils/networkPlan';
import type { EntryMode } from '../utils/networkForecastView';
import { MODE_OPTIONS, MONTHS, amountLabel, modeLabel } from '../utils/networkForecastView';

// Подпись режима: она же переключатель. Режим — свойство бренда, а не формы,
// поэтому и живёт в его строке.
export function EntryModeChip({ mode, disabled, onChange }: {
  mode: EntryMode;
  disabled: boolean;
  onChange: (next: EntryMode) => void;
}) {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);

  return (
    <>
      <Tooltip title={disabled ? 'Сначала сохраните изменения' : 'Как ведут этот бренд'}>
        <Chip
          size="small"
          variant="outlined"
          color={mode.level === 'sku' ? 'success' : 'primary'}
          label={modeLabel(mode)}
          onClick={disabled ? undefined : (event) => setAnchor(event.currentTarget)}
          sx={{ height: 20, '& .MuiChip-label': { px: 0.75, fontSize: 11 } }}
        />
      </Tooltip>
      <Menu anchorEl={anchor} open={anchor != null} onClose={() => setAnchor(null)}>
        {MODE_OPTIONS.map((option) => (
          <MenuItem
            key={`${option.level}-${option.unit}`}
            selected={option.level === mode.level && option.unit === mode.unit}
            onClick={() => {
              setAnchor(null);
              if (option.level !== mode.level || option.unit !== mode.unit) {
                onChange({ level: option.level, unit: option.unit });
              }
            }}
          >
            <Box>
              <Typography variant="body2">{option.label}</Typography>
              <Typography variant="caption" color="text.secondary">{option.hint}</Typography>
            </Box>
          </MenuItem>
        ))}
      </Menu>
    </>
  );
}

// Ячейка месяца: план и факт одной подписью, под ней — введённое значение
// либо расчёт. Закрытый месяц и расчётная строка показывают величину текстом:
// править нечего, и поле ввода только занимало бы место.
export function MonthCell({ row, value, unit, canEdit, showPlan, onChange }: {
  row: NetworkForecastMonth;
  value: string;
  unit: NetworkEntryUnit;
  canEdit: boolean;
  showPlan: boolean;
  onChange: (next: string) => void;
}) {
  const fact = unit === 'units' ? row.fact_units : row.fact_rub;
  const eac = unit === 'units' ? row.eac_units : row.eac_rub;
  const system = unit === 'units' ? row.system_forecast_units : row.system_forecast_rub;
  const editable = canEdit && !row.is_closed && !row.is_derived;

  const caption = [
    showPlan && unit === 'rub' ? `план ${amountLabel(row.plan_rub, unit)}` : null,
    `факт ${amountLabel(fact, unit)}`,
  ].filter(Boolean).join(' · ');

  return (
    <Box sx={{ minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }} noWrap>
        {caption}
      </Typography>
      {editable ? (
        <TextField
          size="small"
          fullWidth
          value={value}
          placeholder={system == null ? 'Прогноз' : formatRub(system)}
          onChange={(event) => onChange(event.target.value)}
          sx={{ mt: 0.25, '& .MuiInputBase-input': { py: 0.75 } }}
          slotProps={{
            htmlInput: {
              inputMode: 'decimal',
              'aria-label': `Прогноз ${row.brand_as}${row.sku ? ` ${row.sku}` : ''} ${MONTHS[row.month - 1]}`,
            },
          }}
        />
      ) : (
        <Tooltip title={row.is_closed ? 'Месяц закрыт: показан факт' : 'Расчётное значение'}>
          <Typography variant="body2" sx={{ mt: 0.25, py: 0.75, fontWeight: 500 }} noWrap>
            {amountLabel(eac, unit, false)}
          </Typography>
        </Tooltip>
      )}
    </Box>
  );
}
