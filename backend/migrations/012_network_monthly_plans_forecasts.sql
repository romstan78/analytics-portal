-- +goose Up
-- Помесячный слой реестра сетей. Квартальный план остаётся источником истины,
-- а три процента задают его распределение внутри квартала. Факт и прогноз
-- хранятся атомарно по месяцу; квартальные поля tbl_NetworkPlans сохраняются
-- для обратной совместимости и быстрых итогов текущей формы.

IF COL_LENGTH('dbo.tbl_NetworkPlans', 'month1_pct') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD month1_pct DECIMAL(5,2) NOT NULL
        CONSTRAINT DF_NetworkPlans_month1_pct DEFAULT 30.00;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'month2_pct') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD month2_pct DECIMAL(5,2) NOT NULL
        CONSTRAINT DF_NetworkPlans_month2_pct DEFAULT 30.00;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'month3_pct') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD month3_pct DECIMAL(5,2) NOT NULL
        CONSTRAINT DF_NetworkPlans_month3_pct DEFAULT 40.00;

IF NOT EXISTS (
    SELECT 1 FROM sys.check_constraints
    WHERE name = 'CK_NetworkPlans_month_distribution'
      AND parent_object_id = OBJECT_ID('dbo.tbl_NetworkPlans')
)
    ALTER TABLE dbo.tbl_NetworkPlans WITH CHECK ADD
        CONSTRAINT CK_NetworkPlans_month_distribution CHECK (
            month1_pct >= 0 AND month1_pct <= 100 AND
            month2_pct >= 0 AND month2_pct <= 100 AND
            month3_pct >= 0 AND month3_pct <= 100 AND
            month1_pct + month2_pct + month3_pct = 100
        );

-- Независимый прогноз инвестиций. Он может учитывать промо и фиксированные
-- выплаты, поэтому не обязан совпадать с investments_pct * forecast_rub.
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'forecast_investments_rub') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD forecast_investments_rub DECIMAL(18,2) NULL;

IF OBJECT_ID('dbo.tbl_NetworkMonthlyFacts', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_NetworkMonthlyFacts (
        id                      BIGINT IDENTITY(1,1) NOT NULL,
        network_id              INT NOT NULL,
        [year]                  INT NOT NULL,
        [month]                 INT NOT NULL,
        brand_as                NVARCHAR(255) NOT NULL,
        sku                     NVARCHAR(255) NULL,
        fact_rub                DECIMAL(18,2) NULL,
        fact_units              DECIMAL(18,2) NULL,
        fact_investments_rub    DECIMAL(18,2) NULL,
        is_final                BIT NOT NULL CONSTRAINT DF_NetworkMonthlyFacts_final DEFAULT 1,
        source_name             NVARCHAR(100) NULL,
        source_updated_at       DATETIME NULL,
        created_at              DATETIME NOT NULL CONSTRAINT DF_NetworkMonthlyFacts_created DEFAULT GETDATE(),
        updated_at              DATETIME NOT NULL CONSTRAINT DF_NetworkMonthlyFacts_updated DEFAULT GETDATE(),
        CONSTRAINT PK_NetworkMonthlyFacts PRIMARY KEY (id),
        CONSTRAINT FK_NetworkMonthlyFacts_Network FOREIGN KEY (network_id) REFERENCES dbo.tbl_Networks(id),
        CONSTRAINT CK_NetworkMonthlyFacts_month CHECK ([month] BETWEEN 1 AND 12),
        CONSTRAINT CK_NetworkMonthlyFacts_values CHECK (
            (fact_rub IS NULL OR fact_rub >= 0) AND
            (fact_units IS NULL OR fact_units >= 0) AND
            (fact_investments_rub IS NULL OR fact_investments_rub >= 0)
        )
    )
END;

IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'UX_NetworkMonthlyFacts_brand'
      AND object_id = OBJECT_ID('dbo.tbl_NetworkMonthlyFacts')
)
    CREATE UNIQUE INDEX UX_NetworkMonthlyFacts_brand
        ON dbo.tbl_NetworkMonthlyFacts(network_id, [year], [month], brand_as)
        WHERE sku IS NULL;

IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'UX_NetworkMonthlyFacts_sku'
      AND object_id = OBJECT_ID('dbo.tbl_NetworkMonthlyFacts')
)
    CREATE UNIQUE INDEX UX_NetworkMonthlyFacts_sku
        ON dbo.tbl_NetworkMonthlyFacts(network_id, [year], [month], brand_as, sku)
        WHERE sku IS NOT NULL;

IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_NetworkMonthlyFacts_network_period'
      AND object_id = OBJECT_ID('dbo.tbl_NetworkMonthlyFacts')
)
    CREATE INDEX IX_NetworkMonthlyFacts_network_period
        ON dbo.tbl_NetworkMonthlyFacts(network_id, [year], [month])
        INCLUDE (brand_as, sku, fact_rub, fact_units, fact_investments_rub, is_final, updated_at);

IF OBJECT_ID('dbo.tbl_NetworkForecasts', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_NetworkForecasts (
        id                          BIGINT IDENTITY(1,1) NOT NULL,
        network_id                  INT NOT NULL,
        [year]                      INT NOT NULL,
        [month]                     INT NOT NULL,
        brand_as                    NVARCHAR(255) NOT NULL,
        sku                         NVARCHAR(255) NULL,
        forecast_rub                DECIMAL(18,2) NULL,
        forecast_units              DECIMAL(18,2) NULL,
        forecast_investments_rub    DECIMAL(18,2) NULL,
        system_forecast_rub         DECIMAL(18,2) NULL,
        system_forecast_units       DECIMAL(18,2) NULL,
        confidence                  NVARCHAR(20) NULL,
        adjustment_reason           NVARCHAR(1000) NULL,
        updated_by                  NVARCHAR(100) NULL,
        created_at                  DATETIME NOT NULL CONSTRAINT DF_NetworkForecasts_created DEFAULT GETDATE(),
        updated_at                  DATETIME NOT NULL CONSTRAINT DF_NetworkForecasts_updated DEFAULT GETDATE(),
        CONSTRAINT PK_NetworkForecasts PRIMARY KEY (id),
        CONSTRAINT FK_NetworkForecasts_Network FOREIGN KEY (network_id) REFERENCES dbo.tbl_Networks(id),
        CONSTRAINT CK_NetworkForecasts_month CHECK ([month] BETWEEN 1 AND 12),
        CONSTRAINT CK_NetworkForecasts_confidence CHECK (
            confidence IS NULL OR confidence IN ('low', 'medium', 'high')
        ),
        CONSTRAINT CK_NetworkForecasts_values CHECK (
            (forecast_rub IS NULL OR forecast_rub >= 0) AND
            (forecast_units IS NULL OR forecast_units >= 0) AND
            (forecast_investments_rub IS NULL OR forecast_investments_rub >= 0) AND
            (system_forecast_rub IS NULL OR system_forecast_rub >= 0) AND
            (system_forecast_units IS NULL OR system_forecast_units >= 0)
        )
    )
END;

IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'UX_NetworkForecasts_brand'
      AND object_id = OBJECT_ID('dbo.tbl_NetworkForecasts')
)
    CREATE UNIQUE INDEX UX_NetworkForecasts_brand
        ON dbo.tbl_NetworkForecasts(network_id, [year], [month], brand_as)
        WHERE sku IS NULL;

IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'UX_NetworkForecasts_sku'
      AND object_id = OBJECT_ID('dbo.tbl_NetworkForecasts')
)
    CREATE UNIQUE INDEX UX_NetworkForecasts_sku
        ON dbo.tbl_NetworkForecasts(network_id, [year], [month], brand_as, sku)
        WHERE sku IS NOT NULL;

IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_NetworkForecasts_network_period'
      AND object_id = OBJECT_ID('dbo.tbl_NetworkForecasts')
)
    CREATE INDEX IX_NetworkForecasts_network_period
        ON dbo.tbl_NetworkForecasts(network_id, [year], [month])
        INCLUDE (brand_as, sku, forecast_rub, forecast_units, forecast_investments_rub,
                 system_forecast_rub, confidence, updated_at);

