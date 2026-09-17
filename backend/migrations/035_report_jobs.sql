-- +goose Up
-- +goose StatementBegin
-- Реестр фоновых заданий на отчёты PDF/PPTX по витрине реестра сетей.
--
-- Устройство повторяет tbl_SalesExportJobs: состояние в БД переживает
-- перезапуск, файл лежит в каталоге выгрузок не дольше TTL. Своя таблица, а не
-- та же: у отчёта есть формат и снимок запроса с данными, по которому файл
-- воспроизводим и объясним, а у Excel-выгрузки — число строк.
--
-- Задание — один файл: PDF и PPTX по одному запросу заводятся двумя записями
-- с одинаковым snapshot_json, чтобы оба формата собирались из одних чисел.
IF OBJECT_ID('dbo.tbl_ReportJobs', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_ReportJobs (
        id            NVARCHAR(36) NOT NULL,
        owner_name    NVARCHAR(100) NOT NULL,
        status        NVARCHAR(20) NOT NULL,
        format        NVARCHAR(10) NOT NULL,
        title         NVARCHAR(200) NOT NULL,
        file_name     NVARCHAR(255) NOT NULL,
        file_path     NVARCHAR(1000) NULL,
        error_text    NVARCHAR(500) NULL,
        snapshot_json NVARCHAR(MAX) NOT NULL,
        created_at    DATETIME2 NOT NULL CONSTRAINT DF_ReportJobs_created_at DEFAULT SYSUTCDATETIME(),
        completed_at  DATETIME2 NULL,
        CONSTRAINT PK_ReportJobs PRIMARY KEY (id),
        CONSTRAINT CK_ReportJobs_status CHECK (status IN ('queued', 'running', 'ready', 'failed')),
        CONSTRAINT CK_ReportJobs_format CHECK (format IN ('pdf', 'pptx'))
    );
END;
-- +goose StatementEnd

-- +goose StatementBegin
-- Обслуживание: чистка обходит реестр по времени создания, лимит активных
-- заданий считается по владельцу.
IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_ReportJobs_created_at' AND object_id = OBJECT_ID('dbo.tbl_ReportJobs')
)
BEGIN
    CREATE INDEX IX_ReportJobs_created_at ON dbo.tbl_ReportJobs (created_at);
END;
IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_ReportJobs_owner_status' AND object_id = OBJECT_ID('dbo.tbl_ReportJobs')
)
BEGIN
    CREATE INDEX IX_ReportJobs_owner_status ON dbo.tbl_ReportJobs (owner_name, status);
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
IF OBJECT_ID('dbo.tbl_ReportJobs', 'U') IS NOT NULL
    DROP TABLE dbo.tbl_ReportJobs;
-- +goose StatementEnd
