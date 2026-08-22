-- +goose Up
-- Валовый объём — свойство бренда, а не всего контракта: в одном квартале часть
-- брендов может входить в общий объём, часть планироваться отдельно. Признак
-- переезжает с квартала (tbl_NetworkPeriods.contract_type) на строку плана
-- (tbl_NetworkPlans.in_gross). Заодно появляются факт и прогноз объёма:
-- форма используется не только для планирования, но и для регулярного
-- сравнения плана, факта и прогноза с пересчётом инвестиций.

-- Признак участия бренда в валовом объёме квартала.
-- Строка общего объёма (brand_as IS NULL) сама в пул не входит — у неё in_gross = 0.
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'in_gross') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans
        ADD in_gross BIT NOT NULL CONSTRAINT DF_NetworkPlans_in_gross DEFAULT 0;

-- Факт заполняется загрузкой отгрузок, в интерфейсе доступен только для чтения.
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'fact_rub') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD fact_rub DECIMAL(18,2) NULL;

-- Прогноз объёма вводит КАМ; инвестиции от прогноза считаются тем же процентом.
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'forecast_rub') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD forecast_rub DECIMAL(18,2) NULL;

-- +goose StatementBegin
-- Перенос данных: в кварталах с валовым контрактом все бренды считались частью
-- общего объёма — этот же смысл сохраняем в in_gross.
IF COL_LENGTH('dbo.tbl_NetworkPeriods', 'contract_type') IS NOT NULL
BEGIN
    EXEC sp_executesql N'
        UPDATE p SET p.in_gross = 1
        FROM dbo.tbl_NetworkPlans p
        JOIN dbo.tbl_NetworkPeriods pe
          ON pe.network_id = p.network_id
         AND pe.[year] = p.[year]
         AND pe.[quarter] = p.[quarter]
        WHERE p.brand_as IS NOT NULL AND pe.contract_type = ''gross''';
END;
-- +goose StatementEnd

-- +goose StatementBegin
-- Тип контракта на уровне квартала больше не источник истины — снимаем его
-- вместе с ограничениями, имена которых в старых базах могли быть сгенерированы.
IF COL_LENGTH('dbo.tbl_NetworkPeriods', 'contract_type') IS NOT NULL
BEGIN
    DECLARE @drop NVARCHAR(MAX) = N'';

    SELECT @drop = @drop + N'ALTER TABLE dbo.tbl_NetworkPeriods DROP CONSTRAINT ' + QUOTENAME(dc.name) + N';'
    FROM sys.default_constraints dc
    JOIN sys.columns c ON c.object_id = dc.parent_object_id AND c.column_id = dc.parent_column_id
    WHERE dc.parent_object_id = OBJECT_ID('dbo.tbl_NetworkPeriods') AND c.name = 'contract_type';

    SELECT @drop = @drop + N'ALTER TABLE dbo.tbl_NetworkPeriods DROP CONSTRAINT ' + QUOTENAME(cc.name) + N';'
    FROM sys.check_constraints cc
    WHERE cc.parent_object_id = OBJECT_ID('dbo.tbl_NetworkPeriods')
      AND cc.definition LIKE '%contract_type%';

    IF @drop <> N'' EXEC sp_executesql @drop;

    EXEC sp_executesql N'ALTER TABLE dbo.tbl_NetworkPeriods DROP COLUMN contract_type';
END;
-- +goose StatementEnd

-- Покрывающий индекс перестраиваем: сетка читает признак валового объёма,
-- факт и прогноз вместе с планом.
IF EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_NetworkPlans_network_year' AND object_id = OBJECT_ID('dbo.tbl_NetworkPlans'))
    DROP INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans;
CREATE INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans(network_id, [year])
    INCLUDE ([quarter], brand_as, in_gross, plan_rub, fact_rub, forecast_rub, investments_pct);

-- +goose Down
IF EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_NetworkPlans_network_year' AND object_id = OBJECT_ID('dbo.tbl_NetworkPlans'))
    DROP INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans;

IF COL_LENGTH('dbo.tbl_NetworkPeriods', 'contract_type') IS NULL
    ALTER TABLE dbo.tbl_NetworkPeriods
        ADD contract_type NVARCHAR(20) NOT NULL CONSTRAINT DF_NetworkPeriods_contract DEFAULT 'regular';

-- +goose StatementBegin
-- Квартал считается валовым, если в нём был хотя бы один бренд в общем объёме.
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'in_gross') IS NOT NULL
BEGIN
    EXEC sp_executesql N'
        UPDATE pe SET pe.contract_type = ''gross''
        FROM dbo.tbl_NetworkPeriods pe
        WHERE EXISTS (
            SELECT 1 FROM dbo.tbl_NetworkPlans p
            WHERE p.network_id = pe.network_id
              AND p.[year] = pe.[year]
              AND p.[quarter] = pe.[quarter]
              AND p.in_gross = 1)';
END;
-- +goose StatementEnd

IF NOT EXISTS (SELECT 1 FROM sys.check_constraints WHERE name = 'CK_NetworkPeriods_contract' AND parent_object_id = OBJECT_ID('dbo.tbl_NetworkPeriods'))
    ALTER TABLE dbo.tbl_NetworkPeriods WITH CHECK ADD CONSTRAINT CK_NetworkPeriods_contract CHECK (contract_type IN ('regular','gross'));

-- +goose StatementBegin
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'in_gross') IS NOT NULL
BEGIN
    DECLARE @dropDefault NVARCHAR(MAX) = N'';

    SELECT @dropDefault = N'ALTER TABLE dbo.tbl_NetworkPlans DROP CONSTRAINT ' + QUOTENAME(dc.name) + N';'
    FROM sys.default_constraints dc
    JOIN sys.columns c ON c.object_id = dc.parent_object_id AND c.column_id = dc.parent_column_id
    WHERE dc.parent_object_id = OBJECT_ID('dbo.tbl_NetworkPlans') AND c.name = 'in_gross';

    IF @dropDefault <> N'' EXEC sp_executesql @dropDefault;

    EXEC sp_executesql N'ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN in_gross';
END;
-- +goose StatementEnd

IF COL_LENGTH('dbo.tbl_NetworkPlans', 'fact_rub') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN fact_rub;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'forecast_rub') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN forecast_rub;

CREATE INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans(network_id, [year])
    INCLUDE ([quarter], brand_as, plan_rub, investments_pct);
