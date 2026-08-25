-- +goose Up
-- Явное удаление SKU из цен сети должно переживать повторную подстановку OLAP SS.
IF OBJECT_ID('dbo.tbl_NetworkContractPriceExclusions', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_NetworkContractPriceExclusions (
        id          BIGINT IDENTITY(1,1) NOT NULL,
        network_id  INT NOT NULL,
        sku         NVARCHAR(255) NOT NULL,
        excluded_by NVARCHAR(255) NULL,
        excluded_at DATETIME NOT NULL
            CONSTRAINT DF_NetworkContractPriceExclusions_at DEFAULT GETDATE(),
        CONSTRAINT PK_NetworkContractPriceExclusions PRIMARY KEY (id),
        CONSTRAINT FK_NetworkContractPriceExclusions_Network
            FOREIGN KEY (network_id) REFERENCES dbo.tbl_Networks(id),
        CONSTRAINT UQ_NetworkContractPriceExclusions_network_sku UNIQUE (network_id, sku)
    )
END;

-- +goose Down
DROP TABLE IF EXISTS dbo.tbl_NetworkContractPriceExclusions;
