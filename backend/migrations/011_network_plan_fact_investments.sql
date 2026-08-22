-- +goose Up
-- Факт инвестиций — третья нога сравнения «план / факт / прогноз» на денежной
-- стороне. Приходит той же загрузкой, что и факт объёма (sync_script/import_network_facts.py),
-- и в интерфейсе доступен только для чтения.

IF COL_LENGTH('dbo.tbl_NetworkPlans', 'fact_investments_rub') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD fact_investments_rub DECIMAL(18,2) NULL;

-- Покрывающий индекс сетки: факт инвестиций читается вместе с остальными суммами.
IF EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_NetworkPlans_network_year' AND object_id = OBJECT_ID('dbo.tbl_NetworkPlans'))
    DROP INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans;
CREATE INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans(network_id, [year])
    INCLUDE ([quarter], brand_as, in_gross, plan_rub, fact_rub, forecast_rub,
             investments_pct, fact_investments_rub);

-- +goose Down
IF EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_NetworkPlans_network_year' AND object_id = OBJECT_ID('dbo.tbl_NetworkPlans'))
    DROP INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans;

IF COL_LENGTH('dbo.tbl_NetworkPlans', 'fact_investments_rub') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN fact_investments_rub;

CREATE INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans(network_id, [year])
    INCLUDE ([quarter], brand_as, in_gross, plan_rub, fact_rub, forecast_rub, investments_pct);
