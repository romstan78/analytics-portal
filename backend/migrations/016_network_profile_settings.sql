-- +goose Up
-- Профильные настройки сети:
--   * единое распределение квартального плана по трём месяцам;
--   * явный признак, что для сети применяется годовой кумулятив инвестиций.
-- Старые month*_pct в tbl_NetworkPlans остаются для совместимости, но приложение
-- использует профиль сети как единственный источник.

IF COL_LENGTH('dbo.tbl_Networks', 'month1_pct') IS NULL
    ALTER TABLE dbo.tbl_Networks ADD month1_pct DECIMAL(5,2) NOT NULL
        CONSTRAINT DF_Networks_month1_pct DEFAULT 30.00;
IF COL_LENGTH('dbo.tbl_Networks', 'month2_pct') IS NULL
    ALTER TABLE dbo.tbl_Networks ADD month2_pct DECIMAL(5,2) NOT NULL
        CONSTRAINT DF_Networks_month2_pct DEFAULT 30.00;
IF COL_LENGTH('dbo.tbl_Networks', 'month3_pct') IS NULL
    ALTER TABLE dbo.tbl_Networks ADD month3_pct DECIMAL(5,2) NOT NULL
        CONSTRAINT DF_Networks_month3_pct DEFAULT 40.00;
IF COL_LENGTH('dbo.tbl_Networks', 'has_annual_investment_cumulative') IS NULL
    ALTER TABLE dbo.tbl_Networks ADD has_annual_investment_cumulative BIT NOT NULL
        CONSTRAINT DF_Networks_annual_investment_cumulative DEFAULT 0;

-- Для существующей сети сохраняем последнее использованное распределение.
-- Если раньше строки отличались, профиль получает наиболее свежую настройку;
-- после миграции она едина для всех брендов и кварталов сети.
UPDATE n
   SET n.month1_pct = latest.month1_pct,
       n.month2_pct = latest.month2_pct,
       n.month3_pct = latest.month3_pct
FROM dbo.tbl_Networks n
CROSS APPLY (
    SELECT TOP 1 p.month1_pct, p.month2_pct, p.month3_pct
    FROM dbo.tbl_NetworkPlans p
    WHERE p.network_id = n.id
    ORDER BY p.updated_at DESC, p.id DESC
) latest;

IF NOT EXISTS (
    SELECT 1 FROM sys.check_constraints
    WHERE name = 'CK_Networks_month_distribution'
      AND parent_object_id = OBJECT_ID('dbo.tbl_Networks')
)
    ALTER TABLE dbo.tbl_Networks WITH CHECK ADD
        CONSTRAINT CK_Networks_month_distribution CHECK (
            month1_pct >= 0 AND month1_pct <= 100 AND
            month2_pct >= 0 AND month2_pct <= 100 AND
            month3_pct >= 0 AND month3_pct <= 100 AND
            month1_pct + month2_pct + month3_pct = 100
        );

-- +goose Down
IF EXISTS (
    SELECT 1 FROM sys.check_constraints
    WHERE name = 'CK_Networks_month_distribution'
      AND parent_object_id = OBJECT_ID('dbo.tbl_Networks')
)
    ALTER TABLE dbo.tbl_Networks DROP CONSTRAINT CK_Networks_month_distribution;

IF EXISTS (
    SELECT 1 FROM sys.default_constraints
    WHERE name = 'DF_Networks_annual_investment_cumulative'
      AND parent_object_id = OBJECT_ID('dbo.tbl_Networks')
)
    ALTER TABLE dbo.tbl_Networks DROP CONSTRAINT DF_Networks_annual_investment_cumulative;
IF EXISTS (
    SELECT 1 FROM sys.default_constraints
    WHERE name = 'DF_Networks_month3_pct'
      AND parent_object_id = OBJECT_ID('dbo.tbl_Networks')
)
    ALTER TABLE dbo.tbl_Networks DROP CONSTRAINT DF_Networks_month3_pct;
IF EXISTS (
    SELECT 1 FROM sys.default_constraints
    WHERE name = 'DF_Networks_month2_pct'
      AND parent_object_id = OBJECT_ID('dbo.tbl_Networks')
)
    ALTER TABLE dbo.tbl_Networks DROP CONSTRAINT DF_Networks_month2_pct;
IF EXISTS (
    SELECT 1 FROM sys.default_constraints
    WHERE name = 'DF_Networks_month1_pct'
      AND parent_object_id = OBJECT_ID('dbo.tbl_Networks')
)
    ALTER TABLE dbo.tbl_Networks DROP CONSTRAINT DF_Networks_month1_pct;

IF COL_LENGTH('dbo.tbl_Networks', 'has_annual_investment_cumulative') IS NOT NULL
    ALTER TABLE dbo.tbl_Networks DROP COLUMN has_annual_investment_cumulative;
IF COL_LENGTH('dbo.tbl_Networks', 'month3_pct') IS NOT NULL
    ALTER TABLE dbo.tbl_Networks DROP COLUMN month3_pct;
IF COL_LENGTH('dbo.tbl_Networks', 'month2_pct') IS NOT NULL
    ALTER TABLE dbo.tbl_Networks DROP COLUMN month2_pct;
IF COL_LENGTH('dbo.tbl_Networks', 'month1_pct') IS NOT NULL
    ALTER TABLE dbo.tbl_Networks DROP COLUMN month1_pct;
