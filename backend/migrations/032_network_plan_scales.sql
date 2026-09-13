-- +goose Up
-- Ступени контракта: план сети с двумя–тремя порогами объёма, у каждого свой
-- процент инвестиций, с крышкой перевыполнения и исключениями по SKU.
--
-- До этой миграции у строки плана «бренд × квартал» была одна ступень —
-- plan_rub и investments_pct, — и правило было одно: объём × процент, если
-- объём закрыл план области. Ступени обобщают порог: достигнутая ступень —
-- наивысшая, чей порог закрыт объёмом, процент берётся её. Крышка ограничивает
-- базу к оплате: открытая (весь объём), процентная (план × (1 + c%) на
-- последней ступени) или закрытая (ровно план ступени). План, процент и крышка
-- могут отличаться у отдельного SKU внутри бренда.
--
-- tbl_NetworkPlans остаётся строкой-итогом: её читают витрина, пересчёт и
-- внешние потребители, и ломать их незачем. Ступень 1 дублирует plan_rub и
-- investments_pct строки намеренно — так ступени объясняют итог целиком, а
-- источником ступени 1 остаётся сама строка плана.
--
-- Каждая существующая строка переносится в ступень 1 с открытой крышкой: это
-- ровно сегодняшнее правило, и пересчёт после миграции даёт те же числа.
--
-- План вводится в рублях или упаковках по режиму бренда (entry_unit); вторая
-- метрика считается по цене контракта и хранится рядом — тем же обещанием,
-- что и пара прогноза: колонки читают без пересчёта по прайсу.

-- ─── Число ступеней и крышка по умолчанию — в профиле сети ──────────────────
IF COL_LENGTH('dbo.tbl_Networks', 'default_scales_count') IS NULL
    ALTER TABLE dbo.tbl_Networks ADD default_scales_count TINYINT NOT NULL
        CONSTRAINT DF_Networks_default_scales_count DEFAULT 1;
IF COL_LENGTH('dbo.tbl_Networks', 'default_cap_mode') IS NULL
    ALTER TABLE dbo.tbl_Networks ADD default_cap_mode NVARCHAR(10) NOT NULL
        CONSTRAINT DF_Networks_default_cap_mode DEFAULT 'open';
IF NOT EXISTS (
    SELECT 1 FROM sys.check_constraints
    WHERE name = 'CK_Networks_scales' AND parent_object_id = OBJECT_ID('dbo.tbl_Networks')
)
    ALTER TABLE dbo.tbl_Networks WITH CHECK ADD CONSTRAINT CK_Networks_scales CHECK (
        default_scales_count BETWEEN 1 AND 3 AND default_cap_mode IN ('open', 'pct', 'closed')
    );

-- Поквартальное исключение: NULL — как в профиле сети.
IF COL_LENGTH('dbo.tbl_NetworkPeriods', 'scales_count') IS NULL
    ALTER TABLE dbo.tbl_NetworkPeriods ADD scales_count TINYINT NULL;
IF NOT EXISTS (
    SELECT 1 FROM sys.check_constraints
    WHERE name = 'CK_NetworkPeriods_scales' AND parent_object_id = OBJECT_ID('dbo.tbl_NetworkPeriods')
)
    ALTER TABLE dbo.tbl_NetworkPeriods WITH CHECK ADD CONSTRAINT CK_NetworkPeriods_scales CHECK (
        scales_count IS NULL OR scales_count BETWEEN 1 AND 3
    );

-- ─── Крышка и итог правила — на строке плана ────────────────────────────────
-- Крышка — свойство владельца порога: у валового контракта это строка пула,
-- у отдельного бренда — его строка. Открытая крышка равна сегодняшнему
-- поведению, поэтому она и есть умолчание.
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'cap_mode') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD cap_mode NVARCHAR(10) NOT NULL
        CONSTRAINT DF_NetworkPlans_cap_mode DEFAULT 'open';
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'cap_pct') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD cap_pct DECIMAL(7,2) NULL;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'forecast_scale') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD forecast_scale TINYINT NULL;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'fact_scale') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD fact_scale TINYINT NULL;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'forecast_base_rub') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD forecast_base_rub DECIMAL(18,2) NULL;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'fact_base_rub') IS NULL
    ALTER TABLE dbo.tbl_NetworkPlans ADD fact_base_rub DECIMAL(18,2) NULL;
