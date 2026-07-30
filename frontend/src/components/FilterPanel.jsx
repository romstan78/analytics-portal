import {
  TextField, Stack, Autocomplete,
  FormControlLabel, Checkbox, ListItemText, Button, Box
} from '@mui/material';

const DEFAULT_MONTH_OPTIONS = [
  { label: 'Январь', value: 1 }, { label: 'Февраль', value: 2 },
  { label: 'Март', value: 3 }, { label: 'Апрель', value: 4 },
  { label: 'Май', value: 5 }, { label: 'Июнь', value: 6 },
  { label: 'Июль', value: 7 }, { label: 'Август', value: 8 },
  { label: 'Сентябрь', value: 9 }, { label: 'Октябрь', value: 10 },
  { label: 'Ноябрь', value: 11 }, { label: 'Декабрь', value: 12 },
];

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
}) {

  const handleTextChange = (field) => (e) => onFiltersChange({ ...filters, [field]: e.target.value });
  const handleArrayChange = (field) => (_, newValue) => onFiltersChange({ ...filters, [field]: newValue });

  const renderCheckboxOption = (props, option, { selected }) => {
    const { key, item, ...rest } = props;
    return (
      <li key={key} {...rest} style={{ padding: '2px 8px' }}>
        <Checkbox size="small" checked={selected} sx={{ mr: 1 }} />
        <ListItemText 
          primary={option?.label ?? option} 
          primaryTypographyProps={{ fontSize: 13 }} 
        />
      </li>
    );
  };

  const filterKeys = visibleFilters || Object.keys(filterOptions);

  const defaultLabels = {
    brandName: 'Бренд', brand: 'Бренд', networkName: 'Сеть', network_name: 'Сеть',
    un_rub: 'Уп/Руб', segment: 'Сегмент', channel: 'Канал',
    productName: 'Продукт', metricType: 'Показатель', sku: 'SKU',
    mechanics: 'Механика', status: 'Статус', kam: 'KAM', gtn_opex: 'GTN/OPEX',
  };

  const getLabel = (key) => labels[key] || defaultLabels[key] || key;

  return (
    <Stack spacing={1.5}>
      {/* Строка фильтров */}
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center', flexWrap: 'wrap' }}>

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
                value={filters[filter.field] || ''} 
                onChange={handleTextChange(filter.field)}
                sx={{ width: 90 }} 
                slotProps={{ htmlInput: { min: 2018, max: 2030 } }} 
              />
            );
          }

          // Месяцы
          if (filter.type === 'months') {
            const selectedMonths = filters[filter.field] || [];
            const monthOptions = filter.options || DEFAULT_MONTH_OPTIONS;
            
            const monthDisplayText = selectedMonths.length === 0 
              ? '' 
              : selectedMonths.length === 1 
                ? monthOptions.find(m => m.value === selectedMonths[0])?.label || ''
                : `Выбрано: ${selectedMonths.length}`;

            return (
              <Autocomplete 
                key={filter.field} 
                multiple 
                disableCloseOnSelect 
                size="small"
                options={monthOptions}
                getOptionLabel={(opt) => opt.label}
                isOptionEqualToValue={(opt, val) => opt.value === val?.value}
                value={monthOptions.filter(m => selectedMonths.includes(m.value))}
                onChange={(_, newVal) => {
                  const values = newVal.map(v => v.value);
                  onFiltersChange({ ...filters, [filter.field]: values });
                }}
                renderTags={() => null}
                renderOption={renderCheckboxOption}
                renderInput={(params) => (
                  <TextField 
                    {...params} 
                    label={filter.label} 
                    placeholder={monthDisplayText}
                    InputLabelProps={{ shrink: true }} 
                  />
                )}
                slotProps={{ 
                  listbox: { style: { maxHeight: 300 } }, 
                  paper: { sx: { minWidth: 300 } } 
                }}
                sx={{ minWidth: 170, '& .MuiAutocomplete-tag': { display: 'none' } }} 
              />
            );
          }

          return null;
        })}

        {/* Остальные фильтры */}
        {filterKeys.map((key) => {
          const options = filterOptions[key];
          if (!options || options.length === 0) return null;

          const selected = filters[key] || [];
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
              renderTags={() => null}
              limitTags={0}
              renderInput={(params) => (
                <TextField
                  {...params}
                  label={getLabel(key)}
                  placeholder={displayText}
                  InputLabelProps={{ shrink: true }}
                />
              )}
              slotProps={{ listbox: { style: { maxHeight: 300 } }, paper: { sx: { minWidth: 350 } } }}
              sx={{ minWidth: 170, '& .MuiAutocomplete-tag': { display: 'none' } }} />
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
              <Checkbox 
                size="small" 
                checked={persistFilters} 
                onChange={(e) => onPersistChange(e.target.checked)} 
              />
            }
            label="Сохранять" 
            sx={{ ml: 1, '& .MuiTypography-root': { fontSize: 13 } }} 
          />
        )}
      </Stack>
    </Stack>
  );
}