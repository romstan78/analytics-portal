-- +goose Up
-- +goose StatementBegin
-- Ключ идемпотентности на создание промо.
--
-- Ответ на «Сохранить» мог не дойти из-за сети, и повторное нажатие создавало
-- второе такое же промо. Дубли вычищал офлайн-скрипт sync_script/dedupe_promo.py,
-- то есть постфактум: до уборки они успевали попасть в отчёты. Ключ выдаёт
-- клиент на открытие формы, и повтор с тем же ключом возвращает уже созданную
-- запись вместо второй вставки.
--
-- Только для вставки: при обновлении от повторов защищает optimistic locking
-- по updated_at.
--
-- Ключ хранится отдельной таблицей, а не колонкой в tbl_PromoActivities:
-- к самому промо он отношения не имеет, живёт ровно до подтверждения записи
-- и в выборках не нужен.
IF OBJECT_ID('dbo.tbl_PromoIdempotency', 'U') IS NULL
BEGIN
    CREATE TABLE dbo.tbl_PromoIdempotency (
        idempotency_key NVARCHAR(64) NOT NULL,
        promo_id        INT NOT NULL,
        username        NVARCHAR(100) NOT NULL,
        created_at      DATETIME2 NOT NULL CONSTRAINT DF_PromoIdempotency_created_at DEFAULT SYSUTCDATETIME(),
        -- Уникальность ключа и есть защита от дубля: параллельный повтор
        -- падает на ней и откатывает свою вставку промо.
        CONSTRAINT PK_PromoIdempotency PRIMARY KEY (idempotency_key),
        CONSTRAINT FK_PromoIdempotency_Promo FOREIGN KEY (promo_id)
            REFERENCES dbo.tbl_PromoActivities(id)
    );
END;
-- +goose StatementEnd

-- +goose StatementBegin
-- Обслуживание: старые ключи чистятся по времени создания, повтор через
-- сутки после вставки — это уже осознанное создание второй записи.
IF NOT EXISTS (
    SELECT 1 FROM sys.indexes
    WHERE name = 'IX_PromoIdempotency_created_at' AND object_id = OBJECT_ID('dbo.tbl_PromoIdempotency')
)
BEGIN
    CREATE INDEX IX_PromoIdempotency_created_at ON dbo.tbl_PromoIdempotency (created_at);
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
IF OBJECT_ID('dbo.tbl_PromoIdempotency', 'U') IS NOT NULL
    DROP TABLE dbo.tbl_PromoIdempotency;
-- +goose StatementEnd