IF NOT EXISTS (
    SELECT 1 FROM sys.check_constraints
    WHERE name = 'CK_NetworkPlans_cap' AND parent_object_id = OBJECT_ID('dbo.tbl_NetworkPlans')
)
    ALTER TABLE dbo.tbl_NetworkPlans WITH CHECK ADD CONSTRAINT CK_NetworkPlans_cap CHECK (
        cap_mode IN ('open', 'pct', 'closed') AND (cap_pct IS NULL OR cap_pct >= 0)
    );

-- ─── Ступени строки плана ───────────────────────────────────────────────────
-- plan_rub — порог ступени у пула и отдельного бренда; у валового бренда —
-- его план на этой ступени, введённый руками: порог валового контракта
-- меряется по пулу, а бренды его распределяют, как и на ступени 1.
--
-- Расчётные колонки заполняются правилом для каждой ступени: прогнозные и
-- фактические — «как если бы эта ступень была достигнутой», а какая достигнута
-- на самом деле, говорят *_reached здесь и forecast_scale / fact_scale строки
-- плана. Так у потребителя есть и итог, и «сколько было бы» на каждой ступени.
IF OBJECT_ID('dbo.tbl_NetworkPlanScales', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_NetworkPlanScales (
        id                            BIGINT IDENTITY(1,1) NOT NULL,
        plan_id                       INT NOT NULL,
        scale_no                      TINYINT NOT NULL,
        plan_rub                      DECIMAL(18,2) NULL,
        plan_units                    DECIMAL(18,2) NULL,
        investments_pct               DECIMAL(5,2) NULL,
        plan_investments_rub          DECIMAL(18,2) NULL,
        plan_investments_rub_net      DECIMAL(18,2) NULL,
        forecast_rub                  DECIMAL(18,2) NULL,
        forecast_base_rub             DECIMAL(18,2) NULL,
        forecast_investments_rub      DECIMAL(18,2) NULL,
        forecast_investments_rub_net  DECIMAL(18,2) NULL,
        forecast_reached              BIT NOT NULL CONSTRAINT DF_NetworkPlanScales_fcst_reached DEFAULT 0,
        fact_rub                      DECIMAL(18,2) NULL,
        fact_base_rub                 DECIMAL(18,2) NULL,
        fact_investments_rub          DECIMAL(18,2) NULL,
        fact_investments_rub_net      DECIMAL(18,2) NULL,
        fact_reached                  BIT NOT NULL CONSTRAINT DF_NetworkPlanScales_fact_reached DEFAULT 0,
        updated_by                    NVARCHAR(100) NULL,
        created_at                    DATETIME NOT NULL CONSTRAINT DF_NetworkPlanScales_created DEFAULT GETDATE(),
        updated_at                    DATETIME NOT NULL CONSTRAINT DF_NetworkPlanScales_updated DEFAULT GETDATE(),
        CONSTRAINT PK_NetworkPlanScales PRIMARY KEY (id),
        -- Строка плана уходит вместе со ступенями: бренд, убранный из плана
        -- года, не должен оставлять после себя лестницу без владельца.
        CONSTRAINT FK_NetworkPlanScales_Plan FOREIGN KEY (plan_id)
            REFERENCES dbo.tbl_NetworkPlans(id) ON DELETE CASCADE,
        CONSTRAINT UQ_NetworkPlanScales_scale UNIQUE (plan_id, scale_no),
        CONSTRAINT CK_NetworkPlanScales_no CHECK (scale_no BETWEEN 1 AND 3),
        CONSTRAINT CK_NetworkPlanScales_values CHECK (
            (plan_rub IS NULL OR plan_rub >= 0) AND
            (plan_units IS NULL OR plan_units >= 0) AND
            (investments_pct IS NULL OR (investments_pct >= 0 AND investments_pct <= 100))
        )
    )
END;

-- ─── SKU на ступени ─────────────────────────────────────────────────────────
-- Строки заводятся только там, где раскладка по SKU меняет результат: у бренда
-- с крышкой не «открытая» или с исключением по проценту. Пустые investments_pct
-- и cap_mode означают «как у бренда» и «как у владельца порога».
IF OBJECT_ID('dbo.tbl_NetworkPlanScaleSKU', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_NetworkPlanScaleSKU (
        id                            BIGINT IDENTITY(1,1) NOT NULL,
        scale_id                      BIGINT NOT NULL,
        sku                           NVARCHAR(255) NOT NULL,
        plan_rub                      DECIMAL(18,2) NULL,
        plan_units                    DECIMAL(18,2) NULL,
        investments_pct               DECIMAL(5,2) NULL,
        cap_mode                      NVARCHAR(10) NULL,
        cap_pct                       DECIMAL(7,2) NULL,
        plan_investments_rub          DECIMAL(18,2) NULL,
        plan_investments_rub_net      DECIMAL(18,2) NULL,
        forecast_rub                  DECIMAL(18,2) NULL,
        forecast_base_rub             DECIMAL(18,2) NULL,
        forecast_investments_rub      DECIMAL(18,2) NULL,
        forecast_investments_rub_net  DECIMAL(18,2) NULL,
        fact_rub                      DECIMAL(18,2) NULL,
        fact_base_rub                 DECIMAL(18,2) NULL,
        fact_investments_rub          DECIMAL(18,2) NULL,
        fact_investments_rub_net      DECIMAL(18,2) NULL,
        created_at                    DATETIME NOT NULL CONSTRAINT DF_NetworkPlanScaleSKU_created DEFAULT GETDATE(),
        updated_at                    DATETIME NOT NULL CONSTRAINT DF_NetworkPlanScaleSKU_updated DEFAULT GETDATE(),
        CONSTRAINT PK_NetworkPlanScaleSKU PRIMARY KEY (id),
        CONSTRAINT FK_NetworkPlanScaleSKU_Scale FOREIGN KEY (scale_id)
            REFERENCES dbo.tbl_NetworkPlanScales(id) ON DELETE CASCADE,
        CONSTRAINT UQ_NetworkPlanScaleSKU_sku UNIQUE (scale_id, sku),
        CONSTRAINT CK_NetworkPlanScaleSKU_values CHECK (
            (plan_rub IS NULL OR plan_rub >= 0) AND
            (plan_units IS NULL OR plan_units >= 0) AND
            (investments_pct IS NULL OR (investments_pct >= 0 AND investments_pct <= 100)) AND
            (cap_mode IS NULL OR cap_mode IN ('open', 'pct', 'closed')) AND
            (cap_pct IS NULL OR cap_pct >= 0)
        )
    )
END;

-- Ступени читаются вместе со строками плана года — покрывающий индекс по plan_id.
IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_NetworkPlanScales_plan' AND object_id = OBJECT_ID('dbo.tbl_NetworkPlanScales')
)
    CREATE INDEX IX_NetworkPlanScales_plan ON dbo.tbl_NetworkPlanScales(plan_id, scale_no)
        INCLUDE (plan_rub, plan_units, investments_pct, updated_at);
IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_NetworkPlanScaleSKU_scale' AND object_id = OBJECT_ID('dbo.tbl_NetworkPlanScaleSKU')
)
    CREATE INDEX IX_NetworkPlanScaleSKU_scale ON dbo.tbl_NetworkPlanScaleSKU(scale_id, sku)
        INCLUDE (plan_rub, plan_units, investments_pct, cap_mode, cap_pct);

