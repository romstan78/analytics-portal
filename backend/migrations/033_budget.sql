-- +goose Up
-- Sources are user selections per quarter, separately for turnover and GTN.
IF EXISTS (SELECT 1 FROM sys.check_constraints WHERE name='CK8_Users_role' AND parent_object_id=OBJECT_ID('dbo.tbl_Users'))
    ALTER TABLE dbo.tbl_Users DROP CONSTRAINT CK8_Users_role;
IF EXISTS (SELECT 1 FROM sys.check_constraints WHERE name='CK_Users_role' AND parent_object_id=OBJECT_ID('dbo.tbl_Users'))
    ALTER TABLE dbo.tbl_Users DROP CONSTRAINT CK_Users_role;
IF EXISTS (SELECT 1 FROM sys.check_constraints WHERE name='CK6_Users_role' AND parent_object_id=OBJECT_ID('dbo.tbl_Users'))
    ALTER TABLE dbo.tbl_Users DROP CONSTRAINT CK6_Users_role;
ALTER TABLE dbo.tbl_Users ADD CONSTRAINT CK33_Users_role CHECK(role IN ('admin','analyst','agreement1','agreement2','kam'));

CREATE TABLE dbo.tbl_BudgetVersions (
    id INT IDENTITY PRIMARY KEY, [year] INT NOT NULL,
    code VARCHAR(4) NOT NULL CHECK(code IN ('B','F1','F2','F3','LIVE')),
    name NVARCHAR(200) NOT NULL, status VARCHAR(10) NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','frozen')),
    sources NVARCHAR(MAX) NOT NULL CHECK(ISJSON(sources)=1),
    metadata NVARCHAR(MAX) NULL,
    frozen_at DATETIME2(7) NULL, created_by NVARCHAR(255) NOT NULL,
    created_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT UQ_BudgetVersions_year_code UNIQUE([year],code)
);
CREATE TABLE dbo.tbl_BudgetLines (
    version_id INT NOT NULL REFERENCES dbo.tbl_BudgetVersions(id),
    network_id INT NOT NULL, network_name NVARCHAR(255) NOT NULL,
    brand_as NVARCHAR(255) NOT NULL, quarter TINYINT NOT NULL CHECK(quarter BETWEEN 1 AND 4),
    metric VARCHAR(40) NOT NULL, value_rub DECIMAL(24,2) NULL, value_net DECIMAL(24,2) NULL,
    value_units DECIMAL(24,4) NULL, mark VARCHAR(12) NOT NULL DEFAULT 'none',
    source VARCHAR(10) NOT NULL CHECK(source IN ('registry','manual')),
    base_rub DECIMAL(24,2) NULL, base_net DECIMAL(24,2) NULL,
    updated_by NVARCHAR(255) NOT NULL, updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT PK_BudgetLines PRIMARY KEY(version_id,network_id,brand_as,quarter,metric,source)
);
CREATE TABLE dbo.tbl_BudgetPromoLines (
    version_id INT NOT NULL REFERENCES dbo.tbl_BudgetVersions(id), promo_id INT NOT NULL,
    network_id INT NOT NULL, network_name NVARCHAR(255) NOT NULL, brand_as NVARCHAR(255) NOT NULL,
    quarter TINYINT NOT NULL, month TINYINT NOT NULL, gtn_opex NVARCHAR(50) NOT NULL,
    status_norm NVARCHAR(30) NOT NULL, mechanics NVARCHAR(255) NOT NULL,
    plan_rub DECIMAL(24,2) NOT NULL, fact_rub DECIMAL(24,2) NOT NULL,
    budget_rub DECIMAL(24,2) NOT NULL, budget_net DECIMAL(24,2) NOT NULL,
    included BIT NOT NULL, included_by NVARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT PK_BudgetPromoLines PRIMARY KEY(version_id,promo_id)
);
CREATE TABLE dbo.tbl_BudgetOlapPrices (
    version_id INT NOT NULL REFERENCES dbo.tbl_BudgetVersions(id), sku NVARCHAR(255) NOT NULL,
    price DECIMAL(24,6) NOT NULL, source_year INT NOT NULL, source_month INT NOT NULL,
    CONSTRAINT PK_BudgetOlapPrices PRIMARY KEY(version_id,sku)
);
CREATE TABLE dbo.tbl_BudgetPromoStatusMap (
    raw_status NVARCHAR(100) NOT NULL, agreement_rule VARCHAR(20) NOT NULL,
    status_norm NVARCHAR(30) NOT NULL, priority INT NOT NULL,
    CONSTRAINT PK_BudgetPromoStatusMap PRIMARY KEY(raw_status,agreement_rule)
);
INSERT dbo.tbl_BudgetPromoStatusMap(raw_status,agreement_rule,status_norm,priority) VALUES
    (N'*','rejected',N'исключено',100),
    (N'отменено','any',N'исключено',100),(N'отклонено','any',N'исключено',100),
    (N'cancelled','any',N'исключено',100),(N'rejected','any',N'исключено',100),
    (N'проведено','any',N'проведено',90),(N'финализировано','any',N'проведено',90),
    (N'*','approved',N'согласовано',80),
    (N'в процессе согласования','any',N'на согласовании',70),
    (N'в процессе','any',N'на согласовании',70),
    (N'*','pending',N'на согласовании',60),
    (N'*','any',N'черновик',0);

-- +goose Down
-- Refuse to discard financial snapshots on an accidental downgrade.
THROW 50001, 'Budget migration requires an explicit data retention plan before rollback', 1;
