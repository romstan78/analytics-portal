// Вкладка «Инвестиции OPEX» реестра сетей.
//
// Второй механизм инвестиций рядом с бонусом за объём: бюджет за услуги сети,
// разложенный по статьям договора. От объёма он не зависит и порога выполнения
// не знает, поэтому живёт отдельной вкладкой, а не колонкой в «Плане и факте».
//
// Вводится квартал по бренду и статье, с НДС и со знаком. В базу уходят месяцы:
// квартальная сумма делится на три равные части до копейки, остаток от деления
// вливается в последний месяц квартала. Делит сервер — раскладка и база
// «без НДС» считаются там же, где и все остальные деньги реестра.

import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from '@mui/material';
import { Save as SaveIcon } from '@mui/icons-material';
import { networkAPI } from '../api/networks';
import { getUsername } from '../api/auth';
import type { NetworkOpexResponse, NetworkOpexSaveRequest } from '../types/network';
import { formatRub } from '../utils/networkPlan';
import {
  OPEX_QUARTERS,
  buildOpexDraft,
  isOpexAmountValid,
  opexCellKey,
  opexCellsByKey,
  opexChangedRows,
  opexDraftSum,
  opexMonthsLabel,
  type OpexDraft,
} from '../utils/networkOpex';
import {
  draftDiffers,
  draftSavedAtLabel,
  draftStorageKey,
  readDraft,
  removeDraft,
  type SavedDraft,
} from '../utils/formDraft';
import { useFormDraft } from '../hooks/useFormDraft';
import { PlanNumberField } from './networkPlanCells';

interface Props {
  networkId: number;
  year: number;
  canEdit: boolean;
}

// ─── Черновик сетки ─────────────────────────────────────────────────────────
// Бюджет вносят массово — бренды на пять статей и четыре квартала — и сохраняют
// одной кнопкой, поэтому обрыв сессии здесь стоит дороже, чем в обычной форме.
// Хранится только набранное: базу «без НДС», месяцы и итоги считает сервер.
const DRAFT_BASE_KEY = 'network_opex_draft_v1';

const opexDraftKey = (networkId: number, year: number) =>
  draftStorageKey(DRAFT_BASE_KEY, `${networkId}-${year}`);

// Черновик предлагается, только если он расходится с пришедшим с сервера:
// совпавший означает, что правки уже сохранены, и держать его незачем.
function opexDraftOffer(key: string, data: NetworkOpexResponse): SavedDraft<OpexDraft> | null {
  const saved = readDraft<OpexDraft>(key, getUsername());
  if (!saved) return null;
  if (!draftDiffers(saved.values, buildOpexDraft(data.cells))) {
    removeDraft(key);
    return null;
  }
  return saved;
}

// Сумма в ячейке итога: ноль показывается как «—», чтобы заполненные статьи
// читались сразу, а не искались среди нулей.
function TotalCell({ value, bold }: { value: number; bold?: boolean }) {
  return (
    <Typography variant="body2" sx={{ fontWeight: bold ? 700 : 500 }}>
      {value === 0 ? '—' : formatRub(value, 2)}
    </Typography>
  );
}

