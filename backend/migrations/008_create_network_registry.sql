-- +goose Up
-- Реестр сетей: карточка сети, квартальные периоды (НДС, тип контракта),
-- планы по брендам и комментарии. Планы ведутся в рублях; колонка plan_units
-- заведена под будущий пересчёт по таблице цен контракта.

IF OBJECT_ID('dbo.tbl_Networks', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_Networks (
        id              INT IDENTITY(1,1) NOT NULL,
        name            NVARCHAR(255) NOT NULL,
        kam             NVARCHAR(255) NULL,
        network_type    NVARCHAR(20) NOT NULL CONSTRAINT DF_Networks_type DEFAULT 'regular',
        is_active       BIT NOT NULL CONSTRAINT DF_Networks_active DEFAULT 1,
        created_at      DATETIME NOT NULL CONSTRAINT DF_Networks_created_at DEFAULT GETDATE(),
        updated_at      DATETIME NOT NULL CONSTRAINT DF_Networks_updated_at DEFAULT GETDATE(),
        CONSTRAINT PK_Networks PRIMARY KEY (id),
        CONSTRAINT UQ_Networks_name UNIQUE (name),
        CONSTRAINT CK_Networks_type CHECK (network_type IN ('regular','warehouse'))
    )
END;

-- Атрибуты, действующие на конкретный квартал: НДС и тип контракта
-- меняются с любого периода, прошлые кварталы остаются как были.
IF OBJECT_ID('dbo.tbl_NetworkPeriods', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_NetworkPeriods (
        id              INT IDENTITY(1,1) NOT NULL,
        network_id      INT NOT NULL,
        [year]          INT NOT NULL,
        [quarter]       INT NOT NULL,
        vat_included    BIT NOT NULL CONSTRAINT DF_NetworkPeriods_vat DEFAULT 1,
        vat_rate        DECIMAL(5,2) NOT NULL CONSTRAINT DF_NetworkPeriods_vat_rate DEFAULT 20.00,
        contract_type   NVARCHAR(20) NOT NULL CONSTRAINT DF_NetworkPeriods_contract DEFAULT 'regular',
        updated_at      DATETIME NOT NULL CONSTRAINT DF_NetworkPeriods_updated_at DEFAULT GETDATE(),
        CONSTRAINT PK_NetworkPeriods PRIMARY KEY (id),
        CONSTRAINT FK_NetworkPeriods_Network FOREIGN KEY (network_id) REFERENCES dbo.tbl_Networks(id),
        CONSTRAINT UQ_NetworkPeriods_period UNIQUE (network_id, [year], [quarter]),
        CONSTRAINT CK_NetworkPeriods_quarter CHECK ([quarter] BETWEEN 1 AND 4),
        CONSTRAINT CK_NetworkPeriods_contract CHECK (contract_type IN ('regular','gross')),
        CONSTRAINT CK_NetworkPeriods_vat_rate CHECK (vat_rate >= 0 AND vat_rate < 100)
    )
END;

-- Строка плана — бренд на квартал. brand_as IS NULL — общий объём валового
-- контракта: UNIQUE в MSSQL считает NULL равными, поэтому такая строка
-- может быть только одна на сеть, год и квартал.
IF OBJECT_ID('dbo.tbl_NetworkPlans', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_NetworkPlans (
        id              INT IDENTITY(1,1) NOT NULL,
        network_id      INT NOT NULL,
        [year]          INT NOT NULL,
        [quarter]       INT NOT NULL,
        brand_as        NVARCHAR(255) NULL,
        plan_rub        DECIMAL(18,2) NULL,
        plan_units      DECIMAL(18,2) NULL,
        investments_pct DECIMAL(5,2) NULL,
        updated_by      NVARCHAR(100) NULL,
        created_at      DATETIME NOT NULL CONSTRAINT DF_NetworkPlans_created_at DEFAULT GETDATE(),
        updated_at      DATETIME NOT NULL CONSTRAINT DF_NetworkPlans_updated_at DEFAULT GETDATE(),
        CONSTRAINT PK_NetworkPlans PRIMARY KEY (id),
        CONSTRAINT FK_NetworkPlans_Network FOREIGN KEY (network_id) REFERENCES dbo.tbl_Networks(id),
        CONSTRAINT UQ_NetworkPlans_row UNIQUE (network_id, [year], [quarter], brand_as),
        CONSTRAINT CK_NetworkPlans_quarter CHECK ([quarter] BETWEEN 1 AND 4),
        CONSTRAINT CK_NetworkPlans_investments CHECK (investments_pct IS NULL OR (investments_pct >= 0 AND investments_pct <= 100))
    )
END;

-- Комментарий без года/квартала/бренда относится ко всей сети,
-- с заполненными полями — к конкретной ячейке плана.
IF OBJECT_ID('dbo.tbl_NetworkComments', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_NetworkComments (
        id              BIGINT IDENTITY(1,1) NOT NULL,
        network_id      INT NOT NULL,
        [year]          INT NULL,
        [quarter]       INT NULL,
        brand_as        NVARCHAR(255) NULL,
        user_name       NVARCHAR(100) NOT NULL,
        role            NVARCHAR(50) NOT NULL,
        comment_text    NVARCHAR(MAX) NOT NULL,
        created_at      DATETIME NOT NULL CONSTRAINT DF_NetworkComments_created_at DEFAULT GETDATE(),
        CONSTRAINT PK_NetworkComments PRIMARY KEY (id),
        CONSTRAINT FK_NetworkComments_Network FOREIGN KEY (network_id) REFERENCES dbo.tbl_Networks(id)
    )
END;

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_NetworkPlans_network_year' AND object_id = OBJECT_ID('dbo.tbl_NetworkPlans'))
    CREATE INDEX IX_NetworkPlans_network_year ON dbo.tbl_NetworkPlans(network_id, [year]) INCLUDE ([quarter], brand_as, plan_rub, investments_pct);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_NetworkPeriods_network_year' AND object_id = OBJECT_ID('dbo.tbl_NetworkPeriods'))
    CREATE INDEX IX_NetworkPeriods_network_year ON dbo.tbl_NetworkPeriods(network_id, [year]);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_NetworkComments_network' AND object_id = OBJECT_ID('dbo.tbl_NetworkComments'))
    CREATE INDEX IX_NetworkComments_network ON dbo.tbl_NetworkComments(network_id, id);

-- Планы в реестре вносят КАМы, поэтому роль появляется в списке допустимых.
IF EXISTS (SELECT 1 FROM sys.check_constraints WHERE name = 'CK_Users_role' AND parent_object_id = OBJECT_ID('dbo.tbl_Users'))
    ALTER TABLE dbo.tbl_Users DROP CONSTRAINT CK_Users_role;
IF EXISTS (SELECT 1 FROM sys.check_constraints WHERE name = 'CK6_Users_role' AND parent_object_id = OBJECT_ID('dbo.tbl_Users'))
    ALTER TABLE dbo.tbl_Users DROP CONSTRAINT CK6_Users_role;
IF NOT EXISTS (SELECT 1 FROM sys.check_constraints WHERE name = 'CK8_Users_role' AND parent_object_id = OBJECT_ID('dbo.tbl_Users'))
    ALTER TABLE dbo.tbl_Users WITH CHECK ADD CONSTRAINT CK8_Users_role CHECK (role IN ('admin','agreement1','agreement2','kam'));

-- +goose Down
IF EXISTS (SELECT 1 FROM sys.check_constraints WHERE name = 'CK8_Users_role' AND parent_object_id = OBJECT_ID('dbo.tbl_Users'))
    ALTER TABLE dbo.tbl_Users DROP CONSTRAINT CK8_Users_role;
-- Возвращаем исходный список ролей, если КАМов в базе не осталось.
IF NOT EXISTS (SELECT 1 FROM sys.check_constraints WHERE name = 'CK_Users_role' AND parent_object_id = OBJECT_ID('dbo.tbl_Users'))
   AND NOT EXISTS (SELECT 1 FROM dbo.tbl_Users WHERE role = 'kam')
    ALTER TABLE dbo.tbl_Users WITH CHECK ADD CONSTRAINT CK_Users_role CHECK (role IN ('admin','agreement1','agreement2'));
DROP TABLE IF EXISTS dbo.tbl_NetworkComments;
DROP TABLE IF EXISTS dbo.tbl_NetworkPlans;
DROP TABLE IF EXISTS dbo.tbl_NetworkPeriods;
DROP TABLE IF EXISTS dbo.tbl_Networks;
