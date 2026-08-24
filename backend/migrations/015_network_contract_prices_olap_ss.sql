-- +goose Up
-- Исправление первичного заполнения цен контракта:
--   * источник — только сегмент OLAP SS;
--   * название сети не участвует в расчёте;
--   * для каждого SKU используется взвешенная цена последнего общего месяца
--     2026 года: SUM(руб) / SUM(уп) по всем сетям.
-- Подтверждённые и ручные цены не перезаписываются.

-- +goose StatementBegin
CREATE TABLE #olap_ss_sku_prices (
    brand_as       NVARCHAR(255) NOT NULL,
    sku            NVARCHAR(255) NOT NULL,
    contract_price DECIMAL(18,4) NOT NULL,
    source_month   INT NOT NULL,
    PRIMARY KEY (sku)
);

DECLARE @latest_month INT;

SELECT TOP 1 @latest_month = n.[month]
FROM dbo.tbl_EcomSalesNormalized n
WHERE n.[year] = 2026
  AND n.segment = N'OLAP SS'
GROUP BY n.[month]
HAVING SUM(CASE WHEN n.un_rub = N'руб' THEN n.metric_value ELSE 0 END) > 0
   AND SUM(CASE WHEN n.un_rub = N'уп' THEN n.metric_value ELSE 0 END) > 0
ORDER BY n.[month] DESC;

IF @latest_month IS NOT NULL
BEGIN
    INSERT INTO #olap_ss_sku_prices (brand_as, sku, contract_price, source_month)
    SELECT
        COALESCE(
            MAX(NULLIF(LTRIM(RTRIM(sm.brand_as)), N'')),
            MAX(NULLIF(LTRIM(RTRIM(n.brandName)), N'')),
            N'Без бренда'
        ),
        LTRIM(RTRIM(n.productName)),
        ROUND(
            SUM(CASE WHEN n.un_rub = N'руб' THEN n.metric_value ELSE 0 END)
            / NULLIF(SUM(CASE WHEN n.un_rub = N'уп' THEN n.metric_value ELSE 0 END), 0),
            4
        ),
        @latest_month
    FROM dbo.tbl_EcomSalesNormalized n
    LEFT JOIN dbo.tbl_SKUMapping sm
      ON LTRIM(RTRIM(sm.sku)) = LTRIM(RTRIM(n.productName))
    WHERE n.[year] = 2026
      AND n.[month] = @latest_month
      AND n.segment = N'OLAP SS'
      AND n.productName IS NOT NULL
      AND LTRIM(RTRIM(n.productName)) <> N''
    GROUP BY LTRIM(RTRIM(n.productName))
    HAVING SUM(CASE WHEN n.un_rub = N'руб' THEN n.metric_value ELSE 0 END) > 0
       AND SUM(CASE WHEN n.un_rub = N'уп' THEN n.metric_value ELSE 0 END) > 0;

    -- Обновляем только ещё не подтверждённые автозначения старой логики.
    UPDATE p
       SET p.brand_as = s.brand_as,
           p.contract_price = s.contract_price,
           p.source_year = 2026,
           p.source_month = s.source_month,
           p.updated_by = 'migration-015',
           p.updated_at = GETDATE()
    FROM dbo.tbl_NetworkContractPrices p
    JOIN #olap_ss_sku_prices s
      ON LTRIM(RTRIM(s.sku)) = LTRIM(RTRIM(p.sku))
    WHERE p.source_type = 'olap_seed'
      AND p.source_year = 2026
      AND p.is_confirmed = 0;

    -- Старые автозначения, которых нет в корректном срезе OLAP SS, больше не
    -- должны выглядеть как доступные контрактные цены.
    DELETE p
    FROM dbo.tbl_NetworkContractPrices p
    WHERE p.source_type = 'olap_seed'
      AND p.source_year = 2026
      AND p.is_confirmed = 0
      AND NOT EXISTS (
          SELECT 1
          FROM #olap_ss_sku_prices s
          WHERE LTRIM(RTRIM(s.sku)) = LTRIM(RTRIM(p.sku))
      );

    -- Одинаковый набор значений по SKU выдаётся каждой сети. Если КАМ уже
    -- завёл пересекающийся ручной или подтверждённый период, он приоритетнее.
    INSERT INTO dbo.tbl_NetworkContractPrices (
        network_id, brand_as, sku, contract_price, valid_from, valid_to,
        source_type, source_year, source_month, is_confirmed, updated_by
    )
    SELECT
        n.id,
        s.brand_as,
        s.sku,
        s.contract_price,
        DATEFROMPARTS(2026, 1, 1),
        DATEFROMPARTS(2026, 12, 31),
        'olap_seed',
        2026,
        s.source_month,
        0,
        'migration-015'
    FROM dbo.tbl_Networks n
    CROSS JOIN #olap_ss_sku_prices s
    WHERE NOT EXISTS (
        SELECT 1
        FROM dbo.tbl_NetworkContractPrices p
        WHERE p.network_id = n.id
          AND LTRIM(RTRIM(p.sku)) = LTRIM(RTRIM(s.sku))
          AND p.valid_from <= DATEFROMPARTS(2026, 12, 31)
          AND p.valid_to >= DATEFROMPARTS(2026, 1, 1)
    );
END;

DROP TABLE #olap_ss_sku_prices;
-- +goose StatementEnd

-- +goose Down
-- Корректирующая миграция данных не восстанавливает ошибочную привязку цены
-- к названию сети. Схема таблицы при этом не менялась.
SELECT 1;
