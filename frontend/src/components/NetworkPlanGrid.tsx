import { useMemo, useState } from 'react';
import {
  Autocomplete,
  Box,
  Button,
  Chip,
  IconButton,
  MenuItem,
  Paper,
  Switch,
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
  ChatBubbleOutlined as CommentIcon,
  DeleteOutlined as DeleteIcon,
  Save as SaveIcon,
} from '@mui/icons-material';
import type {
  ContractType,
  NetworkPlanResponse,
  NetworkPlanSaveRequest,
  NetworkPlanInput,
} from '../types/network';
import {
  QUARTERS,
  brandsFromPlans,
  buildDraft,
  buildSettings,
  calcQuarterTotals,
  formatRub,
  netRub,
  parseNumberInput,
  planKey,
  round2,
} from '../utils/networkPlan';
import type { DraftCell, QuarterSettings } from '../utils/networkPlan';

const DEFAULT_SETTINGS: QuarterSettings = { vat_included: true, vat_rate: 20, contract_type: 'regular' };

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
  const hasGrossQuarter = QUARTERS.some((q) => settings[q]?.contract_type === 'gross');

  const cellValue = (quarter: number, brand: string | null): DraftCell =>
    draft[planKey(quarter, brand)] ?? { planRub: '', investmentsPct: '' };

  const setCell = (quarter: number, brand: string | null, patch: Partial<DraftCell>) => {
    setDraft((prev) => {
      const key = planKey(quarter, brand);
      const current = prev[key] ?? { planRub: '', investmentsPct: '' };
      return { ...prev, [key]: { ...current, ...patch } };
    });
    setDirty(true);
  };

  const setQuarterSetting = (quarter: number, patch: Partial<QuarterSettings>) => {
    setSettings((prev) => ({ ...prev, [quarter]: { ...(prev[quarter] ?? DEFAULT_SETTINGS), ...patch } }));
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
        if (next[planKey(quarter, brand)]) next[planKey(quarter, brand)] = { planRub: '', investmentsPct: '' };
      });
      return next;
    });
    setDirty(true);
  };

  // Остаток валового контракта делим поровну между брендами квартала.
  const distributeRest = (quarter: number) => {
    const total = totals.find((t) => t.quarter === quarter);
    if (!total?.undistributed || brands.length === 0) return;
    const share = round2(total.undistributed / brands.length);
    setDraft((prev) => {
      const next = { ...prev };
      brands.forEach((brand) => {
        const key = planKey(quarter, brand);
        const current = next[key] ?? { planRub: '', investmentsPct: '' };
        const value = parseNumberInput(current.planRub) ?? 0;
        next[key] = { ...current, planRub: String(round2(value + share)) };
      });
      return next;
    });
    setDirty(true);
  };

  // План за год — сумма кварталов как введено; НДС применяется только к инвестициям.
  const yearTotal = (brand: string | null): { plan: number; investments: number; investmentsNet: number } => {
    let plan = 0;
    let investments = 0;
    let investmentsNet = 0;
    QUARTERS.forEach((quarter) => {
      const cell = cellValue(quarter, brand);
      const value = parseNumberInput(cell.planRub);
      if (value == null) return;
      plan = round2(plan + value);
      const pct = parseNumberInput(cell.investmentsPct);
      if (pct == null) return;
      const setting = settings[quarter] ?? DEFAULT_SETTINGS;
      const quarterInvestments = round2((value * pct) / 100);
      investments = round2(investments + quarterInvestments);
      investmentsNet = round2(investmentsNet + netRub(quarterInvestments, setting.vat_included, setting.vat_rate));
    });
    return { plan, investments, investmentsNet };
  };

  const handleSave = () => {
    const versions = new Map(data.plans.map((p) => [planKey(p.quarter, p.brand_as), p.updated_at]));
    const rows: NetworkPlanInput[] = [];

    QUARTERS.forEach((quarter) => {
      const rowBrands: Array<string | null> = [...brands];
      if (settings[quarter]?.contract_type === 'gross') rowBrands.unshift(null);
      // Бренд убрали из таблицы, но строка есть в БД — отправляем пустые значения.
      data.plans.forEach((plan) => {
        if (plan.quarter === quarter && plan.brand_as && !brands.includes(plan.brand_as)) {
          rowBrands.push(plan.brand_as);
        }
      });

      rowBrands.forEach((brand) => {
        const cell = cellValue(quarter, brand);
        rows.push({
          quarter,
          brand_as: brand,
          plan_rub: parseNumberInput(cell.planRub),
          investments_pct: brand === null ? null : parseNumberInput(cell.investmentsPct),
          updated_at: versions.get(planKey(quarter, brand)) ?? '',
        });
      });
    });

    onSave({
      year: data.year,
      periods: QUARTERS.map((quarter) => {
        const setting = settings[quarter] ?? DEFAULT_SETTINGS;
        return {
          quarter,
          vat_included: setting.vat_included,
          vat_rate: setting.vat_rate,
          contract_type: setting.contract_type,
        };
      }),
      plans: rows,
    });
  };

  const availableBrands = brandOptions.filter((b) => !brands.includes(b));

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2.5 }}>
      {/* Настройки кварталов: тип контракта и НДС (влияет только на инвестиции) */}
      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)', lg: 'repeat(4, 1fr)' }, gap: 1.5 }}>
        {QUARTERS.map((quarter) => {
          const setting = settings[quarter] ?? DEFAULT_SETTINGS;
          return (
            <Paper key={quarter} variant="outlined" sx={{ p: 1.5, display: 'flex', flexDirection: 'column', gap: 1 }}>
              <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <Typography variant="subtitle2">Q{quarter}</Typography>
                <Chip
                  size="small"
                  label={setting.vat_included ? 'с НДС' : 'без НДС'}
                  color={setting.vat_included ? 'success' : 'warning'}
                  variant="outlined"
                />
              </Box>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <Switch
                  size="small"
                  checked={setting.vat_included}
                  disabled={!canEdit}
                  onChange={(e) => setQuarterSetting(quarter, { vat_included: e.target.checked })}
                  slotProps={{ input: { 'aria-label': `Сеть работает с НДС в Q${quarter}` } }}
                />
                <TextField
                  label="Ставка, %"
                  value={setting.vat_rate}
                  disabled={!canEdit || !setting.vat_included}
                  onChange={(e) => setQuarterSetting(quarter, { vat_rate: parseNumberInput(e.target.value) ?? 0 })}
                  sx={{ width: 96 }}
                />
              </Box>
              <TextField
                select
                label="Контракт"
                value={setting.contract_type}
                disabled={!canEdit}
                onChange={(e) => setQuarterSetting(quarter, { contract_type: e.target.value as ContractType })}
              >
                <MenuItem value="regular">Обычный</MenuItem>
                <MenuItem value="gross">Валовый</MenuItem>
              </TextField>
            </Paper>
          );
        })}
      </Box>

      {/* Панель действий над сеткой */}
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
        {canEdit && (
          <Autocomplete
            options={availableBrands}
            value={null}
            blurOnSelect
            onChange={(_, value) => addBrand(value)}
            sx={{ minWidth: 260 }}
            renderInput={(params) => <TextField {...params} label="Добавить бренд" />}
          />
        )}
        <Box sx={{ flex: 1 }} />
        {dirty && <Typography variant="caption" color="warning.main">Есть несохранённые изменения</Typography>}
        {canEdit && (
          <Button variant="contained" startIcon={<SaveIcon />} disabled={saving || !dirty} onClick={handleSave}>
            Сохранить
          </Button>
        )}
      </Box>

      <Paper variant="outlined" sx={{ overflowX: 'auto' }}>
        <Table size="small" sx={{ minWidth: 980 }}>
          <TableHead>
            <TableRow>
              <TableCell sx={{ minWidth: 220 }}>Бренд</TableCell>
              {QUARTERS.map((quarter) => [
                <TableCell key={`p${quarter}`} align="right" sx={{ minWidth: 130 }}>Q{quarter}, ₽</TableCell>,
                <TableCell key={`i${quarter}`} align="right" sx={{ minWidth: 96 }}>Инв., %</TableCell>,
              ])}
              <TableCell align="right" sx={{ minWidth: 140 }}>Год, ₽</TableCell>
              <TableCell align="right" sx={{ minWidth: 140 }}>Инвест., ₽</TableCell>
              <TableCell align="right" sx={{ minWidth: 150 }}>Инвест. без НДС, ₽</TableCell>
              <TableCell sx={{ width: 48 }} />
            </TableRow>
          </TableHead>
          <TableBody>
            {/* Общий объём валового контракта — по кварталам, где выбран этот тип */}
            {hasGrossQuarter && (
              <TableRow sx={{ bgcolor: 'action.hover' }}>
                <TableCell sx={{ fontWeight: 600 }}>Общий объём контракта</TableCell>
                {QUARTERS.map((quarter) => {
                  const isGross = settings[quarter]?.contract_type === 'gross';
                  return [
                    <TableCell key={`gp${quarter}`} align="right">
                      {isGross ? (
                        <TextField
                          value={cellValue(quarter, null).planRub}
                          disabled={!canEdit}
                          onChange={(e) => setCell(quarter, null, { planRub: e.target.value })}
                          slotProps={{ htmlInput: { inputMode: 'decimal', style: { textAlign: 'right' } } }}
                        />
                      ) : (
                        <Typography variant="body2" color="text.disabled">—</Typography>
                      )}
                    </TableCell>,
                    <TableCell key={`gi${quarter}`} align="right">
                      <Typography variant="body2" color="text.disabled">—</Typography>
                    </TableCell>,
                  ];
                })}
                <TableCell align="right" sx={{ fontWeight: 600 }}>{formatRub(yearTotal(null).plan)}</TableCell>
                <TableCell align="right">—</TableCell>
                <TableCell align="right">—</TableCell>
                <TableCell />
              </TableRow>
            )}

            {brands.map((brand) => {
              const year = yearTotal(brand);
              return (
                <TableRow key={brand} hover>
                  <TableCell>{brand}</TableCell>
                  {QUARTERS.map((quarter) => {
                    const cell = cellValue(quarter, brand);
                    const hasComment = commentedCells.has(planKey(quarter, brand));
                    return [
                      <TableCell key={`p${quarter}`} align="right">
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                          <TextField
                            value={cell.planRub}
                            disabled={!canEdit}
                            onChange={(e) => setCell(quarter, brand, { planRub: e.target.value })}
                            slotProps={{ htmlInput: { inputMode: 'decimal', style: { textAlign: 'right' } } }}
                          />
                          <Tooltip title={hasComment ? 'Есть комментарий' : 'Комментарий к ячейке'}>
                            <IconButton size="small" onClick={() => onCommentCell(quarter, brand)}>
                              <CommentIcon
                                fontSize="inherit"
                                color={hasComment ? 'warning' : 'disabled'}
                              />
                            </IconButton>
                          </Tooltip>
                        </Box>
                      </TableCell>,
                      <TableCell key={`i${quarter}`} align="right">
                        <TextField
                          value={cell.investmentsPct}
                          disabled={!canEdit}
                          onChange={(e) => setCell(quarter, brand, { investmentsPct: e.target.value })}
                          slotProps={{ htmlInput: { inputMode: 'decimal', step: '0.01', style: { textAlign: 'right' } } }}
                        />
                      </TableCell>,
                    ];
                  })}
                  <TableCell align="right">{formatRub(year.plan)}</TableCell>
                  <TableCell align="right">{formatRub(year.investments)}</TableCell>
                  <TableCell align="right" sx={{ color: 'text.secondary' }}>{formatRub(year.investmentsNet)}</TableCell>
                  <TableCell>
                    {canEdit && (
                      <Tooltip title="Убрать бренд из плана">
                        <IconButton size="small" onClick={() => removeBrand(brand)}>
                          <DeleteIcon fontSize="inherit" />
                        </IconButton>
                      </Tooltip>
                    )}
                  </TableCell>
                </TableRow>
              );
            })}

            {brands.length === 0 && (
              <TableRow>
                <TableCell colSpan={13}>
                  <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
                    Брендов в плане пока нет. Добавьте бренд, чтобы внести суммы по кварталам.
                  </Typography>
                </TableCell>
              </TableRow>
            )}

            <TableRow sx={{ bgcolor: 'action.hover' }}>
              <TableCell sx={{ fontWeight: 600 }}>Распределено по брендам</TableCell>
              {QUARTERS.map((quarter) => {
                const total = totals.find((t) => t.quarter === quarter);
                return [
                  <TableCell key={`tp${quarter}`} align="right" sx={{ fontWeight: 600 }}>
                    {formatRub(total?.planRub ?? 0)}
                  </TableCell>,
                  <TableCell key={`ti${quarter}`} align="right" sx={{ fontWeight: 600 }}>
                    {formatRub(total?.investmentsRub ?? 0)}
                  </TableCell>,
                ];
              })}
              <TableCell align="right" sx={{ fontWeight: 600 }}>
                {formatRub(brands.reduce((sum, brand) => round2(sum + yearTotal(brand).plan), 0))}
              </TableCell>
              <TableCell align="right" sx={{ fontWeight: 600 }}>
                {formatRub(brands.reduce((sum, brand) => round2(sum + yearTotal(brand).investments), 0))}
              </TableCell>
              <TableCell align="right" sx={{ fontWeight: 600, color: 'text.secondary' }}>
                {formatRub(brands.reduce((sum, brand) => round2(sum + yearTotal(brand).investmentsNet), 0))}
              </TableCell>
              <TableCell />
            </TableRow>

            <TableRow>
              <TableCell sx={{ color: 'text.secondary' }}>Инвестиции с вычетом НДС</TableCell>
              {QUARTERS.map((quarter) => {
                const total = totals.find((t) => t.quarter === quarter);
                return [
                  <TableCell key={`np${quarter}`} align="right" sx={{ color: 'text.disabled' }}>—</TableCell>,
                  <TableCell key={`ni${quarter}`} align="right" sx={{ color: 'text.secondary' }}>
                    {formatRub(total?.investmentsRubNet ?? 0)}
                  </TableCell>,
                ];
              })}
              <TableCell align="right" sx={{ color: 'text.disabled' }}>—</TableCell>
              <TableCell align="right" sx={{ color: 'text.disabled' }}>—</TableCell>
              <TableCell align="right" sx={{ color: 'text.secondary' }}>
                {formatRub(brands.reduce((sum, brand) => round2(sum + yearTotal(brand).investmentsNet), 0))}
              </TableCell>
              <TableCell />
            </TableRow>

            {hasGrossQuarter && (
              <TableRow>
                <TableCell sx={{ color: 'text.secondary' }}>Остаток к распределению</TableCell>
                {QUARTERS.map((quarter) => {
                  const total = totals.find((t) => t.quarter === quarter);
                  const rest = total?.undistributed;
                  return [
                    <TableCell key={`rp${quarter}`} align="right">
                      {rest == null ? (
                        <Typography variant="body2" color="text.disabled">—</Typography>
                      ) : (
                        <Typography variant="body2" color={rest === 0 ? 'success.main' : 'warning.main'} sx={{ fontWeight: 600 }}>
                          {formatRub(rest)}
                        </Typography>
                      )}
                    </TableCell>,
                    <TableCell key={`ri${quarter}`} align="right">
                      {canEdit && rest != null && rest !== 0 && brands.length > 0 && (
                        <Button size="small" onClick={() => distributeRest(quarter)}>Поровну</Button>
                      )}
                    </TableCell>,
                  ];
                })}
                <TableCell align="right">—</TableCell>
                <TableCell align="right">—</TableCell>
                <TableCell align="right">—</TableCell>
                <TableCell />
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Paper>

      <Typography variant="caption" color="text.secondary">
        План вводится в рублях и от НДС не зависит. Инвестиции — процент от плана с точностью до сотых;
        колонка «Инвест., ₽» показывает сумму до вычета НДС, «Инвест. без НДС, ₽» — с вычетом ставки того
        квартала, в котором сеть работает с НДС.
      </Typography>
    </Box>
  );
}
