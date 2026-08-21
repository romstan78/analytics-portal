-- +goose Up
-- +goose StatementBegin
-- Серверный реестр refresh-сессий. Позволяет отзывать выданные refresh-токены:
-- до этого logout лишь удалял cookie, и украденный токен оставался годен 7 дней.
--
-- Сам токен не хранится: в таблицу пишется SHA-256 от него, поэтому утечка
-- содержимого таблицы не даёт возможности выпустить access-токен.

IF OBJECT_ID('dbo.tbl_RefreshSessions', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_RefreshSessions (
        id           BIGINT IDENTITY(1,1) NOT NULL,
        username     NVARCHAR(100) NOT NULL,
        token_hash   CHAR(64) NOT NULL,
        issued_at    DATETIME2 NOT NULL CONSTRAINT DF_RefreshSessions_issued_at DEFAULT SYSUTCDATETIME(),
        expires_at   DATETIME2 NOT NULL,
        revoked_at   DATETIME2 NULL,
        revoke_cause NVARCHAR(30) NULL,
        CONSTRAINT PK_RefreshSessions PRIMARY KEY (id),
        CONSTRAINT UQ_RefreshSessions_token_hash UNIQUE (token_hash),
        CONSTRAINT CK_RefreshSessions_revoke_cause CHECK (
            revoke_cause IS NULL OR revoke_cause IN ('logout', 'rotated', 'reuse_detected', 'user_revoked')
        )
    );
END;

-- Поиск активных сессий пользователя при отзыве всей цепочки.
IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_RefreshSessions_username' AND object_id = OBJECT_ID('dbo.tbl_RefreshSessions')
)
BEGIN
    CREATE INDEX IX_RefreshSessions_username
        ON dbo.tbl_RefreshSessions (username, revoked_at)
        INCLUDE (expires_at);
END;

-- Очистка просроченных записей.
IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_RefreshSessions_expires_at' AND object_id = OBJECT_ID('dbo.tbl_RefreshSessions')
)
BEGIN
    CREATE INDEX IX_RefreshSessions_expires_at
        ON dbo.tbl_RefreshSessions (expires_at);
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS dbo.tbl_RefreshSessions;
-- +goose StatementEnd
