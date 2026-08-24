-- +goose Up
-- Правила совместного зачёта планов и инвестиций по смежным кварталам.
-- Поквартальные суммы остаются источником истины; правило хранит только
-- диапазон и область действия: весь портфель (brand_as IS NULL) или бренд.

IF OBJECT_ID('dbo.tbl_NetworkPeriodGroups', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_NetworkPeriodGroups (
        id              INT IDENTITY(1,1) NOT NULL,
        network_id      INT NOT NULL,
        [year]          INT NOT NULL,
        start_quarter   INT NOT NULL,
        end_quarter     INT NOT NULL,
        brand_as        NVARCHAR(255) NULL,
        updated_by      NVARCHAR(100) NULL,
        created_at      DATETIME NOT NULL CONSTRAINT DF_NetworkPeriodGroups_created DEFAULT GETDATE(),
        updated_at      DATETIME NOT NULL CONSTRAINT DF_NetworkPeriodGroups_updated DEFAULT GETDATE(),
        CONSTRAINT PK_NetworkPeriodGroups PRIMARY KEY (id),
        CONSTRAINT FK_NetworkPeriodGroups_Network FOREIGN KEY (network_id) REFERENCES dbo.tbl_Networks(id),
        CONSTRAINT CK_NetworkPeriodGroups_year CHECK ([year] BETWEEN 2000 AND 2100),
        CONSTRAINT CK_NetworkPeriodGroups_range CHECK (
            start_quarter BETWEEN 1 AND 4 AND
            end_quarter BETWEEN 1 AND 4 AND
            start_quarter < end_quarter
        ),
        CONSTRAINT UQ_NetworkPeriodGroups_rule UNIQUE (
            network_id, [year], start_quarter, end_quarter, brand_as
        )
    )
END;

IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_NetworkPeriodGroups_network_year'
      AND object_id = OBJECT_ID('dbo.tbl_NetworkPeriodGroups')
)
    CREATE INDEX IX_NetworkPeriodGroups_network_year
        ON dbo.tbl_NetworkPeriodGroups(network_id, [year], start_quarter, end_quarter)
        INCLUDE (brand_as, updated_by, updated_at);

-- +goose Down
DROP TABLE IF EXISTS dbo.tbl_NetworkPeriodGroups;
