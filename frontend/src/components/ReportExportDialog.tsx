// Конструктор отчёта PDF/PPTX по витрине реестра сетей.
//
// Форма наследует фильтры витрины, даёт выбрать блоки и их порядок, формат и
// лимит строк, показывает текстовый предпросмотр будущих страниц и ведёт
// фоновые задания до готовых файлов. Числа здесь не считаются и не
// показываются: всё берёт сервер по снимку фильтров.

import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Alert,
  Autocomplete,
  Box,
  Button,
  Checkbox,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControlLabel,
  IconButton,
  LinearProgress,
  ListItemText,
  MenuItem,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@mui/material';
import {
  ArrowDownward as ArrowDownwardIcon,
  ArrowUpward as ArrowUpwardIcon,
  Download as DownloadIcon,
} from '@mui/icons-material';
import { saveAs } from 'file-saver';
import { reportAPI } from '../api/reports';
import type { ReportBlock, ReportDraft, ReportFormat, ReportJobStatus, ReportUnit } from '../types/reports';
import { REPORT_TABLE_LIMITS } from '../types/reports';
import { apiErrorMessage } from '../utils/apiError';

const QUARTER_OPTIONS = [1, 2, 3, 4].map((value) => ({ label: `Q${value}`, value }));

const FORMAT_LABELS: Record<ReportFormat, string> = { pdf: 'PDF', pptx: 'PowerPoint' };
const STATUS_LABELS: Record<string, string> = {
  queued: 'в очереди',
  running: 'готовится',
  ready: 'готов',
  failed: 'ошибка',
};

export interface ReportExportDialogProps {
  open: boolean;
  onClose: () => void;
  // Текущие фильтры витрины — стартовое состояние формы.
  initial: { year: number; quarters: number[]; kams: string[]; networkIds: number[] };
  yearOptions: number[];
  kamOptions: string[];
  // Фильтр по КАМу показывается только тем, кто видит больше одного: у
  // закреплённого область задаёт сервер, и разрез по КАМам ему недоступен.
  showKamFilter: boolean;
  networkOptions: Array<{ label: string; value: number }>;
}

function isActive(job: ReportJobStatus): boolean {
  return job.status === 'queued' || job.status === 'running';
}

// Файлы готовятся в фоне, поэтому список заданий живёт во внешнем
// компоненте и переживает закрытие окна; форма же пересоздаётся на каждое
// открытие через key — так она подхватывает текущие фильтры витрины без
// эффектов, синхронизирующих state с props.
export default function ReportExportDialog(props: ReportExportDialogProps) {
  const [jobs, setJobs] = useState<ReportJobStatus[]>([]);
  const [openCount, setOpenCount] = useState(0);
  const [wasOpen, setWasOpen] = useState(false);
  if (props.open !== wasOpen) {
    setWasOpen(props.open);
    if (props.open) setOpenCount((n) => n + 1);
  }

  // Опрос заданий, пока хоть одно не завершилось.
  useEffect(() => {
    if (!jobs.some(isActive)) return;
    let active = true;
    const timer = window.setTimeout(async () => {
      const next = await Promise.all(jobs.map(async (job) => {
        if (!isActive(job)) return job;
        try {
          return await reportAPI.getJob(job.id);
        } catch (err) {
          const message = apiErrorMessage(err, '');
          // 404: задание истекло — показываем это, а не крутим спиннер вечно.
          if (message.includes('не найден')) {
            return { ...job, status: 'failed', error: 'Задание не найдено: срок хранения истёк' };
          }
          return job;
        }
      }));
      if (active) setJobs(next);
    }, 1500);
    return () => { active = false; window.clearTimeout(timer); };
  }, [jobs]);

  return <ReportForm key={openCount} {...props} jobs={jobs} setJobs={setJobs} />;
}

