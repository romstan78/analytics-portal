-- +goose Up
-- +goose StatementBegin
-- Реестр фоновых выгрузок интернет-продаж.
--
-- Раньше он жил в памяти процесса: после перезапуска опрос статуса возвращал
-- 404, и выгрузку приходилось запускать заново. Пока приложение работает в
-- один процесс, это неудобство; со второй репликой задание терялось бы штатно
-- — клиент опросил бы статус у соседа, который о нём не знает.
--
-- file_path локален тому, кто готовил файл: чтобы выгрузка пережила и смену
-- реплики, каталог SALES_EXPORT_DIR выносится на общий том.
IF OBJECT_ID('dbo.tbl_SalesExportJobs', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_SalesExportJobs (
        id           NVARCHAR(36) NOT NULL,
        owner_name   NVARCHAR(100) NOT NULL,
        status       NVARCHAR(20) NOT NULL,
        total_rows   INT NOT NULL CONSTRAINT DF_SalesExportJobs_total_rows DEFAULT 0,
        file_name    NVARCHAR(255) NOT NULL,
        file_path    NVARCHAR(1000) NULL,
        error_text   NVARCHAR(500) NULL,
        created_at   DATETIME2 NOT NULL CONSTRAINT DF_SalesExportJobs_created_at DEFAULT SYSUTCDATETIME(),
        completed_at DATETIME2 NULL,
        CONSTRAINT PK_SalesExportJobs PRIMARY KEY (id),
        CONSTRAINT CK_SalesExportJobs_status CHECK (status IN ('queued', 'running', 'ready', 'failed'))
    );
END;
-- +goose StatementEnd

-- +goose StatementBegin
-- Обслуживание: чистка обходит реестр по времени создания.
IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_SalesExportJobs_created_at' AND object_id = OBJECT_ID('dbo.tbl_SalesExportJobs')
)
BEGIN
    CREATE INDEX IX_SalesExportJobs_created_at ON dbo.tbl_SalesExportJobs (created_at);
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
IF OBJECT_ID('dbo.tbl_SalesExportJobs', 'U') IS NOT NULL
    DROP TABLE dbo.tbl_SalesExportJobs;
-- +goose StatementEnd
