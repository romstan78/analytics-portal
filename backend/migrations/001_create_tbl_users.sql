-- +goose Up
-- +goose StatementBegin
-- Базовые таблицы, необходимые следующим миграциям.
-- tbl_PromoActivities создаётся здесь, потому что миграции 002-005
-- добавляют к ней поля, внешние ключи и индексы.

IF OBJECT_ID('dbo.tbl_PromoActivities', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_PromoActivities (
        id                              INT IDENTITY(1,1) NOT NULL,
        network_name                    NVARCHAR(255) NULL,
        kam                             NVARCHAR(255) NULL,
        id_directum                     NVARCHAR(100) NULL,
        ds_number                       NVARCHAR(100) NULL,
        [year]                          INT NULL,
        [month]                         INT NULL,
        quarter                         INT NULL,
        sku                             NVARCHAR(255) NULL,
        brand                           NVARCHAR(255) NULL,
        brand_as                        NVARCHAR(255) NULL,
        mechanics                       NVARCHAR(500) NULL,
        discount_amount                 DECIMAL(18,2) NULL,
        gtn_opex                        NVARCHAR(100) NULL,
        conditions                      NVARCHAR(MAX) NULL,
        comments                        NVARCHAR(MAX) NULL,
        baseline_units                  DECIMAL(18,2) NULL,
        plan_promo_units                DECIMAL(18,2) NULL,
        plan_investments_rub            DECIMAL(18,2) NULL,
        plan_promo_rub                  DECIMAL(18,2) NULL,
        plan_promo_uplift_units         DECIMAL(18,2) NULL,
        plan_promo_uplift_pct_units     DECIMAL(18,4) NULL,
        plan_promo_uplift_rub           DECIMAL(18,2) NULL,
        plan_promo_uplift_pct_rub       DECIMAL(18,4) NULL,
        plan_investments_pct            DECIMAL(18,4) NULL,
        plan_roi                        DECIMAL(18,4) NULL,
        category_dynamics               DECIMAL(18,2) NULL,
        actual_corrected_baseline       DECIMAL(18,2) NULL,
        actual_network_sales_units      DECIMAL(18,2) NULL,
        actual_promo_sales_units        DECIMAL(18,2) NULL,
        actual_investments              DECIMAL(18,2) NULL,
        ecom_segment                    NVARCHAR(255) NULL,
        actual_external_ecom_units      DECIMAL(18,2) NULL,
        status                          NVARCHAR(100) NULL,
        agreement1                      NVARCHAR(255) NULL,
        agreement2                      NVARCHAR(255) NULL,
        baseline_rub                    DECIMAL(18,2) NULL,
        max_sales_units                 DECIMAL(18,2) NULL,
        contract_price                  DECIMAL(18,2) NULL,
        gm                              DECIMAL(18,2) NULL,
        total_pharmacies                INT NULL,
        promo_pharmacies                INT NULL,
        actual_promo_rub                DECIMAL(18,2) NULL,
        actual_promo_uplift_units       DECIMAL(18,2) NULL,
        actual_promo_uplift_rub         DECIMAL(18,2) NULL,
        net_promo_uplift_rub            DECIMAL(18,2) NULL,
        net_promo_uplift_pct            DECIMAL(18,4) NULL,
        actual_investments_pct          DECIMAL(18,4) NULL,
        actual_roi                      DECIMAL(18,4) NULL,
        actual_promo_rub_wo_ecom        DECIMAL(18,2) NULL,
        actual_promo_uplift_units_wo_ecom DECIMAL(18,2) NULL,
        actual_promo_uplift_rub_wo_ecom DECIMAL(18,2) NULL,
        net_promo_uplift_rub_wo_ecom    DECIMAL(18,2) NULL,
        net_promo_uplift_pct_wo_ecom    DECIMAL(18,4) NULL,
        actual_investments_pct_wo_ecom  DECIMAL(18,4) NULL,
        actual_roi_wo_ecom              DECIMAL(18,4) NULL,
        plan_vs_fact_rub                DECIMAL(18,2) NULL,
        plan_vs_fact_investments        DECIMAL(18,2) NULL,
        key_region                      NVARCHAR(255) NULL,
        turnover_per_point              DECIMAL(18,4) NULL,
        turnover_per_point_promo        DECIMAL(18,4) NULL,
        top20_segment                   NVARCHAR(255) NULL,
        olap_price                      DECIMAL(18,2) NULL,
        plan_promo_cip_olap             DECIMAL(18,2) NULL,
        fact_promo_cip_olap             DECIMAL(18,2) NULL,
        plan_promo_uplift_cip_olap      DECIMAL(18,2) NULL,
        fact_promo_uplift_cip_olap      DECIMAL(18,2) NULL,
        [date]                          DATE NULL,
        created_at                      DATETIME NOT NULL CONSTRAINT DF_PromoActivities_created_at DEFAULT GETDATE(),
        updated_at                      DATETIME NOT NULL CONSTRAINT DF_PromoActivities_updated_at DEFAULT GETDATE(),
        created_by                      NVARCHAR(255) NULL,
        updated_by                      NVARCHAR(255) NULL,
        deleted_at                      DATETIME NULL,
        agreement1_status               NVARCHAR(20) NULL,
        agreement1_comment              NVARCHAR(MAX) NULL,
        agreement2_status               NVARCHAR(20) NULL,
        agreement2_comment              NVARCHAR(MAX) NULL,
        CONSTRAINT PK_PromoActivities PRIMARY KEY (id),
        CONSTRAINT CK_PromoActivities_month CHECK ([month] IS NULL OR [month] BETWEEN 1 AND 12),
        CONSTRAINT CK_PromoActivities_quarter CHECK (quarter IS NULL OR quarter BETWEEN 1 AND 4),
        CONSTRAINT CK_PromoActivities_agreement1_status CHECK (agreement1_status IS NULL OR agreement1_status IN ('pending','commented','approved','rejected')),
        CONSTRAINT CK_PromoActivities_agreement2_status CHECK (agreement2_status IS NULL OR agreement2_status IN ('pending','commented','approved','rejected'))
    )
END;

IF OBJECT_ID('dbo.tbl_Users', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_Users (
        id              INT IDENTITY(1,1) NOT NULL,
        username        NVARCHAR(100) NOT NULL,
        password_hash   NVARCHAR(255) NOT NULL,
        role            NVARCHAR(50) NOT NULL CONSTRAINT DF_Users_role DEFAULT 'agreement1',
        created_at      DATETIME NOT NULL CONSTRAINT DF_Users_created_at DEFAULT GETDATE(),
        updated_at      DATETIME NOT NULL CONSTRAINT DF_Users_updated_at DEFAULT GETDATE(),
        deleted_at      DATETIME NULL,
        CONSTRAINT PK_Users PRIMARY KEY (id),
        CONSTRAINT UQ_Users_username UNIQUE (username),
        CONSTRAINT CK_Users_role CHECK (role IN ('admin','agreement1','agreement2'))
    )
END;

-- Пользователи намеренно не создаются миграцией. Используйте bootstrap-user
-- и передайте уникальный пароль через окружение.

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS dbo.tbl_Users;
-- tbl_PromoActivities могла существовать до внедрения Goose, поэтому
-- автоматический Down намеренно её не удаляет.
-- +goose StatementEnd
