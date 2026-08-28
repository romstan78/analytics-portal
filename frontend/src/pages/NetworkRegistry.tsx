import { useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  InputAdornment,
  List,
  ListItemButton,
  ListItemText,
  Autocomplete,
  Checkbox,
  MenuItem,
  Paper,
  Snackbar,
  Switch,
  FormControlLabel,
  Stack,
  Tab,
  Tabs,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@mui/material';
import {
  Add as AddIcon,
  ArrowBack as ArrowBackIcon,
  Menu as MenuIcon,
  MenuOpen as MenuOpenIcon,
  Search as SearchIcon,
} from '@mui/icons-material';
import { networkAPI } from '../api/networks';
import NetworkDashboardView from '../components/NetworkDashboardView';
import NetworkDetailView from '../components/NetworkDetailView';
import NetworkForecastTab from '../components/NetworkForecastTab';
import NetworkAllocationEditor from '../components/NetworkAllocationEditor';
import NetworkVATEditor from '../components/NetworkVATEditor';
import NetworkInvestmentPaymentModes from '../components/NetworkInvestmentPaymentModes';
import NetworkPlanGrid from '../components/NetworkPlanGrid';
import NetworkPricesTab from '../components/NetworkPricesTab';
import NewNetworkDialog from '../components/NewNetworkDialog';
import type { NewNetworkValues } from '../components/NewNetworkDialog';
import type {
  Network,
  NetworkAuditRow,
	NetworkInvestmentPaymentModesSaveRequest,
  NetworkPlanChange,
  NetworkPlanSaveRequest,
  NetworkType,
} from '../types/network';
import {
  QUARTERS,
  formatPct,
  formatRub,
  isMonthDistributionValid,
  isVATRateValid,
  parseNumberInput,
  planKey,
} from '../utils/networkPlan';

const YEARS = [2026, 2027, 2028];
const DEFAULT_YEAR = 2027;
const CURRENT_YEAR = new Date().getFullYear();

const FIELD_LABELS: Record<string, string> = {
  plan_rub: 'План, ₽',
  forecast_rub: 'Прогноз, ₽',
  forecast_investments_rub: 'Прогноз инвестиций, ₽',
  fact_rub: 'Факт, ₽',
  investments_pct: 'Инвестиции, %',
  in_gross: 'Валовый объём',
  brand: 'Бренд в плане',
  vat_included: 'НДС',
  vat_rate: 'Ставка НДС',
  vat_included_q1: 'Q1 · работа с НДС',
  vat_included_q2: 'Q2 · работа с НДС',
  vat_included_q3: 'Q3 · работа с НДС',
  vat_included_q4: 'Q4 · работа с НДС',
  vat_rate_q1: 'Q1 · ставка НДС',
  vat_rate_q2: 'Q2 · ставка НДС',
  vat_rate_q3: 'Q3 · ставка НДС',
  vat_rate_q4: 'Q4 · ставка НДС',
  period: 'Квартал открыт',
  month_distribution: 'Распределение по месяцам',
  period_group: 'Объединение кварталов',
  pay_investments_from_fact: 'Оплата от факта',
  name: 'Название',
  network_type: 'Тип сети',
  kam: 'КАМ',
  is_active: 'Активность',
  has_annual_investment_cumulative: 'Годовой кумулятив инвестиций',
};

const MONTH_DISTRIBUTION_FIELDS = ['month1_pct', 'month2_pct', 'month3_pct'] as const;

type NetworkProfileDraft = Omit<
  Partial<Network>,
  'month1_pct' | 'month2_pct' | 'month3_pct' | 'vat_included' | 'vat_rate'
> & {
  month1_pct?: string;
  month2_pct?: string;
  month3_pct?: string;
};

interface NetworkProfilePeriodDraft {
  vatIncluded: boolean;
  vatRate: string;
}

const TYPE_LABELS: Record<string, string> = {
  regular: 'Обычная',
  warehouse: 'Складская',
};

// Значения в истории приходят разных типов: суммы, флаги, коды.
function formatAuditValue(field: string, value: unknown): string {
  if (value == null || value === '') return '—';
  if (field === 'vat_included' || field.startsWith('vat_included_q')) return value ? 'с НДС' : 'без НДС';
  if (field === 'is_active') return value ? 'активна' : 'скрыта';
  if (field === 'has_annual_investment_cumulative') return value ? 'включён' : 'выключен';
  if (field === 'in_gross') return value ? 'в валовом объёме' : 'отдельно';
  if (field === 'pay_investments_from_fact') return value ? 'включена' : 'выключена';
  if (field === 'network_type') return TYPE_LABELS[String(value)] ?? String(value);
  if (field === 'investments_pct' || field === 'vat_rate' || field.startsWith('vat_rate_q')) {
    return formatPct(Number(value));
  }
  if (field === 'plan_rub' || field === 'forecast_rub' || field === 'fact_rub') return formatRub(Number(value));
  return String(value);
}

interface AuditEntry {
  where: string;
  field: string;
  old: unknown;
  new: unknown;
}

// changed_fields приходит в двух формах: планы отдают {year, changes[]},
// карточка сети — {поле: {old, new}}.
function parseAudit(row: NetworkAuditRow): AuditEntry[] {
  if (!row.changed_fields) return [];
  try {
    const parsed = JSON.parse(row.changed_fields) as Record<string, unknown>;
    if (Array.isArray(parsed.changes)) {
      const year = parsed.year;
      return (parsed.changes as NetworkPlanChange[]).map((change) => ({
        where: [year, `Q${change.quarter}`, change.brand].filter(Boolean).join(' · '),
        field: change.field,
        old: change.old,
        new: change.new,
      }));
    }
    return Object.entries(parsed).map(([field, value]) => {
      const pair = value as { old?: unknown; new?: unknown };
      const isPair = pair && typeof pair === 'object' && ('old' in pair || 'new' in pair);
      return {
        where: 'Карточка сети',
        field,
        old: isPair ? pair.old : null,
        new: isPair ? pair.new : value,
      };
    });
  } catch {
    return [];
  }
}

interface NetworkRegistryProps {
  role: string | null;
}

// Витрина и карточки — два режима одного блока, а не разные разделы: из итогов
// проваливаются в сеть и возвращаются обратно, не выходя на главную.
type RegistryView = 'dashboard' | 'networks';

// Кварталы витрины — набор, а не диапазон: сравнивают и несмежные, например
// Q1 с Q3. Выбор устроен как в интернет-продажах — мультивыбор с галочками.
const QUARTER_OPTIONS = [
  { label: 'I квартал', value: 1 },
  { label: 'II квартал', value: 2 },
  { label: 'III квартал', value: 3 },
  { label: 'IV квартал', value: 4 },
];

const ALL_QUARTERS = [1, 2, 3, 4];

// Быстрые наборы. Год открыт по умолчанию: разговор об итогах начинается
// с года, а не с текущего квартала.
const QUARTER_PRESETS: Array<{ label: string; quarters: number[] }> = [
  { label: 'Год', quarters: ALL_QUARTERS },
  { label: 'I полугодие', quarters: [1, 2] },
  { label: 'II полугодие', quarters: [3, 4] },
];

function sameQuarters(a: number[], b: number[]): boolean {
  return a.length === b.length && a.every((value, index) => value === b[index]);
}

// Прямая ссылка на карточку: сеть и год приходят в адресе. Значения читаются
// один раз, при первом рендере, — дальше состоянием управляет сама страница.
function paramNetworkID(params: URLSearchParams): number | null {
  const value = Number(params.get('network'));
  return Number.isInteger(value) && value > 0 ? value : null;
}

function paramYear(params: URLSearchParams): number {
  const value = Number(params.get('year'));
  return Number.isInteger(value) && value >= 2000 && value <= 2100 ? value : DEFAULT_YEAR;
}

export default function NetworkRegistry({ role }: NetworkRegistryProps) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [searchParams] = useSearchParams();

  const canEdit = role === 'admin' || role === 'kam';

  const [search, setSearch] = useState('');
  // Фильтр списка сетей по КАМ. Пустая строка — все сети реестра.
  const [kam, setKam] = useState('');
  // Список сетей сворачивается: выбрав сеть, всю ширину отдаём таблице планов.
  const [listOpen, setListOpen] = useState(true);
  const [selectedId, setSelectedId] = useState<number | null>(() => paramNetworkID(searchParams));
  const [year, setYear] = useState(() => paramYear(searchParams));
  const [tab, setTab] = useState(0);
  // Реестр открывается списком сетей: работа идёт по конкретной сети, а итоги
  // смотрят отдельным переключением.
  const [view, setView] = useState<RegistryView>('networks');
  const [quarters, setQuarters] = useState<number[]>(ALL_QUARTERS);
  // Год витрины отделён от года карточки: карточка открывается на плановом
  // горизонте, а итоги — на последнем году, где есть данные. План заводят
  // вперёд, факт приходит назад, и «плановый» год у витрины часто пуст.
  // null означает «год не выбирали руками» — тогда он выводится из данных.
  const [dashboardYear, setDashboardYear] = useState<number | null>(null);
  // Список лет приходит в самом ответе и от запрошенного года не зависит,
  // поэтому он хранится отдельно и переживает смену запроса.
  const [dashboardYears, setDashboardYears] = useState<number[]>([]);
  // Сети витрины: пустой набор — весь доступный портфель. Набор, а не одна
  // сеть, потому что рядом сравнивают и две-три.
  const [dashboardNetworkIds, setDashboardNetworkIds] = useState<number[]>([]);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [commentTarget, setCommentTarget] = useState<{ quarter: number | null; brand: string | null } | null>(null);
  const [commentText, setCommentText] = useState('');
  const [profile, setProfile] = useState<NetworkProfileDraft>({});
  const [profilePeriods, setProfilePeriods] = useState<Record<number, NetworkProfilePeriodDraft>>({});
  const [toast, setToast] = useState<{ text: string; severity: 'success' | 'error' } | null>(null);

  const networksQuery = useQuery({
    queryKey: ['networks', search, kam],
    queryFn: () => networkAPI.getNetworks({ search, kam }),
  });

  const kamsQuery = useQuery({
    queryKey: ['networkKams'],
    queryFn: () => networkAPI.getKAMs(),
    staleTime: 30 * 60 * 1000,
  });

  const brandsQuery = useQuery({
    queryKey: ['networkBrands'],
    queryFn: () => networkAPI.getBrands(),
    staleTime: 30 * 60 * 1000,
  });

  // Витрина читается только в своём режиме: открывая карточку, незачем тянуть
  // весь портфель. Предыдущий ответ сохраняется — иначе на время повторного
  // запроса список лет опустел бы и выбранный год качался бы туда-обратно.
  // Пока год не выбран руками, берём последний год с данными, а при наличии —
  // текущий календарный: именно про него обычно и идёт разговор.
  const effectiveDashboardYear = dashboardYear ?? (
    dashboardYears.includes(CURRENT_YEAR) || dashboardYears.length === 0
      ? CURRENT_YEAR
      : dashboardYears[dashboardYears.length - 1]
  );

  const dashboardQuery = useQuery({
    queryKey: [
      'networkDashboard', effectiveDashboardYear, quarters.join(','), kam,
      dashboardNetworkIds.join(','),
    ],
    queryFn: async () => {
      const response = await networkAPI.getDashboard({
        year: effectiveDashboardYear,
        quarters,
        kam: kam ? [kam] : undefined,
        networkIds: dashboardNetworkIds,
      });
      setDashboardYears(response.availableYears);
      return response;
    },
    enabled: view === 'dashboard',
    placeholderData: (previous) => previous,
  });

  // Сети для фильтра витрины читаются отдельно от списка слева: тот сужается
  // поиском, а в фильтре обязаны остаться все сети области — иначе, выбрав
  // одну, вернуться к остальным было бы не через что.
  const dashboardNetworkOptions = useQuery({
    queryKey: ['networkDashboardOptions', kam],
    queryFn: () => networkAPI.getNetworks({ kam }),
    enabled: view === 'dashboard',
    staleTime: 30 * 60 * 1000,
  });

  const planQuery = useQuery({
    queryKey: ['networkPlan', selectedId, year],
    queryFn: () => networkAPI.getPlan(selectedId!, year),
    enabled: selectedId != null,
  });

  const commentsQuery = useQuery({
    queryKey: ['networkComments', selectedId],
    queryFn: () => networkAPI.getComments(selectedId!),
    enabled: selectedId != null,
  });

  const auditQuery = useQuery({
    queryKey: ['networkAudit', selectedId],
    queryFn: () => networkAPI.getAudit(selectedId!),
    enabled: selectedId != null && tab === 5,
  });

  const showError = (error: unknown) =>
    setToast({ text: (error as { message?: string })?.message ?? 'Ошибка запроса', severity: 'error' });

  const createMutation = useMutation({
    mutationFn: (values: NewNetworkValues) => networkAPI.create(values),
    onSuccess: (res) => {
      setDialogOpen(false);
      setSelectedId(res.data.id);
      setToast({ text: `Сеть «${res.data.name}» заведена`, severity: 'success' });
      void queryClient.invalidateQueries({ queryKey: ['networks'] });
    },
    onError: showError,
  });

  const savePlanMutation = useMutation({
    mutationFn: (request: NetworkPlanSaveRequest) => networkAPI.savePlan(selectedId!, request),
    onSuccess: () => {
      setToast({ text: 'Планы сохранены', severity: 'success' });
      void queryClient.invalidateQueries({ queryKey: ['networkPlan', selectedId, year] });
      void queryClient.invalidateQueries({ queryKey: ['networkAudit', selectedId] });
    },
    onError: showError,
  });

  const paymentModesMutation = useMutation({
    mutationFn: (request: NetworkInvestmentPaymentModesSaveRequest) =>
      networkAPI.saveInvestmentPaymentModes(selectedId!, request),
    onSuccess: () => {
      setToast({ text: 'Режим оплаты инвестиций сохранён', severity: 'success' });
      void queryClient.invalidateQueries({ queryKey: ['networkPlan', selectedId, year] });
      void queryClient.invalidateQueries({ queryKey: ['networkAudit', selectedId] });
    },
    onError: showError,
  });

  const profileMutation = useMutation({
    mutationFn: (network: Network) =>
      networkAPI.update(network.id, {
        name: profile.name ?? network.name,
        kam: profile.kam ?? network.kam ?? '',
        network_type: (profile.network_type ?? network.network_type) as NetworkType,
        is_active: profile.is_active ?? network.is_active,
        month1_pct: parseNumberInput(profile.month1_pct ?? String(network.month1_pct)) ?? network.month1_pct,
        month2_pct: parseNumberInput(profile.month2_pct ?? String(network.month2_pct)) ?? network.month2_pct,
        month3_pct: parseNumberInput(profile.month3_pct ?? String(network.month3_pct)) ?? network.month3_pct,
        has_annual_investment_cumulative:
          profile.has_annual_investment_cumulative ?? network.has_annual_investment_cumulative,
        year,
        periods: profilePeriodValues.map(({ quarter, vatIncluded, vatRate }) => ({
          quarter,
          vat_included: vatIncluded,
          vat_rate: parseNumberInput(vatRate) ?? 0,
        })),
        updated_at: network.updated_at,
      }),
    onSuccess: () => {
      setProfile({});
      setProfilePeriods({});
      setToast({ text: 'Профиль сети сохранён', severity: 'success' });
      void queryClient.invalidateQueries({ queryKey: ['networks'] });
      void queryClient.invalidateQueries({ queryKey: ['networkPlan', selectedId, year] });
      void queryClient.invalidateQueries({ queryKey: ['networkAudit', selectedId] });
    },
    onError: showError,
  });

  const commentMutation = useMutation({
    mutationFn: () =>
      networkAPI.addComment(selectedId!, {
        comment_text: commentText,
        year: commentTarget?.quarter ? year : null,
        quarter: commentTarget?.quarter ?? null,
        brand_as: commentTarget?.brand ?? null,
      }),
    onSuccess: () => {
      setCommentText('');
      setCommentTarget(null);
      void queryClient.invalidateQueries({ queryKey: ['networkComments', selectedId] });
    },
    onError: showError,
  });

  // Годы: к горизонту планирования добавляются те, за которые в реестре есть
  // данные (факт приходит и за прошлые годы), и обязательно выбранный сейчас —
  // иначе селектор показывал бы не то, что открыто.
  const yearOptions = useMemo(() => {
    const options = new Set<number>(YEARS);
    dashboardYears.forEach((value) => options.add(value));
    options.add(year);
    return [...options].sort((a, b) => a - b);
  }, [dashboardYears, year]);

  // Витрина показывает только те годы, за которые в реестре есть данные.
  const dashboardYearOptions = useMemo(() => {
    const options = new Set<number>(dashboardYears);
    options.add(effectiveDashboardYear);
    return [...options].sort((a, b) => a - b);
  }, [dashboardYears, effectiveDashboardYear]);

  const networkFilterOptions = useMemo(
    () => (dashboardNetworkOptions.data?.data ?? []).map((item) => ({ label: item.name, value: item.id })),
    [dashboardNetworkOptions.data],
  );

  // Смена КАМа снимает выбранные сети: они остались у прежнего, а сервер
  // пересекает оба фильтра — витрина опустела бы вместо портфеля выбранного.
  const changeKam = (value: string) => {
    setKam(value);
    setDashboardNetworkIds([]);
  };

  // Клик по сети на витрине проваливает в разбор, а не в карточку: карточка
  // открывается из разбора отдельной кнопкой. Фильтр и есть состояние
  // проваливания — поэтому назад ведут те же крошки, что его снимают.
  const drillIntoNetwork = (networkId: number) => setDashboardNetworkIds([networkId]);

  // Переход из витрины в карточку: сеть открывается сразу на «Плане и факте»,
  // на том же году и без ухода со страницы.
  const openNetworkFromDashboard = (networkId: number) => {
    setSelectedId(networkId);
    setProfile({});
    setProfilePeriods({});
    setYear(effectiveDashboardYear);
    setTab(2);
    setView('networks');
  };

  const networks = networksQuery.data?.data ?? [];
  const selected = planQuery.data?.network ?? networks.find((n) => n.id === selectedId) ?? null;
  const comments = useMemo(() => commentsQuery.data?.data ?? [], [commentsQuery.data]);
  const monthDistribution: [string, string, string] = selected ? [
    profile.month1_pct ?? String(selected.month1_pct),
    profile.month2_pct ?? String(selected.month2_pct),
    profile.month3_pct ?? String(selected.month3_pct),
  ] : ['30', '30', '40'];
  const monthDistributionValid = isMonthDistributionValid(monthDistribution);
  const profilePeriodValues = QUARTERS.map((quarter) => {
    const saved = planQuery.data?.periods.find((period) => period.quarter === quarter);
    const draft = profilePeriods[quarter];
    return {
      quarter,
      vatIncluded: draft?.vatIncluded ?? saved?.vat_included ?? selected?.vat_included ?? true,
      vatRate: draft?.vatRate ?? String(saved?.vat_rate ?? selected?.vat_rate ?? 20),
    };
  });
  // Та же проверка, что подсвечивает поле в NetworkVATEditor: кнопка сохранения
  // и подсказка об ошибке обязаны включаться и гаснуть вместе.
  const profileVATValid = profilePeriodValues.every(({ vatRate }) => isVATRateValid(vatRate));
  const profilePeriodsReady = planQuery.data?.year === year && planQuery.data.network.id === selectedId;
  const profileDirty = Object.keys(profile).length > 0 || Object.keys(profilePeriods).length > 0;

  // Ячейки с комментариями подсвечиваются в сетке планов.
  const commentedCells = useMemo(() => {
    const keys = new Set<string>();
    comments.forEach((c) => {
      if (c.quarter && c.year === year) keys.add(planKey(c.quarter, c.brand_as));
    });
    return keys;
  }, [comments, year]);

  // Валовый объём применяется к брендам, поэтому подпись считается по строкам
  // плана: сколько брендов года отнесено к общему объёму контракта.
  const contractLabel = useMemo(() => {
    const plans = planQuery.data?.plans ?? [];
    const withBrand = plans.filter((p) => p.brand_as);
    if (withBrand.length === 0) return null;
    const grossBrands = new Set(withBrand.filter((p) => p.in_gross).map((p) => p.brand_as));
    const allBrands = new Set(withBrand.map((p) => p.brand_as));
    if (grossBrands.size === 0) return 'без валового объёма';
    if (grossBrands.size === allBrands.size) return 'все бренды в валовом объёме';
    return `в валовом объёме: ${grossBrands.size} из ${allBrands.size}`;
  }, [planQuery.data]);

  return (
    <Box sx={{ p: { xs: 2, md: 3 }, maxWidth: 1760, mx: 'auto', width: '100%' }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 2, flexWrap: 'wrap' }}>
        <Button startIcon={<ArrowBackIcon />} onClick={() => navigate('/')}>На главную</Button>
        {view === 'networks' && (
          <Tooltip title={listOpen ? 'Свернуть список сетей' : 'Показать список сетей'}>
            <IconButton size="small" onClick={() => setListOpen((open) => !open)}>
              {listOpen ? <MenuOpenIcon /> : <MenuIcon />}
            </IconButton>
          </Tooltip>
        )}
        <Typography variant="h5">Реестр сетей</Typography>
        <ToggleButtonGroup
          size="small"
          exclusive
          value={view}
          onChange={(_, value) => value && setView(value as RegistryView)}
        >
          <ToggleButton value="networks">Сети</ToggleButton>
          <ToggleButton value="dashboard">Итоги</ToggleButton>
        </ToggleButtonGroup>
        <Box sx={{ flex: 1 }} />
        {canEdit && view === 'networks' && (
          <Button variant="contained" startIcon={<AddIcon />} onClick={() => setDialogOpen(true)}>
            Новая сеть
          </Button>
        )}
      </Box>

      {view === 'dashboard' && (
        <>
          <Stack direction="row" spacing={1} useFlexGap sx={{ flexWrap: 'wrap', alignItems: 'center', mb: 2 }}>
            <TextField
              select
              label="Год"
              value={effectiveDashboardYear}
              onChange={(e) => setDashboardYear(Number(e.target.value))}
              sx={{ width: 120 }}
            >
              {dashboardYearOptions.map((y) => <MenuItem key={y} value={y}>{y}</MenuItem>)}
            </TextField>
            <Autocomplete<{ label: string; value: number }, true, false, false>
              multiple
              disableCloseOnSelect
              size="small"
              options={QUARTER_OPTIONS}
              getOptionLabel={(option) => option.label}
              isOptionEqualToValue={(option, value) => option.value === value?.value}
              value={QUARTER_OPTIONS.filter((option) => quarters.includes(option.value))}
              onChange={(_, selected) => {
                // Пустой выбор оставил бы витрину без периода вовсе, поэтому
                // снятие последнего квартала возвращает весь год.
                const values = selected.map((option) => option.value).sort((a, b) => a - b);
                setQuarters(values.length > 0 ? values : ALL_QUARTERS);
              }}
              renderValue={() => null}
              renderOption={(props, option, { selected }) => {
                const { key, ...rest } = props;
                return (
                  <li key={key} {...rest} style={{ padding: '2px 8px' }}>
                    <Checkbox size="small" checked={selected} sx={{ mr: 1 }} />
                    <ListItemText
                      primary={option.label}
                      slotProps={{ primary: { sx: { fontSize: 13 } } }}
                    />
                  </li>
                );
              }}
              renderInput={(params) => (
                <TextField
                  {...params}
                  label="Кварталы"
                  placeholder={quarters.length === 4
                    ? 'Весь год'
                    : quarters.map((value) => `Q${value}`).join(', ')}
                  slotProps={{
                    ...params.slotProps,
                    inputLabel: { ...params.slotProps.inputLabel, shrink: true },
                  }}
                />
              )}
              slotProps={{ paper: { sx: { minWidth: 220 } } }}
              sx={{ minWidth: 210, '& .MuiAutocomplete-tag': { display: 'none' } }}
            />
            <ToggleButtonGroup
              size="small"
              exclusive
              value={QUARTER_PRESETS.find((preset) => sameQuarters(preset.quarters, quarters))?.label ?? ''}
              onChange={(_, value) => {
                const preset = QUARTER_PRESETS.find((item) => item.label === value);
                if (preset) setQuarters(preset.quarters);
              }}
            >
              {QUARTER_PRESETS.map((preset) => (
                <ToggleButton key={preset.label} value={preset.label}>{preset.label}</ToggleButton>
              ))}
            </ToggleButtonGroup>
            {/* Фильтр по КАМу бессмыслен для того, кто и так видит только свои
                сети: сервер уже ограничил область его закреплением. */}
            {(kamsQuery.data?.data ?? []).length > 1 && (
              <TextField
                select
                label="КАМ"
                value={kam}
                onChange={(e) => changeKam(e.target.value)}
                sx={{ width: 220 }}
              >
                <MenuItem value="">Все КАМ</MenuItem>
                {(kamsQuery.data?.data ?? []).map((option) => (
                  <MenuItem key={option} value={option}>{option}</MenuItem>
                ))}
              </TextField>
            )}
            {/* Сети выбираются набором: разбирают и одну, и пару рядом.
                Пустой выбор — весь портфель области. */}
            <Autocomplete<{ label: string; value: number }, true, false, false>
              multiple
              disableCloseOnSelect
              limitTags={1}
              size="small"
              options={networkFilterOptions}
              loading={dashboardNetworkOptions.isLoading}
              getOptionLabel={(option) => option.label}
              isOptionEqualToValue={(option, value) => option.value === value?.value}
              value={networkFilterOptions.filter((option) => dashboardNetworkIds.includes(option.value))}
              onChange={(_, selected) => setDashboardNetworkIds(selected.map((option) => option.value))}
              renderOption={(props, option, { selected }) => {
                const { key, ...rest } = props;
                return (
                  <li key={key} {...rest} style={{ padding: '2px 8px' }}>
                    <Checkbox size="small" checked={selected} sx={{ mr: 1 }} />
                    <ListItemText
                      primary={option.label}
                      slotProps={{ primary: { sx: { fontSize: 13 } } }}
                    />
                  </li>
                );
              }}
              renderInput={(params) => (
                <TextField
                  {...params}
                  label="Сеть"
                  placeholder={dashboardNetworkIds.length === 0 ? 'Все сети' : undefined}
                  slotProps={{
                    ...params.slotProps,
                    inputLabel: { ...params.slotProps.inputLabel, shrink: true },
                  }}
                />
              )}
              slotProps={{ paper: { sx: { minWidth: 320 } } }}
              sx={{ minWidth: 260, maxWidth: 380 }}
            />
            {/* На одной сети возврат уже есть в крошках разбора, и вторая
                такая же кнопка рядом была бы шумом. */}
            {dashboardNetworkIds.length > 1 && (
              <Button size="small" onClick={() => setDashboardNetworkIds([])}>Все сети</Button>
            )}
            {dashboardQuery.isFetching && <CircularProgress size={16} />}
          </Stack>

          {dashboardNetworkIds.length === 1 ? (
            <NetworkDetailView
              data={dashboardQuery.data ?? null}
              loading={dashboardQuery.isLoading}
              error={dashboardQuery.isError
                ? (dashboardQuery.error as { message?: string })?.message ?? 'Не удалось загрузить разбор сети'
                : null}
              onBackToAll={() => changeKam('')}
              onBackToKAM={kam ? () => setDashboardNetworkIds([]) : undefined}
              kamCrumb={kam || undefined}
              onOpenCard={openNetworkFromDashboard}
            />
          ) : (
            <NetworkDashboardView
              data={dashboardQuery.data ?? null}
              loading={dashboardQuery.isLoading}
              error={dashboardQuery.isError
                ? (dashboardQuery.error as { message?: string })?.message ?? 'Не удалось загрузить витрину'
                : null}
              onOpenNetwork={drillIntoNetwork}
            />
          )}
        </>
      )}

      {/* Режим сетей не размонтируется, а прячется: выбранная сеть, вкладка и
          несохранённый черновик переживают переход к итогам и обратно. */}
      <Box
        sx={{
          display: view === 'networks' ? 'grid' : 'none',
          gridTemplateColumns: { xs: '1fr', md: listOpen ? '260px minmax(0, 1fr)' : 'minmax(0, 1fr)' },
          gap: 2,
          alignItems: 'start',
        }}
      >
        {/* Список сетей */}
        <Paper variant="outlined" sx={{ p: 1.5, display: listOpen ? 'block' : 'none' }}>
          <TextField
            fullWidth
            placeholder="Поиск сети"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon fontSize="small" />
                  </InputAdornment>
                ),
              },
            }}
          />
          <TextField
            select
            fullWidth
            size="small"
            label="КАМ"
            value={kam}
            onChange={(e) => changeKam(e.target.value)}
            sx={{ mt: 1.5 }}
          >
            <MenuItem value="">Все КАМ</MenuItem>
            {(kamsQuery.data?.data ?? []).map((option) => (
              <MenuItem key={option} value={option}>{option}</MenuItem>
            ))}
          </TextField>
          {networksQuery.isLoading && <Box sx={{ p: 2, textAlign: 'center' }}><CircularProgress size={22} /></Box>}
          {networksQuery.isError && <Alert severity="error" sx={{ mt: 1 }}>Не удалось загрузить список сетей</Alert>}
          <List dense sx={{ maxHeight: '70vh', overflowY: 'auto', mt: 1 }}>
            {networks.map((network) => (
              <ListItemButton
                key={network.id}
                selected={network.id === selectedId}
                onClick={() => { setSelectedId(network.id); setProfile({}); setProfilePeriods({}); }}
              >
                <ListItemText
                  primary={network.name}
                  secondary={`${TYPE_LABELS[network.network_type]}${network.kam ? ` · ${network.kam}` : ''}`}
                />
              </ListItemButton>
            ))}
            {!networksQuery.isLoading && networks.length === 0 && (
              <Typography variant="body2" color="text.secondary" sx={{ p: 2 }}>
                Сети не найдены.
              </Typography>
            )}
          </List>
        </Paper>

        {/* Карточка сети */}
        <Paper variant="outlined" sx={{ p: { xs: 1.5, md: 2.5 }, minHeight: 420, minWidth: 0 }}>
          {!selected && (
            <Typography variant="body1" color="text.secondary">
              Выберите сеть слева, чтобы открыть план, прогноз и цены.
            </Typography>
          )}

          {selected && (
            <>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap', mb: 2 }}>
                <Typography variant="h6">{selected.name}</Typography>
                <Chip
                  size="small"
                  label={TYPE_LABELS[selected.network_type]}
                  color={selected.network_type === 'warehouse' ? 'info' : 'default'}
                  variant="outlined"
                />
                {contractLabel && <Chip size="small" label={contractLabel} variant="outlined" />}
                {selected.kam && <Chip size="small" label={`КАМ: ${selected.kam}`} variant="outlined" />}
                {!selected.is_active && <Chip size="small" label="скрыта" color="warning" />}
                <Box sx={{ flex: 1 }} />
                <TextField
                  select
                  label="Год"
                  value={year}
                  onChange={(e) => { setYear(Number(e.target.value)); setProfilePeriods({}); }}
                  sx={{ width: 120 }}
                >
                  {yearOptions.map((y) => <MenuItem key={y} value={y}>{y}</MenuItem>)}
                </TextField>
              </Box>

              <Tabs value={tab} onChange={(_, v) => setTab(v)} variant="scrollable" scrollButtons="auto" sx={{ mb: 2 }}>
                <Tab label="Профиль сети" />
                <Tab label="Цены и SKU" />
                <Tab label="План и факт" />
                <Tab label="Прогноз" />
                <Tab label={`Комментарии${comments.length ? ` · ${comments.length}` : ''}`} />
                <Tab label="История" />
              </Tabs>

              {tab === 2 && (
                <>
                  {planQuery.isLoading && <Box sx={{ p: 4, textAlign: 'center' }}><CircularProgress /></Box>}
                  {planQuery.isError && <Alert severity="error">Не удалось загрузить планы</Alert>}
                  {planQuery.data && (
                    <NetworkPlanGrid
                      data={planQuery.data}
                      brandOptions={brandsQuery.data?.data ?? []}
                      canEdit={canEdit}
                      saving={savePlanMutation.isPending}
                      onSave={(request) => savePlanMutation.mutate(request)}
                      onCommentCell={(quarter, brand) => setCommentTarget({ quarter, brand })}
                      commentedCells={commentedCells}
                    />
                  )}
                </>
              )}

              {tab === 3 && (
                <NetworkForecastTab key={`${selectedId}-${year}`} networkId={selectedId!} year={year} canEdit={canEdit} />
              )}

              {tab === 1 && (
                <NetworkPricesTab key={`${selectedId}-${year}`} networkId={selectedId!} year={year} canEdit={canEdit} />
              )}

              {tab === 0 && (
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, maxWidth: 720 }}>
                  <TextField
                    label="Название сети"
                    value={profile.name ?? selected.name}
                    disabled={!canEdit}
                    onChange={(e) => setProfile((p) => ({ ...p, name: e.target.value }))}
                  />
                  <TextField
                    label="КАМ"
                    value={profile.kam ?? selected.kam ?? ''}
                    disabled={!canEdit}
                    onChange={(e) => setProfile((p) => ({ ...p, kam: e.target.value }))}
                  />
                  <TextField
                    select
                    label="Тип сети"
                    value={profile.network_type ?? selected.network_type}
                    disabled={!canEdit}
                    onChange={(e) => setProfile((p) => ({ ...p, network_type: e.target.value as NetworkType }))}
                    helperText="У складской сети свой процесс прогнозирования объёмов"
                  >
                    <MenuItem value="regular">Обычная</MenuItem>
                    <MenuItem value="warehouse">Складская</MenuItem>
                  </TextField>
                  <NetworkVATEditor
                    year={year}
                    values={profilePeriodValues}
                    canEdit={canEdit}
                    ready={profilePeriodsReady}
                    onChange={(quarter, next) => setProfilePeriods((current) => ({
                      ...current,
                      [quarter]: next,
                    }))}
                  />
                  <NetworkAllocationEditor
                    values={monthDistribution}
                    canEdit={canEdit}
                    onChange={(index, value) => setProfile((current) => ({
                      ...current,
                      [MONTH_DISTRIBUTION_FIELDS[index]]: value,
                    }))}
                  />
                  {planQuery.data && (
                    <NetworkInvestmentPaymentModes
                      year={year}
                      plans={planQuery.data.plans}
                      canEdit={canEdit}
                      saving={paymentModesMutation.isPending}
                      onSave={(request) => paymentModesMutation.mutate(request)}
                    />
                  )}
                  <FormControlLabel
                    control={
                      <Switch
                        checked={
                          profile.has_annual_investment_cumulative
                          ?? selected.has_annual_investment_cumulative
                        }
                        disabled={!canEdit}
                        onChange={(event) => setProfile((current) => ({
                          ...current,
                          has_annual_investment_cumulative: event.target.checked,
                        }))}
                      />
                    }
                    label="Показывать годовой кумулятив инвестиций"
                  />
                  <Typography variant="caption" color="text.secondary" sx={{ mt: -1.5 }}>
                    Показатель появится во вкладке «План и факт» только для этой сети.
                  </Typography>
                  <FormControlLabel
                    control={
                      <Switch
                        checked={profile.is_active ?? selected.is_active}
                        disabled={!canEdit}
                        onChange={(e) => setProfile((p) => ({ ...p, is_active: e.target.checked }))}
                      />
                    }
                    label="Сеть активна"
                  />
                  {canEdit && (
                    <Box>
                      <Button
                        variant="contained"
                        disabled={
                          profileMutation.isPending
                          || !profileDirty
                          || !profilePeriodsReady
                          || !monthDistributionValid
                          || !profileVATValid
                        }
                        onClick={() => profileMutation.mutate(selected)}
                      >
                        Сохранить профиль
                      </Button>
                    </Box>
                  )}
                </Box>
              )}

              {tab === 4 && (
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, maxWidth: 720 }}>
                  <Box sx={{ display: 'flex', gap: 1 }}>
                    <TextField
                      fullWidth
                      multiline
                      minRows={2}
                      placeholder="Комментарий по сети"
                      value={commentTarget ? '' : commentText}
                      disabled={!!commentTarget}
                      onChange={(e) => setCommentText(e.target.value)}
                    />
                    <Button
                      variant="contained"
                      disabled={commentMutation.isPending || commentText.trim() === '' || !!commentTarget}
                      onClick={() => commentMutation.mutate()}
                    >
                      Отправить
                    </Button>
                  </Box>
                  <Divider />
                  {comments.length === 0 && (
                    <Typography variant="body2" color="text.secondary">Комментариев пока нет.</Typography>
                  )}
                  {[...comments].reverse().map((comment) => (
                    <Paper key={comment.id} variant="outlined" sx={{ p: 1.5 }}>
                      <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', mb: 0.5, flexWrap: 'wrap' }}>
                        <Typography variant="subtitle2">{comment.user_name}</Typography>
                        <Chip size="small" label={comment.role} variant="outlined" />
                        {comment.quarter && (
                          <Chip
                            size="small"
                            color="primary"
                            variant="outlined"
                            label={[comment.year, `Q${comment.quarter}`, comment.brand_as].filter(Boolean).join(' · ')}
                          />
                        )}
                        <Typography variant="caption" color="text.secondary">
                          {comment.created_at?.slice(0, 16) ?? ''}
                        </Typography>
                      </Box>
                      <Typography variant="body2">{comment.comment_text}</Typography>
                    </Paper>
                  ))}
                </Box>
              )}

              {tab === 5 && (
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, maxWidth: 860 }}>
                  {auditQuery.isLoading && <CircularProgress size={22} />}
                  {auditQuery.isError && <Alert severity="error">Не удалось загрузить историю</Alert>}
                  {auditQuery.data?.data.length === 0 && (
                    <Typography variant="body2" color="text.secondary">Изменений пока не было.</Typography>
                  )}
                  {auditQuery.data?.data.map((row) => (
                    <Paper key={row.id} variant="outlined" sx={{ p: 1.5 }}>
                      <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', mb: 0.5, flexWrap: 'wrap' }}>
                        <Typography variant="subtitle2">{row.user_name}</Typography>
                        <Chip size="small" label={row.action_type} variant="outlined" />
                        <Typography variant="caption" color="text.secondary">
                          {row.created_at?.slice(0, 16) ?? ''}
                        </Typography>
                      </Box>
                      {parseAudit(row).map((entry, index) => (
                        <Typography key={index} variant="body2" sx={{ color: 'text.secondary' }}>
                          {entry.where} · {FIELD_LABELS[entry.field] ?? entry.field}:{' '}
                          <Box component="span" sx={{ color: 'error.main', textDecoration: 'line-through' }}>
                            {formatAuditValue(entry.field, entry.old)}
                          </Box>
                          {' → '}
                          <Box component="span" sx={{ color: 'success.main', fontWeight: 600 }}>
                            {formatAuditValue(entry.field, entry.new)}
                          </Box>
                        </Typography>
                      ))}
                    </Paper>
                  ))}
                </Box>
              )}
            </>
          )}
        </Paper>
      </Box>

      <NewNetworkDialog
        open={dialogOpen}
        saving={createMutation.isPending}
        error={createMutation.isError ? (createMutation.error as { message?: string })?.message ?? 'Ошибка' : null}
        onClose={() => setDialogOpen(false)}
        onSubmit={(values) => createMutation.mutate(values)}
      />

      {/* Комментарий к конкретной ячейке плана */}
      <Dialog open={!!commentTarget} onClose={() => setCommentTarget(null)} maxWidth="sm" fullWidth>
        <DialogTitle>
          Комментарий к плану
          {commentTarget && ` · ${year} · Q${commentTarget.quarter} · ${commentTarget.brand ?? 'общий объём'}`}
        </DialogTitle>
        <DialogContent>
          <TextField
            fullWidth
            multiline
            minRows={3}
            autoFocus
            placeholder="Почему план такой"
            value={commentText}
            onChange={(e) => setCommentText(e.target.value)}
            sx={{ mt: 1 }}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => { setCommentTarget(null); setCommentText(''); }}>Отмена</Button>
          <Button
            variant="contained"
            disabled={commentMutation.isPending || commentText.trim() === ''}
            onClick={() => commentMutation.mutate()}
          >
            Сохранить
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar
        open={!!toast}
        autoHideDuration={4000}
        onClose={() => setToast(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert severity={toast?.severity ?? 'success'} onClose={() => setToast(null)}>
          {toast?.text}
        </Alert>
      </Snackbar>
    </Box>
  );
}
