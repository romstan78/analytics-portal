-- +goose Up
-- +goose StatementBegin
-- Журнал обратимой очистки точных дублей промо.

IF OBJECT_ID('dbo.tbl_PromoDedupRuns', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_PromoDedupRuns (
        run_id          UNIQUEIDENTIFIER NOT NULL,
        plan_hash       CHAR(64) NOT NULL,
        status          NVARCHAR(20) NOT NULL,
        executed_by     NVARCHAR(100) NOT NULL,
        started_at      DATETIME2 NOT NULL CONSTRAINT DF_PromoDedupRuns_started_at DEFAULT SYSUTCDATETIME(),
        completed_at    DATETIME2 NULL,
        rolled_back_at  DATETIME2 NULL,
        stats_json      NVARCHAR(MAX) NULL,
        CONSTRAINT PK_PromoDedupRuns PRIMARY KEY (run_id),
        CONSTRAINT UQ_PromoDedupRuns_plan_hash UNIQUE (plan_hash),
        CONSTRAINT CK_PromoDedupRuns_status CHECK (status IN ('RUNNING', 'APPLIED', 'ROLLED_BACK'))
    );
END;

IF OBJECT_ID('dbo.tbl_PromoDedupChanges', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_PromoDedupChanges (
        run_id              UNIQUEIDENTIFIER NOT NULL,
        group_id            CHAR(64) NOT NULL,
        keeper_id           INT NOT NULL,
        duplicate_id        INT NOT NULL,
        original_deleted_at DATETIME NULL,
        original_updated_at DATETIME NOT NULL,
        original_updated_by NVARCHAR(255) NULL,
        CONSTRAINT PK_PromoDedupChanges PRIMARY KEY (run_id, duplicate_id),
        CONSTRAINT FK_PromoDedupChanges_Run FOREIGN KEY (run_id) REFERENCES dbo.tbl_PromoDedupRuns(run_id),
        CONSTRAINT FK_PromoDedupChanges_Keeper FOREIGN KEY (keeper_id) REFERENCES dbo.tbl_PromoActivities(id),
        CONSTRAINT FK_PromoDedupChanges_Duplicate FOREIGN KEY (duplicate_id) REFERENCES dbo.tbl_PromoActivities(id)
    );
END;

IF OBJECT_ID('dbo.tbl_PromoDedupRelatedMoves', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_PromoDedupRelatedMoves (
        run_id          UNIQUEIDENTIFIER NOT NULL,
        related_table   NVARCHAR(30) NOT NULL,
        related_id      BIGINT NOT NULL,
        from_promo_id   INT NOT NULL,
        to_promo_id     INT NOT NULL,
        CONSTRAINT PK_PromoDedupRelatedMoves PRIMARY KEY (run_id, related_table, related_id),
        CONSTRAINT FK_PromoDedupRelatedMoves_Run FOREIGN KEY (run_id) REFERENCES dbo.tbl_PromoDedupRuns(run_id),
        CONSTRAINT CK_PromoDedupRelatedMoves_table CHECK (related_table IN ('tbl_PromoComments', 'tbl_AuditLog'))
    );
END;

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_PromoDedupChanges_keeper' AND object_id = OBJECT_ID('dbo.tbl_PromoDedupChanges'))
    CREATE INDEX IX_PromoDedupChanges_keeper ON dbo.tbl_PromoDedupChanges(keeper_id);

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_PromoDedupRelatedMoves_from' AND object_id = OBJECT_ID('dbo.tbl_PromoDedupRelatedMoves'))
    CREATE INDEX IX_PromoDedupRelatedMoves_from ON dbo.tbl_PromoDedupRelatedMoves(from_promo_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS dbo.tbl_PromoDedupRelatedMoves;
DROP TABLE IF EXISTS dbo.tbl_PromoDedupChanges;
DROP TABLE IF EXISTS dbo.tbl_PromoDedupRuns;
-- +goose StatementEnd
