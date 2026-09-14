// Ступени контракта в профиле сети: сколько порогов у плана по умолчанию,
// поквартальные исключения и крышка новых строк.
//
// Пороги, проценты и крышки самих строк вводятся во вкладке «План и факт»;
// здесь только рамка — сколько ступеней разрешено завести в квартале.

import { useState } from 'react';
import {
  Box,
  Button,
  Chip,
  Collapse,
  Paper,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@mui/material';
import { ExpandLess as ExpandLessIcon, StairsOutlined as StairsIcon } from '@mui/icons-material';
import { CAP_MODES, MAX_SCALES, capModeLabel, pluralRu } from '../utils/networkPlan';
import type { CapMode } from '../utils/networkPlan';

export interface NetworkScalesQuarter {
  quarter: number;
  count: number;
}

interface Props {
  year: number;
  defaultCount: number;
  defaultCapMode: CapMode;
  quarters: NetworkScalesQuarter[];
  canEdit: boolean;
  ready: boolean;
  onDefaultCountChange: (count: number) => void;
  onDefaultCapModeChange: (mode: CapMode) => void;
  onQuarterChange: (quarter: number, count: number) => void;
}

const COUNTS = Array.from({ length: MAX_SCALES }, (_, i) => i + 1);

export default function NetworkScalesEditor({
  year, defaultCount, defaultCapMode, quarters, canEdit, ready,
  onDefaultCountChange, onDefaultCapModeChange, onQuarterChange,
}: Props) {
  const [expanded, setExpanded] = useState(false);
  const exceptions = quarters.filter((q) => q.count !== defaultCount);
  const summary = defaultCount === 1 && exceptions.length === 0
    ? 'Одна ступень: план и процент строки.'
    : `${defaultCount} ${pluralRu(defaultCount, 'ступень', 'ступени', 'ступеней')} по умолчанию`
      + (exceptions.length > 0 ? `, ${exceptions.length} ${pluralRu(exceptions.length, 'исключение', 'исключения', 'исключений')} по кварталам` : '')
      + ` · ${capModeLabel(defaultCapMode)} для новых строк`;

  return (
    <Paper variant="outlined" sx={{ overflow: 'hidden' }}>
      <Box sx={{ p: 1.5, display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
        <StairsIcon fontSize="small" color="action" />
        <Box sx={{ minWidth: 180, flex: 1 }}>
          <Typography variant="subtitle2">Ступени контракта · {year}</Typography>
          <Typography variant="body2" color={exceptions.length > 0 ? 'warning.main' : 'text.secondary'}>
            {summary}
          </Typography>
        </Box>
        {!expanded && exceptions.map((q) => (
          <Chip key={q.quarter} size="small" variant="outlined" label={`Q${q.quarter} · ${q.count}`} />
        ))}
        <Button
          size="small"
          startIcon={expanded ? <ExpandLessIcon /> : <StairsIcon />}
          onClick={() => setExpanded((value) => !value)}
        >
          {expanded ? 'Свернуть' : canEdit ? 'Настроить' : 'Подробнее'}
        </Button>
      </Box>

      <Collapse in={expanded}>
        <Box sx={{ px: 1.5, pb: 1.5, display: 'grid', gap: 1.25 }}>
          <Typography variant="caption" color="text.secondary">
            Число ступеней ограничивает, сколько порогов можно завести в квартале; пороги, проценты
            и крышки вводятся во вкладке «План и факт». Бренд может иметь меньше ступеней, чем квартал.
          </Typography>

          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
            <Typography variant="body2" sx={{ minWidth: 160 }}>По умолчанию</Typography>
            <ToggleButtonGroup
              size="small"
              exclusive
              value={defaultCount}
              disabled={!canEdit}
              onChange={(_, value: number | null) => value != null && onDefaultCountChange(value)}
              sx={{ '& .MuiToggleButton-root': { textTransform: 'none', px: 1.5 } }}
            >
              {COUNTS.map((count) => (
                <ToggleButton key={count} value={count}>{count === 1 ? '1 ступень' : count}</ToggleButton>
              ))}
            </ToggleButtonGroup>
          </Box>

          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
            <Typography variant="body2" sx={{ minWidth: 160 }}>По кварталам</Typography>
            {quarters.map((q) => (
              <Box key={q.quarter} sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                <Typography variant="caption" color="text.secondary">Q{q.quarter}</Typography>
                <ToggleButtonGroup
                  size="small"
                  exclusive
                  value={q.count}
                  disabled={!canEdit || !ready}
                  onChange={(_, value: number | null) => value != null && onQuarterChange(q.quarter, value)}
                  sx={{ '& .MuiToggleButton-root': { textTransform: 'none', px: 1, py: 0.25, fontSize: 12 } }}
                >
                  {COUNTS.map((count) => <ToggleButton key={count} value={count}>{count}</ToggleButton>)}
                </ToggleButtonGroup>
              </Box>
            ))}
          </Box>

          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
            <Typography variant="body2" sx={{ minWidth: 160 }}>Крышка для новых строк</Typography>
            <ToggleButtonGroup
              size="small"
              exclusive
              value={defaultCapMode}
              disabled={!canEdit}
              onChange={(_, value: CapMode | null) => value && onDefaultCapModeChange(value)}
              sx={{ '& .MuiToggleButton-root': { textTransform: 'none', px: 1.5 } }}
            >
              {CAP_MODES.map((mode) => <ToggleButton key={mode.value} value={mode.value}>{mode.label}</ToggleButton>)}
            </ToggleButtonGroup>
            <Typography variant="caption" color="text.secondary">
              подставляется новым строкам плана; у каждой строки меняется отдельно
            </Typography>
          </Box>
        </Box>
      </Collapse>
    </Paper>
  );
}
