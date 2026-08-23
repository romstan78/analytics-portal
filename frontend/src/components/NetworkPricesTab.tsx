import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
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
import { Add as AddIcon, Save as SaveIcon } from '@mui/icons-material';
import { networkAPI } from '../api/networks';
import type { NetworkContractPrice, NetworkContractPriceInput, NetworkPricesSaveRequest } from '../types/network';
import { formatNumberInput, formatRub, parseNumberInput } from '../utils/networkPlan';

interface Props {
  networkId: number;
  year: number;
  canEdit: boolean;
}

interface PriceDraft {
  key: string;
  id: number;
  brand: string;
  sku: string;
  price: string;
  validFrom: string;
  validTo: string;
  confirmed: boolean;
  updatedAt: string;
  sourceType: string;
  sourceYear: number | null;
  sourceMonth: number | null;
  olapPrice: number | null;
  olapYear: number | null;
  olapMonth: number | null;
}

const MONTHS = [
  'январь', 'февраль', 'март', 'апрель', 'май', 'июнь',
  'июль', 'август', 'сентябрь', 'октябрь', 'ноябрь', 'декабрь',
];

const draftOf = (row: NetworkContractPrice): PriceDraft => ({
  key: String(row.id),
  id: row.id,
  brand: row.brand_as,
  sku: row.sku,
  price: formatNumberInput(String(row.contract_price)),
  validFrom: row.valid_from,
  validTo: row.valid_to,
  confirmed: row.is_confirmed,
  updatedAt: row.updated_at,
  sourceType: row.source_type,
  sourceYear: row.source_year,
  sourceMonth: row.source_month,
  olapPrice: row.olap_price,
  olapYear: row.olap_year,
  olapMonth: row.olap_month,
});

