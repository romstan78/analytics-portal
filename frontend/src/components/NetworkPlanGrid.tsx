// Вкладка «Планы» реестра сетей.
//
// Форма используется и для планирования, и для регулярного просмотра, поэтому
// разрез выбирается периодом: «Год» показывает одну величину по четырём
// кварталам (так вносят план), квартал — план, факт, прогноз и инвестиции
// рядом (так сверяют выполнение). Это же держит таблицу в одной ширине листа:
// раньше все четыре квартала стояли в одной строке и уезжали за экран.

import { useMemo, useState } from 'react';
import {
  Autocomplete,
  Box,
  Button,
  Paper,
  Switch,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@mui/material';
import { Save as SaveIcon } from '@mui/icons-material';
import type { NetworkPlanResponse, NetworkPlanSaveRequest, NetworkPlanInput } from '../types/network';
import {
  EMPTY_CELL,
  QUARTERS,
  brandsFromPlans,
  buildDraft,
  buildSettings,
  calcQuarterTotals,
  parseNumberInput,
  planKey,
  round2,
  sumYearTotals,
} from '../utils/networkPlan';
import type { DraftCell, QuarterSettings } from '../utils/networkPlan';
import NetworkPlanSummary from './NetworkPlanSummary';
import NetworkQuarterTable from './NetworkQuarterTable';
import NetworkYearTable from './NetworkYearTable';
import { YEAR_METRICS } from '../utils/networkPlanView';
import type { YearMetric } from '../utils/networkPlanView';

const DEFAULT_SETTINGS: QuarterSettings = { vat_included: true, vat_rate: 20 };

type Period = 'year' | 1 | 2 | 3 | 4;

interface NetworkPlanGridProps {
  data: NetworkPlanResponse;
  brandOptions: string[];
  canEdit: boolean;
  saving: boolean;
  onSave: (request: NetworkPlanSaveRequest) => void;
  onCommentCell: (quarter: number, brand: string | null) => void;
  commentedCells: Set<string>;
}

export default function NetworkPlanGrid({
  data,
  brandOptions,
  canEdit,
  saving,
  onSave,
  onCommentCell,
  commentedCells,
}: NetworkPlanGridProps) {
  const [draft, setDraft] = useState<Record<string, DraftCell>>(() => buildDraft(data.plans));
  const [settings, setSettings] = useState<Record<number, QuarterSettings>>(() => buildSettings(data.periods, DEFAULT_SETTINGS));
  const [brands, setBrands] = useState<string[]>(() => brandsFromPlans(data.plans));
  const [dirty, setDirty] = useState(false);
  const [period, setPeriod] = useState<Period>('year');
  const [yearMetric, setYearMetric] = useState<YearMetric>('plan');

  // Пришли данные другого года или сети — черновик начинается заново.
  // Сброс во время рендера, а не в эффекте: иначе кадр показывает чужие цифры.
  const [loadedData, setLoadedData] = useState(data);
  if (loadedData !== data) {
    setLoadedData(data);
    setDraft(buildDraft(data.plans));
    setSettings(buildSettings(data.periods, DEFAULT_SETTINGS));
    setBrands(brandsFromPlans(data.plans));
    setDirty(false);
  }

  const totals = useMemo(() => calcQuarterTotals(draft, brands, settings), [draft, brands, settings]);
  const periodTotals = period === 'year' ? sumYearTotals(totals) : totals[period - 1];
  const periodLabel = period === 'year' ? `${data.year}` : `Q${period} ${data.year}`;

  const setCell = (quarter: number, brand: string | null, patch: Partial<DraftCell>) => {
    setDraft((prev) => {
      const key = planKey(quarter, brand);
      return { ...prev, [key]: { ...(prev[key] ?? EMPTY_CELL), ...patch } };
    });
    setDirty(true);
  };

  const setQuarterSetting = (quarter: number, patch: Partial<QuarterSettings>) => {
    setSettings((prev) => ({ ...prev, [quarter]: { ...(prev[quarter] ?? DEFAULT_SETTINGS), ...patch } }));
    setDirty(true);
  };

  // НДС обычно одинаков весь год — отдельная кнопка избавляет от четырёх правок.
  const applyVatToAllQuarters = (from: number) => {
    const source = settings[from] ?? DEFAULT_SETTINGS;
    setSettings(() => Object.fromEntries(QUARTERS.map((q) => [q, { ...source }])));
    setDirty(true);
  };

  // Признак валового объёма живёт на строке бренд+квартал.
  const toggleGross = (brand: string, next: boolean, allQuarters: boolean, quarter?: number) => {
    const target = allQuarters ? [...QUARTERS] : [quarter ?? (period === 'year' ? 1 : period)];
    setDraft((prev) => {
      const updated = { ...prev };
      target.forEach((q) => {
        const key = planKey(q, brand);
        updated[key] = { ...(updated[key] ?? EMPTY_CELL), inGross: next };
      });
      return updated;
    });
    setDirty(true);
  };

  const addBrand = (brand: string | null) => {
    if (!brand || brands.includes(brand)) return;
    setBrands((prev) => [...prev, brand].sort((a, b) => a.localeCompare(b, 'ru')));
    setDirty(true);
  };

  // Строку убираем из таблицы и обнуляем значения — сохранение снимет их и в БД.
  const removeBrand = (brand: string) => {
    setBrands((prev) => prev.filter((b) => b !== brand));
    setDraft((prev) => {
      const next = { ...prev };
      QUARTERS.forEach((quarter) => {
        const key = planKey(quarter, brand);
        if (next[key]) next[key] = { ...EMPTY_CELL, factRub: next[key].factRub };
      });
      return next;
    });
    setDirty(true);
  };

  // Остаток валового объёма делим поровну между брендами, которые в него входят.
  const distributeRest = (quarter: number) => {
    const total = totals[quarter - 1];
    const grossBrands = brands.filter((b) => draft[planKey(quarter, b)]?.inGross);
    if (!total.undistributed || grossBrands.length === 0) return;
    const share = round2(total.undistributed / grossBrands.length);
    setDraft((prev) => {
      const next = { ...prev };
      grossBrands.forEach((brand) => {
        const key = planKey(quarter, brand);
        const current = next[key] ?? EMPTY_CELL;
        const value = parseNumberInput(current.planRub) ?? 0;
        next[key] = { ...current, planRub: String(round2(value + share)) };
      });
      return next;
    });
    setDirty(true);
  };

  const handleSave = () => {
    const versions = new Map(data.plans.map((p) => [planKey(p.quarter, p.brand_as), p.updated_at]));
    const rows: NetworkPlanInput[] = [];

    QUARTERS.forEach((quarter) => {
      // Строку пула отправляем всегда: пустая новая не создастся, а существующая
      // очистится, если валовый объём в квартале отменили.
      const rowBrands: Array<string | null> = [null, ...brands];
      // Бренд убрали из таблицы, но строка есть в БД — отправляем пустые значения.
      data.plans.forEach((plan) => {
        if (plan.quarter === quarter && plan.brand_as && !brands.includes(plan.brand_as)) {
          rowBrands.push(plan.brand_as);
        }
      });

      rowBrands.forEach((brand) => {
        const cell = draft[planKey(quarter, brand)] ?? EMPTY_CELL;
        rows.push({
          quarter,
          brand_as: brand,
          in_gross: brand !== null && cell.inGross,
          plan_rub: parseNumberInput(cell.planRub),
          forecast_rub: parseNumberInput(cell.forecastRub),
          investments_pct: brand === null ? null : parseNumberInput(cell.investmentsPct),
          updated_at: versions.get(planKey(quarter, brand)) ?? '',
        });
      });
    });

    onSave({
      year: data.year,
      periods: QUARTERS.map((quarter) => {
        const setting = settings[quarter] ?? DEFAULT_SETTINGS;
        return { quarter, vat_included: setting.vat_included, vat_rate: setting.vat_rate };
      }),
      plans: rows,
    });
  };

  const availableBrands = brandOptions.filter((b) => !brands.includes(b));
  const vatQuarters: number[] = period === 'year' ? [...QUARTERS] : [period];

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      {/* Период, показатель и сохранение */}
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
        <ToggleButtonGroup
          size="small"
          exclusive
          sx={{ '& .MuiToggleButton-root': { textTransform: 'none', px: 1.5 } }}
          value={period}
          onChange={(_, value) => value != null && setPeriod(value as Period)}
        >
          <ToggleButton value="year">Год</ToggleButton>
          {QUARTERS.map((quarter) => (
            <ToggleButton key={quarter} value={quarter}>Q{quarter}</ToggleButton>
          ))}
        </ToggleButtonGroup>

        {period === 'year' && (
          <ToggleButtonGroup
            size="small"
            exclusive
            sx={{ '& .MuiToggleButton-root': { textTransform: 'none', px: 1.5 } }}
            value={yearMetric}
            onChange={(_, value) => value != null && setYearMetric(value as YearMetric)}
          >
            {YEAR_METRICS.map((metric) => (
              <ToggleButton key={metric.value} value={metric.value}>{metric.label}</ToggleButton>
            ))}
          </ToggleButtonGroup>
        )}

        <Box sx={{ flex: 1 }} />

        {canEdit && (
          <Autocomplete
            size="small"
            options={availableBrands}
            value={null}
            blurOnSelect
            onChange={(_, value) => addBrand(value)}
            sx={{ minWidth: 220 }}
            renderInput={(params) => <TextField {...params} label="Добавить бренд" />}
          />
        )}
        {dirty && <Typography variant="caption" color="warning.main">Есть несохранённые изменения</Typography>}
        {canEdit && (
          <Button variant="contained" size="small" startIcon={<SaveIcon />} disabled={saving || !dirty} onClick={handleSave}>
            Сохранить
          </Button>
        )}
      </Box>

      {/* НДС квартала: влияет только на инвестиции */}
      <Paper variant="outlined" sx={{ px: 1.5, py: 1 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
          <Tooltip title="НДС применяется только к инвестициям: объёмы от него не зависят">
            <Typography variant="caption" color="text.secondary">НДС</Typography>
          </Tooltip>
          {vatQuarters.map((quarter) => {
            const setting = settings[quarter] ?? DEFAULT_SETTINGS;
            return (
              <Box key={quarter} sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                <Typography variant="caption" sx={{ minWidth: 20 }}>Q{quarter}</Typography>
                <Switch
                  size="small"
                  checked={setting.vat_included}
                  disabled={!canEdit}
                  onChange={(e) => setQuarterSetting(quarter, { vat_included: e.target.checked })}
                  slotProps={{ input: { 'aria-label': `Сеть работает с НДС в Q${quarter}` } }}
                />
                <TextField
                  size="small"
                  label="ставка"
                  value={setting.vat_rate}
                  disabled={!canEdit || !setting.vat_included}
                  onChange={(e) => setQuarterSetting(quarter, { vat_rate: parseNumberInput(e.target.value) ?? 0 })}
                  sx={{ width: 84 }}
                  slotProps={{ htmlInput: { inputMode: 'decimal', style: { padding: '6px 8px' } } }}
                />
              </Box>
            );
          })}
          {canEdit && period !== 'year' && (
            <Button size="small" onClick={() => applyVatToAllQuarters(period)}>Ко всем кварталам</Button>
          )}
        </Box>
      </Paper>

      <NetworkPlanSummary totals={periodTotals} periodLabel={periodLabel} />

      {period === 'year' ? (
        <NetworkYearTable
          metric={yearMetric}
          brands={brands}
          draft={draft}
          settings={settings}
          totals={totals}
          canEdit={canEdit}
          onCellChange={setCell}
          onToggleGross={toggleGross}
          onRemoveBrand={removeBrand}
        />
      ) : (
        <NetworkQuarterTable
          quarter={period}
          brands={brands}
          draft={draft}
          setting={settings[period] ?? DEFAULT_SETTINGS}
          totals={totals[period - 1]}
          canEdit={canEdit}
          commentedCells={commentedCells}
          onCellChange={(brand, patch) => setCell(period, brand, patch)}
          onToggleGross={(brand, next, allQuarters) => toggleGross(brand, next, allQuarters, period)}
          onRemoveBrand={removeBrand}
          onComment={(brand) => onCommentCell(period, brand)}
          onDistributeRest={() => distributeRest(period)}
        />
      )}

      <Typography variant="caption" color="text.secondary">
        Объёмы вводятся в рублях и от НДС не зависят. Валовый объём применяется к брендам:
        отнесённые к нему бренды распределяют общий объём контракта, остальные планируются
        отдельно и в остаток не попадают. Инвестиции по плану и по прогнозу считаются одним
        процентом, факт инвестиций приходит суммой и процентом не пересчитывается. Сумма
        с вычетом НДС по ставке квартала показывается в подсказке ячейки. Факт объёма
        и факт инвестиций загружаются из отгрузок и в форме не редактируются.
      </Typography>
    </Box>
  );
}
