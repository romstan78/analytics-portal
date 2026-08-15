-- +goose Up
-- +goose StatementBegin
-- Полная схема интернет-продаж и справочников, используемых приложением.
-- Все CREATE защищены проверками для совместимости с существующей БД.

IF OBJECT_ID('dbo.tbl_EcomSalesConsolidated', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_EcomSalesConsolidated (
        id              INT IDENTITY(1,1) NOT NULL,
        [year]          INT NULL,
        [month]         INT NULL,
        brandName       NVARCHAR(255) NULL,
        productName     NVARCHAR(255) NULL,
        networkName     NVARCHAR(255) NULL,
        qty             DECIMAL(18,2) NULL,
        rub             DECIMAL(18,2) NULL,
        qty_ZC          DECIMAL(18,2) NULL,
        rub_ZC          DECIMAL(18,2) NULL,
        qty_PLZ         DECIMAL(18,2) NULL,
        rub_PLZ         DECIMAL(18,2) NULL,
        qty_AR          DECIMAL(18,2) NULL,
        rub_AR          DECIMAL(18,2) NULL,
        qty_OZ          DECIMAL(18,2) NULL,
        rub_OZ          DECIMAL(18,2) NULL,
        qty_EA          DECIMAL(18,2) NULL,
        rub_EA          DECIMAL(18,2) NULL,
        qty_PMP         DECIMAL(18,2) NULL,
        rub_PMP         DECIMAL(18,2) NULL,
        qty_OMNI        DECIMAL(18,2) NULL,
        rub_OMNI        DECIMAL(18,2) NULL,
        qty_NW          DECIMAL(18,2) NULL,
        rub_NW          DECIMAL(18,2) NULL,
        NW_wo_ecom      DECIMAL(18,2) NULL,
        SS_wo_ecom      DECIMAL(18,2) NULL,
        rub_NW_wo_ecom  DECIMAL(18,2) NULL,
        rub_SS_wo_ecom  DECIMAL(18,2) NULL,
        updated_at      DATETIME NULL CONSTRAINT DF_EcomSalesConsolidated_updated_at DEFAULT GETDATE(),
        CONSTRAINT PK_EcomSalesConsolidated PRIMARY KEY (id),
        CONSTRAINT CK_EcomSalesConsolidated_month CHECK ([month] IS NULL OR [month] BETWEEN 1 AND 12)
    )
END;

IF OBJECT_ID('dbo.tbl_ChannelSegmentMapping', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_ChannelSegmentMapping (
        id          INT IDENTITY(1,1) NOT NULL,
        name        NVARCHAR(256) NOT NULL,
        un_rub      NVARCHAR(100) NULL,
        segment     NVARCHAR(100) NULL,
        channel     NVARCHAR(100) NULL,
        CONSTRAINT PK_ChannelSegmentMapping PRIMARY KEY (id),
        CONSTRAINT UQ_ChannelSegmentMapping_name UNIQUE (name)
    )
END;

IF OBJECT_ID('dbo.tbl_EcomSalesNormalized', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_EcomSalesNormalized (
        id              BIGINT IDENTITY(1,1) NOT NULL,
        source_id       INT NOT NULL,
        [year]          INT NOT NULL,
        [month]         INT NOT NULL,
        brandName       NVARCHAR(255) NOT NULL,
        productName     NVARCHAR(255) NOT NULL,
        networkName     NVARCHAR(255) NOT NULL,
        metric_type     NVARCHAR(128) NOT NULL,
        metric_value    DECIMAL(18,2) NULL,
        updated_at      DATETIME NULL,
        un_rub          NVARCHAR(100) NULL,
        segment         NVARCHAR(128) NULL,
        channel         NVARCHAR(128) NULL,
        CONSTRAINT PK_EcomSalesNormalized PRIMARY KEY (id),
        CONSTRAINT FK_EcomSalesNormalized_source FOREIGN KEY (source_id)
            REFERENCES dbo.tbl_EcomSalesConsolidated(id) ON DELETE CASCADE,
        CONSTRAINT UQ_EcomSalesNormalized_source_metric UNIQUE (source_id, metric_type),
        CONSTRAINT CK_EcomSalesNormalized_month CHECK ([month] BETWEEN 1 AND 12)
    )
END;

IF OBJECT_ID('dbo.tbl_SKUMapping', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_SKUMapping (
        id          INT IDENTITY(1,1) NOT NULL,
        sku         NVARCHAR(255) NOT NULL,
        brand       NVARCHAR(255) NULL,
        brand_as    NVARCHAR(255) NULL,
        created_at  DATETIME NOT NULL CONSTRAINT DF_SKUMapping_created_at DEFAULT GETDATE(),
        CONSTRAINT PK_SKUMapping PRIMARY KEY (id),
        CONSTRAINT UQ_SKUMapping_sku UNIQUE (sku)
    )
END;

IF OBJECT_ID('dbo.tbl_KAMNetworkMapping', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_KAMNetworkMapping (
        id              INT IDENTITY(1,1) NOT NULL,
        kam             NVARCHAR(255) NOT NULL,
        network_name    NVARCHAR(255) NOT NULL,
        valid_from      DATE NOT NULL,
        created_at      DATETIME NOT NULL CONSTRAINT DF_KAMNetworkMapping_created_at DEFAULT GETDATE(),
        CONSTRAINT PK_KAMNetworkMapping PRIMARY KEY (id),
        CONSTRAINT UQ_KAMNetworkMapping_period UNIQUE (kam, network_name, valid_from)
    )
END;

IF OBJECT_ID('dbo.tbl_MechanicsChannelMapping', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_MechanicsChannelMapping (
        id          INT IDENTITY(1,1) NOT NULL,
        mechanics   NVARCHAR(255) NOT NULL,
        channel     NVARCHAR(100) NOT NULL,
        created_at  DATETIME NOT NULL CONSTRAINT DF_MechanicsChannelMapping_created_at DEFAULT GETDATE(),
        CONSTRAINT PK_MechanicsChannelMapping PRIMARY KEY (id),
        CONSTRAINT UQ_MechanicsChannelMapping_mechanics UNIQUE (mechanics)
    )
END;

IF OBJECT_ID('dbo.tbl_NetworkGeoMapping', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_NetworkGeoMapping (
        id              INT IDENTITY(1,1) NOT NULL,
        network_name    NVARCHAR(256) NOT NULL,
        kam             NVARCHAR(128) NULL,
        network_type    NVARCHAR(128) NULL,
        top20_segment   NVARCHAR(128) NULL,
        key_region      NVARCHAR(128) NULL,
        CONSTRAINT PK_NetworkGeoMapping PRIMARY KEY (id),
        CONSTRAINT UQ_NetworkGeoMapping_network UNIQUE (network_name)
    )
END;

-- Ограничения для баз, где основные таблицы были созданы до появления миграций.
IF NOT EXISTS (SELECT 1 FROM sys.check_constraints WHERE name IN ('CK_PromoActivities_month','CK6_PromoActivities_month') AND parent_object_id = OBJECT_ID('dbo.tbl_PromoActivities'))
    ALTER TABLE dbo.tbl_PromoActivities WITH CHECK ADD CONSTRAINT CK6_PromoActivities_month CHECK ([month] IS NULL OR [month] BETWEEN 1 AND 12);
IF NOT EXISTS (SELECT 1 FROM sys.check_constraints WHERE name IN ('CK_PromoActivities_quarter','CK6_PromoActivities_quarter') AND parent_object_id = OBJECT_ID('dbo.tbl_PromoActivities'))
    ALTER TABLE dbo.tbl_PromoActivities WITH CHECK ADD CONSTRAINT CK6_PromoActivities_quarter CHECK (quarter IS NULL OR quarter BETWEEN 1 AND 4);
IF NOT EXISTS (SELECT 1 FROM sys.check_constraints WHERE name IN ('CK_PromoActivities_agreement1_status','CK6_PromoActivities_agreement1_status') AND parent_object_id = OBJECT_ID('dbo.tbl_PromoActivities'))
    ALTER TABLE dbo.tbl_PromoActivities WITH CHECK ADD CONSTRAINT CK6_PromoActivities_agreement1_status CHECK (agreement1_status IS NULL OR agreement1_status IN ('pending','commented','approved','rejected'));
IF NOT EXISTS (SELECT 1 FROM sys.check_constraints WHERE name IN ('CK_PromoActivities_agreement2_status','CK6_PromoActivities_agreement2_status') AND parent_object_id = OBJECT_ID('dbo.tbl_PromoActivities'))
    ALTER TABLE dbo.tbl_PromoActivities WITH CHECK ADD CONSTRAINT CK6_PromoActivities_agreement2_status CHECK (agreement2_status IS NULL OR agreement2_status IN ('pending','commented','approved','rejected'));
IF NOT EXISTS (SELECT 1 FROM sys.check_constraints WHERE name IN ('CK_Users_role','CK6_Users_role') AND parent_object_id = OBJECT_ID('dbo.tbl_Users'))
    ALTER TABLE dbo.tbl_Users WITH CHECK ADD CONSTRAINT CK6_Users_role CHECK (role IN ('admin','agreement1','agreement2'));

-- Индексы для основных фильтров и справочников.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE object_id = OBJECT_ID('dbo.tbl_EcomSalesConsolidated') AND name = 'IX_EcomSalesConsolidated_year_month')
    CREATE INDEX IX_EcomSalesConsolidated_year_month ON dbo.tbl_EcomSalesConsolidated([year], [month]) INCLUDE (id, updated_at);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE object_id = OBJECT_ID('dbo.tbl_EcomSalesNormalized') AND name = 'IX_EcomSalesNormalized_filters')
    CREATE INDEX IX_EcomSalesNormalized_filters ON dbo.tbl_EcomSalesNormalized([year], [month], brandName, networkName) INCLUDE (metric_type, metric_value, un_rub, segment, channel, updated_at);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE object_id = OBJECT_ID('dbo.tbl_EcomSalesNormalized') AND name = 'IX_EcomSalesNormalized_channel')
    CREATE INDEX IX_EcomSalesNormalized_channel ON dbo.tbl_EcomSalesNormalized(channel, segment, un_rub);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE object_id = OBJECT_ID('dbo.tbl_SKUMapping') AND name IN ('UQ_SKUMapping_sku','UX_SKUMapping_sku'))
    CREATE UNIQUE INDEX UX_SKUMapping_sku ON dbo.tbl_SKUMapping(sku);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE object_id = OBJECT_ID('dbo.tbl_KAMNetworkMapping') AND name IN ('UQ_KAMNetworkMapping_period','UX_KAMNetworkMapping_period'))
    CREATE UNIQUE INDEX UX_KAMNetworkMapping_period ON dbo.tbl_KAMNetworkMapping(kam, network_name, valid_from);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE object_id = OBJECT_ID('dbo.tbl_MechanicsChannelMapping') AND name IN ('UQ_MechanicsChannelMapping_mechanics','UX_MechanicsChannelMapping_mechanics'))
    CREATE UNIQUE INDEX UX_MechanicsChannelMapping_mechanics ON dbo.tbl_MechanicsChannelMapping(mechanics);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE object_id = OBJECT_ID('dbo.tbl_ChannelSegmentMapping') AND name IN ('UQ_ChannelSegmentMapping_name','UX_ChannelSegmentMapping_name'))
    CREATE UNIQUE INDEX UX_ChannelSegmentMapping_name ON dbo.tbl_ChannelSegmentMapping(name);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE object_id = OBJECT_ID('dbo.tbl_EcomSalesNormalized') AND name IN ('UQ_EcomSalesNormalized_source_metric','UX_EcomSalesNormalized_source_metric'))
   AND NOT EXISTS (
       SELECT 1 FROM dbo.tbl_EcomSalesNormalized
       GROUP BY source_id, metric_type
       HAVING COUNT(*) > 1
   )
    CREATE UNIQUE INDEX UX_EcomSalesNormalized_source_metric ON dbo.tbl_EcomSalesNormalized(source_id, metric_type);

-- Для существующей нормализованной таблицы добавляем связь с источником.
IF NOT EXISTS (SELECT 1 FROM sys.foreign_keys WHERE name = 'FK_EcomSalesNormalized_source')
   AND NOT EXISTS (
       SELECT 1 FROM dbo.tbl_EcomSalesNormalized n
       LEFT JOIN dbo.tbl_EcomSalesConsolidated s ON s.id = n.source_id
       WHERE s.id IS NULL
   )
BEGIN
    ALTER TABLE dbo.tbl_EcomSalesNormalized WITH CHECK
        ADD CONSTRAINT FK_EcomSalesNormalized_source FOREIGN KEY (source_id)
        REFERENCES dbo.tbl_EcomSalesConsolidated(id) ON DELETE CASCADE;
END;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
-- Миграция согласует новую и историческую схемы. Нельзя определить, какие
-- таблицы существовали до Goose, поэтому Down намеренно не удаляет данные.
SELECT 1;
-- +goose StatementEnd