-- ─── Перенос: каждая строка плана — ступень 1 ───────────────────────────────
-- Открытая крышка стоит умолчанием колонки, расчётные колонки ступени
-- переносятся из строки как есть: до пересчёта ступень 1 равна строке плана
-- буквально, после пересчёта — по правилу, и оба раза числа те же.
-- +goose StatementBegin
INSERT INTO dbo.tbl_NetworkPlanScales (
    plan_id, scale_no, plan_rub, plan_units, investments_pct,
    plan_investments_rub, plan_investments_rub_net,
    forecast_rub, forecast_investments_rub, forecast_investments_rub_net,
    fact_rub, fact_investments_rub, fact_investments_rub_net,
    updated_by, updated_at
)
SELECT p.id, 1, p.plan_rub, p.plan_units, p.investments_pct,
       p.plan_investments_rub, p.plan_investments_rub_net,
       p.forecast_rub, p.forecast_investments_rub, p.forecast_investments_rub_net,
       p.fact_rub, p.fact_investments_rub, p.fact_investments_rub_net,
       p.updated_by, p.updated_at
  FROM dbo.tbl_NetworkPlans p
 WHERE NOT EXISTS (
    SELECT 1 FROM dbo.tbl_NetworkPlanScales s WHERE s.plan_id = p.id AND s.scale_no = 1
 );
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS dbo.tbl_NetworkPlanScaleSKU;
DROP TABLE IF EXISTS dbo.tbl_NetworkPlanScales;