export default function NetworkPricesTab({ networkId, year, canEdit }: Props) {
  const [draftEdits, setDraftEdits] = useState<PriceDraft[] | null>(null);
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: ['network-prices', networkId, year],
    queryFn: () => networkAPI.getPrices(networkId, year),
  });

  const baseDraft = useMemo(() => (query.data?.data ?? []).map(draftOf), [query.data]);
  const draft = draftEdits ?? baseDraft;
  const dirty = draftEdits != null;

  const mutation = useMutation({
    mutationFn: (request: NetworkPricesSaveRequest) => networkAPI.savePrices(networkId, request),
    onSuccess: (response) => {
      queryClient.setQueryData(['network-prices', networkId, year], response.data);
      void queryClient.invalidateQueries({ queryKey: ['network-forecast', networkId, year] });
      void queryClient.invalidateQueries({ queryKey: ['networkAudit', networkId] });
      setDraftEdits(null);
    },
  });

  const updateRow = (key: string, patch: Partial<PriceDraft>) => {
    setDraftEdits((current) => (current ?? baseDraft).map((row) => row.key === key ? { ...row, ...patch } : row));
  };

  const addRow = () => {
    setDraftEdits((current) => [...(current ?? baseDraft), {
      key: `new-${Date.now()}`,
      id: 0,
      brand: '',
      sku: '',
      price: '',
      validFrom: `${year}-01-01`,
      validTo: `${year}-12-31`,
      confirmed: true,
      updatedAt: '',
      sourceType: 'manual',
      sourceYear: null,
      sourceMonth: null,
      olapPrice: null,
      olapYear: null,
      olapMonth: null,
    }]);
  };

  const invalidRows = useMemo(() => draft.filter((row) => {
    const price = parseNumberInput(row.price);
    return row.brand.trim() === '' || row.sku.trim() === '' || price == null || price <= 0
      || row.validFrom === '' || row.validTo === '' || row.validFrom > row.validTo;
  }), [draft]);

  const save = () => {
    const rows: NetworkContractPriceInput[] = draft.map((row) => ({
      id: row.id,
      brand_as: row.brand.trim(),
      sku: row.sku.trim(),
      contract_price: parseNumberInput(row.price) ?? 0,
      valid_from: row.validFrom,
      valid_to: row.validTo,
      is_confirmed: row.confirmed,
      updated_at: row.updatedAt,
    }));
    mutation.mutate({ year, rows });
  };

  if (query.isLoading) return <Box sx={{ p: 4, textAlign: 'center' }}><CircularProgress /></Box>;
  if (query.isError || !query.data) return <Alert severity="error">Не удалось загрузить цены контракта.</Alert>;

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
        <Box>
          <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>Цены контракта · {year}</Typography>
          <Typography variant="caption" color="text.secondary">
            Первичные цены: рубли / упаковки за последний доступный месяц 2026 года; OLAP ниже — за выбранный год
          </Typography>
        </Box>
        <Box sx={{ flex: 1 }} />
        {canEdit && <Button size="small" startIcon={<AddIcon />} onClick={addRow}>Добавить SKU или период</Button>}
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
      {invalidRows.length > 0 && <Alert severity="warning">Заполните бренд, SKU, положительную цену и корректный период.</Alert>}

      <Paper variant="outlined" sx={{ overflowX: 'auto' }}>
        <Table size="small" sx={{ minWidth: 1050 }}>
          <TableHead>
            <TableRow>
              <TableCell>Бренд</TableCell>
              <TableCell>SKU</TableCell>
              <TableCell>Цена контракта, ₽</TableCell>
              <TableCell>Действует с</TableCell>
              <TableCell>Действует по</TableCell>
              <TableCell align="right">Цена OLAP</TableCell>
              <TableCell>Источник</TableCell>
              <TableCell align="right">Разница</TableCell>
              <TableCell>Подтверждена</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {draft.map((row) => {
              const price = parseNumberInput(row.price);
              const difference = price != null && row.olapPrice != null && row.olapPrice !== 0
                ? ((price - row.olapPrice) / row.olapPrice) * 100
                : null;
              const source = row.sourceType === 'olap_seed' && row.sourceYear && row.sourceMonth
                ? `OLAP · ${MONTHS[row.sourceMonth - 1]} ${row.sourceYear}`
                : row.sourceType === 'contract_import' ? 'Таблица контрактов' : 'Вручную';
              return (
                <TableRow key={row.key} hover>
                  <TableCell>
                    <TextField
                      size="small"
                      value={row.brand}
                      disabled={!canEdit || row.id > 0}
                      onChange={(event) => updateRow(row.key, { brand: event.target.value })}
                      sx={{ minWidth: 140 }}
                    />
                  </TableCell>
                  <TableCell>
                    <TextField
                      size="small"
                      value={row.sku}
                      disabled={!canEdit || row.id > 0}
                      onChange={(event) => updateRow(row.key, { sku: event.target.value })}
                      sx={{ minWidth: 160 }}
                    />
                  </TableCell>
                  <TableCell>
                    <TextField
                      size="small"
                      value={row.price}
                      disabled={!canEdit}
                      onChange={(event) => updateRow(row.key, { price: event.target.value })}
                      sx={{ width: 135 }}
                      slotProps={{ htmlInput: { inputMode: 'decimal' } }}
                    />
                  </TableCell>
                  <TableCell>
                    <TextField
                      size="small"
                      type="date"
                      value={row.validFrom}
                      disabled={!canEdit}
                      onChange={(event) => updateRow(row.key, { validFrom: event.target.value })}
                      sx={{ width: 145 }}
                    />
                  </TableCell>
                  <TableCell>
                    <TextField
                      size="small"
                      type="date"
                      value={row.validTo}
                      disabled={!canEdit}
                      onChange={(event) => updateRow(row.key, { validTo: event.target.value })}
                      sx={{ width: 145 }}
                    />
                  </TableCell>
                  <TableCell align="right">
                    <Tooltip title={row.olapYear && row.olapMonth ? `${MONTHS[row.olapMonth - 1]} ${row.olapYear}` : ''}>
                      <span>{row.olapPrice == null ? '—' : `${formatRub(row.olapPrice, 2)} ₽`}</span>
                    </Tooltip>
                  </TableCell>
                  <TableCell><Typography variant="caption" color="text.secondary">{source}</Typography></TableCell>
                  <TableCell align="right">
                    <Typography
                      variant="body2"
                      color={difference != null && Math.abs(difference) >= 5 ? 'warning.main' : 'text.primary'}
                      sx={{ fontWeight: 600 }}
                    >
                      {difference == null ? '—' : `${difference >= 0 ? '+' : ''}${difference.toLocaleString('ru-RU', { maximumFractionDigits: 1 })}%`}
                    </Typography>
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
                <TableCell colSpan={9}>
                  <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
                    В OLAP за {year} год не найдено пар ₽/уп. для этой сети. Добавьте цену вручную.
                  </Typography>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Paper>

      <Typography variant="caption" color="text.secondary">
        Изменение цены создаётся новым непересекающимся периодом. OLAP-цена показывается для сравнения и не перезаписывает подтверждённую контрактную цену.
      </Typography>
    </Box>
  );
}
