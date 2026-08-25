-- +goose Up
-- Значения НДС в карточке задают начальные настройки новых кварталов. Рабочие
-- vat_included и vat_rate сохраняются поквартально в tbl_NetworkPeriods.

IF COL_LENGTH('dbo.tbl_Networks', 'vat_included') IS NULL
    ALTER TABLE dbo.tbl_Networks ADD vat_included BIT NOT NULL
        CONSTRAINT DF_Networks_vat_included DEFAULT 1;
IF COL_LENGTH('dbo.tbl_Networks', 'vat_rate') IS NULL
    ALTER TABLE dbo.tbl_Networks ADD vat_rate DECIMAL(5,2) NOT NULL
        CONSTRAINT DF_Networks_vat_rate DEFAULT 20.00;

-- Для существующей сети переносим последнюю сохранённую настройку периода.
UPDATE n
   SET n.vat_included = latest.vat_included,
       n.vat_rate = latest.vat_rate
FROM dbo.tbl_Networks n
CROSS APPLY (
    SELECT TOP 1 p.vat_included, p.vat_rate
    FROM dbo.tbl_NetworkPeriods p
    WHERE p.network_id = n.id
    ORDER BY p.[year] DESC, p.[quarter] DESC, p.updated_at DESC, p.id DESC
) latest;

IF NOT EXISTS (
    SELECT 1 FROM sys.check_constraints
    WHERE name = 'CK_Networks_vat_rate'
      AND parent_object_id = OBJECT_ID('dbo.tbl_Networks')
)
    ALTER TABLE dbo.tbl_Networks WITH CHECK ADD
        CONSTRAINT CK_Networks_vat_rate CHECK (vat_rate >= 0 AND vat_rate < 100);

-- +goose Down
IF EXISTS (
    SELECT 1 FROM sys.check_constraints
    WHERE name = 'CK_Networks_vat_rate'
      AND parent_object_id = OBJECT_ID('dbo.tbl_Networks')
)
    ALTER TABLE dbo.tbl_Networks DROP CONSTRAINT CK_Networks_vat_rate;

IF EXISTS (
    SELECT 1 FROM sys.default_constraints
    WHERE name = 'DF_Networks_vat_rate'
      AND parent_object_id = OBJECT_ID('dbo.tbl_Networks')
)
    ALTER TABLE dbo.tbl_Networks DROP CONSTRAINT DF_Networks_vat_rate;
IF EXISTS (
    SELECT 1 FROM sys.default_constraints
    WHERE name = 'DF_Networks_vat_included'
      AND parent_object_id = OBJECT_ID('dbo.tbl_Networks')
)
    ALTER TABLE dbo.tbl_Networks DROP CONSTRAINT DF_Networks_vat_included;

IF COL_LENGTH('dbo.tbl_Networks', 'vat_rate') IS NOT NULL
    ALTER TABLE dbo.tbl_Networks DROP COLUMN vat_rate;
IF COL_LENGTH('dbo.tbl_Networks', 'vat_included') IS NOT NULL
    ALTER TABLE dbo.tbl_Networks DROP COLUMN vat_included;
