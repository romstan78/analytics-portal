-- +goose Up
-- Цены контракта ведутся по сети и SKU с периодами действия. OLAP-цены остаются
-- отдельным справочным рядом; первоначальное значение за 2026 год рассчитывается
-- как взвешенная цена SUM(rub) / SUM(qty) последнего доступного месяца 2026 года.

IF OBJECT_ID('dbo.tbl_NetworkContractPrices', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_NetworkContractPrices (
        id                  BIGINT IDENTITY(1,1) NOT NULL,
        network_id          INT NOT NULL,
        brand_as            NVARCHAR(255) NOT NULL,
        sku                 NVARCHAR(255) NOT NULL,
        contract_price      DECIMAL(18,4) NOT NULL,
        valid_from          DATE NOT NULL,
        valid_to            DATE NOT NULL,
        source_type         NVARCHAR(30) NOT NULL CONSTRAINT DF_NetworkContractPrices_source DEFAULT 'manual',
        source_year         INT NULL,
        source_month        INT NULL,
        is_confirmed        BIT NOT NULL CONSTRAINT DF_NetworkContractPrices_confirmed DEFAULT 0,
        updated_by          NVARCHAR(100) NULL,
        created_at          DATETIME NOT NULL CONSTRAINT DF_NetworkContractPrices_created DEFAULT GETDATE(),
        updated_at          DATETIME NOT NULL CONSTRAINT DF_NetworkContractPrices_updated DEFAULT GETDATE(),
        CONSTRAINT PK_NetworkContractPrices PRIMARY KEY (id),
        CONSTRAINT FK_NetworkContractPrices_Network FOREIGN KEY (network_id) REFERENCES dbo.tbl_Networks(id),
        CONSTRAINT CK_NetworkContractPrices_price CHECK (contract_price > 0),
        CONSTRAINT CK_NetworkContractPrices_dates CHECK (valid_from <= valid_to),
        CONSTRAINT CK_NetworkContractPrices_source_month CHECK (source_month IS NULL OR source_month BETWEEN 1 AND 12),
        CONSTRAINT CK_NetworkContractPrices_source CHECK (source_type IN ('manual', 'olap_seed', 'contract_import')),
        CONSTRAINT UQ_NetworkContractPrices_start UNIQUE (network_id, sku, valid_from)
    )
END;

IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_NetworkContractPrices_network_dates'
      AND object_id = OBJECT_ID('dbo.tbl_NetworkContractPrices')
)
    CREATE INDEX IX_NetworkContractPrices_network_dates
        ON dbo.tbl_NetworkContractPrices(network_id, valid_from, valid_to)
        INCLUDE (brand_as, sku, contract_price, is_confirmed, source_year, source_month, updated_at);

-- +goose StatementBegin
;WITH latest_month AS (
    SELECT MAX(monthly.[month]) AS [month]
    FROM (
        SELECT [month]
        FROM dbo.tbl_EcomSalesConsolidated
        WHERE [year] = 2026 AND [month] BETWEEN 1 AND 12
        GROUP BY [month]
        HAVING SUM(ISNULL(rub, 0)) > 0 AND SUM(ISNULL(qty, 0)) > 0
    ) monthly
), monthly_price AS (
    SELECT
        n.id AS network_id,
        COALESCE(NULLIF(LTRIM(RTRIM(sm.brand_as)), N''),
                 NULLIF(LTRIM(RTRIM(s.brandName)), N''), N'Без бренда') AS brand_as,
        LTRIM(RTRIM(s.productName)) AS sku,
        s.[month],
        SUM(s.rub) AS rub_sum,
        SUM(s.qty) AS units_sum
    FROM dbo.tbl_EcomSalesConsolidated s
    CROSS JOIN latest_month lm
    JOIN dbo.tbl_Networks n ON LTRIM(RTRIM(n.name)) = LTRIM(RTRIM(s.networkName))
    LEFT JOIN dbo.tbl_SKUMapping sm ON LTRIM(RTRIM(sm.sku)) = LTRIM(RTRIM(s.productName))
    WHERE s.[year] = 2026
      AND s.[month] BETWEEN 1 AND 12
      AND s.[month] = lm.[month]
      AND s.productName IS NOT NULL
      AND LTRIM(RTRIM(s.productName)) <> N''
      AND s.qty IS NOT NULL
      AND s.rub IS NOT NULL
    GROUP BY n.id,
             COALESCE(NULLIF(LTRIM(RTRIM(sm.brand_as)), N''),
                      NULLIF(LTRIM(RTRIM(s.brandName)), N''), N'Без бренда'),
             LTRIM(RTRIM(s.productName)), s.[month]
    HAVING SUM(s.qty) > 0 AND SUM(s.rub) > 0
)
INSERT INTO dbo.tbl_NetworkContractPrices (
    network_id, brand_as, sku, contract_price, valid_from, valid_to,
    source_type, source_year, source_month, is_confirmed, updated_by
)
SELECT
    r.network_id,
    r.brand_as,
    r.sku,
    ROUND(r.rub_sum / NULLIF(r.units_sum, 0), 4),
    DATEFROMPARTS(2026, 1, 1),
    DATEFROMPARTS(2026, 12, 31),
    'olap_seed',
    2026,
    r.[month],
    0,
    'migration-013'
FROM monthly_price r
WHERE NOT EXISTS (
      SELECT 1 FROM dbo.tbl_NetworkContractPrices p
      WHERE p.network_id = r.network_id
        AND p.sku = r.sku
        AND p.valid_from = DATEFROMPARTS(2026, 1, 1)
  );
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS dbo.tbl_NetworkContractPrices;
