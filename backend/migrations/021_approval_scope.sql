-- +goose Up
-- Область согласования: чьи промо пользователь вправе видеть и согласовывать.
--
-- До сих пор роль давала доступ ко всей очереди: agreement1 и agreement2 видели
-- промо всех КАМов, а фильтр по КАМу приходил из query-параметра и потому
-- ничего не ограничивал. Для старшего КАМа, который согласует только своих
-- подчинённых, этого мало — нужно ограничение на стороне сервера.
--
-- Строка означает: «промо этого КАМа на этой ступени согласует этот
-- пользователь». Ступень хранится в строке, а не в карточке пользователя:
-- один и тот же руководитель может отвечать за разных КАМов на разных
-- ступенях, и это описывается теми же строками без отдельной таблицы.
--
-- Отсутствие строк у пользователя означает прежнее поведение — без
-- ограничения. Поэтому миграция ничего не ломает уже заведённым согласующим.
IF OBJECT_ID('dbo.tbl_ApprovalScope', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_ApprovalScope (
        id              INT IDENTITY(1,1) NOT NULL,
        username        NVARCHAR(255) NOT NULL,
        agreement_num   INT NOT NULL,
        kam             NVARCHAR(255) NOT NULL,
        created_at      DATETIME NOT NULL CONSTRAINT DF_ApprovalScope_created DEFAULT GETDATE(),
        CONSTRAINT PK_ApprovalScope PRIMARY KEY (id),
        CONSTRAINT UQ_ApprovalScope_row UNIQUE (username, agreement_num, kam),
        CONSTRAINT CK_ApprovalScope_stage CHECK (agreement_num IN (1, 2))
    )
END;

IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE object_id = OBJECT_ID('dbo.tbl_ApprovalScope') AND name = 'IX_ApprovalScope_user'
)
    CREATE INDEX IX_ApprovalScope_user ON dbo.tbl_ApprovalScope(username, agreement_num) INCLUDE (kam);

-- +goose Down
DROP TABLE IF EXISTS dbo.tbl_ApprovalScope;
