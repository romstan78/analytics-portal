import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
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
import { Add as AddIcon, DeleteOutlined as DeleteIcon, Save as SaveIcon } from '@mui/icons-material';
import { networkAPI } from '../api/networks';
import type { NetworkContractPriceDeleteInput, NetworkPricesSaveRequest } from '../types/network';
import { formatRub, parseNumberInput } from '../utils/networkPlan';
import {
  buildQuarterlyPriceDrafts,
  createEmptyQuarterlyPriceDraft,
  quarterlyPriceInputs,
  type PriceQuarterValues,
  type QuarterlyPriceDraft,
} from '../utils/networkPrices';

interface Props {
  networkId: number;
  year: number;
  canEdit: boolean;
}

const MONTHS = [
  'январь', 'февраль', 'март', 'апрель', 'май', 'июнь',
  'июль', 'август', 'сентябрь', 'октябрь', 'ноябрь', 'декабрь',
];

export default function NetworkPricesTab({ networkId, year, canEdit }: Props) {
  const [draftEdits, setDraftEdits] = useState<QuarterlyPriceDraft[] | null>(null);
  const [deletedRows, setDeletedRows] = useState<NetworkContractPriceDeleteInput[]>([]);
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: ['network-prices', networkId, year],
    queryFn: () => networkAPI.getPrices(networkId, year),
  });

  const baseDraft = useMemo(
    () => buildQuarterlyPriceDrafts(query.data?.data ?? [], year),
    [query.data, year],
  );
  const draft = draftEdits ?? baseDraft;
  const dirty = draftEdits != null || deletedRows.length > 0;

  const mutation = useMutation({
    mutationFn: (request: NetworkPricesSaveRequest) => networkAPI.savePrices(networkId, request),
    onSuccess: (response) => {
      queryClient.setQueryData(['network-prices', networkId, year], response.data);
      void queryClient.invalidateQueries({ queryKey: ['network-forecast', networkId, year] });
      void queryClient.invalidateQueries({ queryKey: ['networkAudit', networkId] });
      setDraftEdits(null);
      setDeletedRows([]);
    },
  });

  const updateRow = (key: string, patch: Partial<QuarterlyPriceDraft>) => {
    setDraftEdits((current) => (current ?? baseDraft).map((row) => row.key === key ? { ...row, ...patch } : row));
  };

  const updateQuarter = (key: string, quarterIndex: number, value: string) => {
    setDraftEdits((current) => (current ?? baseDraft).map((row) => {
      if (row.key !== key) return row;
      const prices = [...row.prices] as PriceQuarterValues<string>;
      prices[quarterIndex] = value;
      return { ...row, prices };
    }));
  };

  const addRow = () => {
    setDraftEdits((current) => [
      ...(current ?? baseDraft),
      createEmptyQuarterlyPriceDraft(`new-${Date.now()}`),
    ]);
  };

  const removeRow = (row: QuarterlyPriceDraft) => {
    setDraftEdits((current) => (current ?? baseDraft).filter((item) => item.key !== row.key));
    if (row.deleteRows.length > 0) {
      setDeletedRows((current) => [...new Map(
        [...current, ...row.deleteRows].map((item) => [item.id, item]),
      ).values()]);
    }
  };

  const invalidRows = useMemo(() => draft.filter((row) => {
    const invalidPrice = row.prices.some((value) => {
      const price = parseNumberInput(value);
      return price == null || price <= 0;
    });
    return row.brand.trim() === '' || row.sku.trim() === '' || invalidPrice;
  }), [draft]);

  const save = () => {
    mutation.mutate({ year, rows: quarterlyPriceInputs(draft, year), deleted_rows: deletedRows });
  };

  if (query.isLoading) return <Box sx={{ p: 4, textAlign: 'center' }}><CircularProgress /></Box>;
  if (query.isError || !query.data) return <Alert severity="error">Не удалось загрузить цены контракта.</Alert>;

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
        <Box>
          <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>Цены контракта · {year}</Typography>
          <Typography variant="caption" color="text.secondary">
            По умолчанию: OLAP SS, общая цена SKU по всем сетям за последний доступный месяц 2026 года
          </Typography>
        </Box>
        <Box sx={{ flex: 1 }} />
        {canEdit && <Button size="small" startIcon={<AddIcon />} onClick={addRow}>Добавить SKU</Button>}
        {dirty && <Typography variant="caption" color="warning.main">Есть несохранённые изменения</Typography>}
        {canEdit && (
          <Button
            variant="contained"
            size="small"
            startIcon={<SaveIcon />}
            disabled={!dirty || invalidRows.length > 0 || mutation.isPending}
            onClick={save}
          >
            Сохранить цены
          </Button>
        )}
      </Box>

      {mutation.isError && <Alert severity="error">{(mutation.error as Error).message}</Alert>}
      {invalidRows.length > 0 && <Alert severity="warning">Заполните бренд, SKU и положительную цену для каждого квартала.</Alert>}

      <Paper variant="outlined" sx={{ overflowX: 'auto' }}>
        <Table size="small" sx={{ minWidth: 1280, tableLayout: 'fixed' }}>
          <TableHead>
            <TableRow>
              <TableCell sx={{ width: 180 }}>Бренд</TableCell>
              <TableCell sx={{ width: 360 }}>SKU</TableCell>
              {[1, 2, 3, 4].map((quarter) => (
                <TableCell key={quarter} align="right" sx={{ width: 130 }}>Q{quarter}</TableCell>
              ))}
              <TableCell align="right" sx={{ width: 150 }}>Цена OLAP SS</TableCell>
              <TableCell sx={{ width: 190 }}>Подтверждение цен</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {draft.map((row) => {
              return (
                <TableRow key={row.key} hover>
                  <TableCell sx={{ verticalAlign: 'top' }}>
                    {row.manualNew ? (
                      <TextField
                        size="small"
                        fullWidth
                        value={row.brand}
                        disabled={!canEdit}
                        onChange={(event) => updateRow(row.key, { brand: event.target.value })}
                      />
                    ) : (
                      <Typography variant="body2" sx={{ whiteSpace: 'normal', overflowWrap: 'anywhere' }}>
                        {row.brand}
                      </Typography>
                    )}
                  </TableCell>
                  <TableCell sx={{ verticalAlign: 'top' }}>
                    <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 0.5 }}>
                      {row.manualNew ? (
                        <TextField
                          size="small"
                          fullWidth
                          multiline
                          minRows={2}
                          value={row.sku}
                          disabled={!canEdit}
                          onChange={(event) => updateRow(row.key, { sku: event.target.value })}
                        />
                      ) : (
                        <Typography
                          variant="body2"
                          sx={{ flex: 1, whiteSpace: 'normal', overflowWrap: 'anywhere', lineHeight: 1.4 }}
                        >
                          {row.sku}
                        </Typography>
                      )}
                      {canEdit && row.canDelete && (
                        <Tooltip title={row.manualNew ? 'Убрать строку' : 'Удалить созданный вручную SKU'}>
                          <IconButton
                            size="small"
                            color="error"
                            disabled={mutation.isPending}
                            aria-label={`Удалить SKU ${row.sku || 'без названия'}`}
                            onClick={() => removeRow(row)}
                          >
                            <DeleteIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                      )}
                    </Box>
                  </TableCell>
                  {row.prices.map((price, quarterIndex) => (
                    <TableCell key={quarterIndex} align="right" sx={{ verticalAlign: 'top' }}>
                      <TextField
                        size="small"
                        value={price}
                        disabled={!canEdit}
                        onChange={(event) => updateQuarter(row.key, quarterIndex, event.target.value)}
                        sx={{ width: 112 }}
                        slotProps={{ htmlInput: { inputMode: 'decimal', style: { textAlign: 'right' } } }}
                      />
                    </TableCell>
                  ))}
                  <TableCell align="right">
                    <Tooltip title={row.olapYear && row.olapMonth ? `${MONTHS[row.olapMonth - 1]} ${row.olapYear}` : ''}>
                      <span>{row.olapPrice == null ? '—' : `${formatRub(row.olapPrice, 2)} ₽`}</span>
                    </Tooltip>
                  </TableCell>
                  <TableCell>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                      <Switch
                        size="small"
                        checked={row.confirmed}
                        disabled={!canEdit}
                        onChange={(event) => updateRow(row.key, { confirmed: event.target.checked })}
                      />
                      <Chip
                        size="small"
                        label={row.confirmed ? 'да' : 'проверить'}
                        color={row.confirmed ? 'success' : 'warning'}
                        variant="outlined"
                      />
                    </Box>
                  </TableCell>
                </TableRow>
              );
            })}
            {draft.length === 0 && (
              <TableRow>
                <TableCell colSpan={8}>
                  <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
                    В OLAP SS за последний доступный месяц 2026 года не найдено пар ₽/уп. по SKU. Добавьте цену вручную.
                  </Typography>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Paper>

      <Typography variant="caption" color="text.secondary">
        По умолчанию одна цена OLAP SS заполняет Q1–Q4. КАМ может изменить цену отдельно для любого квартала; подтверждение применяется ко всем четырём ценам SKU.
      </Typography>
    </Box>
  );
}
