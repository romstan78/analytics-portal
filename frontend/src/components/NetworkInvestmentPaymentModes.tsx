// Компактная матрица режима оплаты инвестиций в профиле сети.

import { useMemo, useState } from 'react';
import {
  Box,
  Button,
  Paper,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material';
import { PaymentsOutlined as PaymentsIcon, Save as SaveIcon } from '@mui/icons-material';
import type { NetworkInvestmentPaymentModesSaveRequest, NetworkPlan } from '../types/network';
import { QUARTERS, planKey } from '../utils/networkPlan';

interface Props {
  year: number;
  plans: NetworkPlan[];
  canEdit: boolean;
  saving: boolean;
  onSave: (request: NetworkInvestmentPaymentModesSaveRequest) => void;
}

const buildValues = (plans: NetworkPlan[]): Record<string, boolean> => Object.fromEntries(
  plans
    .filter((plan) => plan.brand_as != null)
    .map((plan) => [planKey(plan.quarter, plan.brand_as), plan.pay_investments_from_fact]),
);

export default function NetworkInvestmentPaymentModes({
  year,
  plans,
  canEdit,
  saving,
  onSave,
}: Props) {
  const [loadedPlans, setLoadedPlans] = useState(plans);
  const [values, setValues] = useState<Record<string, boolean>>(() => buildValues(plans));
  const [dirty, setDirty] = useState(false);

  if (loadedPlans !== plans) {
    setLoadedPlans(plans);
    setValues(buildValues(plans));
    setDirty(false);
  }

  const plansByKey = useMemo(
    () => new Map(plans.map((plan) => [planKey(plan.quarter, plan.brand_as), plan])),
    [plans],
  );
  const brands = useMemo(() => [...new Set(
    plans.filter((plan) => plan.brand_as != null).map((plan) => plan.brand_as as string),
  )].sort((a, b) => a.localeCompare(b, 'ru')), [plans]);

  const save = () => onSave({
    year,
    rows: plans
      .filter((plan) => plan.brand_as != null)
      .map((plan) => ({
        quarter: plan.quarter,
        brand_as: plan.brand_as as string,
        pay_investments_from_fact: values[planKey(plan.quarter, plan.brand_as)] ?? false,
        updated_at: plan.updated_at,
      })),
  });

  return (
    <Paper variant="outlined" sx={{ overflow: 'hidden' }}>
      <Box sx={{ px: 1.5, py: 1.25, display: 'flex', gap: 1, alignItems: 'center', flexWrap: 'wrap' }}>
        <PaymentsIcon fontSize="small" color="action" />
        <Box sx={{ minWidth: 0 }}>
          <Typography variant="subtitle2">Оплата от факта · {year}</Typography>
          <Typography variant="caption" color="text.secondary">
            Включено: факт × %, без порога 100%. Выключено: EAC × %, только при выполнении плана периода.
          </Typography>
        </Box>
        <Box sx={{ flex: 1 }} />
        {canEdit && (
          <Button
            size="small"
            startIcon={<SaveIcon />}
            disabled={!dirty || saving || brands.length === 0}
            onClick={save}
          >
            Сохранить
          </Button>
        )}
      </Box>

      {brands.length === 0 ? (
        <Typography variant="body2" color="text.secondary" sx={{ px: 1.5, pb: 1.25 }}>
          Сначала добавьте бренды во вкладке «План и факт».
        </Typography>
      ) : (
        <Table size="small" sx={{ borderTop: 1, borderColor: 'divider', '& th, & td': { py: 0.35 } }}>
          <TableHead>
            <TableRow>
              <TableCell>Бренд</TableCell>
              {QUARTERS.map((quarter) => <TableCell key={quarter} align="center">Q{quarter}</TableCell>)}
            </TableRow>
          </TableHead>
          <TableBody>
            {brands.map((brand) => (
              <TableRow key={brand} hover>
                <TableCell sx={{ fontSize: 13 }}>{brand}</TableCell>
                {QUARTERS.map((quarter) => {
                  const key = planKey(quarter, brand);
                  const exists = plansByKey.has(key);
                  return (
                    <TableCell key={quarter} align="center">
                      {exists ? (
                        <Switch
                          size="small"
                          checked={values[key] ?? false}
                          disabled={!canEdit}
                          slotProps={{
                            input: { 'aria-label': `${brand}, Q${quarter}: оплата от факта` },
                          }}
                          onChange={(event) => {
                            setValues((current) => ({ ...current, [key]: event.target.checked }));
                            setDirty(true);
                          }}
                        />
                      ) : (
                        <Typography color="text.disabled">—</Typography>
                      )}
                    </TableCell>
                  );
                })}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </Paper>
  );
}