function ReportForm({
  open, onClose, initial, yearOptions, kamOptions, showKamFilter, networkOptions, jobs, setJobs,
}: ReportExportDialogProps & { jobs: ReportJobStatus[]; setJobs: (jobs: ReportJobStatus[]) => void }) {
  const blocksQuery = useQuery({
    queryKey: ['reportBlocks'],
    queryFn: () => reportAPI.getBlocks(),
    staleTime: 60 * 60 * 1000,
    enabled: open,
  });

  const catalog = useMemo(
    () => (blocksQuery.data ?? []).filter((block) => showKamFilter || !block.kamScopeOnly),
    [blocksQuery.data, showKamFilter],
  );

  const [draft, setDraft] = useState<ReportDraft>({
    title: '',
    year: initial.year,
    quarters: initial.quarters,
    kams: initial.kams,
    networkIds: initial.networkIds,
    unit: 'rub',
    blocks: [],
    formats: ['pdf', 'pptx'],
    tableLimit: 20,
  });
  // Порядок и выбор хранятся как отклонения от каталога: перестановки
  // пользователя и снятые галочки. Итоговый список выводится из каталога,
  // поэтому его загрузка не требует синхронизации state.
  const [userOrder, setUserOrder] = useState<string[]>([]);
  const [deselected, setDeselected] = useState<Set<string>>(new Set());
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [downloading, setDownloading] = useState<string | null>(null);

  const order = useMemo(() => {
    const known = new Set(catalog.map((block) => block.code));
    const kept = userOrder.filter((code) => known.has(code));
    const missing = catalog.map((block) => block.code).filter((code) => !kept.includes(code));
    return [...kept, ...missing];
  }, [catalog, userOrder]);
  const selected = useMemo(
    () => new Set(order.filter((code) => !deselected.has(code))),
    [order, deselected],
  );

  const blockByCode = useMemo(() => new Map(catalog.map((block) => [block.code, block])), [catalog]);
  const orderedSelected = order.filter((code) => selected.has(code));
  const contentBlocks = orderedSelected.filter((code) => !blockByCode.get(code)?.fixed);
  const hasTables = orderedSelected.some((code) => blockByCode.get(code)?.hasTable);

  const move = (code: string, direction: -1 | 1) => {
    const index = order.indexOf(code);
    const target = index + direction;
    if (index < 0 || target < 0 || target >= order.length) return;
    if (blockByCode.get(order[target])?.fixed) return;
    const next = [...order];
    [next[index], next[target]] = [next[target], next[index]];
    setUserOrder(next);
  };

  const toggle = (code: string) => {
    setDeselected((prev) => {
      const next = new Set(prev);
      if (next.has(code)) next.delete(code); else next.add(code);
      return next;
    });
  };

  const submit = async () => {
    setSubmitting(true);
    setError(null);
    try {
      const response = await reportAPI.create({
        ...draft,
        title: draft.title.trim(),
        blocks: orderedSelected,
      });
      setJobs(response.jobs);
    } catch (err) {
      setError(apiErrorMessage(err, 'Не удалось запустить подготовку отчёта'));
    } finally {
      setSubmitting(false);
    }
  };

  const download = async (job: ReportJobStatus) => {
    setDownloading(job.id);
    setError(null);
    try {
      saveAs(await reportAPI.download(job.id), job.fileName);
    } catch (err) {
      setError(apiErrorMessage(err, 'Не удалось скачать отчёт'));
    } finally {
      setDownloading(null);
    }
  };

  const periodLabel = draft.quarters.length === 0 || draft.quarters.length === 4
    ? 'весь год'
    : draft.quarters.map((q) => `Q${q}`).join(', ');
  const preparing = jobs.some(isActive);
  const canSubmit = contentBlocks.length > 0 && draft.formats.length > 0 && !submitting && !preparing;

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>Экспорт отчёта</DialogTitle>
      <DialogContent dividers>
        <Stack spacing={2}>
          <TextField
            label="Название отчёта"
            value={draft.title}
            onChange={(e) => setDraft({ ...draft, title: e.target.value })}
            placeholder="Итоги периода по сетям"
            slotProps={{ htmlInput: { maxLength: 120 } }}
            fullWidth
          />

          <Stack direction="row" spacing={1} useFlexGap sx={{ flexWrap: 'wrap', alignItems: 'center' }}>
            <TextField
              select
              label="Год"
              value={draft.year}
              onChange={(e) => setDraft({ ...draft, year: Number(e.target.value) })}
              sx={{ width: 110 }}
              size="small"
            >
              {yearOptions.map((y) => <MenuItem key={y} value={y}>{y}</MenuItem>)}
            </TextField>
            <Autocomplete<{ label: string; value: number }, true, false, false>
              multiple
              disableCloseOnSelect
              size="small"
              options={QUARTER_OPTIONS}
              getOptionLabel={(option) => option.label}
              isOptionEqualToValue={(option, value) => option.value === value?.value}
              value={QUARTER_OPTIONS.filter((option) => draft.quarters.includes(option.value))}
              onChange={(_, items) => setDraft({ ...draft, quarters: items.map((i) => i.value).sort((a, b) => a - b) })}
              renderInput={(params) => (
                <TextField {...params} label="Кварталы" placeholder={draft.quarters.length === 0 ? 'Весь год' : undefined} />
              )}
              sx={{ minWidth: 220 }}
            />
            {showKamFilter && (
              <Autocomplete<string, true, false, false>
                multiple
                size="small"
                options={kamOptions}
                value={draft.kams}
                onChange={(_, items) => setDraft({ ...draft, kams: items })}
                renderInput={(params) => (
                  <TextField {...params} label="КАМ" placeholder={draft.kams.length === 0 ? 'Все КАМ' : undefined} />
                )}
                sx={{ minWidth: 220 }}
              />
            )}
            <Autocomplete<{ label: string; value: number }, true, false, false>
              multiple
              disableCloseOnSelect
              limitTags={2}
              size="small"
              options={networkOptions}
              getOptionLabel={(option) => option.label}
              isOptionEqualToValue={(option, value) => option.value === value?.value}
              value={networkOptions.filter((option) => draft.networkIds.includes(option.value))}
              onChange={(_, items) => setDraft({ ...draft, networkIds: items.map((i) => i.value) })}
              renderInput={(params) => (
                <TextField {...params} label="Сети" placeholder={draft.networkIds.length === 0 ? 'Все сети' : undefined} />
              )}
              sx={{ minWidth: 280, flex: 1 }}
            />
          </Stack>

          <Stack direction="row" spacing={2} useFlexGap sx={{ flexWrap: 'wrap', alignItems: 'center' }}>
            <ToggleButtonGroup
              size="small"
              exclusive
              value={draft.unit}
              onChange={(_, value: ReportUnit | null) => value && setDraft({ ...draft, unit: value })}
            >
              <ToggleButton value="rub">Рубли</ToggleButton>
              <ToggleButton value="units">Упаковки</ToggleButton>
            </ToggleButtonGroup>
            <ToggleButtonGroup
              size="small"
              value={draft.formats}
              onChange={(_, value: ReportFormat[]) => setDraft({ ...draft, formats: value })}
            >
              <ToggleButton value="pdf">PDF</ToggleButton>
              <ToggleButton value="pptx">PowerPoint</ToggleButton>
            </ToggleButtonGroup>
            {hasTables && (
              <TextField
                select
                size="small"
                label="Строк в таблицах"
                value={draft.tableLimit}
                onChange={(e) => setDraft({ ...draft, tableLimit: Number(e.target.value) })}
                sx={{ width: 160 }}
              >
                {REPORT_TABLE_LIMITS.map((limit) => <MenuItem key={limit} value={limit}>{limit}</MenuItem>)}
              </TextField>
            )}
          </Stack>

          <Divider />

          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '3fr 2fr' }, gap: 2 }}>
            <Box>
              <Typography variant="subtitle2" sx={{ mb: 1 }}>Блоки отчёта</Typography>
              {blocksQuery.isLoading && <LinearProgress />}
              {blocksQuery.isError && (
                <Alert severity="error">{apiErrorMessage(blocksQuery.error, 'Не удалось загрузить каталог блоков')}</Alert>
              )}
              <Stack spacing={0.25}>
                {order.map((code, index) => {
                  const block: ReportBlock | undefined = blockByCode.get(code);
                  if (!block) return null;
                  const movable = !block.fixed;
                  return (
                    <Stack key={code} direction="row" spacing={0.5} sx={{ alignItems: 'center' }}>
                      <FormControlLabel
                        sx={{ flex: 1, mr: 0 }}
                        control={(
                          <Checkbox
                            size="small"
                            checked={selected.has(code)}
                            disabled={block.code === 'cover'}
                            onChange={() => toggle(code)}
                          />
                        )}
                        label={(
                          <ListItemText
                            primary={block.title}
                            secondary={block.description}
                            slotProps={{ primary: { sx: { fontSize: 14 } }, secondary: { sx: { fontSize: 12 } } }}
                          />
                        )}
                      />
                      <Tooltip title="Выше">
                        <span>
                          <IconButton
                            size="small"
                            disabled={!movable || index <= 1}
                            onClick={() => move(code, -1)}
                          >
                            <ArrowUpwardIcon fontSize="inherit" />
                          </IconButton>
                        </span>
                      </Tooltip>
                      <Tooltip title="Ниже">
                        <span>
                          <IconButton
                            size="small"
                            disabled={!movable || index >= order.length - 2}
                            onClick={() => move(code, 1)}
                          >
                            <ArrowDownwardIcon fontSize="inherit" />
                          </IconButton>
                        </span>
                      </Tooltip>
                    </Stack>
                  );
                })}
              </Stack>
            </Box>

            <Box>
              <Typography variant="subtitle2" sx={{ mb: 1 }}>Что получится</Typography>
              <Typography variant="body2" color="text.secondary">
                {draft.year} · {periodLabel}
                {draft.networkIds.length > 0 ? ` · сетей: ${draft.networkIds.length}` : ' · все сети'}
                {showKamFilter && draft.kams.length > 0 ? ` · КАМ: ${draft.kams.join(', ')}` : ''}
                {' · '}{draft.unit === 'rub' ? 'рубли' : 'упаковки'}
              </Typography>
              <Box component="ol" sx={{ pl: 2.5, my: 1, fontSize: 13 }}>
                {orderedSelected.map((code) => (
                  <li key={code}>{blockByCode.get(code)?.title ?? code}</li>
                ))}
              </Box>
              <Typography variant="caption" color="text.secondary">
                Страница PDF и слайд на блок; длинные таблицы продолжаются на следующих.
                Числа берёт сервер по снимку фильтров на момент запуска.
              </Typography>
            </Box>
          </Box>

          {jobs.length > 0 && (
            <>
              <Divider />
              <Stack spacing={1}>
                {jobs.map((job) => (
                  <Stack key={job.id} direction="row" spacing={1} sx={{ alignItems: 'center' }}>
                    <Chip size="small" label={FORMAT_LABELS[job.format as ReportFormat] ?? job.format} />
                    <Typography variant="body2" sx={{ flex: 1 }}>
                      {job.title} — {STATUS_LABELS[job.status] ?? job.status}
                      {job.status === 'failed' && job.error ? `: ${job.error}` : ''}
                    </Typography>
                    {isActive(job) && <CircularProgress size={16} />}
                    {job.status === 'ready' && (
                      <Button
                        size="small"
                        variant="outlined"
                        startIcon={<DownloadIcon />}
                        disabled={downloading === job.id}
                        onClick={() => download(job)}
                      >
                        Скачать
                      </Button>
                    )}
                  </Stack>
                ))}
              </Stack>
            </>
          )}

          {error && <Alert severity="error">{error}</Alert>}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Закрыть</Button>
        <Button variant="contained" onClick={submit} disabled={!canSubmit}>
          {preparing ? 'Готовится…' : 'Сформировать'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
