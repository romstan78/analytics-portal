// Общие ячейки сетки планов: поле ввода суммы и показ рассчитанного значения.
// Вынесены отдельно, потому что квартальная и годовая таблицы показывают
// одни и те же величины в разных разрезах.

import { Box, TextField, Tooltip, Typography } from '@mui/material';
import { formatNumberInput, formatRub, formatRubShort } from '../utils/networkPlan';
import { TONE_COLOR } from '../utils/networkPlanView';
import type { HintTone } from '../utils/networkPlanView';

interface PlanNumberFieldProps {
  value: string;
  disabled?: boolean;
  placeholder?: string;
  suffix?: string;
  onChange: (value: string) => void;
}

// Ввод суммы: справа, разряды расставляются по уходу из поля,
// чтобы «1200000» не приходилось перечитывать по цифрам.
export function PlanNumberField({ value, disabled, placeholder, suffix, onChange }: PlanNumberFieldProps) {
  return (
    <TextField
      size="small"
      fullWidth
      variant="outlined"
      value={value}
      disabled={disabled}
      placeholder={placeholder ?? '—'}
      onChange={(e) => onChange(e.target.value)}
      onBlur={(e) => {
        const formatted = formatNumberInput(e.target.value);
        if (formatted !== e.target.value) onChange(formatted);
      }}
      slotProps={{
        htmlInput: { inputMode: 'decimal', style: { textAlign: 'right', padding: '6px 8px' } },
        input: suffix
          ? { endAdornment: <Typography variant="caption" color="text.secondary">{suffix}</Typography> }
          : undefined,
      }}
    />
  );
}

interface ValueCellProps {
  value: number | null | undefined;
  hint?: string | null;
  // Сумма с вычетом НДС: показывается в подсказке, чтобы не занимать
  // вторую строку ячейки — в плотной таблице она нужна под отклонение.
  netValue?: number | null;
  tone?: HintTone;
  bold?: boolean;
  muted?: boolean;
}

// Показ суммы: коротко в ячейке, полностью — в подсказке.
// Вторая строка отдана отклонению, чтобы не заводить под него колонку.
export function ValueCell({ value, hint, netValue, tone = 'neutral', bold, muted }: ValueCellProps) {
  const main = (
    <Typography
      variant="body2"
      sx={{
        fontWeight: bold ? 600 : 400,
        color: muted ? 'text.disabled' : 'text.primary',
        fontVariantNumeric: 'tabular-nums',
        lineHeight: 1.3,
      }}
    >
      {formatRubShort(value)}
    </Typography>
  );

  const tooltip = value == null
    ? ''
    : netValue == null
      ? `${formatRub(value, 2)} ₽`
      : `${formatRub(value, 2)} ₽ · без НДС ${formatRub(netValue, 2)} ₽`;

  return (
    <Box>
      {value == null ? main : <Tooltip title={tooltip} placement="top">{main}</Tooltip>}
      {hint && (
        <Typography variant="caption" sx={{ color: TONE_COLOR[tone], display: 'block', lineHeight: 1.3 }}>
          {hint}
        </Typography>
      )}
    </Box>
  );
}