-- Существующий квартальный прогноз раскладываем 30/30/40, чтобы после миграции
-- он не исчез из нового рабочего места.
INSERT INTO dbo.tbl_NetworkForecasts (
    network_id, [year], [month], brand_as, forecast_rub,
    forecast_investments_rub, confidence, updated_by
)
SELECT
    p.network_id,
    p.[year],
    (p.[quarter] - 1) * 3 + v.month_in_quarter,
    p.brand_as,
    ROUND(p.forecast_rub * v.share_pct / 100.0, 2),
    CASE WHEN p.investments_pct IS NULL THEN NULL
         ELSE ROUND(p.forecast_rub * p.investments_pct / 100.0 * v.share_pct / 100.0, 2)
    END,
    'low',
    'migration-012'
FROM dbo.tbl_NetworkPlans p
CROSS APPLY (VALUES (1, p.month1_pct), (2, p.month2_pct), (3, p.month3_pct)) v(month_in_quarter, share_pct)
WHERE p.brand_as IS NOT NULL
  AND p.forecast_rub IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM dbo.tbl_NetworkForecasts f
      WHERE f.network_id = p.network_id
        AND f.[year] = p.[year]
        AND f.[month] = (p.[quarter] - 1) * 3 + v.month_in_quarter
        AND f.brand_as = p.brand_as
        AND f.sku IS NULL
  );

-- Покрывающий индекс квартальной формы расширяем месячным профилем и новым EAC инвестиций.
IF EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_NetworkPlans_network_year'
      AND object_id = OBJECT_ID('dbo.tbl_NetworkPlans')
)
    DROP INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans;

CREATE INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans(network_id, [year])
    INCLUDE ([quarter], brand_as, in_gross, plan_rub, plan_units, fact_rub, forecast_rub,
             investments_pct, fact_investments_rub, forecast_investments_rub,
             month1_pct, month2_pct, month3_pct);

-- +goose Down
IF EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_NetworkPlans_network_year'
      AND object_id = OBJECT_ID('dbo.tbl_NetworkPlans')
)
    DROP INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans;

DROP TABLE IF EXISTS dbo.tbl_NetworkForecasts;
DROP TABLE IF EXISTS dbo.tbl_NetworkMonthlyFacts;

IF EXISTS (
    SELECT 1 FROM sys.check_constraints
    WHERE name = 'CK_NetworkPlans_month_distribution'
      AND parent_object_id = OBJECT_ID('dbo.tbl_NetworkPlans')
)
    ALTER TABLE dbo.tbl_NetworkPlans DROP CONSTRAINT CK_NetworkPlans_month_distribution;

-- +goose StatementBegin
DECLARE @dropDefaults NVARCHAR(MAX) = N'';
SELECT @dropDefaults = @dropDefaults +
    N'ALTER TABLE dbo.tbl_NetworkPlans DROP CONSTRAINT ' + QUOTENAME(dc.name) + N';'
FROM sys.default_constraints dc
JOIN sys.columns c
  ON c.object_id = dc.parent_object_id AND c.column_id = dc.parent_column_id
WHERE dc.parent_object_id = OBJECT_ID('dbo.tbl_NetworkPlans')
  AND c.name IN ('month1_pct', 'month2_pct', 'month3_pct');
IF @dropDefaults <> N'' EXEC sp_executesql @dropDefaults;
-- +goose StatementEnd

IF COL_LENGTH('dbo.tbl_NetworkPlans', 'forecast_investments_rub') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN forecast_investments_rub;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'month3_pct') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN month3_pct;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'month2_pct') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN month2_pct;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'month1_pct') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN month1_pct;

CREATE INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans(network_id, [year])
    INCLUDE ([quarter], brand_as, in_gross, plan_rub, fact_rub, forecast_rub,
             investments_pct, fact_investments_rub);