export default function NetworkOpexTab({ networkId, year, canEdit }: Props) {
  const [draftEdits, setDraftEdits] = useState<OpexDraft | null>(null);
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: ['network-opex', networkId, year],
    queryFn: () => networkAPI.getOpex(networkId, year),
  });

  const data = query.data;
  const cells = useMemo(() => data?.cells ?? [], [data?.cells]);
  const brands = useMemo(() => data?.brands ?? [], [data?.brands]);
  const articles = useMemo(() => data?.articles ?? [], [data?.articles]);
  const articleCodes = useMemo(() => articles.map((article) => article.code), [articles]);

  const savedByKey = useMemo(() => opexCellsByKey(cells), [cells]);
  const baseDraft = useMemo(() => buildOpexDraft(cells), [cells]);
  const draft = draftEdits ?? baseDraft;

  const changedRows = useMemo(
    () => opexChangedRows(draft, savedByKey, brands, articleCodes),
    [draft, savedByKey, brands, articleCodes],
  );
  const invalid = useMemo(
    () => Object.values(draft).some((value) => !isOpexAmountValid(value)),
    [draft],
  );
  const dirty = changedRows.length > 0;

  // Черновик прерванной работы. Вкладка пересоздаётся при смене сети и года
  // (key в NetworkRegistry), поэтому ключ неизменен, а предложение считается,
  // когда данные пришли: сравнивать черновик можно только с ними.
  const draftKey = opexDraftKey(networkId, year);
  const [draftOffer, setDraftOffer] = useState<SavedDraft<OpexDraft> | null>(null);
  const [seenData, setSeenData] = useState<NetworkOpexResponse | undefined>(undefined);
  if (seenData !== data) {
    setSeenData(data);
    // Данные могли смениться и под чужой правкой: тогда несохранённое
    // расходится с ними, и его есть смысл предложить обратно.
    setDraftEdits(null);
    setDraftOffer(canEdit && data ? opexDraftOffer(draftKey, data) : null);
  }
  useFormDraft({ storageKey: canEdit ? draftKey : null, values: draft, dirty });

  const mutation = useMutation({
    mutationFn: (request: NetworkOpexSaveRequest) => networkAPI.saveOpex(networkId, request),
    onSuccess: (response) => {
      // Черновик убирается раньше подстановки данных: она тут же пересчитает
      // предложение восстановления, и оно обязано увидеть пустое хранилище.
      removeDraft(draftKey);
      setDraftOffer(null);
      queryClient.setQueryData(['network-opex', networkId, year], response.data);
      void queryClient.invalidateQueries({ queryKey: ['networkAudit', networkId] });
      // Бюджет входит в столбец OPEX витрины — она обязана перечитать его.
      void queryClient.invalidateQueries({ queryKey: ['networkDashboard'] });
      setDraftEdits(null);
    },
  });

  const setCell = (brand: string, article: string, quarter: number, value: string) => {
    const key = opexCellKey(brand, article, quarter);
    setDraftEdits((current) => ({ ...(current ?? baseDraft), [key]: value }));
  };

  const handleSave = () => mutation.mutate({ year, rows: changedRows });

  const restoreDraft = () => {
    if (!draftOffer) return;
    setDraftEdits(draftOffer.values);
    setDraftOffer(null);
  };

  const dismissDraft = () => {
    removeDraft(draftKey);
    setDraftOffer(null);
  };

  if (query.isLoading) {
    return <Box sx={{ p: 4, textAlign: 'center' }}><CircularProgress /></Box>;
  }
  if (query.isError || !data) {
    return <Alert severity="error">Не удалось загрузить бюджет OPEX</Alert>;
  }

  const sum = (scope: { quarter?: number; brand?: string; article?: string }) =>
    opexDraftSum(draft, brands, articleCodes, scope);

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      {/* Значения не подставляются молча: пользователь должен понимать,
          откуда они взялись. */}
      {draftOffer && (
        <Alert
          severity="info"
          action={
            <Box sx={{ display: 'flex', gap: 1 }}>
              <Button size="small" variant="contained" onClick={restoreDraft}>Восстановить</Button>
              <Button size="small" color="inherit" onClick={dismissDraft}>Отклонить</Button>
            </Box>
          }
        >
          Остались несохранённые изменения от {draftSavedAtLabel(draftOffer.savedAt)}. Восстановить их?
        </Alert>
      )}

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
        <Box sx={{ minWidth: 240 }}>
          <Typography variant="subtitle2">Бюджет OPEX по контракту · {year}</Typography>
          <Typography variant="body2" color="text.secondary">
            Пять статей договора, квартал по бренду. Суммы с НДС и со знаком.
          </Typography>
        </Box>
        <Box sx={{ flex: 1 }} />
        {invalid && (
          <Typography variant="caption" color="error">
            Сумма вводится числом до сотых
          </Typography>
        )}
        {dirty && !invalid && (
          <Typography variant="caption" color="warning.main">Есть несохранённые изменения</Typography>
        )}
        {canEdit && (
          <Button
            variant="contained"
            size="small"
            startIcon={<SaveIcon />}
            disabled={mutation.isPending || !dirty || invalid}
            onClick={handleSave}
          >
            Сохранить
          </Button>
        )}
      </Box>

      {mutation.isError && (
        <Alert severity="error">{(mutation.error as Error).message}</Alert>
      )}

      {brands.length === 0 && (
        <Alert severity="info">
          Бюджет заводится по брендам плана: сначала добавьте бренды во вкладке «План и факт».
        </Alert>
      )}

      {brands.length > 0 && (
        <Paper variant="outlined" sx={{ overflowX: 'auto' }}>
          <Table size="small" sx={{ minWidth: 880 }}>
            <TableHead>
              <TableRow>
                <TableCell sx={{ minWidth: 150 }}>Бренд</TableCell>
                <TableCell sx={{ minWidth: 230 }}>Статья расходов</TableCell>
                {OPEX_QUARTERS.map((quarter) => (
                  <TableCell key={quarter} align="right" sx={{ minWidth: 120 }}>Q{quarter}</TableCell>
                ))}
                <TableCell align="right" sx={{ minWidth: 120 }}>Год</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {brands.map((brand) => (
                [
                  ...articles.map((article, index) => (
                    <TableRow key={`${brand}-${article.code}`} hover>
                      {index === 0 && (
                        <TableCell rowSpan={articles.length} sx={{ verticalAlign: 'top' }}>
                          <Typography variant="body2" sx={{ fontWeight: 600 }} title={brand}>
                            {brand}
                          </Typography>
                        </TableCell>
                      )}
                      <TableCell>
                        <Typography variant="body2">{article.label}</Typography>
                      </TableCell>
                      {OPEX_QUARTERS.map((quarter) => {
                        const key = opexCellKey(brand, article.code, quarter);
                        const saved = savedByKey[key];
                        const months = opexMonthsLabel(saved);
                        return (
                          <TableCell key={quarter} align="right">
                            <Tooltip
                              title={saved
                                ? `Без НДС: ${formatRub(saved.amount_rub_net, 2)} ₽ · по месяцам: ${months}`
                                : ''}
                            >
                              {/* Tooltip требует элемент, принимающий ref, а поле
                                  растянуто по ячейке — поэтому обёртка блочная,
                                  иначе ввод сжался бы до ширины текста. */}
                              <span style={{ display: 'block' }}>
                                <PlanNumberField
                                  value={draft[key] ?? ''}
                                  disabled={!canEdit}
                                  onChange={(value) => setCell(brand, article.code, quarter, value)}
                                />
                              </span>
                            </Tooltip>
                          </TableCell>
                        );
                      })}
                      <TableCell align="right">
                        <TotalCell value={sum({ brand, article: article.code })} />
                      </TableCell>
                    </TableRow>
                  )),
                  <TableRow key={`${brand}-total`} sx={{ '& td': { bgcolor: 'action.hover' } }}>
                    <TableCell colSpan={2}>
                      <Typography variant="body2" sx={{ fontWeight: 700 }}>Итого {brand}</Typography>
                    </TableCell>
                    {OPEX_QUARTERS.map((quarter) => (
                      <TableCell key={quarter} align="right">
                        <TotalCell value={sum({ brand, quarter })} bold />
                      </TableCell>
                    ))}
                    <TableCell align="right">
                      <TotalCell value={sum({ brand })} bold />
                    </TableCell>
                  </TableRow>,
                ]
              ))}
              <TableRow>
                <TableCell colSpan={2}>
                  <Typography variant="body2" sx={{ fontWeight: 700 }}>Итого по сети</Typography>
                </TableCell>
                {OPEX_QUARTERS.map((quarter) => (
                  <TableCell key={quarter} align="right">
                    <TotalCell value={sum({ quarter })} bold />
                  </TableCell>
                ))}
                <TableCell align="right">
                  <TotalCell value={sum({})} bold />
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </Paper>
      )}

      <Typography variant="caption" color="text.secondary">
        Суммы вводятся с НДС и с учётом знака: возврат или корректировка бюджета — это
        отрицательная сумма, а не пустая ячейка. Пустая ячейка означает «бюджет не заведён»,
        ноль — «заведён нулевым». Квартал хранится тремя месяцами: сумма делится на три равные
        части до копейки, остаток от деления уходит в последний месяц квартала, поэтому итог
        квартала равен введённому. Базу «без НДС» считает сервер по ставке квартала из профиля
        сети; сохранённые значения и раскладка по месяцам показаны в подсказке ячейки.
        Итоги в таблице складывают введённое, включая несохранённое.
      </Typography>
      <Typography variant="caption" color="text.secondary">
        Сохранено за год: {formatRub(data.totals.amount_rub, 2)} ₽ с НДС ·{' '}
        {formatRub(data.totals.amount_rub_net, 2)} ₽ без НДС.
      </Typography>
    </Box>
  );
}
