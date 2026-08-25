-- +goose Up
-- Уровень ведения бренда (бренд / SKU) и единица ввода (рубли / упаковки).
--
-- Режим — свойство бренда в квартале, а не режим формы: в одной сети часть
-- брендов ведут в рублях по бренду, часть — в упаковках по SKU. Поэтому он
-- живёт на строке квартального плана и один и тот же для вкладок «Планы»
-- и «Прогноз»: обе формы показывают один бренд одинаково.
--
-- entry_level = 'sku'  → введённые значения хранятся в SKU-строках, строка
--                        бренда расчётная и равна их сумме;
-- entry_level = 'brand'→ введена строка бренда, SKU-разложение расчётное.
-- entry_unit  задаёт, какая из двух метрик введена, а какая пересчитана
--             по цене контракта.
-- На SKU-строках плана эти поля не используются: режим един для бренда.

IF COL_LENGTH('dbo.tbl_NetworkPlans', 'entry_level') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD entry_level NVARCHAR(10) NOT NULL
        CONSTRAINT DF_NetworkPlans_entry_level DEFAULT 'brand';
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'entry_unit') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD entry_unit NVARCHAR(10) NOT NULL
        CONSTRAINT DF_NetworkPlans_entry_unit DEFAULT 'rub';

IF NOT EXISTS (
    SELECT 1 FROM sys.check_constraints
    WHERE name = 'CK_NetworkPlans_entry_mode'
      AND parent_object_id = OBJECT_ID('dbo.tbl_NetworkPlans')
)
    ALTER TABLE dbo.tbl_NetworkPlans WITH CHECK ADD
        CONSTRAINT CK_NetworkPlans_entry_mode CHECK (
            entry_level IN ('brand', 'sku') AND entry_unit IN ('rub', 'units')
        );

-- Режим по умолчанию для брендов, которых в плане ещё нет: у каждого КАМа
-- своя привычка, и сеть должна открываться сразу в ней.
IF COL_LENGTH('dbo.tbl_Networks', 'default_entry_level') IS NULL
    ALTER TABLE dbo.tbl_Networks ADD default_entry_level NVARCHAR(10) NOT NULL
        CONSTRAINT DF_Networks_default_entry_level DEFAULT 'brand';
IF COL_LENGTH('dbo.tbl_Networks', 'default_entry_unit') IS NULL
    ALTER TABLE dbo.tbl_Networks ADD default_entry_unit NVARCHAR(10) NOT NULL
        CONSTRAINT DF_Networks_default_entry_unit DEFAULT 'rub';

IF NOT EXISTS (
    SELECT 1 FROM sys.check_constraints
    WHERE name = 'CK_Networks_default_entry_mode'
      AND parent_object_id = OBJECT_ID('dbo.tbl_Networks')
)
    ALTER TABLE dbo.tbl_Networks WITH CHECK ADD
        CONSTRAINT CK_Networks_default_entry_mode CHECK (
            default_entry_level IN ('brand', 'sku') AND default_entry_unit IN ('rub', 'units')
        );

-- Как бренд ведут на самом деле, видно по уже сохранённому прогнозу: если у
-- бренда есть SKU-строки, значит его детализируют, а заполненные упаковки
-- означают, что вводят именно их. Так после обновления никто не увидит свой
-- бренд переключённым в чужой режим.
UPDATE p
   SET p.entry_level = 'sku',
       p.entry_unit = CASE WHEN detail.has_units = 1 THEN 'units' ELSE p.entry_unit END
FROM dbo.tbl_NetworkPlans p
CROSS APPLY (
    SELECT
        MAX(CASE WHEN f.forecast_units IS NOT NULL THEN 1 ELSE 0 END) AS has_units,
        COUNT(*) AS rows_count
    FROM dbo.tbl_NetworkForecasts f
    WHERE f.network_id = p.network_id
      AND f.[year] = p.[year]
      AND f.[month] BETWEEN (p.[quarter] - 1) * 3 + 1 AND (p.[quarter] - 1) * 3 + 3
      AND f.brand_as = p.brand_as
      AND f.sku IS NOT NULL
      AND (f.forecast_rub IS NOT NULL OR f.forecast_units IS NOT NULL)
) detail
WHERE p.brand_as IS NOT NULL
  AND detail.rows_count > 0;

-- Бренд без детализации тоже мог вестись в упаковках: тогда в его собственных
-- строках прогноза заполнены упаковки, а рубли пусты. Единица ввода теперь
-- решает, какая метрика является введённой, поэтому такой бренд нужно отметить —
-- иначе его прогноз был бы отброшен как неведущая метрика.
UPDATE p
   SET p.entry_unit = 'units'
