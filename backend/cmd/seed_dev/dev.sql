SET NOCOUNT ON;
SET XACT_ABORT ON;
BEGIN TRANSACTION;

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_ChannelSegmentMapping WHERE name = 'qty')
    INSERT INTO dbo.tbl_ChannelSegmentMapping(name, un_rub, segment, channel)
    VALUES ('qty', N'уп', N'Демо', N'Все каналы');

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_ChannelSegmentMapping WHERE name = 'rub')
    INSERT INTO dbo.tbl_ChannelSegmentMapping(name, un_rub, segment, channel)
    VALUES ('rub', N'руб', N'Демо', N'Все каналы');

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_SKUMapping WHERE sku = 'DEMO-SKU-001')
    INSERT INTO dbo.tbl_SKUMapping(sku, brand, brand_as)
    VALUES ('DEMO-SKU-001', N'Демо-бренд', N'Демо-бренд');

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_NetworkGeoMapping WHERE network_name = N'Демо-сеть')
    INSERT INTO dbo.tbl_NetworkGeoMapping(network_name, kam, network_type, top20_segment, key_region)
    VALUES (N'Демо-сеть', N'Демо KAM', N'Аптечная сеть', N'Демо', N'Россия');

IF NOT EXISTS (
    SELECT 1 FROM dbo.tbl_KAMNetworkMapping
    WHERE kam = N'Демо KAM' AND network_name = N'Демо-сеть' AND valid_from = '2020-01-01'
)
    INSERT INTO dbo.tbl_KAMNetworkMapping(kam, network_name, valid_from)
    VALUES (N'Демо KAM', N'Демо-сеть', '2020-01-01');

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_MechanicsChannelMapping WHERE mechanics = N'Скидка')
    INSERT INTO dbo.tbl_MechanicsChannelMapping(mechanics, channel)
    VALUES (N'Скидка', N'Аптечные сети');

IF NOT EXISTS (
    SELECT 1 FROM dbo.tbl_EcomSalesConsolidated
    WHERE [year] = YEAR(GETDATE()) AND [month] = MONTH(GETDATE())
      AND brandName = N'Демо-бренд' AND productName = N'DEMO-SKU-001' AND networkName = N'Демо-сеть'
)
    INSERT INTO dbo.tbl_EcomSalesConsolidated(
        [year], [month], brandName, productName, networkName, qty, rub, updated_at
    ) VALUES (
        YEAR(GETDATE()), MONTH(GETDATE()), N'Демо-бренд', N'DEMO-SKU-001', N'Демо-сеть', 120, 24000, GETDATE()
    );

DECLARE @source_id INT = (
    SELECT TOP 1 id FROM dbo.tbl_EcomSalesConsolidated
    WHERE [year] = YEAR(GETDATE()) AND [month] = MONTH(GETDATE())
      AND brandName = N'Демо-бренд' AND productName = N'DEMO-SKU-001' AND networkName = N'Демо-сеть'
    ORDER BY id
);

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_EcomSalesNormalized WHERE source_id = @source_id AND metric_type = 'qty')
    INSERT INTO dbo.tbl_EcomSalesNormalized(
        source_id, [year], [month], brandName, productName, networkName,
        metric_type, metric_value, updated_at, un_rub, segment, channel
    ) VALUES (
        @source_id, YEAR(GETDATE()), MONTH(GETDATE()), N'Демо-бренд', N'DEMO-SKU-001', N'Демо-сеть',
        'qty', 120, GETDATE(), N'уп', N'Демо', N'Все каналы'
    );

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_EcomSalesNormalized WHERE source_id = @source_id AND metric_type = 'rub')
    INSERT INTO dbo.tbl_EcomSalesNormalized(
        source_id, [year], [month], brandName, productName, networkName,
        metric_type, metric_value, updated_at, un_rub, segment, channel
    ) VALUES (
        @source_id, YEAR(GETDATE()), MONTH(GETDATE()), N'Демо-бренд', N'DEMO-SKU-001', N'Демо-сеть',
        'rub', 24000, GETDATE(), N'руб', N'Демо', N'Все каналы'
    );

IF NOT EXISTS (SELECT 1 FROM dbo.tbl_PromoActivities WHERE sku = 'DEMO-SKU-001' AND network_name = N'Демо-сеть' AND deleted_at IS NULL)
    INSERT INTO dbo.tbl_PromoActivities(
        network_name, kam, [year], [month], quarter, sku, brand, brand_as,
        mechanics, baseline_units, plan_promo_units, plan_investments_rub,
        status, agreement1_status, agreement2_status, created_by, updated_by
    ) VALUES (
        N'Демо-сеть', N'Демо KAM', YEAR(GETDATE()), MONTH(GETDATE()), DATEPART(QUARTER, GETDATE()),
        'DEMO-SKU-001', N'Демо-бренд', N'Демо-бренд', N'Скидка', 100, 120, 2000,
        N'Планируется', 'pending', 'pending', 'dev-seed', 'dev-seed'
    );

COMMIT TRANSACTION;