IF EXISTS (SELECT 1 FROM sys.check_constraints WHERE name = 'CK_NetworkPlans_cap' AND parent_object_id = OBJECT_ID('dbo.tbl_NetworkPlans'))
    ALTER TABLE dbo.tbl_NetworkPlans DROP CONSTRAINT CK_NetworkPlans_cap;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'cap_mode') IS NOT NULL
BEGIN
    ALTER TABLE dbo.tbl_NetworkPlans DROP CONSTRAINT DF_NetworkPlans_cap_mode;
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN cap_mode;
END;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'cap_pct') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN cap_pct;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'forecast_scale') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN forecast_scale;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'fact_scale') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN fact_scale;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'forecast_base_rub') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN forecast_base_rub;
IF COL_LENGTH('dbo.tbl_NetworkPlans', 'fact_base_rub') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPlans DROP COLUMN fact_base_rub;

IF EXISTS (SELECT 1 FROM sys.check_constraints WHERE name = 'CK_NetworkPeriods_scales' AND parent_object_id = OBJECT_ID('dbo.tbl_NetworkPeriods'))
    ALTER TABLE dbo.tbl_NetworkPeriods DROP CONSTRAINT CK_NetworkPeriods_scales;
IF COL_LENGTH('dbo.tbl_NetworkPeriods', 'scales_count') IS NOT NULL
    ALTER TABLE dbo.tbl_NetworkPeriods DROP COLUMN scales_count;

IF EXISTS (SELECT 1 FROM sys.check_constraints WHERE name = 'CK_Networks_scales' AND parent_object_id = OBJECT_ID('dbo.tbl_Networks'))
    ALTER TABLE dbo.tbl_Networks DROP CONSTRAINT CK_Networks_scales;
IF COL_LENGTH('dbo.tbl_Networks', 'default_scales_count') IS NOT NULL
BEGIN
    ALTER TABLE dbo.tbl_Networks DROP CONSTRAINT DF_Networks_default_scales_count;
    ALTER TABLE dbo.tbl_Networks DROP COLUMN default_scales_count;
END;
IF COL_LENGTH('dbo.tbl_Networks', 'default_cap_mode') IS NOT NULL
BEGIN
    ALTER TABLE dbo.tbl_Networks DROP CONSTRAINT DF_Networks_default_cap_mode;
    ALTER TABLE dbo.tbl_Networks DROP COLUMN default_cap_mode;
END;