FROM dbo.tbl_NetworkPlans p
CROSS APPLY (
    SELECT
        SUM(CASE WHEN f.forecast_units IS NOT NULL THEN 1 ELSE 0 END) AS units_rows,
        SUM(CASE WHEN f.forecast_rub IS NOT NULL THEN 1 ELSE 0 END) AS rub_rows
    FROM dbo.tbl_NetworkForecasts f
    WHERE f.network_id = p.network_id
      AND f.[year] = p.[year]
      AND f.[month] BETWEEN (p.[quarter] - 1) * 3 + 1 AND (p.[quarter] - 1) * 3 + 3
      AND f.brand_as = p.brand_as
      AND f.sku IS NULL
) own_rows
WHERE p.brand_as IS NOT NULL
  AND p.entry_level = 'brand'
  AND own_rows.units_rows > 0
  AND own_rows.rub_rows = 0;

-- Сеть, где так ведут все бренды, получает тот же режим по умолчанию.
UPDATE n
   SET n.default_entry_level = 'sku',
       n.default_entry_unit = CASE WHEN mode.units_brands = mode.sku_brands THEN 'units' ELSE n.default_entry_unit END
FROM dbo.tbl_Networks n
CROSS APPLY (
    SELECT
        COUNT(*) AS total_brands,
        SUM(CASE WHEN p.entry_level = 'sku' THEN 1 ELSE 0 END) AS sku_brands,
        SUM(CASE WHEN p.entry_unit = 'units' THEN 1 ELSE 0 END) AS units_brands
    FROM dbo.tbl_NetworkPlans p
    WHERE p.network_id = n.id AND p.brand_as IS NOT NULL
) mode
WHERE mode.total_brands > 0 AND mode.sku_brands = mode.total_brands;

IF EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_NetworkPlans_network_year'
      AND object_id = OBJECT_ID('dbo.tbl_NetworkPlans')
)
    DROP INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans;

CREATE INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans(network_id, [year])
    INCLUDE ([quarter], brand_as, in_gross, plan_rub, plan_units, fact_rub, forecast_rub,
             investments_pct, fact_investments_rub, forecast_investments_rub,
             month1_pct, month2_pct, month3_pct, entry_level, entry_unit);

-- +goose Down
IF EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_NetworkPlans_network_year'
      AND object_id = OBJECT_ID('dbo.tbl_NetworkPlans')
)
    DROP INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans;

IF EXISTS (
    SELECT 1 FROM sys.check_constraints
    WHERE name = 'CK_Networks_default_entry_mode'
      AND parent_object_id = OBJECT_ID('dbo.tbl_Networks')
)
    ALTER TABLE dbo.tbl_Networks DROP CONSTRAINT CK_Networks_default_entry_mode;
IF EXISTS (
    SELECT 1 FROM sys.check_constraints
    WHERE name = 'CK_NetworkPlans_entry_mode'
      AND parent_object_id = OBJECT_ID('dbo.tbl_NetworkPlans')
)
    ALTER TABLE dbo.tbl_NetworkPlans DROP CONSTRAINT CK_NetworkPlans_entry_mode;

IF EXISTS (SELECT 1 FROM sys.default_constraints WHERE name = 'DF_Networks_default_entry_unit')
    ALTER TABLE dbo.tbl_Networks DROP CONSTRAINT DF_Networks_default_entry_unit;
IF EXISTS (SELECT 1 FROM sys.default_constraints WHERE name = 'DF_Networks_default_entry_level')
    ALTER TABLE dbo.tbl_Networks DROP CONSTRAINT DF_Networks_default_entry_level;
IF EXISTS (SELECT 1 FROM sys.default_constraints WHERE name = 'DF_NetworkPlans_entry_unit')
    ALTER TABLE dbo.tbl_NetworkPlans DROP CONSTRAINT DF_NetworkPlans_entry_unit;
IF EXISTS (SELECT 1 FROM sys.default_constraints WHERE name = 'DF_NetworkPlans_entry_level')
    ALTER TABLE dbo.tbl_NetworkPlans DROP CONSTRAINT DF_NetworkPlans_entry_level;

IF COL_LENGTH('dbo.tbl_Networks', 'default_entry_unit') IS NOT NULL
    ALTER TABLE dbo.tbl_Networks DROP COLUMN default_entry_unit;
IF COL_LENGTH('dbo.tbl_Networks', 'default_entry_level') IS NOT NULL
    ALTER TABLE dbo.tbl_Networks DROP COLUMN default_entry_level;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'entry_unit') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN entry_unit;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'entry_level') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN entry_level;

CREATE INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans(network_id, [year])
    INCLUDE ([quarter], brand_as, in_gross, plan_rub, plan_units, fact_rub, forecast_rub,
             investments_pct, fact_investments_rub, forecast_investments_rub,
             month1_pct, month2_pct, month3_pct);
