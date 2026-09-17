-- +goose Up
-- +goose StatementBegin
-- Токен печати задания отчёта.
--
-- PDF и слайды PPTX печатает headless Chromium со страницы фронтенда
-- /print/report/:id, а у Chromium нет сессии пользователя, и давать её ему
-- нельзя. Вместо JWT страница получает одноразовый токен задания: в БД
-- хранится только его SHA-256, выдача снимка обнуляет хэш, срок ограничен
-- временем подготовки. Истёкший или использованный токен даёт 403.
IF COL_LENGTH('dbo.tbl_ReportJobs', 'print_token_hash') IS NULL
BEGIN
    ALTER TABLE dbo.tbl_ReportJobs ADD print_token_hash NVARCHAR(64) NULL;
END;
IF COL_LENGTH('dbo.tbl_ReportJobs', 'print_token_expires_at') IS NULL
BEGIN
    ALTER TABLE dbo.tbl_ReportJobs ADD print_token_expires_at DATETIME2 NULL;
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
IF COL_LENGTH('dbo.tbl_ReportJobs', 'print_token_expires_at') IS NOT NULL
    ALTER TABLE dbo.tbl_ReportJobs DROP COLUMN print_token_expires_at;
IF COL_LENGTH('dbo.tbl_ReportJobs', 'print_token_hash') IS NOT NULL
    ALTER TABLE dbo.tbl_ReportJobs DROP COLUMN print_token_hash;
-- +goose StatementEnd
