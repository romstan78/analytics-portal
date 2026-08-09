-- +goose Up
-- Покрывающий индекс для фильтрации промо-активностей.
-- Покрывает наиболее частые фильтры: deleted_at, year, month, kam, network_name, brand_as.

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_PromoActivities_Filters')
BEGIN
    CREATE NONCLUSTERED INDEX IX_PromoActivities_Filters ON dbo.tbl_PromoActivities (deleted_at, year, month, kam, network_name, brand_as) INCLUDE (sku, mechanics, status, id, plan_promo_units, plan_promo_rub, plan_investments_rub, contract_price, created_at, updated_at)
END;

-- +goose Down
DROP INDEX IF EXISTS IX_PromoActivities_Filters ON dbo.tbl_PromoActivities;
