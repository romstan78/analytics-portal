// Совместный зачёт плана и инвестиций по нескольким смежным кварталам.
// Сами квартальные суммы не меняются: компонент редактирует только правила,
// а объединённые итоги получает из backend preview.

import { useMemo, useState } from 'react';
import {
  Box,
  Button,
  IconButton,
  MenuItem,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import {
  Add as AddIcon,
  DeleteOutlined as DeleteIcon,
  Link as LinkIcon,
} from '@mui/icons-material';
import type { NetworkPeriodGroupInput, NetworkPeriodGroupTotals } from '../types/network';
import {
  formatRubShort,
  periodGroupConflict,
  periodGroupKey,
} from '../utils/networkPlan';

interface NetworkPeriodGroupsEditorProps {
  year: number;
  brands: string[];
  groups: NetworkPeriodGroupInput[];
  totals: NetworkPeriodGroupTotals[];
  canEdit: boolean;
  onChange: (groups: NetworkPeriodGroupInput[]) => void;
}

const scopeLabel = (brand: string | null) => brand ?? 'Весь портфель';

export default function NetworkPeriodGroupsEditor({
  year,
  brands,
  groups,
  totals,
  canEdit,
  onChange,
}: NetworkPeriodGroupsEditorProps) {
  const [startQuarter, setStartQuarter] = useState(1);
  const [endQuarter, setEndQuarter] = useState(2);
  const [scope, setScope] = useState('*');
  const [error, setError] = useState<string | null>(null);
  const selectedScope = scope === '*' || brands.includes(scope) ? scope : '*';

  const totalsByKey = useMemo(() => new Map(
    totals.map((row) => [periodGroupKey(row), row]),
  ), [totals]);

  const addGroup = () => {
    const candidate: NetworkPeriodGroupInput = {
      start_quarter: startQuarter,
      end_quarter: endQuarter,
      brand_as: selectedScope === '*' ? null : selectedScope,
      updated_at: '',
    };
    const conflict = periodGroupConflict(groups, candidate);
    if (conflict) {
      setError(conflict);
      return;
    }
    onChange([...groups, candidate].sort((a, b) => (
      a.start_quarter - b.start_quarter
      || a.end_quarter - b.end_quarter
      || scopeLabel(a.brand_as).localeCompare(scopeLabel(b.brand_as), 'ru')
    )));
    setError(null);
  };

  const removeGroup = (target: NetworkPeriodGroupInput) => {
    const key = periodGroupKey(target);
    onChange(groups.filter((group) => periodGroupKey(group) !== key));
    setError(null);
  };

  return (
    <Paper variant="outlined" sx={{ p: 1.5 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: groups.length || canEdit ? 1.25 : 0 }}>
        <LinkIcon fontSize="small" color="action" />
        <Box sx={{ minWidth: 0 }}>
          <Typography variant="subtitle2">Объединение кварталов</Typography>
          <Typography variant="caption" color="text.secondary">
            Совместный зачёт плана и инвестиций за {year} год; квартальные суммы сохраняются отдельно.
          </Typography>
        </Box>
      </Box>

      {canEdit && (
        <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1, flexWrap: 'wrap', mb: 1.25 }}>
          <TextField
            select
            size="small"
            label="С квартала"
            value={startQuarter}
            onChange={(event) => {
              const next = Number(event.target.value);
              setStartQuarter(next);
              if (endQuarter <= next) setEndQuarter(next + 1);
              setError(null);
            }}
            sx={{ width: 116 }}
          >
            {[1, 2, 3].map((quarter) => <MenuItem key={quarter} value={quarter}>Q{quarter}</MenuItem>)}
          </TextField>
          <TextField
            select
            size="small"
            label="По квартал"
            value={endQuarter}
            onChange={(event) => { setEndQuarter(Number(event.target.value)); setError(null); }}
            sx={{ width: 116 }}
          >
            {[2, 3, 4].filter((quarter) => quarter > startQuarter).map((quarter) => (
              <MenuItem key={quarter} value={quarter}>Q{quarter}</MenuItem>
            ))}
          </TextField>
          <TextField
            select
            size="small"
            label="Область"
            value={selectedScope}
            onChange={(event) => { setScope(event.target.value); setError(null); }}
            sx={{ minWidth: 210 }}
          >
            <MenuItem value="*">Весь портфель</MenuItem>
            {brands.map((brand) => <MenuItem key={brand} value={brand}>{brand}</MenuItem>)}
          </TextField>
          <Button size="small" startIcon={<AddIcon />} onClick={addGroup} sx={{ mt: 0.5 }}>
            Объединить
          </Button>
          {error && (
            <Typography variant="caption" color="error" sx={{ alignSelf: 'center', flexBasis: '100%' }}>
              {error}
            </Typography>
          )}
        </Box>
      )}

      {groups.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          Кварталы оцениваются отдельно. Добавьте правило, если условия сети допускают общий зачёт.
        </Typography>
      ) : (
        <Box sx={{ overflowX: 'auto' }}>
          <Table size="small" sx={{ minWidth: 760 }}>
            <TableHead>
              <TableRow>
                <TableCell>Общий период</TableCell>
                <TableCell>Область</TableCell>
                <TableCell align="right">План</TableCell>
				<TableCell align="right">EAC</TableCell>
				<TableCell align="right">Выполнение</TableCell>
				<TableCell align="right">К выплате</TableCell>
                <TableCell padding="none" />
              </TableRow>
            </TableHead>
            <TableBody>
              {groups.map((group) => {
                const combined = totalsByKey.get(periodGroupKey(group));
                return (
                  <TableRow key={periodGroupKey(group)} hover>
                    <TableCell sx={{ fontWeight: 600 }}>Q{group.start_quarter}–Q{group.end_quarter}</TableCell>
                    <TableCell>{scopeLabel(group.brand_as)}</TableCell>
                    <TableCell align="right">{formatRubShort(combined?.plan_rub)}</TableCell>
					<TableCell align="right">{formatRubShort(combined?.eac_rub)}</TableCell>
					<TableCell align="right">
						<Typography variant="body2" color={combined?.completed ? 'success.main' : 'warning.main'}>
							{combined?.completion_pct == null
								? '—'
								: `${combined.completion_pct.toLocaleString('ru-RU', { maximumFractionDigits: 1 })} %`}
						</Typography>
					</TableCell>
					<TableCell align="right">{formatRubShort(combined?.payable_investments_rub)}</TableCell>
                    <TableCell padding="none">
                      {canEdit && (
                        <Tooltip title="Удалить объединение">
                          <IconButton size="small" onClick={() => removeGroup(group)}>
                            <DeleteIcon fontSize="inherit" />
                          </IconButton>
                        </Tooltip>
                      )}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </Box>
      )}
    </Paper>
  );
}
