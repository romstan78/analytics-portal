import { useMemo, useState } from 'react';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Box,
  Button,
  Paper,
  TextField,
  Typography,
} from '@mui/material';
import { ExpandMore as ExpandMoreIcon } from '@mui/icons-material';
import {
  EMPTY_CELL,
  formatRubShort,
  parseNumberInput,
  planKey,
} from '../utils/networkPlan';
import type { DraftCell } from '../utils/networkPlan';

const MONTHS = [
  'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
];

interface Props {
  year: number;
  quarter: number;
  brands: string[];
  draft: Record<string, DraftCell>;
  canEdit: boolean;
  onCellChange: (brand: string | null, patch: Partial<DraftCell>) => void;
}

const profileOf = (cell: DraftCell): [string, string, string] => [
  cell.month1Pct,
  cell.month2Pct,
  cell.month3Pct,
];

const profileSum = (profile: [string, string, string]): number =>
  profile.reduce((sum, value) => sum + (parseNumberInput(value) ?? 0), 0);

const monthClosed = (year: number, month: number): boolean => {
  const now = new Date();
  return year < now.getFullYear() || (year === now.getFullYear() && month < now.getMonth() + 1);
};

export default function NetworkAllocationEditor({
  year,
  quarter,
  brands,
  draft,
  canEdit,
  onCellChange,
}: Props) {
  const firstCell = draft[planKey(quarter, brands[0] ?? null)] ?? EMPTY_CELL;
  const [common, setCommon] = useState<[string, string, string]>(() => profileOf(firstCell));
  const monthNumbers = useMemo(() => {
    const start = (quarter - 1) * 3 + 1;
    return [start, start + 1, start + 2];
  }, [quarter]);
  const anyClosed = monthNumbers.some((month) => monthClosed(year, month));
  const pool = draft[planKey(quarter, null)] ?? EMPTY_CELL;
  const rows: Array<{ label: string; brand: string | null }> = [
    ...(pool.planRub.trim() !== '' ? [{ label: 'Валовый пул', brand: null }] : []),
    ...brands.map((brand) => ({ label: brand, brand })),
  ];

  const updateCommon = (index: number, value: string) => {
    setCommon((current) => current.map((item, currentIndex) => currentIndex === index ? value : item) as [string, string, string]);
  };

  const applyCommon = () => {
    if (profileSum(common) !== 100 || anyClosed) return;
    rows.forEach(({ brand }) => onCellChange(brand, {
      month1Pct: common[0],
      month2Pct: common[1],
      month3Pct: common[2],
    }));
  };

  return (
    <Accordion disableGutters variant="outlined" sx={{ borderRadius: 1, '&:before': { display: 'none' } }}>
      <AccordionSummary expandIcon={<ExpandMoreIcon />}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
          <Typography variant="subtitle2">Распределение плана по месяцам</Typography>
          <Typography variant="caption" color="text.secondary">
            {monthNumbers.map((month) => MONTHS[month - 1]).join(' · ')} · по умолчанию 30% / 30% / 40%
          </Typography>
        </Box>
      </AccordionSummary>
      <AccordionDetails sx={{ pt: 0 }}>
        <Paper variant="outlined" sx={{ p: 1.5, mb: 1.5 }}>
          <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', flexWrap: 'wrap' }}>
            <Typography variant="caption" color="text.secondary" sx={{ mr: 0.5 }}>
              Общий профиль
            </Typography>
            {common.map((value, index) => (
              <TextField
                key={monthNumbers[index]}
                size="small"
                label={`${MONTHS[monthNumbers[index] - 1]}, %`}
                value={value}
                disabled={!canEdit || anyClosed}
                error={profileSum(common) !== 100}
                onChange={(event) => updateCommon(index, event.target.value)}
                sx={{ width: 116 }}
                slotProps={{ htmlInput: { inputMode: 'decimal' } }}
              />
            ))}
            <Button
              size="small"
              disabled={!canEdit || anyClosed || profileSum(common) !== 100 || rows.length === 0}
              onClick={applyCommon}
            >
              Применить ко всем
            </Button>
          </Box>
          {anyClosed && (
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 1 }}>
              Закрытые месяцы заблокированы. Открытые проценты корректируются в строке бренда.
            </Typography>
          )}
        </Paper>

        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          {rows.map(({ label, brand }) => {
            const cell = draft[planKey(quarter, brand)] ?? EMPTY_CELL;
            const profile = profileOf(cell);
            const sum = profileSum(profile);
            const plan = parseNumberInput(cell.planRub);
            return (
              <Box
                key={brand ?? '__pool__'}
                sx={{
                  display: 'grid',
                  gridTemplateColumns: { xs: '1fr', md: 'minmax(150px, 1fr) repeat(3, 150px)' },
                  gap: 1,
                  alignItems: 'center',
                }}
              >
                <Typography variant="body2" noWrap title={label} sx={{ fontWeight: 600 }}>{label}</Typography>
                {profile.map((value, index) => {
                  const month = monthNumbers[index];
                  const amount = plan == null ? null : plan * (parseNumberInput(value) ?? 0) / 100;
                  const field = (['month1Pct', 'month2Pct', 'month3Pct'] as const)[index];
                  return (
                    <TextField
                      key={month}
                      size="small"
                      label={`${MONTHS[month - 1]}, %`}
                      value={value}
                      disabled={!canEdit || monthClosed(year, month)}
                      error={sum !== 100}
                      helperText={amount == null ? '—' : `${formatRubShort(amount)} ₽`}
                      onChange={(event) => onCellChange(brand, { [field]: event.target.value })}
                      slotProps={{ htmlInput: { inputMode: 'decimal' } }}
                    />
                  );
                })}
              </Box>
            );
          })}
        </Box>
        {rows.some(({ brand }) => profileSum(profileOf(draft[planKey(quarter, brand)] ?? EMPTY_CELL)) !== 100) && (
          <Alert severity="warning" sx={{ mt: 1.5 }}>В каждой строке сумма трёх месяцев должна быть ровно 100%.</Alert>
        )}
      </AccordionDetails>
    </Accordion>
  );
}
