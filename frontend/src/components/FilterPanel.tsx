import type { HTMLAttributes, Key, SyntheticEvent } from 'react';
import {
  TextField, Stack, Autocomplete, Box,
  FormControlLabel, Checkbox, ListItemText, Button, Switch
} from '@mui/material';

// Плотность фильтров держится на уровне остальной страницы: текст 0.875rem,
// как в тулбаре таблицы, а высоту строки списка задаёт текст, а не хитбокс
// чекбокса — иначе семь значений разворачиваются на треть экрана.
const FIELD_FONT_SIZE = '0.875rem';

const FIELD_SX = {
  minWidth: 170,
  '& .MuiAutocomplete-tag': { display: 'none' },
  '& .MuiInputBase-input': { fontSize: FIELD_FONT_SIZE },
};

const YEAR_FIELD_SX = { width: 90, '& .MuiInputBase-input': { fontSize: FIELD_FONT_SIZE } };

// Удвоенный «&» поднимает вес правил: собственные стили Autocomplete приходят
// селектором из двух классов и иначе перебивают отступы и подсветку.
const OPTION_SX = {
  '&&': { gap: 1, px: 1, py: 0.5, borderRadius: '8px' },
  '&&[aria-selected="true"]': { bgcolor: '#eef2ff' },
  '&&[aria-selected="true"].Mui-focused': { bgcolor: '#e0e7ff' },
};

// Невыбранный чекбокс держим в тон рамкам полей, выбранный — в цвет темы.
const OPTION_CHECKBOX_SX = {
  p: 0.375,
  color: '#cbd5e1',
  '&.Mui-checked': { color: 'primary.main' },
  '& .MuiSvgIcon-root': { fontSize: 18 },
};

// Выпадающий список — такая же поверхность, как карточки страницы: светлая
// рамка и мягкая тень вместо почти плоской тени по умолчанию.
const popupSlotProps = (minWidth: number) => ({
  listbox: { sx: { maxHeight: 300, p: 0.5 } },
  paper: {
    sx: {
      minWidth,
      borderRadius: '12px',
      border: '1px solid #e2e8f0',
      boxShadow: '0 10px 15px -3px rgba(15, 23, 42, 0.08), 0 4px 6px -4px rgba(15, 23, 42, 0.08)',
    },
  },
});

interface NumberOption {
  label: string;
  value: number;
}

// filters приходит как Record<string, unknown>, поэтому значения нормализуем.
const asNumberArray = (value: unknown): number[] =>
  Array.isArray(value) ? value.filter((item): item is number => typeof item === 'number') : [];

const asStringArray = (value: unknown): string[] =>
  Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : [];

const DEFAULT_MONTH_OPTIONS = [
  { label: 'Январь', value: 1 }, { label: 'Февраль', value: 2 },
  { label: 'Март', value: 3 }, { label: 'Апрель', value: 4 },
  { label: 'Май', value: 5 }, { label: 'Июнь', value: 6 },
  { label: 'Июль', value: 7 }, { label: 'Август', value: 8 },
  { label: 'Сентябрь', value: 9 }, { label: 'Октябрь', value: 10 },
  { label: 'Ноябрь', value: 11 }, { label: 'Декабрь', value: 12 },
];

const DEFAULT_QUARTER_OPTIONS = [
  { label: 'I квартал', value: 1 }, { label: 'II квартал', value: 2 },
  { label: 'III квартал', value: 3 }, { label: 'IV квартал', value: 4 },
];

export interface ExtraFilter {
  type: 'year' | 'months' | 'quarters';
  field: string;
  label: string;
  options?: Array<{ label: string; value: number }>;
}

interface FilterPanelProps {
  filters: Record<string, unknown>;
  filterOptions?: Record<string, string[]>;
  onFiltersChange: (filters: Record<string, unknown>) => void;
  onSearch: () => void;
  onReset: () => void;
  loading?: boolean;
  extraFilters?: ExtraFilter[];
  persistFilters?: boolean;
  onPersistChange?: ((checked: boolean) => void) | null;
  visibleFilters?: string[] | null;
  labels?: Record<string, string>;
}

