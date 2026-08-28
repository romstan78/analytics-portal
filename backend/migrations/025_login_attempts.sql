-- +goose Up
-- +goose StatementBegin
-- Учёт неудачных попыток входа и временная блокировка учётной записи.
--
-- Лимит по адресу (5 попыток в минуту) не мешает перебору с ботнета: каждый
-- адрес остаётся в рамках, а учётная запись перебирается сообща. Счётчик здесь
-- ведётся по логину и потому ловит распределённый подбор.
--
-- Строка заводится и для логина, которого нет в tbl_Users: иначе ответ на
-- несуществующий логин отличался бы от ответа на заблокированный, и по этой
-- разнице проверялось бы существование учётной записи — ровно то, что
-- закрывает сравнение с фиктивным хешем в handlers/auth.go.
--
-- Состояние живёт в БД, а не в памяти процесса: перезапуск не должен обнулять
-- защиту, иначе достаточно дождаться деплоя.
IF OBJECT_ID('dbo.tbl_LoginAttempts', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_LoginAttempts (
        username       NVARCHAR(100) NOT NULL,
        failed_count   INT NOT NULL CONSTRAINT DF_LoginAttempts_failed_count DEFAULT 0,
        last_failed_at DATETIME2 NULL,
        locked_until   DATETIME2 NULL,
        updated_at     DATETIME2 NOT NULL CONSTRAINT DF_LoginAttempts_updated_at DEFAULT SYSUTCDATETIME(),
        CONSTRAINT PK_LoginAttempts PRIMARY KEY (username)
    );
END;
-- +goose StatementEnd

-- +goose StatementBegin
-- Обслуживание: строки давно не заходивших чистятся по времени обновления.
IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_LoginAttempts_updated_at' AND object_id = OBJECT_ID('dbo.tbl_LoginAttempts')
)
BEGIN
    CREATE INDEX IX_LoginAttempts_updated_at ON dbo.tbl_LoginAttempts (updated_at);
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
IF OBJECT_ID('dbo.tbl_LoginAttempts', 'U') IS NOT NULL
    DROP TABLE dbo.tbl_LoginAttempts;
-- +goose StatementEnd
