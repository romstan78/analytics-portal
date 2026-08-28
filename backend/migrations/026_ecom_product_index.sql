-- +goose Up
-- +goose StatementBegin
-- Индекс под справочник SKU панели интернет-продаж.
--
-- Список товаров строится через SELECT DISTINCT productName. Существующие
-- индексы начинаются с [year]/[month] или channel, поэтому для этого запроса не
-- годятся, и он шёл сканированием: на демо-объёме 461 208 строк это 453 мс CPU
-- при каждом открытии панели фильтров.
--
-- Ведущая колонка productName делает выборку различных значений обходом самого
-- индекса; [year] и [month] в ключе позволяют тому же индексу обслуживать
-- сужение справочника по периоду.
IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_EcomSalesNormalized_product' AND object_id = OBJECT_ID('dbo.tbl_EcomSalesNormalized')
)
BEGIN
    CREATE INDEX IX_EcomSalesNormalized_product
        ON dbo.tbl_EcomSalesNormalized(productName, [year], [month])
        INCLUDE (brandName, networkName);
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
IF EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_EcomSalesNormalized_product' AND object_id = OBJECT_ID('dbo.tbl_EcomSalesNormalized')
)
BEGIN
    DROP INDEX IX_EcomSalesNormalized_product ON dbo.tbl_EcomSalesNormalized;
END;
-- +goose StatementEnd