export default function FilterPanel({
  filters,
  filterOptions = {},
  onFiltersChange,
  onSearch,
  onReset,
  loading = false,
  extraFilters = [],
  persistFilters = false,
  onPersistChange = null,
  visibleFilters = null,
  labels = {},
}: FilterPanelProps) {

  const handleTextChange = (field: string) => (e: { target: { value: string } }) =>
    onFiltersChange({ ...filters, [field]: e.target.value });
  const handleArrayChange = (field: string) => (_: SyntheticEvent, newValue: string[]) =>
    onFiltersChange({ ...filters, [field]: newValue });

  const renderCheckboxOption = (
    props: HTMLAttributes<HTMLLIElement> & { key?: Key },
    option: string | NumberOption,
    { selected }: { selected: boolean },
  ) => {
    const { key, ...rest } = props;
    return (
      <Box component="li" key={key} {...rest} sx={OPTION_SX}>
        <Checkbox size="small" disableRipple checked={selected} sx={OPTION_CHECKBOX_SX} />
        <ListItemText
          primary={typeof option === 'string' ? option : option.label}
          sx={{ my: 0 }}
          slotProps={{ primary: { sx: { fontSize: FIELD_FONT_SIZE, fontWeight: selected ? 600 : 400 } } }}
        />
      </Box>
    );
  };

  const filterKeys = visibleFilters || Object.keys(filterOptions);

  const defaultLabels: Record<string, string> = {
    brandName: 'Бренд', brand: 'Бренд', networkName: 'Сеть', network_name: 'Сеть',
    un_rub: 'Уп/Руб', segment: 'Сегмент', channel: 'Канал',
    productName: 'Продукт', metricType: 'Показатель', sku: 'SKU',
    mechanics: 'Механика', status: 'Статус', kam: 'KAM', gtn_opex: 'GTN/OPEX',
  };

  const getLabel = (key: string) => labels[key] || defaultLabels[key] || key;

  return (
    <Stack spacing={1.5}>
      {/* Строка фильтров */}
      {/* useFlexGap — иначе при переносе строки фильтров слипаются: отступ
          Stack живёт на margin-left и вертикального зазора не даёт. */}
      <Stack direction="row" spacing={1} useFlexGap sx={{ alignItems: 'center', flexWrap: 'wrap' }}>

        {/* Годы и месяцы — один проход */}
        {extraFilters.map((filter) => {
          // Год
          if (filter.type === 'year') {
            return (
              <TextField 
                key={filter.field} 
                label={filter.label} 
                size="small" 
                type="number"
                value={typeof filters[filter.field] === 'string' || typeof filters[filter.field] === 'number' ? filters[filter.field] : ''}
                onChange={handleTextChange(filter.field)}
                sx={YEAR_FIELD_SX}
                slotProps={{ htmlInput: { min: 2018, max: 2030 } }}
              />
            );
          }

          // Месяцы и кварталы
          if (filter.type === 'months' || filter.type === 'quarters') {
            const selectedValues = asNumberArray(filters[filter.field]);
            const options = filter.options || (filter.type === 'quarters' ? DEFAULT_QUARTER_OPTIONS : DEFAULT_MONTH_OPTIONS);
            
            const displayText = selectedValues.length === 0
              ? '' 
              : selectedValues.length === 1
                ? options.find(option => option.value === selectedValues[0])?.label || ''
                : `Выбрано: ${selectedValues.length}`;

            return (
              <Autocomplete<NumberOption, true, false, false>
                key={filter.field}
                multiple
                disableCloseOnSelect
                size="small"
                options={options}
                getOptionLabel={(opt) => opt.label}
                isOptionEqualToValue={(opt, val) => opt.value === val?.value}
                value={options.filter(option => selectedValues.includes(option.value))}
                onChange={(_, newVal) => {
                  const values = newVal.map(v => v.value);
                  onFiltersChange({ ...filters, [filter.field]: values });
                }}
                renderValue={() => null}
                renderOption={renderCheckboxOption}
                renderInput={(params) => (
                  <TextField 
                    {...params} 
                    label={filter.label} 
                    placeholder={displayText}
                    slotProps={{
                      ...params.slotProps,
                      inputLabel: { ...params.slotProps.inputLabel, shrink: true },
                    }}
                  />
                )}
                slotProps={popupSlotProps(300)}
                sx={FIELD_SX}
              />
            );
          }

          return null;
        })}

        {/* Остальные фильтры */}
        {filterKeys.map((key) => {
          const options = filterOptions[key];
          if (!options || options.length === 0) return null;

          const selected = asStringArray(filters[key]);
          const displayText = selected.length === 0
            ? '' 
            : selected.length === 1 
              ? selected[0] 
              : `Выбрано: ${selected.length}`;

          return (
            <Autocomplete key={key} multiple disableCloseOnSelect size="small"
              options={options} value={selected}
              onChange={handleArrayChange(key)}
              renderOption={renderCheckboxOption}
              renderValue={() => null}
              limitTags={0}
              renderInput={(params) => (
                <TextField
                  {...params}
                  label={getLabel(key)}
                  placeholder={displayText}
                  slotProps={{
                    ...params.slotProps,
                    inputLabel: { ...params.slotProps.inputLabel, shrink: true },
                  }}
                />
              )}
              slotProps={popupSlotProps(350)}
              sx={FIELD_SX} />
          );
        })}
      </Stack>

      {/* Кнопки */}
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
        <Button variant="contained" onClick={onSearch} disabled={loading} size="small">
          {loading ? '...' : 'Применить'}
        </Button>
        <Button variant="outlined" onClick={onReset} disabled={loading} size="small">
          Сброс
        </Button>

        {onPersistChange && (
          <FormControlLabel
            control={
              <Switch
                size="small"
                checked={persistFilters}
                onChange={(e) => onPersistChange(e.target.checked)}
              />
            }
            label="Сохранять"
            sx={{ ml: 1, '& .MuiTypography-root': { fontSize: FIELD_FONT_SIZE } }}
          />
        )}
      </Stack>
    </Stack>
  );
}
